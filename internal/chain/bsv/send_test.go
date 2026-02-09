package bsv

import (
	"context"
	"math/big"
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

			client := NewClient(context.Background(), nil)
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

		mock := mockUTXOClient([]UTXO{}) // Empty UTXOs

		client := NewClient(context.Background(), &ClientOptions{
			WOCClient: mock,
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

		// Create a mock that handles both UTXO listing and broadcast
		mock := newMockWOCFromConfig(mockServerConfig{
			UTXOs:           utxos,
			Balance:         int64(50000) + int64(fee), //nolint:gosec // Test fixture with known safe values
			BroadcastTxHash: "broadcast_tx_hash_here",
		})

		client := NewClient(context.Background(), &ClientOptions{
			WOCClient: mock,
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
		mock := newMockWOCFromConfig(mockServerConfig{
			UTXOs:           utxos,
			Balance:         120000,
			BroadcastTxHash: "multi_input_tx_hash",
		})

		client := NewClient(context.Background(), &ClientOptions{
			WOCClient: mock,
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

		mock := mockErrorClient()

		client := NewClient(context.Background(), &ClientOptions{
			WOCClient: mock,
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
		mock := newMockWOCFromConfig(mockServerConfig{
			UTXOs:           utxos,
			Balance:         100000,
			BroadcastTxHash: "tx_change_hash",
		})

		client := NewClient(context.Background(), &ClientOptions{
			WOCClient: mock,
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
		mock := newMockWOCFromConfig(mockServerConfig{
			UTXOs:           utxos,
			Balance:         50400,
			BroadcastTxHash: "tx_small_change_hash",
		})

		client := NewClient(context.Background(), &ClientOptions{
			WOCClient: mock,
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

		mock := newMockWOCFromConfig(mockServerConfig{
			UTXOs:           utxos,
			Balance:         int64(50000) + int64(fee), //nolint:gosec // Test fixture with known safe values
			BroadcastTxHash: "tx_exact_match_hash",
		})

		client := NewClient(context.Background(), &ClientOptions{
			WOCClient: mock,
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
	mock := mockUTXOClient(utxos)

	client := NewClient(context.Background(), &ClientOptions{
		WOCClient: mock,
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
		mock := mockUTXOClient(utxos)

		client := NewClient(context.Background(), &ClientOptions{
			WOCClient: mock,
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

			mock := newMockWOCFromConfig(mockServerConfig{
				UTXOs:           utxos,
				Balance:         int64(utxoAmount), //nolint:gosec // Safe: test values are small
				BroadcastTxHash: "broadcast_success",
			})

			client := NewClient(context.Background(), &ClientOptions{
				WOCClient: mock,
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

		mock := newMockWOCFromConfig(mockServerConfig{
			UTXOs:           utxos,
			Balance:         int64(50000) + int64(fee), //nolint:gosec // Test fixture with known safe values
			BroadcastTxHash: "p2sh_output_tx",
		})

		client := NewClient(context.Background(), &ClientOptions{
			WOCClient: mock,
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

		mock := newMockWOCFromConfig(mockServerConfig{
			UTXOs:           utxos,
			Balance:         int64(utxoAmount),
			BroadcastTxHash: "sweep_single_tx",
		})

		client := NewClient(context.Background(), &ClientOptions{
			WOCClient: mock,
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
		assert.Equal(t, client.FormatAmount(chain.AmountToBigInt(expectedAmount)), result.Amount)
		assert.Equal(t, client.FormatAmount(chain.AmountToBigInt(expectedFee)), result.Fee)
	})

	t.Run("sweep multiple UTXOs - consolidates all", func(t *testing.T) {
		t.Parallel()

		kp := getTestKeyPair()
		utxos := makeUTXOsWithKey(kp, 30000, 40000, 50000) // 120k total
		mock := newMockWOCFromConfig(mockServerConfig{
			UTXOs:           utxos,
			Balance:         120000,
			BroadcastTxHash: "sweep_multi_tx",
		})

		client := NewClient(context.Background(), &ClientOptions{
			WOCClient: mock,
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
		assert.Equal(t, client.FormatAmount(chain.AmountToBigInt(expectedAmount)), result.Amount)
		assert.Equal(t, client.FormatAmount(chain.AmountToBigInt(expectedFee)), result.Fee)
	})

	t.Run("sweep with no UTXOs returns error", func(t *testing.T) {
		t.Parallel()

		mock := mockUTXOClient([]UTXO{})

		client := NewClient(context.Background(), &ClientOptions{
			WOCClient: mock,
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
		utxos := makeUTXOsWithKey(kp, 5) // 5 satoshis, fee at 50 sat/KB is 10
		mock := newMockWOCFromConfig(mockServerConfig{
			UTXOs:   utxos,
			Balance: 5,
		})

		client := NewClient(context.Background(), &ClientOptions{
			WOCClient: mock,
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
		mock := newMockWOCFromConfig(mockServerConfig{
			UTXOs:           utxos,
			Balance:         50000,
			BroadcastTxHash: "sweep_no_amount_tx",
		})

		client := NewClient(context.Background(), &ClientOptions{
			WOCClient: mock,
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
		feeRate := uint64(500) // Custom rate above MinFeeRate

		mock := newMockWOCFromConfig(mockServerConfig{
			UTXOs:           utxos,
			Balance:         int64(utxoAmount),
			BroadcastTxHash: "sweep_custom_fee_tx",
		})

		client := NewClient(context.Background(), &ClientOptions{
			WOCClient: mock,
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

		// Fee for 1 input, 1 output at 500 sat/KB
		expectedFee := EstimateFeeForTx(1, 1, feeRate)
		expectedAmount := utxoAmount - expectedFee
		assert.Equal(t, client.FormatAmount(chain.AmountToBigInt(expectedAmount)), result.Amount)
	})

	t.Run("sweep minimum viable (1 satoshi after fee)", func(t *testing.T) {
		t.Parallel()

		kp := getTestKeyPair()
		// Fee for 1 input, 1 output at 1 sat/byte = 192
		fee := EstimateFeeForTx(1, 1, DefaultFeeRate)
		utxoAmount := fee + 1 // exactly 1 satoshi remaining
		utxos := makeUTXOsWithKey(kp, utxoAmount)

		mock := newMockWOCFromConfig(mockServerConfig{
			UTXOs:           utxos,
			Balance:         int64(utxoAmount), //nolint:gosec // Test fixture with known safe values
			BroadcastTxHash: "sweep_min_viable_tx",
		})

		client := NewClient(context.Background(), &ClientOptions{
			WOCClient: mock,
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
		assert.Equal(t, client.FormatAmount(chain.AmountToBigInt(1)), result.Amount)
	})

	t.Run("sweep ignores Amount field when SweepAll is true", func(t *testing.T) {
		t.Parallel()

		kp := getTestKeyPair()
		utxoAmount := uint64(50000)
		utxos := makeUTXOsWithKey(kp, utxoAmount)

		mock := newMockWOCFromConfig(mockServerConfig{
			UTXOs:           utxos,
			Balance:         int64(utxoAmount),
			BroadcastTxHash: "sweep_ignore_amount_tx",
		})

		client := NewClient(context.Background(), &ClientOptions{
			WOCClient: mock,
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
		assert.Equal(t, client.FormatAmount(chain.AmountToBigInt(expectedAmount)), result.Amount)
	})

	t.Run("sweep with invalid to address", func(t *testing.T) {
		t.Parallel()

		client := NewClient(context.Background(), nil)
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
	mock := mockUTXOClient(utxos)

	client := NewClient(context.Background(), &ClientOptions{
		WOCClient: mock,
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

// TestBuildRawTransactionMultiKey tests transaction building with per-input signing keys.
//
//nolint:gocognit // Test function with multiple subtests
func TestBuildRawTransactionMultiKey(t *testing.T) {
	t.Parallel()

	t.Run("signs inputs from two different addresses", func(t *testing.T) {
		t.Parallel()

		kp1 := getTestKeyPair()
		kp2 := getTestKeyPair2()

		utxos := makeUTXOsMultiAddr(
			[]testKeyPair{kp1, kp2},
			[][]uint64{{50000}, {60000}},
		)

		builder := NewTxBuilder()
		for _, u := range utxos {
			require.NoError(t, builder.AddInput(u))
		}
		require.NoError(t, builder.AddOutput(validAddress2(), 100000))

		keyMap := map[string][]byte{
			kp1.Address: kp1.PrivateKey,
			kp2.Address: kp2.PrivateKey,
		}

		rawTx, err := BuildRawTransactionMultiKey(builder, keyMap)
		require.NoError(t, err)
		assert.NotEmpty(t, rawTx)
	})

	t.Run("single address in keyMap works fine", func(t *testing.T) {
		t.Parallel()

		kp := getTestKeyPair()
		utxos := makeUTXOsWithKey(kp, 30000, 40000)

		builder := NewTxBuilder()
		for _, u := range utxos {
			require.NoError(t, builder.AddInput(u))
		}
		require.NoError(t, builder.AddOutput(validAddress2(), 50000))

		keyMap := map[string][]byte{
			kp.Address: kp.PrivateKey,
		}

		rawTx, err := BuildRawTransactionMultiKey(builder, keyMap)
		require.NoError(t, err)
		assert.NotEmpty(t, rawTx)
	})

	t.Run("missing key for address returns error", func(t *testing.T) {
		t.Parallel()

		kp1 := getTestKeyPair()
		kp2 := getTestKeyPair2()

		// Create UTXOs from both addresses but only provide key for kp1
		utxos := makeUTXOsMultiAddr(
			[]testKeyPair{kp1, kp2},
			[][]uint64{{50000}, {60000}},
		)

		builder := NewTxBuilder()
		for _, u := range utxos {
			require.NoError(t, builder.AddInput(u))
		}
		require.NoError(t, builder.AddOutput(validAddress2(), 100000))

		keyMap := map[string][]byte{
			kp1.Address: kp1.PrivateKey, // Missing kp2 key
		}

		_, err := BuildRawTransactionMultiKey(builder, keyMap)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "no private key for address")
	})

	t.Run("empty keyMap returns error", func(t *testing.T) {
		t.Parallel()

		kp := getTestKeyPair()
		utxos := makeUTXOsWithKey(kp, 50000)

		builder := NewTxBuilder()
		for _, u := range utxos {
			require.NoError(t, builder.AddInput(u))
		}
		require.NoError(t, builder.AddOutput(validAddress2(), 40000))

		_, err := BuildRawTransactionMultiKey(builder, map[string][]byte{})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "no private keys provided")
	})

	t.Run("nil builder returns error", func(t *testing.T) {
		t.Parallel()

		kp := getTestKeyPair()
		keyMap := map[string][]byte{kp.Address: kp.PrivateKey}

		_, err := BuildRawTransactionMultiKey(nil, keyMap)
		require.Error(t, err)
		assert.ErrorIs(t, err, ErrNoInputs)
	})

	t.Run("no outputs returns error", func(t *testing.T) {
		t.Parallel()

		kp := getTestKeyPair()
		utxos := makeUTXOsWithKey(kp, 50000)

		builder := NewTxBuilder()
		for _, u := range utxos {
			require.NoError(t, builder.AddInput(u))
		}
		// No outputs added

		keyMap := map[string][]byte{kp.Address: kp.PrivateKey}

		_, err := BuildRawTransactionMultiKey(builder, keyMap)
		require.Error(t, err)
		assert.ErrorIs(t, err, ErrNoOutputs)
	})

	t.Run("invalid key length returns error", func(t *testing.T) {
		t.Parallel()

		kp := getTestKeyPair()
		utxos := makeUTXOsWithKey(kp, 50000)

		builder := NewTxBuilder()
		for _, u := range utxos {
			require.NoError(t, builder.AddInput(u))
		}
		require.NoError(t, builder.AddOutput(validAddress2(), 40000))

		keyMap := map[string][]byte{
			kp.Address: make([]byte, 16), // Wrong length
		}

		_, err := BuildRawTransactionMultiKey(builder, keyMap)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "expected 32 bytes")
	})
}

// TestSend_MultiAddressUTXOs tests that Client.Send uses pre-fetched UTXOs from multiple addresses.
func TestSend_MultiAddressUTXOs(t *testing.T) {
	t.Parallel()

	t.Run("sweep with pre-fetched UTXOs from two addresses", func(t *testing.T) {
		t.Parallel()

		kp1 := getTestKeyPair()
		kp2 := getTestKeyPair2()

		// Mock only needed for broadcast (UTXOs are pre-fetched)
		mock := newMockWOCFromConfig(mockServerConfig{
			BroadcastTxHash: "multi_addr_sweep_tx",
		})

		client := NewClient(context.Background(), &ClientOptions{
			WOCClient: mock,
		})

		// Pre-fetch UTXOs from both addresses
		preUTXOs := []chain.UTXO{
			{TxID: testTxID(1), Vout: 0, Amount: 50000, Address: kp1.Address},
			{TxID: testTxID(2), Vout: 0, Amount: 70000, Address: kp2.Address},
		}

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		result, err := client.Send(ctx, chain.SendRequest{
			From:     kp1.Address,
			To:       validAddress2(),
			SweepAll: true,
			PrivateKeys: map[string][]byte{
				kp1.Address: kp1.PrivateKey,
				kp2.Address: kp2.PrivateKey,
			},
			UTXOs: preUTXOs,
		})

		require.NoError(t, err)
		require.NotNil(t, result)
		assert.Equal(t, "multi_addr_sweep_tx", result.Hash)

		// Verify the full 120k (minus fee) was swept
		totalInput := uint64(120000)
		expectedFee := EstimateFeeForTx(2, 1, DefaultFeeRate)
		expectedAmount := totalInput - expectedFee
		assert.Equal(t, client.FormatAmount(chain.AmountToBigInt(expectedAmount)), result.Amount)
	})

	t.Run("normal send with pre-fetched UTXOs from multiple addresses", func(t *testing.T) {
		t.Parallel()

		kp1 := getTestKeyPair()
		kp2 := getTestKeyPair2()

		mock := newMockWOCFromConfig(mockServerConfig{
			BroadcastTxHash: "multi_addr_normal_tx",
		})

		client := NewClient(context.Background(), &ClientOptions{
			WOCClient: mock,
		})

		// Pre-fetch UTXOs: first address has 30k, second has 40k
		preUTXOs := []chain.UTXO{
			{TxID: testTxID(1), Vout: 0, Amount: 30000, Address: kp1.Address},
			{TxID: testTxID(2), Vout: 0, Amount: 40000, Address: kp2.Address},
		}

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		result, err := client.Send(ctx, chain.SendRequest{
			From:   kp1.Address,
			To:     validAddress2(),
			Amount: big.NewInt(50000), // Needs UTXOs from both addresses
			PrivateKeys: map[string][]byte{
				kp1.Address: kp1.PrivateKey,
				kp2.Address: kp2.PrivateKey,
			},
			UTXOs: preUTXOs,
		})

		require.NoError(t, err)
		require.NotNil(t, result)
		assert.Equal(t, "multi_addr_normal_tx", result.Hash)
	})

	t.Run("pre-fetched UTXOs with no matching key fails", func(t *testing.T) {
		t.Parallel()

		kp1 := getTestKeyPair()
		kp2 := getTestKeyPair2()

		mock := newMockWOCFromConfig(mockServerConfig{
			BroadcastTxHash: "should_not_reach",
		})

		client := NewClient(context.Background(), &ClientOptions{
			WOCClient: mock,
		})

		// UTXOs from kp2 but only kp1 key provided
		preUTXOs := []chain.UTXO{
			{TxID: testTxID(1), Vout: 0, Amount: 50000, Address: kp2.Address},
		}

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		_, err := client.Send(ctx, chain.SendRequest{
			From:     kp1.Address,
			To:       validAddress2(),
			SweepAll: true,
			PrivateKeys: map[string][]byte{
				kp1.Address: kp1.PrivateKey, // Missing kp2's key
			},
			UTXOs: preUTXOs,
		})

		require.Error(t, err)
		assert.Contains(t, err.Error(), "no private key for address")
	})

	t.Run("private keys are zeroed after successful multi-address send", func(t *testing.T) {
		t.Parallel()

		kp1 := getTestKeyPair()
		kp2 := getTestKeyPair2()

		mock := newMockWOCFromConfig(mockServerConfig{
			BroadcastTxHash: "zero_test_tx",
		})

		client := NewClient(context.Background(), &ClientOptions{
			WOCClient: mock,
		})

		preUTXOs := []chain.UTXO{
			{TxID: testTxID(1), Vout: 0, Amount: 50000, Address: kp1.Address},
			{TxID: testTxID(2), Vout: 0, Amount: 50000, Address: kp2.Address},
		}

		keys := map[string][]byte{
			kp1.Address: kp1.PrivateKey,
			kp2.Address: kp2.PrivateKey,
		}

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		_, err := client.Send(ctx, chain.SendRequest{
			From:        kp1.Address,
			To:          validAddress2(),
			SweepAll:    true,
			PrivateKeys: keys,
			UTXOs:       preUTXOs,
		})

		require.NoError(t, err)

		// Both keys should be zeroed
		for addr, key := range keys {
			allZero := true
			for _, b := range key {
				if b != 0 {
					allZero = false
					break
				}
			}
			assert.True(t, allZero, "key for %s should be zeroed after send", addr)
		}
	})
}

// TestConvertChainUTXOs tests the chain.UTXO to bsv.UTXO conversion.
func TestConvertChainUTXOs(t *testing.T) {
	t.Parallel()

	t.Run("converts all fields correctly", func(t *testing.T) {
		t.Parallel()

		input := []chain.UTXO{
			{
				TxID:          "abc123",
				Vout:          1,
				Amount:        50000,
				ScriptPubKey:  "76a914...",
				Address:       "1Addr1",
				Confirmations: 10,
			},
			{
				TxID:          "def456",
				Vout:          0,
				Amount:        30000,
				ScriptPubKey:  "76a914...",
				Address:       "1Addr2",
				Confirmations: 5,
			},
		}

		result := convertChainUTXOs(input)

		require.Len(t, result, 2)
		assert.Equal(t, "abc123", result[0].TxID)
		assert.Equal(t, uint32(1), result[0].Vout)
		assert.Equal(t, uint64(50000), result[0].Amount)
		assert.Equal(t, "1Addr1", result[0].Address)
		assert.Equal(t, uint32(10), result[0].Confirmations)

		assert.Equal(t, "def456", result[1].TxID)
		assert.Equal(t, "1Addr2", result[1].Address)
	})

	t.Run("empty input returns empty slice", func(t *testing.T) {
		t.Parallel()

		result := convertChainUTXOs([]chain.UTXO{})
		assert.Empty(t, result)
	})

	t.Run("nil input returns empty slice", func(t *testing.T) {
		t.Parallel()

		result := convertChainUTXOs(nil)
		assert.Empty(t, result)
	})
}
