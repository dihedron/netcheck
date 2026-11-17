package version

import (
	"fmt"
	"log/slog"
	"os"
	"strings"

	"github.com/dihedron/netcheck/v2/pkg/metadata"
	flags "github.com/jessevdk/go-flags"
)

// Version is the command that prints information about the application
// or plugin to the console; it supports both compact and verbose mode.
type Version struct {
	// Verbose is the flag that indicates whether to print verbose information about the application.
	Verbose bool `short:"v" long:"verbose" description:"Print verbose information about the application."`
	// DotEnv is the optional path to the .env file.
	DotEnv *string `short:"D" long:"dotenv" description:"The path to the .env file." env:"NETCHECK_DOTENV"`
}

// Execute is the real implementation of the Version command.
func (cmd *Version) Execute(args []string) error {
	slog.Debug("running version command")
	if cmd.Verbose {
		metadata.PrintFull(os.Stdout)
		if cmd.DotEnv != nil {
			if err := metadata.LoadDotEnv(*cmd.DotEnv); err != nil {
				slog.Error("error loading .env file", "error", err)
			} else {
				slog.Info("successfully loaded .env file", "path", *cmd.DotEnv)
			}
			fmt.Printf("  - Environment               :\n")
			for _, env := range os.Environ() {
				if line, ok := relevantEnv(env); ok {
					fmt.Printf("    - %s\n", line)
				}
			}
		}
	} else {
		metadata.Print(os.Stdout)
	}
	slog.Debug("command done")
	return nil
}

// Run runs the command with args and returns the exit code: how a plugin, which
// has no other commands, answers "NAME version [-v]".
func Run(args []string) int {
	var cmd Version
	if _, err := flags.ParseArgs(&cmd, args); err != nil {
		if flags.WroteHelp(err) {
			return 0
		}
		return 2
	}
	if err := cmd.Execute(nil); err != nil {
		return 1
	}
	return 0
}

// relevantEnv reports whether an environment entry concerns the application
// (NETCHECK_* settings) and returns it for printing, with the values of secrets
// masked.
func relevantEnv(env string) (string, bool) {
	name, value, _ := strings.Cut(env, "=")
	if !strings.HasPrefix(name, "NETCHECK_") {
		return "", false
	}
	for _, secret := range []string{"PASSWORD", "SECRET", "TOKEN", "PASSCODE", "DSN"} {
		if strings.Contains(name, secret) {
			return name + "=********", true
		}
	}
	// userinfo in a URL may hold a password, even if the URL does not parse
	if strings.Contains(name, "URL") && strings.Contains(value, "@") {
		return name + "=********", true
	}
	return env, true
}
