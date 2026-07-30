package wallet

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newDerivedWallet builds a wallet with `count` receive addresses derived for
// the given chains from the shared deterministic test seed.
func newDerivedWallet(t *testing.T, count int, chains ...ChainID) *Wallet {
	t.Helper()
	seed := getTestSeed(t)
	w, err := NewWallet("acc", chains)
	require.NoError(t, err)
	require.NoError(t, w.DeriveAddresses(seed, count))
	return w
}

func TestNewWallet(t *testing.T) {
	t.Parallel()

	t.Run("defaults enabled chains when empty", func(t *testing.T) {
		t.Parallel()
		w, err := NewWallet("main", nil)
		require.NoError(t, err)
		assert.Equal(t, []ChainID{ChainETH, ChainBSV, ChainBTC}, w.EnabledChains)
		assert.Equal(t, 1, w.Version)
		assert.NotNil(t, w.Addresses)
		assert.NotNil(t, w.ChangeAddresses)
		assert.Equal(t, 20, w.DerivationConfig.AddressGap)
		assert.NotNil(t, w.DerivationConfig.Paths)
	})

	t.Run("honors explicit enabled chains", func(t *testing.T) {
		t.Parallel()
		w, err := NewWallet("main", []ChainID{ChainBTC})
		require.NoError(t, err)
		assert.Equal(t, []ChainID{ChainBTC}, w.EnabledChains)
	})

	t.Run("rejects invalid name", func(t *testing.T) {
		t.Parallel()
		_, err := NewWallet("bad name!", nil)
		require.ErrorIs(t, err, ErrInvalidWalletName)
	})
}

func TestWallet_GetPrimaryAddress(t *testing.T) {
	t.Parallel()
	w := newDerivedWallet(t, 3, ChainETH, ChainBSV)

	addr, ok := w.GetPrimaryAddress(ChainETH)
	require.True(t, ok)
	assert.NotEmpty(t, addr)
	assert.Equal(t, w.Addresses[ChainETH][0].Address, addr)

	// A chain with no derived addresses reports not-found.
	_, ok = w.GetPrimaryAddress(ChainLTC)
	assert.False(t, ok)
}

func TestWallet_ToSummary(t *testing.T) {
	t.Parallel()
	w := newDerivedWallet(t, 2, ChainETH, ChainBSV)

	summary := w.ToSummary()
	assert.Equal(t, w.Name, summary.Name)
	assert.Equal(t, w.EnabledChains, summary.EnabledChains)
	assert.Equal(t, w.CreatedAt, summary.CreatedAt)

	// Each chain with derived addresses maps to its primary address.
	for _, chainID := range []ChainID{ChainETH, ChainBSV} {
		primary, ok := w.GetPrimaryAddress(chainID)
		require.True(t, ok)
		assert.Equal(t, primary, summary.Addresses[chainID])
	}
	// No entry is created for chains without addresses.
	_, ok := summary.Addresses[ChainLTC]
	assert.False(t, ok)
}

func TestWallet_GetReceiveAddress(t *testing.T) {
	t.Parallel()
	w := newDerivedWallet(t, 3, ChainETH)

	addr := w.GetReceiveAddress(ChainETH, 1)
	require.NotNil(t, addr)
	assert.Equal(t, w.Addresses[ChainETH][1].Address, addr.Address)

	// Negative, out-of-range, and unknown-chain lookups all return nil.
	assert.Nil(t, w.GetReceiveAddress(ChainETH, -1))
	assert.Nil(t, w.GetReceiveAddress(ChainETH, 99))
	assert.Nil(t, w.GetReceiveAddress(ChainLTC, 0))
}

func TestWallet_GetAllAddresses(t *testing.T) {
	t.Parallel()
	seed := getTestSeed(t)
	w := newDerivedWallet(t, 2, ChainBSV)

	change, err := w.DeriveNextChangeAddress(seed, ChainBSV)
	require.NoError(t, err)

	all := w.GetAllAddresses(ChainBSV)
	require.Len(t, all, 3) // 2 receive + 1 change
	// Change addresses are appended after receive addresses.
	assert.Equal(t, change.Address, all[2].Address)
	assert.True(t, all[2].IsChange)

	// A chain with nothing derived yields an empty (non-nil) slice.
	empty := w.GetAllAddresses(ChainETH)
	assert.NotNil(t, empty)
	assert.Empty(t, empty)
}

func TestWallet_FindAddressByString(t *testing.T) {
	t.Parallel()
	seed := getTestSeed(t)
	w := newDerivedWallet(t, 2, ChainBSV)
	change, err := w.DeriveNextChangeAddress(seed, ChainBSV)
	require.NoError(t, err)

	// Receive address: found, not flagged as change.
	recv := w.Addresses[ChainBSV][0].Address
	found, isChange := w.FindAddressByString(ChainBSV, recv)
	require.NotNil(t, found)
	assert.Equal(t, recv, found.Address)
	assert.False(t, isChange)

	// Change address: found, flagged as change.
	found, isChange = w.FindAddressByString(ChainBSV, change.Address)
	require.NotNil(t, found)
	assert.Equal(t, change.Address, found.Address)
	assert.True(t, isChange)

	// Unknown address: not found.
	found, isChange = w.FindAddressByString(ChainBSV, "1DoesNotExistAddress")
	assert.Nil(t, found)
	assert.False(t, isChange)
}
