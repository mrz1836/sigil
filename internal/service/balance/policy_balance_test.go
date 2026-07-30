package balance

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/mrz1836/sigil/internal/chain"
	"github.com/mrz1836/sigil/internal/chain/eth"
)

// TestHasNonZeroBalance exercises both branches of hasNonZeroBalance: the native
// balance string check (all chains) and the ETH-only USDC token fallback.
func TestHasNonZeroBalance(t *testing.T) {
	t.Parallel()

	const addr = "0xabc"

	tests := []struct {
		name      string
		chainID   chain.ID
		nativeBal string
		usdcBal   string // "" means no USDC cache entry is present
		want      bool
	}{
		// Native-only branch (non-ETH chain): the native string drives the result.
		{name: "BSV zero string", chainID: chain.BSV, nativeBal: "0", want: false},
		{name: "BSV empty string", chainID: chain.BSV, nativeBal: "", want: false},
		{name: "BSV zero with decimals trims to empty", chainID: chain.BSV, nativeBal: "0.0", want: false},
		{name: "BSV whole plus fraction", chainID: chain.BSV, nativeBal: "1.5", want: true},
		{name: "BSV tiny fraction", chainID: chain.BSV, nativeBal: "0.00000001", want: true},
		// "-0" is treated as non-zero: TrimLeft(cutset "0.") never strips the '-'.
		{name: "BSV negative-zero quirk", chainID: chain.BSV, nativeBal: "-0", want: true},

		// ETH token branch: a zero native balance can still be non-zero via USDC.
		{name: "ETH zero native, non-zero USDC", chainID: chain.ETH, nativeBal: "0", usdcBal: "100.0", want: true},
		{name: "ETH zero native, zero USDC", chainID: chain.ETH, nativeBal: "0", usdcBal: "0", want: false},
		{name: "ETH zero native, no USDC entry", chainID: chain.ETH, nativeBal: "0.0", want: false},
		// Non-zero native short-circuits to true while still visiting the ETH branch.
		{name: "ETH non-zero native, zero USDC", chainID: chain.ETH, nativeBal: "2.0", usdcBal: "0", want: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			cache := newMockCacheProvider()
			if tt.usdcBal != "" {
				cache.Set(CacheEntry{
					Chain:     chain.ETH,
					Address:   addr,
					Token:     eth.USDCMainnet,
					Balance:   tt.usdcBal,
					Symbol:    "USDC",
					Decimals:  6,
					UpdatedAt: time.Now(),
				})
			}

			policy := &RefreshPolicy{cache: cache}
			got := policy.hasNonZeroBalance(tt.chainID, addr, &CacheEntry{Balance: tt.nativeBal})
			assert.Equal(t, tt.want, got)
		})
	}
}
