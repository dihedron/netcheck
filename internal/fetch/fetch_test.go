package fetch

import (
	"errors"
	"io/fs"
	"testing"
)

func TestFromFileErrors(t *testing.T) {
	if _, _, err := FromFile(t.TempDir() + "/absent.yaml"); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("want a not-exist error, got %v", err)
	}
}

// Required query parameters and numbers are checked before any connection is
// made (nothing listens on port 1).
func TestLocationsAreValidated(t *testing.T) {
	fetchers := map[string]func(string) error{
		"redis":     func(p string) error { _, _, err := FromRedis(p); return err },
		"consul KV": func(p string) error { _, _, err := FromConsulKV(p); return err },
		"consul SR": func(p string) error { _, _, err := FromConsulSR(p); return err },
	}
	for _, tc := range []struct{ fetcher, location string }{
		{"redis", "redis://127.0.0.1:1"},
		{"redis", "redis://127.0.0.1:1?key=k&db=notanumber"},
		{"consul KV", "consulkv://127.0.0.1:1"},
		{"consul SR", "consulsr://127.0.0.1:1"},
		{"consul SR", "consulsr://127.0.0.1:1?service=s"},
		{"consul SR", "consulsr://127.0.0.1:1?service=s&tag=t"},
	} {
		if err := fetchers[tc.fetcher](tc.location); err == nil {
			t.Errorf("%s %s: expected an error", tc.fetcher, tc.location)
		}
	}
}
