package discovery

import (
	"time"

	"github.com/mrz1836/sigil/internal/chain"
)

// RefreshRequest specifies parameters for refreshing address balances and UTXOs.
type RefreshRequest struct {
	ChainID    chain.ID
	Addresses  []string
	Concurrent int           // Max concurrent refreshes (0 = sequential)
	Timeout    time.Duration // Per-address timeout
}

// RefreshResult contains the outcome of refreshing a single address.
type RefreshResult struct {
	Address string
	Success bool
	Error   error
}

// CheckRequest specifies parameters for checking an address for activity.
type CheckRequest struct {
	ChainID chain.ID
	Address string
	Timeout time.Duration
}

// CheckResult contains the outcome of checking an address.
type CheckResult struct {
	Address     string
	ChainID     chain.ID
	Balance     uint64 // Satoshis for UTXO chains, Wei for account chains
	UTXOs       []UTXO // Empty for account-based chains
	HasActivity bool
	Label       string
}

// UTXO represents a single unspent transaction output.
type UTXO struct {
	TxID          string
	Vout          uint32
	Amount        uint64
	Confirmations uint32
}
