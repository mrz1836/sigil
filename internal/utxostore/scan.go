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

// BulkChainClient defines the interface for bulk UTXO operations.
type BulkChainClient interface {
	ChainClient
	BulkAddressUTXOFetch(ctx context.Context, addresses []string) ([]BulkUTXOResult, error)
}

// BulkUTXOResult represents UTXOs for an address from bulk operations.
type BulkUTXOResult struct {
	Address          string
	ConfirmedUTXOs   []chain.UTXO
	UnconfirmedUTXOs []chain.UTXO
	Error            error
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

// Refresh re-scans all known addresses and merges changes.
// New UTXOs are added; UTXOs no longer in chain response are marked spent.
func (s *Store) Refresh(ctx context.Context, chainID chain.ID, client ChainClient) (*ScanResult, error) {
	// Get addresses to scan (copy under lock)
	addresses := s.getAddressesForChain(chainID)
	if len(addresses) == 0 {
		return &ScanResult{}, nil
	}

	result := &ScanResult{}
	seenUTXOs := make(map[string]bool)

	// Scan all known addresses
	for _, addr := range addresses {
		if ctx.Err() != nil {
			return result, ctx.Err()
		}

		s.refreshAddress(ctx, addr, chainID, client, result, seenUTXOs)
	}

	// Mark UTXOs not seen in this scan as spent
	s.markMissingAsSpent(chainID, seenUTXOs)

	// Save changes
	if err := s.Save(); err != nil {
		return result, fmt.Errorf("saving UTXOs: %w", err)
	}

	return result, nil
}

// getAddressesForChain returns a copy of addresses for a chain.
func (s *Store) getAddressesForChain(chainID chain.ID) []*AddressMetadata {
	s.mu.RLock()
	defer s.mu.RUnlock()

	addresses := make([]*AddressMetadata, 0, len(s.data.Addresses))
	for _, addr := range s.data.Addresses {
		if addr.ChainID == chainID {
			addresses = append(addresses, addr)
		}
	}
	return addresses
}

// refreshAddress scans a single address during refresh and tracks seen UTXOs.
func (s *Store) refreshAddress(ctx context.Context, addr *AddressMetadata, chainID chain.ID, client ChainClient, result *ScanResult, seenUTXOs map[string]bool) {
	result.AddressesScanned++

	utxos, err := client.ListUTXOs(ctx, addr.Address)
	if err != nil {
		result.Errors = append(result.Errors, fmt.Errorf("address %s: %w", addr.Address, err))
		return
	}

	// Create a copy to avoid racing on the original pointer's fields.
	// The original addr came from getAddressesForChain which returns pointers
	// to internal data, so modifying it directly would race with other access.
	updatedAddr := &AddressMetadata{
		Address:        addr.Address,
		ChainID:        addr.ChainID,
		DerivationPath: addr.DerivationPath,
		Index:          addr.Index,
		Label:          addr.Label,
		LastScanned:    time.Now(),
		HasActivity:    addr.HasActivity || len(utxos) > 0,
	}
	s.AddAddress(updatedAddr)

	// Process and track UTXOs
	for _, u := range utxos {
		key := fmt.Sprintf("%s:%s:%d", chainID, u.TxID, u.Vout)
		seenUTXOs[key] = true

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

// RefreshAddress refreshes UTXOs for a single address.
// Returns the scan result for just that address.
func (s *Store) RefreshAddress(ctx context.Context, address string, chainID chain.ID, client ChainClient) (*ScanResult, error) {
	// Look up the address metadata
	addr := s.getAddressByString(address, chainID)
	if addr == nil {
		// Address not in store - create minimal metadata
		addr = &AddressMetadata{
			Address: address,
			ChainID: chainID,
		}
	}

	result := &ScanResult{}
	seenUTXOs := make(map[string]bool)

	// Refresh the single address
	s.refreshAddress(ctx, addr, chainID, client, result, seenUTXOs)

	// Mark UTXOs for this address that weren't seen as spent
	s.markAddressUTXOsAsSpent(address, chainID, seenUTXOs)

	// Save changes
	if err := s.Save(); err != nil {
		return result, fmt.Errorf("saving UTXOs: %w", err)
	}

	return result, nil
}

// getAddressByString looks up address metadata by address string.
func (s *Store) getAddressByString(address string, chainID chain.ID) *AddressMetadata {
	s.mu.RLock()
	defer s.mu.RUnlock()

	for _, addr := range s.data.Addresses {
		if addr.Address == address && addr.ChainID == chainID {
			return addr
		}
	}
	return nil
}

// markAddressUTXOsAsSpent marks UTXOs for a specific address not seen in the scan as spent.
func (s *Store) markAddressUTXOsAsSpent(address string, chainID chain.ID, seenUTXOs map[string]bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	for key, utxo := range s.data.UTXOs {
		if utxo.ChainID != chainID || utxo.Address != address || utxo.Spent {
			continue
		}
		if !seenUTXOs[key] {
			utxo.Spent = true
			utxo.LastUpdated = time.Now()
		}
	}
}

// markMissingAsSpent marks UTXOs not seen in the scan as spent.
func (s *Store) markMissingAsSpent(chainID chain.ID, seenUTXOs map[string]bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	for key, utxo := range s.data.UTXOs {
		if utxo.ChainID != chainID || utxo.Spent {
			continue
		}
		if !seenUTXOs[key] {
			utxo.Spent = true
			utxo.LastUpdated = time.Now()
		}
	}
}

// ScanWalletBulk scans a wallet's addresses using bulk operations.
// Significantly faster than ScanWallet for wallets with many addresses.
//
//nolint:gocognit,gocyclo // Bulk scanning logic inherently complex
func (s *Store) ScanWalletBulk(ctx context.Context, w *wallet.Wallet, chainID chain.ID, bulkClient BulkChainClient) (*ScanResult, error) {
	addresses, ok := w.Addresses[chainID]
	if !ok || len(addresses) == 0 {
		return &ScanResult{}, nil
	}

	result := &ScanResult{}
	consecutiveEmpty := 0

	// Extract address strings
	addrStrings := make([]string, 0, len(addresses))
	addrMap := make(map[string]wallet.Address)
	for _, addr := range addresses {
		addrStrings = append(addrStrings, addr.Address)
		addrMap[addr.Address] = addr
	}

	// Fetch UTXOs using bulk operations
	bulkResults, err := bulkClient.BulkAddressUTXOFetch(ctx, addrStrings)
	if err != nil {
		// Fall back to individual scanning
		return s.ScanWallet(ctx, w, chainID, bulkClient)
	}

	// Process bulk results
	for _, bulkResult := range bulkResults {
		if ctx.Err() != nil {
			return result, ctx.Err()
		}

		if bulkResult.Error != nil {
			result.Errors = append(result.Errors, fmt.Errorf("address %s: %w", bulkResult.Address, bulkResult.Error))
			consecutiveEmpty++
			continue
		}

		addr := addrMap[bulkResult.Address]
		result.AddressesScanned++

		// Combine confirmed and unconfirmed UTXOs
		allUTXOs := append(bulkResult.ConfirmedUTXOs, bulkResult.UnconfirmedUTXOs...)

		// Track address metadata
		s.trackAddress(addr, chainID, len(allUTXOs) > 0)

		if len(allUTXOs) == 0 {
			consecutiveEmpty++
		} else {
			consecutiveEmpty = 0
			// Store UTXOs
			s.storeUTXOs(allUTXOs, chainID, result)
		}

		// Check gap limit
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

// RefreshBulk re-scans addresses using bulk operations.
// Much faster than Refresh for many addresses.
//
//nolint:gocognit // Bulk refresh logic inherently complex
func (s *Store) RefreshBulk(ctx context.Context, chainID chain.ID, bulkClient BulkChainClient) (*ScanResult, error) {
	// Get addresses to scan (copy under lock)
	addresses := s.getAddressesForChain(chainID)
	if len(addresses) == 0 {
		return &ScanResult{}, nil
	}

	result := &ScanResult{}
	seenUTXOs := make(map[string]bool)

	// Extract address strings
	addrStrings := make([]string, len(addresses))
	addrMap := make(map[string]*AddressMetadata)
	for i, addr := range addresses {
		addrStrings[i] = addr.Address
		addrMap[addr.Address] = addr
	}

	// Fetch UTXOs using bulk operations
	bulkResults, err := bulkClient.BulkAddressUTXOFetch(ctx, addrStrings)
	if err != nil {
		// Fall back to individual refresh
		return s.Refresh(ctx, chainID, bulkClient)
	}

	// Process bulk results
	for _, bulkResult := range bulkResults {
		if ctx.Err() != nil {
			return result, ctx.Err()
		}

		if bulkResult.Error != nil {
			result.Errors = append(result.Errors, fmt.Errorf("address %s: %w", bulkResult.Address, bulkResult.Error))
			continue
		}

		addr := addrMap[bulkResult.Address]
		result.AddressesScanned++

		// Update address metadata
		updatedAddr := &AddressMetadata{
			Address:        addr.Address,
			ChainID:        addr.ChainID,
			DerivationPath: addr.DerivationPath,
			Index:          addr.Index,
			Label:          addr.Label,
			LastScanned:    time.Now(),
			HasActivity:    addr.HasActivity,
		}

		// Combine confirmed and unconfirmed UTXOs
		allUTXOs := append(bulkResult.ConfirmedUTXOs, bulkResult.UnconfirmedUTXOs...)

		if len(allUTXOs) > 0 {
			updatedAddr.HasActivity = true
		}

		s.AddAddress(updatedAddr)

		// Process and track UTXOs
		for _, u := range allUTXOs {
			key := fmt.Sprintf("%s:%s:%d", chainID, u.TxID, u.Vout)
			seenUTXOs[key] = true

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

	// Mark UTXOs not seen in this scan as spent
	s.markMissingAsSpent(chainID, seenUTXOs)

	// Save changes
	if err := s.Save(); err != nil {
		return result, fmt.Errorf("saving UTXOs: %w", err)
	}

	return result, nil
}
