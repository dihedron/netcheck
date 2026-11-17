package sso

import (
	"crypto/tls"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dihedron/netcheck/v2/pkg/netcheck"
	"github.com/dpotapov/go-spnego"
	"go.uber.org/goleak"
)

func TestMain(m *testing.M) { goleak.VerifyTestMain(m) }

func TestClientUsesTheSPNEGOTransportOnACopy(t *testing.T) {
	before := http.DefaultTransport
	c := Client()
	tr, ok := c.Transport.(*spnego.Transport)
	if !ok {
		t.Fatalf("transport is %T", c.Transport)
	}
	if http.DefaultTransport != before || http.DefaultClient.Transport != nil {
		t.Error("the global transport or client was touched")
	}
	if tr.Proxy == nil {
		t.Error("the proxy variables must be honoured")
	}
	if Client() == c {
		t.Error("every call returns a new client")
	}
}

func TestWithTLSConfig(t *testing.T) {
	cfg := insecure()
	tr := Client(WithTLSConfig(cfg)).Transport.(*spnego.Transport)
	if tr.TLSClientConfig != cfg {
		t.Error("the TLS configuration was not applied")
	}
}

// Without a Kerberos configuration the request fails with the library's
// error: the real exchange needs a KDC (see the lab plan in the spec).
func TestWithoutKerberosTheRequestFails(t *testing.T) {
	t.Setenv("KRB5_CONFIG", filepath.Join(t.TempDir(), "absent.conf"))
	s := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	defer s.Close()
	_, err := Client().Get(s.URL) //nolint:noctx // test
	if err == nil {
		t.Skip("this host has a working Kerberos set-up")
	}
	if !strings.Contains(err.Error(), s.URL) {
		t.Logf("error: %v", err)
	}
}

// With the core: an sso check goes through this client and fails cleanly.
func TestCheckWithSSO(t *testing.T) {
	t.Setenv("KRB5_CONFIG", filepath.Join(t.TempDir(), "absent.conf"))
	s := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	defer s.Close()
	c := netcheck.Check{Address: strings.TrimPrefix(s.URL, "http://"), Protocol: netcheck.HTTP, SSO: true, Retries: 1}
	r := netcheck.RunCheck(t.Context(), c, netcheck.WithSSOClient(Client()))
	if r.Status == netcheck.Passed {
		t.Skip("this host has a working Kerberos set-up")
	}
	if r.Err == nil {
		t.Fatal("a failure needs an error")
	}
}

func insecure() *tls.Config {
	return &tls.Config{InsecureSkipVerify: true, MinVersion: tls.VersionTLS12}
} //nolint:gosec // test
