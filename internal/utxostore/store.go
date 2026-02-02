// Package utxostore provides persistent UTXO storage for Sigil wallets.
package utxostore

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/mrz1836/sigil/internal/chain"
)

var (
	// ErrVersionTooNew is returned when the utxos.json version is newer than supported.
	ErrVersionTooNew = errors.New("utxos.json version is newer than supported")

	// ErrAddressNotFound is returned when an address is not found in the store.
	ErrAddressNotFound = errors.New("address not found")
)

const (
	// utxoFileName is the name of the UTXO storage file.
	utxoFileName = "utxos.json"

	// currentVersion is the current file format version.
	currentVersion = 1

	// filePermissions for utxos.json
	filePermissions = 0o600
)

// StoredUTXO represents a UTXO persisted to disk.
// It extends chain.UTXO with storage-specific fields.
type StoredUTXO struct {
	ChainID       chain.ID `json:"chain_id"`
	TxID          string   `json:"txid"`
	Vout          uint32   `json:"vout"`
	Amount        uint64   `json:"amount"` // satoshis
	ScriptPubKey  string   `json:"script"`
	Address       string   `json:"address"`
	Confirmations uint32   `json:"confirmations"`

	// Storage-specific fields
	Spent       bool      `json:"spent"`
	SpentTxID   string    `json:"spent_txid,omitempty"` // txid that spent this UTXO
	FirstSeen   time.Time `json:"first_seen"`
	LastUpdated time.Time `json:"last_updated"`
}

// Key returns the unique identifier for this UTXO (chainID:txid:vout)
func (u *StoredUTXO) Key() string {
	return fmt.Sprintf("%s:%s:%d", u.ChainID, u.TxID, u.Vout)
}

// AddressMetadata stores information about a derived address.
type AddressMetadata struct {
	Address        string   `json:"address"`
	ChainID        chain.ID `json:"chain_id"`
	DerivationPath string   `json:"derivation_path"`
	Index          uint32   `json:"index"`
	Label          string   `json:"label,omitempty"`     // User-defined label
	IsChange       bool     `json:"is_change,omitempty"` // True for change addresses (internal chain)

	// Scan state
	LastScanned time.Time `json:"last_scanned,omitempty"`
	HasActivity bool      `json:"has_activity"` // Has ever received funds
}

// Key returns the unique identifier for this address (chainID:address)
func (a *AddressMetadata) Key() string {
	return fmt.Sprintf("%s:%s", a.ChainID, a.Address)
}

// UTXOFile represents the JSON file structure (versioned).
type UTXOFile struct {
	Version   int                         `json:"version"`
	UpdatedAt time.Time                   `json:"updated_at"`
	UTXOs     map[string]*StoredUTXO      `json:"utxos"`     // key: chainID:txid:vout
	Addresses map[string]*AddressMetadata `json:"addresses"` // key: chainID:address
}

// Store manages UTXO persistence for a single wallet.
type Store struct {
	walletPath string
	mu         sync.RWMutex
	data       *UTXOFile
}

// New creates a new UTXOStore for the given wallet directory.
// The store is not loaded until Load() is called.
func New(walletPath string) *Store {
	return &Store{
		walletPath: walletPath,
		data: &UTXOFile{
			Version:   currentVersion,
			UpdatedAt: time.Now(),
			UTXOs:     make(map[string]*StoredUTXO),
			Addresses: make(map[string]*AddressMetadata),
		},
	}
}

// Load reads UTXOs from disk. Returns nil error if file doesn't exist (fresh wallet).
func (s *Store) Load() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	data, err := os.ReadFile(s.filePath())
	if err != nil {
		if os.IsNotExist(err) {
			// Fresh wallet, no UTXOs yet
			return nil
		}
		return fmt.Errorf("reading utxos.json: %w", err)
	}

	var file UTXOFile
	if err := json.Unmarshal(data, &file); err != nil {
		return fmt.Errorf("parsing utxos.json: %w", err)
	}

	// Version migration (future-proofing)
	if file.Version > currentVersion {
		return fmt.Errorf("%w: version %d (supported %d)", ErrVersionTooNew, file.Version, currentVersion)
	}

	s.data = &file
	return nil
}

// Save writes UTXOs to disk atomically.
func (s *Store) Save() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.data.UpdatedAt = time.Now()
	s.data.Version = currentVersion

	data, err := json.MarshalIndent(s.data, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling utxos: %w", err)
	}

	// Atomic write via temp file
	tempPath := s.filePath() + ".tmp"
	if err := os.WriteFile(tempPath, data, filePermissions); err != nil {
		return fmt.Errorf("writing temp file: %w", err)
	}

	if err := os.Rename(tempPath, s.filePath()); err != nil {
		_ = os.Remove(tempPath) // Best effort cleanup
		return fmt.Errorf("renaming temp file: %w", err)
	}

	return nil
}

// GetUTXOs returns unspent UTXOs for a chain and optional address filter.
// If address is empty, returns all unspent UTXOs for the chain.
func (s *Store) GetUTXOs(chainID chain.ID, address string) []*StoredUTXO {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var result []*StoredUTXO
	for _, utxo := range s.data.UTXOs {
		if utxo.ChainID != chainID || utxo.Spent {
			continue
		}
		if address != "" && utxo.Address != address {
			continue
		}
		result = append(result, utxo)
	}
	return result
}

// GetBalance returns total unspent balance for a chain.
func (s *Store) GetBalance(chainID chain.ID) uint64 {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var total uint64
	for _, utxo := range s.data.UTXOs {
		if utxo.ChainID == chainID && !utxo.Spent {
			total += utxo.Amount
		}
	}
	return total
}

// GetAddresses returns all tracked addresses for a chain.
func (s *Store) GetAddresses(chainID chain.ID) []*AddressMetadata {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var result []*AddressMetadata
	for _, addr := range s.data.Addresses {
		if addr.ChainID == chainID {
			result = append(result, addr)
		}
	}
	return result
}

// IsEmpty returns true if no UTXOs are stored.
func (s *Store) IsEmpty() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.data.UTXOs) == 0
}

// MarkSpent marks a UTXO as spent. The UTXO is preserved for history.
// Returns true if the UTXO was found and updated.
func (s *Store) MarkSpent(chainID chain.ID, txid string, vout uint32, spentTxID string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	key := fmt.Sprintf("%s:%s:%d", chainID, txid, vout)
	utxo, exists := s.data.UTXOs[key]
	if !exists {
		return false
	}

	utxo.Spent = true
	utxo.SpentTxID = spentTxID
	utxo.LastUpdated = time.Now()
	return true
}

// AddUTXO adds or updates a UTXO in the store.
func (s *Store) AddUTXO(utxo *StoredUTXO) {
	s.mu.Lock()
	defer s.mu.Unlock()

	utxo.LastUpdated = time.Now()
	if utxo.FirstSeen.IsZero() {
		utxo.FirstSeen = utxo.LastUpdated
	}

	s.data.UTXOs[utxo.Key()] = utxo
}

// AddAddress adds or updates address metadata.
func (s *Store) AddAddress(addr *AddressMetadata) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.data.Addresses[addr.Key()] = addr
}

// SetAddressLabel sets or updates the label for an address.
// Returns error if the address is not found.
func (s *Store) SetAddressLabel(chainID chain.ID, address, label string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	key := fmt.Sprintf("%s:%s", chainID, address)
	addr, exists := s.data.Addresses[key]
	if !exists {
		return fmt.Errorf("%w: %s", ErrAddressNotFound, address)
	}

	addr.Label = label
	return nil
}

// GetAddressBalance returns the total unspent balance for a specific address.
func (s *Store) GetAddressBalance(chainID chain.ID, address string) uint64 {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var total uint64
	for _, utxo := range s.data.UTXOs {
		if utxo.ChainID == chainID && utxo.Address == address && !utxo.Spent {
			total += utxo.Amount
		}
	}
	return total
}

// MarkAddressUsed marks an address as having activity (received funds).
func (s *Store) MarkAddressUsed(chainID chain.ID, address string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	key := fmt.Sprintf("%s:%s", chainID, address)
	if addr, exists := s.data.Addresses[key]; exists {
		addr.HasActivity = true
	}
}

// GetUnusedAddresses returns addresses that have never received funds.
func (s *Store) GetUnusedAddresses(chainID chain.ID) []*AddressMetadata {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var result []*AddressMetadata
	for _, addr := range s.data.Addresses {
		if addr.ChainID == chainID && !addr.HasActivity {
			result = append(result, addr)
		}
	}
	return result
}

// GetAddress returns address metadata by address string.
func (s *Store) GetAddress(chainID chain.ID, address string) *AddressMetadata {
	s.mu.RLock()
	defer s.mu.RUnlock()

	key := fmt.Sprintf("%s:%s", chainID, address)
	return s.data.Addresses[key]
}

// GetAddressesByLabel returns addresses matching the given label.
func (s *Store) GetAddressesByLabel(chainID chain.ID, label string) []*AddressMetadata {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var result []*AddressMetadata
	for _, addr := range s.data.Addresses {
		if addr.ChainID == chainID && addr.Label == label {
			result = append(result, addr)
		}
	}
	return result
}

// filePath returns the full path to utxos.json
func (s *Store) filePath() string {
	return filepath.Join(s.walletPath, utxoFileName)
}
