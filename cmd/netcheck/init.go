package main

import (
	"fmt"
	"io"
	"log/slog"
	"os"
	"path"
	"runtime"
	"runtime/pprof"
	"strings"

	"github.com/dihedron/netcheck/v2/pkg/metadata"
)

var (
	cpuprof *os.File
	memprof *os.File
)

// envName returns the name of the environment variable that carries the
// given setting for this binary: for the binary "my-app" and the setting
// "LOG_LEVEL" it is "MY_APP_LOG_LEVEL".
func envName(setting string) string {
	return strings.ReplaceAll(strings.ToUpper(path.Base(os.Args[0])), "-", "_") + "_" + setting
}

// init sets up logging and profiling when the program starts; see setup.
func init() {
	setup()
}

// setup configures the logging and profiling, based on the relevant environment
// variables. The environment variables are:
//
// - <binary-name>_LOG_LEVEL: the log level (debug, info, warn, error, off)
// - <binary-name>_LOG_STREAM: the log stream (stderr, stdout, file)
// - <binary-name>_CPU_PROFILE: the CPU profile file name
// - <binary-name>_MEM_PROFILE: the memory profile file name
//
// where <binary-name> is the name of the binary, in uppercase, with hyphens replaced by
// underscores. By default logging is off (nothing is written) and there is no profiling.
// Environment variables can be loaded from a .env file by setting the
// <binary-name>_DOTENV environment variable to the path of the .env file.
func setup() {
	const LevelNone = slog.Level(1000)

	options := &slog.HandlerOptions{
		Level:     LevelNone,
		AddSource: true,
	}

	// load .env file if specified and present; it may set the variables below,
	// so it comes first, and is logged once the logger is configured
	dotenv, dotenvSet := os.LookupEnv(metadata.DotEnvVarName)
	var dotenvErr error
	if dotenvSet {
		dotenvErr = metadata.LoadDotEnv(dotenv)
	}

	level, levelSet := os.LookupEnv(envName("LOG_LEVEL"))
	if levelSet {
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
		}
	}

	var writer io.Writer = os.Stderr
	if stream, ok := os.LookupEnv(envName("LOG_STREAM")); ok {
		switch strings.ToLower(stream) {
		case "stderr", "error", "err", "e":
			writer = os.Stderr
		case "stdout", "output", "out", "o":
			writer = os.Stdout
		case "file":
			filename := fmt.Sprintf("%s-%d.log", path.Base(os.Args[0]), os.Getpid())
			var err error
			writer, err = os.Create(path.Clean(filename))
			if err != nil {
				writer = os.Stderr
			}
		}
	}

	// initialise the logger
	slog.SetDefault(slog.New(slog.NewTextHandler(writer, options)))

	if dotenvSet {
		switch {
		case dotenvErr == nil:
			slog.Info("successfully loaded .env file", "path", dotenv)
		case levelSet:
			slog.Error("error loading .env file", "path", dotenv, "error", dotenvErr)
		default: // no level set means no logger, but a failed load must not go unseen
			fmt.Fprintf(os.Stderr, "error loading .env file %s: %v\n", dotenv, dotenvErr)
		}
	}

	// check if CPU profiling should be enabled
	if filename, ok := os.LookupEnv(envName("CPU_PROFILE")); ok && filename != "" {
		f, err := os.Create(path.Clean(filename))
		if err != nil {
			slog.Error("could not create CPU profile", "error", err)
		} else {
			cpuprof = f
			if err := pprof.StartCPUProfile(f); err != nil {
				slog.Error("could not start CPU profile", "error", err)
			}
		}
	}

	// check if memory profiling should be enabled
	if filename, ok := os.LookupEnv(envName("MEM_PROFILE")); ok && filename != "" {
		f, err := os.Create(path.Clean(filename))
		if err != nil {
			slog.Error("could not create memory profile", "error", err)
		} else {
			memprof = f
		}
	}
}

// cleanup is called when the program is exiting: it stops the CPU profile and
// writes the memory profile.
func cleanup() {
	if cpuprof != nil {
		pprof.StopCPUProfile()
		closeProfile(cpuprof)
	}
	if memprof != nil {
		runtime.GC() // get up-to-date statistics
		if err := pprof.WriteHeapProfile(memprof); err != nil {
			slog.Error("could not write memory profile", "error", err)
		}
		closeProfile(memprof)
	}
}

// closeProfile closes a profile file, reporting a failure, which can lose
// the end of the profile.
func closeProfile(f *os.File) {
	if err := f.Close(); err != nil {
		slog.Error("could not close profile", "file", f.Name(), "error", err)
	}
}
