package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path"
	"strings"
	"text/template"

	"github.com/Masterminds/sprig/v3"
	"github.com/dihedron/netcheck/v2/internal/checks"
	"github.com/dihedron/netcheck/v2/internal/extensions"
	"github.com/dihedron/netcheck/v2/internal/logging"
	"github.com/mattn/go-isatty"
	"gopkg.in/yaml.v3"
)

const NameLength = 32

func printAsText(bundle *checks.Bundle, source string) {
	writeText(os.Stdout, bundle, isatty.IsTerminal(os.Stdout.Fd()))
}

// shorten cuts a check name to NameLength characters.
func shorten(name string) string {
	name = strings.TrimSpace(name)
	if len(name) > NameLength {
		return name[:NameLength-3] + "..."
	}
	return name
}

// defaultPort is the port shown, on a terminal, for a check whose address has none.
func defaultPort(p checks.Protocol) string {
	switch p {
	case checks.HTTP:
		return "80"
	case checks.HTTPS:
		return "443"
	case checks.SSH:
		return "22"
	default:
		return "-"
	}
}

// writeText writes one line per check of the bundle: a success or failure
// mark, the port, the protocol, the host, the name of the check and, for a
// failure, the error. On a terminal the output is coloured, the error is in
// parentheses and ports the protocol defaults to are shown.
func writeText(w io.Writer, bundle *checks.Bundle, terminal bool) {
	plain := func(format string, a ...any) string { return fmt.Sprintf(format, a...) }
	mark, port, protocol, failure := func(up bool) string {
		if up {
			return "▲"
		}
		return "▼"
	}, plain, plain, plain
	if terminal {
		port, protocol, failure = cyan, magenta, func(format string, a ...any) string { return blue(format, a...) }
	}
	for _, check := range bundle.Checks {
		target, p := getHostnamePort(check.Address)
		if p == "" {
			p = "-"
			if terminal {
				p = defaultPort(check.Protocol)
			}
		}
		symbol := mark(!check.Result.IsError())
		if terminal {
			if check.Result.IsError() {
				symbol = red("%s", symbol)
			} else {
				symbol = green("%s", symbol)
			}
		}
		line := fmt.Sprintf("%s %s%s %s%s → %-32s : %-32s",
			symbol,
			strings.Repeat(" ", max(0, 5-len(p))), port("%s", p),
			protocol("%s", check.Protocol.String()), strings.Repeat(" ", max(0, 5-len(check.Protocol.String()))),
			target, shorten(check.Name))
		if check.Result.IsError() {
			if terminal {
				line += " " + failure("(%s)", check.Result.String())
			} else {
				line += " " + check.Result.String()
			}
		}
		_, _ = fmt.Fprintln(w, line)
	}
}

func printAsJSON(bundles any) {
	data, err := json.MarshalIndent(bundles, "", "  ")
	if err != nil {
		slog.Error("error marshalling results to JSON", "error", err)
		fmt.Fprintf(os.Stderr, "Error writing result as JSON: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("%s\n", string(data))
}

func printAsYAML(bundles any) {
	data, err := yaml.Marshal(bundles)
	if err != nil {
		slog.Error("error marshalling results to YAML", "error", err)
		fmt.Fprintf(os.Stderr, "Error writing result as YAML: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("%s\n", string(data))
}

func printAsTemplate(bundles any, templ string) {
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
		slog.Error("error parsing template", "path", templ, "error", err)
		fmt.Fprintf(os.Stderr, "Error parsing template file %s: %v\n", templ, err)
		os.Exit(1)
	}
	if err := tmpl.Execute(os.Stdout, bundles); err != nil {
		slog.Error("error executing template", "data", logging.ToJSON(bundles), "error", err)
		fmt.Fprintf(os.Stderr, "Error applying template file %s: %v\n", templ, err)
		os.Exit(1)
	}
}

func printDiagnostics(bundles []checks.TrackedBundle) {
	fmt.Fprintf(os.Stderr, "ACCESSED FIELDS:\n")
	for _, bundle := range bundles {
		fmt.Fprintf(os.Stderr, "Bundle . {\n")

		if bundle.IDAccessed() {
			fmt.Fprintf(os.Stderr, "  %s\n", magenta(".ID"))
		} else {
			fmt.Fprintf(os.Stderr, "  %s\n", ".ID")
		}

		if bundle.DescriptionAccessed() {
			fmt.Fprintf(os.Stderr, "  %s\n", magenta(".Description"))
		} else {
			fmt.Fprintf(os.Stderr, "  %s\n", ".Description")
		}

		if bundle.TimeoutAccessed() {
			fmt.Fprintf(os.Stderr, "  %s\n", magenta(".Timeout"))
		} else {
			fmt.Fprintf(os.Stderr, "  %s\n", ".Timeout")
		}

		if bundle.RetriesAccessed() {
			fmt.Fprintf(os.Stderr, "  %s\n", magenta(".Retries"))
		} else {
			fmt.Fprintf(os.Stderr, "  %s\n", ".Retries")
		}

		if bundle.WaitAccessed() {
			fmt.Fprintf(os.Stderr, "  %s\n", magenta(".Wait"))
		} else {
			fmt.Fprintf(os.Stderr, "  %s\n", ".Wait")
		}

		if bundle.ConcurrencyAccessed() {
			fmt.Fprintf(os.Stderr, "  %s\n", magenta(".Concurrency"))
		} else {
			fmt.Fprintf(os.Stderr, "  %s\n", ".Concurrency")
		}

		if bundle.ChecksAccessed() {
			fmt.Fprintf(os.Stderr, "  %s [\n", magenta(".Checks"))
		} else {
			fmt.Fprintf(os.Stderr, "  %s [\n", ".Checks")
		}

		for _, check := range bundle.Checks() {
			fmt.Fprintf(os.Stderr, "    {\n")
			if check.DescriptionAccessed() {
				fmt.Fprintf(os.Stderr, "      %s\n", magenta(".Description"))
			} else {
				fmt.Fprintf(os.Stderr, "      %s\n", ".Description")
			}

			if check.TimeoutAccessed() {
				fmt.Fprintf(os.Stderr, "      %s\n", magenta(".Timeout"))
			} else {
				fmt.Fprintf(os.Stderr, "      %s\n", ".Timeout")
			}

			if check.RetriesAccessed() {
				fmt.Fprintf(os.Stderr, "      %s\n", magenta(".Retries"))
			} else {
				fmt.Fprintf(os.Stderr, "      %s\n", ".Retries")
			}

			if check.WaitAccessed() {
				fmt.Fprintf(os.Stderr, "      %s\n", magenta(".Wait"))
			} else {
				fmt.Fprintf(os.Stderr, "      %s\n", ".Wait")
			}

			if check.AddressAccessed() {
				fmt.Fprintf(os.Stderr, "      %s\n", magenta(".Address"))
			} else {
				fmt.Fprintf(os.Stderr, "      %s\n", ".Address")
			}

			if check.ProtocolAccessed() {
				fmt.Fprintf(os.Stderr, "      %s\n", magenta(".Protocol"))
			} else {
				fmt.Fprintf(os.Stderr, "      %s\n", ".Protocol")
			}

			if check.WaitAccessed() {
				fmt.Fprintf(os.Stderr, "      %s\n", magenta(".Wait"))
			} else {
				fmt.Fprintf(os.Stderr, "      %s\n", ".Wait")
			}

			if check.ResultAccessed() {
				fmt.Fprintf(os.Stderr, "      %s\n", magenta(".Result"))
			} else {
				fmt.Fprintf(os.Stderr, "      %s\n", ".Result")
			}
			fmt.Fprintf(os.Stderr, "    }\n")
		}
		fmt.Fprintf(os.Stderr, "  ]\n")
		fmt.Fprintf(os.Stderr, "}\n")
	}
}
