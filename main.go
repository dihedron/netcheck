package main

import (
	"fmt"
	"log/slog"
	"os"
	"strings"

	"github.com/dihedron/netcheck/checks"
	"github.com/fatih/color"
	"github.com/mattn/go-isatty"
)

func init() {
	options := &slog.HandlerOptions{
		Level:     slog.LevelWarn,
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

	for _, arg := range os.Args[1:] {
		pkg, err := checks.New(arg)
		if err != nil {
			slog.Error("error loading package", "path", arg, "error", err)
			os.Exit(1)
		}

		if isatty.IsTerminal(os.Stdout.Fd()) {
			yellow(os.Stdout, "► %s\n", pkg.ID)
			for _, result := range pkg.Check() {
				if result.Success {
					green(os.Stdout, "✔ %-4s → %s\n", result.Protocol, result.Endpoint)
				} else {
					red(os.Stdout, "✖ %-4s → %s\n", result.Protocol, result.Endpoint)
				}
			}
		} else {
			fmt.Printf("package: %s\n", pkg.ID)
			for _, result := range pkg.Check() {
				if result.Success {
					fmt.Printf(" - %s/%s: ok\n", result.Protocol, result.Endpoint)
				} else {
					fmt.Printf(" - %s/%s: ko\n", result.Protocol, result.Endpoint)
				}
			}
		}
	}
}
