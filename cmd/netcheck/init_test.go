package main

import (
	"bytes"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// built returns a netcheck binary: init() reads variables named after the
// binary, and DotEnvVarName is stamped at build time.
func built(t *testing.T) string {
	t.Helper()
	bin := filepath.Join(t.TempDir(), "netcheck")
	cmd := exec.Command("go", "build", "-ldflags", "-X github.com/dihedron/netcheck/v2/pkg/metadata.DotEnvVarName=NETCHECK_DOTENV", "-o", bin, ".")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("build: %v\n%s", err, out)
	}
	return bin
}

func runVersion(t *testing.T, bin string, env ...string) (code int, stderr string) {
	t.Helper()
	cmd := exec.Command(bin, "version")
	cmd.Env = append([]string{"HOME=" + t.TempDir()}, env...)
	var errb bytes.Buffer
	cmd.Stderr = &errb
	err := cmd.Run()
	var ee *exec.ExitError
	if errors.As(err, &ee) {
		code = ee.ExitCode()
	} else if err != nil {
		t.Fatal(err)
	}
	return code, errb.String()
}

// TestLogLevels: every documented level is accepted and the command still runs.
func TestLogLevels(t *testing.T) {
	bin := built(t)
	for _, v := range []string{"debug", "info", "warn", "error", "off", "bogus"} {
		if code, stderr := runVersion(t, bin, "NETCHECK_LOG_LEVEL="+v); code != 0 {
			t.Errorf("%s: exit %d: %s", v, code, stderr)
		}
	}
}

// TestDotEnvLogging: its lines follow the configured level.
func TestDotEnvLogging(t *testing.T) {
	bin := built(t)
	good := filepath.Join(t.TempDir(), ".env")
	if err := os.WriteFile(good, []byte("X=1\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	const loaded, failed = "successfully loaded", "error loading"
	missing := filepath.Join(t.TempDir(), "missing")
	for _, tc := range []struct {
		name, file, level, want string
		silent                  bool
	}{
		{name: "off, good", file: good, level: "off", silent: true},
		{name: "off, missing", file: missing, level: "off", silent: true},
		{name: "unset, good", file: good, silent: true},
		{name: "error, good", file: good, level: "error", silent: true},
		{name: "info, good", file: good, level: "info", want: loaded},
		{name: "error, missing", file: missing, level: "error", want: failed},
		{name: "unset, missing", file: missing, want: failed}, // a failure is never lost by default
	} {
		t.Run(tc.name, func(t *testing.T) {
			env := []string{"NETCHECK_DOTENV=" + tc.file}
			if tc.level != "" {
				env = append(env, "NETCHECK_LOG_LEVEL="+tc.level)
			}
			code, stderr := runVersion(t, bin, env...)
			if code != 0 {
				t.Fatalf("exit %d: %s", code, stderr)
			}
			if tc.silent && stderr != "" {
				t.Errorf("stderr: %s", stderr)
			}
			if !strings.Contains(stderr, tc.want) {
				t.Errorf("stderr %q does not contain %q", stderr, tc.want)
			}
		})
	}
}

// TestMalformedDotEnvIsNotQuoted: godotenv's parse error quotes the file from
// the bad line on, and later lines may hold secrets.
func TestMalformedDotEnvIsNotQuoted(t *testing.T) {
	bin := built(t)
	const canary, badLine = "canary-value", "bad line here"
	bad := filepath.Join(t.TempDir(), ".env")
	if err := os.WriteFile(bad, []byte(badLine+"\nNETCHECK_REDIS_PASSWORD="+canary+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name string
		args []string
		env  []string
	}{
		{"init, no level", []string{"version"}, []string{"NETCHECK_DOTENV=" + bad}},
		{"init, level error", []string{"version"}, []string{"NETCHECK_DOTENV=" + bad, "NETCHECK_LOG_LEVEL=error"}},
		{"version -v, no level", []string{"version", "-v", "--dotenv", bad}, nil},
		{"version -v, level error", []string{"version", "-v", "--dotenv", bad}, []string{"NETCHECK_LOG_LEVEL=error"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cmd := exec.Command(bin, tc.args...)
			cmd.Env = append([]string{"HOME=" + t.TempDir()}, tc.env...)
			out, _ := cmd.CombinedOutput()
			if strings.Contains(string(out), canary) || strings.Contains(string(out), badLine) {
				t.Errorf("the file is quoted: %s", out)
			}
		})
	}
	// the failure itself is still reported
	cmd := exec.Command(bin, "version")
	cmd.Env = []string{"HOME=" + t.TempDir(), "NETCHECK_DOTENV=" + bad}
	if out, _ := cmd.CombinedOutput(); !strings.Contains(string(out), "malformed") {
		t.Errorf("no report of the failure: %s", out)
	}
}
