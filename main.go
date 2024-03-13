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
	"github.com/dihedron/netcheck/version"
	"github.com/fatih/color"
	"github.com/jessevdk/go-flags"
	"github.com/mattn/go-isatty"
	"gopkg.in/yaml.v3"
)

func init() {

	const LevelNone = slog.Level(1000)

	options := &slog.HandlerOptions{
		Level:     LevelNone,
		AddSource: true,
	}

	// my-app -> MY_APP_LOG_LEVEL
	level, ok := os.LookupEnv(
		fmt.Sprintf(
			"%s_LOG_LEVEL",
			strings.ReplaceAll(
				strings.ToUpper(
					path.Base(os.Args[0]),
				),
				"-",
				"_",
			),
		),
	)
	if ok {
		switch strings.ToLower(level) {
		case "debug", "dbg", "d", "trace", "trc", "t":
			options.Level = slog.LevelDebug
		case "informational", "info", "inf", "i":
			options.Level = slog.LevelInfo
		case "warning", "warn", "wrn", "w":
			options.Level = slog.LevelWarn
		case "error", "err", "e", "fatal", "ftl", "f":
			options.Level = slog.LevelError
		case "off", "none", "null", "nil", "no", "n":
			options.Level = LevelNone
			return
		}
	}
	handler := slog.NewTextHandler(os.Stderr, options)
	slog.SetDefault(slog.New(handler))
}

var (
	red     = color.New(color.FgRed).SprintfFunc()
	green   = color.New(color.FgGreen).SprintfFunc()
	yellow  = color.New(color.FgYellow).SprintfFunc()
	magenta = color.New(color.FgMagenta).SprintfFunc()
	cyan    = color.New(color.FgCyan).SprintfFunc()
	blue    = color.New(color.FgBlue).SprintfFunc()
)

func main() {

	var options struct {
		Version     bool    `short:"v" long:"version" description:"Show version information"`
		Format      string  `short:"f" long:"format" choice:"json" choice:"yaml" choice:"text" choice:"template" optional:"true" default:"text"`
		Template    *string `short:"t" long:"template" optional:"true"`
		Diagnostics bool    `long:"print-diagnostics" optional:"true"`
	}

	args, err := flags.Parse(&options)
	if err != nil {
		slog.Error("error parsing command line", "error", err)
		fmt.Fprintf(os.Stderr, "Invalid command line: %v\n", err)
		os.Exit(1)
	}

	source, err := os.Hostname()
	if err != nil {
		slog.Error("error retrieving hostname", "error", err)
		fmt.Fprintf(os.Stderr, "Error retrieving current host name: %v\n", err)
		os.Exit(1)
	}

	if options.Version && options.Format == "text" {
		fmt.Printf("%s v%s.%s.%s (%s/%s built with %s on %s)\n", version.Name, version.VersionMajor, version.VersionMinor, version.VersionPatch, version.GoOS, version.GoArch, version.GoVersion, version.BuildTime)
	}

	if options.Template != nil {
		slog.Debug("forcing format to be template")
		options.Format = "template"
	}
	if options.Format == "template" {
		if options.Template == nil {
			slog.Error("null template specified")
			fmt.Fprintf(os.Stderr, "No template specified\n")
			os.Exit(1)
		} else if !isFile(*options.Template) {
			slog.Error("template is not a valid file", "path", *options.Template)
			fmt.Fprintf(os.Stderr, "Specified template is not a file: %s\n", *options.Template)
			os.Exit(1)
		}
	}

	var output any

	if len(args) == 0 {
		// there is no input provided, so we're playing with
		// mock data to check the template provided by the user
		output = checks.MockBundles
	} else {
		bundles := []*checks.Bundle{}
		for _, arg := range args {
			bundle, err := checks.New(arg)
			if err != nil {
				slog.Error("error loading package", "path", arg, "error", err)
				fmt.Fprintf(os.Stderr, "Cannot load package from %s: %v\n", arg, err)
				os.Exit(1)
			}

			bundle.Check()

			switch options.Format {
			case "text":
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
			default:
				bundles = append(bundles, bundle)
			}
		}
		output = bundles
	}

	switch options.Format {
	case "json":
		data, err := json.MarshalIndent(output, "", "  ")
		if err != nil {
			slog.Error("error marshalling results to JSON", "error", err)
			fmt.Fprintf(os.Stderr, "Error writing result as JSON: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("%s\n", string(data))
	case "yaml":
		data, err := yaml.Marshal(output)
		if err != nil {
			slog.Error("error marshalling results to YAML", "error", err)
			fmt.Fprintf(os.Stderr, "Error writing result as YAML: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("%s\n", string(data))
	case "template":
		slog.Debug("using template file", "path", *options.Template)
		functions := template.FuncMap{}
		for k, v := range extensions.FuncMap() {
			functions[k] = v
		}
		for k, v := range sprig.FuncMap() {
			functions[k] = v
		}
		tmpl, err := template.New(path.Base(*options.Template)).Funcs(functions).ParseFiles(*options.Template)
		if err != nil {
			slog.Error("error parsing template", "path", *options.Template, "error", err)
			fmt.Fprintf(os.Stderr, "Error parsing template file %s: %v\n", *options.Template, err)
			os.Exit(1)
		}
		if err := tmpl.Execute(os.Stdout, output); err != nil {
			slog.Error("error executing template", "data", logging.ToJSON(output), "error", err)
			fmt.Fprintf(os.Stderr, "Error applying template file %s: %v\n", *options.Template, err)
			os.Exit(1)
		}
		if len(args) == 0 && options.Diagnostics {
			// dump the template diagnostics
			fmt.Fprintf(os.Stderr, "ACCESSED FIELDS:\n")
			printDiagnostics(output.([]checks.TrackedBundle))
		}
	}
}

func getHostnamePort(address string) (string, string) {
	tokens := strings.Split(address, ":")
	switch len(tokens) {
	case 0:
		return "", ""
	case 1:
		return tokens[0], ""
	default:
		return tokens[0], tokens[1]
	}
}

func isFile(path string) bool {
	info, err := os.Stat(path)
	if os.IsNotExist(err) {
		return false
	}
	return !info.IsDir()
}

func printDiagnostics(bundles []checks.TrackedBundle) {

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
