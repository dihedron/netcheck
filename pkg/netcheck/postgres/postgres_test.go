package postgres

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"errors"
	"io"
	"math/big"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/dihedron/netcheck/v2/pkg/netcheck"
)

var sslRequestBytes = []byte{0, 0, 0, 8, 0x04, 0xd2, 0x16, 0x2f}

// server is a PostgreSQL stand-in: it reads the SSLRequest, answers with
// answer, and for 'S' completes a TLS handshake with cert.
func server(t *testing.T, answer []byte, cert *tls.Certificate) (addr string, requests chan []byte) {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = l.Close() })
	requests = make(chan []byte, 8)
	go func() {
		for {
			c, err := l.Accept()
			if err != nil {
				return
			}
			go func() {
				defer func() { _ = c.Close() }()
				buf := make([]byte, 8)
				if _, err := io.ReadFull(c, buf); err != nil {
					return
				}
				requests <- buf
				if len(answer) == 0 {
					return // EOF
				}
				if _, err := c.Write(answer); err != nil || answer[0] != 'S' {
					return
				}
				s := tls.Server(c, &tls.Config{Certificates: []tls.Certificate{*cert}})
				_ = s.Handshake()
			}()
		}
	}()
	return l.Addr().String(), requests
}

func ca(t *testing.T) (*x509.CertPool, *tls.Certificate) {
	t.Helper()
	key, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	tmpl := &x509.Certificate{SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: "pg CA"},
		NotBefore: time.Now().Add(-time.Hour), NotAfter: time.Now().Add(time.Hour), IsCA: true, BasicConstraintsValid: true, KeyUsage: x509.KeyUsageCertSign}
	der, _ := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	caCert, _ := x509.ParseCertificate(der)
	pool := x509.NewCertPool()
	pool.AddCert(caCert)
	skey, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	stmpl := &x509.Certificate{SerialNumber: big.NewInt(2), Subject: pkix.Name{CommonName: "pg"},
		NotBefore: time.Now().Add(-time.Hour), NotAfter: time.Now().Add(time.Hour), IPAddresses: []net.IP{net.ParseIP("127.0.0.1")},
		KeyUsage: x509.KeyUsageDigitalSignature, ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth}}
	sder, _ := x509.CreateCertificate(rand.Reader, stmpl, caCert, &skey.PublicKey, key)
	return pool, &tls.Certificate{Certificate: [][]byte{sder}, PrivateKey: skey}
}

func check(addr string, roots *x509.CertPool) netcheck.Check {
	c := netcheck.Check{Address: addr, Protocol: netcheck.PostgreSQL, Retries: 1, Timeout: netcheck.Duration(3 * time.Second)}
	if roots != nil {
		c.TLS = &netcheck.TLSOptions{RootCAs: roots}
	}
	return c
}

func run(t *testing.T, c netcheck.Check) netcheck.Result {
	t.Helper()
	return netcheck.RunCheck(context.Background(), c, netcheck.WithRunner(netcheck.PostgreSQL, Runner()))
}

// NV-4
func TestPostgresTLS(t *testing.T) {
	pool, cert := ca(t)
	addr, requests := server(t, []byte("S"), cert)
	r := run(t, check(addr, pool))
	if r.Status != netcheck.Passed {
		t.Fatalf("%s: %v", r.Status, r.Err)
	}
	if got := <-requests; !bytes.Equal(got, sslRequestBytes) {
		t.Errorf("the server received % x, want the SSLRequest", got)
	}
	if f := r.Detail.TLS; f == nil || !f.Verified || f.Subject != "CN=pg" {
		t.Errorf("facts %+v", f)
	}
}

func TestPostgresRefusals(t *testing.T) {
	pool, cert := ca(t)
	for name, tc := range map[string]struct {
		answer []byte
		want   string
	}{
		"no TLS":  {[]byte("N"), "does not support TLS"},
		"error":   {[]byte("E"), "error"},
		"garbage": {[]byte("X"), "unexpected"},
		"EOF":     {nil, "EOF"},
	} {
		addr, _ := server(t, tc.answer, cert)
		r := run(t, check(addr, pool))
		if r.Status != netcheck.Failed || r.Err == nil || !strings.Contains(r.Err.Error(), tc.want) {
			t.Errorf("%s: %s %v, want a failure about %q", name, r.Status, r.Err, tc.want)
		}
		if r.Detail.TLS != nil && r.Detail.TLS.Verified {
			t.Errorf("%s: verified", name)
		}
	}
}

func TestPostgresUntrusted(t *testing.T) {
	_, cert := ca(t)
	otherPool, _ := ca(t)
	addr, _ := server(t, []byte("S"), cert)
	r := run(t, check(addr, otherPool))
	if r.Status != netcheck.Failed {
		t.Fatalf("an untrusted certificate: %s", r.Status)
	}
	var unknown x509.UnknownAuthorityError
	if !errors.As(r.Err, &unknown) {
		t.Errorf("error %v", r.Err)
	}
}

// the check's own AfterConnect still runs, after the SSLRequest
func TestPostgresOwnHook(t *testing.T) {
	pool, cert := ca(t)
	addr, _ := server(t, []byte("S"), cert)
	c := check(addr, pool)
	var ran bool
	c.Hooks = &netcheck.Hooks{AfterConnect: func(context.Context, net.Conn) error { ran = true; return nil }}
	if r := run(t, c); r.Status != netcheck.Passed || !ran {
		t.Errorf("%s %v, hook ran: %v", r.Status, r.Err, ran)
	}
}

func FuzzAnswer(f *testing.F) {
	for _, b := range []byte("SNE\x00X") {
		f.Add(b)
	}
	f.Fuzz(func(t *testing.T, b byte) {
		err := answer(b)
		if (b == 'S') != (err == nil) {
			t.Errorf("answer(%q) = %v", b, err)
		}
	})
}
