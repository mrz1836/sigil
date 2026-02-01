// Package utxostore provides persistent UTXO storage for Sigil wallets.
package utxostore

import (
	"fmt"
	"path/filepath"
	"sync"
	"time"

	"github.com/mrz1836/sigil/internal/chain"
)

const (
	// utxoFileName is the name of the UTXO storage file.
	utxoFileName = "utxos.json" //nolint:unused // Used in Phase 2

	// currentVersion is the current file format version.
	currentVersion = 1

	// filePermissions for utxos.json
	filePermissions = 0o600 //nolint:unused // Used in Phase 2
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
	Label          string   `json:"label,omitempty"` // User-defined label

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
	mu         sync.RWMutex //nolint:unused // Used in Phase 2
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

// filePath returns the full path to utxos.json
func (s *Store) filePath() string { //nolint:unused // Used in Phase 2
	return filepath.Join(s.walletPath, utxoFileName)
}
