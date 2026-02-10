package discovery

import (
	"context"
	"fmt"
	"time"

	"github.com/mrz1836/sigil/internal/chain/bsv"
	sigilerr "github.com/mrz1836/sigil/pkg/errors"
)

const (
	// RecoveryGapLimit is the extended gap limit for old wallet recovery.
	RecoveryGapLimit = 100

	// ExtendedRecoveryGapLimit is for very old wallets with large gaps.
	ExtendedRecoveryGapLimit = 200
)

// RecoveryScenarios provides specialized workflows for fund recovery.
type RecoveryScenarios struct {
	scanner *Scanner
	bulkOps *bsv.BulkOperations
	deriver KeyDeriver
	logger  Logger
}

// Logger is the interface for recovery logging.
type Logger interface {
	Debug(format string, args ...any)
	Error(format string, args ...any)
}

// NewRecoveryScenarios creates a new recovery scenarios handler.
func NewRecoveryScenarios(scanner *Scanner, bulkOps *bsv.BulkOperations, deriver KeyDeriver, logger Logger) *RecoveryScenarios {
	return &RecoveryScenarios{
		scanner: scanner,
		bulkOps: bulkOps,
		deriver: deriver,
		logger:  logger,
	}
}

// RecoveryMode defines the recovery strategy.
type RecoveryMode int

const (
	// RecoveryModeStandard uses default gap limits.
	RecoveryModeStandard RecoveryMode = iota

	// RecoveryModeExtended uses larger gap limits for old wallets.
	RecoveryModeExtended

	// RecoveryModeAggressive uses very large gap limits and all schemes.
	RecoveryModeAggressive
)

// RecoverOldWalletOptions configures old wallet recovery.
type RecoverOldWalletOptions struct {
	// Mode determines the recovery strategy.
	Mode RecoveryMode

	// CustomGapLimit overrides the mode's default gap limit.
	CustomGapLimit int

	// SpecificSchemes limits scanning to specific schemes (nil = all schemes).
	SpecificSchemes []string

	// ProgressCallback receives updates during recovery.
	ProgressCallback ProgressCallback

	// ScanChangeAddresses determines whether to scan change addresses.
	ScanChangeAddresses bool
}

// RecoverOldWallet performs extended gap limit scanning for old wallets.
// Uses bulk operations for efficiency.
func (r *RecoveryScenarios) RecoverOldWallet(ctx context.Context, seed []byte, opts *RecoverOldWalletOptions) (*Result, error) {
	if opts == nil {
		opts = &RecoverOldWalletOptions{
			Mode:                RecoveryModeStandard,
			ScanChangeAddresses: true,
		}
	}

	// Determine gap limit
	gapLimit := r.getGapLimitForMode(opts.Mode)
	if opts.CustomGapLimit > 0 {
		gapLimit = opts.CustomGapLimit
	}

	r.debug("recovering old wallet: mode=%d gapLimit=%d", opts.Mode, gapLimit)

	// Configure scanner options
	scanOpts := &Options{
		GapLimit:            gapLimit,
		ExtendedGapLimit:    gapLimit,
		PathSchemes:         r.getSchemes(opts.SpecificSchemes),
		ProgressCallback:    opts.ProgressCallback,
		ScanChangeAddresses: opts.ScanChangeAddresses,
		MaxConcurrent:       DefaultMaxConcurrent,
	}

	// Create scanner with recovery options
	recoveryScanner := NewScanner(r.scanner.client, r.deriver, scanOpts)

	// Perform scan
	result, err := recoveryScanner.Scan(ctx, seed)
	if err != nil {
		return nil, fmt.Errorf("recovery scan: %w", err)
	}

	r.debug("recovery complete: %d addresses found, %d satoshis", len(result.AllAddresses()), result.TotalBalance)

	return result, nil
}

// RecoverBeyondGapOptions configures manual range scanning.
type RecoverBeyondGapOptions struct {
	// DerivationPath is the path pattern (e.g., "m/44'/236'/0'/0/*").
	DerivationPath string

	// CoinType for the derivation path.
	CoinType uint32

	// StartIndex is the first address index to scan.
	StartIndex uint32

	// Count is the number of addresses to scan.
	Count int

	// ProgressCallback receives updates during recovery.
	ProgressCallback ProgressCallback
}

// RecoverBeyondGap scans a specific range of addresses beyond the gap limit.
// Useful when funds are known to exist at specific indices.
//
//nolint:gocognit,gocyclo,nestif,gosec // Recovery logic inherently complex; G115 false positive for index conversion
func (r *RecoveryScenarios) RecoverBeyondGap(ctx context.Context, seed []byte, opts *RecoverBeyondGapOptions) (*Result, error) {
	if opts == nil {
		return nil, sigilerr.WithDetails(sigilerr.ErrInvalidInput, map[string]string{
			"reason": "options required for beyond-gap recovery",
		})
	}

	if opts.Count <= 0 {
		return nil, sigilerr.WithDetails(sigilerr.ErrInvalidInput, map[string]string{
			"reason": "count must be positive",
		})
	}

	r.debug("scanning beyond gap: path=%s start=%d count=%d", opts.DerivationPath, opts.StartIndex, opts.Count)

	startTime := time.Now()
	result := &Result{
		FoundAddresses: make(map[string][]DiscoveredAddress),
	}

	// Generate addresses in the specified range
	addresses := make([]string, opts.Count)
	addressPaths := make([]string, opts.Count)

	for i := 0; i < opts.Count; i++ {
		index := opts.StartIndex + uint32(i)

		// Derive address (using m/44'/coinType/0'/0/index pattern)
		addr, path, err := r.deriver.DeriveAddress(seed, opts.CoinType, 0, 0, index)
		if err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("index %d: %v", index, err))
			continue
		}

		addresses[i] = addr
		addressPaths[i] = path
		result.AddressesScanned++

		if opts.ProgressCallback != nil {
			opts.ProgressCallback(ProgressUpdate{
				Phase:            "scanning",
				SchemeName:       "Custom Range",
				AddressesScanned: result.AddressesScanned,
				CurrentAddress:   addr,
			})
		}
	}

	// Use bulk operations to check activity
	if r.bulkOps != nil {
		activities, err := r.bulkOps.BulkAddressActivityCheck(ctx, addresses)
		if err != nil {
			r.logError("bulk activity check failed: %v", err)
			// Fall back to individual checks
			return r.scanRangeIndividually(ctx, seed, opts)
		}

		// Collect active addresses
		activeAddresses := make([]string, 0)
		activeIndices := make([]int, 0)
		for i, activity := range activities {
			if activity.HasHistory {
				activeAddresses = append(activeAddresses, activity.Address)
				activeIndices = append(activeIndices, i)
			}
		}

		if len(activeAddresses) > 0 {
			// Fetch UTXOs for active addresses
			utxoResults, err := r.bulkOps.BulkAddressUTXOFetch(ctx, activeAddresses)
			if err != nil {
				r.logError("bulk UTXO fetch failed: %v", err)
			} else {
				// Process results
				for j, utxoResult := range utxoResults {
					if utxoResult.Error != nil {
						continue
					}

					originalIndex := activeIndices[j]
					totalUTXOs := len(utxoResult.ConfirmedUTXOs) + len(utxoResult.UnconfirmedUTXOs)

					if totalUTXOs > 0 {
						var balance uint64
						for _, u := range utxoResult.ConfirmedUTXOs {
							balance += u.Amount
						}
						for _, u := range utxoResult.UnconfirmedUTXOs {
							balance += u.Amount
						}

						discovered := DiscoveredAddress{
							Address:    utxoResult.Address,
							Path:       addressPaths[originalIndex],
							SchemeName: "Custom Range",
							Balance:    balance,
							UTXOCount:  totalUTXOs,
							Index:      opts.StartIndex + uint32(originalIndex),
							CoinType:   opts.CoinType,
						}

						result.FoundAddresses["Custom Range"] = append(
							result.FoundAddresses["Custom Range"],
							discovered,
						)
						result.TotalBalance += balance
						result.TotalUTXOs += totalUTXOs

						if opts.ProgressCallback != nil {
							opts.ProgressCallback(ProgressUpdate{
								Phase:        "found",
								SchemeName:   "Custom Range",
								BalanceFound: balance,
								UTXOsFound:   totalUTXOs,
								Message:      fmt.Sprintf("Found %d satoshis at index %d", balance, discovered.Index),
							})
						}
					}
				}
			}
		}
	} else {
		// No bulk operations available, fall back to individual scanning
		return r.scanRangeIndividually(ctx, seed, opts)
	}

	result.SchemesScanned = []string{"Custom Range"}
	result.Duration = time.Since(startTime)

	return result, nil
}

// scanRangeIndividually scans a range using individual API calls (fallback).
func (r *RecoveryScenarios) scanRangeIndividually(ctx context.Context, seed []byte, opts *RecoverBeyondGapOptions) (*Result, error) {
	// Create a custom scheme for this range
	scheme := PathScheme{
		Name:     "Custom Range",
		CoinType: opts.CoinType,
		Purpose:  44,
		Accounts: []uint32{0},
	}

	scanOpts := &Options{
		GapLimit:         opts.Count, // Scan the full range
		ExtendedGapLimit: opts.Count,
		PathSchemes:      []PathScheme{scheme},
		ProgressCallback: opts.ProgressCallback,
		MaxConcurrent:    DefaultMaxConcurrent,
	}

	scanner := NewScanner(r.scanner.client, r.deriver, scanOpts)
	return scanner.Scan(ctx, seed)
}

// ValidateAndRefreshCacheOptions configures cache validation.
type ValidateAndRefreshCacheOptions struct {
	// ChainID to validate (required).
	ChainID string

	// ForceRefresh forces re-fetching all UTXOs even if valid.
	ForceRefresh bool

	// ProgressCallback receives updates during validation.
	ProgressCallback func(validated, total int, spent int)
}

// getGapLimitForMode returns the gap limit for a recovery mode.
func (r *RecoveryScenarios) getGapLimitForMode(mode RecoveryMode) int {
	switch mode {
	case RecoveryModeStandard:
		return DefaultGapLimit
	case RecoveryModeExtended:
		return RecoveryGapLimit
	case RecoveryModeAggressive:
		return ExtendedRecoveryGapLimit
	default:
		return DefaultGapLimit
	}
}

// getSchemes returns the schemes to scan based on options.
func (r *RecoveryScenarios) getSchemes(specificSchemes []string) []PathScheme {
	if len(specificSchemes) == 0 {
		return DefaultSchemes()
	}

	schemes := make([]PathScheme, 0, len(specificSchemes))
	for _, name := range specificSchemes {
		scheme := SchemeByName(name)
		if scheme != nil {
			schemes = append(schemes, *scheme)
		}
	}

	// If no valid schemes found, use defaults
	if len(schemes) == 0 {
		return DefaultSchemes()
	}

	return schemes
}

// debug logs a debug message if a logger is configured.
func (r *RecoveryScenarios) debug(format string, args ...any) {
	if r.logger != nil {
		r.logger.Debug(format, args...)
	}
}

// logError logs an error message if a logger is configured.
func (r *RecoveryScenarios) logError(format string, args ...any) {
	if r.logger != nil {
		r.logger.Error(format, args...)
	}
}
