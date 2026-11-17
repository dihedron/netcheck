package metadata

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log/slog"
	"os"
	"path"
	"runtime/debug"

	"github.com/joho/godotenv"
)

// NOTE: these variables are populated at compile time by using the -ldflags
// linker flag:
//   $> go build -ldflags "-X github.com/dihedron/netcheck/v2/metadata.GitHash=$(hash)"
// in order to get the package path to the GitHash variable to use in the
// linker flag, use the nm utility and look for the variable in the built
// application symbols, then use its path in the linker flag:
//   $> nm ./dist/linux/amd64/netcheck-plugin | grep GitHash
//   00000000015db9c0 b github.com/dihedron/netcheck/v2/metadata.GitHash

var (
	// Name is the name of the application or plugin.
	Name string
	// Description is a one-liner description of the application or plugin.
	Description string
	// Copyright is the copyright clause of the application or plugin.
	Copyright string
	// License is the license under which the code is released.
	License string
	// LicenseURL is the URL at which the license is available.
	LicenseURL string
	// BuildTime is the time at which the application was built.
	BuildTime string
	// GitTag is the current Git tag (e.g. "1.0.3").
	GitTag string
	// GitBranch is the branch of this version of the application.
	GitBranch string
	// GitCommit is the commit of this version of the application.
	GitCommit string
	// GitTime is the modification time associated with the Git commit.
	GitTime string
	// GitTreeState reports the Git tree state.
	GitTreeState string
	// GoVersion is the version of the Go compiler used in the build process.
	GoVersion string
	// GoOS is the operating system used to build this application; it may differ
	// from that of the compiler in case of cross-compilation (GOOS).
	GoOS string
	// GoOS is the architecture used during the build of this application; it
	// may differ from that of the compiler in case of cross-compilation (GOARCH).
	GoArch string
	// Version is the complete version string of the application.
	Version = "0.0.0"
	// VersionMajor is the major version of the application.
	VersionMajor = "0"
	// VersionMinor is the minor version of the application.
	VersionMinor = "0"
	// VersionPatch is the patch or revision level of the application.
	VersionPatch = "0"
	// Vendor is the name of the vendor or organization responsible for the application.
	Vendor string
	// Maintainer is the name of the person or organization responsible for maintaining the application.
	Maintainer string
	// GoReleaserVersion is the version of goreleaser used in the build process.
	GoReleaserVersion string
	// DotEnvVarName is the name of the environment variable holding the name of the .env file for environment variables lookup.
	DotEnvVarName string
)

func init() {
	if Name == "" {
		Name = path.Base(os.Args[0])
	}

	bi, ok := debug.ReadBuildInfo()
	if !ok {
		slog.Error("no build info available")
		return
	}

	GoVersion = bi.GoVersion

	for _, setting := range bi.Settings {
		switch setting.Key {
		case "GOOS":
			GoOS = setting.Value
		case "GOARCH":
			GoArch = setting.Value
		}
	}
}

func Print(writer io.Writer) {
	_, _ = fmt.Fprintf(writer,
		"\n  %s v%s.%s.%s (%s - %s/%s) - %s - %s\n\n",
		path.Base(os.Args[0]),
		VersionMajor,
		VersionMinor,
		VersionPatch,
		GoVersion,
		GoOS,
		GoArch,
		Copyright,
		Description)
}

func PrintFull(writer io.Writer) {
	_, _ = fmt.Fprintf(writer, "  - Name                      : %s\n", Name)
	_, _ = fmt.Fprintf(writer, "  - Description               : %s\n", Description)
	_, _ = fmt.Fprintf(writer, "  - Copyright                 : %s\n", Copyright)
	_, _ = fmt.Fprintf(writer, "  - License                   : %s\n", License)
	_, _ = fmt.Fprintf(writer, "  - License URL               : %s\n", LicenseURL)
	_, _ = fmt.Fprintf(writer, "  - Version                   : %s\n", Version)
	_, _ = fmt.Fprintf(writer, "  - Major Version             : %s\n", VersionMajor)
	_, _ = fmt.Fprintf(writer, "  - Minor Version             : %s\n", VersionMinor)
	_, _ = fmt.Fprintf(writer, "  - Patch Version             : %s\n", VersionPatch)
	_, _ = fmt.Fprintf(writer, "  - Vendor                    : %s\n", Vendor)
	_, _ = fmt.Fprintf(writer, "  - Maintainer                : %s\n", Maintainer)
	_, _ = fmt.Fprintf(writer, "  - Built on                  : %s\n", BuildTime)
	_, _ = fmt.Fprintf(writer, "  - GoReleaser Version        : %s\n", GoReleaserVersion)
	_, _ = fmt.Fprintf(writer, "  - Compiler                  : %s\n", GoVersion)
	_, _ = fmt.Fprintf(writer, "  - Operating System          : %s\n", GoOS)
	_, _ = fmt.Fprintf(writer, "  - Architecture              : %s\n", GoArch)
	_, _ = fmt.Fprintf(writer, "  - Git Time                  : %s\n", GitTime)
	_, _ = fmt.Fprintf(writer, "  - Git Tag                   : %s\n", GitTag)
	_, _ = fmt.Fprintf(writer, "  - Git Branch                : %s\n", GitBranch)
	_, _ = fmt.Fprintf(writer, "  - Git Tree State            : %s\n", GitTreeState)
	_, _ = fmt.Fprintf(writer, "  - Build Commit              : %s\n", GitCommit)
	_, _ = fmt.Fprintf(writer, "  - Variable Name (.env)      : %s\n", DotEnvVarName)
}

// LoadDotEnv loads a .env file. A parse error from godotenv quotes the file
// from the bad line on, secrets included, so it never leaves this function.
func LoadDotEnv(filename string) error {
	err := godotenv.Load(filename)
	var pathErr *fs.PathError
	if err == nil || errors.As(err, &pathErr) {
		return err
	}
	return errors.New("the file is malformed (details withheld: they may quote secrets)")
}
