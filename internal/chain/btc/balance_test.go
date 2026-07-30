package btc

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mrz1836/sigil/internal/wallet/bitcoin"
)

// validMainnetAddr returns a distinct valid mainnet P2PKH address seeded by b.
func validMainnetAddr(b byte) string {
	hash := make([]byte, 20)
	for i := range hash {
		hash[i] = b
	}
	return bitcoin.Base58CheckEncode(versionP2PKHMainnet, hash)
}

func statsClient(statsFn func(ctx context.Context, address string) (*AddressStats, error)) *Client {
	return &Client{
		provider: &mockEsplora{statsFn: statsFn},
		network:  NetworkMainnet,
	}
}

func TestGetNativeBalance_ConfirmedAndUnconfirmed(t *testing.T) {
	t.Parallel()

	addr := validMainnetAddr(0x01)
	c := statsClient(func(_ context.Context, _ string) (*AddressStats, error) {
		return &AddressStats{
			ChainStats:   ChainStats{FundedTxoSum: 150000, SpentTxoSum: 50000},
			MempoolStats: ChainStats{FundedTxoSum: 3000, SpentTxoSum: 1000},
		}, nil
	})

	bal, err := c.GetNativeBalance(context.Background(), addr)
	require.NoError(t, err)
	assert.Equal(t, int64(100000), bal.Amount.Int64())
	require.NotNil(t, bal.Unconfirmed)
	assert.Equal(t, int64(2000), bal.Unconfirmed.Int64())
	assert.Equal(t, "BTC", bal.Symbol)
	assert.Equal(t, 8, bal.Decimals)
}

func TestGetNativeBalance_ZeroUnconfirmedOmitted(t *testing.T) {
	t.Parallel()

	addr := validMainnetAddr(0x02)
	c := statsClient(func(_ context.Context, _ string) (*AddressStats, error) {
		return &AddressStats{ChainStats: ChainStats{FundedTxoSum: 5000}}, nil
	})

	bal, err := c.GetNativeBalance(context.Background(), addr)
	require.NoError(t, err)
	assert.Equal(t, int64(5000), bal.Amount.Int64())
	assert.Nil(t, bal.Unconfirmed, "zero mempool delta should leave Unconfirmed nil")
}

func TestGetNativeBalance_NegativeUnconfirmed(t *testing.T) {
	t.Parallel()

	addr := validMainnetAddr(0x03)
	c := statsClient(func(_ context.Context, _ string) (*AddressStats, error) {
		return &AddressStats{
			ChainStats:   ChainStats{FundedTxoSum: 10000},
			MempoolStats: ChainStats{FundedTxoSum: 0, SpentTxoSum: 4000}, // pending spend
		}, nil
	})

	bal, err := c.GetNativeBalance(context.Background(), addr)
	require.NoError(t, err)
	require.NotNil(t, bal.Unconfirmed)
	assert.Equal(t, int64(-4000), bal.Unconfirmed.Int64())
}

func TestGetNativeBalance_InvalidAddress(t *testing.T) {
	t.Parallel()

	c := statsClient(func(_ context.Context, _ string) (*AddressStats, error) {
		t.Fatal("provider should not be called for invalid address")
		return nil, errTestProvider
	})
	_, err := c.GetNativeBalance(context.Background(), "not-an-address")
	assert.ErrorIs(t, err, ErrInvalidAddress)
}

func TestGetBulkNativeBalance_PartialResults(t *testing.T) {
	t.Parallel()

	good := validMainnetAddr(0x11)
	bad := validMainnetAddr(0x22)

	c := statsClient(func(_ context.Context, address string) (*AddressStats, error) {
		if address == bad {
			return nil, errTestProvider
		}
		return &AddressStats{ChainStats: ChainStats{FundedTxoSum: 7000}}, nil
	})

	results, err := c.GetBulkNativeBalance(context.Background(), []string{good, bad})
	require.NoError(t, err, "partial success should not error")
	require.Contains(t, results, good)
	assert.Equal(t, int64(7000), results[good].Amount.Int64())
	assert.NotContains(t, results, bad, "failed address omitted to trigger per-address fallback")
}

func TestGetBulkNativeBalance_AllFail(t *testing.T) {
	t.Parallel()

	a1, a2 := validMainnetAddr(0x31), validMainnetAddr(0x32)
	c := statsClient(func(_ context.Context, _ string) (*AddressStats, error) {
		return nil, errTestProvider
	})

	results, err := c.GetBulkNativeBalance(context.Background(), []string{a1, a2})
	require.Error(t, err, "all failures should surface an error for cache fallback")
	assert.Empty(t, results)
}

func TestGetBulkNativeBalance_Empty(t *testing.T) {
	t.Parallel()

	c := statsClient(func(_ context.Context, _ string) (*AddressStats, error) {
		t.Fatal("should not be called")
		return nil, errTestProvider
	})
	results, err := c.GetBulkNativeBalance(context.Background(), nil)
	require.NoError(t, err)
	assert.Empty(t, results)
}
