//go:build lab

package acceptance

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/dihedron/netcheck/v2/pkg/netcheck"
)

// load runs netcheck as probe on a bundle location, with extra environment
// (KEY=value) for the process, and returns the reports and the exit status.
func load(t *testing.T, env []string, location string, flags ...string) ([]*netcheck.Report, result) {
	t.Helper()
	args := []string{"runuser", "-u", "probe", "--", "env"}
	args = append(args, env...)
	args = append(args, "netcheck", "--format=json")
	args = append(args, flags...)
	args = append(args, location)
	r := asProbeRaw(t, args...)
	var reports []*netcheck.Report
	if r.Stdout != "" {
		if err := json.Unmarshal([]byte(r.Stdout), &reports); err != nil {
			t.Fatalf("no valid JSON for %s (exit %d): %v\n%s\n%s", location, r.Code, err, r.Stdout, r.Stderr)
		}
	}
	return reports, r
}

// asProbeRaw runs a complete command line in nc-client (no runuser added).
func asProbeRaw(t *testing.T, args ...string) result { t.Helper(); return run(t, "nc-client", args...) }

// loaded fails the test unless the location loaded, ran and passed, and
// returns the report.
func loaded(t *testing.T, env []string, location, id string, flags ...string) *netcheck.Report {
	t.Helper()
	reports, r := load(t, env, location, flags...)
	if r.Code != 0 || len(reports) != 1 || reports[0].BundleID != id || !reports[0].OK() {
		t.Fatalf("%s: expected bundle %q to load and pass, exit %d:\n%s\n%s", location, id, r.Code, r.Stdout, r.Stderr)
	}
	return reports[0]
}

// refused fails the test unless the location could not be loaded (exit 1,
// nothing run) and the message says why.
func refused(t *testing.T, location string, why string, flags ...string) {
	t.Helper()
	reports, r := load(t, nil, location, flags...)
	if r.Code != 1 || len(reports) != 0 {
		t.Fatalf("%s: expected a load error (exit 1), got exit %d and %d reports:\n%s", location, r.Code, len(reports), r.Stderr)
	}
	if !has(r.Stderr, why) {
		t.Errorf("%s: the message does not mention %q:\n%s", location, why, r.Stderr)
	}
}

// LB-03: every location scheme against the real servers.

func TestSchemeFile(t *testing.T) {
	run(t, "nc-client", "sh", "-c", "printf 'id: from-file\\nchecks:\\n  - address: nc-servers.lab:80\\n' >/tmp/from-file.yaml && chmod 644 /tmp/from-file.yaml")
	loaded(t, nil, "/tmp/from-file.yaml", "from-file")
}

func TestSchemeHTTP(t *testing.T) {
	loaded(t, nil, "http://nc-servers.lab/bundle.yaml", "from-http")
	loaded(t, nil, "http://nc-servers.lab/bundle.json", "from-http-json") // the Content-Type says JSON
	loaded(t, nil, "http://[fd42:77:77:2::10]/bundle.yaml", "from-http")  // IPv6
}

func TestSchemeHTTPS(t *testing.T) {
	loaded(t, nil, "https://nc-servers.lab:8443/bundle.yaml", "from-http")
	refused(t, "https://nc-servers.lab:8446/bundle.yaml", "unknown authority") // verification is on
	loaded(t, nil, "https-://nc-servers.lab:8446/bundle.yaml", "from-http")    // the - scheme skips it
	refused(t, "https://nc-servers.lab:8444/bundle.yaml", "expired")
}

func TestSchemeRedis(t *testing.T) {
	loaded(t, nil, "redis://nc-servers.lab:6379?key=checks/lab", "from-redis")
	loaded(t, nil, "rediss://nc-servers.lab:6380?key=checks/lab", "from-redis") // TLS, trusted through the lab CA
	refused(t, "redis://nc-servers.lab:6379?key=checks/missing", "missing")
}

func TestSchemeConsul(t *testing.T) {
	loaded(t, nil, "consulkv://nc-servers.lab:8500?key=checks/lab", "from-consul-kv")
	loaded(t, nil, "consulkvs://nc-servers.lab:8501?key=checks/lab", "from-consul-kv")
	loaded(t, nil, "consulsr://nc-servers.lab:8500?service=web&tag=lab&meta=netcheck", "from-consul-sr")
	loaded(t, nil, "consulsrs://nc-servers.lab:8501?service=web&tag=lab&meta=netcheck", "from-consul-sr")
	refused(t, "consulkv://nc-servers.lab:8500?key=checks/missing", "missing")
}

func TestSourceFailures(t *testing.T) {
	refused(t, "gopher://nc-servers.lab/x", "no source registered")
	refused(t, "http://nc-servers.lab/nothing", "404")
	refused(t, "http://nc-servers.lab/huge", "larger than")
	refused(t, "redis://nc-servers.lab:1?key=x", "refused")
}

// A server that accepts and never answers ends the load at --fetch-timeout:
// the lab found that the command hung for ever.
func TestSourceHangingServer(t *testing.T) {
	start := time.Now()
	refused(t, "http://nc-servers.lab:9201/bundle.yaml", "deadline exceeded", "--fetch-timeout=3s")
	if took := time.Since(start); took > 15*time.Second {
		t.Errorf("took %v", took)
	}
}

// The real Kerberos exchange: the probe user has a ticket from the lab's KDC,
// and the server accepts only SPNEGO.
func TestSchemeSSO(t *testing.T) {
	const ccache = "KRB5CCNAME=FILE:/tmp/krb5cc_lab"
	if r := run(t, "nc-client", "runuser", "-u", "probe", "--", "env", ccache, "sh", "-c", "kinit probe@LAB.TEST </home/probe/.krb5pw"); r.Code != 0 {
		t.Fatalf("kinit: %s %s", r.Stdout, r.Stderr)
	}
	t.Cleanup(func() { run(t, "nc-client", "rm", "-f", "/tmp/krb5cc_lab") })
	loaded(t, []string{ccache}, "http+sso://nc-servers.lab:8081/secure/bundle.yaml", "from-http")
	// without a ticket the server answers 401 and the load fails
	run(t, "nc-client", "runuser", "-u", "probe", "--", "env", ccache, "kdestroy")
	reports, r := load(t, []string{ccache}, "http+sso://nc-servers.lab:8081/secure/bundle.yaml")
	if r.Code != 1 || len(reports) != 0 {
		t.Errorf("without a ticket: exit %d, %d reports\n%s", r.Code, len(reports), r.Stderr)
	}
	// and a plain http location is refused by the server
	refused(t, "http://nc-servers.lab:8081/secure/bundle.yaml", "401")
}
