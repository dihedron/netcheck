package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/dihedron/netcheck/v2/internal/mock"
	"github.com/dihedron/netcheck/v2/pkg/netcheck"
	"gopkg.in/yaml.v3"
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

// results is the report of a bundle with one passed and two failed checks.
func results(t *testing.T) *netcheck.Report {
	t.Helper()
	l := mustListen(t)
	// an HTTPS server whose certificate the system does not trust: the check fails
	secure := httptest.NewTLSServer(http.NotFoundHandler())
	t.Cleanup(secure.Close)
	bundle := &netcheck.Bundle{
		ID:          "printed",
		Timeout:     netcheck.Duration(time.Second),
		Retries:     1,
		Concurrency: 2,
		Checks: []netcheck.Check{
			{Name: "up", Address: l, Protocol: netcheck.TCP},
			{Name: "a very long name that does not fit the column of thirty-two characters", Address: "127.0.0.1:1", Protocol: netcheck.TCP},
			{Name: "web", Address: strings.TrimPrefix(secure.URL, "https://"), Protocol: netcheck.HTTPS},
		},
	}
	report, err := netcheck.Run(t.Context(), bundle)
	if err != nil {
		t.Fatal(err)
	}
	return report
}

func TestWriteJSONAndYAML(t *testing.T) {
	reports := []*netcheck.Report{results(t)}
	var out bytes.Buffer
	if err := writeJSON(&out, reports); err != nil {
		t.Fatal(err)
	}
	var back []netcheck.Report
	if err := json.Unmarshal(out.Bytes(), &back); err != nil || len(back) != 1 || back[0].BundleID != "printed" || len(back[0].Results) != 3 {
		t.Fatalf("JSON: %v\n%s", err, out.String())
	}
	if back[0].Results[0].Status != netcheck.Passed || back[0].Results[1].Status != netcheck.Failed || back[0].Results[1].Err == nil {
		t.Errorf("statuses: %+v", back[0].Results)
	}
	out.Reset()
	if err := writeYAML(&out, reports); err != nil {
		t.Fatal(err)
	}
	var again []netcheck.Report
	if err := yaml.Unmarshal(out.Bytes(), &again); err != nil || again[0].BundleID != "printed" || again[0].Results[0].Status != netcheck.Passed {
		t.Fatalf("YAML: %v\n%s", err, out.String())
	}
}

func TestWriteTemplate(t *testing.T) {
	tpl := filepath.Join(t.TempDir(), "t.tpl")
	body := `{{ range . }}{{ .BundleID | upper }}:{{ range .Results }}{{ .Check.Protocol }}={{ .Status }};{{ end }}{{ end }}`
	if err := os.WriteFile(tpl, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	if err := writeTemplate(&out, []*netcheck.Report{results(t)}, tpl); err != nil {
		t.Fatal(err)
	}
	if out.String() != "PRINTED:tcp=passed;tcp=failed;https=failed;" {
		t.Errorf("template output: %q", out.String())
	}
	// errors are returned, not fatal
	if err := writeTemplate(&out, nil, filepath.Join(t.TempDir(), "absent.tpl")); err == nil {
		t.Error("a missing template must be an error")
	}
	bad := filepath.Join(t.TempDir(), "bad.tpl")
	_ = os.WriteFile(bad, []byte("{{ .NoSuchField.X }"), 0o600)
	if err := writeTemplate(&out, nil, bad); err == nil {
		t.Error("a broken template must be an error")
	}
	_ = os.WriteFile(bad, []byte("{{ range . }}{{ .NoSuchField }}{{ end }}"), 0o600)
	if err := writeTemplate(&out, []*netcheck.Report{{}}, bad); err == nil {
		t.Error("a template that fails when applied must be an error")
	}
}

// Every template shipped with the repository works on the mock data and on real data.
func TestShippedTemplates(t *testing.T) {
	for _, tpl := range []string{filepath.Join("..", "..", "examples", "output.tpl"), filepath.Join("..", "..", "test", "fixtures", "output.tpl")} {
		var out bytes.Buffer
		if err := writeTemplate(&out, mock.Reports(), tpl); err != nil || !strings.Contains(out.String(), "mock-bundle-1") {
			t.Errorf("%s on mock data: %v\n%s", tpl, err, out.String())
		}
		out.Reset()
		if err := writeTemplate(&out, []*netcheck.Report{results(t)}, tpl); err != nil || !strings.Contains(out.String(), "printed") {
			t.Errorf("%s on real data: %v\n%s", tpl, err, out.String())
		}
	}
}

// The mock-data run prints which template fields were read.
func TestWriteDiagnostics(t *testing.T) {
	reports := mock.Reports()
	_ = reports[0].BundleID()
	_ = reports[0].Results()
	var out bytes.Buffer
	writeDiagnostics(&out, reports)
	for _, want := range []string{"ACCESSED FIELDS", ".BundleID", ".Description", ".Results", ".Address", ".Protocol", ".Status", ".Err", ".TLS"} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("diagnostics lack %q:\n%s", want, out.String())
		}
	}
}

// textReport has checks with and without a port, a long name, a failure and a skip.
func textReport() *netcheck.Report {
	check := func(name, address string, p netcheck.Protocol) netcheck.Check {
		return netcheck.Check{Name: name, Address: address, Protocol: p}
	}
	return &netcheck.Report{BundleID: "t", Results: []netcheck.Result{
		{Check: check("db", "db.example.com:5432", netcheck.TCP), Status: netcheck.Failed, Err: errors.New("boom")},
		{Check: check("", "www.example.com/health", netcheck.HTTPS), Status: netcheck.Passed},
		{Check: check(strings.Repeat("n", 40), "git.example.com", netcheck.SSH), Status: netcheck.Passed},
		{Check: check("", "gw.example.com", netcheck.ICMP), Status: netcheck.Skipped, Err: errCanceled},
		{Check: check("v6", "[2001:db8::1]:443", netcheck.TLS), Status: netcheck.Passed},
	}}
}

var errCanceled = errors.New("context canceled")

// Off a terminal the output is plain, with "-" where a check has no port.
func TestWriteTextPlain(t *testing.T) {
	var out bytes.Buffer
	writeText(&out, textReport(), false)
	// the layout: "%s %5s %-5s → %-32s : %-32s", then the reason if any
	line := func(mark, port, protocol, host, name, why string) string {
		out := fmt.Sprintf("%s %5s %-5s → %-32s : %-32s", mark, port, protocol, host, name)
		if why != "" {
			out += " " + why
		}
		return out + "\n"
	}
	want := line("▼", "5432", "tcp", "db.example.com", "db", "boom") +
		line("▲", "-", "https", "www.example.com", "", "") +
		line("▲", "-", "ssh", "git.example.com", strings.Repeat("n", 29)+"...", "") +
		line("◆", "-", "icmp", "gw.example.com", "", "context canceled") +
		line("▲", "443", "tls", "2001:db8::1", "v6", "")
	if out.String() != want {
		t.Errorf("plain output:\n%q\nwant\n%q", out.String(), want)
	}
}

// On a terminal the ports a protocol defaults to are shown, and the reason is
// in parentheses (colours are off in tests: fatih/color follows the terminal).
func TestWriteTextTerminal(t *testing.T) {
	var out bytes.Buffer
	writeText(&out, textReport(), true)
	lines := strings.Split(strings.TrimSuffix(out.String(), "\n"), "\n")
	if len(lines) != 5 {
		t.Fatalf("%d lines:\n%s", len(lines), out.String())
	}
	for i, want := range []string{"5432", "443", "22", "-", "443"} {
		if !strings.Contains(lines[i], " "+want+" ") {
			t.Errorf("line %d lacks port %s: %q", i, want, lines[i])
		}
	}
	if !strings.HasSuffix(lines[0], "(boom)") || !strings.HasSuffix(lines[3], "(context canceled)") {
		t.Errorf("the reason must be in parentheses: %q / %q", lines[0], lines[3])
	}
}
