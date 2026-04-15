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

	"github.com/dihedron/netcheck/metadata"
	"github.com/joho/godotenv"
)

var (
	cpuprof *os.File
	memprof *os.File
)

// init is a function that is automatically called when the program is starting;
// it is used to set up the logging and profiling, based on the relevant environment
// variables. The environment variables are:
//
// - <binary-name>_LOG_LEVEL: the log level (debug, info, warn, error, off)
// - <binary-name>_LOG_STREAM: the log stream (stderr, stdout, file)
// - <binary-name>_CPU_PROFILE: the CPU profile file name
// - <binary-name>_MEM_PROFILE: the memory profile file name
//
// where <binary-name> is the name of the binary, in uppercase, with hyphens replaced by
// underscores. The default values are:
//
// - <binary-name>_LOG_LEVEL: "info"
// - <binary-name>_LOG_STREAM: "stderr"
// - <binary-name>_CPU_PROFILE: ""
// - <binary-name>_MEM_PROFILE: ""
// Environment variables can be loaded from a .env file by setting the
// <binary-name>_DOTENV environment variable to the path of the .env file.
func init() {
	const LevelNone = slog.Level(1000)

	options := &slog.HandlerOptions{
		Level:     LevelNone,
		AddSource: true,
	}

	// load .env file if specified and present
	if dotenv, ok := os.LookupEnv(metadata.DotEnvVarName); ok {
		slog.Info("loading .env file", "path", dotenv)
		if err := godotenv.Load(dotenv); err != nil {
			slog.Error("error loading .env file", "error", err)
		}
		slog.Info("successfully loaded .env file", "path", dotenv)
	}

	// get log level from environment variable where, given
	// the binary name "my-app", the environment variable is "MY_APP_LOG_LEVEL"
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

	// get the name of the file to log to from environment variable where, given
	// the binary name "my-app", the environment variable is "MY_APP_LOG_STREAM"
	var writer io.Writer = os.Stderr
	stream, ok := os.LookupEnv(
		fmt.Sprintf(
			"%s_LOG_STREAM",
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
	handler := slog.NewTextHandler(writer, options)
	slog.SetDefault(slog.New(handler))

	// check if CPU profiling should be enabled
	filename, ok := os.LookupEnv(
		fmt.Sprintf(
			"%s_CPU_PROFILE",
			strings.ReplaceAll(
				strings.ToUpper(
					path.Base(os.Args[0]),
				),
				"-",
				"_",
			),
		),
	)
	if ok && filename != "" {
		f, err := os.Create(path.Clean(filename))
		if err != nil {
			slog.Error("could not create CPU profile", "error", err)
		}
		cpuprof = f
		if err := pprof.StartCPUProfile(f); err != nil {
			slog.Error("could not start CPU profile", "error", err)
		}
	}

	// check if CPU profiling should be enabled
	filename, ok = os.LookupEnv(
		fmt.Sprintf(
			"%s_MEM_PROFILE",
			strings.ReplaceAll(
				strings.ToUpper(
					path.Base(os.Args[0]),
				),
				"-",
				"_",
			),
		),
	)
	if ok && filename != "" {
		f, err := os.Create(path.Clean(filename))
		if err != nil {
			slog.Error("could not create memory profile", "error", err)
		}
		memprof = f
	}
}

// cleanup is a function that is called when the program is exiting.
func cleanup() {
	if cpuprof != nil {
		defer cpuprof.Close()
		defer pprof.StopCPUProfile()
	}
	if memprof != nil {
		defer memprof.Close()
		runtime.GC() // get up-to-date statistics
		if err := pprof.WriteHeapProfile(memprof); err != nil {
			slog.Error("could not write memory profile", "error", err)
		}
	}
}
