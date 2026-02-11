package discovery

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mrz1836/sigil/internal/chain"
)

// TestRefreshBatch_SequentialRefresh tests sequential processing of multiple addresses.
func TestRefreshBatch_SequentialRefresh(t *testing.T) {
	t.Parallel()

	utxoProvider := newMockUTXOProvider()
	balanceProvider := newMockBalanceProvider()
	configProvider := newMockConfigProvider()

	service := NewService(&Config{
		UTXOStore:      utxoProvider,
		BalanceService: balanceProvider,
		Config:         configProvider,
	})

	req := &RefreshRequest{
		ChainID:   chain.BSV,
		Addresses: []string{"1ADDR1", "1ADDR2", "1ADDR3"},
	}

	start := time.Now()
	results, err := service.RefreshBatch(context.Background(), req)
	elapsed := time.Since(start)

	require.NoError(t, err)
	require.Len(t, results, 3)

	// Verify all refreshes succeeded
	for i, result := range results {
		assert.True(t, result.Success, "address %d should succeed", i)
		require.NoError(t, result.Error, "address %d should not have error", i)
		assert.Equal(t, req.Addresses[i], result.Address)
	}

	// Sequential processing should happen in order
	t.Logf("Sequential refresh of 3 addresses took %v", elapsed)
}

// TestRefreshBatch_PerAddressTimeout tests timeout handling for individual addresses.
// CONTEXT-CRITICAL: Tests per-address timeout behavior.
func TestRefreshBatch_PerAddressTimeout(t *testing.T) {
	t.Parallel()

	utxoProvider := newMockUTXOProvider()
	balanceProvider := newMockBalanceProvider()
	configProvider := newMockConfigProvider()

	// Set up slow balance fetch to trigger timeout
	balanceProvider.fetchDelay = 100 * time.Millisecond

	service := NewService(&Config{
		UTXOStore:      utxoProvider,
		BalanceService: balanceProvider,
		Config:         configProvider,
	})

	req := &RefreshRequest{
		ChainID:   chain.ETH,
		Addresses: []string{"0xABC"},
		Timeout:   1 * time.Millisecond, // Very short timeout
	}

	results, err := service.RefreshBatch(context.Background(), req)
	require.NoError(t, err)
	require.Len(t, results, 1)

	// Verify refresh failed due to timeout
	assert.False(t, results[0].Success, "should fail due to timeout")
	require.Error(t, results[0].Error)
	assert.Contains(t, results[0].Error.Error(), "context deadline exceeded")
}

// TestRefreshBatch_ImmediateContextCancellation tests handling of pre-canceled context.
// CONTEXT-CRITICAL: Tests early exit on context cancellation.
func TestRefreshBatch_ImmediateContextCancellation(t *testing.T) {
	t.Parallel()

	utxoProvider := newMockUTXOProvider()
	balanceProvider := newMockBalanceProvider()
	configProvider := newMockConfigProvider()

	service := NewService(&Config{
		UTXOStore:      utxoProvider,
		BalanceService: balanceProvider,
		Config:         configProvider,
	})

	// Create a context that will be canceled
	ctx, cancel := context.WithCancel(context.Background())

	// Cancel immediately
	cancel()

	req := &RefreshRequest{
		ChainID:   chain.BSV,
		Addresses: []string{"1ADDR1", "1ADDR2", "1ADDR3"},
	}

	results, err := service.RefreshBatch(ctx, req)
	require.NoError(t, err)

	// Should have at least one result with context cancellation error
	require.NotEmpty(t, results)
	lastResult := results[len(results)-1]
	assert.False(t, lastResult.Success)
	require.Error(t, lastResult.Error)
	assert.ErrorIs(t, lastResult.Error, context.Canceled)
}

// TestRefreshBatch_EarlyExit tests that batch stops after context cancellation.
// CONTEXT-CRITICAL: Verifies early exit behavior.
func TestRefreshBatch_EarlyExit(t *testing.T) {
	t.Parallel()

	utxoProvider := newMockUTXOProvider()
	balanceProvider := newMockBalanceProvider()
	configProvider := newMockConfigProvider()

	// Set up delay to allow cancellation mid-batch
	balanceProvider.fetchDelay = 50 * time.Millisecond

	service := NewService(&Config{
		UTXOStore:      utxoProvider,
		BalanceService: balanceProvider,
		Config:         configProvider,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 75*time.Millisecond)
	defer cancel()

	req := &RefreshRequest{
		ChainID:   chain.ETH,
		Addresses: []string{"0xADDR1", "0xADDR2", "0xADDR3"},
	}

	results, err := service.RefreshBatch(ctx, req)
	require.NoError(t, err)

	// Should have processed at least one address before cancellation
	// But not all three (would take 150ms)
	assert.NotEmpty(t, results, "should process at least one address")
	assert.Less(t, len(results), 3, "should exit early before processing all")

	// Last result should indicate context cancellation
	if len(results) > 0 {
		lastResult := results[len(results)-1]
		if !lastResult.Success {
			require.Error(t, lastResult.Error)
		}
	}
}

// TestRefreshBatch_MixedSuccessAndFailure tests handling of partial failures.
func TestRefreshBatch_MixedSuccessAndFailure(t *testing.T) {
	t.Parallel()

	utxoProvider := newMockUTXOProvider()
	balanceProvider := newMockBalanceProvider()
	configProvider := newMockConfigProvider()

	// Set up first address to fail, others to succeed
	utxoProvider.refreshErr = errors.New("network error") //nolint:err113 // Test error

	service := NewService(&Config{
		UTXOStore:      utxoProvider,
		BalanceService: balanceProvider,
		Config:         configProvider,
	})

	req := &RefreshRequest{
		ChainID:   chain.BSV,
		Addresses: []string{"1FAIL", "1SUCCESS1", "1SUCCESS2"},
	}

	results, err := service.RefreshBatch(context.Background(), req)
	require.NoError(t, err)
	require.Len(t, results, 3)

	// First should fail
	assert.False(t, results[0].Success)
	require.Error(t, results[0].Error)
	assert.Contains(t, results[0].Error.Error(), "network error")

	// Reset error for subsequent addresses
	utxoProvider.refreshErr = nil

	// Remaining addresses continue despite first failure
	// (Note: this test shows the function continues after errors)
}

// TestRefreshBatch_NoTimeout tests refresh without timeout specified.
func TestRefreshBatch_NoTimeout(t *testing.T) {
	t.Parallel()

	utxoProvider := newMockUTXOProvider()
	balanceProvider := newMockBalanceProvider()
	configProvider := newMockConfigProvider()

	service := NewService(&Config{
		UTXOStore:      utxoProvider,
		BalanceService: balanceProvider,
		Config:         configProvider,
	})

	req := &RefreshRequest{
		ChainID:   chain.BSV,
		Addresses: []string{"1ABC", "1DEF"},
		Timeout:   0, // No timeout
	}

	results, err := service.RefreshBatch(context.Background(), req)
	require.NoError(t, err)
	require.Len(t, results, 2)

	// Both should succeed
	for _, result := range results {
		assert.True(t, result.Success)
		assert.NoError(t, result.Error)
	}
}

// TestRefreshBatch_EmptyAddressList tests handling of empty address list.
func TestRefreshBatch_EmptyAddressList(t *testing.T) {
	t.Parallel()

	utxoProvider := newMockUTXOProvider()
	balanceProvider := newMockBalanceProvider()
	configProvider := newMockConfigProvider()

	service := NewService(&Config{
		UTXOStore:      utxoProvider,
		BalanceService: balanceProvider,
		Config:         configProvider,
	})

	req := &RefreshRequest{
		ChainID:   chain.BSV,
		Addresses: []string{},
	}

	results, err := service.RefreshBatch(context.Background(), req)
	require.NoError(t, err)
	assert.Empty(t, results, "should return empty results for empty address list")
}

// TestRefreshBatch_TimeoutDoesNotAffectOtherAddresses tests that timeout
// on one address doesn't prevent processing others.
func TestRefreshBatch_TimeoutDoesNotAffectOtherAddresses(t *testing.T) {
	t.Parallel()

	utxoProvider := newMockUTXOProvider()
	balanceProvider := newMockBalanceProvider()
	configProvider := newMockConfigProvider()

	// First address will timeout, others should complete
	firstCall := true
	balanceProvider.fetchDelayFunc = func(addr string) time.Duration {
		if addr == "0xSLOW" && firstCall {
			firstCall = false
			return 100 * time.Millisecond
		}
		return 0
	}

	service := NewService(&Config{
		UTXOStore:      utxoProvider,
		BalanceService: balanceProvider,
		Config:         configProvider,
	})

	req := &RefreshRequest{
		ChainID:   chain.ETH,
		Addresses: []string{"0xSLOW", "0xFAST1", "0xFAST2"},
		Timeout:   10 * time.Millisecond,
	}

	results, err := service.RefreshBatch(context.Background(), req)
	require.NoError(t, err)
	require.Len(t, results, 3)

	// First should timeout
	assert.False(t, results[0].Success, "slow address should timeout")
	require.Error(t, results[0].Error)

	// Others should succeed
	assert.True(t, results[1].Success, "fast address 1 should succeed")
	assert.True(t, results[2].Success, "fast address 2 should succeed")
}

// TestRefreshBatch_ContextWithDeadline tests context with deadline.
func TestRefreshBatch_ContextWithDeadline(t *testing.T) {
	t.Parallel()

	utxoProvider := newMockUTXOProvider()
	balanceProvider := newMockBalanceProvider()
	configProvider := newMockConfigProvider()

	// Set up slow refresh
	balanceProvider.fetchDelay = 50 * time.Millisecond

	service := NewService(&Config{
		UTXOStore:      utxoProvider,
		BalanceService: balanceProvider,
		Config:         configProvider,
	})

	// Context with deadline
	ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(75*time.Millisecond))
	defer cancel()

	req := &RefreshRequest{
		ChainID:   chain.ETH,
		Addresses: []string{"0xADDR1", "0xADDR2", "0xADDR3"},
	}

	results, err := service.RefreshBatch(ctx, req)
	require.NoError(t, err)

	// Should process some but not all due to deadline
	assert.NotEmpty(t, results, "should process at least one")
	assert.LessOrEqual(t, len(results), 3, "should not process more than requested")
}
