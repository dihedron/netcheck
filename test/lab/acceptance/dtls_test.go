//go:build lab

package acceptance

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// LB-09: DTLS under the conditions in which the first soak saw its only
// failures: a client that gives up on a handshake leaves a half-done one on
// the server, and a later client is given the same source port. The server's
// answer to that must not depend on what came before (dtlsd's listener; a
// timeout there reads as a failure of the check).

func envInt(name string, def int) int {
	if v, err := strconv.Atoi(os.Getenv(name)); err == nil && v > 0 {
		return v
	}
	return def
}

func dtlsBundle(n int) string {
	var b strings.Builder
	b.WriteString("id: dtls\nretries: 1\ntimeout: 4s\nwait: 10ms\nconcurrency: 64\nchecks:\n")
	for i := 0; i < n; i++ {
		fmt.Fprintf(&b, "  - {name: \"dtls#%d\", protocol: dtls, address: \"nc-servers.lab:4433\"}\n", i)
	}
	return b.String()
}

// After a cycle in which every handshake is abandoned (the path delays packets
// by more than the timeout), the very next calm cycle passes: the soak's
// pattern, with the installed command. NETCHECK_DTLS_CYCLES sets how many.
func TestDTLSAfterAbandonedHandshakes(t *testing.T) {
	for i := 0; i < envInt("NETCHECK_DTLS_CYCLES", 3); i++ {
		undo := netem("4500ms")(t)
		_, code := check(t, dtlsBundle(25))
		undo()
		if code != 2 {
			t.Fatalf("cycle %d: with every packet 4.5 s late the checks should all fail (exit 2), got exit %d", i, code)
		}
		reports, code := check(t, dtlsBundle(100))
		if code != 0 {
			t.Fatalf("cycle %d: the calm cycle after the abandoned handshakes failed (exit %d): %+v", i, code, reports)
		}
	}
}

// The probe abandons handshakes and reuses their source ports at random
// moments, against the server directly, far more often than the command
// would meet it. NETCHECK_DTLS_ROUNDS sets how many rounds.
func TestDTLSPortReuse(t *testing.T) {
	bin := filepath.Join(t.TempDir(), "dtlsprobe")
	build := exec.Command("go", "build", "-o", bin, "../dtlsprobe")
	build.Env = append(os.Environ(), "CGO_ENABLED=0", "GOOS=linux", "GOARCH=amd64")
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("building the probe: %v\n%s", err, out)
	}
	push := exec.Command("lxc", "--project", project(), "file", "push", bin, "nc-client/root/dtlsprobe", "--mode", "0755")
	if out, err := push.CombinedOutput(); err != nil {
		t.Fatalf("pushing the probe: %v\n%s", err, out)
	}
	r := run(t, "nc-client", "/root/dtlsprobe", "-rounds", strconv.Itoa(envInt("NETCHECK_DTLS_ROUNDS", 300)), "-workers", "48")
	t.Log(tail(r.Stdout, 3))
	if r.Code != 0 {
		t.Errorf("the server ignored clients on reused ports (exit %d):\n%s%s", r.Code, tail(r.Stdout, 20), r.Stderr)
	}
}
