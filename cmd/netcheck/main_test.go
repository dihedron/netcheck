package main

import (
	"net"
	"os"
	"path/filepath"
	"testing"
)

// run executes the command line args (without the program name), as main does.
func runArgs(t *testing.T, args ...string) int {
	t.Helper()
	saved := os.Args
	t.Cleanup(func() { os.Args = saved })
	os.Args = append([]string{"netcheck"}, args...)
	return run()
}

func TestRunVersion(t *testing.T) {
	for _, args := range [][]string{{"version"}, {"--version"}, {"version", "--verbose"}, {"version", "-v"}} {
		if code := runArgs(t, args...); code != 0 {
			t.Errorf("%v: exit code %d, want 0", args, code)
		}
	}
}

func TestRunUsageErrors(t *testing.T) {
	for _, args := range [][]string{
		{"--format=bogus"},
		{"--bogus-flag"},
		{"--format=template"},                     // no template given
		{"--template=" + t.TempDir()},             // a directory, not a file
		{"--template=" + t.TempDir() + "/absent"}, // no such file
		{"/nonexistent/bundle.yaml"},              // bundle that cannot be loaded
	} {
		if code := runArgs(t, args...); code != 1 {
			t.Errorf("%v: exit code %d, want 1", args, code)
		}
	}
}

// With no bundle the command runs on mock data, to develop templates.
func TestRunTemplateOnMockData(t *testing.T) {
	tpl := filepath.Join("..", "..", "test", "fixtures", "output.tpl")
	if code := runArgs(t, "--template="+tpl, "--print-diagnostics"); code != 0 {
		t.Fatalf("exit code %d, want 0", code)
	}
}

// A local bundle with one check against a listener of ours passes in every
// output format.
func TestRunLocalBundle(t *testing.T) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = l.Close() }()
	bundle := filepath.Join(t.TempDir(), "bundle.yaml")
	body := "---\nid: local\nretries: 1\ntimeout: 2s\nchecks:\n  - address: " + l.Addr().String() + "\n    protocol: tcp\n"
	if err := os.WriteFile(bundle, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, format := range []string{"text", "json", "yaml"} {
		if code := runArgs(t, "--format="+format, bundle); code != 0 {
			t.Errorf("format %s: exit code %d, want 0", format, code)
		}
	}
}

func TestGetHostnamePort(t *testing.T) {
	for _, tc := range []struct{ in, host, port string }{
		{"example.com:443", "example.com", "443"},
		{"example.com", "example.com", ""},
		{"www.example.com/path?x=1", "www.example.com", ""},
		{"example.com:8080/path", "example.com", "8080"},
	} {
		if h, p := getHostnamePort(tc.in); h != tc.host || p != tc.port {
			t.Errorf("%q: got %q, %q; want %q, %q", tc.in, h, p, tc.host, tc.port)
		}
	}
}

func TestIsFile(t *testing.T) {
	dir := t.TempDir()
	f := filepath.Join(dir, "f")
	if err := os.WriteFile(f, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	if !isFile(f) || isFile(dir) || isFile(dir+"/absent") {
		t.Error("isFile wrong")
	}
}

func TestCleanupProfiles(t *testing.T) {
	dir := t.TempDir()
	var err error
	if cpuprof, err = os.Create(dir + "/cpu"); err != nil {
		t.Fatal(err)
	}
	if memprof, err = os.Create(dir + "/mem"); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { cpuprof, memprof = nil, nil })
	cleanup()
	if info, err := os.Stat(dir + "/mem"); err != nil || info.Size() == 0 {
		t.Errorf("memory profile not written: %v", err)
	}
}
