package metadata

import (
	"bytes"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// PrintFull writes to its writer, not to the process's standard output.
func TestPrintFullUsesWriter(t *testing.T) {
	var out bytes.Buffer
	PrintFull(&out)
	if !strings.Contains(out.String(), "Version") {
		t.Fatalf("nothing written: %q", out.String())
	}
	if n := strings.Count(out.String(), "Git Time"); n != 1 {
		t.Errorf("Git Time printed %d times", n)
	}
}

func TestLoadDotEnv(t *testing.T) {
	dir := t.TempDir()
	if err := LoadDotEnv(filepath.Join(dir, "missing")); err == nil || !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("missing file: got %v", err)
	}
	bad := filepath.Join(dir, "bad")
	if err := os.WriteFile(bad, []byte("bad line\nX="+"canary\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := LoadDotEnv(bad); err == nil || strings.Contains(err.Error(), "canary") {
		t.Errorf("malformed file: got %v", err)
	}
	good := filepath.Join(dir, "good")
	if err := os.WriteFile(good, []byte("NETCHECK_TEST_DOTENV_GOOD=1\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Unsetenv("NETCHECK_TEST_DOTENV_GOOD") })
	if err := LoadDotEnv(good); err != nil || os.Getenv("NETCHECK_TEST_DOTENV_GOOD") != "1" {
		t.Errorf("good file: %v", err)
	}
}
