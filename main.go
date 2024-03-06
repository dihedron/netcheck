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

	level, ok := os.LookupEnv("NETCHECK_LOG_LEVEL")
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
	red    = color.New(color.FgRed).FprintfFunc()
	green  = color.New(color.FgGreen).FprintfFunc()
	yellow = color.New(color.FgYellow).FprintfFunc()
)

func main() {

	var options struct {
		Version  bool    `short:"v" long:"version" description:"Show version information"`
		Format   string  `short:"f" long:"format" choice:"json" choice:"yaml" choice:"text" choice:"template" optional:"true" default:"text"`
		Template *string `short:"t" long:"template" optional:"true"`
	}

	args, err := flags.Parse(&options)
	if err != nil {
		slog.Error("error parsing command line", "error", err)
		fmt.Fprintf(os.Stderr, "Invalid command line: %v\n", err)
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
				yellow(os.Stdout, "► %s\n", bundle.ID)
				for _, check := range bundle.Checks {
					if check.Result.IsError() {
						red(os.Stdout, "▼ %-4s → %s (%v)\n", check.Protocol, check.Address, check.Result.String()) // was ✖
					} else {
						green(os.Stdout, "▲ %-4s → %s\n", check.Protocol, check.Address) // was ✔
					}
				}
			} else {
				fmt.Printf("bundle: %s\n", bundle.ID)
				for _, check := range bundle.Checks {
					if check.Result.IsError() {
						fmt.Printf(" - %s/%s: ko (%v)\n", check.Protocol, check.Address, check.Result.String())
					} else {
						fmt.Printf(" - %s/%s: ok\n", check.Protocol, check.Address)
					}
				}
			}
		default:
			bundles = append(bundles, bundle)
		}
	}

	switch options.Format {
	case "json":
		data, err := json.MarshalIndent(bundles, "", "  ")
		if err != nil {
			slog.Error("error marshalling results to JSON", "error", err)
			fmt.Fprintf(os.Stderr, "Error writing result as JSON: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("%s\n", string(data))
	case "yaml":
		data, err := yaml.Marshal(bundles)
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
		if err := tmpl.Execute(os.Stdout, bundles); err != nil {
			slog.Error("error executing template", "data", logging.ToJSON(bundles), "error", err)
			fmt.Fprintf(os.Stderr, "Error applying template file %s: %v\n", *options.Template, err)
			os.Exit(1)
		}
	}
}

func isFile(path string) bool {
	info, err := os.Stat(path)
	if os.IsNotExist(err) {
		return false
	}
	return !info.IsDir()
}
