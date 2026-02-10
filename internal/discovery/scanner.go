package discovery

import (
	"context"
	"fmt"
	"time"

	sigilerr "github.com/mrz1836/sigil/pkg/errors"
)

// Scanner performs multi-path wallet discovery.
type Scanner struct {
	client  ChainClient
	deriver KeyDeriver
	opts    *Options
	bulkOps BulkOperations // Optional bulk operations for efficiency
}

// BulkOperations defines the interface for bulk blockchain operations.
type BulkOperations interface {
	BulkAddressActivityCheck(ctx context.Context, addresses []string) ([]AddressActivity, error)
	BulkAddressUTXOFetch(ctx context.Context, addresses []string) ([]BulkUTXOResult, error)
}

// AddressActivity represents activity status for an address (from bulk operations).
type AddressActivity struct {
	Address    string
	HasHistory bool
	Error      error
}

// BulkUTXOResult represents UTXOs for an address (from bulk operations).
type BulkUTXOResult struct {
	Address          string
	ConfirmedUTXOs   []UTXO
	UnconfirmedUTXOs []UTXO
	Error            error
}

// NewScanner creates a new discovery scanner.
func NewScanner(client ChainClient, deriver KeyDeriver, opts *Options) *Scanner {
	if opts == nil {
		opts = DefaultOptions()
	}
	return &Scanner{
		client:  client,
		deriver: deriver,
		opts:    opts,
	}
}

// NewScannerWithBulk creates a new discovery scanner with bulk operations.
func NewScannerWithBulk(client ChainClient, deriver KeyDeriver, opts *Options, bulkOps BulkOperations) *Scanner {
	if opts == nil {
		opts = DefaultOptions()
	}
	return &Scanner{
		client:  client,
		deriver: deriver,
		opts:    opts,
		bulkOps: bulkOps,
	}
}

// Scan performs a complete discovery scan across all configured path schemes.
//
//nolint:gocognit,gocyclo // Multi-scheme scanning requires complexity for path iteration
func (s *Scanner) Scan(ctx context.Context, seed []byte) (*Result, error) {
	if err := s.opts.Validate(); err != nil {
		return nil, fmt.Errorf("invalid options: %w", err)
	}

	if len(seed) == 0 {
		return nil, ErrInvalidSeed
	}

	startTime := time.Now()
	result := &Result{
		FoundAddresses: make(map[string][]DiscoveredAddress),
	}

	// Sort schemes by priority
	schemes := SortByPriority(s.opts.PathSchemes)

	// Try with each passphrase (empty string first if no passphrases specified)
	passphrases := s.opts.Passphrases
	if len(passphrases) == 0 {
		passphrases = []string{""} // No passphrase
	}

	for _, passphrase := range passphrases {
		// For BIP39, the seed would need to be regenerated with each passphrase
		// This is handled at the caller level - seed passed here is already derived
		// with the correct passphrase. We just track if one was used.
		if passphrase != "" {
			result.PassphraseUsed = true
		}

		// Scan each scheme
		for i, scheme := range schemes {
			if ctx.Err() != nil {
				result.Errors = append(result.Errors, ErrScanCanceled.Error())
				break
			}

			// Use extended gap limit for first (highest priority) scheme
			gapLimit := s.opts.GapLimit
			if i == 0 {
				gapLimit = s.opts.ExtendedGapLimit
			}

			schemeResult, err := s.scanScheme(ctx, seed, scheme, gapLimit)
			if err != nil {
				result.Errors = append(result.Errors, fmt.Sprintf("%s: %v", scheme.Name, err))
				continue
			}

			// Merge results
			if len(schemeResult.addresses) > 0 {
				result.FoundAddresses[scheme.Name] = append(
					result.FoundAddresses[scheme.Name],
					schemeResult.addresses...,
				)
				result.TotalBalance += schemeResult.balance
				result.TotalUTXOs += schemeResult.utxoCount
			}

			result.SchemesScanned = append(result.SchemesScanned, scheme.Name)
			result.AddressesScanned += schemeResult.scanned
		}
	}

	result.Duration = time.Since(startTime)

	return result, nil
}

// schemeResult holds results from scanning a single scheme.
type schemeResult struct {
	addresses []DiscoveredAddress
	balance   uint64
	utxoCount int
	scanned   int
}

// scanScheme scans a single path scheme.
//
//nolint:gocognit,funcorder // Account and change iteration is necessary for BIP44 scanning; grouped with caller
func (s *Scanner) scanScheme(ctx context.Context, seed []byte, scheme PathScheme, gapLimit int) (*schemeResult, error) {
	result := &schemeResult{}

	s.reportProgress(ProgressUpdate{
		Phase:      "scanning",
		SchemeName: scheme.Name,
		Message:    fmt.Sprintf("Scanning %s paths...", scheme.Name),
	})

	// Scan each account
	for _, account := range scheme.Accounts {
		if ctx.Err() != nil {
			return result, ctx.Err()
		}

		// Scan external chain (receiving addresses)
		extResult, err := s.scanChain(ctx, seed, scheme, account, 0, gapLimit)
		if err != nil {
			return result, err
		}
		result.addresses = append(result.addresses, extResult.addresses...)
		result.balance += extResult.balance
		result.utxoCount += extResult.utxoCount
		result.scanned += extResult.scanned

		// Scan internal chain (change addresses) if requested
		if scheme.ScanChange && s.opts.ScanChangeAddresses {
			intResult, err := s.scanChain(ctx, seed, scheme, account, 1, gapLimit)
			if err != nil {
				return result, err
			}
			result.addresses = append(result.addresses, intResult.addresses...)
			result.balance += intResult.balance
			result.utxoCount += intResult.utxoCount
			result.scanned += intResult.scanned
		}
	}

	return result, nil
}

// scanChain scans a single chain (external or internal) within a scheme.
func (s *Scanner) scanChain(ctx context.Context, seed []byte, scheme PathScheme, account, change uint32, gapLimit int) (*schemeResult, error) {
	// Use bulk scanning if available
	if s.bulkOps != nil {
		return s.scanChainBulk(ctx, seed, scheme, account, change, gapLimit)
	}

	// Fall back to individual scanning
	return s.scanChainIndividual(ctx, seed, scheme, account, change, gapLimit)
}

// scanChainBulk scans a chain using bulk operations (three-phase approach).
//
//nolint:gocognit,gocyclo,nestif,funcorder // Three-phase bulk scanning requires complexity; grouped with scanChain
func (s *Scanner) scanChainBulk(ctx context.Context, seed []byte, scheme PathScheme, account, change uint32, gapLimit int) (*schemeResult, error) {
	result := &schemeResult{}
	consecutiveEmpty := 0
	index := uint32(0)

	// Phase 1: Quick activity detection in batches
	for consecutiveEmpty < gapLimit {
		if ctx.Err() != nil {
			return result, ctx.Err()
		}

		// Generate batch of addresses
		batchSize := 20 // Match bulk API limit
		addresses := make([]string, 0, batchSize)
		paths := make([]string, 0, batchSize)
		indices := make([]uint32, 0, batchSize)

		for i := 0; i < batchSize && consecutiveEmpty < gapLimit; i++ {
			var address, path string
			var err error

			if scheme.IsLegacy {
				address, path, err = s.deriver.DeriveLegacyAddress(seed, index)
			} else {
				address, path, err = s.deriver.DeriveAddress(seed, scheme.CoinType, account, change, index)
			}

			if err != nil {
				return result, fmt.Errorf("deriving address at index %d: %w", index, err)
			}

			addresses = append(addresses, address)
			paths = append(paths, path)
			indices = append(indices, index)
			index++
			result.scanned++
		}

		// Phase 2: Check activity for batch
		activities, err := s.bulkOps.BulkAddressActivityCheck(ctx, addresses)
		if err != nil {
			// Fall back to individual scanning for this batch
			for i, addr := range addresses {
				utxos, utxoErr := s.client.ListUTXOs(ctx, addr)
				if utxoErr == nil && len(utxos) > 0 {
					consecutiveEmpty = 0
					s.processFoundAddress(addr, paths[i], indices[i], scheme.Name, account, change, scheme.CoinType, utxos, result)
				} else {
					consecutiveEmpty++
				}
			}
			continue
		}

		// Collect active addresses
		activeAddresses := make([]string, 0)
		activeIndices := make([]int, 0)
		batchHasActivity := false

		for i, activity := range activities {
			if activity.HasHistory {
				activeAddresses = append(activeAddresses, activity.Address)
				activeIndices = append(activeIndices, i)
				batchHasActivity = true
			}
		}

		// Phase 3: Fetch UTXOs for active addresses
		if len(activeAddresses) > 0 {
			utxoResults, err := s.bulkOps.BulkAddressUTXOFetch(ctx, activeAddresses)
			if err != nil {
				// Fall back to individual fetching
				for _, addr := range activeAddresses {
					utxos, utxoErr := s.client.ListUTXOs(ctx, addr)
					if utxoErr == nil && len(utxos) > 0 {
						// Find original index
						originalIdx := -1
						for i, a := range addresses {
							if a == addr {
								originalIdx = i
								break
							}
						}
						if originalIdx >= 0 {
							consecutiveEmpty = 0
							s.processFoundAddress(addr, paths[originalIdx], indices[originalIdx], scheme.Name, account, change, scheme.CoinType, utxos, result)
						}
					}
				}
			} else {
				// Process bulk UTXO results
				for j, utxoResult := range utxoResults {
					if utxoResult.Error != nil {
						continue
					}

					originalIdx := activeIndices[j]
					allUTXOs := s.convertBulkUTXOs(utxoResult)

					if len(allUTXOs) > 0 {
						consecutiveEmpty = 0
						s.processFoundAddress(
							addresses[originalIdx],
							paths[originalIdx],
							indices[originalIdx],
							scheme.Name,
							account,
							change,
							scheme.CoinType,
							allUTXOs,
							result,
						)
					}
				}
			}
		}

		// Update consecutive empty counter for addresses without activity
		if !batchHasActivity {
			consecutiveEmpty += len(addresses)
		} else {
			// Reset if we found any activity in this batch
			if consecutiveEmpty > 0 {
				consecutiveEmpty = 0
			}
		}
	}

	return result, nil
}

// scanChainIndividual scans a chain using individual API calls (fallback).
//
//nolint:gocognit,funcorder // Address iteration with gap limit logic is inherently complex; grouped with caller
func (s *Scanner) scanChainIndividual(ctx context.Context, seed []byte, scheme PathScheme, account, change uint32, gapLimit int) (*schemeResult, error) {
	result := &schemeResult{}
	consecutiveEmpty := 0

	for index := uint32(0); consecutiveEmpty < gapLimit; index++ {
		if ctx.Err() != nil {
			return result, ctx.Err()
		}

		// Derive address
		var address, path string
		var err error

		if scheme.IsLegacy {
			address, path, err = s.deriver.DeriveLegacyAddress(seed, index)
		} else {
			address, path, err = s.deriver.DeriveAddress(seed, scheme.CoinType, account, change, index)
		}

		if err != nil {
			return result, fmt.Errorf("deriving address at index %d: %w", index, err)
		}

		result.scanned++

		// Report progress
		s.reportProgress(ProgressUpdate{
			Phase:            "scanning",
			SchemeName:       scheme.Name,
			AddressesScanned: result.scanned,
			UTXOsFound:       result.utxoCount,
			BalanceFound:     result.balance,
			CurrentAddress:   address,
		})

		// Query UTXOs
		utxos, err := s.client.ListUTXOs(ctx, address)
		if err != nil {
			// Log error but continue scanning
			s.reportProgress(ProgressUpdate{
				Phase:      "error",
				SchemeName: scheme.Name,
				Message:    fmt.Sprintf("Error scanning %s: %v", address, err),
			})
			consecutiveEmpty++
			continue
		}

		if len(utxos) == 0 {
			consecutiveEmpty++
			continue
		}

		// Found UTXOs - reset gap counter and record address
		consecutiveEmpty = 0
		s.processFoundAddress(address, path, index, scheme.Name, account, change, scheme.CoinType, utxos, result)
	}

	return result, nil
}

// processFoundAddress processes a discovered address with UTXOs.
//
//nolint:funcorder // Helper method grouped with scanners
func (s *Scanner) processFoundAddress(
	address, path string,
	index uint32,
	schemeName string,
	account, change, coinType uint32,
	utxos []UTXO,
	result *schemeResult,
) {
	var balance uint64
	for _, utxo := range utxos {
		balance += utxo.Amount
	}

	discovered := DiscoveredAddress{
		Address:    address,
		Path:       path,
		SchemeName: schemeName,
		Balance:    balance,
		UTXOCount:  len(utxos),
		IsChange:   change == 1,
		Index:      index,
		Account:    account,
		CoinType:   coinType,
	}

	result.addresses = append(result.addresses, discovered)
	result.balance += balance
	result.utxoCount += len(utxos)

	// Report found funds
	s.reportProgress(ProgressUpdate{
		Phase:            "found",
		SchemeName:       schemeName,
		AddressesScanned: result.scanned,
		UTXOsFound:       result.utxoCount,
		BalanceFound:     result.balance,
		CurrentAddress:   address,
		Message:          fmt.Sprintf("Found %d satoshis at %s", balance, address),
	})
}

// convertBulkUTXOs converts bulk UTXO results to standard UTXO format.
//
//nolint:funcorder // Helper method grouped with scanners
func (s *Scanner) convertBulkUTXOs(result BulkUTXOResult) []UTXO {
	utxos := make([]UTXO, 0, len(result.ConfirmedUTXOs)+len(result.UnconfirmedUTXOs))
	utxos = append(utxos, result.ConfirmedUTXOs...)
	utxos = append(utxos, result.UnconfirmedUTXOs...)
	return utxos
}

// reportProgress safely calls the progress callback if configured.
//
//nolint:funcorder // Helper method grouped with callers for readability
func (s *Scanner) reportProgress(update ProgressUpdate) {
	if s.opts.ProgressCallback != nil {
		s.opts.ProgressCallback(update)
	}
}

// ErrUnknownScheme indicates an unknown path scheme was requested.
var ErrUnknownScheme = &sigilerr.SigilError{
	Code:     "UNKNOWN_SCHEME",
	Message:  "unknown path scheme",
	ExitCode: sigilerr.ExitInput,
}

// ScanSingleScheme scans only a specific path scheme.
// Useful for targeted recovery when the user knows which wallet they used.
func (s *Scanner) ScanSingleScheme(ctx context.Context, seed []byte, schemeName string) (*Result, error) {
	scheme := SchemeByName(schemeName)
	if scheme == nil {
		return nil, sigilerr.WithDetails(ErrUnknownScheme, map[string]string{"scheme": schemeName})
	}

	startTime := time.Now()
	result := &Result{
		FoundAddresses: make(map[string][]DiscoveredAddress),
	}

	schemeResult, err := s.scanScheme(ctx, seed, *scheme, s.opts.ExtendedGapLimit)
	if err != nil {
		return nil, err
	}

	if len(schemeResult.addresses) > 0 {
		result.FoundAddresses[scheme.Name] = schemeResult.addresses
		result.TotalBalance = schemeResult.balance
		result.TotalUTXOs = schemeResult.utxoCount
	}

	result.SchemesScanned = []string{scheme.Name}
	result.AddressesScanned = schemeResult.scanned
	result.Duration = time.Since(startTime)

	return result, nil
}

// ScanCustomPath scans a user-provided derivation path pattern.
// The pattern should include a wildcard for the index, e.g., "m/44'/0'/0'/0/*".
func (s *Scanner) ScanCustomPath(ctx context.Context, seed []byte, pathPattern string, coinType uint32) (*Result, error) {
	// Create a custom scheme from the path pattern
	scheme := PathScheme{
		Name:       fmt.Sprintf("Custom (%s)", pathPattern),
		CoinType:   coinType,
		Purpose:    44,
		Accounts:   []uint32{0},
		ScanChange: false,
		Priority:   0,
	}

	startTime := time.Now()
	result := &Result{
		FoundAddresses: make(map[string][]DiscoveredAddress),
	}

	schemeResult, err := s.scanScheme(ctx, seed, scheme, s.opts.GapLimit)
	if err != nil {
		return nil, err
	}

	if len(schemeResult.addresses) > 0 {
		result.FoundAddresses[scheme.Name] = schemeResult.addresses
		result.TotalBalance = schemeResult.balance
		result.TotalUTXOs = schemeResult.utxoCount
	}

	result.SchemesScanned = []string{scheme.Name}
	result.AddressesScanned = schemeResult.scanned
	result.Duration = time.Since(startTime)

	return result, nil
}
