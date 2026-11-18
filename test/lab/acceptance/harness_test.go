//go:build lab

// Package acceptance runs the lab's scenarios (.specs/netcheck-lab.md) against
// the package installed in the lab's VMs. It needs a lab that is up and
// deployed: test/lab/lab.sh up, deploy, test.
package acceptance

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"testing"

	"github.com/dihedron/netcheck/v2/pkg/netcheck"
)

// result is what a command run in a VM left behind.
type result struct {
	Stdout, Stderr string
	Code           int
}

func project() string {
	if p := os.Getenv("NETCHECK_LAB_PROJECT"); p != "" {
		return p
	}
	return "netcheck-lab"
}

// serversIP is the address of the real servers.
func serversIP() string {
	if ip := os.Getenv("NETCHECK_LAB_SERVERS_IP"); ip != "" {
		return ip
	}
	return "10.78.0.10"
}

// run executes a command in a VM as root and returns its output and status;
// an error to start the command at all fails the test.
func run(t *testing.T, vm string, args ...string) result {
	t.Helper()
	return runIn(t, vm, "", args...)
}

// runIn is run with a standard input.
func runIn(t *testing.T, vm, stdin string, args ...string) result {
	t.Helper()
	cmd := exec.Command("lxc", append([]string{"--project", project(), "exec", "--cwd", "/tmp", vm, "--"}, args...)...)
	var out, errb bytes.Buffer
	cmd.Stdin = strings.NewReader(stdin)
	cmd.Stdout, cmd.Stderr = &out, &errb
	err := cmd.Run()
	r := result{Stdout: out.String(), Stderr: errb.String()}
	var exit *exec.ExitError
	switch {
	case err == nil:
	case errors.As(err, &exit):
		r.Code = exit.ExitCode()
	default:
		t.Fatalf("lxc exec %s %v: %v", vm, args, err)
	}
	return r
}

// asProbe runs a command in nc-client as the unprivileged user probe.
func asProbe(t *testing.T, args ...string) result {
	t.Helper()
	return run(t, "nc-client", append([]string{"runuser", "-u", "probe", "--"}, args...)...)
}

// check runs netcheck on nc-client as probe on a bundle given as YAML, and
// returns the reports it printed and the exit status.
func check(t *testing.T, bundle string, flags ...string) ([]*netcheck.Report, int) {
	t.Helper()
	path := "/tmp/lab-bundle.yaml"
	put := runIn(t, "nc-client", bundle, "sh", "-c", "cat >"+path+" && chmod 644 "+path)
	if put.Code != 0 {
		t.Fatalf("writing the bundle: %s", put.Stderr)
	}
	args := append([]string{"netcheck", "--format=json"}, flags...)
	r := asProbe(t, append(args, path)...)
	var reports []*netcheck.Report
	if r.Stdout != "" {
		if err := json.Unmarshal([]byte(r.Stdout), &reports); err != nil {
			t.Fatalf("netcheck printed no valid JSON (exit %d): %v\nstdout: %s\nstderr: %s", r.Code, err, r.Stdout, r.Stderr)
		}
	}
	return reports, r.Code
}

func has(s, sub string) bool { return strings.Contains(s, sub) }

// one runs a single check from nc-client, as probe, once (no retries, a short
// timeout), and returns its result.
func one(t *testing.T, protocol, address string, flags ...string) netcheck.Result {
	t.Helper()
	bundle := "id: one\nretries: 1\ntimeout: 3s\nchecks:\n  - name: one\n    protocol: " + protocol + "\n    address: \"" + address + "\"\n"
	reports, _ := check(t, bundle, flags...)
	if len(reports) != 1 || len(reports[0].Results) != 1 {
		t.Fatalf("expected one report with one result, got %+v", reports)
	}
	return reports[0].Results[0]
}

// wantPass fails the test unless the result passed.
func wantPass(t *testing.T, r netcheck.Result) {
	t.Helper()
	if r.Status != netcheck.Passed {
		t.Fatalf("expected the check to pass, got %s: %v (detail %+v)", r.Status, r.Err, r.Detail)
	}
}

// wantFail fails the test unless the result failed with an error that
// mentions every given text.
func wantFail(t *testing.T, r netcheck.Result, texts ...string) {
	t.Helper()
	if r.Status != netcheck.Failed || r.Err == nil {
		t.Fatalf("expected the check to fail, got %s (detail %+v)", r.Status, r.Detail)
	}
	for _, text := range texts {
		if !has(strings.ToLower(r.Err.Error()), strings.ToLower(text)) {
			t.Errorf("the error %q does not mention %q", r.Err, text)
		}
	}
}

// fault puts a rule (which must hold a counter) in the router's fault chain and removes
// every fault when the test ends. It returns a function that says how many
// packets the rule matched: proof that the traffic crosses the router.
func fault(t *testing.T, rule string) func() int {
	t.Helper()
	t.Cleanup(func() { run(t, "nc-fw", "nft", "flush", "chain", "inet", "lab", "faults") })
	if r := run(t, "nc-fw", "sh", "-c", "nft add rule inet lab faults "+rule); r.Code != 0 {
		t.Fatalf("adding the fault %q: %s", rule, r.Stderr)
	}
	return func() int {
		out := run(t, "nc-fw", "nft", "list", "chain", "inet", "lab", "faults").Stdout
		var n int
		if i := strings.Index(out, "packets "); i >= 0 {
			_, _ = fmt.Sscanf(out[i:], "packets %d", &n)
		}
		return n
	}
}
