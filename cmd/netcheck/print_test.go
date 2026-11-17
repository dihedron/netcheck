package main

import (
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dihedron/netcheck/v2/internal/checks"
)

// capture runs fn and returns what it wrote to stdout and stderr.
func capture(t *testing.T, fn func()) (stdout, stderr string) {
	t.Helper()
	outR, outW, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	errR, errW, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	savedOut, savedErr := os.Stdout, os.Stderr
	os.Stdout, os.Stderr = outW, errW
	defer func() { os.Stdout, os.Stderr = savedOut, savedErr }()
	fn()
	_ = outW.Close()
	_ = errW.Close()
	o, _ := io.ReadAll(outR)
	e, _ := io.ReadAll(errR)
	return string(o), string(e)
}

// results is a bundle with one passed and one failed check.
func results(t *testing.T) *checks.Bundle {
	t.Helper()
	l := mustListen(t)
	// an HTTPS server whose certificate the system does not trust: the check fails
	secure := httptest.NewTLSServer(http.NotFoundHandler())
	t.Cleanup(secure.Close)
	bundle := &checks.Bundle{
		ID:          "printed",
		Timeout:     checks.Timeout(1_000_000_000),
		Retries:     1,
		Concurrency: 2,
		Checks: []checks.Check{
			{Name: "up", Address: l, Protocol: checks.TCP},
			{Name: "a very long name that does not fit the column of thirty-two characters", Address: "127.0.0.1:1", Protocol: checks.TCP},
			{Name: "web", Address: strings.TrimPrefix(secure.URL, "https://"), Protocol: checks.HTTPS},
		},
	}
	bundle.Check()
	return bundle
}

func TestPrintAsText(t *testing.T) {
	bundle := results(t)
	out, _ := capture(t, func() { printAsText(bundle, "host") })
	for _, want := range []string{"▲", "▼", "tcp", "https", "up", "..."} {
		if !strings.Contains(out, want) {
			t.Errorf("text output lacks %q:\n%s", want, out)
		}
	}
	if strings.Count(out, "\n") != 3 {
		t.Errorf("expected one line per check:\n%s", out)
	}
}

func TestPrintAsJSONAndYAML(t *testing.T) {
	bundles := []*checks.Bundle{results(t)}
	out, _ := capture(t, func() { printAsJSON(bundles) })
	if !strings.Contains(out, `"id": "printed"`) || !strings.Contains(out, `"protocol": "tcp"`) || !strings.Contains(out, `"result": "success"`) {
		t.Errorf("JSON:\n%s", out)
	}
	out, _ = capture(t, func() { printAsYAML(bundles) })
	if !strings.Contains(out, "id: printed") || !strings.Contains(out, "result: success") {
		t.Errorf("YAML:\n%s", out)
	}
}

func TestPrintAsTemplate(t *testing.T) {
	tpl := filepath.Join(t.TempDir(), "t.tpl")
	if err := os.WriteFile(tpl, []byte(`{{ range . }}{{ .ID | upper }}:{{ range .Checks }}{{ .Protocol.String }}={{ if .Result.IsError }}fail{{ else }}ok{{ end }};{{ end }}{{ end }}`), 0o600); err != nil {
		t.Fatal(err)
	}
	bundles := []*checks.Bundle{results(t)}
	out, _ := capture(t, func() { printAsTemplate(bundles, tpl) })
	if !strings.HasPrefix(out, "PRINTED:tcp=ok;tcp=fail;https=fail;") {
		t.Errorf("template output: %q", out)
	}
}

// The mock-data run prints which template fields were read.
func TestPrintDiagnostics(t *testing.T) {
	mock := checks.MockBundles
	_ = mock[0].ID()
	_ = mock[0].Checks()
	_, errOut := capture(t, func() { printDiagnostics(mock) })
	for _, want := range []string{"ACCESSED FIELDS", ".ID", ".Description", ".Concurrency", ".Checks", ".Address", ".Protocol", ".Result"} {
		if !strings.Contains(errOut, want) {
			t.Errorf("diagnostics lack %q:\n%s", want, errOut)
		}
	}
}
