package transaction

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	whatsonchain "github.com/mrz1836/go-whatsonchain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mrz1836/sigil/internal/chain"
	"github.com/mrz1836/sigil/internal/chain/bsv"
	"github.com/mrz1836/sigil/internal/wallet"
)

// mockWOCClient for BSV client testing
type mockWOCClient struct {
	utxoFunc  func(ctx context.Context, address string) (whatsonchain.AddressHistory, error)
	mu        sync.Mutex
	callCount int
}

func (m *mockWOCClient) AddressUnspentTransactions(ctx context.Context, address string) (whatsonchain.AddressHistory, error) {
	m.mu.Lock()
	m.callCount++
	m.mu.Unlock()

	if m.utxoFunc != nil {
		return m.utxoFunc(ctx, address)
	}
	return whatsonchain.AddressHistory{}, nil
}

// Implement other required WOCClient methods (unused in these tests)
func (m *mockWOCClient) AddressBalance(_ context.Context, _ string) (*whatsonchain.AddressBalance, error) {
	return &whatsonchain.AddressBalance{}, nil
}

func (m *mockWOCClient) GetMinerFeesStats(_ context.Context, _, _ int64) ([]*whatsonchain.MinerFeeStats, error) {
	return nil, nil
}

func (m *mockWOCClient) BroadcastTx(_ context.Context, _ string) (string, error) {
	return "", nil
}

func (m *mockWOCClient) BulkAddressConfirmedBalance(_ context.Context, _ *whatsonchain.AddressList) (whatsonchain.AddressBalances, error) {
	return whatsonchain.AddressBalances{}, nil
}

func (m *mockWOCClient) BulkAddressUnconfirmedBalance(_ context.Context, _ *whatsonchain.AddressList) (whatsonchain.AddressBalances, error) {
	return whatsonchain.AddressBalances{}, nil
}

func (m *mockWOCClient) BulkAddressHistory(_ context.Context, _ *whatsonchain.AddressList) (whatsonchain.BulkAddressHistoryResponse, error) {
	return whatsonchain.BulkAddressHistoryResponse{}, nil
}

func (m *mockWOCClient) BulkAddressConfirmedUTXOs(_ context.Context, _ *whatsonchain.AddressList) (whatsonchain.BulkUnspentResponse, error) {
	return whatsonchain.BulkUnspentResponse{}, nil
}

func (m *mockWOCClient) BulkAddressUnconfirmedUTXOs(_ context.Context, _ *whatsonchain.AddressList) (whatsonchain.BulkUnspentResponse, error) {
	return whatsonchain.BulkUnspentResponse{}, nil
}

func (m *mockWOCClient) BulkSpentOutputs(_ context.Context, _ *whatsonchain.BulkSpentOutputRequest) (whatsonchain.BulkSpentOutputResponse, error) {
	return whatsonchain.BulkSpentOutputResponse{}, nil
}

func (m *mockWOCClient) getCallCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.callCount
}

// TestAggregateBSVUTXOs_SingleAddress tests UTXO aggregation for a single address.
func TestAggregateBSVUTXOs_SingleAddress(t *testing.T) {
	t.Parallel()

	addresses := []wallet.Address{
		{Address: "1A1zP1eP5QGefi2DMPTfTL5SLmv7DivfNa", Index: 0, Path: "m/44'/236'/0'/0/0"},
	}

	mockWOC := &mockWOCClient{
		utxoFunc: func(_ context.Context, addr string) (whatsonchain.AddressHistory, error) {
			if addr == "1A1zP1eP5QGefi2DMPTfTL5SLmv7DivfNa" {
				return whatsonchain.AddressHistory{
					{TxHash: "tx1", TxPos: 0, Value: 100000},
					{TxHash: "tx2", TxPos: 1, Value: 200000},
				}, nil
			}
			return nil, fmt.Errorf("unknown address: %s", addr) //nolint:err113 // Test error
		},
	}

	client := bsv.NewClient(context.Background(), &bsv.ClientOptions{WOCClient: mockWOC})
	ctx := context.Background()
	utxos, err := aggregateBSVUTXOs(ctx, client, addresses)

	require.NoError(t, err)
	require.NotNil(t, utxos)
	assert.Len(t, utxos, 2)

	// Check both UTXOs are present with correct amounts (order is non-deterministic)
	utxoData := make(map[string]uint64)
	for _, utxo := range utxos {
		utxoData[utxo.TxID] = utxo.Amount
	}
	assert.Equal(t, uint64(100000), utxoData["tx1"], "tx1 should have amount 100000")
	assert.Equal(t, uint64(200000), utxoData["tx2"], "tx2 should have amount 200000")
	assert.Equal(t, 1, mockWOC.getCallCount(), "should call ListUTXOs once")
}

// TestAggregateBSVUTXOs_MultipleAddresses tests concurrent UTXO fetching for multiple addresses.
func TestAggregateBSVUTXOs_MultipleAddresses(t *testing.T) {
	t.Parallel()

	addresses := []wallet.Address{
		{Address: "1A1zP1eP5QGefi2DMPTfTL5SLmv7DivfNa", Index: 0, Path: "m/44'/236'/0'/0/0"},
		{Address: "1BvBMSEYstWetqTFn5Au4m4GFg7xJaNVN2", Index: 1, Path: "m/44'/236'/0'/0/1"},
		{Address: "1JwSSubhmg6iPtRjtyqhUYYH7bZg3Lfy1T", Index: 2, Path: "m/44'/236'/0'/0/2"},
	}

	mockWOC := &mockWOCClient{
		utxoFunc: func(_ context.Context, addr string) (whatsonchain.AddressHistory, error) {
			// Simulate network delay
			time.Sleep(10 * time.Millisecond)

			switch addr {
			case "1A1zP1eP5QGefi2DMPTfTL5SLmv7DivfNa":
				return whatsonchain.AddressHistory{{TxHash: "tx1", TxPos: 0, Value: 100000}}, nil
			case "1BvBMSEYstWetqTFn5Au4m4GFg7xJaNVN2":
				return whatsonchain.AddressHistory{{TxHash: "tx2", TxPos: 0, Value: 200000}}, nil
			case "1JwSSubhmg6iPtRjtyqhUYYH7bZg3Lfy1T":
				return whatsonchain.AddressHistory{{TxHash: "tx3", TxPos: 0, Value: 300000}}, nil
			default:
				return nil, fmt.Errorf("unknown address: %s", addr) //nolint:err113 // Test error
			}
		},
	}

	client := bsv.NewClient(context.Background(), &bsv.ClientOptions{WOCClient: mockWOC})
	ctx := context.Background()
	start := time.Now()
	utxos, err := aggregateBSVUTXOs(ctx, client, addresses)
	elapsed := time.Since(start)

	require.NoError(t, err)
	require.NotNil(t, utxos)
	assert.Len(t, utxos, 3, "should aggregate UTXOs from all addresses")

	// Verify concurrent execution: should take ~10ms, not 30ms (3 * 10ms)
	// Use 100ms threshold to account for race detector overhead (5-10x slower)
	// Sequential would be 30ms without race detector, ~60-90ms with race detector
	assert.Less(t, elapsed, 100*time.Millisecond, "should execute concurrently")
	assert.Equal(t, 3, mockWOC.getCallCount(), "should call ListUTXOs three times")

	// Verify all addresses present
	addrSet := make(map[string]bool)
	for _, u := range utxos {
		addrSet[u.Address] = true
	}
	assert.True(t, addrSet["1A1zP1eP5QGefi2DMPTfTL5SLmv7DivfNa"])
	assert.True(t, addrSet["1BvBMSEYstWetqTFn5Au4m4GFg7xJaNVN2"])
	assert.True(t, addrSet["1JwSSubhmg6iPtRjtyqhUYYH7bZg3Lfy1T"])
}

// TestAggregateBSVUTXOs_ConcurrentFetch tests race detection with many concurrent goroutines.
// Run with: go test -race
func TestAggregateBSVUTXOs_ConcurrentFetch(t *testing.T) {
	t.Parallel()

	// Create 20 addresses to fetch concurrently (using repeated valid addresses with different paths)
	addresses := make([]wallet.Address, 20)
	validAddrs := []string{
		"1A1zP1eP5QGefi2DMPTfTL5SLmv7DivfNa",
		"1BvBMSEYstWetqTFn5Au4m4GFg7xJaNVN2",
		"1JwSSubhmg6iPtRjtyqhUYYH7bZg3Lfy1T",
		"3J98t1WpEZ73CNmQviecrnyiWrnqRhWNLy",
	}
	for i := 0; i < 20; i++ {
		addresses[i] = wallet.Address{
			Address: validAddrs[i%len(validAddrs)], //nolint:gosec // Test data, i < 20 and len(validAddrs) = 4
			Index:   uint32(i),                     //nolint:gosec // Test data, i < 20 fits in uint32
			Path:    fmt.Sprintf("m/44'/236'/0'/0/%d", i),
		}
	}

	mockWOC := &mockWOCClient{
		utxoFunc: func(_ context.Context, addr string) (whatsonchain.AddressHistory, error) {
			// Simulate variable network delay
			time.Sleep(time.Duration(1+len(addr)%3) * time.Millisecond)
			return whatsonchain.AddressHistory{
				{TxHash: "tx-" + addr, TxPos: 0, Value: 100000},
			}, nil
		},
	}

	client := bsv.NewClient(context.Background(), &bsv.ClientOptions{WOCClient: mockWOC})
	ctx := context.Background()
	utxos, err := aggregateBSVUTXOs(ctx, client, addresses)

	require.NoError(t, err)
	require.NotNil(t, utxos)
	assert.Len(t, utxos, 20, "should aggregate UTXOs from all 20 addresses")
	assert.Equal(t, 20, mockWOC.getCallCount(), "should call ListUTXOs 20 times")
}

// TestAggregateBSVUTXOs_ContextCancellation tests that context cancellation stops UTXO fetching.
func TestAggregateBSVUTXOs_ContextCancellation(t *testing.T) {
	t.Parallel()

	addresses := []wallet.Address{
		{Address: "1A1zP1eP5QGefi2DMPTfTL5SLmv7DivfNa", Index: 0, Path: "m/44'/236'/0'/0/0"},
		{Address: "1BvBMSEYstWetqTFn5Au4m4GFg7xJaNVN2", Index: 1, Path: "m/44'/236'/0'/0/1"},
	}

	mockWOC := &mockWOCClient{
		utxoFunc: func(ctx context.Context, _ string) (whatsonchain.AddressHistory, error) {
			// Simulate slow network call
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(100 * time.Millisecond):
				return whatsonchain.AddressHistory{{TxHash: "tx1", TxPos: 0, Value: 100000}}, nil
			}
		},
	}

	client := bsv.NewClient(context.Background(), &bsv.ClientOptions{WOCClient: mockWOC})
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()

	utxos, err := aggregateBSVUTXOs(ctx, client, addresses)

	// Should return error due to context cancellation
	require.Error(t, err)
	assert.Nil(t, utxos)
	assert.Contains(t, err.Error(), "listing UTXOs")
}

// TestAggregateBSVUTXOs_EmptyAddresses tests handling of empty address list.
func TestAggregateBSVUTXOs_EmptyAddresses(t *testing.T) {
	t.Parallel()

	addresses := []wallet.Address{}

	mockWOC := &mockWOCClient{
		utxoFunc: func(_ context.Context, _ string) (whatsonchain.AddressHistory, error) {
			t.Fatal("should not be called for empty address list")
			return nil, nil
		},
	}

	client := bsv.NewClient(context.Background(), &bsv.ClientOptions{WOCClient: mockWOC})
	ctx := context.Background()
	utxos, err := aggregateBSVUTXOs(ctx, client, addresses)

	require.NoError(t, err)
	// With empty addresses, allUTXOs remains nil (not an empty slice)
	assert.Nil(t, utxos)
	assert.Equal(t, 0, mockWOC.getCallCount(), "should not call ListUTXOs")
}

// TestAggregateBSVUTXOs_ErrorFromOneAddress tests error handling when one address fetch fails.
func TestAggregateBSVUTXOs_ErrorFromOneAddress(t *testing.T) {
	t.Parallel()

	addresses := []wallet.Address{
		{Address: "1A1zP1eP5QGefi2DMPTfTL5SLmv7DivfNa", Index: 0, Path: "m/44'/236'/0'/0/0"},
		{Address: "1BvBMSEYstWetqTFn5Au4m4GFg7xJaNVN2", Index: 1, Path: "m/44'/236'/0'/0/1"},
		{Address: "1JwSSubhmg6iPtRjtyqhUYYH7bZg3Lfy1T", Index: 2, Path: "m/44'/236'/0'/0/2"},
	}

	mockWOC := &mockWOCClient{
		utxoFunc: func(_ context.Context, addr string) (whatsonchain.AddressHistory, error) {
			if addr == "1BvBMSEYstWetqTFn5Au4m4GFg7xJaNVN2" {
				return nil, errors.New("network error") //nolint:err113 // Test error
			}
			return whatsonchain.AddressHistory{{TxHash: "tx-" + addr, TxPos: 0, Value: 100000}}, nil
		},
	}

	client := bsv.NewClient(context.Background(), &bsv.ClientOptions{WOCClient: mockWOC})
	ctx := context.Background()
	utxos, err := aggregateBSVUTXOs(ctx, client, addresses)

	require.Error(t, err)
	assert.Nil(t, utxos)
	assert.Contains(t, err.Error(), "listing UTXOs for 1BvBMSEYstWetqTFn5Au4m4GFg7xJaNVN2")
	assert.Contains(t, err.Error(), "network error")
	assert.Equal(t, 3, mockWOC.getCallCount(), "should call ListUTXOs for all addresses")
}

// TestAggregateBSVUTXOs_NoUTXOs tests handling when addresses have no UTXOs.
func TestAggregateBSVUTXOs_NoUTXOs(t *testing.T) {
	t.Parallel()

	addresses := []wallet.Address{
		{Address: "1A1zP1eP5QGefi2DMPTfTL5SLmv7DivfNa", Index: 0, Path: "m/44'/236'/0'/0/0"},
		{Address: "1BvBMSEYstWetqTFn5Au4m4GFg7xJaNVN2", Index: 1, Path: "m/44'/236'/0'/0/1"},
	}

	mockWOC := &mockWOCClient{
		utxoFunc: func(_ context.Context, _ string) (whatsonchain.AddressHistory, error) {
			return whatsonchain.AddressHistory{}, nil // No UTXOs
		},
	}

	client := bsv.NewClient(context.Background(), &bsv.ClientOptions{WOCClient: mockWOC})
	ctx := context.Background()
	utxos, err := aggregateBSVUTXOs(ctx, client, addresses)

	require.NoError(t, err)
	// When appending empty slices to nil, it remains nil
	assert.Nil(t, utxos)
	assert.Equal(t, 2, mockWOC.getCallCount())
}

// TestFilterSpentBSVUTXOs_NilStore tests filtering with nil store.
func TestFilterSpentBSVUTXOs_NilStore(t *testing.T) {
	t.Parallel()

	utxos := []chain.UTXO{
		{TxID: "tx1", Vout: 0, Amount: 100000, Address: "1ABC"},
		{TxID: "tx2", Vout: 0, Amount: 200000, Address: "1DEF"},
	}

	// With nil store, all UTXOs should be returned
	filtered := filterSpentBSVUTXOs(utxos, nil)

	assert.Len(t, filtered, 2)
	assert.Equal(t, utxos, filtered)
}

// TestFilterSpentBSVUTXOs_EmptyList tests filtering empty UTXO list.
func TestFilterSpentBSVUTXOs_EmptyList(t *testing.T) {
	t.Parallel()

	utxos := []chain.UTXO{}
	store := newMockUTXOProvider()

	filtered := filterSpentBSVUTXOs(utxos, store)

	assert.Empty(t, filtered)
}

// TestFilterSpentBSVUTXOs_SomeSpent tests filtering when some UTXOs are spent.
func TestFilterSpentBSVUTXOs_SomeSpent(t *testing.T) {
	t.Parallel()

	utxos := []chain.UTXO{
		{TxID: "tx1", Vout: 0, Amount: 100000, Address: "1ABC"},
		{TxID: "tx2", Vout: 0, Amount: 200000, Address: "1DEF"},
		{TxID: "tx3", Vout: 1, Amount: 300000, Address: "1GHI"},
	}

	store := newMockUTXOProvider()
	// Mark tx1:0 and tx3:1 as spent
	store.MarkSpent(chain.BSV, "tx1", 0, "spending-tx1")
	store.MarkSpent(chain.BSV, "tx3", 1, "spending-tx2")

	filtered := filterSpentBSVUTXOs(utxos, store)

	// Only tx2:0 should remain
	assert.Len(t, filtered, 1)
	assert.Equal(t, "tx2", filtered[0].TxID)
	assert.Equal(t, uint32(0), filtered[0].Vout)
}

// TestFilterSpentBSVUTXOs_AllSpent tests filtering when all UTXOs are spent.
func TestFilterSpentBSVUTXOs_AllSpent(t *testing.T) {
	t.Parallel()

	utxos := []chain.UTXO{
		{TxID: "tx1", Vout: 0, Amount: 100000, Address: "1ABC"},
		{TxID: "tx2", Vout: 0, Amount: 200000, Address: "1DEF"},
	}

	store := newMockUTXOProvider()
	store.MarkSpent(chain.BSV, "tx1", 0, "spending-tx1")
	store.MarkSpent(chain.BSV, "tx2", 0, "spending-tx2")

	filtered := filterSpentBSVUTXOs(utxos, store)

	assert.Empty(t, filtered, "should return empty list when all UTXOs are spent")
}

// TestFilterSpentBSVUTXOs_NoneSpent tests filtering when no UTXOs are spent.
func TestFilterSpentBSVUTXOs_NoneSpent(t *testing.T) {
	t.Parallel()

	utxos := []chain.UTXO{
		{TxID: "tx1", Vout: 0, Amount: 100000, Address: "1ABC"},
		{TxID: "tx2", Vout: 0, Amount: 200000, Address: "1DEF"},
	}

	store := newMockUTXOProvider()

	filtered := filterSpentBSVUTXOs(utxos, store)

	assert.Len(t, filtered, 2, "should return all UTXOs when none are spent")
	assert.Equal(t, utxos, filtered)
}

// TestMarkSpentBSVUTXOs_NilStore tests marking spent with nil store.
func TestMarkSpentBSVUTXOs_NilStore(t *testing.T) {
	t.Parallel()

	utxos := []chain.UTXO{
		{TxID: "tx1", Vout: 0, Amount: 100000, Address: "1ABC"},
	}

	logger := newMockLogWriter()

	// Should not panic
	markSpentBSVUTXOs(logger, nil, utxos, "spending-tx")

	assert.Empty(t, logger.errorMessages, "should not log errors with nil store")
}

// TestMarkSpentBSVUTXOs_SingleUTXO tests marking a single UTXO as spent.
func TestMarkSpentBSVUTXOs_SingleUTXO(t *testing.T) {
	t.Parallel()

	utxos := []chain.UTXO{
		{TxID: "tx1", Vout: 0, Amount: 100000, Address: "1ABC", ScriptPubKey: "script1"},
	}

	store := newMockUTXOProvider()
	logger := newMockLogWriter()

	markSpentBSVUTXOs(logger, store, utxos, "spending-tx")

	// Verify UTXO was marked spent
	assert.True(t, store.IsSpent(chain.BSV, "tx1", 0))
	assert.Empty(t, logger.errorMessages)
}

// TestMarkSpentBSVUTXOs_MultipleUTXOs tests marking multiple UTXOs as spent.
func TestMarkSpentBSVUTXOs_MultipleUTXOs(t *testing.T) {
	t.Parallel()

	utxos := []chain.UTXO{
		{TxID: "tx1", Vout: 0, Amount: 100000, Address: "1ABC"},
		{TxID: "tx2", Vout: 1, Amount: 200000, Address: "1DEF"},
		{TxID: "tx3", Vout: 2, Amount: 300000, Address: "1GHI"},
	}

	store := newMockUTXOProvider()
	logger := newMockLogWriter()

	markSpentBSVUTXOs(logger, store, utxos, "spending-tx")

	// Verify all UTXOs were marked spent
	assert.True(t, store.IsSpent(chain.BSV, "tx1", 0))
	assert.True(t, store.IsSpent(chain.BSV, "tx2", 1))
	assert.True(t, store.IsSpent(chain.BSV, "tx3", 2))
	assert.Empty(t, logger.errorMessages)
}

// TestMarkSpentBSVUTXOs_SaveError tests error handling when store.Save fails.
func TestMarkSpentBSVUTXOs_SaveError(t *testing.T) {
	t.Parallel()

	utxos := []chain.UTXO{
		{TxID: "tx1", Vout: 0, Amount: 100000, Address: "1ABC"},
	}

	store := &mockUTXOProviderWithSaveError{
		mockUTXOProvider: newMockUTXOProvider(),
		saveError:        errors.New("disk full"), //nolint:err113 // Test error
	}
	logger := newMockLogWriter()

	markSpentBSVUTXOs(logger, store, utxos, "spending-tx")

	// UTXO should still be marked spent (in memory)
	assert.True(t, store.IsSpent(chain.BSV, "tx1", 0))

	// Error should be logged
	require.Len(t, logger.errorMessages, 1)
	assert.Contains(t, logger.errorMessages[0], "failed to save utxo store")
}

// TestMarkSpentBSVUTXOs_NilLogger tests marking spent with nil logger.
func TestMarkSpentBSVUTXOs_NilLogger(t *testing.T) {
	t.Parallel()

	utxos := []chain.UTXO{
		{TxID: "tx1", Vout: 0, Amount: 100000, Address: "1ABC"},
	}

	store := &mockUTXOProviderWithSaveError{
		mockUTXOProvider: newMockUTXOProvider(),
		saveError:        errors.New("save failed"), //nolint:err113 // Test error
	}

	// Should not panic with nil logger
	markSpentBSVUTXOs(nil, store, utxos, "spending-tx")

	// UTXO should still be marked spent
	assert.True(t, store.IsSpent(chain.BSV, "tx1", 0))
}

// mockUTXOProviderWithSaveError extends mockUTXOProvider to simulate Save errors.
type mockUTXOProviderWithSaveError struct {
	*mockUTXOProvider

	saveError error
}

func (m *mockUTXOProviderWithSaveError) Save() error {
	return m.saveError
}
