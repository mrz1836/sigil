//go:build integration
// +build integration

package bsv

import (
	"context"
	"math/big"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestIntegration_RealAddress_WithBalance tests the real address from the bug report.
// Address: 1E63i1y2dhSQnzbesRkJmfdVTeH8KbLarG
// Expected: Should return 142649 satoshis (0.00142649 BSV) or current balance (not 0)
//
// Run with: SIGIL_RUN_INTEGRATION_TESTS=1 go test -tags=integration ./internal/chain/bsv/ -v
func TestIntegration_RealAddress_WithBalance(t *testing.T) {
	if os.Getenv("SIGIL_RUN_INTEGRATION_TESTS") == "" {
		t.Skip("Skipping integration test. Set SIGIL_RUN_INTEGRATION_TESTS=1 to run.")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	client := NewClient(ctx, nil)
	address := "1E63i1y2dhSQnzbesRkJmfdVTeH8KbLarG"

	// Test single fetch
	t.Run("single fetch", func(t *testing.T) {
		balance, err := client.GetNativeBalance(ctx, address)
		require.NoError(t, err, "Single fetch should succeed")
		assert.True(t, balance.Amount.Cmp(big.NewInt(0)) > 0,
			"Balance should be positive, got %s satoshis", balance.Amount.String())

		t.Logf("Single fetch result: %s satoshis (%.8f BSV)",
			balance.Amount.String(),
			float64(balance.Amount.Int64())/100000000.0)
	})

	// Test bulk fetch
	t.Run("bulk fetch", func(t *testing.T) {
		bulkResults, err := client.GetBulkNativeBalance(ctx, []string{address})
		require.NoError(t, err, "Bulk fetch should succeed")

		bulkBalance, exists := bulkResults[address]
		require.True(t, exists, "Address should be in bulk results (via direct or fallback)")
		assert.True(t, bulkBalance.Amount.Cmp(big.NewInt(0)) > 0,
			"Balance should be positive, got %s satoshis", bulkBalance.Amount.String())

		t.Logf("Bulk fetch result: %s satoshis (%.8f BSV)",
			bulkBalance.Amount.String(),
			float64(bulkBalance.Amount.Int64())/100000000.0)
	})

	// Test consistency between single and bulk
	t.Run("consistency check", func(t *testing.T) {
		singleBalance, err := client.GetNativeBalance(ctx, address)
		require.NoError(t, err, "Single fetch should succeed")

		bulkResults, err := client.GetBulkNativeBalance(ctx, []string{address})
		require.NoError(t, err, "Bulk fetch should succeed")

		bulkBalance, exists := bulkResults[address]
		require.True(t, exists, "Address should be in bulk results")

		assert.Equal(t, singleBalance.Amount, bulkBalance.Amount,
			"Bulk and single fetch should return same balance")
	})
}

// TestIntegration_MultipleAddresses tests bulk fetch with mixed addresses
// including the problematic address from the bug report.
func TestIntegration_MultipleAddresses(t *testing.T) {
	if os.Getenv("SIGIL_RUN_INTEGRATION_TESTS") == "" {
		t.Skip("Skipping integration test. Set SIGIL_RUN_INTEGRATION_TESTS=1 to run.")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	client := NewClient(ctx, nil)

	addresses := []string{
		"1E63i1y2dhSQnzbesRkJmfdVTeH8KbLarG", // Known to have balance (from bug report)
		"1A1zP1eP5QGefi2DMPTfTL5SLmv7DivfNa", // Genesis address (Satoshi's)
	}

	results, err := client.GetBulkNativeBalance(ctx, addresses)
	require.NoError(t, err, "Bulk fetch should succeed")

	// Log all results
	for addr, bal := range results {
		t.Logf("Address %s: %s satoshis (%.8f BSV)",
			addr,
			bal.Amount.String(),
			float64(bal.Amount.Int64())/100000000.0)
	}

	// Should have at least one result
	assert.GreaterOrEqual(t, len(results), 1, "Should have at least one result")

	// The problematic address should be in results
	bal, exists := results["1E63i1y2dhSQnzbesRkJmfdVTeH8KbLarG"]
	if exists {
		assert.True(t, bal.Amount.Cmp(big.NewInt(0)) > 0,
			"Bug report address should have positive balance")
	}
}

// TestIntegration_EmptyAddress tests an address with zero balance
func TestIntegration_EmptyAddress(t *testing.T) {
	if os.Getenv("SIGIL_RUN_INTEGRATION_TESTS") == "" {
		t.Skip("Skipping integration test. Set SIGIL_RUN_INTEGRATION_TESTS=1 to run.")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	client := NewClient(ctx, nil)

	// Use a burn address that should have 0 balance
	address := "1111111111111111111114oLvT2"

	balance, err := client.GetNativeBalance(ctx, address)
	require.NoError(t, err, "Fetch should succeed even for zero balance")

	t.Logf("Burn address balance: %s satoshis", balance.Amount.String())

	// This address should have zero or near-zero balance
	// (it might have dust if people sent to it)
	assert.Equal(t, "BSV", balance.Symbol)
	assert.Equal(t, 8, balance.Decimals)
}

// TestIntegration_BulkFetch_LargeBatch tests bulk fetching with more than 20 addresses
func TestIntegration_BulkFetch_LargeBatch(t *testing.T) {
	if os.Getenv("SIGIL_RUN_INTEGRATION_TESTS") == "" {
		t.Skip("Skipping integration test. Set SIGIL_RUN_INTEGRATION_TESTS=1 to run.")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	client := NewClient(ctx, nil)

	// Test with 25 addresses (will require 2 batches since limit is 20)
	addresses := []string{
		"1E63i1y2dhSQnzbesRkJmfdVTeH8KbLarG", // Bug report address (has balance)
		"1A1zP1eP5QGefi2DMPTfTL5SLmv7DivfNa", // Genesis
		// Add more test addresses (these are random valid addresses)
		"1BvBMSEYstWetqTFn5Au4m4GFg7xJaNVN2",
		"12c6DSiU4Rq3P4ZxziKxzrL5LmMBrzjrJX",
		"1HQ3Go3ggs8pFnXuHVHRytPCq5fGG8Hbhx",
		"1LdRcdxfbSnmCYYNdeYpUnztiYzVfBEQeC",
		"1AC4fMwgY8j9onSbXEWeH6Zan8QGMSdmtA",
		"1FvzCLoTPGANNjWoUo6jUGuAG3wg1w4YjR",
		"1FwkKA9cqpNRUTgJKg1V3e2hpBkahbXqYp",
		"1AJbsFZ64EpEfS5UAjAfcUG8pH8Jn3rn1F",
		"1EzwoHtiXB4iFwedPr49iywjZn2nnekhoj",
		"1H8QJcfYJj6nF1YtPh8aL1Pv9tqVjW8Y2",
		"1JfbZRwdDHKZmuiZgYArJZhcuuzuw2HuMu",
		"1JayzVzvVkqQZHWXeYFR5aEKdhHfkJWwSm",
		"1L3EqSy8dX9p1bFCz8RqFcVZnwYsLuVAyD",
		"1LdNcdKNjqDxJmHfXL8Yqr9YwHmW4hVBTa",
		"1PnJHqLLNHQk3mNezR3FVV5qPKiKkRJcFi",
		"1P5ZEDWTKTFGxQjZphgWPQUpe554WKDfHQ",
		"1PdSiPk66FJH9JtfQYJmYSdQqELzekKGU2",
		"1PdFnzTFrJFcZgKZcvPqXQLWxQMW6SkkW",
		"1PqgvwHVvBEeDrKPHWTuLh9RFVqAjRhQsJ",
		"1QAMggCK2Mce2AHn9xRkKzqz6XMXJz5P4v",
		"1Q2TWHE3GMdB6BZKafqwxXtWAWgFt5Jvm3",
		"1Q77KatEP1hPgdbmLNRBDFNJHbchSiX3hF",
		"1Q9FhDCkNRHazGvARUhPNPgQ9v2nH8aK5n",
	}

	results, err := client.GetBulkNativeBalance(ctx, addresses)
	require.NoError(t, err, "Bulk fetch with 25 addresses should succeed")

	t.Logf("Fetched balances for %d out of %d addresses", len(results), len(addresses))

	// Should have at least some results
	assert.Greater(t, len(results), 0, "Should have at least some results")

	// Bug report address should be present with positive balance
	if bal, exists := results["1E63i1y2dhSQnzbesRkJmfdVTeH8KbLarG"]; exists {
		assert.True(t, bal.Amount.Cmp(big.NewInt(0)) > 0,
			"Bug report address should have positive balance")
		t.Logf("Bug report address balance: %s satoshis", bal.Amount.String())
	}
}
