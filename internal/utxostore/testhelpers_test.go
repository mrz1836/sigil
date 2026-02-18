package utxostore

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mrz1836/sigil/internal/chain"
)

// createTestStore creates a new UTXO store in a temporary directory for testing.
func createTestStore(t *testing.T) *Store {
	t.Helper()
	tmpDir := t.TempDir()
	store := New(tmpDir)
	require.NotNil(t, store)
	return store
}

// createTestUTXO creates a StoredUTXO for testing with the given parameters.
func createTestUTXO(chainID chain.ID, address, txID string, vout uint32, amount uint64, spent bool) *StoredUTXO {
	return &StoredUTXO{
		ChainID:       chainID,
		TxID:          txID,
		Vout:          vout,
		Amount:        amount,
		ScriptPubKey:  "76a914..." + address[:8], // Simplified script
		Address:       address,
		Confirmations: 10,
		Spent:         spent,
		FirstSeen:     time.Now(),
		LastUpdated:   time.Now(),
	}
}

// createTestAddress creates an AddressMetadata for testing.
func createTestAddress(chainID chain.ID, address string, index uint32, isChange bool) *AddressMetadata {
	changeStr := "0"
	if isChange {
		changeStr = "1"
	}
	return &AddressMetadata{
		ChainID:        chainID,
		Address:        address,
		DerivationPath: fmt.Sprintf("m/44'/%d'/0'/%s/%d", chainID.CoinType(), changeStr, index),
		Index:          index,
		IsChange:       isChange,
		HasActivity:    false,
	}
}

// testTxID generates a unique transaction ID for testing.
func testTxID(n int) string {
	return fmt.Sprintf("%064x", n)
}

// testAddressN generates a unique test address string for testing purposes.
func testAddressN(n int) string {
	// Use a pattern that looks like a valid address but with varying content
	return fmt.Sprintf("1Test%058d", n)
}

// assertBalanceEquals asserts that the store balance for a chain equals the expected value.
func assertBalanceEquals(t *testing.T, store *Store, chainID chain.ID, expected uint64) {
	t.Helper()
	actual := store.GetBalance(chainID)
	assert.Equal(t, expected, actual, "balance mismatch for chain %s", chainID)
}

// assertUTXOCount asserts that the number of UTXOs for a chain/address equals expected.
func assertUTXOCount(t *testing.T, store *Store, chainID chain.ID, address string, expected int) {
	t.Helper()
	utxos := store.GetUTXOs(chainID, address)
	assert.Len(t, utxos, expected, "UTXO count mismatch for chain %s, address %s", chainID, address)
}

// assertAddressCount asserts that the number of addresses for a chain equals expected.
func assertAddressCount(t *testing.T, store *Store, chainID chain.ID, expected int) {
	t.Helper()
	addrs := store.GetAddresses(chainID)
	assert.Len(t, addrs, expected, "address count mismatch for chain %s", chainID)
}

// createLargeScaleStore creates a store with many addresses and UTXOs for performance testing.
func createLargeScaleStore(t *testing.T, chainID chain.ID, numAddresses, utxosPerAddress int, amountPerUTXO uint64) (*Store, uint64) {
	t.Helper()
	store := createTestStore(t)

	var total uint64
	txCounter := 0
	for i := 0; i < numAddresses; i++ {
		addr := testAddressN(i)
		metadata := createTestAddress(chainID, addr, uint32(i), false)
		store.AddAddress(metadata)

		for j := 0; j < utxosPerAddress; j++ {
			utxo := createTestUTXO(chainID, addr, testTxID(txCounter), uint32(j), amountPerUTXO, false)
			store.AddUTXO(utxo)
			total += amountPerUTXO
			txCounter++
		}
	}

	return store, total
}

// mockBulkChainClient implements BulkChainClient for testing bulk operations.
type mockBulkChainClient struct {
	*mockChainClient

	// Bulk operation functions
	bulkFetchFunc func(addresses []string) ([]BulkUTXOResult, error)

	// Call tracking
	bulkFetchCallCount int
}

func newMockBulkClient() *mockBulkChainClient {
	return &mockBulkChainClient{
		mockChainClient: newMockClient(),
	}
}

func (m *mockBulkChainClient) BulkAddressUTXOFetch(_ context.Context, addresses []string) ([]BulkUTXOResult, error) {
	m.bulkFetchCallCount++

	// Use custom function if provided
	if m.bulkFetchFunc != nil {
		return m.bulkFetchFunc(addresses)
	}

	// Default behavior: convert individual lookups to bulk results
	results := make([]BulkUTXOResult, len(addresses))
	for i, addr := range addresses {
		if err, ok := m.errors[addr]; ok {
			results[i] = BulkUTXOResult{
				Address: addr,
				Error:   err,
			}
			continue
		}

		utxos := m.utxosByAddress[addr]
		results[i] = BulkUTXOResult{
			Address:          addr,
			ConfirmedUTXOs:   utxos,
			UnconfirmedUTXOs: []chain.UTXO{},
		}
	}

	return results, nil
}

func (m *mockBulkChainClient) setBulkFetchFunc(fn func(addresses []string) ([]BulkUTXOResult, error)) {
	m.bulkFetchFunc = fn
}
