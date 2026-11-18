//go:build lab

package acceptance

import (
	"strings"
	"testing"
)

// reinstall puts the package back as its install step leaves it: purged, then
// installed again from the file lab.sh deploy pushed. A test must never set the
// capability itself, or it would hide a package that does not.
func reinstall(t *testing.T) {
	t.Helper()
	if r := run(t, "nc-client", "dpkg", "--purge", "netcheck"); r.Code != 0 {
		t.Fatalf("purging the package: %s", r.Stderr)
	}
	if r := run(t, "nc-client", "dpkg", "-i", "/root/netcheck.deb"); r.Code != 0 {
		t.Fatalf("installing the package: %s", r.Stderr)
	}
}

// LB-01: the installed package carries the capability that lets an ordinary
// user send ICMP echo requests on raw sockets.
func TestPackageSetsCapNetRaw(t *testing.T) {
	reinstall(t)
	r := run(t, "nc-client", "getcap", "/usr/bin/netcheck")
	if !strings.Contains(r.Stdout, "cap_net_raw") {
		t.Fatalf("/usr/bin/netcheck has no cap_net_raw (getcap printed %q): the package's post-install step did not set it", strings.TrimSpace(r.Stdout))
	}
}

func TestPackageInstallsTheDefaultsFile(t *testing.T) {
	if r := run(t, "nc-client", "test", "-f", "/etc/netcheck.conf"); r.Code != 0 {
		t.Fatal("/etc/netcheck.conf is not installed")
	}
}

func TestIcmpAsAnOrdinaryUser(t *testing.T) {
	reports, code := check(t, "id: icmp\nretries: 1\nchecks:\n  - name: router\n    protocol: icmp\n    address: 10.77.0.2\n")
	if code != 0 {
		t.Fatalf("exit %d: %+v", code, reports)
	}
	res := reports[0].Results[0]
	if res.Detail.PacketsRecv == 0 {
		t.Errorf("no echo reply: %+v", res)
	}
}

// Without the capability the same check fails, and says why.
func TestIcmpWithoutTheCapabilityNamesThePrivilege(t *testing.T) {
	reinstall(t)
	run(t, "nc-client", "setcap", "-r", "/usr/bin/netcheck")
	t.Cleanup(func() { reinstall(t) })
	reports, code := check(t, "id: icmp\nretries: 1\nchecks:\n  - protocol: icmp\n    address: 10.77.0.2\n")
	if code != 2 || reports[0].Results[0].Err == nil {
		t.Fatalf("expected a failed check (exit 2), got exit %d: %+v", code, reports)
	}
	if !has(reports[0].Results[0].Err.Error(), "privileged") && !has(reports[0].Results[0].Err.Error(), "permit") {
		t.Errorf("the error does not point at the privilege: %v", reports[0].Results[0].Err)
	}
}

// CX-6: the package installs the manual page and the completions, readable by
// an ordinary user, and bash can load the completion and use it.
func TestPackageInstallsTheManPageAndTheCompletions(t *testing.T) {
	for _, f := range []string{
		"/usr/share/man/man1/netcheck.1.gz",
		"/usr/share/bash-completion/completions/netcheck",
		"/usr/share/zsh/site-functions/_netcheck",
		"/usr/share/fish/vendor_completions.d/netcheck.fish",
	} {
		if r := asProbe(t, "test", "-r", f); r.Code != 0 {
			t.Errorf("%s is missing or not readable by an ordinary user", f)
		}
	}
	if r := asProbe(t, "sh", "-c", "command -v man >/dev/null"); r.Code == 0 {
		if r := asProbe(t, "man", "-w", "netcheck"); r.Code != 0 || !strings.Contains(r.Stdout, "netcheck.1") {
			t.Errorf("man does not find the page: %q %q", r.Stdout, r.Stderr)
		}
	}
}

func TestBashCompletionWorksOnTheInstalledPackage(t *testing.T) {
	script := `source /usr/share/bash-completion/completions/netcheck
COMP_WORDS=(netcheck --fo); COMP_CWORD=1; _netcheck; echo "${COMPREPLY[*]}"
COMP_WORDS=(netcheck --format ""); COMP_CWORD=2; _netcheck; echo "${COMPREPLY[*]}"`
	r := asProbe(t, "bash", "-c", script)
	if r.Code != 0 || !strings.Contains(r.Stdout, "--format") || !strings.Contains(r.Stdout, "json yaml text template") {
		t.Fatalf("exit %d: %q %q", r.Code, r.Stdout, r.Stderr)
	}
}

func TestCompletionCommandOfTheInstalledBinary(t *testing.T) {
	for _, shell := range []string{"bash", "zsh", "fish", "powershell"} {
		if r := asProbe(t, "netcheck", "completion", shell); r.Code != 0 || !strings.Contains(r.Stdout, "format") {
			t.Errorf("completion %s: exit %d", shell, r.Code)
		}
	}
	if r := asProbe(t, "netcheck", "completion", "tcsh"); r.Code != 1 {
		t.Errorf("completion tcsh: exit %d, want 1", r.Code)
	}
}
