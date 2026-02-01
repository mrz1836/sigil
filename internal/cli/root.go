// Package cli implements the Sigil command-line interface.
//
// This package provides two ways to access CLI state:
//  1. Global variables (legacy) - for backwards compatibility
//  2. Context-based access (recommended) - via GetCmdContext(cmd)
//
// The globals are initialized in PersistentPreRunE and cleaned up in
// PersistentPostRun. New code should prefer GetCmdContext(cmd) for better
// testability and explicit dependency passing.
//
//nolint:gochecknoglobals // Cobra CLI pattern requires package-level state
package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

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

	// Command context for dependency injection
	cmdCtx *CommandContext
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
	PersistentPreRunE: func(cmd *cobra.Command, _ []string) error {
		return initGlobals(cmd)
	},
	PersistentPostRun: func(_ *cobra.Command, _ []string) {
		cleanup()
	},
}

// Execute runs the root command.
func Execute() error {
	err := rootCmd.Execute()
	if err != nil {
		formatErr(err)
		return err
	}
	return nil
}

// formatErr prints the error with proper formatting.
func formatErr(err error) {
	format := output.FormatText
	if formatter != nil {
		format = formatter.Format()
	}
	if fmtErr := output.FormatError(os.Stderr, err, format); fmtErr != nil {
		fmt.Fprintf(os.Stderr, "Error: %v (formatting failed: %v)\n", err, fmtErr)
	}
}

// ExitCode returns the appropriate exit code for an error.
func ExitCode(err error) int {
	return sigilerr.ExitCode(err)
}

// initGlobals initializes global configuration, logger, and formatter.
//
//nolint:gocognit,gocyclo // Initialization logic requires multiple conditional branches
func initGlobals(cmd *cobra.Command) error {
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
		if os.IsNotExist(err) {
			// Expected case: no config file yet, use defaults
			cfg = config.Defaults()
			cfg.Home = home
		} else {
			// Unexpected error: log warning but continue with defaults
			fmt.Fprintf(os.Stderr, "Warning: failed to load config: %v\n", err)
			cfg = config.Defaults()
			cfg.Home = home
		}
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

	// Expand tilde in Home path if present
	if strings.HasPrefix(cfg.Home, "~/") {
		if userHome, homeErr := os.UserHomeDir(); homeErr == nil {
			cfg.Home = filepath.Join(userHome, cfg.Home[2:])
		}
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

	// Create command context
	cmdCtx = NewCommandContext(cfg, logger, formatter)

	// Also store in cobra context for context-based access
	// This allows commands to use GetCmdContext(cmd) instead of globals
	SetCmdContext(cmd, cmdCtx)

	// Initialize session manager for wallet authentication caching
	initSessionManager()

	return nil
}

// cleanup releases resources.
func cleanup() {
	if logger != nil {
		if closeErr := logger.Close(); closeErr != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to close logger: %v\n", closeErr)
		}
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

// Context returns the global command context.
func Context() *CommandContext {
	return cmdCtx
}

// Version information, set at build time.
//
//nolint:gochecknoglobals // Version info set at build time via ldflags
var (
	Version   = "dev"
	GitCommit = "unknown"
	BuildDate = "unknown"
)

// versionCmd shows version information.
//
//nolint:gochecknoglobals // Cobra CLI pattern requires package-level command variables
var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Show version information",
	Long:  `Display the version, build commit, and build date.`,
	Run: func(cmd *cobra.Command, _ []string) {
		if formatter != nil && formatter.Format() == output.FormatJSON {
			cmd.Println("{")
			cmd.Printf(`  "version": "%s",`+"\n", Version)
			cmd.Printf(`  "commit": "%s",`+"\n", GitCommit)
			cmd.Printf(`  "date": "%s"`+"\n", BuildDate)
			cmd.Println("}")
		} else {
			cmd.Printf("sigil version %s\n", Version)
			cmd.Printf("  commit: %s\n", GitCommit)
			cmd.Printf("  built:  %s\n", BuildDate)
		}
	},
}

//nolint:gochecknoinits // Cobra CLI pattern requires init for flag registration
func init() {
	rootCmd.AddCommand(versionCmd)
	rootCmd.PersistentFlags().StringVar(&homeDir, "home", "", "sigil data directory (default: ~/.sigil)")
	rootCmd.PersistentFlags().StringVarP(&outputFormat, "output", "o", "auto", "output format: text, json, auto")
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "enable verbose output")
}
