package main

import (
	"encoding/json"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/dihedron/netcheck/v2/pkg/metadata"
	"github.com/dihedron/netcheck/v2/pkg/netcheck"
	"github.com/jessevdk/go-flags"
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
		{"--allow-net=nonsense", "/nonexistent"},  // not a network
		{"--deny-net=10.0.0.0/33", "/nonexistent"},
		{"redis://127.0.0.1:1"}, // a bundle source that does not answer
		{},                      // no bundle and no template
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

// bundleFile writes a bundle with the given checks (YAML lines) and returns its path.
func bundleFile(t *testing.T, checks string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "bundle.yaml")
	body := "---\nid: local\nretries: 1\ntimeout: 2s\nchecks:\n" + checks
	if err := os.WriteFile(p, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	return p
}

// NC-18: exit 0 when every check passed, 2 when at least one failed, in every format.
func TestRunExitCodes(t *testing.T) {
	up := "  - address: " + mustListen(t) + "\n    protocol: tcp\n"
	l, _ := net.Listen("tcp", "127.0.0.1:0")
	down := "  - address: " + l.Addr().String() + "\n    protocol: tcp\n"
	_ = l.Close()
	tpl := filepath.Join("..", "..", "test", "fixtures", "output.tpl")
	for _, format := range []string{"--format=text", "--format=json", "--format=yaml", "--template=" + tpl} {
		if code := runArgs(t, format, bundleFile(t, up)); code != 0 {
			t.Errorf("%s all passed: exit code %d, want 0", format, code)
		}
		if code := runArgs(t, format, bundleFile(t, up+down)); code != 2 {
			t.Errorf("%s one failed: exit code %d, want 2", format, code)
		}
	}
	// several bundles: one failure anywhere is a failure
	if code := runArgs(t, bundleFile(t, up), bundleFile(t, down)); code != 2 {
		t.Errorf("two bundles: exit code %d, want 2", code)
	}
	// a bundle the library refuses is a usage error, not a failed check
	if code := runArgs(t, bundleFile(t, "  - address: nowhere\n    protocol: tcp\n")); code != 1 {
		t.Errorf("invalid bundle: exit code %d, want 1", code)
	}
}

// The JSON output is the list of reports, with the result of every check.
func TestRunJSONOutput(t *testing.T) {
	up := mustListen(t)
	var code int
	out, _ := capture(t, func() { code = runArgs(t, "--format=json", bundleFile(t, "  - address: "+up+"\n    protocol: tcp\n")) })
	var reports []netcheck.Report
	if err := json.Unmarshal([]byte(out), &reports); err != nil || code != 0 || len(reports) != 1 {
		t.Fatalf("exit %d: %v\n%s", code, err, out)
	}
	if r := reports[0]; r.BundleID != "local" || r.Results[0].Status != netcheck.Passed || r.Results[0].Detail.Remote != up {
		t.Errorf("%+v", r)
	}
}

// The network limits reach the checks.
func TestRunNetworkLimits(t *testing.T) {
	file := bundleFile(t, "  - address: "+mustListen(t)+"\n    protocol: tcp\n")
	if code := runArgs(t, "--deny-net=127.0.0.0/8", file); code != 2 {
		t.Errorf("denied: exit code %d, want 2", code)
	}
	if code := runArgs(t, "--allow-net=127.0.0.1", file); code != 0 {
		t.Errorf("allowed: exit code %d, want 0", code)
	}
}

// Ctrl-C ends the run: what had not finished is reported skipped, and the exit code is 1.
func TestRunInterrupted(t *testing.T) {
	release := make(chan struct{})
	slow := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { <-release }))
	defer slow.Close()
	defer close(release)
	file := bundleFile(t, "  - address: "+strings.TrimPrefix(slow.URL, "http://")+"\n    protocol: http\n    timeout: 30s\n")
	go func() {
		time.Sleep(300 * time.Millisecond)
		_ = syscall.Kill(os.Getpid(), syscall.SIGINT)
	}()
	var code int
	start := time.Now()
	out, _ := capture(t, func() { code = runArgs(t, "--format=json", file) })
	if code != 1 || time.Since(start) > 10*time.Second {
		t.Fatalf("exit code %d after %v", code, time.Since(start))
	}
	if !strings.Contains(out, `"status": "skipped"`) {
		t.Errorf("the unfinished check must be skipped:\n%s", out)
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
		{"[2001:db8::1]:443", "2001:db8::1", "443"},
		{"[2001:db8::1]/x", "2001:db8::1", ""},
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

// A bundle server that accepts and never answers must not hang the command: the
// lab found that it did (T-7). The fetch has a deadline, --fetch-timeout.
func TestRunFetchTimesOut(t *testing.T) {
	release := make(chan struct{})
	hang := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { <-release }))
	defer hang.Close()
	defer close(release)
	var code int
	start := time.Now()
	_, stderr := capture(t, func() { code = runArgs(t, "--fetch-timeout=300ms", "--format=json", hang.URL+"/bundle.yaml") })
	if code != 1 || time.Since(start) > 5*time.Second {
		t.Fatalf("exit code %d after %v, want 1 within the timeout", code, time.Since(start))
	}
	if !strings.Contains(stderr, "deadline exceeded") {
		t.Errorf("the error does not say that the fetch timed out:\n%s", stderr)
	}
}

func TestRunRejectsANegativeFetchTimeout(t *testing.T) {
	var code int
	_, stderr := capture(t, func() { code = runArgs(t, "--fetch-timeout=-1s", "x.yaml") })
	if code != 1 || !strings.Contains(stderr, "negative") {
		t.Errorf("exit code %d, stderr %q: want 1 and a message about the negative value", code, stderr)
	}
}

// CX-2: "completion" is answered before the checks' command line is parsed.
func TestRunCompletion(t *testing.T) {
	if code := runArgs(t, "completion", "bash"); code != 0 {
		t.Errorf("completion bash: exit code %d, want 0", code)
	}
	if code := runArgs(t, "completion", "tcsh"); code != 1 {
		t.Errorf("completion tcsh: exit code %d, want 1", code)
	}
}

// The options carry the descriptions the man page and the completions print.
func TestEveryOptionIsDescribed(t *testing.T) {
	for _, o := range flags.NewParser(&cliOptions{}, flags.Default).Group.Options() {
		if o.Description == "" {
			t.Errorf("--%s has no description", o.LongName)
		}
	}
}

// CX-4, T-K4: the man page documents every option the parser has, the
// commands and the exit codes, and its roff is the one its Markdown builds.
func TestManPageMatchesTheCommand(t *testing.T) {
	md, err := os.ReadFile(filepath.Join("..", "..", "docs", "man", "netcheck.1.md"))
	if err != nil {
		t.Fatal(err)
	}
	roff, err := os.ReadFile(filepath.Join("..", "..", "docs", "man", "netcheck.1"))
	if err != nil {
		t.Fatal(err)
	}
	page := strings.ReplaceAll(string(md), `\--`, "--")
	parser := flags.NewParser(&cliOptions{}, flags.Default)
	for _, o := range parser.Group.Options() {
		if !strings.Contains(page, "**--"+o.LongName+"**") {
			t.Errorf("the man page does not document --%s", o.LongName)
		}
		if o.ShortName != 0 && !strings.Contains(page, "**-"+string(o.ShortName)+"**") {
			t.Errorf("the man page does not document -%c", o.ShortName)
		}
		if !strings.Contains(string(roff), `\-\-`+strings.ReplaceAll(o.LongName, "-", `\-`)) {
			t.Errorf("netcheck.1 is stale: --%s is missing (make man)", o.LongName)
		}
	}
	for _, want := range []string{"**version**", "**completion**", "# EXIT STATUS", "**0**", "**1**", "**2**", "**--version**"} {
		if !strings.Contains(page, want) {
			t.Errorf("the man page lacks %q", want)
		}
	}
	for _, name := range []string{"NETCHECK_LOG_LEVEL", "NETCHECK_LOG_STREAM", "NETCHECK_DOTENV"} {
		if !strings.Contains(page, name) {
			t.Errorf("the man page lacks %s", name)
		}
	}
	for _, p := range defaultsPaths {
		if !strings.Contains(page, p) {
			t.Errorf("the man page lacks the defaults file %s", p)
		}
	}
}

// CX-4: groff renders the page without a warning.
func TestManPageRenders(t *testing.T) {
	groff, err := exec.LookPath("groff")
	if err != nil {
		t.Skip("groff is not installed")
	}
	out, err := exec.Command(groff, "-man", "-ww", "-z", filepath.Join("..", "..", "docs", "man", "netcheck.1")).CombinedOutput()
	if err != nil || len(out) != 0 {
		t.Errorf("groff: %v: %s", err, out)
	}
}
