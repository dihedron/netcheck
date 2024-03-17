package main

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path"
	"strings"
	"text/template"

	"github.com/Masterminds/sprig/v3"
	"github.com/dihedron/netcheck/checks"
	"github.com/dihedron/netcheck/extensions"
	"github.com/dihedron/netcheck/logging"
	"github.com/mattn/go-isatty"
	"gopkg.in/yaml.v3"
)

func printAsText(bundle *checks.Bundle, source string) {
	if isatty.IsTerminal(os.Stdout.Fd()) {
		fmt.Printf("%s %s\n", yellow("►"), bundle.ID)
		for _, check := range bundle.Checks {
			target, port := getHostnamePort(check.Address)
			if port == "" {
				port = "-"
			}
			if check.Result.IsError() {
				fmt.Printf(
					"%s %5s %-4s : %s → %s %v\n",
					red("▼"),
					strings.Repeat(" ", 5-len(port))+cyan(port),
					magenta(check.Protocol.String())+strings.Repeat(" ", 4-len(check.Protocol.String())),
					source,
					target,
					blue("("+check.Result.String()+")")) // was ✖
			} else {
				fmt.Printf("%s %5s %-4s : %s → %s\n",
					green("▲"),
					strings.Repeat(" ", 5-len(port))+cyan(port),
					magenta(check.Protocol.String())+strings.Repeat(" ", 4-len(check.Protocol.String())),
					source,
					target) // was ✔
			}
		}
	} else {
		fmt.Printf("%s %s\n", "►", bundle.ID)
		for _, check := range bundle.Checks {
			target, port := getHostnamePort(check.Address)
			if port == "" {
				port = "-"
			}
			if check.Result.IsError() {
				fmt.Printf("%s %5s %-4s : %s → %s (%v)\n", "▼", port, check.Protocol.String(), source, target, check.Result.String()) // was ✖
			} else {
				fmt.Printf("%s %5s %-4s : %s → %s\n", "▲", port, check.Protocol.String(), source, target) // was ✔
			}
		}
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
