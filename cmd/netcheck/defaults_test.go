package main

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/dihedron/netcheck/v2/pkg/netcheck"
)

func writeConf(t *testing.T, body string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "netcheck.conf")
	if err := os.WriteFile(p, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestLoadDefaultsWithoutAFile(t *testing.T) {
	d, err := loadDefaults([]string{filepath.Join(t.TempDir(), "absent")})
	if err != nil {
		t.Fatal(err)
	}
	want := netcheck.DefaultDefaults()
	want.Ping.Privileged = runtime.GOOS == "linux" || runtime.GOOS == "windows"
	if d != want {
		t.Errorf("%+v, want %+v", d, want)
	}
}

func TestLoadDefaultsFromTheFirstExistingFile(t *testing.T) {
	first := writeConf(t, "---\ntimeout: 5s\nretries: 7\nwait: 50ms\nconcurrency: 3\nping:\n  count: 2\n  interval: 10ms\n  size: 32\n  privileged: false\n")
	second := writeConf(t, "---\nretries: 99\n")
	d, err := loadDefaults([]string{filepath.Join(t.TempDir(), "absent"), first, second})
	if err != nil {
		t.Fatal(err)
	}
	if d.Timeout != netcheck.Duration(5*time.Second) || d.Retries != 7 || d.Wait != netcheck.Duration(50*time.Millisecond) || d.Concurrency != 3 ||
		d.Ping.Count != 2 || d.Ping.Interval != netcheck.Duration(10*time.Millisecond) || d.Ping.Size != 32 || d.Ping.Privileged {
		t.Errorf("%+v", d)
	}
	if d.UDPGrace != netcheck.DefaultDefaults().UDPGrace {
		t.Error("what the file does not say keeps its default")
	}
}

// A file that exists and is wrong is an error, not a reason to try the next one.
func TestLoadDefaultsRejectsABrokenFile(t *testing.T) {
	for _, body := range []string{"timeout: [", "timeout: soon\n", "bogus: 1\n"} {
		if _, err := loadDefaults([]string{writeConf(t, body), writeConf(t, "retries: 2\n")}); err == nil {
			t.Errorf("%q: expected an error", body)
		}
	}
}

func TestExpandHome(t *testing.T) {
	t.Setenv("HOME", "/home/test")
	if got := expandHome("~/netcheck.conf"); got != "/home/test/netcheck.conf" {
		t.Errorf("%q", got)
	}
	if got := expandHome("/etc/netcheck.conf"); got != "/etc/netcheck.conf" {
		t.Errorf("%q", got)
	}
}

// The sample defaults file of the package loads, and says what the guide says.
func TestExampleConfLoads(t *testing.T) {
	d, err := loadDefaults([]string{filepath.Join("..", "..", "examples", "netcheck.conf")})
	if err != nil {
		t.Fatal(err)
	}
	if d.Timeout != netcheck.Duration(2*time.Second) || d.Retries != 3 || d.Concurrency != 10 {
		t.Errorf("%+v", d)
	}
}
