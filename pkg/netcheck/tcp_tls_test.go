package netcheck

import (
	"net"
	"strings"
	"testing"
	"time"
)

// NC-06
func TestTCP(t *testing.T) {
	addr := listener(t)
	r := runOne(t, TCP, addr)
	wantPassed(t, r)
	if r.Detail.Remote != addr {
		t.Errorf("remote %q, want %q", r.Detail.Remote, addr)
	}
	wantFailed(t, runOne(t, TCP, closedPort(t)))
}

func TestTCPTimesOut(t *testing.T) {
	start := time.Now()
	r := RunCheck(t.Context(), Check{Address: "192.0.2.1:9", Protocol: TCP, Retries: 1, Timeout: Duration(200 * time.Millisecond)}) // TEST-NET-1: black hole
	wantFailed(t, r)
	if time.Since(start) > 2*time.Second {
		t.Fatalf("took %v", time.Since(start))
	}
}

// NC-07
func TestTLSTrusted(t *testing.T) {
	ca := newCA(t)
	addr := tlsServer(t, "tcp", "127.0.0.1:0", ca.serverCert(t, []string{"localhost"}, []net.IP{net.ParseIP("127.0.0.1")}, time.Now().Add(time.Hour)))
	for name, opts := range map[string]*TLSOptions{
		"by IP":   {RootCAs: ca.pool},
		"by name": {RootCAs: ca.pool, ServerName: "localhost"},
	} {
		c := Check{Address: addr, Protocol: TLS, Retries: 1, Timeout: Duration(3 * time.Second), TLS: opts}
		r := RunCheck(t.Context(), c)
		if r.Status != Passed {
			t.Fatalf("%s: %v", name, r.Err)
		}
		f := r.Detail.TLS
		if f == nil || !f.Verified || f.Version == "" || f.CipherSuite == "" || f.Issuer != "CN=test CA" || f.Subject != "CN=server" || f.NotAfter.IsZero() || len(f.SANs) == 0 {
			t.Errorf("%s: facts %+v", name, f)
		}
	}
}

func TestTLSFailures(t *testing.T) {
	ca := newCA(t)
	good := tlsServer(t, "tcp", "127.0.0.1:0", ca.serverCert(t, []string{"localhost"}, []net.IP{net.ParseIP("127.0.0.1")}, time.Now().Add(time.Hour)))
	expired := tlsServer(t, "tcp", "127.0.0.1:0", ca.serverCert(t, []string{"localhost"}, []net.IP{net.ParseIP("127.0.0.1")}, time.Now().Add(-time.Hour)))
	run := func(addr string, o *TLSOptions) Result {
		return RunCheck(t.Context(), Check{Address: addr, Protocol: TLS, Retries: 1, Timeout: Duration(3 * time.Second), TLS: o})
	}

	r := run(good, nil) // the system does not trust the test CA
	wantFailed(t, r)
	if f := r.Detail.TLS; f == nil || f.Verified || f.Issuer != "CN=test CA" {
		t.Errorf("an unverified certificate is still described: %+v", f)
	}
	wantFailed(t, run(good, &TLSOptions{RootCAs: ca.pool, ServerName: "wrong.example"}))
	if r := run(expired, &TLSOptions{RootCAs: ca.pool}); r.Status != Failed || !strings.Contains(r.Err.Error(), "expired") {
		t.Errorf("expired: %+v", r)
	}
	wantFailed(t, runOne(t, TLS, listener(t))) // plain TCP service
	wantFailed(t, runOne(t, TLS, closedPort(t)))
}

func TestTLSSkipVerify(t *testing.T) {
	ca := newCA(t)
	addr := tlsServer(t, "tcp", "127.0.0.1:0", ca.serverCert(t, []string{"other"}, nil, time.Now().Add(time.Hour)))
	r := RunCheck(t.Context(), Check{Address: addr, Protocol: TLS, Retries: 1, TLS: &TLSOptions{InsecureSkipVerify: true}})
	wantPassed(t, r)
	if r.Detail.TLS == nil || r.Detail.TLS.Verified {
		t.Errorf("must record that nothing was verified: %+v", r.Detail.TLS)
	}
}

func TestTLSIPv6(t *testing.T) {
	ca := newCA(t)
	addr := tlsServer(t, "tcp6", "[::1]:0", ca.serverCert(t, nil, []net.IP{net.IPv6loopback}, time.Now().Add(time.Hour)))
	wantPassed(t, RunCheck(t.Context(), Check{Address: addr, Protocol: TLS, Retries: 1, TLS: &TLSOptions{RootCAs: ca.pool}}))
}
