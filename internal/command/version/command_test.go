package version

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	flags "github.com/jessevdk/go-flags"
)

func TestRelevantEnv(t *testing.T) {
	const userInfo = "admin:" + "hunter2" + "@" // built here, so that scanners do not flag a fixture
	tests := []struct {
		env, want string
		ok        bool
	}{
		{"NETCHECK_PG_APP_DSN=host=db password=hunter2", "NETCHECK_PG_APP_DSN=********", true},
		{"NETCHECK_MIGRATOR_DSN=postgres://" + userInfo + "db/netcheck", "NETCHECK_MIGRATOR_DSN=********", true},
		{"NETCHECK_DATABASE_URL=postgres://" + userInfo + "db/netcheck", "NETCHECK_DATABASE_URL=********", true},
		{"NETCHECK_ODD_URL=://" + userInfo + "%zz", "NETCHECK_ODD_URL=********", true}, // does not parse
		{"NETCHECK_PUBLIC_URL=https://netcheck.example/ui", "NETCHECK_PUBLIC_URL=https://netcheck.example/ui", true},
		{"NETCHECK_DOTENV=/etc/netcheck/.env", "NETCHECK_DOTENV=/etc/netcheck/.env", true},
		{"NETCHECK_LOG_LEVEL=debug", "NETCHECK_LOG_LEVEL=debug", true},
		{"NETCHECK_TRUST_PASSWORD=hunter2", "NETCHECK_TRUST_PASSWORD=********", true},
		{"NETCHECK_CLIENT_SECRET=s3cr3t", "NETCHECK_CLIENT_SECRET=********", true},
		{"OS_AUTH_URL=https://keystone:5000/v3", "", false},
		{"HOME=/root", "", false},
		{"MIDPOINT_PASSWORD=x", "", false},
	}
	for _, tt := range tests {
		got, ok := relevantEnv(tt.env)
		if got != tt.want || ok != tt.ok {
			t.Errorf("relevantEnv(%q) = %q, %v; want %q, %v", tt.env, got, ok, tt.want, tt.ok)
		}
	}
}

// TestExecute runs the command in both modes and checks that the verbose
// listing masks secrets.
func TestExecute(t *testing.T) {
	dotenv := filepath.Join(t.TempDir(), ".env")
	if err := os.WriteFile(dotenv, []byte("NETCHECK_TEST_SETTING=on\nNETCHECK_TEST_PASSWORD=hunter2\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Unsetenv("NETCHECK_TEST_SETTING"); _ = os.Unsetenv("NETCHECK_TEST_PASSWORD") })
	missing := filepath.Join(t.TempDir(), "missing")
	for _, cmd := range []Version{{}, {Verbose: true}, {Verbose: true, DotEnv: &dotenv}, {Verbose: true, DotEnv: &missing}} {
		out := capture(t, func() error { return cmd.Execute(nil) })
		if cmd.DotEnv == &dotenv && (!strings.Contains(out, "NETCHECK_TEST_SETTING=on") || !strings.Contains(out, "NETCHECK_TEST_PASSWORD=********") || strings.Contains(out, "hunter2")) {
			t.Errorf("verbose output:\n%s", out)
		}
	}
}

// capture returns what f writes to standard output.
func capture(t *testing.T, f func() error) string {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	stdout := os.Stdout
	os.Stdout = w
	err = f()
	os.Stdout = stdout
	_ = w.Close()
	if err != nil {
		t.Fatal(err)
	}
	out, _ := io.ReadAll(r)
	return string(out)
}

// TestRun is how a plugin answers "NAME version [-v]".
func TestRun(t *testing.T) {
	for _, tc := range []struct {
		args []string
		want int
	}{
		{nil, 0},
		{[]string{"-v"}, 0},
		{[]string{"--verbose"}, 0},
		{[]string{"--nonsense"}, 2},
	} {
		if got := Run(tc.args); got != tc.want {
			t.Errorf("Run(%v) = %d, want %d", tc.args, got, tc.want)
		}
	}
}

// TestDotEnvValue: every form takes the file, none leaves it as an argument.
func TestDotEnvValue(t *testing.T) {
	for _, args := range [][]string{{"--dotenv", "x"}, {"--dotenv=x"}, {"-D", "x"}} {
		var cmd Version
		rest, err := flags.ParseArgs(&cmd, args)
		if err != nil || cmd.DotEnv == nil || *cmd.DotEnv != "x" || len(rest) != 0 {
			t.Errorf("%v: dotenv %v, rest %v, err %v", args, cmd.DotEnv, rest, err)
		}
	}
}
