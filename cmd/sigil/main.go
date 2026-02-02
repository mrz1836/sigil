// Package main is the entry point for the Sigil CLI.
package main

import (
	"os"

	"github.com/mrz1836/sigil/internal/cli"
)

// Build info variables set via ldflags during build.
//
//nolint:gochecknoglobals // Required for ldflags injection at build time
var (
	version   = "dev"
	commit    = "unknown"
	buildDate = "unknown"
)

func main() {
	if err := cli.Execute(cli.BuildInfo{
		Version: version,
		Commit:  commit,
		Date:    buildDate,
	}); err != nil {
		os.Exit(cli.ExitCode(err))
	}
}
