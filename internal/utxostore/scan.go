package utxostore

import (
	"context"
	"fmt"
	"time"

	"github.com/mrz1836/sigil/internal/chain"
	"github.com/mrz1836/sigil/internal/wallet"
)

const (
	// DefaultGapLimit is the standard HD wallet gap limit.
	// Scanning stops after this many consecutive addresses with no UTXOs.
	DefaultGapLimit = 20
)

// ChainClient defines the interface for UTXO lookups.
type ChainClient interface {
	// ListUTXOs returns unspent transaction outputs for an address.
	ListUTXOs(ctx context.Context, address string) ([]chain.UTXO, error)
}

// ScanResult contains the results of a wallet scan.
type ScanResult struct {
	// AddressesScanned is the number of addresses checked.
	AddressesScanned int

	// UTXOsFound is the number of UTXOs discovered.
	UTXOsFound int

	// TotalBalance is the sum of all UTXO amounts in satoshis.
	TotalBalance uint64

	// Errors contains any errors encountered during scanning.
	// Scanning continues even if some addresses fail.
	Errors []error
}

// ScanWallet scans a wallet's addresses and stores discovered UTXOs.
// Uses gap limit: stops after DefaultGapLimit consecutive addresses with no UTXOs.
// This implements BIP44 address discovery.
func (s *Store) ScanWallet(ctx context.Context, w *wallet.Wallet, chainID chain.ID, client ChainClient) (*ScanResult, error) {
	addresses, ok := w.Addresses[chainID]
	if !ok || len(addresses) == 0 {
		return &ScanResult{}, nil
	}

	result := &ScanResult{}
	consecutiveEmpty := 0

	for _, addr := range addresses {
		if ctx.Err() != nil {
			return result, ctx.Err()
		}

		hasActivity := s.scanAddress(ctx, addr, chainID, client, result)
		consecutiveEmpty = s.updateGapCounter(consecutiveEmpty, hasActivity)

		if consecutiveEmpty >= DefaultGapLimit {
			break
		}
	}

	// Save after scan
	if err := s.Save(); err != nil {
		return result, fmt.Errorf("saving UTXOs: %w", err)
	}

	return result, nil
}

// scanAddress scans a single address and updates the result.
// Returns true if the address had activity (UTXOs found).
func (s *Store) scanAddress(ctx context.Context, addr wallet.Address, chainID chain.ID, client ChainClient, result *ScanResult) bool {
	result.AddressesScanned++

	// Fetch UTXOs from chain
	utxos, err := client.ListUTXOs(ctx, addr.Address)
	if err != nil {
		result.Errors = append(result.Errors, fmt.Errorf("address %s: %w", addr.Address, err))
		return false
	}

	// Track address metadata
	s.trackAddress(addr, chainID, len(utxos) > 0)

	if len(utxos) == 0 {
		return false
	}

	// Store UTXOs
	s.storeUTXOs(utxos, chainID, result)
	return true
}

// trackAddress records address metadata.
func (s *Store) trackAddress(addr wallet.Address, chainID chain.ID, hasActivity bool) {
	meta := &AddressMetadata{
		Address:        addr.Address,
		ChainID:        chainID,
		DerivationPath: addr.Path,
		Index:          addr.Index,
		LastScanned:    time.Now(),
		HasActivity:    hasActivity,
	}
	s.AddAddress(meta)
}

// storeUTXOs stores a slice of UTXOs and updates the result counters.
func (s *Store) storeUTXOs(utxos []chain.UTXO, chainID chain.ID, result *ScanResult) {
	for _, u := range utxos {
		stored := &StoredUTXO{
			ChainID:       chainID,
			TxID:          u.TxID,
			Vout:          u.Vout,
			Amount:        u.Amount,
			ScriptPubKey:  u.ScriptPubKey,
			Address:       u.Address,
			Confirmations: u.Confirmations,
			Spent:         false,
		}
		s.AddUTXO(stored)
		result.UTXOsFound++
		result.TotalBalance += u.Amount
	}
}

// updateGapCounter updates the consecutive empty address counter.
// Returns 0 if hasActivity is true, otherwise increments the counter.
func (s *Store) updateGapCounter(current int, hasActivity bool) int {
	if hasActivity {
		return 0
	}
	return current + 1
}
