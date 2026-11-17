//go:build lab

// Package acceptance runs the lab's scenarios (.specs/netcheck-lab.md) against
// the package installed in the lab's VMs. It needs a lab that is up and
// deployed: test/lab/lab.sh up, deploy, test.
package acceptance

import (
	"bytes"
	"encoding/json"
	"errors"
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
