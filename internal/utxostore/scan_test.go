package utxostore

import (
	"context"
	"errors"
	"testing"

	"github.com/mrz1836/sigil/internal/chain"
	"github.com/mrz1836/sigil/internal/wallet"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// errNetwork is a static error for testing network failures.
var errNetwork = errors.New("network error")

// mockChainClient implements ChainClient for testing.
type mockChainClient struct {
	// utxosByAddress maps address -> UTXOs
	utxosByAddress map[string][]chain.UTXO
	// errors maps address -> error
	errors map[string]error
	// callCount tracks number of ListUTXOs calls
	callCount int
}

func newMockClient() *mockChainClient {
	return &mockChainClient{
		utxosByAddress: make(map[string][]chain.UTXO),
		errors:         make(map[string]error),
	}
}

func (m *mockChainClient) ListUTXOs(_ context.Context, address string) ([]chain.UTXO, error) {
	m.callCount++
	if err, ok := m.errors[address]; ok {
		return nil, err
	}
	return m.utxosByAddress[address], nil
}

func (m *mockChainClient) setUTXOs(address string, utxos []chain.UTXO) {
	m.utxosByAddress[address] = utxos
}

func (m *mockChainClient) setError(address string, err error) {
	m.errors[address] = err
}

func TestScanWallet(t *testing.T) {
	tmpDir := t.TempDir()
	store := New(tmpDir)
	client := newMockClient()

	// Create wallet with addresses
	w := &wallet.Wallet{
		Addresses: map[chain.ID][]wallet.Address{
			chain.BSV: {
				{Address: "addr0", Path: "m/44'/236'/0'/0/0", Index: 0},
				{Address: "addr1", Path: "m/44'/236'/0'/0/1", Index: 1},
				{Address: "addr2", Path: "m/44'/236'/0'/0/2", Index: 2},
			},
		},
	}

	// addr0 has UTXOs, addr1 and addr2 are empty
	client.setUTXOs("addr0", []chain.UTXO{
		{TxID: "tx1", Vout: 0, Amount: 1000, Address: "addr0"},
		{TxID: "tx2", Vout: 1, Amount: 2000, Address: "addr0"},
	})

	result, err := store.ScanWallet(context.Background(), w, chain.BSV, client)
	require.NoError(t, err)

	assert.Equal(t, 3, result.AddressesScanned)
	assert.Equal(t, 2, result.UTXOsFound)
	assert.Equal(t, uint64(3000), result.TotalBalance)
	assert.Empty(t, result.Errors)

	// Verify UTXOs were stored
	assert.Equal(t, uint64(3000), store.GetBalance(chain.BSV))
	utxos := store.GetUTXOs(chain.BSV, "")
	assert.Len(t, utxos, 2)

	// Verify addresses were tracked
	addrs := store.GetAddresses(chain.BSV)
	assert.Len(t, addrs, 3)
}

func TestScanWalletGapLimit(t *testing.T) {
	tmpDir := t.TempDir()
	store := New(tmpDir)
	client := newMockClient()

	// Create wallet with 30 addresses
	addresses := make([]wallet.Address, 30)
	for i := range 30 {
		addresses[i] = wallet.Address{
			Address: "addr" + string(rune('A'+i)),
			Index:   uint32(i), //nolint:gosec // test code, i is bounded by loop
		}
	}

	w := &wallet.Wallet{
		Addresses: map[chain.ID][]wallet.Address{
			chain.BSV: addresses,
		},
	}

	// Only addr0 has UTXOs - should scan 21 addresses (1 with activity + 20 gap)
	client.setUTXOs("addrA", []chain.UTXO{
		{TxID: "tx1", Vout: 0, Amount: 1000, Address: "addrA"},
	})

	result, err := store.ScanWallet(context.Background(), w, chain.BSV, client)
	require.NoError(t, err)

	// Should stop after gap limit (20 consecutive empty)
	assert.Equal(t, 21, result.AddressesScanned)
	assert.Equal(t, 1, result.UTXOsFound)
}

func TestScanWalletGapLimitReset(t *testing.T) {
	tmpDir := t.TempDir()
	store := New(tmpDir)
	client := newMockClient()

	// Create wallet with 50 addresses
	addresses := make([]wallet.Address, 50)
	for i := range 50 {
		addresses[i] = wallet.Address{
			Address: "addr" + string(rune('0'+i/10)) + string(rune('0'+i%10)),
			Index:   uint32(i), //nolint:gosec // test code, i is bounded by loop
		}
	}

	w := &wallet.Wallet{
		Addresses: map[chain.ID][]wallet.Address{
			chain.BSV: addresses,
		},
	}

	// UTXOs at positions 0, 15, 25 - gap counter should reset
	client.setUTXOs("addr00", []chain.UTXO{{TxID: "tx1", Vout: 0, Amount: 1000, Address: "addr00"}})
	client.setUTXOs("addr15", []chain.UTXO{{TxID: "tx2", Vout: 0, Amount: 2000, Address: "addr15"}})
	client.setUTXOs("addr25", []chain.UTXO{{TxID: "tx3", Vout: 0, Amount: 3000, Address: "addr25"}})

	result, err := store.ScanWallet(context.Background(), w, chain.BSV, client)
	require.NoError(t, err)

	// Should scan: 0-25 (26 addrs) + 20 more empty = 46 total
	assert.Equal(t, 46, result.AddressesScanned)
	assert.Equal(t, 3, result.UTXOsFound)
	assert.Equal(t, uint64(6000), result.TotalBalance)
}

func TestScanWalletEmpty(t *testing.T) {
	tmpDir := t.TempDir()
	store := New(tmpDir)
	client := newMockClient()

	// Wallet with no addresses for BSV
	w := &wallet.Wallet{
		Addresses: map[chain.ID][]wallet.Address{},
	}

	result, err := store.ScanWallet(context.Background(), w, chain.BSV, client)
	require.NoError(t, err)

	assert.Equal(t, 0, result.AddressesScanned)
	assert.Equal(t, 0, result.UTXOsFound)
}

func TestScanWalletWithErrors(t *testing.T) {
	tmpDir := t.TempDir()
	store := New(tmpDir)
	client := newMockClient()

	w := &wallet.Wallet{
		Addresses: map[chain.ID][]wallet.Address{
			chain.BSV: {
				{Address: "addr0", Index: 0},
				{Address: "addr1", Index: 1},
				{Address: "addr2", Index: 2},
			},
		},
	}

	// addr0 succeeds, addr1 errors, addr2 succeeds
	client.setUTXOs("addr0", []chain.UTXO{{TxID: "tx1", Vout: 0, Amount: 1000, Address: "addr0"}})
	client.setError("addr1", errNetwork)
	client.setUTXOs("addr2", []chain.UTXO{{TxID: "tx2", Vout: 0, Amount: 2000, Address: "addr2"}})

	result, err := store.ScanWallet(context.Background(), w, chain.BSV, client)
	require.NoError(t, err) // Scan should continue despite errors

	assert.Equal(t, 3, result.AddressesScanned)
	assert.Equal(t, 2, result.UTXOsFound)
	assert.Len(t, result.Errors, 1)
	assert.Contains(t, result.Errors[0].Error(), "addr1")
}

func TestScanWalletCancellation(t *testing.T) {
	tmpDir := t.TempDir()
	store := New(tmpDir)
	client := newMockClient()

	w := &wallet.Wallet{
		Addresses: map[chain.ID][]wallet.Address{
			chain.BSV: {
				{Address: "addr0", Index: 0},
				{Address: "addr1", Index: 1},
			},
		},
	}

	// Cancel context immediately
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := store.ScanWallet(ctx, w, chain.BSV, client)
	require.Error(t, err)
	assert.Equal(t, context.Canceled, err)
}

func TestRefresh(t *testing.T) {
	tmpDir := t.TempDir()
	store := New(tmpDir)
	client := newMockClient()

	// Pre-populate store with addresses
	store.AddAddress(&AddressMetadata{ChainID: chain.BSV, Address: "addr0"})
	store.AddAddress(&AddressMetadata{ChainID: chain.BSV, Address: "addr1"})

	// Pre-populate with UTXOs
	store.AddUTXO(&StoredUTXO{ChainID: chain.BSV, TxID: "old1", Vout: 0, Amount: 1000, Address: "addr0"})
	store.AddUTXO(&StoredUTXO{ChainID: chain.BSV, TxID: "old2", Vout: 0, Amount: 2000, Address: "addr1"})

	// Refresh: old1 still exists, old2 is gone, new1 appears
	client.setUTXOs("addr0", []chain.UTXO{
		{TxID: "old1", Vout: 0, Amount: 1000, Address: "addr0"},
		{TxID: "new1", Vout: 0, Amount: 500, Address: "addr0"},
	})
	// addr1 returns empty - old2 should be marked spent

	result, err := store.Refresh(context.Background(), chain.BSV, client)
	require.NoError(t, err)

	assert.Equal(t, 2, result.AddressesScanned)
	assert.Equal(t, 2, result.UTXOsFound)
	assert.Equal(t, uint64(1500), result.TotalBalance)

	// Check balance reflects only unspent
	assert.Equal(t, uint64(1500), store.GetBalance(chain.BSV))

	// old2 should be marked as spent but still in store
	utxos := store.GetUTXOs(chain.BSV, "")
	assert.Len(t, utxos, 2) // old1 and new1 (unspent only)
}

func TestRefreshNoAddresses(t *testing.T) {
	tmpDir := t.TempDir()
	store := New(tmpDir)
	client := newMockClient()

	result, err := store.Refresh(context.Background(), chain.BSV, client)
	require.NoError(t, err)

	assert.Equal(t, 0, result.AddressesScanned)
	assert.Equal(t, 0, result.UTXOsFound)
}

func TestRefreshPreservesSpentHistory(t *testing.T) {
	tmpDir := t.TempDir()
	store := New(tmpDir)
	client := newMockClient()

	// Add address
	store.AddAddress(&AddressMetadata{ChainID: chain.BSV, Address: "addr0"})

	// Add UTXO
	store.AddUTXO(&StoredUTXO{ChainID: chain.BSV, TxID: "tx1", Vout: 0, Amount: 1000, Address: "addr0"})

	// Refresh with no UTXOs - marks as spent
	result, err := store.Refresh(context.Background(), chain.BSV, client)
	require.NoError(t, err)

	assert.Equal(t, 0, result.UTXOsFound)
	assert.Equal(t, uint64(0), store.GetBalance(chain.BSV))

	// UTXO should still exist (preserved for history)
	assert.False(t, store.IsEmpty())
}
