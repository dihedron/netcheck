package main

import (
	"fmt"
	"log/slog"
	"os"
	"strings"

	"github.com/dihedron/netcheck/v2/internal/checks"
	"github.com/dihedron/netcheck/v2/internal/command/version"
	"github.com/dihedron/netcheck/v2/internal/spinner"
	"github.com/dihedron/netcheck/v2/pkg/metadata"
	"github.com/fatih/color"
	"github.com/jessevdk/go-flags"
	"github.com/mattn/go-isatty"
)

var (
	red     = color.New(color.FgRed).SprintfFunc()
	green   = color.New(color.FgGreen).SprintfFunc()
	yellow  = color.New(color.FgYellow).SprintfFunc()
	magenta = color.New(color.FgMagenta).SprintfFunc()
	cyan    = color.New(color.FgCyan).SprintfFunc()
	blue    = color.New(color.FgBlue).SprintfFunc()
)

func main() {
	os.Exit(run())
}

// run parses the command line and runs the command; it returns the exit
// code so that the deferred cleanup runs before the process exits.
func run() int {
	defer cleanup()

	// "netcheck version [-v] [-D file]" and "netcheck --version" are answered
	// before the flat command line of the checks is parsed
	if len(os.Args) >= 2 && os.Args[1] == "version" {
		return version.Run(os.Args[2:])
	}
	if len(os.Args) == 2 && os.Args[1] == "--version" {
		metadata.Print(os.Stdout)
		return 0
	}

	var options struct {
		Format      string  `short:"f" long:"format" choice:"json" choice:"yaml" choice:"text" choice:"template" optional:"true" default:"text"`
		Template    *string `short:"t" long:"template" optional:"true"`
		Diagnostics bool    `long:"print-diagnostics" optional:"true"`
	}
	args, err := flags.ParseArgs(&options, os.Args[1:])
	if err != nil {
		if flags.WroteHelp(err) {
			return 0
		}
		slog.Error("error parsing command line", "error", err)
		fmt.Fprintf(os.Stderr, "Invalid command line: %v\n", err)
		return 1
	}

	source, err := os.Hostname()
	if err != nil {
		slog.Error("error retrieving hostname", "error", err)
		fmt.Fprintf(os.Stderr, "Error retrieving current host name: %v\n", err)
		return 1
	}

	if options.Template != nil {
		slog.Debug("forcing format to be template")
		options.Format = "template"
	}

	if options.Format == "template" {
		if options.Template == nil {
			slog.Error("null template specified")
			fmt.Fprintf(os.Stderr, "No template specified\n")
			return 1
		} else if !isFile(*options.Template) {
			slog.Error("template is not a valid file", "path", *options.Template)
			fmt.Fprintf(os.Stderr, "Specified template is not a file: %s\n", *options.Template)
			return 1
		}
	}

	var output any
	if len(args) == 0 {
		// there is no input provided, so we're playing with
		// mock data to check the template provided by the user
		output = checks.MockBundles
	} else {
		var s *spinner.Spinner
		bundles := []*checks.Bundle{}
		for _, arg := range args {
			bundle, err := checks.New(arg)
			if err != nil {
				slog.Error("error loading package", "path", arg, "error", err)
				fmt.Fprintf(os.Stderr, "Cannot load package from %s: %v\n", arg, err)
				return 1
			}
			if options.Format == "text" {
				if isatty.IsTerminal(os.Stdout.Fd()) {
					fmt.Printf("%s %s ", yellow("►"), bundle.ID)
					if s == nil {
						s = spinner.New(
							spinner.WithSequence(
								spinner.Sequence7,
							),
						)
					}
					s.Start()
				} else {
					fmt.Printf("%s %s\n", "►", bundle.ID)
				}
			}

			// do the real check here!
			bundle.Check()

			switch options.Format {
			case "text":
				// text bundles are printed out as they are evaluated...
				if s != nil {
					s.Stop()
					fmt.Printf("\n")
				}
				printAsText(bundle, source)
			default:
				// ... whereas in all other cases we need to ensure that
				// the output is valid, so results are accumulated in order
				// for them to be treated as a whole in one go
				bundles = append(bundles, bundle)
			}
		}
		// we need to cast to any because MockBundle,
		// which is used for tracking accesses in golang
		// templates, is not the same as Bundle
		output = bundles
	}

	switch options.Format {
	case "json":
		printAsJSON(output)
	case "yaml":
		printAsYAML(output)
	case "template":
		printAsTemplate(output, *options.Template)
		if len(args) == 0 && options.Diagnostics {
			// dump the template diagnostics
			printDiagnostics(output.([]checks.TrackedBundle))
		}
	}
	return 0
}

func getHostnamePort(address string) (string, string) {
	// remove the trailing path (in case it's an HTTP resource URL)
	tokens := strings.Split(address, "/")
	tokens = strings.Split(tokens[0], ":")
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
