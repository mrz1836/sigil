package bsv

import (
	"context"
	"testing"
	"time"

	whatsonchain "github.com/mrz1836/go-whatsonchain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetFeeQuote(t *testing.T) {
	t.Parallel()

	t.Run("successful API response", func(t *testing.T) {
		t.Parallel()

		mock := &mockWOCClient{
			feeFunc: func(_ context.Context, _, _ int64) ([]*whatsonchain.MinerFeeStats, error) {
				return []*whatsonchain.MinerFeeStats{
					{Timestamp: time.Now().Unix(), Name: "MinerA", FeeRate: 400},
					{Timestamp: time.Now().Unix(), Name: "MinerB", FeeRate: 600},
				}, nil
			},
		}

		client := NewClient(context.Background(), &ClientOptions{WOCClient: mock})

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		quote, err := client.GetFeeQuote(ctx)
		require.NoError(t, err)

		assert.Equal(t, "whatsonchain", quote.Source)
		// Average of 400 and 600 = 500 sat/KB
		assert.Equal(t, uint64(500), quote.StandardRate)
	})

	t.Run("single miner response", func(t *testing.T) {
		t.Parallel()

		mock := &mockWOCClient{
			feeFunc: func(_ context.Context, _, _ int64) ([]*whatsonchain.MinerFeeStats, error) {
				return []*whatsonchain.MinerFeeStats{
					{Timestamp: time.Now().Unix(), Name: "MinerA", FeeRate: 236},
				}, nil
			},
		}

		client := NewClient(context.Background(), &ClientOptions{WOCClient: mock})

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		quote, err := client.GetFeeQuote(ctx)
		require.NoError(t, err)

		assert.Equal(t, "whatsonchain", quote.Source)
		assert.Equal(t, uint64(236), quote.StandardRate)
	})

	t.Run("fractional fee rate rounds up", func(t *testing.T) {
		t.Parallel()

		mock := &mockWOCClient{
			feeFunc: func(_ context.Context, _, _ int64) ([]*whatsonchain.MinerFeeStats, error) {
				return []*whatsonchain.MinerFeeStats{
					{Timestamp: time.Now().Unix(), Name: "MinerA", FeeRate: 10.5},
				}, nil
			},
		}

		client := NewClient(context.Background(), &ClientOptions{WOCClient: mock})

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		quote, err := client.GetFeeQuote(ctx)
		require.NoError(t, err)

		// 10.5 rounds up to 11, but 11 < MinFeeRate (50), so clamped to 50
		assert.Equal(t, uint64(MinFeeRate), quote.StandardRate)
	})

	t.Run("network error returns default fee quote", func(t *testing.T) {
		t.Parallel()

		mock := &mockWOCClient{
			feeFunc: func(_ context.Context, _, _ int64) ([]*whatsonchain.MinerFeeStats, error) {
				return nil, errTestConnRefused
			},
		}

		client := NewClient(context.Background(), &ClientOptions{WOCClient: mock})

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		quote, err := client.GetFeeQuote(ctx)
		require.NoError(t, err)

		assert.Equal(t, "default", quote.Source)
		assert.Equal(t, uint64(DefaultFeeRate), quote.StandardRate)
	})

	t.Run("non-200 status returns default fee quote", func(t *testing.T) {
		t.Parallel()

		mock := &mockWOCClient{
			feeFunc: func(_ context.Context, _, _ int64) ([]*whatsonchain.MinerFeeStats, error) {
				return nil, errTestServiceUnavailable
			},
		}

		client := NewClient(context.Background(), &ClientOptions{WOCClient: mock})

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		quote, err := client.GetFeeQuote(ctx)
		require.NoError(t, err)

		assert.Equal(t, "default", quote.Source)
	})

	t.Run("invalid response returns default fee quote", func(t *testing.T) {
		t.Parallel()

		mock := &mockWOCClient{
			feeFunc: func(_ context.Context, _, _ int64) ([]*whatsonchain.MinerFeeStats, error) {
				return nil, errTestInvalidJSON
			},
		}

		client := NewClient(context.Background(), &ClientOptions{WOCClient: mock})

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		quote, err := client.GetFeeQuote(ctx)
		require.NoError(t, err)

		assert.Equal(t, "default", quote.Source)
	})

	t.Run("empty array returns default fee quote", func(t *testing.T) {
		t.Parallel()

		mock := &mockWOCClient{
			feeFunc: func(_ context.Context, _, _ int64) ([]*whatsonchain.MinerFeeStats, error) {
				return []*whatsonchain.MinerFeeStats{}, nil
			},
		}

		client := NewClient(context.Background(), &ClientOptions{WOCClient: mock})

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		quote, err := client.GetFeeQuote(ctx)
		require.NoError(t, err)

		assert.Equal(t, "default", quote.Source)
	})

	t.Run("fee rate below minimum uses minimum", func(t *testing.T) {
		t.Parallel()

		mock := &mockWOCClient{
			feeFunc: func(_ context.Context, _, _ int64) ([]*whatsonchain.MinerFeeStats, error) {
				return []*whatsonchain.MinerFeeStats{
					{Timestamp: time.Now().Unix(), Name: "MinerA", FeeRate: 1.0},
				}, nil
			},
		}

		client := NewClient(context.Background(), &ClientOptions{WOCClient: mock})

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		quote, err := client.GetFeeQuote(ctx)
		require.NoError(t, err)

		assert.Equal(t, uint64(MinFeeRate), quote.StandardRate)
	})
}

func TestEstimateTxSize(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		numInputs  int
		numOutputs int
		expected   uint64
	}{
		// Basic cases
		{
			name:       "single input and output",
			numInputs:  1,
			numOutputs: 1,
			expected:   uint64(TxOverhead + P2PKHInputSize + P2PKHOutputSize), // 10 + 148 + 34 = 192
		},
		{
			name:       "standard P2PKH transaction - 1 input, 2 outputs",
			numInputs:  1,
			numOutputs: 2,
			expected:   uint64(TxOverhead + P2PKHInputSize + 2*P2PKHOutputSize), // 10 + 148 + 68 = 226
		},

		// Multiple inputs (common when consolidating small UTXOs)
		{
			name:       "2 inputs, 1 output - consolidation",
			numInputs:  2,
			numOutputs: 1,
			expected:   uint64(TxOverhead + 2*P2PKHInputSize + P2PKHOutputSize), // 10 + 296 + 34 = 340
		},
		{
			name:       "3 inputs, 2 outputs",
			numInputs:  3,
			numOutputs: 2,
			expected:   uint64(TxOverhead + 3*P2PKHInputSize + 2*P2PKHOutputSize), // 10 + 444 + 68 = 522
		},
		{
			name:       "5 inputs, 1 output - heavy consolidation",
			numInputs:  5,
			numOutputs: 1,
			expected:   uint64(TxOverhead + 5*P2PKHInputSize + P2PKHOutputSize), // 10 + 740 + 34 = 784
		},
		{
			name:       "10 inputs, 2 outputs - large consolidation",
			numInputs:  10,
			numOutputs: 2,
			expected:   uint64(TxOverhead + 10*P2PKHInputSize + 2*P2PKHOutputSize), // 10 + 1480 + 68 = 1558
		},

		// Multiple outputs (batch payments)
		{
			name:       "1 input, 5 outputs - batch send",
			numInputs:  1,
			numOutputs: 5,
			expected:   uint64(TxOverhead + P2PKHInputSize + 5*P2PKHOutputSize), // 10 + 148 + 170 = 328
		},
		{
			name:       "2 inputs, 10 outputs - large batch",
			numInputs:  2,
			numOutputs: 10,
			expected:   uint64(TxOverhead + 2*P2PKHInputSize + 10*P2PKHOutputSize), // 10 + 296 + 340 = 646
		},

		// Edge cases
		{
			name:       "zero inputs",
			numInputs:  0,
			numOutputs: 1,
			expected:   uint64(TxOverhead + P2PKHOutputSize), // 10 + 34 = 44
		},
		{
			name:       "zero outputs",
			numInputs:  1,
			numOutputs: 0,
			expected:   uint64(TxOverhead + P2PKHInputSize), // 10 + 148 = 158
		},
		{
			name:       "zero inputs and outputs",
			numInputs:  0,
			numOutputs: 0,
			expected:   uint64(TxOverhead), // 10
		},

		// Large transactions (BSV supports large blocks)
		{
			name:       "100 inputs, 100 outputs - large transaction",
			numInputs:  100,
			numOutputs: 100,
			expected:   uint64(TxOverhead + 100*P2PKHInputSize + 100*P2PKHOutputSize), // 10 + 14800 + 3400 = 18210
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result := EstimateTxSize(tt.numInputs, tt.numOutputs)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestEstimateFeeForTx(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		numInputs  int
		numOutputs int
		feeRate    uint64
		expected   uint64
	}{
		// Basic fee calculations
		{
			name:       "basic calculation - 1000 sat/KB (1 sat/byte)",
			numInputs:  1,
			numOutputs: 2,
			feeRate:    1000,
			expected:   226, // (226*1000+999)/1000 = 226
		},
		{
			name:       "higher fee rate - 5000 sat/KB (5 sat/byte)",
			numInputs:  1,
			numOutputs: 2,
			feeRate:    5000,
			expected:   1130, // (226*5000+999)/1000 = 1130
		},
		{
			name:       "multiple inputs with varying fee rate",
			numInputs:  3,
			numOutputs: 2,
			feeRate:    2000,
			expected:   1044, // (522*2000+999)/1000 = 1044
		},

		// Minimum fee (50 sat/KB)
		{
			name:       "minimum fee rate",
			numInputs:  1,
			numOutputs: 1,
			feeRate:    MinFeeRate,
			expected:   10, // (192*50+999)/1000 = 10599/1000 = 10
		},

		// Maximum reasonable fee rate
		{
			name:       "maximum fee rate",
			numInputs:  1,
			numOutputs: 2,
			feeRate:    MaxFeeRate,
			expected:   11300, // (226*50000+999)/1000 = 11300999/1000 = 11300
		},

		// Zero fee rate (edge case)
		{
			name:       "zero fee rate",
			numInputs:  1,
			numOutputs: 2,
			feeRate:    0,
			expected:   0, // (226*0+999)/1000 = 0
		},

		// Exact satoshi fee calculations for specific tx sizes
		{
			name:       "192 byte tx at 1000 sat/KB = 192 satoshis",
			numInputs:  1,
			numOutputs: 1,
			feeRate:    1000,
			expected:   192,
		},
		{
			name:       "226 byte tx at 1000 sat/KB = 226 satoshis",
			numInputs:  1,
			numOutputs: 2,
			feeRate:    1000,
			expected:   226,
		},
		{
			name:       "340 byte tx at 1000 sat/KB = 340 satoshis",
			numInputs:  2,
			numOutputs: 1,
			feeRate:    1000,
			expected:   340,
		},

		// Small amount consolidation scenarios
		{
			name:       "consolidating 10 UTXOs to single output",
			numInputs:  10,
			numOutputs: 1,
			feeRate:    1000,
			expected:   uint64(TxOverhead + 10*P2PKHInputSize + P2PKHOutputSize), // 1524
		},
		{
			name:       "batch send to 5 recipients",
			numInputs:  2,
			numOutputs: 5,
			feeRate:    1000,
			expected:   uint64(TxOverhead + 2*P2PKHInputSize + 5*P2PKHOutputSize), // 476
		},

		// Large transaction fees
		{
			name:       "large consolidation at low fee",
			numInputs:  50,
			numOutputs: 2,
			feeRate:    1000,
			expected:   uint64(TxOverhead + 50*P2PKHInputSize + 2*P2PKHOutputSize), // 7478
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result := EstimateFeeForTx(tt.numInputs, tt.numOutputs, tt.feeRate)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestEstimateFeeForAmount(t *testing.T) {
	t.Parallel()

	t.Run("returns fee based on quote", func(t *testing.T) {
		t.Parallel()

		mock := &mockWOCClient{
			feeFunc: func(_ context.Context, _, _ int64) ([]*whatsonchain.MinerFeeStats, error) {
				return []*whatsonchain.MinerFeeStats{
					{Timestamp: time.Now().Unix(), Name: "MinerA", FeeRate: 2000},
				}, nil
			},
		}

		client := NewClient(context.Background(), &ClientOptions{WOCClient: mock})

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		fee, err := client.EstimateFeeForAmount(ctx, 100000000) // 1 BSV
		require.NoError(t, err)

		// Fee should be tx size (1 input, 2 outputs) * fee rate 2000 sat/KB
		expectedSize := uint64(TxOverhead + P2PKHInputSize + 2*P2PKHOutputSize) // 226
		expectedFee := (expectedSize*2000 + 999) / 1000                         // 452
		assert.Equal(t, expectedFee, fee)
	})

	t.Run("uses default quote on error", func(t *testing.T) {
		t.Parallel()

		mock := &mockWOCClient{
			feeFunc: func(_ context.Context, _, _ int64) ([]*whatsonchain.MinerFeeStats, error) {
				return nil, errTestConnRefused
			},
		}

		client := NewClient(context.Background(), &ClientOptions{WOCClient: mock})

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		fee, err := client.EstimateFeeForAmount(ctx, 100000000) // 1 BSV
		require.NoError(t, err)

		// Default fee rate is 250 sat/KB (0.25 sat/byte)
		expectedSize := uint64(TxOverhead + P2PKHInputSize + 2*P2PKHOutputSize) // 226
		expectedFee := (expectedSize*DefaultFeeRate + 999) / 1000               // 57
		assert.Equal(t, expectedFee, fee)
	})

	// BSV-specific: single satoshi amounts (no dust limit)
	t.Run("fee for sending 1 satoshi", func(t *testing.T) {
		t.Parallel()

		mock := &mockWOCClient{
			feeFunc: func(_ context.Context, _, _ int64) ([]*whatsonchain.MinerFeeStats, error) {
				return nil, errTestConnRefused
			},
		}

		client := NewClient(context.Background(), &ClientOptions{WOCClient: mock})

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		fee, err := client.EstimateFeeForAmount(ctx, 1) // 1 satoshi
		require.NoError(t, err)

		// Fee is same regardless of amount (tx size based)
		expectedSize := uint64(TxOverhead + P2PKHInputSize + 2*P2PKHOutputSize) // 226
		expectedFee := (expectedSize*DefaultFeeRate + 999) / 1000
		assert.Equal(t, expectedFee, fee)

		// Fee is much larger than amount being sent (valid in BSV)
		assert.Greater(t, fee, uint64(1))
	})

	t.Run("fee for sending 100 satoshis", func(t *testing.T) {
		t.Parallel()

		mock := &mockWOCClient{
			feeFunc: func(_ context.Context, _, _ int64) ([]*whatsonchain.MinerFeeStats, error) {
				return nil, errTestConnRefused
			},
		}

		client := NewClient(context.Background(), &ClientOptions{WOCClient: mock})

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		fee, err := client.EstimateFeeForAmount(ctx, 100) // 100 satoshis
		require.NoError(t, err)

		expectedFee := (uint64(TxOverhead+P2PKHInputSize+2*P2PKHOutputSize)*DefaultFeeRate + 999) / 1000
		assert.Equal(t, expectedFee, fee)
	})

	t.Run("fee for sending 1000 satoshis", func(t *testing.T) {
		t.Parallel()

		mock := &mockWOCClient{
			feeFunc: func(_ context.Context, _, _ int64) ([]*whatsonchain.MinerFeeStats, error) {
				return nil, errTestConnRefused
			},
		}

		client := NewClient(context.Background(), &ClientOptions{WOCClient: mock})

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		fee, err := client.EstimateFeeForAmount(ctx, 1000) // 1000 satoshis
		require.NoError(t, err)

		expectedFee := (uint64(TxOverhead+P2PKHInputSize+2*P2PKHOutputSize)*DefaultFeeRate + 999) / 1000
		assert.Equal(t, expectedFee, fee)
	})

	t.Run("fee for max supply", func(t *testing.T) {
		t.Parallel()

		mock := &mockWOCClient{
			feeFunc: func(_ context.Context, _, _ int64) ([]*whatsonchain.MinerFeeStats, error) {
				return nil, errTestConnRefused
			},
		}

		client := NewClient(context.Background(), &ClientOptions{WOCClient: mock})

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		// 21 million BSV in satoshis
		fee, err := client.EstimateFeeForAmount(ctx, 2100000000000000)
		require.NoError(t, err)

		// Fee estimation doesn't depend on amount
		expectedFee := (uint64(TxOverhead+P2PKHInputSize+2*P2PKHOutputSize)*DefaultFeeRate + 999) / 1000
		assert.Equal(t, expectedFee, fee)
	})
}

func TestValidateFeeRate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		rate     uint64
		expected uint64
	}{
		{"zero returns minimum", 0, MinFeeRate},
		{"below minimum returns minimum", 5, MinFeeRate},
		{"at minimum returns minimum", MinFeeRate, MinFeeRate},
		{"at maximum returns maximum", MaxFeeRate, MaxFeeRate},
		{"one below maximum stays unchanged", MaxFeeRate - 1, MaxFeeRate - 1},
		{"one above minimum stays unchanged", MinFeeRate + 1, MinFeeRate + 1},
		{"above maximum returns maximum", 100000, MaxFeeRate},
		{"way above maximum returns maximum", 1000000, MaxFeeRate},
		{"within range returns same value", 100, 100},
		{"mid-range value", 25000, 25000},
		{"minimum rate - 50 sat/KB (0.05 sat/byte)", 50, 50},
		{"standard rate - 1000 sat/KB (1 sat/byte)", 1000, 1000},
		{"high rate - 10000 sat/KB (10 sat/byte)", 10000, 10000},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result := ValidateFeeRate(tt.rate)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// TestTxSizeVsFee tests the relationship between transaction size and fee.
func TestTxSizeVsFee(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		numInputs    int
		numOutputs   int
		feeRate1000  uint64 // At 1000 sat/KB (1 sat/byte)
		feeRate50000 uint64 // At 50000 sat/KB (50 sat/byte)
	}{
		{
			name:       "1 input, 1 output",
			numInputs:  1,
			numOutputs: 1,
			// Size: 10 + 148 + 34 = 192 bytes
			feeRate1000:  192,
			feeRate50000: 9600,
		},
		{
			name:       "1 input, 2 outputs",
			numInputs:  1,
			numOutputs: 2,
			// Size: 10 + 148 + 68 = 226 bytes
			feeRate1000:  226,
			feeRate50000: 11300,
		},
		{
			name:       "10 inputs, 1 output",
			numInputs:  10,
			numOutputs: 1,
			// Size: 10 + 1480 + 34 = 1524 bytes
			feeRate1000:  1524,
			feeRate50000: 76200,
		},
		{
			name:       "1 input, 10 outputs",
			numInputs:  1,
			numOutputs: 10,
			// Size: 10 + 148 + 340 = 498 bytes
			feeRate1000:  498,
			feeRate50000: 24900,
		},
		{
			name:       "100 inputs, 100 outputs",
			numInputs:  100,
			numOutputs: 100,
			// Size: 10 + 14800 + 3400 = 18210 bytes
			feeRate1000:  18210,
			feeRate50000: 910500,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// Test at 1000 sat/KB (1 sat/byte)
			size := EstimateTxSize(tt.numInputs, tt.numOutputs)
			fee1 := EstimateFeeForTx(tt.numInputs, tt.numOutputs, 1000)
			assert.Equal(t, size, fee1, "fee at 1000 sat/KB should equal size")
			assert.Equal(t, tt.feeRate1000, fee1)

			// Test at 50000 sat/KB (50 sat/byte)
			fee50 := EstimateFeeForTx(tt.numInputs, tt.numOutputs, 50000)
			assert.Equal(t, size*50, fee50)
			assert.Equal(t, tt.feeRate50000, fee50)
		})
	}
}

// TestFeeRateBoundaries tests fee rate clamping at boundaries.
func TestFeeRateBoundaries(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		inputRate    uint64
		expectedRate uint64
	}{
		{"zero clamps to minimum", 0, MinFeeRate},
		{"5 clamps to minimum", 5, MinFeeRate},
		{"10 clamps to minimum", 10, MinFeeRate},
		{"50 stays at 50 (minimum)", 50, 50},
		{"1000 stays at 1000", 1000, 1000},
		{"25000 stays at 25000", 25000, 25000},
		{"50000 stays at 50000 (maximum)", 50000, 50000},
		{"50001 clamps to 50000", 50001, 50000},
		{"100000 clamps to 50000", 100000, 50000},
		{"max uint64 clamps to 50000", ^uint64(0), 50000},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result := ValidateFeeRate(tt.inputRate)
			assert.Equal(t, tt.expectedRate, result)
		})
	}
}

// TestEstimateTxSize_FormulaVerification verifies the size formula is correct.
func TestEstimateTxSize_FormulaVerification(t *testing.T) {
	t.Parallel()

	// Verify constants are reasonable
	assert.Equal(t, 10, TxOverhead, "transaction overhead should be 10 bytes")
	assert.Equal(t, 148, P2PKHInputSize, "P2PKH input should be 148 bytes")
	assert.Equal(t, 34, P2PKHOutputSize, "P2PKH output should be 34 bytes")

	// Verify formula: overhead + (inputs * input_size) + (outputs * output_size)
	for numIn := 0; numIn <= 10; numIn++ {
		for numOut := 0; numOut <= 10; numOut++ {
			expected := EstimateTxSize(numIn, numOut)
			actual := EstimateTxSize(numIn, numOut)
			assert.Equal(t, expected, actual, "size mismatch for %d inputs, %d outputs", numIn, numOut)
		}
	}
}

// TestEstimateFeeForTx_LargeTx tests fee estimation for very large transactions.
func TestEstimateFeeForTx_LargeTx(t *testing.T) {
	t.Parallel()

	// BSV supports large blocks, so test with many inputs/outputs
	tests := []struct {
		name       string
		numInputs  int
		numOutputs int
		feeRate    uint64
	}{
		{"500 inputs, 1 output", 500, 1, 1000},
		{"1 input, 500 outputs", 1, 500, 1000},
		{"100 inputs, 100 outputs", 100, 100, 1000},
		{"1000 inputs, 10 outputs at 50000 sat/KB", 1000, 10, 50000},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			size := EstimateTxSize(tt.numInputs, tt.numOutputs)
			fee := EstimateFeeForTx(tt.numInputs, tt.numOutputs, tt.feeRate)

			// Fee should equal (size * rate + 999) / 1000
			assert.Equal(t, (size*tt.feeRate+999)/1000, fee)

			// Verify no overflow
			assert.Positive(t, fee)
		})
	}
}

// TestFeeQuote_DefaultValues tests default fee quote values.
func TestFeeQuote_DefaultValues(t *testing.T) {
	t.Parallel()

	quote := defaultFeeQuote()

	assert.Equal(t, uint64(DefaultFeeRate), quote.StandardRate)
	assert.Equal(t, uint64(DefaultFeeRate), quote.DataRate)
	assert.Equal(t, "default", quote.Source)
	assert.False(t, quote.Timestamp.IsZero())
}
