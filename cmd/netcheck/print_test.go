package main

import (
	"bytes"
	"errors"
	"fmt"
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

// textBundle has checks with and without a port, a long name and a failure.
func textBundle() *checks.Bundle {
	failed := checks.Check{Name: "db", Address: "db.example.com:5432", Protocol: checks.TCP}
	passed := checks.Check{Name: "", Address: "www.example.com/health", Protocol: checks.HTTPS}
	ssh := checks.Check{Name: strings.Repeat("n", 40), Address: "git.example.com", Protocol: checks.SSH}
	ping := checks.Check{Address: "gw.example.com", Protocol: checks.ICMP}
	failed.Result = checks.FailedResult(errors.New("boom"))
	return &checks.Bundle{ID: "t", Checks: []checks.Check{failed, passed, ssh, ping}}
}

// Off a terminal the output is plain, with "-" where a check has no port.
func TestWriteTextPlain(t *testing.T) {
	var out bytes.Buffer
	writeText(&out, textBundle(), false)
	// the original layout: "%s %5s %-5s → %-32s : %-32s", then the error if any
	line := func(mark, port, protocol, host, name, failure string) string {
		out := fmt.Sprintf("%s %5s %-5s → %-32s : %-32s", mark, port, protocol, host, name)
		if failure != "" {
			out += " " + failure
		}
		return out + "\n"
	}
	want := line("▼", "5432", "tcp", "db.example.com", "db", "boom") +
		line("▲", "-", "https", "www.example.com", "", "") +
		line("▲", "-", "ssh", "git.example.com", strings.Repeat("n", 29)+"...", "") +
		line("▲", "-", "icmp", "gw.example.com", "", "")
	if out.String() != want {
		t.Errorf("plain output:\n%q\nwant\n%q", out.String(), want)
	}
}

// On a terminal the ports a protocol defaults to are shown, and the error is
// in parentheses (colours are off in tests: fatih/color follows the terminal).
func TestWriteTextTerminal(t *testing.T) {
	var out bytes.Buffer
	writeText(&out, textBundle(), true)
	lines := strings.Split(strings.TrimSuffix(out.String(), "\n"), "\n")
	if len(lines) != 4 {
		t.Fatalf("%d lines:\n%s", len(lines), out.String())
	}
	for i, want := range []string{"5432", "443", "22", "-"} {
		if !strings.Contains(lines[i], " "+want+" ") {
			t.Errorf("line %d lacks port %s: %q", i, want, lines[i])
		}
	}
	if !strings.HasSuffix(lines[0], "(boom)") {
		t.Errorf("the error must be in parentheses: %q", lines[0])
	}
}
