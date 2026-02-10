package utxostore

import (
	"context"
	"fmt"
	"time"

	"github.com/mrz1836/sigil/internal/chain"
	"github.com/mrz1836/sigil/internal/chain/bsv"
)

// ValidationReport contains the results of UTXO validation.
type ValidationReport struct {
	// TotalChecked is the number of UTXOs validated.
	TotalChecked int

	// StillUnspent is the number of UTXOs confirmed as unspent.
	StillUnspent int

	// NowSpent is the number of UTXOs found to be spent.
	NowSpent int

	// ValidationErrors contains errors encountered during validation.
	ValidationErrors []error

	// Duration is how long the validation took.
	Duration time.Duration

	// SpentUTXOs is the list of UTXOs that are now spent.
	SpentUTXOs []StoredUTXO
}

// ReconcileReport contains the results of cache reconciliation.
type ReconcileReport struct {
	// AddressesScanned is the number of addresses checked.
	AddressesScanned int

	// NewUTXOs is the number of new UTXOs discovered.
	NewUTXOs int

	// RemovedUTXOs is the number of spent UTXOs removed from cache.
	RemovedUTXOs int

	// UpdatedBalance is the total balance change (can be negative).
	UpdatedBalance int64

	// Duration is how long the reconciliation took.
	Duration time.Duration

	// Errors contains any errors encountered.
	Errors []error
}

// BulkOperationsClient defines the interface for bulk blockchain operations.
type BulkOperationsClient interface {
	BulkUTXOValidation(ctx context.Context, utxos []bsv.UTXO) ([]bsv.UTXOSpentStatus, error)
	BulkAddressUTXOFetch(ctx context.Context, addresses []string) ([]bsv.BulkUTXOResult, error)
}

// ValidateUTXOs validates cached UTXOs using bulk operations.
// Returns a report detailing which UTXOs are spent.
//
//nolint:gocognit // Validation logic inherently complex
func (s *Store) ValidateUTXOs(ctx context.Context, chainID chain.ID, bulkClient BulkOperationsClient) (*ValidationReport, error) {
	start := time.Now()
	report := &ValidationReport{}

	// Get all unspent UTXOs for this chain
	utxos := s.getUnspentUTXOsForChain(chainID)
	if len(utxos) == 0 {
		report.Duration = time.Since(start)
		return report, nil
	}

	report.TotalChecked = len(utxos)

	// Convert to bsv.UTXO format for validation
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
	statuses, err := bulkClient.BulkUTXOValidation(ctx, bsvUTXOs)
	if err != nil {
		report.Duration = time.Since(start)
		report.ValidationErrors = append(report.ValidationErrors, err)
		return report, fmt.Errorf("bulk UTXO validation: %w", err)
	}

	// Process validation results
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, status := range statuses {
		if status.Error != nil {
			report.ValidationErrors = append(report.ValidationErrors, status.Error)
			continue
		}

		key := fmt.Sprintf("%s:%s:%d", chainID, status.TxID, status.Vout)

		if status.Spent {
			// Mark UTXO as spent
			if utxo, exists := s.data.UTXOs[key]; exists {
				utxo.Spent = true
				utxo.LastUpdated = time.Now()
				report.NowSpent++
				report.SpentUTXOs = append(report.SpentUTXOs, *utxo)
			}
		} else {
			// Update last validated timestamp
			if utxo, exists := s.data.UTXOs[key]; exists {
				utxo.LastUpdated = time.Now()
				report.StillUnspent++
			}
		}
	}

	// Save updated store
	if err := s.Save(); err != nil {
		report.Duration = time.Since(start)
		return report, fmt.Errorf("saving validated UTXOs: %w", err)
	}

	report.Duration = time.Since(start)
	return report, nil
}

// ReconcileWithChain syncs the local cache with the current chain state.
// Uses bulk operations to fetch current UTXOs for all known addresses.
//
//nolint:gocognit,gocyclo,gosec // Reconciliation logic inherently complex; G115 false positives for balance calculations
func (s *Store) ReconcileWithChain(ctx context.Context, chainID chain.ID, bulkClient BulkOperationsClient) (*ReconcileReport, error) {
	start := time.Now()
	report := &ReconcileReport{}

	// Get all addresses for this chain
	addresses := s.getAddressStringsForChain(chainID)
	if len(addresses) == 0 {
		report.Duration = time.Since(start)
		return report, nil
	}

	report.AddressesScanned = len(addresses)

	// Fetch current UTXOs using bulk operations
	results, err := bulkClient.BulkAddressUTXOFetch(ctx, addresses)
	if err != nil {
		report.Duration = time.Since(start)
		report.Errors = append(report.Errors, err)
		return report, fmt.Errorf("bulk UTXO fetch: %w", err)
	}

	// Track which UTXOs exist on chain
	chainUTXOs := make(map[string]bool)

	s.mu.Lock()
	defer s.mu.Unlock()

	// Process results and add/update UTXOs
	for _, result := range results {
		if result.Error != nil {
			report.Errors = append(report.Errors, result.Error)
			continue
		}

		// Process confirmed UTXOs
		for _, u := range result.ConfirmedUTXOs {
			key := fmt.Sprintf("%s:%s:%d", chainID, u.TxID, u.Vout)
			chainUTXOs[key] = true

			// Check if this is a new UTXO
			if _, exists := s.data.UTXOs[key]; !exists {
				s.data.UTXOs[key] = &StoredUTXO{
					ChainID:       chainID,
					TxID:          u.TxID,
					Vout:          u.Vout,
					Amount:        u.Amount,
					ScriptPubKey:  u.ScriptPubKey,
					Address:       u.Address,
					Confirmations: u.Confirmations,
					Spent:         false,
					LastUpdated:   time.Now(),
				}
				report.NewUTXOs++
				report.UpdatedBalance += int64(u.Amount)
			} else {
				// Update existing UTXO
				s.data.UTXOs[key].Confirmations = u.Confirmations
				s.data.UTXOs[key].LastUpdated = time.Now()
			}
		}

		// Process unconfirmed UTXOs
		for _, u := range result.UnconfirmedUTXOs {
			key := fmt.Sprintf("%s:%s:%d", chainID, u.TxID, u.Vout)
			chainUTXOs[key] = true

			// Check if this is a new UTXO
			if _, exists := s.data.UTXOs[key]; !exists {
				s.data.UTXOs[key] = &StoredUTXO{
					ChainID:       chainID,
					TxID:          u.TxID,
					Vout:          u.Vout,
					Amount:        u.Amount,
					ScriptPubKey:  u.ScriptPubKey,
					Address:       u.Address,
					Confirmations: 0,
					Spent:         false,
					LastUpdated:   time.Now(),
				}
				report.NewUTXOs++
				report.UpdatedBalance += int64(u.Amount)
			}
		}
	}

	// Mark UTXOs not seen on chain as spent
	for key, utxo := range s.data.UTXOs {
		if utxo.ChainID != chainID || utxo.Spent {
			continue
		}

		if !chainUTXOs[key] {
			utxo.Spent = true
			utxo.LastUpdated = time.Now()
			report.RemovedUTXOs++
			report.UpdatedBalance -= int64(utxo.Amount)
		}
	}

	// Save reconciled state
	if err := s.Save(); err != nil {
		report.Duration = time.Since(start)
		return report, fmt.Errorf("saving reconciled UTXOs: %w", err)
	}

	report.Duration = time.Since(start)
	return report, nil
}

// getUnspentUTXOsForChain returns all unspent UTXOs for a chain.
// Caller must NOT hold the lock.
func (s *Store) getUnspentUTXOsForChain(chainID chain.ID) []*StoredUTXO {
	s.mu.RLock()
	defer s.mu.RUnlock()

	utxos := make([]*StoredUTXO, 0)
	for _, utxo := range s.data.UTXOs {
		if utxo.ChainID == chainID && !utxo.Spent {
			utxos = append(utxos, utxo)
		}
	}
	return utxos
}

// getAddressStringsForChain returns all unique addresses for a chain.
// Caller must NOT hold the lock.
func (s *Store) getAddressStringsForChain(chainID chain.ID) []string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	addrMap := make(map[string]bool)
	for _, addr := range s.data.Addresses {
		if addr.ChainID == chainID {
			addrMap[addr.Address] = true
		}
	}

	addresses := make([]string, 0, len(addrMap))
	for addr := range addrMap {
		addresses = append(addresses, addr)
	}
	return addresses
}
