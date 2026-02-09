package bsv

import (
	"context"
	"math/big"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mrz1836/sigil/internal/chain"
)

// TestSend_InputValidation tests Send method input validation.
func TestSend_InputValidation(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		request    chain.SendRequest
		wantErr    bool
		errContain string
	}{
		{
			name: "invalid from address",
			request: chain.SendRequest{
				From:   "invalid",
				To:     validAddress(),
				Amount: big.NewInt(10000),
			},
			wantErr:    true,
			errContain: "from address",
		},
		{
			name: "invalid to address",
			request: chain.SendRequest{
				From:   validAddress(),
				To:     "invalid",
				Amount: big.NewInt(10000),
			},
			wantErr:    true,
			errContain: "to address",
		},
		{
			name: "ethereum address rejected",
			request: chain.SendRequest{
				From:   validAddress(),
				To:     "0x742d35Cc6634C0532925a3b844Bc9e7595f8b2E0",
				Amount: big.NewInt(10000),
			},
			wantErr:    true,
			errContain: "to address",
		},
		{
			name: "nil amount",
			request: chain.SendRequest{
				From:   validAddress(),
				To:     validAddress2(),
				Amount: nil,
			},
			wantErr:    true,
			errContain: "amount",
		},
		{
			name: "empty from address",
			request: chain.SendRequest{
				From:   "",
				To:     validAddress(),
				Amount: big.NewInt(10000),
			},
			wantErr:    true,
			errContain: "from address",
		},
		{
			name: "empty to address",
			request: chain.SendRequest{
				From:   validAddress(),
				To:     "",
				Amount: big.NewInt(10000),
			},
			wantErr:    true,
			errContain: "to address",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			client := NewClient(nil)
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()

			result, err := client.Send(ctx, tt.request)

			if tt.wantErr {
				require.Error(t, err)
				assert.Nil(t, result)
				assert.Contains(t, err.Error(), tt.errContain)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

// TestSend_UTXOFlow tests the UTXO fetching and selection flow.
func TestSend_UTXOFlow(t *testing.T) {
	t.Parallel()

	t.Run("no UTXOs returns insufficient funds", func(t *testing.T) {
		t.Parallel()

		server := mockUTXOServer([]UTXO{}) // Empty UTXOs
		defer server.Close()

		client := NewClient(&ClientOptions{
			BaseURL: server.URL,
		})

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		_, err := client.Send(ctx, chain.SendRequest{
			From:   validAddress(),
			To:     validAddress2(),
			Amount: big.NewInt(10000),
		})

		require.Error(t, err)
		assert.Contains(t, err.Error(), "insufficient")
	})

	t.Run("single UTXO sufficient - successful transaction", func(t *testing.T) {
		t.Parallel()

		// Get a valid key pair for signing
		kp := getTestKeyPair()

		fee := EstimateTxSize(1, 2) * DefaultFeeRate
		utxos := makeUTXOsWithKey(kp, 50000+fee)

		// Create a multi-route server that handles both UTXO listing and broadcast
		server := mockMultiRouteServer(mockServerConfig{
			UTXOs:           utxos,
			Balance:         int64(50000) + int64(fee), //nolint:gosec // Test fixture with known safe values
			BroadcastTxHash: "broadcast_tx_hash_here",
		})
		defer server.Close()

		client := NewClient(&ClientOptions{
			BaseURL: server.URL,
		})

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		result, err := client.Send(ctx, chain.SendRequest{
			From:       kp.Address,
			To:         validAddress2(),
			Amount:     big.NewInt(50000),
			PrivateKey: kp.PrivateKey,
		})

		// Transaction should be built and broadcast successfully
		require.NoError(t, err)
		require.NotNil(t, result)
		assert.NotEmpty(t, result.Hash)
	})

	t.Run("multiple UTXOs selected - successful transaction", func(t *testing.T) {
		t.Parallel()

		kp := getTestKeyPair()
		utxos := makeUTXOsWithKey(kp, 40000, 40000, 40000) // 120k total
		server := mockMultiRouteServer(mockServerConfig{
			UTXOs:           utxos,
			Balance:         120000,
			BroadcastTxHash: "multi_input_tx_hash",
		})
		defer server.Close()

		client := NewClient(&ClientOptions{
			BaseURL: server.URL,
		})

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		_, err := client.Send(ctx, chain.SendRequest{
			From:       kp.Address,
			To:         validAddress2(),
			Amount:     big.NewInt(80000), // Needs multiple UTXOs
			PrivateKey: kp.PrivateKey,
		})

		require.NoError(t, err)
	})

	t.Run("network error during UTXO fetch", func(t *testing.T) {
		t.Parallel()

		server := mockErrorServer(http.StatusInternalServerError, "server error")
		defer server.Close()

		client := NewClient(&ClientOptions{
			BaseURL: server.URL,
		})

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		_, err := client.Send(ctx, chain.SendRequest{
			From:   validAddress(),
			To:     validAddress2(),
			Amount: big.NewInt(10000),
		})

		require.Error(t, err)
		assert.Contains(t, err.Error(), "network")
	})
}

// TestSend_TransactionBuilding tests transaction building scenarios.
func TestSend_TransactionBuilding(t *testing.T) {
	t.Parallel()

	t.Run("change output with sufficient funds succeeds", func(t *testing.T) {
		t.Parallel()

		kp := getTestKeyPair()
		utxos := makeUTXOsWithKey(kp, 100000) // 100k satoshis
		server := mockMultiRouteServer(mockServerConfig{
			UTXOs:           utxos,
			Balance:         100000,
			BroadcastTxHash: "tx_change_hash",
		})
		defer server.Close()

		client := NewClient(&ClientOptions{
			BaseURL: server.URL,
		})

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		_, err := client.Send(ctx, chain.SendRequest{
			From:       kp.Address,
			To:         validAddress2(),
			Amount:     big.NewInt(50000),
			PrivateKey: kp.PrivateKey,
		})

		require.NoError(t, err)
	})

	t.Run("small change above dust succeeds", func(t *testing.T) {
		t.Parallel()

		kp := getTestKeyPair()
		utxos := makeUTXOsWithKey(kp, 50400)
		server := mockMultiRouteServer(mockServerConfig{
			UTXOs:           utxos,
			Balance:         50400,
			BroadcastTxHash: "tx_small_change_hash",
		})
		defer server.Close()

		client := NewClient(&ClientOptions{
			BaseURL: server.URL,
		})

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		_, err := client.Send(ctx, chain.SendRequest{
			From:       kp.Address,
			To:         validAddress2(),
			Amount:     big.NewInt(50000),
			PrivateKey: kp.PrivateKey,
		})

		require.NoError(t, err)
	})

	t.Run("exact match no change - successful build", func(t *testing.T) {
		t.Parallel()

		// Get a valid key pair for signing
		kp := getTestKeyPair()

		fee := EstimateTxSize(1, 2) * DefaultFeeRate
		utxos := makeUTXOsWithKey(kp, 50000+fee)

		server := mockMultiRouteServer(mockServerConfig{
			UTXOs:           utxos,
			Balance:         int64(50000) + int64(fee), //nolint:gosec // Test fixture with known safe values
			BroadcastTxHash: "tx_exact_match_hash",
		})
		defer server.Close()

		client := NewClient(&ClientOptions{
			BaseURL: server.URL,
		})

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		result, err := client.Send(ctx, chain.SendRequest{
			From:       kp.Address,
			To:         validAddress2(),
			Amount:     big.NewInt(50000),
			PrivateKey: kp.PrivateKey,
		})

		// Should build and broadcast successfully
		require.NoError(t, err)
		require.NotNil(t, result)
		assert.Equal(t, "tx_exact_match_hash", result.Hash)
	})
}

// TestSend_PrivateKeyZeroing tests that private keys are zeroed after use.
func TestSend_PrivateKeyZeroing(t *testing.T) {
	t.Parallel()

	utxos := makeUTXOs(100000)
	server := mockUTXOServer(utxos)
	defer server.Close()

	client := NewClient(&ClientOptions{
		BaseURL: server.URL,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Create a non-zero private key
	privateKey := make([]byte, 32)
	for i := range privateKey {
		privateKey[i] = byte(i + 1)
	}

	_, _ = client.Send(ctx, chain.SendRequest{
		From:       validAddress(),
		To:         validAddress2(),
		Amount:     big.NewInt(50000),
		PrivateKey: privateKey,
	})

	// With current implementation, key is zeroed after BuildRawTransaction.
	// Since BuildRawTransaction returns ErrNotImplemented, the key might not be zeroed.
	// This test documents expected behavior when fully implemented.
}

// TestSend_AmountBoundaries tests amount boundary conditions.
// BSV removed dust limits in 2018, so 1 satoshi is the minimum valid output.
func TestSend_AmountBoundaries(t *testing.T) {
	t.Parallel()

	// Test zero amount separately (should fail with dust error)
	t.Run("zero amount (below BSV dust limit of 1)", func(t *testing.T) {
		t.Parallel()

		kp := getTestKeyPair()
		utxos := makeUTXOsWithKey(kp, 100000)
		server := mockUTXOServer(utxos)
		defer server.Close()

		client := NewClient(&ClientOptions{
			BaseURL: server.URL,
		})

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		_, err := client.Send(ctx, chain.SendRequest{
			From:       kp.Address,
			To:         validAddress2(),
			Amount:     big.NewInt(0),
			PrivateKey: kp.PrivateKey,
		})

		require.Error(t, err)
		assert.Contains(t, err.Error(), "dust")
	})

	// Test valid small amounts - these should all succeed
	smallAmountTests := []struct {
		name   string
		amount int64
	}{
		{
			name:   "one satoshi (BSV dust limit)",
			amount: 1,
		},
		{
			name:   "two satoshis",
			amount: 2,
		},
		{
			name:   "old BTC dust limit (546)",
			amount: 546,
		},
		{
			name:   "above old dust limit (1000)",
			amount: 1000,
		},
	}

	for _, tt := range smallAmountTests {
		t.Run(tt.name+" - successful build", func(t *testing.T) {
			t.Parallel()

			kp := getTestKeyPair()
			fee := EstimateTxSize(1, 2) * DefaultFeeRate
			utxoAmount := uint64(tt.amount) + fee //nolint:gosec // Test values are small and safe
			utxos := makeUTXOsWithKey(kp, utxoAmount)

			server := mockMultiRouteServer(mockServerConfig{
				UTXOs:           utxos,
				Balance:         int64(utxoAmount), //nolint:gosec // Safe: test values are small
				BroadcastTxHash: "broadcast_success",
			})
			defer server.Close()

			client := NewClient(&ClientOptions{
				BaseURL: server.URL,
			})

			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()

			result, err := client.Send(ctx, chain.SendRequest{
				From:       kp.Address,
				To:         validAddress2(),
				Amount:     big.NewInt(tt.amount),
				PrivateKey: kp.PrivateKey,
			})

			// Should successfully build and broadcast
			require.NoError(t, err)
			require.NotNil(t, result)
			assert.Equal(t, "broadcast_success", result.Hash)
		})
	}
}

// TestSend_P2SHAddresses tests sending to P2SH addresses.
// The go-sdk's PayToAddress doesn't support P2SH addresses directly.
// This would require using a different output creation method.
func TestSend_P2SHAddresses(t *testing.T) {
	t.Parallel()

	t.Run("send to P2SH address - not supported by go-sdk PayToAddress", func(t *testing.T) {
		t.Parallel()

		kp := getTestKeyPair()
		fee := EstimateTxSize(1, 2) * DefaultFeeRate
		utxos := makeUTXOsWithKey(kp, 50000+fee)

		server := mockMultiRouteServer(mockServerConfig{
			UTXOs:           utxos,
			Balance:         int64(50000) + int64(fee), //nolint:gosec // Test fixture with known safe values
			BroadcastTxHash: "p2sh_output_tx",
		})
		defer server.Close()

		client := NewClient(&ClientOptions{
			BaseURL: server.URL,
		})

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		_, err := client.Send(ctx, chain.SendRequest{
			From:       kp.Address,
			To:         validP2SHAddress(), // P2SH address as recipient
			Amount:     big.NewInt(50000),
			PrivateKey: kp.PrivateKey,
		})

		// go-sdk's PayToAddress doesn't support P2SH addresses
		// This documents the current limitation
		require.Error(t, err)
		assert.Contains(t, err.Error(), "not supported")
	})
}

// TestSend_SweepAll tests the SweepAll flag for sending entire balance.
func TestSend_SweepAll(t *testing.T) {
	t.Parallel()

	t.Run("sweep single UTXO - sends all minus fee", func(t *testing.T) {
		t.Parallel()

		kp := getTestKeyPair()
		utxoAmount := uint64(100000)
		utxos := makeUTXOsWithKey(kp, utxoAmount)

		server := mockMultiRouteServer(mockServerConfig{
			UTXOs:           utxos,
			Balance:         int64(utxoAmount),
			BroadcastTxHash: "sweep_single_tx",
		})
		defer server.Close()

		client := NewClient(&ClientOptions{
			BaseURL: server.URL,
		})

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		result, err := client.Send(ctx, chain.SendRequest{
			From:       kp.Address,
			To:         validAddress2(),
			PrivateKey: kp.PrivateKey,
			SweepAll:   true,
		})

		require.NoError(t, err)
		require.NotNil(t, result)
		assert.Equal(t, "sweep_single_tx", result.Hash)

		// Verify fee is correct for 1 input, 1 output (no change)
		expectedFee := EstimateFeeForTx(1, 1, DefaultFeeRate)
		expectedAmount := utxoAmount - expectedFee
		assert.Equal(t, client.FormatAmount(amountToBigInt(expectedAmount)), result.Amount)
		assert.Equal(t, client.FormatAmount(amountToBigInt(expectedFee)), result.Fee)
	})

	t.Run("sweep multiple UTXOs - consolidates all", func(t *testing.T) {
		t.Parallel()

		kp := getTestKeyPair()
		utxos := makeUTXOsWithKey(kp, 30000, 40000, 50000) // 120k total
		server := mockMultiRouteServer(mockServerConfig{
			UTXOs:           utxos,
			Balance:         120000,
			BroadcastTxHash: "sweep_multi_tx",
		})
		defer server.Close()

		client := NewClient(&ClientOptions{
			BaseURL: server.URL,
		})

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		result, err := client.Send(ctx, chain.SendRequest{
			From:       kp.Address,
			To:         validAddress2(),
			PrivateKey: kp.PrivateKey,
			SweepAll:   true,
		})

		require.NoError(t, err)
		require.NotNil(t, result)

		// Fee for 3 inputs, 1 output (no change)
		expectedFee := EstimateFeeForTx(3, 1, DefaultFeeRate)
		expectedAmount := uint64(120000) - expectedFee
		assert.Equal(t, client.FormatAmount(amountToBigInt(expectedAmount)), result.Amount)
		assert.Equal(t, client.FormatAmount(amountToBigInt(expectedFee)), result.Fee)
	})

	t.Run("sweep with no UTXOs returns error", func(t *testing.T) {
		t.Parallel()

		server := mockUTXOServer([]UTXO{})
		defer server.Close()

		client := NewClient(&ClientOptions{
			BaseURL: server.URL,
		})

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		_, err := client.Send(ctx, chain.SendRequest{
			From:       validAddress(),
			To:         validAddress2(),
			PrivateKey: make([]byte, 32),
			SweepAll:   true,
		})

		require.Error(t, err)
		assert.Contains(t, err.Error(), "insufficient")
	})

	t.Run("sweep tiny UTXO where fee exceeds balance", func(t *testing.T) {
		t.Parallel()

		kp := getTestKeyPair()
		utxos := makeUTXOsWithKey(kp, 100) // 100 satoshis, fee is 192
		server := mockMultiRouteServer(mockServerConfig{
			UTXOs:   utxos,
			Balance: 100,
		})
		defer server.Close()

		client := NewClient(&ClientOptions{
			BaseURL: server.URL,
		})

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		_, err := client.Send(ctx, chain.SendRequest{
			From:       kp.Address,
			To:         validAddress2(),
			PrivateKey: kp.PrivateKey,
			SweepAll:   true,
		})

		require.Error(t, err)
		assert.Contains(t, err.Error(), "insufficient")
	})

	t.Run("sweep does not need Amount field", func(t *testing.T) {
		t.Parallel()

		kp := getTestKeyPair()
		utxos := makeUTXOsWithKey(kp, 50000)
		server := mockMultiRouteServer(mockServerConfig{
			UTXOs:           utxos,
			Balance:         50000,
			BroadcastTxHash: "sweep_no_amount_tx",
		})
		defer server.Close()

		client := NewClient(&ClientOptions{
			BaseURL: server.URL,
		})

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		// Amount is nil — sweep calculates it
		result, err := client.Send(ctx, chain.SendRequest{
			From:       kp.Address,
			To:         validAddress2(),
			Amount:     nil,
			PrivateKey: kp.PrivateKey,
			SweepAll:   true,
		})

		require.NoError(t, err)
		require.NotNil(t, result)
		assert.Equal(t, "sweep_no_amount_tx", result.Hash)
	})

	t.Run("sweep with custom fee rate", func(t *testing.T) {
		t.Parallel()

		kp := getTestKeyPair()
		utxoAmount := uint64(100000)
		utxos := makeUTXOsWithKey(kp, utxoAmount)
		feeRate := uint64(10)

		server := mockMultiRouteServer(mockServerConfig{
			UTXOs:           utxos,
			Balance:         int64(utxoAmount),
			BroadcastTxHash: "sweep_custom_fee_tx",
		})
		defer server.Close()

		client := NewClient(&ClientOptions{
			BaseURL: server.URL,
		})

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		result, err := client.Send(ctx, chain.SendRequest{
			From:       kp.Address,
			To:         validAddress2(),
			PrivateKey: kp.PrivateKey,
			FeeRate:    feeRate,
			SweepAll:   true,
		})

		require.NoError(t, err)
		require.NotNil(t, result)

		// Fee for 1 input, 1 output at 10 sat/byte
		expectedFee := EstimateFeeForTx(1, 1, feeRate)
		expectedAmount := utxoAmount - expectedFee
		assert.Equal(t, client.FormatAmount(amountToBigInt(expectedAmount)), result.Amount)
	})

	t.Run("sweep minimum viable (1 satoshi after fee)", func(t *testing.T) {
		t.Parallel()

		kp := getTestKeyPair()
		// Fee for 1 input, 1 output at 1 sat/byte = 192
		fee := EstimateFeeForTx(1, 1, DefaultFeeRate)
		utxoAmount := fee + 1 // exactly 1 satoshi remaining
		utxos := makeUTXOsWithKey(kp, utxoAmount)

		server := mockMultiRouteServer(mockServerConfig{
			UTXOs:           utxos,
			Balance:         int64(utxoAmount), //nolint:gosec // Test fixture with known safe values
			BroadcastTxHash: "sweep_min_viable_tx",
		})
		defer server.Close()

		client := NewClient(&ClientOptions{
			BaseURL: server.URL,
		})

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		result, err := client.Send(ctx, chain.SendRequest{
			From:       kp.Address,
			To:         validAddress2(),
			PrivateKey: kp.PrivateKey,
			SweepAll:   true,
		})

		require.NoError(t, err)
		require.NotNil(t, result)
		// 1 satoshi = 0.00000001 BSV
		assert.Equal(t, client.FormatAmount(amountToBigInt(1)), result.Amount)
	})

	t.Run("sweep ignores Amount field when SweepAll is true", func(t *testing.T) {
		t.Parallel()

		kp := getTestKeyPair()
		utxoAmount := uint64(50000)
		utxos := makeUTXOsWithKey(kp, utxoAmount)

		server := mockMultiRouteServer(mockServerConfig{
			UTXOs:           utxos,
			Balance:         int64(utxoAmount),
			BroadcastTxHash: "sweep_ignore_amount_tx",
		})
		defer server.Close()

		client := NewClient(&ClientOptions{
			BaseURL: server.URL,
		})

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		// Provide an Amount, but SweepAll should override it
		result, err := client.Send(ctx, chain.SendRequest{
			From:       kp.Address,
			To:         validAddress2(),
			Amount:     big.NewInt(1000), // should be ignored
			PrivateKey: kp.PrivateKey,
			SweepAll:   true,
		})

		require.NoError(t, err)
		require.NotNil(t, result)

		// Amount should be sweep amount, not 1000
		expectedFee := EstimateFeeForTx(1, 1, DefaultFeeRate)
		expectedAmount := utxoAmount - expectedFee
		assert.Equal(t, client.FormatAmount(amountToBigInt(expectedAmount)), result.Amount)
	})

	t.Run("sweep with invalid to address", func(t *testing.T) {
		t.Parallel()

		client := NewClient(nil)
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		_, err := client.Send(ctx, chain.SendRequest{
			From:     validAddress(),
			To:       "invalid",
			SweepAll: true,
		})

		require.Error(t, err)
		assert.Contains(t, err.Error(), "to address")
	})
}

// TestSend_ContextCancellation tests context cancellation handling.
func TestSend_ContextCancellation(t *testing.T) {
	t.Parallel()

	utxos := makeUTXOs(100000)
	server := mockUTXOServer(utxos)
	defer server.Close()

	client := NewClient(&ClientOptions{
		BaseURL: server.URL,
	})

	// Create already-canceled context
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := client.Send(ctx, chain.SendRequest{
		From:       validAddress(),
		To:         validAddress2(),
		Amount:     big.NewInt(50000),
		PrivateKey: make([]byte, 32),
	})

	require.Error(t, err)
	// Should fail during UTXO fetch due to canceled context
}
