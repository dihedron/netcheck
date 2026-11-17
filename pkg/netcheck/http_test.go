package netcheck

import (
	"crypto/x509"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
)

func httpRun(t *testing.T, p Protocol, addr string, mod func(*Check)) Result {
	t.Helper()
	c := Check{Address: addr, Protocol: p, Retries: 1}
	if mod != nil {
		mod(&c)
	}
	return RunCheck(t.Context(), c)
}

// NC-10
func TestHTTPStatuses(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/ok", func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte("ok")) })
	mux.HandleFunc("/moved", func(w http.ResponseWriter, r *http.Request) { http.Redirect(w, r, "/ok", http.StatusFound) })
	mux.HandleFunc("/missing", http.NotFound)
	mux.HandleFunc("/boom", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(500) })
	mux.HandleFunc("/loop", func(w http.ResponseWriter, r *http.Request) { http.Redirect(w, r, "/loop", http.StatusFound) })
	mux.HandleFunc("/notmodified", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNotModified) })
	server := httptest.NewServer(mux)
	defer server.Close()
	host := strings.TrimPrefix(server.URL, "http://")

	for path, want := range map[string]struct {
		pass   bool
		status int
	}{"/ok": {true, 200}, "/moved": {true, 200}, "/notmodified": {true, 304}, "/missing": {false, 404}, "/boom": {false, 500}, "/loop": {false, 0}} {
		r := httpRun(t, HTTP, host+path, nil)
		if (r.Status == Passed) != want.pass || r.Detail.HTTPStatus != want.status {
			t.Errorf("%s: %s status %d (%v), want pass=%v status %d", path, r.Status, r.Detail.HTTPStatus, r.Err, want.pass, want.status)
		}
	}
	wantFailed(t, httpRun(t, HTTP, closedPort(t), nil))
}

// The body is read up to a cap and discarded.
func TestHTTPBodyIsCapped(t *testing.T) {
	var sent atomic.Int64
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		chunk := make([]byte, 32<<10)
		for i := 0; i < 1000 && r.Context().Err() == nil; i++ {
			n, err := w.Write(chunk)
			sent.Add(int64(n))
			if err != nil {
				return
			}
		}
	}))
	defer server.Close()
	wantPassed(t, httpRun(t, HTTP, strings.TrimPrefix(server.URL, "http://"), nil))
	if n := sent.Load(); n > 8<<20 {
		t.Errorf("the check let the server send %d bytes", n)
	}
}

func TestHTTPSAndCertificates(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {}))
	defer server.Close()
	host := strings.TrimPrefix(server.URL, "https://")
	wantFailed(t, httpRun(t, HTTPS, host, nil)) // untrusted CA

	pool := x509.NewCertPool()
	pool.AddCert(server.Certificate())
	r := httpRun(t, HTTPS, host, func(c *Check) { c.TLS = &TLSOptions{RootCAs: pool} })
	wantPassed(t, r)
	if r.Detail.HTTPStatus != 200 || r.Detail.TLS == nil || !r.Detail.TLS.Verified {
		t.Errorf("%+v", r.Detail)
	}
	r = httpRun(t, HTTPS, host, func(c *Check) { c.TLS = &TLSOptions{InsecureSkipVerify: true} })
	wantPassed(t, r)
	if r.Detail.TLS == nil || r.Detail.TLS.Verified {
		t.Errorf("skipped verification must be reported: %+v", r.Detail.TLS)
	}
	// WithHTTPClient is used as given and left untouched
	client := server.Client()
	before := client.Transport
	wantPassed(t, RunCheck(t.Context(), Check{Address: host, Protocol: HTTPS, Retries: 1}, WithHTTPClient(client)))
	if client.Transport != before || client.CheckRedirect != nil {
		t.Error("the caller's client was modified")
	}
}

// T-8
func TestHTTPSDowngrade(t *testing.T) {
	plain := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	defer plain.Close()
	secure := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { http.Redirect(w, r, plain.URL, http.StatusFound) }))
	defer secure.Close()
	pool := x509.NewCertPool()
	pool.AddCert(secure.Certificate())
	host := strings.TrimPrefix(secure.URL, "https://")

	r := httpRun(t, HTTPS, host, func(c *Check) { c.TLS = &TLSOptions{RootCAs: pool} })
	if r.Status != Failed || !strings.Contains(r.Err.Error(), "downgrade") {
		t.Fatalf("%+v", r)
	}
	wantPassed(t, httpRun(t, HTTPS, host, func(c *Check) { c.TLS = &TLSOptions{RootCAs: pool}; c.AllowDowngrade = true }))
}

func TestHTTPSSONeedsAPlugin(t *testing.T) {
	r := httpRun(t, HTTP, "localhost:1", func(c *Check) { c.SSO = true })
	if r.Status != Failed || !strings.Contains(r.Err.Error(), "sso") {
		t.Fatalf("%+v", r)
	}
}

// NC-17: the credentials of an address do not come back in the result.
func TestHTTPCredentialsDoNotLeak(t *testing.T) {
	r := httpRun(t, HTTP, "user:s3cret@"+closedPort(t)+"/x", nil)
	wantFailed(t, r)
	if strings.Contains(r.Err.Error(), "s3cret") || strings.Contains(r.Check.Address, "s3cret") {
		t.Fatalf("credentials leaked: %v / %s", r.Err, r.Check.Address)
	}
}

// An SSO check uses the client it was given, as it is; TLS options and
// network limits cannot be combined with it (they would not apply).
func TestHTTPSSOUsesTheSSOClient(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	defer server.Close()
	host := strings.TrimPrefix(server.URL, "http://")
	var used atomic.Int32
	client := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		used.Add(1)
		return http.DefaultTransport.RoundTrip(r)
	})}
	c := Check{Address: host, Protocol: HTTP, Retries: 1, SSO: true}
	wantPassed(t, RunCheck(t.Context(), c, WithSSOClient(client)))
	if used.Load() != 1 {
		t.Fatalf("the SSO client was used %d times", used.Load())
	}
	c.TLS = &TLSOptions{}
	if r := RunCheck(t.Context(), c, WithSSOClient(client)); r.Status != Failed || !strings.Contains(r.Err.Error(), "sso") {
		t.Errorf("TLS options with SSO: %+v", r)
	}
	c.TLS = nil
	if r := RunCheck(t.Context(), c, WithSSOClient(client), WithDeniedNetworks(prefixes(t, "10.0.0.0/8")...)); r.Status != Failed || !strings.Contains(r.Err.Error(), "sso") {
		t.Errorf("network limits with SSO: %+v", r)
	}
	// a plain check does not use it
	used.Store(0)
	wantPassed(t, RunCheck(t.Context(), Check{Address: host, Protocol: HTTP, Retries: 1}, WithSSOClient(client)))
	if used.Load() != 0 {
		t.Error("a check without SSO used the SSO client")
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
