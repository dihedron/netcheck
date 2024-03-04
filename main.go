package main

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"strings"

	"github.com/dihedron/netcheck/checks"
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
		Version bool   `short:"v" long:"version" description:"Show version information"`
		Format  string `short:"f" long:"format" choice:"json" choice:"yaml" choice:"text" optional:"true" default:"text"`
	}

	args, err := flags.Parse(&options)
	if err != nil {
		slog.Error("error parsing command line", "error", err)
		os.Exit(1)
	}

	if options.Version && options.Format == "text" {
		fmt.Printf("%s v%s.%s.%s (%s/%s built with %s on %s)\n", version.Name, version.VersionMajor, version.VersionMinor, version.VersionPatch, version.GoOS, version.GoArch, version.GoVersion, version.BuildTime)
	}

	bundles := map[string][]checks.Result{}

	for _, arg := range args {
		bundle, err := checks.New(arg)
		if err != nil {
			slog.Error("error loading package", "path", arg, "error", err)
			os.Exit(1)
		}

		switch options.Format {
		case "text":
			if isatty.IsTerminal(os.Stdout.Fd()) {
				yellow(os.Stdout, "► %s\n", bundle.ID)
				for _, result := range bundle.Check() {
					if result.Error == nil {
						green(os.Stdout, "▲ %-4s → %s\n", result.Protocol, result.Endpoint) // was ✔
					} else {
						red(os.Stdout, "▼ %-4s → %s (%v)\n", result.Protocol, result.Endpoint, result.Error) // was ✖
					}
				}
			} else {
				fmt.Printf("package: %s\n", bundle.ID)
				for _, result := range bundle.Check() {
					if result.Error == nil {
						fmt.Printf(" - %s/%s: ok\n", result.Protocol, result.Endpoint)
					} else {
						fmt.Printf(" - %s/%s: ko (%v)\n", result.Protocol, result.Endpoint, result.Error)
					}
				}
			}
		default:
			bundles[bundle.ID] = bundle.Check()
		}
	}

	switch options.Format {
	case "json":
		data, err := json.MarshalIndent(bundles, "", "  ")
		if err != nil {
			slog.Error("error marshalling results to JSON", "error", err)
			os.Exit(1)
		}
		fmt.Printf("%s\n", string(data))
	case "yaml":
		data, err := yaml.Marshal(bundles)
		if err != nil {
			slog.Error("error marshalling results to YAML", "error", err)
			os.Exit(1)
		}
		fmt.Printf("%s\n", string(data))
	}
}
