package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/dihedron/netcheck/v2/internal/command/completion"
	"github.com/dihedron/netcheck/v2/internal/command/version"
	"github.com/dihedron/netcheck/v2/internal/mock"
	"github.com/dihedron/netcheck/v2/internal/spinner"
	"github.com/dihedron/netcheck/v2/pkg/metadata"
	"github.com/dihedron/netcheck/v2/pkg/netcheck"
	"github.com/dihedron/netcheck/v2/pkg/netcheck/dtls"
	"github.com/dihedron/netcheck/v2/pkg/netcheck/sso"
	"github.com/fatih/color"
	"github.com/jessevdk/go-flags"
	"github.com/mattn/go-isatty"
)

// The exit codes: 0 every check passed; 1 a usage, load or template error, or
// an interruption; 2 at least one check failed.
const (
	exitOK     = 0
	exitError  = 1
	exitFailed = 2
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

// cliOptions is the flat command line of the checks.
type cliOptions struct {
	Format       string        `short:"f" long:"format" choice:"json" choice:"yaml" choice:"text" choice:"template" optional:"true" default:"text" description:"how to write the report"`
	Template     *string       `short:"t" long:"template" optional:"true" value-name:"FILE" description:"a Go template for the report; implies --format=template"`
	Diagnostics  bool          `long:"print-diagnostics" optional:"true" description:"print the report's diagnostics to the standard error"`
	AllowNet     []string      `long:"allow-net" description:"only connect to this network (CIDR or address); repeatable"`
	FetchTimeout time.Duration `long:"fetch-timeout" default:"30s" description:"longest time to load one bundle; 0 for no limit"`
	DenyNet      []string      `long:"deny-net" description:"never connect to this network (CIDR or address); repeatable, wins over --allow-net"`
}

// fail reports an error to the user and the log, and returns the exit code.
func fail(message string, err error, args ...any) int {
	slog.Error(message, append(args, "error", err)...)
	fmt.Fprintf(os.Stderr, "%s: %v\n", message, err)
	return exitError
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
	if len(os.Args) >= 2 && os.Args[1] == "completion" {
		return completion.Run(os.Args[2:], flags.NewParser(&cliOptions{}, flags.Default), os.Stdout, os.Stderr)
	}
	if len(os.Args) == 2 && os.Args[1] == "--version" {
		metadata.Print(os.Stdout)
		return exitOK
	}

	var options cliOptions
	parser := flags.NewParser(&options, flags.Default)
	args, err := parser.ParseArgs(os.Args[1:])
	if err != nil {
		if flags.WroteHelp(err) {
			return exitOK
		}
		return fail("Invalid command line", err)
	}

	if options.FetchTimeout < 0 {
		return fail("Invalid --fetch-timeout", errors.New("it must not be negative"))
	}
	if options.Template != nil {
		slog.Debug("forcing format to be template")
		options.Format = "template"
	}
	if options.Format == "template" {
		if options.Template == nil {
			return fail("No template specified", errors.New("use --template=FILE"))
		} else if !isFile(*options.Template) {
			return fail("Specified template is not a file", errors.New(*options.Template))
		}
	}

	if len(args) == 0 {
		// there is no input provided, so we're playing with
		// mock data to check the template provided by the user
		if options.Format != "template" {
			return fail("No bundle specified", errors.New("give the location of at least one bundle, or a template to try on mock data"))
		}
		reports := mock.Reports()
		if err := writeTemplate(os.Stdout, reports, *options.Template); err != nil {
			return fail("Cannot apply the template", err)
		}
		if options.Diagnostics {
			writeDiagnostics(os.Stderr, reports)
		}
		return exitOK
	}

	defaults, err := loadDefaults(defaultsPaths)
	if err != nil {
		return fail("Cannot read the defaults file", err)
	}
	allowed, err := parseNetworks(options.AllowNet)
	if err != nil {
		return fail("Invalid --allow-net", err)
	}
	denied, err := parseNetworks(options.DenyNet)
	if err != nil {
		return fail("Invalid --deny-net", err)
	}
	runOptions := []netcheck.Option{
		netcheck.WithDefaults(defaults),
		netcheck.WithLogger(slog.Default()),
		netcheck.WithRunner(netcheck.DTLS, dtls.Runner()),
		netcheck.WithSSOClient(sso.Client()),
		netcheck.WithAllowedNetworks(allowed...),
		netcheck.WithDeniedNetworks(denied...),
	}

	// Ctrl-C ends the run: the checks that did not finish are reported skipped
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	var (
		spin    *spinner.Spinner
		reports = []*netcheck.Report{}
		allOK   = true
	)
	for _, arg := range args {
		fetchCtx, cancel := ctx, context.CancelFunc(func() {})
		if options.FetchTimeout > 0 {
			fetchCtx, cancel = context.WithTimeout(ctx, options.FetchTimeout)
		}
		bundle, err := loadBundle(fetchCtx, arg)
		cancel()
		if err != nil {
			return fail("Cannot load the bundle", err, "path", arg)
		}
		if options.Format == "text" {
			if isatty.IsTerminal(os.Stdout.Fd()) {
				fmt.Printf("%s %s ", yellow("►"), bundle.ID)
				if spin == nil {
					spin = spinner.New(spinner.WithSequence(spinner.Sequence7))
				}
				spin.Start()
			} else {
				fmt.Printf("%s %s\n", "►", bundle.ID)
			}
		}

		// do the real check here!
		report, err := netcheck.Run(ctx, bundle, runOptions...)
		if options.Format == "text" && spin != nil {
			spin.Stop()
			fmt.Printf("\n")
		}
		if report == nil { // the bundle is invalid
			return fail("Invalid bundle", err, "path", arg)
		}
		reports = append(reports, report)
		allOK = allOK && report.OK()
		if options.Format == "text" {
			// text reports are printed out as they are evaluated...
			writeText(os.Stdout, report, isatty.IsTerminal(os.Stdout.Fd()))
		}
		if err != nil {
			// ... whereas in all other cases the output must be valid, so what
			// was done is written as a whole before leaving
			if perr := printReports(options.Format, reports, options.Template); perr != nil {
				slog.Error("cannot write the partial results", "error", perr)
			}
			fmt.Fprintf(os.Stderr, "Interrupted: %v\n", err)
			return exitError
		}
	}

	if err := printReports(options.Format, reports, options.Template); err != nil {
		return fail("Cannot write the results", err)
	}
	if !allOK {
		return exitFailed
	}
	return exitOK
}

// printReports writes the reports in the machine formats; text was written
// as the bundles ran.
func printReports(format string, reports []*netcheck.Report, templ *string) error {
	switch format {
	case "json":
		return writeJSON(os.Stdout, reports)
	case "yaml":
		return writeYAML(os.Stdout, reports)
	case "template":
		return writeTemplate(os.Stdout, reports, *templ)
	}
	return nil
}

// getHostnamePort splits the host and the port of a check's address, ignoring
// the path of an HTTP address; the port is empty when there is none.
func getHostnamePort(address string) (string, string) {
	if i := strings.IndexAny(address, "/?#"); i >= 0 {
		address = address[:i]
	}
	if host, port, err := net.SplitHostPort(address); err == nil {
		return host, port
	}
	return strings.Trim(address, "[]"), ""
}

func isFile(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return !info.IsDir()
}
