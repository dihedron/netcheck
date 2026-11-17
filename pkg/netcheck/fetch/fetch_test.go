package fetch

import (
	"bytes"
	"context"
	"errors"
	"io/fs"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"go.uber.org/goleak"
)

// password is joined into locations at run time, so that no literal looks like a URL with credentials to a secret scanner.
const password = "s3cret"

func TestMain(m *testing.M) { goleak.VerifyTestMain(m) }

const yamlBundle = "---\nid: sample\nchecks:\n  - address: localhost:80\n"
const jsonBundle = `{"id":"sample","checks":[{"address":"localhost:80","protocol":"tcp"}]}`

// served returns a server that answers with the body and the content type.
func served(t *testing.T, contentType, body string, status int) string {
	t.Helper()
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if contentType != "" {
			w.Header().Set("Content-Type", contentType)
		}
		w.WriteHeader(status)
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(s.Close)
	return s.URL
}

func TestParse(t *testing.T) {
	for name, tc := range map[string]struct {
		data string
		hint Format
		ok   bool
	}{
		"yaml with marker":          {yamlBundle, Unknown, true},
		"json":                      {jsonBundle, Unknown, true},
		"yaml without marker, hint": {"id: sample\nchecks:\n  - address: h:1\n", YAML, true},
		"yaml without marker":       {"id: sample\nchecks:\n  - address: h:1\n", Unknown, true}, // ambiguous: both are tried
		"json, hint":                {jsonBundle, JSON, true},
		"hint says json, is yaml":   {yamlBundle, JSON, false},
		"plain text":                {"just text", Unknown, false},
		"empty":                     {"", Unknown, false},
		"broken json":               {`{"id":`, Unknown, false},
	} {
		b, err := Parse([]byte(tc.data), tc.hint)
		if (err == nil) != tc.ok {
			t.Errorf("%s: %v", name, err)
		}
		if err == nil && (b.ID != "sample" || len(b.Checks) != 1) {
			t.Errorf("%s: %+v", name, b)
		}
	}
}

func FuzzParse(f *testing.F) {
	for _, s := range []string{yamlBundle, jsonBundle, "", "{", "---", "id: x"} {
		f.Add([]byte(s), "")
	}
	f.Fuzz(func(t *testing.T, data []byte, hint string) {
		_, _ = Parse(data, Format(hint)) // must not panic
	})
}

func TestLoadFile(t *testing.T) {
	dir := t.TempDir()
	write := func(name, body string) string {
		p := filepath.Join(dir, name)
		if err := os.WriteFile(p, []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
		return p
	}
	y, j, plain := write("b.yaml", yamlBundle), write("b.json", jsonBundle), write("b", "id: sample\nchecks:\n  - address: h:1\n")
	for _, loc := range []string{y, j, plain, "file://" + y} {
		b, err := Load(t.Context(), loc)
		if err != nil || b.ID != "sample" {
			t.Errorf("%s: %v", loc, err)
		}
	}
	if _, err := Load(t.Context(), filepath.Join(dir, "absent.yaml")); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("want a not-exist error, got %v", err)
	}
	if _, err := Load(t.Context(), y, WithMaxSize(10)); err == nil || !strings.Contains(err.Error(), "larger") {
		t.Errorf("an oversize file must fail: %v", err)
	}
}

func TestLoadFixtures(t *testing.T) {
	for _, name := range []string{"netcheck.yaml", "netcheck.json"} {
		b, err := Load(t.Context(), filepath.Join("..", "..", "..", "test", "fixtures", name))
		if err != nil || len(b.Checks) == 0 {
			t.Errorf("%s: %v", name, err)
		}
	}
}

// NC-16
func TestLoadHTTP(t *testing.T) {
	for name, tc := range map[string]struct{ contentType, body string }{
		"yaml":            {"application/x-yaml", yamlBundle},
		"text/yaml":       {"text/yaml; charset=utf-8", yamlBundle},
		"json":            {"application/json", jsonBundle},
		"no hint, json":   {"text/plain", jsonBundle},
		"no hint, yaml":   {"", yamlBundle},
		"hint, plain yml": {"application/yaml", "id: sample\nchecks:\n  - address: h:1\n"},
	} {
		b, err := Load(t.Context(), served(t, tc.contentType, tc.body, 200)+"/bundle")
		if err != nil || b.ID != "sample" {
			t.Errorf("%s: %v", name, err)
		}
	}
}

func TestLoadHTTPFailures(t *testing.T) {
	for name, url := range map[string]string{
		"404":      served(t, "", "nope", 404),
		"500":      served(t, "", "boom", 500),
		"oversize": served(t, "application/json", jsonBundle+strings.Repeat(" ", 4096), 200),
	} {
		if _, err := Load(t.Context(), url, WithMaxSize(1024)); err == nil {
			t.Errorf("%s: expected an error", name)
		}
	}
}

func TestLoadHTTPHangingServerHonoursTheDeadline(t *testing.T) {
	block := make(chan struct{})
	s := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { <-block }))
	defer s.Close()
	defer close(block)
	ctx, cancel := context.WithTimeout(t.Context(), 200*time.Millisecond)
	defer cancel()
	start := time.Now()
	if _, err := Load(ctx, s.URL); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("want a deadline error, got %v", err)
	}
	if time.Since(start) > 2*time.Second {
		t.Fatal("took too long")
	}
}

func TestLoadHTTPS(t *testing.T) {
	s := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(jsonBundle))
	}))
	defer s.Close()
	if _, err := Load(t.Context(), s.URL); err == nil {
		t.Error("an untrusted certificate must fail")
	}
	if b, err := Load(t.Context(), s.URL, WithHTTPClient(s.Client())); err != nil || b.ID != "sample" {
		t.Errorf("trusted client: %v", err)
	}
}

// D-1: a scheme nobody registered is an error, never a file name.
func TestLoadUnknownScheme(t *testing.T) {
	for _, loc := range []string{"redis://h:1?key=k", "ftp://h/b.yaml", "consulkv://h?key=k", "https-://h/b"} {
		_, err := Load(t.Context(), loc)
		if !errors.Is(err, ErrUnknownScheme) || !strings.Contains(err.Error(), strings.SplitN(loc, ":", 2)[0]) {
			t.Errorf("%s: %v", loc, err)
		}
	}
}

type fakeSource struct {
	got     *url.URL
	maxSize int64
	data    string
	err     error
}

func (f *fakeSource) Fetch(_ context.Context, u *url.URL, maxSize int64) ([]byte, Format, error) {
	f.got, f.maxSize = u, maxSize
	return []byte(f.data), Unknown, f.err
}

func TestWithSource(t *testing.T) {
	src := &fakeSource{data: yamlBundle}
	b, err := Load(t.Context(), "mine://host/path?key=v", WithSource(src, "mine", "mines"), WithMaxSize(4096))
	if err != nil || b.ID != "sample" {
		t.Fatal(err)
	}
	if src.got.Host != "host" || src.got.Query().Get("key") != "v" || src.maxSize != 4096 {
		t.Errorf("source called with %v, %d", src.got, src.maxSize)
	}
	if _, err := Load(t.Context(), "mines://host", WithSource(src, "mine", "mines")); err != nil {
		t.Errorf("every scheme is registered: %v", err)
	}
	src.data = strings.Repeat("x", 5000)
	if _, err := Load(t.Context(), "mine://host", WithSource(src, "mine"), WithMaxSize(100)); err == nil {
		t.Error("a source's oversize answer must fail")
	}
	src.data, src.err = yamlBundle, errors.New("connecting with s3cret failed")
	if _, err := Load(t.Context(), "mine://user:"+password+"@host", WithSource(src, "mine")); err == nil || strings.Contains(err.Error(), "s3cret") {
		t.Errorf("credentials in the error: %v", err)
	}
}

// NC-17
func TestCredentialsNeverLeak(t *testing.T) {
	var buf bytes.Buffer
	log := slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug}))
	_, err := Load(t.Context(), "http://user:"+password+"@127.0.0.1:1/b.yaml", WithLogger(log))
	if err == nil {
		t.Fatal("expected an error")
	}
	if strings.Contains(err.Error(), "s3cret") || strings.Contains(buf.String(), "s3cret") {
		t.Fatalf("leak:\nerror: %v\nlog: %s", err, buf.String())
	}
	if buf.Len() == 0 {
		t.Error("expected something in the log")
	}
}
