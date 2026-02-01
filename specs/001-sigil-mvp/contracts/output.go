// Package contracts defines the interface contracts for Sigil MVP.
// These are design artifacts - not compiled code.
// Actual implementations go in internal/output/

package contracts

import (
	"io"
)

// OutputFormat represents supported output formats.
type OutputFormat string

const (
	OutputFormatText OutputFormat = "text"
	OutputFormatJSON OutputFormat = "json"
	OutputFormatAuto OutputFormat = "auto"
)

// Formatter defines the interface for output formatting.
type Formatter interface {
	// Format returns the current output format.
	Format() OutputFormat

	// FormatWallet formats wallet information for display.
	FormatWallet(wallet *Wallet) string

	// FormatWalletList formats a list of wallets for display.
	FormatWalletList(wallets []WalletSummary) string

	// FormatBalance formats balance information for display.
	FormatBalance(response *BalanceResponse) string

	// FormatTransaction formats a transaction result for display.
	FormatTransaction(tx *TransactionResult) string

	// FormatUTXOList formats a list of UTXOs for display.
	FormatUTXOList(utxos []UTXO, chain string) string

	// FormatError formats an error with suggestions for display.
	FormatError(err error) string

	// FormatMnemonic formats a mnemonic phrase for secure display.
	FormatMnemonic(mnemonic string) string

	// FormatConfig formats configuration for display.
	FormatConfig(config interface{}) string
}

// FormatterFactory creates formatters based on output preferences.
type FormatterFactory interface {
	// Create returns a formatter for the given format and output writer.
	Create(format OutputFormat, w io.Writer) Formatter

	// DetectFormat determines the appropriate format based on context.
	// Returns JSON for non-TTY, text for TTY, unless explicitly overridden.
	DetectFormat(w io.Writer, explicit OutputFormat) OutputFormat
}

// BalanceResponse represents the balance query result for formatting.
type BalanceResponse struct {
	Wallet    string
	Balances  []BalanceEntry
	Timestamp string
	Stale     []StaleEntry // Entries that came from cache
}

// BalanceEntry represents a single balance in the response.
type BalanceEntry struct {
	Chain    string
	Address  string
	Balance  string
	Symbol   string
	Decimals int
	Token    string // Empty for native tokens
}

// StaleEntry represents a cached balance with staleness info.
type StaleEntry struct {
	Chain   string
	Address string
	Age     string // Human-readable age, e.g., "5 minutes"
}

// TableConfig defines table rendering options.
type TableConfig struct {
	// Headers are the column names.
	Headers []string

	// Rows are the data rows.
	Rows [][]string

	// NoHeader suppresses the header row.
	NoHeader bool

	// Separator is the column separator (default: spaces).
	Separator string
}

// TableRenderer renders tabular data for text output.
type TableRenderer interface {
	// Render renders a table to the writer.
	Render(w io.Writer, config TableConfig) error
}

// ErrorFormatter formats errors with context and suggestions.
type ErrorFormatter interface {
	// Format formats an error for display.
	// For text: multi-line with details and suggestion.
	// For JSON: structured error object.
	Format(err error) string

	// WithDetails adds details to an error.
	WithDetails(err error, details map[string]string) error

	// WithSuggestion adds an actionable suggestion to an error.
	WithSuggestion(err error, suggestion string) error
}

// ProgressReporter reports progress for long-running operations.
type ProgressReporter interface {
	// Start begins a progress indicator with the given message.
	Start(message string)

	// Update updates the progress message.
	Update(message string)

	// Done completes the progress indicator with success.
	Done(message string)

	// Fail completes the progress indicator with failure.
	Fail(message string)
}
