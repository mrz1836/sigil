package bsv

import (
	"context"
	"encoding/json"
	"math/big"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestNewClient tests client creation.
func TestNewClient(t *testing.T) {
	t.Parallel()
	t.Run("creates client with defaults", func(t *testing.T) {
		t.Parallel()
		client := NewClient(nil)
		assert.NotNil(t, client)
	})

	t.Run("creates client with custom options", func(t *testing.T) {
		t.Parallel()
		client := NewClient(&ClientOptions{
			APIKey:  "test-key",
			Network: NetworkMainnet,
		})
		assert.NotNil(t, client)
	})
}

// TestGetBalance tests BSV balance queries.
func TestGetBalance(t *testing.T) {
	t.Parallel()
	t.Run("returns balance for valid address", func(t *testing.T) {
		t.Parallel()
		// Mock WhatsOnChain API
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Contains(t, r.URL.Path, "/address/")
			assert.Contains(t, r.URL.Path, "/balance")

			// Return balance in satoshis
			resp := BalanceResponse{
				Confirmed:   123456789,
				Unconfirmed: 0,
			}
			err := json.NewEncoder(w).Encode(resp)
			assert.NoError(t, err)
		}))
		defer server.Close()

		client := NewClient(&ClientOptions{
			BaseURL: server.URL,
		})

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		balance, err := client.GetBalance(ctx, "1A1zP1eP5QGefi2DMPTfTL5SLmv7DivfNa")
		require.NoError(t, err)

		expected := big.NewInt(123456789)
		assert.Equal(t, expected, balance)
	})

	t.Run("returns error for invalid address", func(t *testing.T) {
		t.Parallel()
		client := NewClient(nil)

		ctx := context.Background()
		_, err := client.GetBalance(ctx, "invalid")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid BSV address")
	})
}

// TestGetTokenBalance tests token balance (not supported for BSV).
func TestGetTokenBalance(t *testing.T) {
	t.Parallel()
	client := NewClient(nil)

	ctx := context.Background()
	_, err := client.GetTokenBalance(ctx, "1A1zP1eP5QGefi2DMPTfTL5SLmv7DivfNa", "token")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not supported")
}

// TestListUTXOs tests UTXO listing.
func TestListUTXOs(t *testing.T) {
	t.Parallel()
	t.Run("returns UTXOs for address", func(t *testing.T) {
		t.Parallel()
		// Mock WhatsOnChain API
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Contains(t, r.URL.Path, "/address/")
			assert.Contains(t, r.URL.Path, "/unspent")

			resp := []UTXOResponse{
				{
					TxID:   "abc123def456",
					Vout:   0,
					Value:  50000000,
					Height: 100,
				},
				{
					TxID:   "def456abc789",
					Vout:   1,
					Value:  25000000,
					Height: 101,
				},
			}
			err := json.NewEncoder(w).Encode(resp)
			assert.NoError(t, err)
		}))
		defer server.Close()

		client := NewClient(&ClientOptions{
			BaseURL: server.URL,
		})

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		utxos, err := client.ListUTXOs(ctx, "1A1zP1eP5QGefi2DMPTfTL5SLmv7DivfNa")
		require.NoError(t, err)
		assert.Len(t, utxos, 2)

		assert.Equal(t, "abc123def456", utxos[0].TxID)
		assert.Equal(t, uint64(50000000), utxos[0].Amount)
		assert.Equal(t, "def456abc789", utxos[1].TxID)
	})
}

// TestValidateAddress tests address validation.
func TestValidateAddress(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		address string
		wantErr bool
	}{
		{
			name:    "valid P2PKH address starting with 1",
			address: "1A1zP1eP5QGefi2DMPTfTL5SLmv7DivfNa",
			wantErr: false,
		},
		{
			name:    "valid P2SH address starting with 3",
			address: "3J98t1WpEZ73CNmQviecrnyiWrnqRhWNLy",
			wantErr: false,
		},
		{
			name:    "invalid - too short",
			address: "1A1zP1",
			wantErr: true,
		},
		{
			name:    "invalid - contains 0 (not in Base58)",
			address: "1A0zP1eP5QGefi2DMPTfTL5SLmv7DivfNa",
			wantErr: true,
		},
		{
			name:    "invalid - contains O (not in Base58)",
			address: "1AOzP1eP5QGefi2DMPTfTL5SLmv7DivfNa",
			wantErr: true,
		},
		{
			name:    "invalid - contains I (not in Base58)",
			address: "1AIzP1eP5QGefi2DMPTfTL5SLmv7DivfNa",
			wantErr: true,
		},
		{
			name:    "invalid - contains l (not in Base58)",
			address: "1AlzP1eP5QGefi2DMPTfTL5SLmv7DivfNa",
			wantErr: true,
		},
		{
			name:    "empty address",
			address: "",
			wantErr: true,
		},
	}

	client := NewClient(nil)
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := client.ValidateAddress(tt.address)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// TestFormatAmount tests amount formatting.
func TestFormatAmount(t *testing.T) {
	t.Parallel()
	client := NewClient(nil)

	tests := []struct {
		name     string
		amount   *big.Int
		expected string
	}{
		{
			name:     "1 BSV",
			amount:   big.NewInt(100000000),
			expected: "1.00000000",
		},
		{
			name:     "0.5 BSV",
			amount:   big.NewInt(50000000),
			expected: "0.50000000",
		},
		{
			name:     "1 satoshi",
			amount:   big.NewInt(1),
			expected: "0.00000001",
		},
		{
			name:     "zero",
			amount:   big.NewInt(0),
			expected: "0.00000000",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result := client.FormatAmount(tt.amount)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// TestParseAmount tests amount parsing.
func TestParseAmount(t *testing.T) {
	t.Parallel()
	client := NewClient(nil)

	tests := []struct {
		name     string
		input    string
		expected *big.Int
		wantErr  bool
	}{
		{
			name:     "1 BSV",
			input:    "1",
			expected: big.NewInt(100000000),
			wantErr:  false,
		},
		{
			name:     "0.5 BSV",
			input:    "0.5",
			expected: big.NewInt(50000000),
			wantErr:  false,
		},
		{
			name:     "0.00000001 BSV (1 satoshi)",
			input:    "0.00000001",
			expected: big.NewInt(1),
			wantErr:  false,
		},
		{
			name:    "invalid - not a number",
			input:   "abc",
			wantErr: true,
		},
		{
			name:    "invalid - negative",
			input:   "-1",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result, err := client.ParseAmount(tt.input)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.expected, result)
			}
		})
	}
}

// TestSelectUTXOs tests UTXO selection.
func TestSelectUTXOs(t *testing.T) {
	t.Parallel()
	client := NewClient(nil)

	utxos := []UTXO{
		{TxID: "tx1", Vout: 0, Amount: 50000000},  // 0.5 BSV
		{TxID: "tx2", Vout: 0, Amount: 30000000},  // 0.3 BSV
		{TxID: "tx3", Vout: 0, Amount: 100000000}, // 1.0 BSV
	}

	t.Run("selects sufficient UTXOs", func(t *testing.T) {
		t.Parallel()
		selected, change, err := client.SelectUTXOs(utxos, 40000000, 1000) // 0.4 BSV
		require.NoError(t, err)
		assert.NotEmpty(t, selected)

		// Total selected should cover amount + fee
		var total uint64
		for _, u := range selected {
			total += u.Amount
		}
		assert.Greater(t, total, uint64(40000000))
		_ = change // Change is calculated but we don't assert a specific value
	})

	t.Run("returns error for insufficient funds", func(t *testing.T) {
		t.Parallel()
		_, _, err := client.SelectUTXOs(utxos, 200000000, 1000) // 2 BSV - more than available
		require.Error(t, err)
		assert.Contains(t, err.Error(), "insufficient")
	})

	t.Run("handles empty UTXO list", func(t *testing.T) {
		t.Parallel()
		_, _, err := client.SelectUTXOs([]UTXO{}, 10000, 1000)
		require.Error(t, err)
	})
}

// TestSelectUTXOs_Algorithm tests the UTXO selection algorithm in detail.
func TestSelectUTXOs_Algorithm(t *testing.T) {
	t.Parallel()
	client := NewClient(nil)

	tests := []struct {
		name            string
		utxoAmounts     []uint64
		targetAmount    uint64
		feeRate         uint64
		expectSelected  int // Expected number of UTXOs selected
		expectError     bool
		expectMinChange uint64 // Minimum expected change (0 if not checking)
	}{
		{
			name:           "single large UTXO covers target",
			utxoAmounts:    []uint64{100000},
			targetAmount:   50000,
			feeRate:        1000,
			expectSelected: 1,
			expectError:    false,
		},
		{
			name:           "multiple small UTXOs needed",
			utxoAmounts:    []uint64{10000, 20000, 30000},
			targetAmount:   50000,
			feeRate:        1000,
			expectSelected: 3,
			expectError:    false,
		},
		{
			name:           "all UTXOs needed",
			utxoAmounts:    []uint64{10000, 10000, 10000},
			targetAmount:   29000,
			feeRate:        1000,
			expectSelected: 3,
			expectError:    false,
		},
		{
			name:           "insufficient funds",
			utxoAmounts:    []uint64{1000, 2000},
			targetAmount:   10000,
			feeRate:        1000,
			expectSelected: 0,
			expectError:    true,
		},
		{
			name:           "exact amount match with fee",
			utxoAmounts:    []uint64{50000 + EstimateTxSize(1, 2)}, // 50000 + fee
			targetAmount:   50000,
			feeRate:        1000,
			expectSelected: 1,
			expectError:    false,
		},
		{
			name:           "many small UTXOs",
			utxoAmounts:    []uint64{1000, 1000, 1000, 1000, 1000, 1000, 1000, 1000, 1000, 1000},
			targetAmount:   5000,
			feeRate:        1000,
			expectSelected: 6,
			expectError:    false,
		},
		{
			name:           "single UTXO exactly at target plus fee",
			utxoAmounts:    []uint64{10000 + EstimateTxSize(1, 2)}, // 10000 + fee
			targetAmount:   10000,
			feeRate:        1000,
			expectSelected: 1,
			expectError:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			utxos := makeUTXOs(tt.utxoAmounts...)
			selected, change, err := client.SelectUTXOs(utxos, tt.targetAmount, tt.feeRate)

			if tt.expectError {
				require.Error(t, err)
				assert.Contains(t, err.Error(), "insufficient")
				return
			}

			require.NoError(t, err)
			assert.Len(t, selected, tt.expectSelected)

			// Verify total selected covers target + estimated fee
			var total uint64
			for _, u := range selected {
				total += u.Amount
			}
			estimatedFee := estimateFee(len(selected), 2, tt.feeRate)
			assert.GreaterOrEqual(t, total, tt.targetAmount+estimatedFee)

			// Verify change calculation
			expectedChange := total - tt.targetAmount - estimatedFee
			assert.Equal(t, expectedChange, change)
		})
	}
}

// TestSelectUTXOs_ChangeHandling tests change output edge cases.
func TestSelectUTXOs_ChangeHandling(t *testing.T) {
	t.Parallel()
	client := NewClient(nil)

	tests := []struct {
		name           string
		utxoAmount     uint64
		targetAmount   uint64
		feeRate        uint64
		expectedChange uint64
		description    string
	}{
		{
			name:           "change equals dust limit (546)",
			utxoAmount:     50000 + EstimateTxSize(1, 2) + 546, // 50000 + fee + 546 change
			targetAmount:   50000,
			feeRate:        1000,
			expectedChange: 546,
			description:    "change exactly at dust limit should be kept",
		},
		{
			name:           "change below dust limit (545)",
			utxoAmount:     50000 + EstimateTxSize(1, 2) + 545, // 50000 + fee + 545 change
			targetAmount:   50000,
			feeRate:        1000,
			expectedChange: 545,
			description:    "change below dust - in production would be absorbed into fee",
		},
		{
			name:           "change is zero (exact match)",
			utxoAmount:     50000 + EstimateTxSize(1, 2), // 50000 + fee, no change
			targetAmount:   50000,
			feeRate:        1000,
			expectedChange: 0,
			description:    "exact match should have zero change",
		},
		{
			name:           "change is 1 satoshi",
			utxoAmount:     50000 + EstimateTxSize(1, 2) + 1, // 50000 + fee + 1
			targetAmount:   50000,
			feeRate:        1000,
			expectedChange: 1,
			description:    "1 satoshi change - would be absorbed into fee",
		},
		{
			name:           "large change",
			utxoAmount:     100000,
			targetAmount:   50000,
			feeRate:        1000,
			expectedChange: 100000 - 50000 - EstimateTxSize(1, 2), // amount - fee
			description:    "large change should be returned",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			utxos := makeUTXOs(tt.utxoAmount)
			_, change, err := client.SelectUTXOs(utxos, tt.targetAmount, tt.feeRate)

			require.NoError(t, err, tt.description)
			assert.Equal(t, tt.expectedChange, change, tt.description)
		})
	}
}

// TestSelectUTXOs_FeeRateImpact tests how fee rate affects UTXO selection.
func TestSelectUTXOs_FeeRateImpact(t *testing.T) {
	t.Parallel()
	client := NewClient(nil)

	tests := []struct {
		name           string
		utxoAmounts    []uint64
		targetAmount   uint64
		feeRate        uint64
		expectSelected int
		expectError    bool
	}{
		{
			name:           "min fee rate (1 sat/byte) - fewer UTXOs needed",
			utxoAmounts:    []uint64{10000, 10000, 10000},
			targetAmount:   10000,
			feeRate:        1000,
			expectSelected: 2,
			expectError:    false,
		},
		{
			name:           "max fee rate (50 sat/byte) - more UTXOs needed",
			utxoAmounts:    []uint64{15000, 15000, 15000},
			targetAmount:   5000,
			feeRate:        50000,
			expectSelected: 2,
			expectError:    false,
		},
		{
			name:           "high fee rate makes tx unaffordable",
			utxoAmounts:    []uint64{5000, 5000, 5000}, // 15000 total
			targetAmount:   5000,
			feeRate:        50000,
			expectSelected: 0,
			expectError:    true, // 5000 + (522*50000+999)/1000 = 5000 + 26100 > 15000
		},
		{
			name:           "mid-range fee rate needs more UTXOs",
			utxoAmounts:    []uint64{10000, 10000},
			targetAmount:   5000,
			feeRate:        25000,
			expectSelected: 2,
			expectError:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			utxos := makeUTXOs(tt.utxoAmounts...)
			selected, _, err := client.SelectUTXOs(utxos, tt.targetAmount, tt.feeRate)

			if tt.expectError {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)
			assert.Len(t, selected, tt.expectSelected)
		})
	}
}

// TestSelectUTXOs_SortingOrder tests that UTXOs are sorted largest-first.
func TestSelectUTXOs_SortingOrder(t *testing.T) {
	t.Parallel()
	client := NewClient(nil)

	// Provide UTXOs in random order
	utxos := []UTXO{
		makeUTXO(testTxID(1), 10000),  // Smallest
		makeUTXO(testTxID(2), 100000), // Largest
		makeUTXO(testTxID(3), 50000),  // Middle
	}

	selected, _, err := client.SelectUTXOs(utxos, 50000, 1000)
	require.NoError(t, err)

	// Should select largest first
	assert.Len(t, selected, 1)
	assert.Equal(t, uint64(100000), selected[0].Amount)
}

// TestSelectUTXOs_LargeNumber tests selection with many UTXOs.
func TestSelectUTXOs_LargeNumber(t *testing.T) {
	t.Parallel()
	client := NewClient(nil)

	// Create 100 UTXOs of 1000 satoshis each = 100,000 total
	var utxos []UTXO
	for i := 0; i < 100; i++ {
		utxos = append(utxos, makeUTXO(testTxID(i), 1000))
	}

	// Try to select 50,000 + fee
	selected, _, err := client.SelectUTXOs(utxos, 50000, 1000)
	require.NoError(t, err)

	// Should select enough UTXOs to cover amount plus fee growth
	assert.GreaterOrEqual(t, len(selected), 59)
}

// TestSelectUTXOs_SingleUTXOExact tests when single UTXO exactly matches.
func TestSelectUTXOs_SingleUTXOExact(t *testing.T) {
	t.Parallel()
	client := NewClient(nil)

	// UTXO exactly covers amount + default estimated fee
	utxos := makeUTXOs(10000 + EstimateTxSize(1, 2)) // 10000 + fee
	selected, change, err := client.SelectUTXOs(utxos, 10000, 1000)

	require.NoError(t, err)
	assert.Len(t, selected, 1)
	assert.Equal(t, uint64(0), change)
}

// TestSelectUTXOs_DoesNotMutateInput tests that original slice isn't modified.
func TestSelectUTXOs_DoesNotMutateInput(t *testing.T) {
	t.Parallel()
	client := NewClient(nil)

	utxos := []UTXO{
		testMakeUTXO(testTxID(1), 0, 10000),
		testMakeUTXO(testTxID(2), 0, 50000),
		testMakeUTXO(testTxID(3), 0, 30000),
	}

	// Store original order
	originalFirst := utxos[0].Amount
	originalSecond := utxos[1].Amount
	originalThird := utxos[2].Amount

	_, _, err := client.SelectUTXOs(utxos, 20000, 1000)
	require.NoError(t, err)

	// Verify original slice is unchanged
	assert.Equal(t, originalFirst, utxos[0].Amount)
	assert.Equal(t, originalSecond, utxos[1].Amount)
	assert.Equal(t, originalThird, utxos[2].Amount)
}

// testMakeUTXO is a local helper for tests that need to specify vout.
func testMakeUTXO(txid string, vout uint32, amount uint64) UTXO {
	return UTXO{
		TxID:    txid,
		Vout:    vout,
		Amount:  amount,
		Address: testAddress,
	}
}
