package checks

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// withDefaults restores the global defaults when the test ends.
func withDefaults(t *testing.T) {
	t.Helper()
	saved := Default
	t.Cleanup(func() { Default = saved })
}

func TestLoadDefaultsFillsMissingValues(t *testing.T) {
	withDefaults(t)
	path := filepath.Join(t.TempDir(), "netcheck.conf")
	if err := os.WriteFile(path, []byte("timeout: 5s\nping:\n  count: 3\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := loadDefaultsFrom(path); err != nil {
		t.Fatal(err)
	}
	if *Default.Timeout != Timeout(5*time.Second) || *Default.Ping.Count != 3 {
		t.Errorf("values of the file not used: %v %v", *Default.Timeout, *Default.Ping.Count)
	}
	if *Default.Retries != DefaultRetries || *Default.Wait != DefaultWait || *Default.Concurrency != DefaultConcurrency ||
		*Default.Ping.Interval != DefaultPingInterval || *Default.Ping.Size != DefaultPingSize {
		t.Error("missing values must take the compiled-in defaults")
	}
}

func TestLoadDefaultsErrors(t *testing.T) {
	withDefaults(t)
	dir := t.TempDir()
	if err := loadDefaultsFrom(filepath.Join(dir, "absent.conf")); err == nil {
		t.Error("a missing file must be an error (the caller tries the next path)")
	}
	bad := filepath.Join(dir, "bad.conf")
	if err := os.WriteFile(bad, []byte("timeout: [not, a, duration]\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := loadDefaultsFrom(bad); err == nil {
		t.Error("an invalid file must be an error")
	}
}
