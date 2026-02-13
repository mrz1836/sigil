package balance

import (
	"time"

	"github.com/mrz1836/sigil/internal/chain"
)

// AddressInput represents an address to fetch balances for.
type AddressInput struct {
	ChainID chain.ID
	Address string
}

// BalanceEntry represents a balance for a single address (native or token).
// This is the framework-agnostic domain type returned by the service.
type BalanceEntry struct {
	Chain       chain.ID
	Address     string
	Balance     string
	Unconfirmed string
	Symbol      string
	Token       string
	Decimals    int
	Stale       bool
	UpdatedAt   time.Time
}

// FetchRequest represents a request to fetch balances for a single address.
type FetchRequest struct {
	ChainID      chain.ID
	Address      string
	ForceRefresh bool
	Timeout      time.Duration
}

// FetchResult represents the result of fetching balances for a single address.
type FetchResult struct {
	ChainID  chain.ID
	Address  string
	Balances []BalanceEntry
	Stale    bool
	Error    error
}

// FetchBatchRequest represents a request to fetch balances for multiple addresses concurrently.
type FetchBatchRequest struct {
	Addresses        []AddressInput
	ForceRefresh     bool
	MaxConcurrent    int
	Timeout          time.Duration
	ProgressCallback ProgressCallback
}

// FetchBatchResult represents the result of fetching balances for multiple addresses.
type FetchBatchResult struct {
	Results []*FetchResult
	Errors  []error
}

// ProgressUpdate provides feedback during balance fetching operations.
type ProgressUpdate struct {
	// Phase indicates the current phase: "building", "fetching_bsv", "fetching_eth"
	Phase string

	// TotalAddresses is the total number of addresses being processed
	TotalAddresses int

	// CompletedAddresses is the number of addresses completed so far
	CompletedAddresses int

	// ChainID is the chain currently being processed
	ChainID chain.ID

	// CurrentAddress is the address currently being fetched (for ETH)
	CurrentAddress string

	// Message provides additional context
	Message string
}

// ProgressCallback is called during batch fetching to report progress.
type ProgressCallback func(ProgressUpdate)

// CacheEntry is the service-layer representation of a cached balance.
// This decouples the service from the cache package implementation.
type CacheEntry struct {
	Chain       chain.ID
	Address     string
	Balance     string
	Unconfirmed string
	Symbol      string
	Token       string
	Decimals    int
	UpdatedAt   time.Time
}

// AddressMetadata contains metadata about an address for refresh policy decisions.
type AddressMetadata struct {
	ChainID     chain.ID
	Address     string
	HasActivity bool
	LastScanned time.Time
}
