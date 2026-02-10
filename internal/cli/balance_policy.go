package cli

import (
	"time"

	"github.com/mrz1836/sigil/internal/cache"
	"github.com/mrz1836/sigil/internal/utxostore"
	"github.com/mrz1836/sigil/internal/wallet"
)

// RefreshPolicy determines when to fetch fresh balance data vs using cached data.
// It implements a tiered strategy based on address activity and cache age.
type RefreshPolicy struct {
	utxoStore *utxostore.Store
	cache     *cache.BalanceCache
}

// RefreshDecision indicates whether an address requires a fresh balance fetch.
type RefreshDecision int

const (
	// RefreshRequired means the address must be fetched fresh from the network.
	RefreshRequired RefreshDecision = iota
	// CacheOK means the cached balance is acceptable and no network fetch is needed.
	CacheOK
)

// Policy constants define cache tolerance thresholds for different address types.
const (
	// freshAddressWindow is the time period during which newly created addresses
	// are checked frequently (they're likely to receive initial funds).
	freshAddressWindow = 24 * time.Hour

	// mediumPriorityStaleness is the cache tolerance for inactive addresses
	// (addresses that have received funds before but currently have zero balance).
	mediumPriorityStaleness = 30 * time.Minute

	// lowPriorityStaleness is the cache tolerance for never-used addresses
	// (addresses that have never received any funds).
	lowPriorityStaleness = 2 * time.Hour
)

// NewRefreshPolicy creates a new refresh policy instance.
func NewRefreshPolicy(utxoStore *utxostore.Store, cache *cache.BalanceCache) *RefreshPolicy {
	return &RefreshPolicy{
		utxoStore: utxoStore,
		cache:     cache,
	}
}

// ShouldRefresh determines if an address needs a fresh balance fetch or if cached data is acceptable.
//
// The decision logic implements a tiered strategy:
//   - High Priority (Always Fresh): Active addresses with non-zero balance or newly created addresses
//   - Medium Priority (30min cache): Inactive addresses that were used before
//   - Low Priority (2hr cache): Never-used addresses
//
// This strategy balances freshness for important addresses with performance optimization
// for less active addresses, reducing API calls while maintaining accuracy.
func (p *RefreshPolicy) ShouldRefresh(chainID wallet.ChainID, address string) RefreshDecision {
	// Get cached balance and its age
	nativeEntry, nativeExists, nativeAge := p.cache.Get(chainID, address, "")

	// If no cache exists, we must fetch fresh
	if !nativeExists {
		return RefreshRequired
	}

	// Get address metadata from UTXO store
	addressMeta := p.utxoStore.GetAddress(chainID, address)

	// If no metadata exists, default to fresh fetch (safe fallback)
	if addressMeta == nil {
		return RefreshRequired
	}

	// Check if address is newly created - always fetch fresh
	if p.isNewlyCreatedAddress(addressMeta) {
		return RefreshRequired
	}

	// Check if address has non-zero balance (including tokens)
	hasBalance := p.hasNonZeroBalance(chainID, address, nativeEntry)

	// Determine refresh based on priority tier
	return p.shouldRefreshBasedOnPriority(addressMeta.HasActivity, hasBalance, nativeAge)
}

// isNewlyCreatedAddress checks if an address is newly created (within 24 hours).
func (p *RefreshPolicy) isNewlyCreatedAddress(addressMeta *utxostore.AddressMetadata) bool {
	return !addressMeta.LastScanned.IsZero() && time.Since(addressMeta.LastScanned) < freshAddressWindow
}

// hasNonZeroBalance checks if an address has a non-zero balance, including token balances.
func (p *RefreshPolicy) hasNonZeroBalance(chainID wallet.ChainID, address string, nativeEntry *cache.BalanceCacheEntry) bool {
	hasBalance := isNonZeroBalance(nativeEntry.Balance)

	// Check for token balances (ETH/USDC case)
	if chainID == wallet.ChainETH {
		usdcEntry, usdcExists, _ := p.cache.Get(chainID, address, "0xA0b86991c6218b36c1d19D4a2e9Eb0cE3606eB48")
		if usdcExists && isNonZeroBalance(usdcEntry.Balance) {
			hasBalance = true
		}
	}

	return hasBalance
}

// shouldRefreshBasedOnPriority determines if refresh is needed based on priority tier.
func (p *RefreshPolicy) shouldRefreshBasedOnPriority(hasActivity, hasBalance bool, cacheAge time.Duration) RefreshDecision {
	// High Priority: Active addresses with current balance (HasActivity=true AND Balance>0)
	// Always fetch fresh for addresses with current balance
	if hasActivity && hasBalance {
		return RefreshRequired
	}

	// Medium Priority: Inactive but used addresses (HasActivity=true AND Balance=0)
	// Use extended cache tolerance of 30 minutes
	if hasActivity && !hasBalance {
		if cacheAge < mediumPriorityStaleness {
			return CacheOK
		}
		return RefreshRequired
	}

	// Low Priority: Never-used addresses (HasActivity=false AND Balance=0)
	// Use maximum cache tolerance of 2 hours
	if !hasActivity && !hasBalance {
		if cacheAge < lowPriorityStaleness {
			return CacheOK
		}
		return RefreshRequired
	}

	// Default: refresh required
	return RefreshRequired
}
