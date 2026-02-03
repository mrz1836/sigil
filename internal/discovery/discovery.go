package discovery

import (
	"context"
	"fmt"
	"time"

	sigilerr "github.com/mrz1836/sigil/pkg/errors"
)

// Default scanning parameters.
const (
	// DefaultGapLimit is the standard HD wallet gap limit.
	// Scanning stops after this many consecutive empty addresses.
	DefaultGapLimit = 20

	// ExtendedGapLimit is used for the primary (most likely) path scheme.
	// Higher limit catches wallets with more historical addresses.
	ExtendedGapLimit = 50

	// DefaultMaxConcurrent limits parallel API requests.
	DefaultMaxConcurrent = 3

	// DefaultTimeout is the default context timeout for discovery operations.
	DefaultTimeout = 5 * time.Minute
)

// Errors specific to discovery operations.
var (
	// ErrNoFundsFound indicates no funds were discovered across any path scheme.
	ErrNoFundsFound = &sigilerr.SigilError{
		Code:     "NO_FUNDS_FOUND",
		Message:  "no funds discovered across any derivation path",
		ExitCode: sigilerr.ExitNotFound,
	}

	// ErrInvalidSeed indicates the provided seed is invalid.
	ErrInvalidSeed = &sigilerr.SigilError{
		Code:     "INVALID_SEED",
		Message:  "invalid seed for derivation",
		ExitCode: sigilerr.ExitInput,
	}

	// ErrScanCanceled indicates the scan was canceled by context.
	ErrScanCanceled = &sigilerr.SigilError{
		Code:     "SCAN_CANCELED",
		Message:  "discovery scan was canceled",
		ExitCode: sigilerr.ExitGeneral,
	}
)

// ProgressUpdate provides feedback during scanning operations.
type ProgressUpdate struct {
	// Phase indicates the current scanning phase.
	Phase string

	// SchemeName is the name of the path scheme being scanned.
	SchemeName string

	// AddressesScanned is the number of addresses checked so far.
	AddressesScanned int

	// UTXOsFound is the number of UTXOs discovered so far.
	UTXOsFound int

	// BalanceFound is the total balance discovered so far (in satoshis).
	BalanceFound uint64

	// CurrentAddress is the address currently being scanned.
	CurrentAddress string

	// Message provides additional context about the progress.
	Message string
}

// ProgressCallback is called during scanning to report progress.
type ProgressCallback func(ProgressUpdate)

// Options configures the discovery operation.
type Options struct {
	// GapLimit is the number of consecutive empty addresses before stopping.
	// Default: DefaultGapLimit (20).
	GapLimit int

	// ExtendedGapLimit is used for the primary path scheme.
	// Default: ExtendedGapLimit (50).
	ExtendedGapLimit int

	// PathSchemes defines which derivation paths to scan.
	// Default: DefaultSchemes().
	PathSchemes []PathScheme

	// CustomPaths allows scanning user-provided derivation paths.
	// Format: "m/44'/0'/0'/0/*" where * is replaced with indices.
	CustomPaths []string

	// Passphrases is a list of BIP39 passphrases to try.
	// Empty string is always tried first (no passphrase).
	Passphrases []string

	// MaxConcurrent limits parallel API requests.
	// Default: DefaultMaxConcurrent (3).
	MaxConcurrent int

	// ProgressCallback receives updates during scanning.
	ProgressCallback ProgressCallback

	// ScanChangeAddresses determines whether to scan change (internal) addresses.
	// Default: true.
	ScanChangeAddresses bool
}

// DefaultOptions returns options with sensible defaults.
func DefaultOptions() *Options {
	return &Options{
		GapLimit:            DefaultGapLimit,
		ExtendedGapLimit:    ExtendedGapLimit,
		PathSchemes:         DefaultSchemes(),
		MaxConcurrent:       DefaultMaxConcurrent,
		ScanChangeAddresses: true,
	}
}

// ErrInvalidGapLimit indicates the gap limit is invalid.
var ErrInvalidGapLimit = &sigilerr.SigilError{
	Code:     "INVALID_GAP_LIMIT",
	Message:  "gap limit must be positive",
	ExitCode: sigilerr.ExitInput,
}

// ErrInvalidMaxConcurrent indicates max concurrent is invalid.
var ErrInvalidMaxConcurrent = &sigilerr.SigilError{
	Code:     "INVALID_MAX_CONCURRENT",
	Message:  "max concurrent must be positive",
	ExitCode: sigilerr.ExitInput,
}

// Validate checks that the options are valid.
func (o *Options) Validate() error {
	if o.GapLimit <= 0 {
		return sigilerr.WithDetails(ErrInvalidGapLimit, map[string]string{"value": fmt.Sprintf("%d", o.GapLimit)})
	}
	if o.MaxConcurrent <= 0 {
		return sigilerr.WithDetails(ErrInvalidMaxConcurrent, map[string]string{"value": fmt.Sprintf("%d", o.MaxConcurrent)})
	}
	return nil
}

// DiscoveredAddress represents an address found with a non-zero balance.
type DiscoveredAddress struct {
	// Address is the BSV address string.
	Address string `json:"address"`

	// Path is the full derivation path used (e.g., "m/44'/236'/0'/0/5").
	Path string `json:"path"`

	// SchemeName identifies which path scheme was used.
	SchemeName string `json:"scheme_name"`

	// Balance is the confirmed balance in satoshis.
	Balance uint64 `json:"balance"`

	// UTXOCount is the number of unspent outputs at this address.
	UTXOCount int `json:"utxo_count"`

	// IsChange indicates if this is a change (internal) address.
	IsChange bool `json:"is_change,omitempty"`

	// Index is the address index within the derivation path.
	Index uint32 `json:"index"`

	// Account is the account number (typically 0).
	Account uint32 `json:"account"`

	// CoinType is the BIP44 coin type used.
	CoinType uint32 `json:"coin_type"`
}

// Result contains the complete discovery scan results.
type Result struct {
	// FoundAddresses maps scheme names to discovered addresses.
	FoundAddresses map[string][]DiscoveredAddress `json:"found_addresses"`

	// TotalBalance is the sum of all discovered balances in satoshis.
	TotalBalance uint64 `json:"total_balance"`

	// TotalUTXOs is the total number of UTXOs discovered.
	TotalUTXOs int `json:"total_utxos"`

	// SchemesScanned lists the path schemes that were scanned.
	SchemesScanned []string `json:"schemes_scanned"`

	// AddressesScanned is the total number of addresses checked.
	AddressesScanned int `json:"addresses_scanned"`

	// Duration is how long the scan took.
	Duration time.Duration `json:"duration_ms"`

	// Passphrase indicates if a BIP39 passphrase was used (empty if no passphrase).
	// The actual passphrase is never stored, only whether one was used.
	PassphraseUsed bool `json:"passphrase_used,omitempty"`

	// Errors contains non-fatal errors encountered during scanning.
	Errors []string `json:"errors,omitempty"`
}

// HasFunds returns true if any funds were discovered.
func (r *Result) HasFunds() bool {
	return r.TotalBalance > 0
}

// AllAddresses returns a flat list of all discovered addresses.
func (r *Result) AllAddresses() []DiscoveredAddress {
	// Count total addresses for preallocation
	total := 0
	for _, addresses := range r.FoundAddresses {
		total += len(addresses)
	}

	all := make([]DiscoveredAddress, 0, total)
	for _, addresses := range r.FoundAddresses {
		all = append(all, addresses...)
	}
	return all
}

// AddressesByScheme returns discovered addresses for a specific scheme.
func (r *Result) AddressesByScheme(schemeName string) []DiscoveredAddress {
	return r.FoundAddresses[schemeName]
}

// ChainClient defines the interface for blockchain queries during discovery.
// This allows for easy mocking in tests.
type ChainClient interface {
	// ListUTXOs returns unspent transaction outputs for an address.
	ListUTXOs(ctx context.Context, address string) ([]UTXO, error)

	// ValidateAddress checks if an address is valid.
	ValidateAddress(address string) error
}

// UTXO represents an unspent transaction output.
type UTXO struct {
	TxID          string `json:"tx_id"`
	Vout          uint32 `json:"vout"`
	Amount        uint64 `json:"amount"`
	ScriptPubKey  string `json:"script_pubkey,omitempty"`
	Address       string `json:"address"`
	Confirmations uint32 `json:"confirmations,omitempty"`
}

// KeyDeriver defines the interface for deriving keys from seeds.
// This allows for easy mocking in tests.
type KeyDeriver interface {
	// DeriveAddress derives an address for the given parameters.
	DeriveAddress(seed []byte, coinType, account, change, index uint32) (string, string, error)

	// DeriveLegacyAddress derives an address using legacy path (m/0'/index).
	DeriveLegacyAddress(seed []byte, index uint32) (string, string, error)
}
