package netcheck

import (
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"strings"
	"testing"
	"time"
)

func prefixes(t *testing.T, s ...string) []netip.Prefix {
	t.Helper()
	var out []netip.Prefix
	for _, p := range s {
		out = append(out, netip.MustParsePrefix(p))
	}
	return out
}

// T-1: what a bundle may reach is bounded at dial time, on the resolved address.
func TestDeniedNetworks(t *testing.T) {
	addr := listener(t)
	deny := WithDeniedNetworks(prefixes(t, "127.0.0.0/8")...)
	for _, p := range []Protocol{TCP, TLS, SSH, UDP} {
		r := runOne(t, p, addr, deny)
		if r.Status != Failed || !errors.Is(r.Err, ErrNetworkNotAllowed) {
			t.Errorf("%s: %s %v, want ErrNetworkNotAllowed", p, r.Status, r.Err)
		}
	}
	wantPassed(t, runOne(t, TCP, addr, WithDeniedNetworks(prefixes(t, "10.0.0.0/8")...)))
}

func TestAllowedNetworks(t *testing.T) {
	addr := listener(t)
	wantPassed(t, runOne(t, TCP, addr, WithAllowedNetworks(prefixes(t, "127.0.0.1/32")...)))
	r := runOne(t, TCP, addr, WithAllowedNetworks(prefixes(t, "10.0.0.0/8")...))
	if !errors.Is(r.Err, ErrNetworkNotAllowed) {
		t.Fatalf("%v", r.Err)
	}
	// denied wins over allowed
	r = runOne(t, TCP, addr, WithAllowedNetworks(prefixes(t, "127.0.0.0/8")...), WithDeniedNetworks(prefixes(t, "127.0.0.1/32")...))
	if !errors.Is(r.Err, ErrNetworkNotAllowed) {
		t.Fatalf("%v", r.Err)
	}
}

// A redirect cannot lead out of the allowed networks.
func TestRedirectToDeniedNetwork(t *testing.T) {
	l, err := net.Listen("tcp", "127.0.0.2:0")
	if err != nil {
		t.Skipf("no 127.0.0.2 on this host: %v", err)
	}
	other := httptest.NewUnstartedServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	other.Listener = l
	other.Start()
	defer other.Close()
	first := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { http.Redirect(w, r, other.URL, http.StatusFound) }))
	defer first.Close()

	allow := WithAllowedNetworks(prefixes(t, "127.0.0.1/32")...)
	r := runOne(t, HTTP, strings.TrimPrefix(first.URL, "http://"), allow)
	if r.Status != Failed || !errors.Is(r.Err, ErrNetworkNotAllowed) {
		t.Fatalf("%s %v", r.Status, r.Err)
	}
	wantPassed(t, runOne(t, HTTP, strings.TrimPrefix(first.URL, "http://"), WithAllowedNetworks(prefixes(t, "127.0.0.0/8")...)))
}

func TestICMPHonoursNetworks(t *testing.T) {
	r := RunCheck(t.Context(), Check{Address: "127.0.0.1", Protocol: ICMP, Retries: 1, Timeout: Duration(time.Second)}, WithDeniedNetworks(prefixes(t, "127.0.0.0/8")...))
	if !errors.Is(r.Err, ErrNetworkNotAllowed) {
		t.Fatalf("%v", r.Err)
	}
}

func TestNetworkLimitsDisableTheProxy(t *testing.T) {
	// with limits the transport must dial the target itself: a proxy would
	// hide the real destination from the check
	h := httpRunner{cfg: newConfig([]Option{WithDeniedNetworks(prefixes(t, "10.0.0.0/8")...)})}
	client, err := h.client(Check{Protocol: HTTP})
	if err != nil {
		t.Fatal(err)
	}
	if tr := client.Transport.(*http.Transport); tr.Proxy != nil || tr.DialContext == nil {
		t.Error("proxy not disabled or dialer not set")
	}
}
