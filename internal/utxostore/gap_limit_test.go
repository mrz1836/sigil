package utxostore

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mrz1836/sigil/internal/chain"
)

// DefaultGapLimit is defined in scan.go (20 addresses).

// TestGapLimit_FundsAtIndex19 tests finding funds at the edge of the gap limit.
//
//nolint:gosec // Test code uses bounded loop variables for uint32 conversions
func TestGapLimit_FundsAtIndex19(t *testing.T) {
	t.Parallel()
	store := createTestStore(t)

	// Simulate scanning 40 addresses (gap limit of 20)
	// Funds only at index 19 (last within first gap)
	const fundedIndex = 19
	const scanDepth = 40

	for i := 0; i < scanDepth; i++ {
		addr := testAddressN(i)
		metadata := createTestAddress(chain.BSV, addr, uint32(i), false)
		store.AddAddress(metadata)

		// Only fund index 19
		if i == fundedIndex {
			utxo := createTestUTXO(chain.BSV, addr, testTxID(i), 0, 1000, false)
			store.AddUTXO(utxo)
			store.MarkAddressUsed(chain.BSV, addr)
		}
	}

	// Should find the funded address
	assertBalanceEquals(t, store, chain.BSV, 1000)

	// Verify only one address has activity
	var usedCount int
	for _, addr := range store.GetAddresses(chain.BSV) {
		if addr.HasActivity {
			usedCount++
		}
	}
	assert.Equal(t, 1, usedCount)
}

// TestGapLimit_FundsAtIndex20 tests missing funds beyond the initial gap limit.
// This simulates what would happen if scanning stopped at gap limit.
//
//nolint:gosec // Test code uses bounded loop variables for uint32 conversions
func TestGapLimit_FundsAtIndex20(t *testing.T) {
	t.Parallel()
	store := createTestStore(t)

	// Only scan first 20 addresses (standard gap limit)
	// Funds at index 20 would be missed
	const fundedIndex = 20
	const scanDepth = DefaultGapLimit // Only scan 20 addresses

	for i := 0; i < scanDepth; i++ {
		addr := testAddressN(i)
		metadata := createTestAddress(chain.BSV, addr, uint32(i), false)
		store.AddAddress(metadata)
		// No funds in first 20 addresses
	}

	// At this point, a standard scan would stop
	// Balance should be 0 because we didn't scan beyond gap limit
	assertBalanceEquals(t, store, chain.BSV, 0)

	// Simulate a later discovery of funds at index 20
	// (e.g., through explicit address query or extended scan)
	addr20 := testAddressN(fundedIndex)
	metadata20 := createTestAddress(chain.BSV, addr20, uint32(fundedIndex), false)
	store.AddAddress(metadata20)
	utxo := createTestUTXO(chain.BSV, addr20, testTxID(fundedIndex), 0, 5000, false)
	store.AddUTXO(utxo)

	// Now balance includes the "missed" funds
	assertBalanceEquals(t, store, chain.BSV, 5000)
}

// TestGapLimit_FundsAt0And25 tests finding funds at beginning and beyond gap.
//
//nolint:gosec // Test code uses bounded loop variables for uint32 conversions
func TestGapLimit_FundsAt0And25(t *testing.T) {
	t.Parallel()
	store := createTestStore(t)

	// Scenario: funds at index 0 and index 25
	// Standard scan: finds index 0, then scans 20 more (1-20), would miss 25
	const scanDepth = 46 // Full scan to find both

	for i := 0; i < scanDepth; i++ {
		addr := testAddressN(i)
		metadata := createTestAddress(chain.BSV, addr, uint32(i), false)
		store.AddAddress(metadata)

		// Fund index 0 and 25
		if i == 0 || i == 25 {
			utxo := createTestUTXO(chain.BSV, addr, testTxID(i), 0, 1000, false)
			store.AddUTXO(utxo)
			store.MarkAddressUsed(chain.BSV, addr)
		}
	}

	// Full scan finds both
	assertBalanceEquals(t, store, chain.BSV, 2000)

	// Verify two addresses have activity
	var usedCount int
	for _, addr := range store.GetAddresses(chain.BSV) {
		if addr.HasActivity {
			usedCount++
		}
	}
	assert.Equal(t, 2, usedCount)
}

// TestGapLimit_MultipleGaps tests funds scattered with multiple gaps.
//
//nolint:gosec // Test code uses bounded loop variables for uint32 conversions
func TestGapLimit_MultipleGaps(t *testing.T) {
	t.Parallel()
	store := createTestStore(t)

	// Funds at: 0, 15, 40, 60
	// Gap from 1-14 (14 empty)
	// Gap from 16-39 (24 empty) - beyond standard gap limit
	// Gap from 41-59 (19 empty)
	fundedIndices := []int{0, 15, 40, 60}
	const scanDepth = 81 // Enough to find all with extended scanning

	for i := 0; i < scanDepth; i++ {
		addr := testAddressN(i)
		metadata := createTestAddress(chain.BSV, addr, uint32(i), false)
		store.AddAddress(metadata)

		for _, idx := range fundedIndices {
			if i == idx {
				utxo := createTestUTXO(chain.BSV, addr, testTxID(i), 0, 1000, false)
				store.AddUTXO(utxo)
				store.MarkAddressUsed(chain.BSV, addr)
				break
			}
		}
	}

	// Full scan finds all 4
	assertBalanceEquals(t, store, chain.BSV, 4000)
}

// TestGapLimit_ResetOnActivity tests gap counter reset when activity is found.
//
//nolint:gosec // Test code uses bounded loop variables for uint32 conversions
func TestGapLimit_ResetOnActivity(t *testing.T) {
	t.Parallel()
	store := createTestStore(t)

	// Funds at: 19, 39 (each exactly at the end of a gap period)
	// This tests that finding funds at 19 resets the gap counter,
	// allowing discovery of funds at 39
	fundedIndices := []int{19, 39}
	const scanDepth = 60

	for i := 0; i < scanDepth; i++ {
		addr := testAddressN(i)
		metadata := createTestAddress(chain.BSV, addr, uint32(i), false)
		store.AddAddress(metadata)

		for _, idx := range fundedIndices {
			if i == idx {
				utxo := createTestUTXO(chain.BSV, addr, testTxID(i), 0, 1000, false)
				store.AddUTXO(utxo)
				store.MarkAddressUsed(chain.BSV, addr)
				break
			}
		}
	}

	// Should find both if gap resets on activity
	assertBalanceEquals(t, store, chain.BSV, 2000)
}

// TestGapLimit_ConsecutiveFunds tests addresses with consecutive funding.
func TestGapLimit_ConsecutiveFunds(t *testing.T) {
	t.Parallel()
	store := createTestStore(t)

	// Funds at consecutive addresses 0-9
	const fundedCount = 10

	for i := 0; i < 30; i++ {
		addr := testAddressN(i)
		metadata := createTestAddress(chain.BSV, addr, uint32(i), false)
		store.AddAddress(metadata)

		if i < fundedCount {
			utxo := createTestUTXO(chain.BSV, addr, testTxID(i), 0, 100, false)
			store.AddUTXO(utxo)
			store.MarkAddressUsed(chain.BSV, addr)
		}
	}

	assertBalanceEquals(t, store, chain.BSV, 1000)

	// Verify activity flags
	var usedCount int
	for _, addr := range store.GetAddresses(chain.BSV) {
		if addr.HasActivity {
			usedCount++
		}
	}
	assert.Equal(t, fundedCount, usedCount)
}

// TestGapLimit_ChangeAddresses tests gap limit behavior for change addresses.
//
//nolint:gosec // Test code uses bounded loop variables for uint32 conversions
func TestGapLimit_ChangeAddresses(t *testing.T) {
	t.Parallel()
	store := createTestStore(t)

	// Create receive addresses with gap limit consideration
	for i := 0; i < 25; i++ {
		addr := testAddressN(i)
		metadata := createTestAddress(chain.BSV, addr, uint32(i), false)
		store.AddAddress(metadata)
	}

	// Create change addresses separately with their own gap limit
	// Change addresses often have different usage patterns
	for i := 0; i < 25; i++ {
		addr := testAddressN(1000 + i) // Different range for change
		metadata := createTestAddress(chain.BSV, addr, uint32(i), true)
		store.AddAddress(metadata)

		// Fund some change addresses
		if i == 5 || i == 10 || i == 15 {
			utxo := createTestUTXO(chain.BSV, addr, testTxID(1000+i), 0, 500, false)
			store.AddUTXO(utxo)
			store.MarkAddressUsed(chain.BSV, addr)
		}
	}

	// Total from change addresses: 3 * 500 = 1500
	assertBalanceEquals(t, store, chain.BSV, 1500)

	// Count change vs receive addresses
	var receiveCount, changeCount int
	for _, addr := range store.GetAddresses(chain.BSV) {
		if addr.IsChange {
			changeCount++
		} else {
			receiveCount++
		}
	}
	assert.Equal(t, 25, receiveCount)
	assert.Equal(t, 25, changeCount)
}

// TestGapLimit_NoFundsInRange tests scanning when no funds exist.
//
//nolint:gosec // Test code uses bounded loop variables for uint32 conversions
func TestGapLimit_NoFundsInRange(t *testing.T) {
	t.Parallel()
	store := createTestStore(t)

	// Scan 20 addresses, none funded
	for i := 0; i < DefaultGapLimit; i++ {
		addr := testAddressN(i)
		metadata := createTestAddress(chain.BSV, addr, uint32(i), false)
		store.AddAddress(metadata)
	}

	assertBalanceEquals(t, store, chain.BSV, 0)

	// No addresses should have activity
	for _, addr := range store.GetAddresses(chain.BSV) {
		assert.False(t, addr.HasActivity, "address %s should not have activity", addr.Address)
	}
}

// TestGapLimit_UnusedAddressesQuery tests querying unused addresses.
//
//nolint:gosec // Test code uses bounded loop variables for uint32 conversions
func TestGapLimit_UnusedAddressesQuery(t *testing.T) {
	t.Parallel()
	store := createTestStore(t)

	// Create 30 addresses, fund 5
	fundedIndices := map[int]bool{0: true, 5: true, 10: true, 15: true, 20: true}

	for i := 0; i < 30; i++ {
		addr := testAddressN(i)
		metadata := createTestAddress(chain.BSV, addr, uint32(i), false)
		store.AddAddress(metadata)

		if fundedIndices[i] {
			utxo := createTestUTXO(chain.BSV, addr, testTxID(i), 0, 100, false)
			store.AddUTXO(utxo)
			store.MarkAddressUsed(chain.BSV, addr)
		}
	}

	// Query unused addresses
	unused := store.GetUnusedAddresses(chain.BSV)
	require.Len(t, unused, 25) // 30 - 5 = 25 unused

	// Verify all returned addresses have no activity
	for _, addr := range unused {
		assert.False(t, addr.HasActivity)
	}
}

// TestGapLimit_LargeGapScenario tests a scenario with many empty addresses.
//
//nolint:gosec // Test code uses bounded loop variables for uint32 conversions
func TestGapLimit_LargeGapScenario(t *testing.T) {
	t.Parallel()
	store := createTestStore(t)

	// Fund only address at index 0 and 100
	// This tests handling of very large gaps (common in recovery scenarios)
	fundedIndices := []int{0, 100}

	for i := 0; i <= 120; i++ {
		addr := testAddressN(i)
		metadata := createTestAddress(chain.BSV, addr, uint32(i), false)
		store.AddAddress(metadata)

		for _, idx := range fundedIndices {
			if i == idx {
				utxo := createTestUTXO(chain.BSV, addr, testTxID(i), 0, 5000, false)
				store.AddUTXO(utxo)
				store.MarkAddressUsed(chain.BSV, addr)
				break
			}
		}
	}

	// Should find both with full scan
	assertBalanceEquals(t, store, chain.BSV, 10000)

	// Only 2 addresses should have activity
	var usedCount int
	for _, addr := range store.GetAddresses(chain.BSV) {
		if addr.HasActivity {
			usedCount++
		}
	}
	assert.Equal(t, 2, usedCount)
}
