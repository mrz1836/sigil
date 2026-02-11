package eth

import (
	"math"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const testAddress = "0x742d35Cc6634C0532925a3b844Bc454e4438f44e"

// TestNewNonceManager verifies basic construction.
func TestNewNonceManager(t *testing.T) {
	t.Parallel()
	nm := NewNonceManager()
	require.NotNil(t, nm)
	require.NotNil(t, nm.nonces)
}

// TestNonceManager_SingleAddressProgression tests normal nonce progression for a single address.
func TestNonceManager_SingleAddressProgression(t *testing.T) {
	t.Parallel()
	nm := NewNonceManager()
	addr := testAddress

	// First call with RPC nonce 0 → should return 0
	nonce := nm.Next(addr, 0)
	assert.Equal(t, uint64(0), nonce)

	// Second call with RPC nonce 0 → should return 1 (local incremented)
	nonce = nm.Next(addr, 0)
	assert.Equal(t, uint64(1), nonce)

	// Third call with RPC nonce 0 → should return 2
	nonce = nm.Next(addr, 0)
	assert.Equal(t, uint64(2), nonce)

	// Call with RPC nonce 3 → RPC caught up, should return 3
	nonce = nm.Next(addr, 3)
	assert.Equal(t, uint64(3), nonce)

	// Next call with RPC nonce 3 → should return 4
	nonce = nm.Next(addr, 3)
	assert.Equal(t, uint64(4), nonce)
}

// TestNonceManager_MultipleAddresses verifies independent tracking of multiple addresses.
func TestNonceManager_MultipleAddresses(t *testing.T) {
	t.Parallel()
	nm := NewNonceManager()

	addr1 := testAddress
	addr2 := "0x5aAeb6053F3E94C9b9A09f33669435E7Ef1BeAed"
	addr3 := "0xfB6916095ca1df60bB79Ce92cE3Ea74c37c5d359"

	// Get nonces for different addresses
	nonce1 := nm.Next(addr1, 0)
	nonce2 := nm.Next(addr2, 0)
	nonce3 := nm.Next(addr3, 5)

	assert.Equal(t, uint64(0), nonce1)
	assert.Equal(t, uint64(0), nonce2)
	assert.Equal(t, uint64(5), nonce3)

	// Get next nonces - should be independent
	nonce1 = nm.Next(addr1, 0)
	nonce2 = nm.Next(addr2, 0)
	nonce3 = nm.Next(addr3, 5)

	assert.Equal(t, uint64(1), nonce1)
	assert.Equal(t, uint64(1), nonce2)
	assert.Equal(t, uint64(6), nonce3)

	// Interleaved calls should not cross-contaminate
	nm.Next(addr1, 0) // addr1 → 2
	nm.Next(addr2, 0) // addr2 → 2
	nm.Next(addr1, 0) // addr1 → 3
	nm.Next(addr3, 5) // addr3 → 7

	nonce1 = nm.Next(addr1, 0)
	nonce2 = nm.Next(addr2, 0)
	nonce3 = nm.Next(addr3, 5)

	assert.Equal(t, uint64(4), nonce1)
	assert.Equal(t, uint64(3), nonce2)
	assert.Equal(t, uint64(8), nonce3)
}

// TestNonceManager_RPCAdvancement tests various RPC nonce advancement scenarios.
func TestNonceManager_RPCAdvancement(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		rpcSequence []uint64
		expected    []uint64
	}{
		{
			name:        "RPC stays behind local",
			rpcSequence: []uint64{0, 0, 0, 0},
			expected:    []uint64{0, 1, 2, 3},
		},
		{
			name:        "RPC catches up",
			rpcSequence: []uint64{0, 0, 0, 3},
			expected:    []uint64{0, 1, 2, 3},
		},
		{
			name:        "RPC jumps ahead (another client sent txs)",
			rpcSequence: []uint64{0, 0, 10},
			expected:    []uint64{0, 1, 10},
		},
		{
			name:        "RPC synchronized",
			rpcSequence: []uint64{0, 1, 2, 3},
			expected:    []uint64{0, 1, 2, 3},
		},
		{
			name:        "Large RPC jump",
			rpcSequence: []uint64{0, 0, 0, 1000},
			expected:    []uint64{0, 1, 2, 1000},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			nm := NewNonceManager()
			addr := testAddress

			for i, rpcNonce := range tc.rpcSequence {
				nonce := nm.Next(addr, rpcNonce)
				assert.Equal(t, tc.expected[i], nonce, "iteration %d", i)
			}
		})
	}
}

// TestNonceManager_Reset verifies reset functionality.
func TestNonceManager_Reset(t *testing.T) {
	t.Parallel()
	nm := NewNonceManager()
	addr := testAddress

	// Build up some local state
	nm.Next(addr, 0) // 0
	nm.Next(addr, 0) // 1
	nm.Next(addr, 0) // 2

	// Verify local state exists
	nonce := nm.Next(addr, 0)
	assert.Equal(t, uint64(3), nonce)

	// Reset the address
	nm.Reset(addr)

	// Next call should use fresh RPC nonce
	nonce = nm.Next(addr, 5)
	assert.Equal(t, uint64(5), nonce)

	// And then increment from there
	nonce = nm.Next(addr, 5)
	assert.Equal(t, uint64(6), nonce)
}

// TestNonceManager_ResetDoesNotAffectOtherAddresses verifies reset isolation.
func TestNonceManager_ResetDoesNotAffectOtherAddresses(t *testing.T) {
	t.Parallel()
	nm := NewNonceManager()

	addr1 := testAddress
	addr2 := "0x5aAeb6053F3E94C9b9A09f33669435E7Ef1BeAed"

	// Build up state for both addresses
	nm.Next(addr1, 0) // 0
	nm.Next(addr1, 0) // 1
	nm.Next(addr2, 0) // 0
	nm.Next(addr2, 0) // 1

	// Reset addr1
	nm.Reset(addr1)

	// addr1 should be reset
	nonce1 := nm.Next(addr1, 10)
	assert.Equal(t, uint64(10), nonce1)

	// addr2 should be unaffected
	nonce2 := nm.Next(addr2, 0)
	assert.Equal(t, uint64(2), nonce2)
}

// TestNonceManager_ResetNonExistent verifies reset on non-existent address is a no-op.
func TestNonceManager_ResetNonExistent(t *testing.T) {
	t.Parallel()
	nm := NewNonceManager()
	addr := testAddress

	// Reset an address that was never used
	nm.Reset(addr)

	// Should still work normally
	nonce := nm.Next(addr, 5)
	assert.Equal(t, uint64(5), nonce)
}

// TestNonceManager_ConcurrentSingleAddress tests thread-safety for a single address.
// CRITICAL: This verifies no nonce collisions under concurrent access.
func TestNonceManager_ConcurrentSingleAddress(t *testing.T) {
	t.Parallel()
	nm := NewNonceManager()
	addr := testAddress

	const numGoroutines = 100
	var wg sync.WaitGroup
	results := make(chan uint64, numGoroutines)

	// Launch 100 goroutines requesting nonces
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			nonce := nm.Next(addr, 0)
			results <- nonce
		}()
	}

	wg.Wait()
	close(results)

	// Collect all nonces
	nonces := make([]uint64, 0, numGoroutines)
	for nonce := range results {
		nonces = append(nonces, nonce)
	}

	// Verify we got exactly numGoroutines nonces
	assert.Len(t, nonces, numGoroutines)

	// Verify all nonces are unique (no collisions)
	seen := make(map[uint64]bool)
	for _, nonce := range nonces {
		assert.False(t, seen[nonce], "duplicate nonce %d", nonce)
		seen[nonce] = true
	}

	// Verify nonces are in range [0, numGoroutines)
	for _, nonce := range nonces {
		assert.Less(t, nonce, uint64(numGoroutines))
	}
}

// TestNonceManager_ConcurrentMultipleAddresses tests concurrent access across multiple addresses.
func TestNonceManager_ConcurrentMultipleAddresses(t *testing.T) {
	t.Parallel()
	nm := NewNonceManager()

	addresses := []string{
		testAddress,
		"0x5aAeb6053F3E94C9b9A09f33669435E7Ef1BeAed",
		"0xfB6916095ca1df60bB79Ce92cE3Ea74c37c5d359",
	}

	const requestsPerAddress = 50
	var wg sync.WaitGroup

	// Map to collect results per address
	results := make(map[string][]uint64)
	resultsMu := sync.Mutex{}

	// For each address, launch requestsPerAddress goroutines
	for _, addr := range addresses {
		for i := 0; i < requestsPerAddress; i++ {
			wg.Add(1)
			go func(address string) {
				defer wg.Done()
				nonce := nm.Next(address, 0)

				resultsMu.Lock()
				results[address] = append(results[address], nonce)
				resultsMu.Unlock()
			}(addr)
		}
	}

	wg.Wait()

	// Verify each address got unique nonces
	for _, addr := range addresses {
		nonces := results[addr]
		assert.Len(t, nonces, requestsPerAddress, "address %s", addr)

		// Verify all nonces are unique for this address
		seen := make(map[uint64]bool)
		for _, nonce := range nonces {
			assert.False(t, seen[nonce], "address %s has duplicate nonce %d", addr, nonce)
			seen[nonce] = true
		}

		// Verify nonces are in range [0, requestsPerAddress)
		for _, nonce := range nonces {
			assert.Less(t, nonce, uint64(requestsPerAddress), "address %s", addr)
		}
	}
}

// TestNonceManager_ConcurrentResets tests concurrent reset operations.
func TestNonceManager_ConcurrentResets(t *testing.T) {
	t.Parallel()
	nm := NewNonceManager()
	addr := testAddress

	const numRequesters = 50
	const numResetters = 10
	var wg sync.WaitGroup

	// Launch goroutines requesting nonces
	for i := 0; i < numRequesters; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 10; j++ {
				nm.Next(addr, 0)
			}
		}()
	}

	// Launch goroutines calling Reset
	for i := 0; i < numResetters; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 10; j++ {
				nm.Reset(addr)
			}
		}()
	}

	// Should not panic or deadlock
	wg.Wait()

	// Final state should be consistent (no corruption)
	// Just verify we can still use the manager
	nonce := nm.Next(addr, 100)
	assert.GreaterOrEqual(t, nonce, uint64(100))
}

// TestNonceManager_StressTest is a comprehensive stress test with random operations.
func TestNonceManager_StressTest(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping stress test in short mode")
	}
	t.Parallel()

	nm := NewNonceManager()

	// Pool of 10 addresses
	addresses := []string{
		testAddress,
		"0x5aAeb6053F3E94C9b9A09f33669435E7Ef1BeAed",
		"0xfB6916095ca1df60bB79Ce92cE3Ea74c37c5d359",
		"0xdbF03B407c01E7cD3CBea99509d93f8DDDC8C6FB",
		"0xD1220A0cf47c7B9Be7A2E6BA89F429762e7b9aDb",
		"0x2c7536E3605D9C16a7a3D7b1898e529396a65c23",
		"0x8626f6940E2eb28930eFb4CeF49B2d1F2C9C1199",
		"0xdD870fA1b7C4700F2BD7f44238821C26f7392148",
		"0x06012c8cf97BEaD5deAe237070F9587f8E7A266d",
		"0xB8c77482e45F1F44dE1745F52C74426C631bDD52",
	}

	const numGoroutines = 1000
	var wg sync.WaitGroup

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()

			// Pick an address based on goroutine index
			addr := addresses[idx%len(addresses)]

			// Do some operations
			for j := 0; j < 5; j++ {
				// Occasional reset
				if (idx+j)%50 == 0 {
					nm.Reset(addr)
				}

				// Request nonce with varying RPC values
				//nolint:gosec // Intentional modulo for test variation
				rpcNonce := uint64((idx + j) % 20)
				nm.Next(addr, rpcNonce)
			}
		}(i)
	}

	// Should not panic, deadlock, or race
	wg.Wait()

	// Verify manager is still functional
	for _, addr := range addresses {
		_ = nm.Next(addr, 0)
		// Manager still works without panic
	}
}

// TestNonceManager_MaxUint64 tests handling of maximum uint64 nonce.
func TestNonceManager_MaxUint64(t *testing.T) {
	t.Parallel()
	nm := NewNonceManager()
	addr := testAddress

	// Use max uint64 as RPC nonce
	maxNonce := uint64(math.MaxUint64)
	nonce := nm.Next(addr, maxNonce)
	assert.Equal(t, maxNonce, nonce)

	// Next call: local wraps to 0, but RPC is still MaxUint64, so we use RPC (the higher value)
	nonce = nm.Next(addr, maxNonce)
	assert.Equal(t, maxNonce, nonce, "should use RPC nonce as it's higher than wrapped local")

	// If RPC also reports 0 (both wrapped), we'd use local (0)
	nonce = nm.Next(addr, 0)
	assert.Equal(t, uint64(0), nonce, "both wrapped")
}

// TestNonceManager_EmptyAddress tests handling of empty address string.
func TestNonceManager_EmptyAddress(t *testing.T) {
	t.Parallel()
	nm := NewNonceManager()

	// Empty address should work (just another string key)
	nonce := nm.Next("", 5)
	assert.Equal(t, uint64(5), nonce)

	nonce = nm.Next("", 5)
	assert.Equal(t, uint64(6), nonce)
}

// TestNonceManager_CaseSensitivity tests that addresses are case-sensitive.
func TestNonceManager_CaseSensitivity(t *testing.T) {
	t.Parallel()
	nm := NewNonceManager()

	addr1 := testAddress
	addr2 := "0x742D35CC6634C0532925A3B844BC454E4438F44E" // Same address, different case

	// These should be treated as different addresses
	nonce1 := nm.Next(addr1, 0)
	nonce2 := nm.Next(addr2, 0)

	assert.Equal(t, uint64(0), nonce1)
	assert.Equal(t, uint64(0), nonce2)

	// Verify they maintain separate state
	nonce1 = nm.Next(addr1, 0)
	nonce2 = nm.Next(addr2, 0)

	assert.Equal(t, uint64(1), nonce1)
	assert.Equal(t, uint64(1), nonce2)
}

// TestNonceManager_RPCRegression tests RPC nonce regression scenario (network reorg).
func TestNonceManager_RPCRegression(t *testing.T) {
	t.Parallel()
	nm := NewNonceManager()
	addr := testAddress

	// Build up local nonce
	nm.Next(addr, 0) // 0
	nm.Next(addr, 0) // 1
	nm.Next(addr, 0) // 2
	nm.Next(addr, 5) // 5

	// Local should now be 6
	nonce := nm.Next(addr, 0)
	assert.Equal(t, uint64(6), nonce)

	// RPC regresses (network reorg scenario) - local should still be used
	nonce = nm.Next(addr, 3)
	assert.Equal(t, uint64(7), nonce, "should use local nonce even if RPC regresses")
}

// TestNonceManager_RapidSuccession tests rapid transaction sending.
func TestNonceManager_RapidSuccession(t *testing.T) {
	t.Parallel()
	nm := NewNonceManager()
	addr := testAddress

	// Simulate rapid sending where RPC hasn't caught up
	const numTxs = 20
	nonces := make([]uint64, numTxs)

	for i := 0; i < numTxs; i++ {
		nonces[i] = nm.Next(addr, 0) // RPC still reports 0
	}

	// Verify nonces are sequential
	for i := 0; i < numTxs; i++ {
		//nolint:gosec // Loop index conversion is safe for test
		assert.Equal(t, uint64(i), nonces[i], "nonce at index %d", i)
	}
}
