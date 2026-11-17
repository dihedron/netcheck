package checks

import (
	"errors"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
)

const sampleBundle = `---
id: sample
description: a sample bundle
checks:
  - address: localhost:80
    protocol: tcp
`

// TestNewFromHTTP is the regression test for the reversed strings.HasPrefix
// arguments in New (D-1): an http:// location was treated as a file name.
func TestNewFromHTTP(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/yaml")
		_, _ = w.Write([]byte(sampleBundle))
	}))
	defer server.Close()

	bundle, err := New(server.URL + "/bundle.yaml")
	if err != nil {
		t.Fatalf("New(%s): %v", server.URL, err)
	}
	if bundle.ID != "sample" || len(bundle.Checks) != 1 {
		t.Fatalf("unexpected bundle: %+v", bundle)
	}
}

// TestNewDispatchesRedis checks that a redis:// location reaches the Redis
// fetcher (D-1): it must fail on the missing key, never as a missing file.
func TestNewDispatchesRedis(t *testing.T) {
	_, err := New("redis://127.0.0.1:1")
	if err == nil {
		t.Fatal("expected an error")
	}
	if errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("redis:// was read as a file: %v", err)
	}
}

func TestNewFromFile(t *testing.T) {
	path := t.TempDir() + "/bundle.yaml"
	if err := os.WriteFile(path, []byte(sampleBundle), 0o600); err != nil {
		t.Fatal(err)
	}
	bundle, err := New(path)
	if err != nil {
		t.Fatal(err)
	}
	if bundle.Timeout != *Default.Timeout {
		t.Fatalf("timeout %v: the bundle must inherit the default", bundle.Timeout)
	}
}

func TestNewMissingFile(t *testing.T) {
	if _, err := New(t.TempDir() + "/absent.yaml"); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("want a not-exist error, got %v", err)
	}
}
