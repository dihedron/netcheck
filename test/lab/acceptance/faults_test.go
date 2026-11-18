//go:build lab

package acceptance

import (
	"strings"
	"testing"

	"github.com/dihedron/netcheck/v2/pkg/netcheck"
)

// LB-06: faults. The same bundle runs through phases in which the router drops,
// rejects and delays, and the servers stop; each report must match the truth
// table of its phase.

// The bundle: names tell what each check needs.
const faultBundle = `id: faults
retries: 1
timeout: 2s
wait: 10ms
concurrency: 8
checks:
  - {name: tcp,   protocol: tcp,  address: "10.78.0.10:9001"}
  - {name: tls,   protocol: tls,  address: "nc-servers.lab:8443"}
  - {name: http,  protocol: http, address: "nc-servers.lab/ok"}
  - {name: https, protocol: https, address: "nc-servers.lab:8443/ok"}
  - {name: udp,   protocol: udp,  address: "10.78.0.10:9101"}
  - {name: ssh,   protocol: ssh,  address: "10.78.0.10"}
  - {name: icmp,  protocol: icmp, address: "10.78.0.10"}
  - {name: dtls,  protocol: dtls, address: "nc-servers.lab:4433"}
`

var allChecks = []string{"tcp", "tls", "http", "https", "udp", "ssh", "icmp", "dtls"}

// statuses runs the bundle and returns the status of each check by name.
func statuses(t *testing.T) map[string]netcheck.Status {
	t.Helper()
	reports, _ := check(t, faultBundle)
	if len(reports) != 1 {
		t.Fatalf("no report")
	}
	got := map[string]netcheck.Status{}
	for _, r := range reports[0].Results {
		got[r.Check.Name] = r.Status
	}
	return got
}

// expect fails the test unless exactly the named checks failed.
func expect(t *testing.T, phase string, failing ...string) {
	t.Helper()
	want := map[string]bool{}
	for _, name := range failing {
		want[name] = true
	}
	got := statuses(t)
	for _, name := range allChecks {
		wantStatus := netcheck.Passed
		if want[name] {
			wantStatus = netcheck.Failed
		}
		if got[name] != wantStatus {
			t.Errorf("%s: %s is %s, want %s", phase, name, got[name], wantStatus)
		}
	}
}

func TestFaults(t *testing.T) {
	expect(t, "calm")

	// the router drops everything to the servers' web ports
	hits := fault(t, "ip daddr "+serversIP()+" tcp dport { 80, 8443 } counter drop")
	expect(t, "tcp 80 and 8443 dropped", "tls", "http", "https")
	if hits() == 0 {
		t.Error("the drop rule matched nothing")
	}
	run(t, "nc-fw", "nft", "flush", "chain", "inet", "lab", "faults")
	expect(t, "faults removed")

	// the router rejects: refused at once, not after the timeout
	fault(t, "ip daddr "+serversIP()+" tcp dport 9001 counter reject with tcp reset")
	expect(t, "tcp 9001 reset", "tcp")
	run(t, "nc-fw", "nft", "flush", "chain", "inet", "lab", "faults")

	// ICMP and UDP are filtered
	fault(t, "ip daddr "+serversIP()+" icmp type echo-request counter drop")
	expect(t, "icmp dropped", "icmp")
	run(t, "nc-fw", "nft", "flush", "chain", "inet", "lab", "faults")
	fault(t, "ip daddr "+serversIP()+" udp dport 4433 counter drop")
	expect(t, "dtls dropped", "dtls")
	run(t, "nc-fw", "nft", "flush", "chain", "inet", "lab", "faults")

	// a server that goes away: refused
	run(t, "nc-servers", "systemctl", "stop", "nginx")
	t.Cleanup(func() { run(t, "nc-servers", "systemctl", "start", "nginx") })
	expect(t, "nginx stopped", "tls", "http", "https")
	run(t, "nc-servers", "systemctl", "start", "nginx")
	expect(t, "nginx started")

	// delay beyond the timeout: everything over TCP and UDP times out, ICMP too
	run(t, "nc-fw", "tc", "qdisc", "replace", "dev", "enp6s0", "root", "netem", "delay", "2500ms")
	t.Cleanup(func() { run(t, "nc-fw", "tc", "qdisc", "del", "dev", "enp6s0", "root") })
	expect(t, "delay 2.5 s", "tcp", "tls", "http", "https", "ssh", "icmp", "dtls")
	run(t, "nc-fw", "tc", "qdisc", "del", "dev", "enp6s0", "root")
	expect(t, "delay removed")

	if hits := run(t, "nc-fw", "nft", "list", "chain", "inet", "lab", "faults").Stdout; strings.Contains(hits, "drop") {
		t.Errorf("a fault is left behind:\n%s", hits)
	}
}
