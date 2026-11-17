package main

import (
	"github.com/dihedron/netcheck/v2/pkg/metadata"
	"log/slog"
	"net"
	"os"
	"path/filepath"
	"strings"
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

// mustListen returns the address of a TCP listener that lives until the test ends.
func mustListen(t *testing.T) string {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = l.Close() })
	return l.Addr().String()
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

// setupWith runs setup with the given environment and restores the logger and
// the profile state when the test ends.
func setupWith(t *testing.T, env map[string]string) {
	t.Helper()
	for k, v := range env {
		t.Setenv(k, v)
	}
	saved := slog.Default()
	t.Cleanup(func() { slog.SetDefault(saved); cpuprof, memprof = nil, nil })
	setup()
}

func TestSetupLevelsAndStreams(t *testing.T) {
	t.Chdir(t.TempDir()) // the "file" stream writes netcheck-<pid>.log in the working directory
	name := "NETCHECK_"
	if !strings.EqualFold(filepath.Base(os.Args[0]), "netcheck") {
		name = strings.ToUpper(strings.ReplaceAll(filepath.Base(os.Args[0]), "-", "_")) + "_"
	}
	for _, level := range []string{"debug", "info", "warn", "error", "off", "trace", "fatal", "none", "bogus"} {
		for _, stream := range []string{"stderr", "stdout", "file", "other"} {
			setupWith(t, map[string]string{name + "LOG_LEVEL": level, name + "LOG_STREAM": stream})
		}
	}
	if files, _ := filepath.Glob("*.log"); len(files) == 0 {
		t.Error("the file stream must create a log file")
	}
}

func TestSetupProfiles(t *testing.T) {
	dir := t.TempDir()
	name := strings.ToUpper(strings.ReplaceAll(filepath.Base(os.Args[0]), "-", "_")) + "_"
	setupWith(t, map[string]string{name + "CPU_PROFILE": filepath.Join(dir, "cpu"), name + "MEM_PROFILE": filepath.Join(dir, "mem")})
	if cpuprof == nil || memprof == nil {
		t.Fatal("the profile files must be open")
	}
	cleanup()
	for _, f := range []string{"cpu", "mem"} {
		if _, err := os.Stat(filepath.Join(dir, f)); err != nil {
			t.Error(err)
		}
	}
	// a profile that cannot be created is reported, not fatal
	cpuprof, memprof = nil, nil
	setupWith(t, map[string]string{name + "CPU_PROFILE": dir + "/absent/cpu", name + "MEM_PROFILE": dir + "/absent/mem"})
	if cpuprof != nil || memprof != nil {
		t.Error("no profile file can be open")
	}
}

func TestSetupDotEnv(t *testing.T) {
	dotenv := filepath.Join(t.TempDir(), ".env")
	if err := os.WriteFile(dotenv, []byte("NETCHECK_TEST_FROM_DOTENV=1\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Unsetenv("NETCHECK_TEST_FROM_DOTENV") })
	saved := metadata.DotEnvVarName
	metadata.DotEnvVarName = "NETCHECK_TEST_DOTENV" // stamped at build time, empty in tests
	t.Cleanup(func() { metadata.DotEnvVarName = saved })
	setupWith(t, map[string]string{"NETCHECK_TEST_DOTENV": dotenv})
	if os.Getenv("NETCHECK_TEST_FROM_DOTENV") != "1" {
		t.Error("the .env file must be loaded")
	}
}
