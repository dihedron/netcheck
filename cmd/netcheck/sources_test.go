package main

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSplitScheme(t *testing.T) {
	for _, tc := range []struct {
		in       string
		location string
		insecure bool
		sso      bool
	}{
		{"https://h/b.yaml", "https://h/b.yaml", false, false},
		{"https-://h/b.yaml", "https://h/b.yaml", true, false},
		{"http+sso://h/b", "http://h/b", false, true},
		{"https+sso://h/b", "https://h/b", false, true},
		{"https+sso-://h/b", "https://h/b", true, true},
		{"rediss-://h:1?key=k", "rediss://h:1?key=k", true, false},
		{"consulkvs-://:tok@h?key=k", "consulkvs://:tok@h?key=k", true, false},
		{"consulsrs-://h?service=s", "consulsrs://h?service=s", true, false},
		{"/etc/bundle.yaml", "/etc/bundle.yaml", false, false},
		{"C:/bundle.yaml", "C:/bundle.yaml", false, false},
	} {
		location, insecure, sso := splitScheme(tc.in)
		if location != tc.location || insecure != tc.insecure || sso != tc.sso {
			t.Errorf("%s: %q %v %v", tc.in, location, insecure, sso)
		}
	}
}

func TestLoadBundleFromAFile(t *testing.T) {
	p := filepath.Join(t.TempDir(), "b.yaml")
	if err := os.WriteFile(p, []byte("---\nid: f\nchecks:\n  - address: h:1\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if b, err := loadBundle(t.Context(), p); err != nil || b.ID != "f" {
		t.Fatalf("%v", err)
	}
}

// https-:// skips the certificate verification; https:// does not.
func TestLoadBundleOverHTTPS(t *testing.T) {
	s := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/x-yaml")
		_, _ = w.Write([]byte("---\nid: secure\nchecks:\n  - address: h:1\n"))
	}))
	defer s.Close()
	if _, err := loadBundle(t.Context(), s.URL+"/b"); err == nil {
		t.Error("an untrusted certificate must fail")
	}
	if b, err := loadBundle(t.Context(), strings.Replace(s.URL, "https://", "https-://", 1)+"/b"); err != nil || b.ID != "secure" {
		t.Errorf("https-://: %v", err)
	}
}

// Every source the command always had is registered: a bad Redis or Consul
// location fails on its own terms, not as "no source registered".
func TestEverySchemeIsRegistered(t *testing.T) {
	for _, loc := range []string{
		"redis://127.0.0.1:1", "rediss://127.0.0.1:1", "rediss-://127.0.0.1:1",
		"consulkv://127.0.0.1:1", "consulkvs://127.0.0.1:1", "consulkvs-://127.0.0.1:1",
		"consulsr://127.0.0.1:1", "consulsrs://127.0.0.1:1", "consulsrs-://127.0.0.1:1",
		"http+sso://127.0.0.1:1/b", "https+sso://127.0.0.1:1/b", "https+sso-://127.0.0.1:1/b",
	} {
		_, err := loadBundle(t.Context(), loc)
		if err == nil || strings.Contains(err.Error(), "no source registered") {
			t.Errorf("%s: %v", loc, err)
		}
	}
}

func TestParseNetworks(t *testing.T) {
	if p, err := parseNetworks([]string{"10.0.0.0/8", "2001:db8::/32", "192.0.2.1"}); err != nil || len(p) != 3 {
		t.Fatalf("%v %v", p, err)
	}
	for _, bad := range []string{"nonsense", "10.0.0.0/33", ""} {
		if _, err := parseNetworks([]string{bad}); err == nil {
			t.Errorf("%q: expected an error", bad)
		}
	}
}
