package dtls

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"errors"
	"math/big"
	"net"
	"net/netip"
	"strings"
	"testing"
	"time"

	"github.com/dihedron/netcheck/v2/pkg/netcheck"
	pion "github.com/pion/dtls/v3"
	"go.uber.org/goleak"
)

func TestMain(m *testing.M) { goleak.VerifyTestMain(m) }

type ca struct {
	cert *x509.Certificate
	key  *ecdsa.PrivateKey
	pool *x509.CertPool
}

func newCA(t *testing.T) *ca {
	t.Helper()
	key, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: "test CA"},
		NotBefore: time.Now().Add(-time.Hour), NotAfter: time.Now().Add(time.Hour),
		IsCA: true, BasicConstraintsValid: true, KeyUsage: x509.KeyUsageCertSign,
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	cert, _ := x509.ParseCertificate(der)
	pool := x509.NewCertPool()
	pool.AddCert(cert)
	return &ca{cert, key, pool}
}

func (c *ca) serverCert(t *testing.T) tls.Certificate {
	t.Helper()
	key, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(2), Subject: pkix.Name{CommonName: "server"},
		NotBefore: time.Now().Add(-time.Hour), NotAfter: time.Now().Add(time.Hour),
		DNSNames: []string{"localhost"}, IPAddresses: []net.IP{net.ParseIP("127.0.0.1")},
		KeyUsage: x509.KeyUsageDigitalSignature, ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, c.cert, &key.PublicKey, c.key)
	if err != nil {
		t.Fatal(err)
	}
	return tls.Certificate{Certificate: [][]byte{der}, PrivateKey: key}
}

// server starts a pion DTLS server and returns its address.
func server(t *testing.T, cert tls.Certificate) string {
	t.Helper()
	l, err := pion.ListenWithOptions("udp", &net.UDPAddr{IP: net.ParseIP("127.0.0.1")}, pion.WithCertificates(cert))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = l.Close() })
	go func() {
		for {
			c, err := l.Accept()
			if err != nil {
				return
			}
			go func() {
				ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
				defer cancel()
				_ = c.(*pion.Conn).HandshakeContext(ctx) // the handshake is lazy: serve it
				time.Sleep(200 * time.Millisecond)
				_ = c.Close()
			}()
		}
	}()
	return l.Addr().String()
}

func run(t *testing.T, address string, tlsOpts *netcheck.TLSOptions, opts ...netcheck.Option) netcheck.Result {
	t.Helper()
	c := netcheck.Check{Address: address, Protocol: netcheck.DTLS, Retries: 1, Timeout: netcheck.Duration(2 * time.Second), TLS: tlsOpts}
	return netcheck.RunCheck(t.Context(), c, append([]netcheck.Option{netcheck.WithRunner(netcheck.DTLS, Runner())}, opts...)...)
}

// NC-08
func TestDTLSPasses(t *testing.T) {
	authority := newCA(t)
	addr := server(t, authority.serverCert(t))
	for name, o := range map[string]*netcheck.TLSOptions{
		"by IP":   {RootCAs: authority.pool},
		"by name": {RootCAs: authority.pool, ServerName: "localhost"},
	} {
		r := run(t, addr, o)
		if r.Status != netcheck.Passed {
			t.Fatalf("%s: %v", name, r.Err)
		}
		f := r.Detail.TLS
		if f == nil || !f.Verified || f.Version != "DTLS 1.2" || f.CipherSuite == "" || f.Issuer != "CN=test CA" || f.Subject != "CN=server" || len(f.SANs) == 0 {
			t.Errorf("%s: facts %+v", name, f)
		}
		if r.Detail.Remote != addr {
			t.Errorf("remote %q", r.Detail.Remote)
		}
	}
}

func TestDTLSVerifies(t *testing.T) {
	authority := newCA(t)
	addr := server(t, authority.serverCert(t))
	if r := run(t, addr, nil); r.Status != netcheck.Failed { // the system does not trust the test CA
		t.Errorf("untrusted: %s", r.Status)
	}
	if r := run(t, addr, &netcheck.TLSOptions{RootCAs: authority.pool, ServerName: "wrong.example"}); r.Status != netcheck.Failed {
		t.Errorf("wrong name: %s", r.Status)
	}
	r := run(t, addr, &netcheck.TLSOptions{InsecureSkipVerify: true})
	if r.Status != netcheck.Passed || r.Detail.TLS == nil || r.Detail.TLS.Verified {
		t.Errorf("skipping verification must pass and say so: %+v", r)
	}
}

// NC-08: a plain UDP port is not a DTLS server.
func TestDTLSFailsAgainstPlainUDP(t *testing.T) {
	pc, err := net.ListenPacket("udp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = pc.Close() }()
	c := netcheck.Check{Address: pc.LocalAddr().String(), Protocol: netcheck.DTLS, Retries: 1, Timeout: netcheck.Duration(400 * time.Millisecond)}
	start := time.Now()
	r := netcheck.RunCheck(t.Context(), c, netcheck.WithRunner(netcheck.DTLS, Runner()))
	if r.Status != netcheck.Failed || time.Since(start) > 3*time.Second {
		t.Fatalf("%s after %v", r.Status, time.Since(start))
	}
}

func TestDTLSCancellation(t *testing.T) {
	pc, _ := net.ListenPacket("udp", "127.0.0.1:0")
	defer func() { _ = pc.Close() }()
	ctx, cancel := context.WithTimeout(t.Context(), 150*time.Millisecond)
	defer cancel()
	c := netcheck.Check{Address: pc.LocalAddr().String(), Protocol: netcheck.DTLS, Retries: 1, Timeout: netcheck.Duration(30 * time.Second)}
	start := time.Now()
	_ = netcheck.RunCheck(ctx, c, netcheck.WithRunner(netcheck.DTLS, Runner()))
	if time.Since(start) > 2*time.Second {
		t.Fatalf("took %v", time.Since(start))
	}
}

// T-1 binds plug-in runners too.
func TestDTLSHonoursNetworks(t *testing.T) {
	authority := newCA(t)
	addr := server(t, authority.serverCert(t))
	r := run(t, addr, &netcheck.TLSOptions{RootCAs: authority.pool}, netcheck.WithDeniedNetworks(netip.MustParsePrefix("127.0.0.0/8")))
	if r.Status != netcheck.Failed || !errors.Is(r.Err, netcheck.ErrNetworkNotAllowed) {
		t.Fatalf("%s %v", r.Status, r.Err)
	}
}

func TestDTLSBadAddress(t *testing.T) {
	for _, a := range []string{"", "no-port", "127.0.0.1:notaport"} {
		if r := run(t, a, nil); r.Status != netcheck.Failed || strings.Contains(r.Err.Error(), "panic") {
			t.Errorf("%q: %s %v", a, r.Status, r.Err)
		}
	}
}
