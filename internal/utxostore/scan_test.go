package utxostore

import (
	"context"
	"errors"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mrz1836/sigil/internal/chain"
	"github.com/mrz1836/sigil/internal/wallet"
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
	t.Parallel()
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
	t.Parallel()
	tmpDir := t.TempDir()
	store := New(tmpDir)
	client := newMockClient()

	// Create wallet with 30 addresses
	addresses := make([]wallet.Address, 30)
	for i := range 30 {
		addresses[i] = wallet.Address{
			Address: "addr" + string(rune('A'+i)),
			Index:   uint32(i),
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
	t.Parallel()
	tmpDir := t.TempDir()
	store := New(tmpDir)
	client := newMockClient()

	// Create wallet with 50 addresses
	addresses := make([]wallet.Address, 50)
	for i := range 50 {
		addresses[i] = wallet.Address{
			Address: "addr" + string(rune('0'+i/10)) + string(rune('0'+i%10)),
			Index:   uint32(i),
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
	t.Parallel()
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
	t.Parallel()
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
	t.Parallel()
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
	t.Parallel()
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
	t.Parallel()
	tmpDir := t.TempDir()
	store := New(tmpDir)
	client := newMockClient()

	result, err := store.Refresh(context.Background(), chain.BSV, client)
	require.NoError(t, err)

	assert.Equal(t, 0, result.AddressesScanned)
	assert.Equal(t, 0, result.UTXOsFound)
}

func TestRefreshPreservesSpentHistory(t *testing.T) {
	t.Parallel()
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

// ========== Bulk Operation Tests ==========

func TestScanWalletBulk_Success(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	store := New(tmpDir)
	client := newMockBulkClient()

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

	// addr0 has confirmed UTXOs, addr1 has unconfirmed, addr2 is empty
	client.setUTXOs("addr0", []chain.UTXO{
		{TxID: "tx1", Vout: 0, Amount: 1000, Address: "addr0", Confirmations: 6},
		{TxID: "tx2", Vout: 1, Amount: 2000, Address: "addr0", Confirmations: 3},
	})
	client.setBulkFetchFunc(func(addresses []string) ([]BulkUTXOResult, error) {
		results := make([]BulkUTXOResult, len(addresses))
		for i, addr := range addresses {
			switch addr {
			case "addr0":
				results[i] = BulkUTXOResult{
					Address: addr,
					ConfirmedUTXOs: []chain.UTXO{
						{TxID: "tx1", Vout: 0, Amount: 1000, Address: "addr0", Confirmations: 6},
						{TxID: "tx2", Vout: 1, Amount: 2000, Address: "addr0", Confirmations: 3},
					},
				}
			case "addr1":
				results[i] = BulkUTXOResult{
					Address: addr,
					UnconfirmedUTXOs: []chain.UTXO{
						{TxID: "tx3", Vout: 0, Amount: 500, Address: "addr1", Confirmations: 0},
					},
				}
			default:
				results[i] = BulkUTXOResult{Address: addr}
			}
		}
		return results, nil
	})

	result, err := store.ScanWalletBulk(context.Background(), w, chain.BSV, client)
	require.NoError(t, err)

	assert.Equal(t, 3, result.AddressesScanned)
	assert.Equal(t, 3, result.UTXOsFound)
	assert.Equal(t, uint64(3500), result.TotalBalance)
	assert.Empty(t, result.Errors)

	// Verify bulk method was called
	assert.Equal(t, 1, client.bulkFetchCallCount)

	// Verify UTXOs were stored
	assert.Equal(t, uint64(3500), store.GetBalance(chain.BSV))
}

func TestScanWalletBulk_GapLimitRespect(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	store := New(tmpDir)
	client := newMockBulkClient()

	// Create wallet with 30 addresses
	addresses := make([]wallet.Address, 30)
	for i := range 30 {
		addresses[i] = wallet.Address{
			Address: "addr" + string(rune('A'+i)),
			Index:   uint32(i),
		}
	}

	w := &wallet.Wallet{
		Addresses: map[chain.ID][]wallet.Address{
			chain.BSV: addresses,
		},
	}

	// Only first address has UTXOs - should stop after gap limit
	client.setBulkFetchFunc(func(addrs []string) ([]BulkUTXOResult, error) {
		results := make([]BulkUTXOResult, len(addrs))
		for i, addr := range addrs {
			if addr == "addrA" {
				results[i] = BulkUTXOResult{
					Address: addr,
					ConfirmedUTXOs: []chain.UTXO{
						{TxID: "tx1", Vout: 0, Amount: 1000, Address: "addrA"},
					},
				}
			} else {
				results[i] = BulkUTXOResult{Address: addr}
			}
		}
		return results, nil
	})

	result, err := store.ScanWalletBulk(context.Background(), w, chain.BSV, client)
	require.NoError(t, err)

	// Should stop after gap limit (20 consecutive empty)
	assert.Equal(t, 21, result.AddressesScanned)
	assert.Equal(t, 1, result.UTXOsFound)
}

func TestScanWalletBulk_FallbackToSequential(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	store := New(tmpDir)
	client := newMockBulkClient()

	w := &wallet.Wallet{
		Addresses: map[chain.ID][]wallet.Address{
			chain.BSV: {
				{Address: "addr0", Index: 0},
				{Address: "addr1", Index: 1},
			},
		},
	}

	// Bulk operation fails - should fallback to sequential
	client.setBulkFetchFunc(func(_ []string) ([]BulkUTXOResult, error) {
		return nil, errNetwork
	})

	// Set up sequential responses
	client.setUTXOs("addr0", []chain.UTXO{
		{TxID: "tx1", Vout: 0, Amount: 1000, Address: "addr0"},
	})

	result, err := store.ScanWalletBulk(context.Background(), w, chain.BSV, client)
	require.NoError(t, err)

	// Should have fallen back to sequential scanning
	assert.Equal(t, 2, result.AddressesScanned)
	assert.Equal(t, 1, result.UTXOsFound)
	assert.Equal(t, 1, client.bulkFetchCallCount)
	assert.Positive(t, client.callCount) // Sequential calls were made
}

func TestScanWalletBulk_PartialErrors(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	store := New(tmpDir)
	client := newMockBulkClient()

	w := &wallet.Wallet{
		Addresses: map[chain.ID][]wallet.Address{
			chain.BSV: {
				{Address: "addr0", Index: 0},
				{Address: "addr1", Index: 1},
				{Address: "addr2", Index: 2},
			},
		},
	}

	// addr1 fails, others succeed
	client.setBulkFetchFunc(func(addresses []string) ([]BulkUTXOResult, error) {
		results := make([]BulkUTXOResult, len(addresses))
		for i, addr := range addresses {
			if addr == "addr1" {
				results[i] = BulkUTXOResult{
					Address: addr,
					Error:   errNetwork,
				}
			} else {
				results[i] = BulkUTXOResult{
					Address: addr,
					ConfirmedUTXOs: []chain.UTXO{
						{TxID: "tx" + addr, Vout: 0, Amount: 1000, Address: addr},
					},
				}
			}
		}
		return results, nil
	})

	result, err := store.ScanWalletBulk(context.Background(), w, chain.BSV, client)
	require.NoError(t, err)

	assert.Equal(t, 2, result.AddressesScanned) // addr0 and addr2
	assert.Equal(t, 2, result.UTXOsFound)
	assert.Len(t, result.Errors, 1)
	assert.Contains(t, result.Errors[0].Error(), "addr1")
}

func TestScanWalletBulk_EmptyWallet(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	store := New(tmpDir)
	client := newMockBulkClient()

	// Wallet with no addresses for BSV
	w := &wallet.Wallet{
		Addresses: map[chain.ID][]wallet.Address{},
	}

	result, err := store.ScanWalletBulk(context.Background(), w, chain.BSV, client)
	require.NoError(t, err)

	assert.Equal(t, 0, result.AddressesScanned)
	assert.Equal(t, 0, result.UTXOsFound)
	assert.Equal(t, 0, client.bulkFetchCallCount)
}

func TestScanWalletBulk_ContextCancellation(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	store := New(tmpDir)
	client := newMockBulkClient()

	w := &wallet.Wallet{
		Addresses: map[chain.ID][]wallet.Address{
			chain.BSV: {
				{Address: "addr0", Index: 0},
				{Address: "addr1", Index: 1},
			},
		},
	}

	// Create canceled context
	ctx, cancel := context.WithCancel(context.Background())

	// Cancel after first address is processed
	client.setBulkFetchFunc(func(addresses []string) ([]BulkUTXOResult, error) {
		cancel()
		results := make([]BulkUTXOResult, len(addresses))
		for i, addr := range addresses {
			results[i] = BulkUTXOResult{
				Address: addr,
				ConfirmedUTXOs: []chain.UTXO{
					{TxID: "tx" + addr, Vout: 0, Amount: 1000, Address: addr},
				},
			}
		}
		return results, nil
	})

	_, err := store.ScanWalletBulk(ctx, w, chain.BSV, client)
	require.Error(t, err)
	assert.Equal(t, context.Canceled, err)
}

func TestRefreshBulk_Success(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	store := New(tmpDir)
	client := newMockBulkClient()

	// Pre-populate store with addresses and UTXOs
	store.AddAddress(&AddressMetadata{ChainID: chain.BSV, Address: "addr0"})
	store.AddAddress(&AddressMetadata{ChainID: chain.BSV, Address: "addr1"})
	store.AddUTXO(&StoredUTXO{ChainID: chain.BSV, TxID: "old1", Vout: 0, Amount: 1000, Address: "addr0"})
	store.AddUTXO(&StoredUTXO{ChainID: chain.BSV, TxID: "old2", Vout: 0, Amount: 2000, Address: "addr1"})

	// Refresh: old1 still exists, old2 is gone, new1 appears
	client.setBulkFetchFunc(func(addresses []string) ([]BulkUTXOResult, error) {
		results := make([]BulkUTXOResult, len(addresses))
		for i, addr := range addresses {
			if addr == "addr0" {
				results[i] = BulkUTXOResult{
					Address: addr,
					ConfirmedUTXOs: []chain.UTXO{
						{TxID: "old1", Vout: 0, Amount: 1000, Address: "addr0"},
						{TxID: "new1", Vout: 0, Amount: 500, Address: "addr0"},
					},
				}
			} else {
				results[i] = BulkUTXOResult{Address: addr}
			}
		}
		return results, nil
	})

	result, err := store.RefreshBulk(context.Background(), chain.BSV, client)
	require.NoError(t, err)

	assert.Equal(t, 2, result.AddressesScanned)
	assert.Equal(t, 2, result.UTXOsFound)
	assert.Equal(t, uint64(1500), result.TotalBalance)

	// Check balance reflects only unspent
	assert.Equal(t, uint64(1500), store.GetBalance(chain.BSV))
}

func TestRefreshBulk_AllUTXOsGone(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	store := New(tmpDir)
	client := newMockBulkClient()

	// Pre-populate with addresses and UTXOs
	store.AddAddress(&AddressMetadata{ChainID: chain.BSV, Address: "addr0"})
	store.AddUTXO(&StoredUTXO{ChainID: chain.BSV, TxID: "tx1", Vout: 0, Amount: 1000, Address: "addr0"})

	// All UTXOs gone
	client.setBulkFetchFunc(func(addresses []string) ([]BulkUTXOResult, error) {
		results := make([]BulkUTXOResult, len(addresses))
		for i, addr := range addresses {
			results[i] = BulkUTXOResult{Address: addr}
		}
		return results, nil
	})

	result, err := store.RefreshBulk(context.Background(), chain.BSV, client)
	require.NoError(t, err)

	assert.Equal(t, 1, result.AddressesScanned)
	assert.Equal(t, 0, result.UTXOsFound)
	assert.Equal(t, uint64(0), store.GetBalance(chain.BSV))

	// UTXO should be marked spent
	assert.True(t, store.IsSpent(chain.BSV, "tx1", 0))
}

func TestRefreshBulk_FallbackToSequential(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	store := New(tmpDir)
	client := newMockBulkClient()

	// Pre-populate
	store.AddAddress(&AddressMetadata{ChainID: chain.BSV, Address: "addr0"})

	// Bulk fails, fallback to sequential
	client.setBulkFetchFunc(func(_ []string) ([]BulkUTXOResult, error) {
		return nil, errNetwork
	})

	client.setUTXOs("addr0", []chain.UTXO{
		{TxID: "tx1", Vout: 0, Amount: 1000, Address: "addr0"},
	})

	result, err := store.RefreshBulk(context.Background(), chain.BSV, client)
	require.NoError(t, err)

	assert.Equal(t, 1, result.AddressesScanned)
	assert.Equal(t, 1, result.UTXOsFound)
	assert.Positive(t, client.callCount) // Sequential calls were made
}

func TestRefreshBulk_UnconfirmedUTXOs(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	store := New(tmpDir)
	client := newMockBulkClient()

	store.AddAddress(&AddressMetadata{ChainID: chain.BSV, Address: "addr0"})

	// Mix of confirmed and unconfirmed
	client.setBulkFetchFunc(func(addresses []string) ([]BulkUTXOResult, error) {
		results := make([]BulkUTXOResult, len(addresses))
		for i, addr := range addresses {
			results[i] = BulkUTXOResult{
				Address: addr,
				ConfirmedUTXOs: []chain.UTXO{
					{TxID: "confirmed1", Vout: 0, Amount: 1000, Address: addr, Confirmations: 6},
				},
				UnconfirmedUTXOs: []chain.UTXO{
					{TxID: "unconfirmed1", Vout: 0, Amount: 500, Address: addr, Confirmations: 0},
				},
			}
		}
		return results, nil
	})

	result, err := store.RefreshBulk(context.Background(), chain.BSV, client)
	require.NoError(t, err)

	assert.Equal(t, 2, result.UTXOsFound) // Both confirmed and unconfirmed
	assert.Equal(t, uint64(1500), result.TotalBalance)
}

func TestRefreshBulk_NoAddresses(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	store := New(tmpDir)
	client := newMockBulkClient()

	result, err := store.RefreshBulk(context.Background(), chain.BSV, client)
	require.NoError(t, err)

	assert.Equal(t, 0, result.AddressesScanned)
	assert.Equal(t, 0, result.UTXOsFound)
	assert.Equal(t, 0, client.bulkFetchCallCount)
}

func TestRefreshBulk_PartialErrors(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	store := New(tmpDir)
	client := newMockBulkClient()

	store.AddAddress(&AddressMetadata{ChainID: chain.BSV, Address: "addr0"})
	store.AddAddress(&AddressMetadata{ChainID: chain.BSV, Address: "addr1"})

	// addr1 fails
	client.setBulkFetchFunc(func(addresses []string) ([]BulkUTXOResult, error) {
		results := make([]BulkUTXOResult, len(addresses))
		for i, addr := range addresses {
			if addr == "addr1" {
				results[i] = BulkUTXOResult{
					Address: addr,
					Error:   errNetwork,
				}
			} else {
				results[i] = BulkUTXOResult{
					Address: addr,
					ConfirmedUTXOs: []chain.UTXO{
						{TxID: "tx1", Vout: 0, Amount: 1000, Address: addr},
					},
				}
			}
		}
		return results, nil
	})

	result, err := store.RefreshBulk(context.Background(), chain.BSV, client)
	require.NoError(t, err)

	assert.Equal(t, 1, result.AddressesScanned) // Only addr0
	assert.Equal(t, 1, result.UTXOsFound)
	assert.Len(t, result.Errors, 1)
}

func TestRefreshAddress_ExistingAddress(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	store := New(tmpDir)
	client := newMockClient()

	// Pre-populate address and UTXOs
	store.AddAddress(&AddressMetadata{ChainID: chain.BSV, Address: "addr0"})
	store.AddUTXO(&StoredUTXO{ChainID: chain.BSV, TxID: "old1", Vout: 0, Amount: 1000, Address: "addr0"})
	store.AddUTXO(&StoredUTXO{ChainID: chain.BSV, TxID: "old2", Vout: 0, Amount: 2000, Address: "addr0"})

	// Refresh: old1 still exists (should remain), old2 is gone (marked spent), new1 appears
	client.setUTXOs("addr0", []chain.UTXO{
		{TxID: "old1", Vout: 0, Amount: 1000, Address: "addr0"},
		{TxID: "new1", Vout: 0, Amount: 500, Address: "addr0"},
	})

	result, err := store.RefreshAddress(context.Background(), "addr0", chain.BSV, client)
	require.NoError(t, err)

	assert.Equal(t, 1, result.AddressesScanned)
	assert.Equal(t, 2, result.UTXOsFound)
	assert.Equal(t, uint64(1500), result.TotalBalance)

	// old2 should be marked spent
	assert.True(t, store.IsSpent(chain.BSV, "old2", 0))
	assert.False(t, store.IsSpent(chain.BSV, "old1", 0))
}

func TestRefreshAddress_NewAddress(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	store := New(tmpDir)
	client := newMockClient()

	// Address not in store yet
	client.setUTXOs("newaddr", []chain.UTXO{
		{TxID: "tx1", Vout: 0, Amount: 1000, Address: "newaddr"},
	})

	result, err := store.RefreshAddress(context.Background(), "newaddr", chain.BSV, client)
	require.NoError(t, err)

	assert.Equal(t, 1, result.AddressesScanned)
	assert.Equal(t, 1, result.UTXOsFound)

	// Address should now be in store
	addr := store.GetAddress(chain.BSV, "newaddr")
	require.NotNil(t, addr)
	assert.Equal(t, "newaddr", addr.Address)
}

func TestRefreshAddress_AllUTXOsSpent(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	store := New(tmpDir)
	client := newMockClient()

	// Pre-populate with UTXOs
	store.AddAddress(&AddressMetadata{ChainID: chain.BSV, Address: "addr0"})
	store.AddUTXO(&StoredUTXO{ChainID: chain.BSV, TxID: "tx1", Vout: 0, Amount: 1000, Address: "addr0"})
	store.AddUTXO(&StoredUTXO{ChainID: chain.BSV, TxID: "tx2", Vout: 0, Amount: 2000, Address: "addr0"})

	// All UTXOs now spent (chain returns empty)
	client.setUTXOs("addr0", []chain.UTXO{})

	result, err := store.RefreshAddress(context.Background(), "addr0", chain.BSV, client)
	require.NoError(t, err)

	assert.Equal(t, 0, result.UTXOsFound)
	assert.Equal(t, uint64(0), store.GetBalance(chain.BSV))

	// Both should be marked spent
	assert.True(t, store.IsSpent(chain.BSV, "tx1", 0))
	assert.True(t, store.IsSpent(chain.BSV, "tx2", 0))
}

func TestRefreshAddress_NetworkError(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	store := New(tmpDir)
	client := newMockClient()

	store.AddAddress(&AddressMetadata{ChainID: chain.BSV, Address: "addr0"})

	// Network error
	client.setError("addr0", errNetwork)

	result, err := store.RefreshAddress(context.Background(), "addr0", chain.BSV, client)
	require.NoError(t, err) // Errors are collected in result, not returned

	assert.Equal(t, 1, result.AddressesScanned)
	assert.Equal(t, 0, result.UTXOsFound)
	assert.Len(t, result.Errors, 1)
	assert.Contains(t, result.Errors[0].Error(), "addr0")
}

func TestRefreshAddress_SaveError(t *testing.T) {
	t.Parallel()
	// Create store with read-only directory to trigger save error
	tmpDir := t.TempDir()
	store := New(tmpDir)
	client := newMockClient()

	store.AddAddress(&AddressMetadata{ChainID: chain.BSV, Address: "addr0"})

	client.setUTXOs("addr0", []chain.UTXO{
		{TxID: "tx1", Vout: 0, Amount: 1000, Address: "addr0"},
	})

	// Make directory read-only to trigger save error
	require.NoError(t, os.Chmod(tmpDir, 0o400))
	defer os.Chmod(tmpDir, 0o700) //nolint:errcheck,gosec // cleanup

	_, err := store.RefreshAddress(context.Background(), "addr0", chain.BSV, client)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "saving UTXOs")
}

// ========== Helper Function Tests ==========

func TestGetAddressByString_Found(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	store := New(tmpDir)

	// Add address
	store.AddAddress(&AddressMetadata{
		ChainID: chain.BSV,
		Address: "addr0",
		Index:   0,
	})

	// Test the internal function through RefreshAddress
	client := newMockClient()
	client.setUTXOs("addr0", []chain.UTXO{{TxID: "tx1", Vout: 0, Amount: 1000, Address: "addr0"}})

	result, err := store.RefreshAddress(context.Background(), "addr0", chain.BSV, client)
	require.NoError(t, err)
	assert.Equal(t, 1, result.AddressesScanned)
}

func TestGetAddressByString_NotFound(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	store := New(tmpDir)
	client := newMockClient()

	// Address not in store - should still work
	client.setUTXOs("unknown", []chain.UTXO{{TxID: "tx1", Vout: 0, Amount: 1000, Address: "unknown"}})

	result, err := store.RefreshAddress(context.Background(), "unknown", chain.BSV, client)
	require.NoError(t, err)
	assert.Equal(t, 1, result.AddressesScanned)

	// Address should now exist
	addr := store.GetAddress(chain.BSV, "unknown")
	require.NotNil(t, addr)
}

func TestGetAddressByString_MultipleChains(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	store := New(tmpDir)
	client := newMockClient()

	// Same address on different chains
	store.AddAddress(&AddressMetadata{ChainID: chain.BSV, Address: "addr0"})
	store.AddAddress(&AddressMetadata{ChainID: chain.BTC, Address: "addr0"})

	client.setUTXOs("addr0", []chain.UTXO{{TxID: "tx1", Vout: 0, Amount: 1000, Address: "addr0"}})

	// Refresh BSV address
	result, err := store.RefreshAddress(context.Background(), "addr0", chain.BSV, client)
	require.NoError(t, err)
	assert.Equal(t, 1, result.AddressesScanned)

	// BTC address should not be affected
	assert.Equal(t, uint64(0), store.GetBalance(chain.BTC))
	assert.Equal(t, uint64(1000), store.GetBalance(chain.BSV))
}

func TestMarkAddressUTXOsAsSpent_Success(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	store := New(tmpDir)
	client := newMockClient()

	// Add UTXOs to address
	store.AddAddress(&AddressMetadata{ChainID: chain.BSV, Address: "addr0"})
	store.AddUTXO(&StoredUTXO{ChainID: chain.BSV, TxID: "tx1", Vout: 0, Amount: 1000, Address: "addr0"})
	store.AddUTXO(&StoredUTXO{ChainID: chain.BSV, TxID: "tx2", Vout: 0, Amount: 2000, Address: "addr0"})

	// Refresh with empty result - marks all as spent
	client.setUTXOs("addr0", []chain.UTXO{})

	result, err := store.RefreshAddress(context.Background(), "addr0", chain.BSV, client)
	require.NoError(t, err)

	assert.Equal(t, 0, result.UTXOsFound)
	assert.True(t, store.IsSpent(chain.BSV, "tx1", 0))
	assert.True(t, store.IsSpent(chain.BSV, "tx2", 0))
}

func TestMarkAddressUTXOsAsSpent_AlreadySpent(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	store := New(tmpDir)
	client := newMockClient()

	// Add already-spent UTXO
	store.AddAddress(&AddressMetadata{ChainID: chain.BSV, Address: "addr0"})
	store.AddUTXO(&StoredUTXO{ChainID: chain.BSV, TxID: "tx1", Vout: 0, Amount: 1000, Address: "addr0", Spent: true})

	// Refresh - should be idempotent
	client.setUTXOs("addr0", []chain.UTXO{})

	result, err := store.RefreshAddress(context.Background(), "addr0", chain.BSV, client)
	require.NoError(t, err)

	assert.Equal(t, 0, result.UTXOsFound)
	assert.True(t, store.IsSpent(chain.BSV, "tx1", 0))
}
