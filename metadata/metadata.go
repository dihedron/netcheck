package metadata

import (
	"fmt"
	"io"
	"log/slog"
	"os"
	"path"
	"runtime/debug"
)

// NOTE: these variables are populated at compile time by using the -ldflags
// linker flag:
//   $> go build -ldflags "-X github.com/dihedron/snoop/metadata.GitHash=$(hash)"
// in order to get the package path to the GitHash variable to use in the
// linker flag, use the nm utility and look for the variable in the built
// application symbols, then use its path in the linker flag:
//   $> nm ./dist/linux/amd64/snoop | grep GitHash
//   00000000015db9c0 b github.com/dihedron/snoop/metadata.GitHash

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
	// GitCommit is the commit of this version of the application.
	GitCommit string
	// GitTime is the modification time associated with the Git commit.
	GitTime string
	// GitModified reports whether the repository had outstanding local changes at time of build.
	GitModified string
	// GoVersion is the version of the Go compiler used in the build process.
	GoVersion string
	// GoOS is the operating system used to build this application; it may differ
	// from that of the compiler in case of cross-compilation (GOOS).
	GoOS string
	// GoOS is the architecture used during the build of this application; it
	// may differ from that of the compiler in case of cross-compilation (GOARCH).
	GoArch string
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
	// RulesMkVersion is the version of the rules.mk file used in the build process.
	RulesMkVersion string
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
	fmt.Fprintf(writer,
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
	fmt.Printf("  - Name                      : %s\n", Name)
	fmt.Printf("  - Description               : %s\n", Description)
	fmt.Printf("  - Copyright                 : %s\n", Copyright)
	fmt.Printf("  - License                   : %s\n", License)
	fmt.Printf("  - License URL               : %s\n", LicenseURL)
	fmt.Printf("  - Major Version             : %s\n", VersionMajor)
	fmt.Printf("  - Minor Version             : %s\n", VersionMinor)
	fmt.Printf("  - Patch Version             : %s\n", VersionPatch)
	fmt.Printf("  - Vendor                    : %s\n", Vendor)
	fmt.Printf("  - Maintainer                : %s\n", Maintainer)
	fmt.Printf("  - Built on                  : %s\n", BuildTime)
	fmt.Printf("  - Rules.mk Version          : %s\n", RulesMkVersion)
	fmt.Printf("  - Compiler                  : %s\n", GoVersion)
	fmt.Printf("  - Operating System          : %s\n", GoOS)
	fmt.Printf("  - Architecture              : %s\n", GoArch)
	fmt.Printf("  - Git Time                  : %s\n", GitTime)
	fmt.Printf("  - Build Commit              : %s\n", GitCommit)
	fmt.Printf("  - Variable Name (.env)      : %s\n", DotEnvVarName)
}
