// Package main is the entry point for the Sigil CLI.
package main

import (
	"os"

	"github.com/mrz1836/sigil/internal/cli"
)

func main() {
	if err := cli.Execute(); err != nil {
		os.Exit(cli.ExitCode(err))
	}
}
