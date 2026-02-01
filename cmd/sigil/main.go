// Package main is the entry point for the Sigil CLI.
package main

import (
	"os"

	"sigil/internal/cli"
)

func main() {
	if err := cli.Execute(); err != nil {
		os.Exit(1)
	}
}
