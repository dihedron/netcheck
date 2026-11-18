package main

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

	"github.com/pion/dtls/v3"
)

// testServer starts the lab's server on a loopback port and returns its address.
func testServer(t *testing.T) *net.UDPAddr {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	tmpl := &x509.Certificate{SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: "dtlsd.test"}, NotBefore: time.Now().Add(-time.Hour), NotAfter: time.Now().Add(time.Hour)}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	l, err := listen(&net.UDPAddr{IP: net.IPv4(127, 0, 0, 1)}, tls.Certificate{Certificate: [][]byte{der}, PrivateKey: key})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = l.Close() })
	go serve(l, 10*time.Second)
	return l.Addr().(*net.UDPAddr)
}

// deaf is a socket that never delivers what it receives: a client on it
// sends its ClientHello and never sees the answer.
type deaf struct{ net.PacketConn }

func (d deaf) ReadFrom(b []byte) (int, net.Addr, error) {
	for {
		if _, _, err := d.PacketConn.ReadFrom(b); err != nil {
			return 0, nil, err
		}
	}
}

func handshake(t *testing.T, pc net.PacketConn, server *net.UDPAddr, within time.Duration) error {
	t.Helper()
	c, err := dtls.ClientWithOptions(pc, server, dtls.WithInsecureSkipVerify(true)) //nolint:gosec // a test of the server
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = c.Close() }()
	ctx, cancel := context.WithTimeout(context.Background(), within)
	defer cancel()
	return c.HandshakeContext(ctx)
}

// A check that gave up leaves a half-done handshake behind; the next check
// that is given the same source port must not be ignored (the soak found a
// server that did: .specs/netcheck-lab.md).
func TestAClientOnAReusedPortIsServed(t *testing.T) {
	server := testServer(t)
	for range 20 {
		first, err := net.ListenUDP("udp", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1)})
		if err != nil {
			t.Fatal(err)
		}
		port := first.LocalAddr().(*net.UDPAddr).Port
		// the abandoned attempt: the server gets the ClientHello, the client never answers
		_ = handshake(t, deaf{first}, server, 300*time.Millisecond)
		second, err := net.ListenUDP("udp", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: port})
		if err != nil {
			t.Fatal(err)
		}
		if err := handshake(t, second, server, 3*time.Second); err != nil {
			t.Fatalf("a client on the port %d of an abandoned handshake: %v", port, err)
		}
	}
}

func TestManyClientsAtOnce(t *testing.T) {
	server := testServer(t)
	errs := make(chan error, 64)
	for range 64 {
		go func() {
			pc, err := net.ListenUDP("udp", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1)})
			if err != nil {
				errs <- err
				return
			}
			errs <- handshake(t, pc, server, 5*time.Second)
		}()
	}
	for range 64 {
		if err := <-errs; err != nil {
			t.Error(err)
		}
	}
}

func FuzzClientHello(f *testing.F) {
	f.Add([]byte{})
	f.Add([]byte{22, 254, 253, 0, 0, 0, 0, 0, 0, 0, 0, 0, 50, 1, 0, 0, 38})
	f.Fuzz(func(t *testing.T, p []byte) { _, _ = clientHello(p) })
}
