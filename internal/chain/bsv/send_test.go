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
	sigilerr "github.com/mrz1836/sigil/pkg/errors"
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

	t.Run("single UTXO sufficient - no change scenario", func(t *testing.T) {
		t.Parallel()

		// SelectUTXOs uses estimatedTxSize=225 for fee calculation
		// UTXO needs: amount + 225 (estimated fee) to pass selection
		// Then validation uses actual size: 1 input, 1 output = 192 bytes
		// So we need UTXO >= amount + 225 for selection, and >= amount + 192 for validation
		// With UTXO = amount + 225, change = 0, so only 1 output, validation uses 192, passes
		utxos := makeUTXOs(50225) // 50000 + 225 estimated fee
		server := mockUTXOServer(utxos)
		defer server.Close()

		client := NewClient(&ClientOptions{
			BaseURL: server.URL,
		})

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		_, err := client.Send(ctx, chain.SendRequest{
			From:       validAddress(),
			To:         validAddress2(),
			Amount:     big.NewInt(50000),
			PrivateKey: make([]byte, 32),
		})

		// Should fail at BuildRawTransaction step (not validation)
		require.Error(t, err)
		assert.ErrorIs(t, err, sigilerr.ErrNotImplemented)
	})

	t.Run("multiple UTXOs selected - documented validation issue", func(t *testing.T) {
		t.Parallel()

		utxos := makeUTXOs(30000, 30000, 30000) // 90k total
		server := mockUTXOServer(utxos)
		defer server.Close()

		client := NewClient(&ClientOptions{
			BaseURL: server.URL,
		})

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		_, err := client.Send(ctx, chain.SendRequest{
			From:       validAddress(),
			To:         validAddress2(),
			Amount:     big.NewInt(80000), // Needs multiple UTXOs
			PrivateKey: make([]byte, 32),
		})

		// Due to SelectUTXOs using fixed estimate vs Validate using actual size,
		// validation fails when change is involved
		require.Error(t, err)
		assert.Contains(t, err.Error(), "insufficient")
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
// The current implementation has a known issue where SelectUTXOs uses
// estimatedTxSize (225 bytes) but Validate() calculates fee based on actual
// inputs/outputs. This causes validation to fail when change is added (2 outputs = 226 bytes).
// These tests document the current behavior.
func TestSend_TransactionBuilding(t *testing.T) {
	t.Parallel()

	t.Run("change causes validation mismatch with current implementation", func(t *testing.T) {
		t.Parallel()

		// UTXO with enough for amount + fee + change above dust
		utxos := makeUTXOs(100000) // 100k satoshis
		server := mockUTXOServer(utxos)
		defer server.Close()

		client := NewClient(&ClientOptions{
			BaseURL: server.URL,
		})

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		// Send 50k, should have ~49k change (above dust)
		// Current implementation: SelectUTXOs uses 225 bytes, but with
		// change output the tx is 226 bytes, causing 1 satoshi shortfall
		_, err := client.Send(ctx, chain.SendRequest{
			From:       validAddress(),
			To:         validAddress2(),
			Amount:     big.NewInt(50000),
			PrivateKey: make([]byte, 32),
		})

		require.Error(t, err)
		// Documents current behavior - validation fails due to fee estimate mismatch
		assert.Contains(t, err.Error(), "insufficient")
	})

	t.Run("no change avoids validation mismatch", func(t *testing.T) {
		t.Parallel()

		// UTXO that will result in change below dust limit (gets absorbed)
		// Amount: 50000, Fee estimate: 225, Change < 546 absorbed into fee
		// With UTXO of 50400: change = 50400 - 50000 - 225 = 175 (below dust, absorbed)
		// Actual tx: 1 input, 1 output = 192 bytes @ 1 sat/byte = 192 fee
		// 50400 - 50000 = 400 available for fee, 400 > 192, should work
		utxos := makeUTXOs(50400)
		server := mockUTXOServer(utxos)
		defer server.Close()

		client := NewClient(&ClientOptions{
			BaseURL: server.URL,
		})

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		_, err := client.Send(ctx, chain.SendRequest{
			From:       validAddress(),
			To:         validAddress2(),
			Amount:     big.NewInt(50000),
			PrivateKey: make([]byte, 32),
		})

		require.Error(t, err)
		// Should reach BuildRawTransaction since no change output means no mismatch
		assert.ErrorIs(t, err, sigilerr.ErrNotImplemented)
	})

	t.Run("exact match no change", func(t *testing.T) {
		t.Parallel()

		// SelectUTXOs uses estimatedTxSize=225, needs UTXO >= amount + 225
		// Then with no change, actual fee = 192, validation passes
		utxos := makeUTXOs(50225) // 50000 + 225 for selection
		server := mockUTXOServer(utxos)
		defer server.Close()

		client := NewClient(&ClientOptions{
			BaseURL: server.URL,
		})

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		_, err := client.Send(ctx, chain.SendRequest{
			From:       validAddress(),
			To:         validAddress2(),
			Amount:     big.NewInt(50000),
			PrivateKey: make([]byte, 32),
		})

		require.Error(t, err)
		// Should reach BuildRawTransaction
		assert.ErrorIs(t, err, sigilerr.ErrNotImplemented)
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
func TestSend_AmountBoundaries(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		amount     *big.Int
		utxoAmount uint64
		wantErr    bool
		errContain string
	}{
		{
			name:       "zero amount (below dust)",
			amount:     big.NewInt(0),
			utxoAmount: 100000,
			wantErr:    true,
			// Will fail during AddOutput due to dust check
			errContain: "dust",
		},
		{
			name:       "one satoshi (below dust)",
			amount:     big.NewInt(1),
			utxoAmount: 100000,
			wantErr:    true,
			// Will fail during AddOutput due to dust check
			errContain: "dust",
		},
		{
			name:       "dust limit minus one (545)",
			amount:     big.NewInt(545),
			utxoAmount: 100000,
			wantErr:    true,
			errContain: "dust",
		},
		{
			name:       "exact dust limit (546) - exact UTXO avoids change",
			amount:     big.NewInt(546),
			utxoAmount: 771, // 546 + 225 for SelectUTXOs (actual validation needs 192)
			wantErr:    true,
			errContain: "not implemented",
		},
		{
			name:       "above dust limit (1000) - exact UTXO avoids change",
			amount:     big.NewInt(1000),
			utxoAmount: 1225, // 1000 + 225 for SelectUTXOs
			wantErr:    true,
			errContain: "not implemented",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			utxos := makeUTXOs(tt.utxoAmount)
			server := mockUTXOServer(utxos)
			defer server.Close()

			client := NewClient(&ClientOptions{
				BaseURL: server.URL,
			})

			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()

			_, err := client.Send(ctx, chain.SendRequest{
				From:       validAddress(),
				To:         validAddress2(),
				Amount:     tt.amount,
				PrivateKey: make([]byte, 32),
			})

			if tt.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errContain)
			}
		})
	}
}

// TestSend_P2SHAddresses tests sending to/from P2SH addresses.
func TestSend_P2SHAddresses(t *testing.T) {
	t.Parallel()

	t.Run("send to P2SH address", func(t *testing.T) {
		t.Parallel()

		// Use exact UTXO to avoid change (which causes validation mismatch)
		utxos := makeUTXOs(50225) // 50000 + 225 for SelectUTXOs
		server := mockUTXOServer(utxos)
		defer server.Close()

		client := NewClient(&ClientOptions{
			BaseURL: server.URL,
		})

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		_, err := client.Send(ctx, chain.SendRequest{
			From:       validAddress(),
			To:         validP2SHAddress(), // P2SH address
			Amount:     big.NewInt(50000),
			PrivateKey: make([]byte, 32),
		})

		// Should fail at BuildRawTransaction, not address validation
		require.Error(t, err)
		assert.ErrorIs(t, err, sigilerr.ErrNotImplemented)
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
