package cli

import (
	"github.com/mrz1836/sigil/internal/config"
	"github.com/mrz1836/sigil/internal/output"
)

// Compile-time interface checks.
var (
	_ ConfigProvider = (*config.Config)(nil)
	_ LogWriter      = (*config.Logger)(nil)
	_ FormatProvider = (*output.Formatter)(nil)
)

// ConfigProvider provides read access to configuration values.
// This interface enables mocking configuration in tests.
type ConfigProvider interface {
	// GetHome returns the sigil home directory path.
	GetHome() string

	// GetETHRPC returns the Ethereum RPC URL.
	GetETHRPC() string

	// GetETHFallbackRPCs returns the fallback Ethereum RPC URLs.
	GetETHFallbackRPCs() []string

	// GetETHProvider returns the ETH balance provider ("rpc" or "etherscan").
	GetETHProvider() string

	// GetETHEtherscanAPIKey returns the Etherscan API key.
	GetETHEtherscanAPIKey() string

	// GetBSVAPIKey returns the BSV API key.
	GetBSVAPIKey() string

	// GetLoggingLevel returns the configured logging level.
	GetLoggingLevel() string

	// GetLoggingFile returns the configured log file path.
	GetLoggingFile() string

	// GetOutputFormat returns the default output format.
	GetOutputFormat() string

	// IsVerbose returns true if verbose output is enabled.
	IsVerbose() bool

	// GetSecurity returns the security configuration.
	GetSecurity() config.SecurityConfig
}

// LogWriter provides logging capabilities.
// This interface enables mocking logging in tests.
type LogWriter interface {
	// Debug logs a debug-level message.
	Debug(format string, args ...any)

	// Error logs an error-level message.
	Error(format string, args ...any)

	// Close closes the logger and releases resources.
	Close() error
}

// FormatProvider provides output format information.
// This interface enables mocking output formatting in tests.
type FormatProvider interface {
	// Format returns the current output format.
	Format() output.Format
}
