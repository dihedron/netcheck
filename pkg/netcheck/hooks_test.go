package netcheck

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"net"
	"net/netip"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func hookCheck(p Protocol, addr string, h *Hooks, o *TLSOptions) Check {
	return Check{Address: addr, Protocol: p, Retries: 1, Timeout: Duration(3 * time.Second), Hooks: h, TLS: o}
}

// NV-1
func TestHookAfterConnect(t *testing.T) {
	ca := newCA(t)
	addr := tlsServer(t, "tcp", "127.0.0.1:0", ca.serverCert(t, nil, []net.IP{net.ParseIP("127.0.0.1")}, time.Now().Add(time.Hour)))
	var called atomic.Int32
	h := &Hooks{AfterConnect: func(_ context.Context, c net.Conn) error {
		called.Add(1)
		if c.RemoteAddr().String() != addr {
			t.Errorf("connected to %s, want %s", c.RemoteAddr(), addr)
		}
		return nil
	}}
	wantPassed(t, RunCheck(t.Context(), hookCheck(TLS, addr, h, &TLSOptions{RootCAs: ca.pool})))
	if called.Load() != 1 {
		t.Errorf("AfterConnect ran %d times", called.Load())
	}
	h.AfterConnect = func(context.Context, net.Conn) error { return errors.New("not today") }
	r := RunCheck(t.Context(), hookCheck(TLS, addr, h, &TLSOptions{RootCAs: ca.pool}))
	wantFailed(t, r)
	if !strings.Contains(r.Err.Error(), "not today") {
		t.Errorf("error %v", r.Err)
	}
	// a tcp check calls it too
	called.Store(0)
	h.AfterConnect = func(context.Context, net.Conn) error { called.Add(1); return nil }
	wantPassed(t, RunCheck(t.Context(), hookCheck(TCP, addr, h, nil)))
	if called.Load() != 1 {
		t.Errorf("tcp: AfterConnect ran %d times", called.Load())
	}
}

// NV-2
func TestHookVerifyCertificate(t *testing.T) {
	ca := newCA(t)
	// a certificate for another name, from a CA nobody trusts here: only the hook can accept it
	addr := tlsServer(t, "tcp", "127.0.0.1:0", ca.serverCert(t, []string{"other.example"}, nil, time.Now().Add(time.Hour)))
	var seen *x509.Certificate
	h := &Hooks{VerifyCertificate: func(chain []*x509.Certificate) error { seen = chain[0]; return nil }}
	r := RunCheck(t.Context(), hookCheck(TLS, addr, h, nil))
	wantPassed(t, r)
	if f := r.Detail.TLS; f == nil || !f.Verified || f.Verifier != "custom" {
		t.Errorf("facts %+v", f)
	}
	if seen == nil || seen.Subject.CommonName != "server" {
		t.Errorf("the hook saw %v", seen)
	}
	h.VerifyCertificate = func([]*x509.Certificate) error { return errors.New("wrong SPIFFE ID") }
	r = RunCheck(t.Context(), hookCheck(TLS, addr, h, nil))
	wantFailed(t, r)
	if !strings.Contains(r.Err.Error(), "wrong SPIFFE ID") {
		t.Errorf("error %v", r.Err)
	}
	if r.Detail.TLS != nil && r.Detail.TLS.Verified {
		t.Errorf("a rejected certificate is verified: %+v", r.Detail.TLS)
	}
}

func TestHookAfterHandshake(t *testing.T) {
	ca := newCA(t)
	addr := tlsServer(t, "tcp", "127.0.0.1:0", ca.serverCert(t, nil, []net.IP{net.ParseIP("127.0.0.1")}, time.Now().Add(time.Hour)))
	var version uint16
	h := &Hooks{AfterHandshake: func(_ context.Context, s tls.ConnectionState) error { version = s.Version; return nil }}
	wantPassed(t, RunCheck(t.Context(), hookCheck(TLS, addr, h, &TLSOptions{RootCAs: ca.pool})))
	if version < tls.VersionTLS12 {
		t.Errorf("the hook saw version %x", version)
	}
	h.AfterHandshake = func(context.Context, tls.ConnectionState) error { return errors.New("TLS 1.3 required") }
	r := RunCheck(t.Context(), hookCheck(TLS, addr, h, &TLSOptions{RootCAs: ca.pool}))
	wantFailed(t, r)
	if !strings.Contains(r.Err.Error(), "TLS 1.3 required") {
		t.Errorf("error %v", r.Err)
	}
}

// NV-1: the run's hooks apply, the check's replace them field by field
func TestHooksLayering(t *testing.T) {
	addr := listener(t)
	var run, check atomic.Int32
	runHooks := Hooks{AfterConnect: func(context.Context, net.Conn) error { run.Add(1); return nil }}
	wantPassed(t, RunCheck(t.Context(), hookCheck(TCP, addr, nil, nil), WithHooks(runHooks)))
	if run.Load() != 1 {
		t.Errorf("the run's hook ran %d times", run.Load())
	}
	own := &Hooks{AfterConnect: func(context.Context, net.Conn) error { check.Add(1); return nil }}
	wantPassed(t, RunCheck(t.Context(), hookCheck(TCP, addr, own, nil), WithHooks(runHooks)))
	if run.Load() != 1 || check.Load() != 1 {
		t.Errorf("run %d, check %d: the check's hook must replace the run's", run.Load(), check.Load())
	}
}

// NV-1: BeforeConnect only vetoes; the limits cannot be widened by it
func TestHookBeforeConnect(t *testing.T) {
	addr := listener(t)
	var runSaw, checkSaw atomic.Int32
	runHooks := Hooks{BeforeConnect: func(_ context.Context, ip netip.Addr) error {
		runSaw.Add(1)
		if !ip.IsLoopback() {
			t.Errorf("address %s", ip)
		}
		return nil
	}}
	own := &Hooks{BeforeConnect: func(context.Context, netip.Addr) error { checkSaw.Add(1); return nil }}
	wantPassed(t, RunCheck(t.Context(), hookCheck(TCP, addr, own, nil), WithHooks(runHooks)))
	if runSaw.Load() != 1 || checkSaw.Load() != 1 {
		t.Errorf("run %d, check %d: both must run", runSaw.Load(), checkSaw.Load())
	}

	veto := &Hooks{BeforeConnect: func(context.Context, netip.Addr) error { return errors.New("not this one") }}
	r := RunCheck(t.Context(), hookCheck(TCP, addr, veto, nil))
	wantFailed(t, r)
	if !errors.Is(r.Err, ErrNetworkNotAllowed) || !strings.Contains(r.Err.Error(), "not this one") {
		t.Errorf("error %v", r.Err)
	}

	// a hook that says yes does not lift a denied network (T-V5)
	r = RunCheck(t.Context(), hookCheck(TCP, addr, own, nil), WithDeniedNetworks(netip.MustParsePrefix("127.0.0.0/8")))
	wantFailed(t, r)
	if !errors.Is(r.Err, ErrNetworkNotAllowed) {
		t.Errorf("error %v", r.Err)
	}
}

// NV-3
func TestHookValidation(t *testing.T) {
	yes := func(context.Context, net.Conn) error { return nil }
	for name, c := range map[string]Check{
		"AfterConnect on ssh":    hookCheck(SSH, "127.0.0.1:22", &Hooks{AfterConnect: yes}, nil),
		"VerifyCertificate, tcp": hookCheck(TCP, "127.0.0.1:22", &Hooks{VerifyCertificate: func([]*x509.Certificate) error { return nil }}, nil),
		"AfterHandshake, dtls":   hookCheck(DTLS, "127.0.0.1:22", &Hooks{AfterHandshake: func(context.Context, tls.ConnectionState) error { return nil }}, nil),
		"spiffe_id on tls":       {Address: "127.0.0.1:22", Protocol: TLS, SPIFFEID: "spiffe://example.org/x"},
	} {
		if err := c.Validate(); err == nil {
			t.Errorf("%s: valid", name)
		}
	}
	for name, c := range map[string]Check{
		"AfterConnect on tcp": hookCheck(TCP, "127.0.0.1:22", &Hooks{AfterConnect: yes}, nil),
		"BeforeConnect, ssh":  hookCheck(SSH, "127.0.0.1:22", &Hooks{BeforeConnect: func(context.Context, netip.Addr) error { return nil }}, nil),
		"spiffe_id on spire":  {Address: "127.0.0.1:22", Protocol: SPIRE, SPIFFEID: "spiffe://example.org/x"},
		"postgresql":          {Address: "127.0.0.1", Protocol: PostgreSQL},
	} {
		if err := c.Validate(); err != nil {
			t.Errorf("%s: %v", name, err)
		}
	}
}
