//go:build lab

package acceptance

import (
	"fmt"
	"strings"
	"testing"
)

// LB-04: the networks limits and the bounds, against real addresses on both of
// the lab's subnets.

func TestNetworkLimitsAcrossTheSubnets(t *testing.T) {
	wantPass(t, one(t, "tcp", addr("9001"), "--allow-net="+"10.78.0.0/24"))
	wantFail(t, one(t, "tcp", addr("9001"), "--deny-net=10.78.0.0/24"), "network not allowed")
	// the router is on the client's subnet: outside the allowed network
	wantFail(t, one(t, "icmp", "10.77.0.2", "--allow-net=10.78.0.0/24"), "network not allowed")
	wantPass(t, one(t, "icmp", "10.77.0.2", "--allow-net=10.77.0.0/24"))
	// a denied address inside an allowed network: deny wins
	wantFail(t, one(t, "tcp", addr("9001"), "--allow-net=10.78.0.0/24", "--deny-net="+serversIP()+"/32"), "network not allowed")
}

// The test is made on the address that is connected to: a name that resolves
// into a denied network is refused, and so is a redirect.
func TestNetworkLimitsFollowNamesAndRedirects(t *testing.T) {
	wantFail(t, one(t, "http", "denied.lab/ok", "--deny-net=10.78.0.20/32"), "network not allowed") // denied.lab is 10.78.0.20
	wantPass(t, one(t, "http", "nc-servers.lab/ok", "--deny-net=10.78.0.20/32"))
	// the server is allowed, but it redirects to a denied address
	wantFail(t, one(t, "http", "nc-servers.lab/to-denied", "--deny-net=10.78.0.20/32"), "network not allowed")
	// without the limit the redirect is followed and fails for another reason
	r := one(t, "http", "nc-servers.lab/to-denied")
	wantFail(t, r)
	if has(r.Err.Error(), "network not allowed") {
		t.Errorf("without a limit the error must not be about the limits: %v", r.Err)
	}
}

func TestNetworkLimitsApplyToDTLSAndSSH(t *testing.T) {
	wantFail(t, one(t, "dtls", "nc-servers.lab:4433", "--deny-net=10.78.0.0/24"), "network not allowed")
	wantFail(t, one(t, "ssh", serversIP(), "--deny-net=10.78.0.0/24"), "network not allowed")
	wantFail(t, one(t, "tls", "nc-servers.lab:8443", "--deny-net=10.78.0.0/24"), "network not allowed")
}

func TestBundleBounds(t *testing.T) {
	var many strings.Builder
	many.WriteString("id: many\nchecks:\n")
	for i := 0; i < 1001; i++ {
		fmt.Fprintf(&many, "  - address: %s\n", addr("9001"))
	}
	reports, code := check(t, many.String())
	if code != 1 || len(reports) != 0 {
		t.Errorf("1,001 checks: exit %d, %d reports, want 1 and none", code, len(reports))
	}
	reports, code = check(t, "id: r\nretries: 11\nchecks:\n  - address: "+addr("9001")+"\n")
	if code != 1 || len(reports) != 0 {
		t.Errorf("11 attempts: exit %d, want 1", code)
	}
	reports, code = check(t, "id: ok\nchecks:\n  - address: "+addr("9001")+"\n")
	if code != 0 || len(reports) != 1 {
		t.Errorf("a normal bundle: exit %d", code)
	}
}
