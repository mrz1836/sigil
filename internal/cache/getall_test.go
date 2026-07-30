package cache

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/mrz1836/sigil/internal/chain"
)

func TestBalanceCache_GetAllForAddress(t *testing.T) {
	t.Parallel()

	c := NewBalanceCache()
	// Same address on two chains, plus a different address.
	c.Set(BalanceCacheEntry{Chain: chain.ETH, Address: "0xABC", Balance: "1.0", Symbol: "ETH"})
	c.Set(BalanceCacheEntry{Chain: chain.BSV, Address: "0xABC", Balance: "2.0", Symbol: "BSV"})
	c.Set(BalanceCacheEntry{Chain: chain.BTC, Address: "0xDEF", Balance: "3.0", Symbol: "BTC"})

	got := c.GetAllForAddress("0xABC")
	assert.Len(t, got, 2, "both chain entries for the address are returned")
	symbols := map[string]bool{}
	for _, e := range got {
		assert.Equal(t, "0xABC", e.Address)
		symbols[e.Symbol] = true
	}
	assert.Equal(t, map[string]bool{"ETH": true, "BSV": true}, symbols)

	// A distinct address returns only its own entry.
	assert.Len(t, c.GetAllForAddress("0xDEF"), 1)

	// An unknown address returns nothing.
	assert.Empty(t, c.GetAllForAddress("0xNOPE"))
}
