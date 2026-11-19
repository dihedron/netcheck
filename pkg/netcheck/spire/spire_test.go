package spire

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/dihedron/netcheck/v2/pkg/netcheck"
	"github.com/spiffe/go-spiffe/v2/bundle/x509bundle"
	"github.com/spiffe/go-spiffe/v2/spiffeid"
)

const id = "spiffe://example.org/ns/lab/sa/server"

type authority struct {
	cert *x509.Certificate
	key  *ecdsa.PrivateKey
}

func newAuthority(t *testing.T) *authority {
	t.Helper()
	key, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	tmpl := &x509.Certificate{SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: "spire CA"},
		NotBefore: time.Now().Add(-time.Hour), NotAfter: time.Now().Add(time.Hour), IsCA: true, BasicConstraintsValid: true, KeyUsage: x509.KeyUsageCertSign}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	cert, _ := x509.ParseCertificate(der)
	return &authority{cert, key}
}

func (a *authority) pem() []byte {
	return pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: a.cert.Raw})
}

// svid issues a leaf with the URI SAN id (none when id is empty) and a DNS name that is not the host's.
func (a *authority) svid(t *testing.T, id string, notAfter time.Time) tls.Certificate {
	t.Helper()
	key, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	tmpl := &x509.Certificate{SerialNumber: big.NewInt(2), NotBefore: time.Now().Add(-2 * time.Hour), NotAfter: notAfter,
		KeyUsage: x509.KeyUsageDigitalSignature, ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth, x509.ExtKeyUsageClientAuth}}
	if id != "" {
		u, _ := url.Parse(id)
		tmpl.URIs = []*url.URL{u}
	} else {
		tmpl.DNSNames = []string{"other.example"}
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, a.cert, &key.PublicKey, a.key)
	if err != nil {
		t.Fatal(err)
	}
	return tls.Certificate{Certificate: [][]byte{der}, PrivateKey: key}
}

func serve(t *testing.T, cert tls.Certificate) string {
	t.Helper()
	l, err := tls.Listen("tcp", "127.0.0.1:0", &tls.Config{Certificates: []tls.Certificate{cert}})
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
			go func() { _ = c.(*tls.Conn).Handshake(); _ = c.Close() }()
		}
	}()
	return l.Addr().String()
}

type source struct{ b *x509bundle.Bundle }

func (s source) GetX509BundleForTrustDomain(spiffeid.TrustDomain) (*x509bundle.Bundle, error) {
	return s.b, nil
}

func trusting(a *authority) Option {
	return WithSource(source{x509bundle.FromX509Authorities(spiffeid.RequireTrustDomainFromString("example.org"), []*x509.Certificate{a.cert})})
}

func run(addr, want string, opts ...Option) netcheck.Result {
	c := netcheck.Check{Address: addr, Protocol: netcheck.SPIRE, SPIFFEID: want, Retries: 1, Timeout: netcheck.Duration(3 * time.Second)}
	return netcheck.RunCheck(context.Background(), c, netcheck.WithRunner(netcheck.SPIRE, Runner(opts...)))
}

// NV-5
func TestSPIREPasses(t *testing.T) {
	a := newAuthority(t)
	addr := serve(t, a.svid(t, id, time.Now().Add(time.Hour)))
	for name, want := range map[string]string{"any ID of the bundle": "", "the expected ID": id} {
		r := run(addr, want, trusting(a))
		if r.Status != netcheck.Passed {
			t.Fatalf("%s: %v", name, r.Err)
		}
		f := r.Detail.TLS
		if f == nil || !f.Verified || f.Verifier != "custom" || !contains(f.SANs, id) {
			t.Errorf("%s: facts %+v", name, f)
		}
	}
}

func TestSPIRERefusals(t *testing.T) {
	a, other := newAuthority(t), newAuthority(t)
	good := serve(t, a.svid(t, id, time.Now().Add(time.Hour)))
	for name, tc := range map[string]struct {
		addr, want string
		opt        Option
		reason     string
	}{
		"wrong ID":        {good, "spiffe://example.org/ns/lab/sa/other", trusting(a), "expected"},
		"wrong authority": {good, "", trusting(other), "verifying"},
		"expired":         {serve(t, a.svid(t, id, time.Now().Add(-time.Minute))), "", trusting(a), "verifying"},
		"no URI SAN":      {serve(t, a.svid(t, "", time.Now().Add(time.Hour))), "", trusting(a), "verifying"},
		"no bundle":       {good, "", func(*runner) {}, "no trust bundle"},
	} {
		r := run(tc.addr, tc.want, tc.opt)
		if r.Status != netcheck.Failed || r.Err == nil || !strings.Contains(r.Err.Error(), tc.reason) {
			t.Errorf("%s: %s %v, want a failure about %q", name, r.Status, r.Err, tc.reason)
			continue
		}
		if r.Detail.TLS != nil && r.Detail.TLS.Verified {
			t.Errorf("%s: verified", name)
		}
	}
	if r := run(good, "not an id", trusting(a)); r.Status != netcheck.Failed || !strings.Contains(r.Err.Error(), "spiffe_id") {
		t.Errorf("a malformed spiffe_id: %s %v", r.Status, r.Err)
	}
}

// the file is read at each attempt: a rotated bundle is picked up
func TestSPIREBundleFile(t *testing.T) {
	a, b := newAuthority(t), newAuthority(t)
	file := filepath.Join(t.TempDir(), "bundle.pem")
	if err := os.WriteFile(file, a.pem(), 0o600); err != nil {
		t.Fatal(err)
	}
	addr := serve(t, b.svid(t, id, time.Now().Add(time.Hour)))
	r := run(addr, "", WithBundleFile(file))
	if r.Status != netcheck.Failed {
		t.Fatalf("before the rotation: %s", r.Status)
	}
	if err := os.WriteFile(file, b.pem(), 0o600); err != nil {
		t.Fatal(err)
	}
	if r := run(addr, "", WithBundleFile(file)); r.Status != netcheck.Passed {
		t.Fatalf("after the rotation: %v", r.Err)
	}
	if r := run(addr, "", WithBundleFile(file+".missing")); r.Status != netcheck.Failed {
		t.Errorf("a missing file: %s", r.Status)
	}
}

func contains(l []string, s string) bool {
	for _, x := range l {
		if x == s {
			return true
		}
	}
	return false
}
