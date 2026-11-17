package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"path"
	"strings"
	"text/template"

	"github.com/Masterminds/sprig/v3"
	"github.com/dihedron/netcheck/v2/internal/extensions"
	"github.com/dihedron/netcheck/v2/internal/mock"
	"github.com/dihedron/netcheck/v2/pkg/netcheck"
	"gopkg.in/yaml.v3"
)

const nameLength = 32

// shorten cuts a check name to nameLength characters.
func shorten(name string) string {
	name = strings.TrimSpace(name)
	if len(name) > nameLength {
		return name[:nameLength-3] + "..."
	}
	return name
}

// defaultPort is the port shown, on a terminal, for a check whose address has none.
func defaultPort(p netcheck.Protocol) string {
	switch p {
	case netcheck.HTTP:
		return "80"
	case netcheck.HTTPS:
		return "443"
	case netcheck.SSH:
		return "22"
	default:
		return "-"
	}
}

// writeText writes one line per check of the report: a status mark (▲ passed,
// ▼ failed, ◆ skipped), the port, the protocol, the host, the name of the
// check and, unless it passed, the reason. On a terminal the output is
// coloured, the reason is in parentheses and ports the protocol defaults to
// are shown.
func writeText(w io.Writer, report *netcheck.Report, terminal bool) {
	plain := func(format string, a ...any) string { return fmt.Sprintf(format, a...) }
	port, protocol, reason := plain, plain, plain
	if terminal {
		port, protocol, reason = cyan, magenta, blue
	}
	for _, result := range report.Results {
		check := result.Check
		target, p := getHostnamePort(check.Address)
		if p == "" {
			p = "-"
			if terminal {
				p = defaultPort(check.Protocol)
			}
		}
		symbol := "▲"
		colour := green
		switch result.Status {
		case netcheck.Failed:
			symbol, colour = "▼", red
		case netcheck.Skipped:
			symbol, colour = "◆", yellow
		}
		if !terminal {
			colour = plain
		}
		line := fmt.Sprintf("%s %s%s %s%s → %-32s : %-32s",
			colour("%s", symbol),
			strings.Repeat(" ", max(0, 5-len(p))), port("%s", p),
			protocol("%s", check.Protocol), strings.Repeat(" ", max(0, 5-len(check.Protocol))),
			target, shorten(check.Name))
		if result.Status != netcheck.Passed {
			why := string(result.Status)
			if result.Err != nil {
				why = result.Err.Error()
			}
			if terminal {
				line += " " + reason("(%s)", why)
			} else {
				line += " " + why
			}
		}
		_, _ = fmt.Fprintln(w, line)
	}
}

func writeJSON(w io.Writer, v any) error {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return fmt.Errorf("marshalling the results to JSON: %w", err)
	}
	_, err = fmt.Fprintf(w, "%s\n", data)
	return err
}

func writeYAML(w io.Writer, v any) error {
	data, err := yaml.Marshal(v)
	if err != nil {
		return fmt.Errorf("marshalling the results to YAML: %w", err)
	}
	_, err = fmt.Fprintf(w, "%s\n", data)
	return err
}

// writeTemplate executes the template file on the data: a list of reports.
func writeTemplate(w io.Writer, data any, templ string) error {
	slog.Debug("using template file", "path", templ)
	functions := template.FuncMap{}
	for k, v := range extensions.FuncMap() {
		functions[k] = v
	}
	for k, v := range sprig.FuncMap() {
		functions[k] = v
	}
	tmpl, err := template.New(path.Base(templ)).Funcs(functions).ParseFiles(templ)
	if err != nil {
		return fmt.Errorf("parsing template file %s: %w", templ, err)
	}
	if err := tmpl.Execute(w, data); err != nil {
		return fmt.Errorf("applying template file %s: %w", templ, err)
	}
	return nil
}

// writeDiagnostics shows which fields of the mock reports a template read.
func writeDiagnostics(w io.Writer, reports []*mock.Report) {
	mock.Diagnostics(w, reports, magenta)
}
