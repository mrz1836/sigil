package cli

import (
	"bytes"
	"slices"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mrz1836/sigil/internal/chain"
	"github.com/mrz1836/sigil/internal/service/address"
	"github.com/mrz1836/sigil/internal/service/discovery"
)

func TestGroupTargetsByChain(t *testing.T) {
	t.Parallel()

	targets := []refreshTarget{
		{address: "1bsvA", chainID: chain.BSV},
		{address: "1bsvB", chainID: chain.BSV},
		{address: "0xethA", chainID: chain.ETH},
		{address: "1btcA", chainID: chain.BTC},
		{address: "qbchA", chainID: chain.BCH}, // excluded (not MVP)
		{address: "ltcA", chainID: chain.LTC},  // excluded (not MVP)
	}

	grouped := groupTargetsByChain(targets)

	// BCH and LTC are dropped entirely.
	assert.NotContains(t, grouped, chain.BCH)
	assert.NotContains(t, grouped, chain.LTC)
	assert.Len(t, grouped, 3, "only BSV, ETH, BTC groups remain")

	// BSV keeps both of its addresses in order.
	require.Len(t, grouped[chain.BSV], 2)
	assert.Equal(t, "1bsvA", grouped[chain.BSV][0].address)
	assert.Equal(t, "1bsvB", grouped[chain.BSV][1].address)

	require.Len(t, grouped[chain.ETH], 1)
	assert.Equal(t, "0xethA", grouped[chain.ETH][0].address)
	require.Len(t, grouped[chain.BTC], 1)
	assert.Equal(t, "1btcA", grouped[chain.BTC][0].address)
}

func TestGroupTargetsByChain_Empty(t *testing.T) {
	t.Parallel()

	// No targets yields an empty (but non-nil) map.
	grouped := groupTargetsByChain(nil)
	assert.Empty(t, grouped)
}

func TestExtractAddresses(t *testing.T) {
	t.Parallel()

	t.Run("preserves order", func(t *testing.T) {
		t.Parallel()
		targets := []refreshTarget{
			{address: "addr1", chainID: chain.BSV},
			{address: "addr2", chainID: chain.ETH},
			{address: "addr3", chainID: chain.BTC},
		}
		assert.Equal(t, []string{"addr1", "addr2", "addr3"}, extractAddresses(targets))
	})

	t.Run("empty input yields empty slice", func(t *testing.T) {
		t.Parallel()
		// Returns a zero-length (non-nil) slice, not nil.
		got := extractAddresses(nil)
		assert.Empty(t, got)
		assert.NotNil(t, got)
	})
}

func TestDisplayRefreshProgress(t *testing.T) {
	t.Parallel()

	t.Run("short address, uppercased chain", func(t *testing.T) {
		t.Parallel()
		var buf bytes.Buffer
		displayRefreshProgress(&buf, []string{"1ShortAddr"}, chain.BSV)
		assert.Equal(t, "  Refreshing 1ShortAddr [BSV]...\n", buf.String())
	})

	t.Run("long address is truncated", func(t *testing.T) {
		t.Parallel()
		// 50-char address exceeds the 42-char display cap, so it is elided.
		long := "1BvBMSEYstWetqTFn5Au4m4GFg7xJaNVN2eeeeeeeeeeeeeeee"
		var buf bytes.Buffer
		displayRefreshProgress(&buf, []string{long}, chain.ETH)
		out := buf.String()
		assert.Contains(t, out, "[ETH]")
		assert.Contains(t, out, "...")
		assert.NotContains(t, out, long, "full address should not appear")
	})

	t.Run("multiple addresses each get a line", func(t *testing.T) {
		t.Parallel()
		var buf bytes.Buffer
		displayRefreshProgress(&buf, []string{"a1", "a2", "a3"}, chain.BTC)
		lines := bytes.Count(buf.Bytes(), []byte("\n"))
		assert.Equal(t, 3, lines)
	})
}

func TestConvertRefreshResults(t *testing.T) {
	t.Parallel()

	t.Run("only failures are converted", func(t *testing.T) {
		t.Parallel()
		results := []discovery.RefreshResult{
			{Address: "1ok", Success: true, Error: nil},
			{Address: "1bad", Success: false, Error: errTestError},
			{Address: "1ok2", Success: true, Error: nil},
			{Address: "1bad2", Success: false, Error: errReadError},
		}
		errs := convertRefreshResults(results)
		require.Len(t, errs, 2)
		assert.Equal(t, "1bad", errs[0].address)
		assert.Equal(t, errTestError, errs[0].err)
		assert.Equal(t, "1bad2", errs[1].address)
		assert.Equal(t, errReadError, errs[1].err)
	})

	t.Run("all success yields no errors", func(t *testing.T) {
		t.Parallel()
		results := []discovery.RefreshResult{
			{Address: "1ok", Success: true},
		}
		assert.Empty(t, convertRefreshResults(results))
	})
}

func TestCompareAddressInfo(t *testing.T) {
	t.Parallel()

	mk := func(c chain.ID, ty address.AddressType, idx uint32) address.AddressInfo {
		return address.AddressInfo{ChainID: c, Type: ty, Index: idx}
	}

	t.Run("chain is the primary sort key", func(t *testing.T) {
		t.Parallel()
		// "bsv" < "eth" lexicographically, regardless of type/index.
		assert.Negative(t, compareAddressInfo(mk(chain.BSV, address.Change, 99), mk(chain.ETH, address.Receive, 0)))
		assert.Positive(t, compareAddressInfo(mk(chain.ETH, address.Receive, 0), mk(chain.BSV, address.Change, 99)))
	})

	t.Run("type breaks ties within a chain", func(t *testing.T) {
		t.Parallel()
		// Receive (0) sorts before Change (1).
		assert.Negative(t, compareAddressInfo(mk(chain.BSV, address.Receive, 5), mk(chain.BSV, address.Change, 0)))
	})

	t.Run("index breaks ties within chain+type", func(t *testing.T) {
		t.Parallel()
		assert.Negative(t, compareAddressInfo(mk(chain.BSV, address.Receive, 0), mk(chain.BSV, address.Receive, 1)))
		assert.Zero(t, compareAddressInfo(mk(chain.BSV, address.Receive, 3), mk(chain.BSV, address.Receive, 3)))
	})

	t.Run("sorts a slice into chain,type,index order", func(t *testing.T) {
		t.Parallel()
		got := []address.AddressInfo{
			mk(chain.ETH, address.Receive, 0),
			mk(chain.BSV, address.Change, 1),
			mk(chain.BSV, address.Receive, 2),
			mk(chain.BSV, address.Receive, 0),
		}
		slices.SortFunc(got, compareAddressInfo)
		want := []address.AddressInfo{
			mk(chain.BSV, address.Receive, 0),
			mk(chain.BSV, address.Receive, 2),
			mk(chain.BSV, address.Change, 1),
			mk(chain.ETH, address.Receive, 0),
		}
		assert.Equal(t, want, got)
	})
}

func TestDisplayAddressesTextWide_MultiChain(t *testing.T) {
	t.Parallel()

	// Two chains, each with unconfirmed data, exercises the chain-separator branch.
	addresses := []address.AddressInfo{
		{ChainID: chain.BSV, Type: address.Receive, Index: 0, Address: "1bsvaddr", Balance: "1.5", Unconfirmed: "-0.5", HasActivity: true},
		{ChainID: chain.ETH, Type: address.Receive, Index: 0, Address: "0xethaddr", Balance: "2.0", Unconfirmed: "0.1", HasActivity: true},
	}

	var buf bytes.Buffer
	displayAddressesTextWide(&buf, addresses)
	out := buf.String()

	// Wide table shows both Confirmed and Unconfirmed columns.
	assert.Contains(t, out, "Confirmed")
	assert.Contains(t, out, "Unconfirmed")
	// Each chain gets a header and its rows.
	assert.Contains(t, out, "[BSV]")
	assert.Contains(t, out, "[ETH]")
	assert.Contains(t, out, "1bsvaddr")
	assert.Contains(t, out, "0xethaddr")
	assert.Contains(t, out, "-0.5")
	assert.Contains(t, out, "0.1")
}
