// Package transaction provides sweep functionality for consolidating UTXOs.
package transaction

import (
	"context"
	"fmt"

	"github.com/mrz1836/sigil/internal/chain"
	"github.com/mrz1836/sigil/internal/chain/bsv"
	"github.com/mrz1836/sigil/internal/wallet"
	sigilerr "github.com/mrz1836/sigil/pkg/errors"
)

// BSVClient defines the interface for BSV blockchain operations needed by sweep.
type BSVClient interface {
	ValidateAddress(address string) error
	ListUTXOs(ctx context.Context, address string) ([]bsv.UTXO, error)
	Send(ctx context.Context, req chain.SendRequest) (*chain.TransactionResult, error)
}

// BulkOperations defines the interface for bulk blockchain operations.
type BulkOperations interface {
	BulkAddressUTXOFetch(ctx context.Context, addresses []string) ([]bsv.BulkUTXOResult, error)
	BulkUTXOValidation(ctx context.Context, utxos []bsv.UTXO) ([]bsv.UTXOSpentStatus, error)
}

// SweepService provides validated multi-address UTXO consolidation.
type SweepService struct {
	client  BSVClient
	bulkOps BulkOperations
	logger  Logger
}

// SweepOptions configures the sweep operation.
type SweepOptions struct {
	// Destination is the address to sweep funds to.
	Destination string

	// Addresses are the source addresses to sweep from.
	Addresses []wallet.Address

	// Seed is used to derive private keys for signing.
	Seed []byte

	// ValidateUTXOs enables UTXO validation before building transaction.
	ValidateUTXOs bool

	// DryRun calculates the sweep without broadcasting.
	DryRun bool

	// FeeRate in satoshis per kilobyte.
	FeeRate uint64
}

// SweepResult contains the result of a sweep operation.
type SweepResult struct {
	// TotalInput is the sum of all input UTXOs.
	TotalInput uint64

	// Fee is the transaction fee.
	Fee uint64

	// NetAmount is the amount sent to destination (TotalInput - Fee).
	NetAmount uint64

	// UTXOsUsed is the number of UTXOs consolidated.
	UTXOsUsed int

	// AddressesUsed is the number of addresses swept from.
	AddressesUsed int

	// ValidatedUTXOs is the number of UTXOs validated (if validation enabled).
	ValidatedUTXOs int

	// SpentUTXOs is the number of UTXOs found to be already spent.
	SpentUTXOs int

	// TxID is the transaction hash (empty for dry run).
	TxID string

	// TxHex is the raw transaction hex (for dry run inspection).
	TxHex string

	// DryRun indicates this was a dry run (no broadcast).
	DryRun bool
}

// Logger is the interface for sweep logging.
type Logger interface {
	Debug(format string, args ...any)
	Error(format string, args ...any)
}

// NewSweepService creates a new sweep service.
func NewSweepService(client BSVClient, bulkOps BulkOperations, logger Logger) *SweepService {
	return &SweepService{
		client:  client,
		bulkOps: bulkOps,
		logger:  logger,
	}
}

// Sweep consolidates all UTXOs from multiple addresses into a single address.
//
//nolint:gocognit,gocyclo,nestif // Sweep logic with validation inherently complex
func (s *SweepService) Sweep(ctx context.Context, opts *SweepOptions) (*SweepResult, error) {
	if opts == nil {
		return nil, sigilerr.WithDetails(sigilerr.ErrInvalidInput, map[string]string{
			"reason": "sweep options required",
		})
	}

	if err := s.validateOptions(opts); err != nil {
		return nil, err
	}

	s.debug("sweep: collecting UTXOs from %d addresses", len(opts.Addresses))

	// Collect all UTXOs from all addresses
	allUTXOs, err := s.collectUTXOs(ctx, opts.Addresses)
	if err != nil {
		return nil, fmt.Errorf("collecting UTXOs: %w", err)
	}

	if len(allUTXOs) == 0 {
		return nil, sigilerr.WithSuggestion(
			sigilerr.ErrInsufficientFunds,
			"no UTXOs found across any address",
		)
	}

	s.debug("sweep: collected %d UTXOs", len(allUTXOs))

	result := &SweepResult{
		UTXOsUsed:     len(allUTXOs),
		AddressesUsed: s.countUniqueAddresses(allUTXOs),
		DryRun:        opts.DryRun,
	}

	// Validate UTXOs if requested
	if opts.ValidateUTXOs && s.bulkOps != nil {
		validatedUTXOs, validationErr := s.validateAndFilterUTXOs(ctx, allUTXOs)
		if validationErr != nil {
			s.logError("UTXO validation failed: %v", validationErr)
			// Continue with unvalidated UTXOs
		} else {
			result.ValidatedUTXOs = len(allUTXOs)
			result.SpentUTXOs = len(allUTXOs) - len(validatedUTXOs)
			allUTXOs = validatedUTXOs

			if len(allUTXOs) == 0 {
				return nil, sigilerr.WithSuggestion(
					sigilerr.ErrInsufficientFunds,
					"all UTXOs have been spent",
				)
			}

			// Update counts after filtering spent UTXOs
			result.UTXOsUsed = len(allUTXOs)
			result.AddressesUsed = s.countUniqueAddresses(allUTXOs)

			s.debug("sweep: %d UTXOs validated, %d spent", result.ValidatedUTXOs, result.SpentUTXOs)
		}
	}

	// Calculate total input
	for _, utxo := range allUTXOs {
		result.TotalInput += utxo.Amount
	}

	// Calculate fee
	feeRate := opts.FeeRate
	if feeRate == 0 {
		feeRate = bsv.DefaultFeeRate
	}

	// Sweep transaction: N inputs, 1 output (no change)
	result.Fee = bsv.EstimateFeeForTx(len(allUTXOs), 1, feeRate)

	// Calculate net amount
	if result.TotalInput <= result.Fee {
		return nil, sigilerr.WithSuggestion(
			sigilerr.ErrInsufficientFunds,
			fmt.Sprintf("total input (%d sat) is less than fee (%d sat)", result.TotalInput, result.Fee),
		)
	}

	result.NetAmount = result.TotalInput - result.Fee

	// Check dust threshold
	if result.NetAmount < chain.BSV.DustLimit() {
		return nil, sigilerr.WithSuggestion(
			sigilerr.ErrInsufficientFunds,
			fmt.Sprintf("net amount (%d sat) is below dust threshold (%d sat)", result.NetAmount, chain.BSV.DustLimit()),
		)
	}

	s.debug("sweep: input=%d fee=%d net=%d", result.TotalInput, result.Fee, result.NetAmount)

	// Dry run: return without building transaction
	if opts.DryRun {
		return result, nil
	}

	// Derive private keys for all addresses
	privateKeys, err := deriveKeysForUTXOs(allUTXOs, opts.Addresses, opts.Seed)
	if err != nil {
		return nil, fmt.Errorf("deriving private keys: %w", err)
	}
	defer func() {
		for _, k := range privateKeys {
			wallet.ZeroBytes(k)
		}
	}()

	// Build and send sweep transaction
	sendReq := chain.SendRequest{
		To:          opts.Destination,
		Amount:      chain.AmountToBigInt(result.NetAmount),
		UTXOs:       allUTXOs,
		PrivateKeys: privateKeys,
		FeeRate:     feeRate,
		SweepAll:    true, // No change output
	}

	sendResult, err := s.client.Send(ctx, sendReq)
	if err != nil {
		s.logError("sweep transaction failed: %v", err)
		return nil, fmt.Errorf("sending sweep transaction: %w", err)
	}

	result.TxID = sendResult.Hash
	s.debug("sweep: broadcast successful, txid=%s", result.TxID)

	return result, nil
}

// validateOptions validates sweep options.
func (s *SweepService) validateOptions(opts *SweepOptions) error {
	if opts.Destination == "" {
		return sigilerr.WithDetails(sigilerr.ErrInvalidInput, map[string]string{
			"field": "destination",
		})
	}

	if s.client != nil {
		if err := s.client.ValidateAddress(opts.Destination); err != nil {
			return sigilerr.WithSuggestion(
				sigilerr.ErrInvalidAddress,
				fmt.Sprintf("invalid destination address: %s", opts.Destination),
			)
		}
	}

	if len(opts.Addresses) == 0 {
		return sigilerr.WithDetails(sigilerr.ErrInvalidInput, map[string]string{
			"field": "addresses",
		})
	}

	if len(opts.Seed) == 0 && !opts.DryRun {
		return sigilerr.WithDetails(sigilerr.ErrInvalidInput, map[string]string{
			"field": "seed",
		})
	}

	return nil
}

// collectUTXOs collects UTXOs from all addresses using bulk operations.
func (s *SweepService) collectUTXOs(ctx context.Context, addresses []wallet.Address) ([]chain.UTXO, error) {
	// Extract address strings
	addrStrings := make([]string, len(addresses))
	for i, addr := range addresses {
		addrStrings[i] = addr.Address
	}

	// Use bulk operations if available
	if s.bulkOps != nil {
		return s.collectUTXOsBulk(ctx, addrStrings)
	}

	// Fall back to individual collection
	return s.collectUTXOsIndividual(ctx, addrStrings)
}

// collectUTXOsBulk collects UTXOs using bulk operations.
func (s *SweepService) collectUTXOsBulk(ctx context.Context, addresses []string) ([]chain.UTXO, error) {
	results, err := s.bulkOps.BulkAddressUTXOFetch(ctx, addresses)
	if err != nil {
		s.logError("bulk UTXO fetch failed: %v", err)
		// Fall back to individual collection
		return s.collectUTXOsIndividual(ctx, addresses)
	}

	allUTXOs := make([]chain.UTXO, 0)

	for _, result := range results {
		if result.Error != nil {
			s.logError("UTXO fetch failed for %s: %v", result.Address, result.Error)
			continue
		}

		// Add confirmed UTXOs
		for _, u := range result.ConfirmedUTXOs {
			allUTXOs = append(allUTXOs, chain.UTXO{
				TxID:          u.TxID,
				Vout:          u.Vout,
				Amount:        u.Amount,
				ScriptPubKey:  u.ScriptPubKey,
				Address:       u.Address,
				Confirmations: u.Confirmations,
			})
		}

		// Add unconfirmed UTXOs
		for _, u := range result.UnconfirmedUTXOs {
			allUTXOs = append(allUTXOs, chain.UTXO{
				TxID:          u.TxID,
				Vout:          u.Vout,
				Amount:        u.Amount,
				ScriptPubKey:  u.ScriptPubKey,
				Address:       u.Address,
				Confirmations: 0,
			})
		}
	}

	return allUTXOs, nil
}

// collectUTXOsIndividual collects UTXOs using individual API calls.
func (s *SweepService) collectUTXOsIndividual(ctx context.Context, addresses []string) ([]chain.UTXO, error) {
	allUTXOs := make([]chain.UTXO, 0)

	for _, addr := range addresses {
		utxos, err := s.client.ListUTXOs(ctx, addr)
		if err != nil {
			s.logError("ListUTXOs failed for %s: %v", addr, err)
			continue
		}

		for _, u := range utxos {
			allUTXOs = append(allUTXOs, chain.UTXO{
				TxID:          u.TxID,
				Vout:          u.Vout,
				Amount:        u.Amount,
				ScriptPubKey:  u.ScriptPubKey,
				Address:       u.Address,
				Confirmations: u.Confirmations,
			})
		}
	}

	return allUTXOs, nil
}

// validateAndFilterUTXOs validates UTXOs and filters out spent ones.
func (s *SweepService) validateAndFilterUTXOs(ctx context.Context, utxos []chain.UTXO) ([]chain.UTXO, error) {
	// Convert to bsv.UTXO format
	bsvUTXOs := make([]bsv.UTXO, len(utxos))
	for i, u := range utxos {
		bsvUTXOs[i] = bsv.UTXO{
			TxID:          u.TxID,
			Vout:          u.Vout,
			Amount:        u.Amount,
			ScriptPubKey:  u.ScriptPubKey,
			Address:       u.Address,
			Confirmations: u.Confirmations,
		}
	}

	// Validate using bulk operations
	statuses, err := s.bulkOps.BulkUTXOValidation(ctx, bsvUTXOs)
	if err != nil {
		return nil, err
	}

	// Filter out spent UTXOs
	validUTXOs := make([]chain.UTXO, 0, len(utxos))
	for i, status := range statuses {
		if status.Error != nil {
			s.logError("validation failed for UTXO %s:%d: %v", status.TxID, status.Vout, status.Error)
			// Include UTXOs with validation errors (better than excluding valid ones)
			validUTXOs = append(validUTXOs, utxos[i])
			continue
		}

		if !status.Spent {
			validUTXOs = append(validUTXOs, utxos[i])
		}
	}

	return validUTXOs, nil
}

// countUniqueAddresses counts unique addresses in UTXO list.
func (s *SweepService) countUniqueAddresses(utxos []chain.UTXO) int {
	addrMap := make(map[string]bool)
	for _, u := range utxos {
		addrMap[u.Address] = true
	}
	return len(addrMap)
}

// debug logs a debug message if a logger is configured.
func (s *SweepService) debug(format string, args ...any) {
	if s.logger != nil {
		s.logger.Debug(format, args...)
	}
}

// logError logs an error message if a logger is configured.
func (s *SweepService) logError(format string, args ...any) {
	if s.logger != nil {
		s.logger.Error(format, args...)
	}
}
