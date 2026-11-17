package netcheck

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"math/big"
	"net"
	"testing"
	"time"
)

// listener returns a TCP listener of ours that accepts and closes every connection.
func listener(t *testing.T) string { return listenerB(t) }

func listenerB(t testing.TB) string {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
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
			_ = c.Close()
		}
	}()
	return l.Addr().String()
}

// closedPort returns an address where nothing listens, for TCP and UDP.
func closedPort(t *testing.T) string {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := l.Addr().String()
	_ = l.Close()
	return addr
}

// runOne runs one check, one attempt, with a short timeout.
func runOne(t *testing.T, p Protocol, address string, opts ...Option) Result {
	t.Helper()
	return RunCheck(context.Background(), Check{Address: address, Protocol: p, Retries: 1, Timeout: Duration(3 * time.Second)}, opts...)
}

func wantPassed(t *testing.T, r Result) {
	t.Helper()
	if r.Status != Passed || r.Err != nil {
		t.Fatalf("want passed, got %s: %v", r.Status, r.Err)
	}
}

func wantFailed(t *testing.T, r Result) {
	t.Helper()
	if r.Status != Failed || r.Err == nil {
		t.Fatalf("want failed, got %s", r.Status)
	}
}

// testCA is an ephemeral certificate authority.
type testCA struct {
	cert *x509.Certificate
	key  *ecdsa.PrivateKey
	pool *x509.CertPool
}

func newCA(t *testing.T) *testCA {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
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
	return &testCA{cert, key, pool}
}

// serverCert issues a server certificate for the names, valid for the given time.
func (ca *testCA) serverCert(t *testing.T, names []string, ips []net.IP, notAfter time.Time) tls.Certificate {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(2), Subject: pkix.Name{CommonName: "server"},
		NotBefore: time.Now().Add(-2 * time.Hour), NotAfter: notAfter,
		DNSNames: names, IPAddresses: ips, KeyUsage: x509.KeyUsageDigitalSignature, ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, ca.cert, &key.PublicKey, ca.key)
	if err != nil {
		t.Fatal(err)
	}
	return tls.Certificate{Certificate: [][]byte{der}, PrivateKey: key}
}

// tlsServer serves TLS on addr (e.g. "127.0.0.1:0") with the certificate;
// every accepted connection completes the handshake and is closed.
func tlsServer(t *testing.T, network, addr string, cert tls.Certificate) string {
	t.Helper()
	l, err := tls.Listen(network, addr, &tls.Config{Certificates: []tls.Certificate{cert}, MinVersion: tls.VersionTLS12})
	if err != nil {
		t.Skipf("cannot listen on %s: %v", addr, err)
	}
	t.Cleanup(func() { _ = l.Close() })
	go func() {
		for {
			c, err := l.Accept()
			if err != nil {
				return
			}
			go func() {
				defer func() { _ = c.Close() }()
				_ = c.SetDeadline(time.Now().Add(2 * time.Second))
				_ = c.(*tls.Conn).Handshake()
			}()
		}
	}()
	return l.Addr().String()
}
