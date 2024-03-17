package main

import (
	"fmt"
	"log/slog"
	"os"
	"path"
	"strings"

	"github.com/dihedron/netcheck/checks"
	"github.com/dihedron/netcheck/version"
	"github.com/fatih/color"
	"github.com/jessevdk/go-flags"
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

			// do the real check here!
			bundle.Check()

			switch options.Format {
			case "text":
				// text bundles are printed out as they are evaluated...
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
