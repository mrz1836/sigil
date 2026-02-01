// Package cli implements the Sigil command-line interface.
//
// This package uses global variables to manage CLI state, which is the standard
// pattern for Cobra-based CLI applications. The globals are initialized in
// PersistentPreRunE and cleaned up in PersistentPostRun.
//
//nolint:gochecknoglobals // Cobra CLI pattern requires package-level state
package cli

import (
	"os"

	"github.com/spf13/cobra"

	"github.com/mrz1836/sigil/internal/config"
	"github.com/mrz1836/sigil/internal/output"
	sigilerr "github.com/mrz1836/sigil/pkg/errors"
)

var (
	// Global flags
	homeDir      string
	outputFormat string
	verbose      bool

	// Global state initialized in PersistentPreRunE
	cfg       *config.Config
	logger    *config.Logger
	formatter *output.Formatter
)

// rootCmd is the base command when called without any subcommands.
var rootCmd = &cobra.Command{
	Use:   "sigil",
	Short: "A secure multi-chain wallet CLI",
	Long: `Sigil is a terminal-based multi-chain cryptocurrency wallet for power users.

It supports HD wallet creation with BIP39 mnemonics, balance checking, and
transactions across Ethereum (ETH/USDC) and Bitcoin SV (BSV) networks.

Example:
  sigil wallet create main --words 24
  sigil balance show --wallet main
  sigil tx send --wallet main --to 0x... --amount 0.1 --chain eth`,
	SilenceUsage:  true,
	SilenceErrors: true,
	PersistentPreRunE: func(_ *cobra.Command, _ []string) error {
		return initGlobals()
	},
	PersistentPostRun: func(_ *cobra.Command, _ []string) {
		cleanup()
	},
}

// Execute runs the root command.
func Execute() error {
	err := rootCmd.Execute()
	if err != nil {
		// Format and print error
		if formatter != nil {
			_ = output.FormatError(os.Stderr, err, formatter.Format())
		} else {
			_ = output.FormatError(os.Stderr, err, output.FormatText)
		}
		return err
	}
	return nil
}

// ExitCode returns the appropriate exit code for an error.
func ExitCode(err error) int {
	return sigilerr.ExitCode(err)
}

// initGlobals initializes global configuration, logger, and formatter.
func initGlobals() error {
	// Determine home directory
	home := homeDir
	if home == "" {
		home = os.Getenv(config.EnvHome)
	}
	if home == "" {
		home = config.DefaultHome()
	}

	// Load or create config
	configPath := config.Path(home)
	var err error
	cfg, err = config.Load(configPath)
	if err != nil {
		// Use defaults if config doesn't exist
		cfg = config.Defaults()
		cfg.Home = home
	}

	// Apply environment variable overrides
	config.ApplyEnvironment(cfg)

	// Override with command-line flags
	if homeDir != "" {
		cfg.Home = homeDir
	}
	if verbose {
		cfg.Output.Verbose = true
		cfg.Logging.Level = "debug"
	}
	if outputFormat != "" && outputFormat != "auto" {
		cfg.Output.DefaultFormat = outputFormat
	}

	// Initialize logger
	logLevel := config.ParseLogLevel(cfg.Logging.Level)
	logger, err = config.NewLogger(logLevel, cfg.Logging.File)
	if err != nil {
		// Use null logger if we can't create the file
		logger = config.NullLogger()
	}

	// Initialize formatter
	explicitFormat := output.ParseFormat(cfg.Output.DefaultFormat)
	detectedFormat := output.DetectFormat(os.Stdout, explicitFormat)
	formatter = output.NewFormatter(detectedFormat, os.Stdout)

	return nil
}

// cleanup releases resources.
func cleanup() {
	if logger != nil {
		_ = logger.Close()
	}
}

// Config returns the global configuration.
func Config() *config.Config {
	return cfg
}

// Logger returns the global logger.
func Logger() *config.Logger {
	return logger
}

// Formatter returns the global output formatter.
func Formatter() *output.Formatter {
	return formatter
}

//nolint:gochecknoinits // Cobra CLI pattern requires init for flag registration
func init() {
	rootCmd.PersistentFlags().StringVar(&homeDir, "home", "", "sigil data directory (default: ~/.sigil)")
	rootCmd.PersistentFlags().StringVarP(&outputFormat, "output", "o", "auto", "output format: text, json, auto")
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "enable verbose output")
}
