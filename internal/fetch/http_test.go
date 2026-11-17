package fetch

import (
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dihedron/netcheck/v2/internal/format"
)

// fixtures serves the bundles in test/fixtures with the Content-Type the
// fetcher uses to tell YAML from JSON.
func fixtures() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/{path}", func(w http.ResponseWriter, r *http.Request) {
		path := filepath.Join("..", "..", "test", "fixtures", filepath.Base(r.PathValue("path")))
		switch {
		case strings.HasSuffix(path, ".yaml"):
			w.Header().Set("Content-Type", "application/x-yaml")
		case strings.HasSuffix(path, ".json"):
			w.Header().Set("Content-Type", "application/json")
		}
		http.ServeFile(w, r, path)
	})
	return mux
}

var bundleFormats = map[string]format.Format{
	"netcheck.json": format.JSON,
	"netcheck.yaml": format.YAML,
}

func TestFromHTTP(t *testing.T) {
	server := httptest.NewServer(fixtures())
	defer server.Close()

	for name, expected := range bundleFormats {
		_, actual, err := FromHTTP(server.URL + "/" + name)
		if err != nil {
			t.Fatalf("could not download %s: %v", name, err)
		}
		if expected != actual {
			t.Fatalf("%s: expected format %v, got %v", name, expected, actual)
		}
	}
}

// The test server's certificate is signed by a CA the client does not trust
// (httptest generates it in memory, nothing on disk), so verification fails.
func TestFromHTTPsRejectsUntrustedCertificate(t *testing.T) {
	server := httptest.NewTLSServer(fixtures())
	defer server.Close()

	for name := range bundleFormats {
		if _, _, err := FromHTTP(server.URL + "/" + name); err == nil {
			t.Fatalf("expected a TLS verification error downloading %s, got none", name)
		}
	}
}

// The https-:// scheme skips the certificate verification.
func TestFromHTTPsSkipTLS(t *testing.T) {
	server := httptest.NewTLSServer(fixtures())
	defer server.Close()

	for name, expected := range bundleFormats {
		_, actual, err := FromHTTP(strings.Replace(server.URL, "https://", "https-://", 1) + "/" + name)
		if err != nil {
			t.Fatalf("could not download %s: %v", name, err)
		}
		if expected != actual {
			t.Fatalf("%s: expected format %v, got %v", name, expected, actual)
		}
	}
}
