package bsv

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mrz1836/sigil/internal/chain"
)

// TestCalculateSweepAmount tests sweep amount calculation.
func TestCalculateSweepAmount(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		totalInputs    uint64
		numInputs      int
		feeRate        uint64
		expectedAmount uint64
		expectError    bool
		errContain     string
	}{
		{
			name:        "single UTXO sweep at 1000 sat/KB",
			totalInputs: 10000,
			numInputs:   1,
			feeRate:     1000,
			// Fee: (192*1000+999)/1000 = 192 satoshis
			expectedAmount: 10000 - 192,
			expectError:    false,
		},
		{
			name:        "multiple UTXO sweep at 1000 sat/KB",
			totalInputs: 10000,
			numInputs:   2,
			feeRate:     1000,
			// Fee: (340*1000+999)/1000 = 340 satoshis
			expectedAmount: 10000 - 340,
			expectError:    false,
		},
		{
			name:        "high fee rate sweep",
			totalInputs: 100000,
			numInputs:   1,
			feeRate:     50000,
			// Fee: (192*50000+999)/1000 = 9600 satoshis
			expectedAmount: 100000 - 9600,
			expectError:    false,
		},
		{
			name:        "tiny UTXO (fee exceeds value)",
			totalInputs: 100,
			numInputs:   1,
			feeRate:     1000,
			// Fee: 192 satoshis > 100 total, so error
			expectedAmount: 0,
			expectError:    true,
			errContain:     "insufficient",
		},
		{
			name:        "BSV allows 1 satoshi outputs",
			totalInputs: 193,
			numInputs:   1,
			feeRate:     1000,
			// Fee: 192 satoshis, remaining 193-192 = 1 = BSV dust limit
			expectedAmount: 1,
			expectError:    false,
		},
		{
			name:        "exactly covers fee leaves nothing",
			totalInputs: 192,
			numInputs:   1,
			feeRate:     1000,
			// Fee: 192 satoshis, remaining 0 < dust limit (1)
			expectedAmount: 0,
			expectError:    true,
			errContain:     "insufficient",
		},
		{
			name:        "two satoshis remaining after fee",
			totalInputs: 194,
			numInputs:   1,
			feeRate:     1000,
			// Fee: 192 satoshis, remaining 194-192 = 2 > dust limit (1)
			expectedAmount: 2,
			expectError:    false,
		},
		{
			name:        "old BTC dust limit amount after fee",
			totalInputs: 738,
			numInputs:   1,
			feeRate:     1000,
			// Fee: 192 satoshis, remaining 738-192 = 546 (old BTC dust, valid on BSV)
			expectedAmount: 546,
			expectError:    false,
		},
		{
			name:        "many small UTXOs",
			totalInputs: 50000,
			numInputs:   10,
			feeRate:     1000,
			// Fee: (1524*1000+999)/1000 = 1524 satoshis
			expectedAmount: 50000 - 1524,
			expectError:    false,
		},
		{
			name:        "large sweep 1 BSV",
			totalInputs: 100000000, // 1 BSV
			numInputs:   1,
			feeRate:     1000,
			// Fee: 192 satoshis
			expectedAmount: 100000000 - 192,
			expectError:    false,
		},
		{
			name:        "large sweep 21 million BSV",
			totalInputs: 2100000000000000, // 21 million BSV
			numInputs:   1,
			feeRate:     1000,
			// Fee: 192 satoshis
			expectedAmount: 2100000000000000 - 192,
			expectError:    false,
		},
		{
			name:        "zero total inputs",
			totalInputs: 0,
			numInputs:   0,
			feeRate:     1000,
			// Fee: (44*1000+999)/1000 = 44 > 0, so error
			expectedAmount: 0,
			expectError:    true,
			errContain:     "insufficient",
		},
		{
			name:        "zero fee rate uses minimum",
			totalInputs: 10000,
			numInputs:   1,
			feeRate:     0, // Should be clamped to MinFeeRate (10)
			// Fee: (192*10+999)/1000 = 2 satoshis
			expectedAmount: 9998,
			expectError:    false,
		},
		{
			name:        "very high fee rate clamped to max",
			totalInputs: 100000,
			numInputs:   1,
			feeRate:     100000, // Should be clamped to MaxFeeRate (50000)
			// Fee: (192*50000+999)/1000 = 9600 satoshis
			expectedAmount: 100000 - 9600,
			expectError:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			amount, err := CalculateSweepAmount(tt.totalInputs, tt.numInputs, tt.feeRate)

			if tt.expectError {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errContain)
				assert.Equal(t, uint64(0), amount)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.expectedAmount, amount)
			}
		})
	}
}

// TestCalculateSweepAmount_FeeCalculation verifies fee calculation accuracy.
func TestCalculateSweepAmount_FeeCalculation(t *testing.T) {
	t.Parallel()

	// Verify fee calculation matches EstimateFeeForTx
	tests := []struct {
		numInputs int
		feeRate   uint64
	}{
		{1, 1000},
		{2, 1000},
		{5, 1000},
		{10, 1000},
		{1, 10000},
		{1, 50000},
		{5, 25000},
	}

	for _, tt := range tests {
		t.Run("", func(t *testing.T) {
			t.Parallel()

			expectedFee := EstimateFeeForTx(tt.numInputs, 1, tt.feeRate)
			totalInputs := expectedFee + 1000 // Ensure enough for fee + reasonable output

			amount, err := CalculateSweepAmount(totalInputs, tt.numInputs, tt.feeRate)
			require.NoError(t, err)

			// amount + fee should equal totalInputs
			assert.Equal(t, totalInputs, amount+expectedFee)
		})
	}
}

// TestCalculateSweepAmount_ConsolidationScenarios tests real-world consolidation scenarios.
func TestCalculateSweepAmount_ConsolidationScenarios(t *testing.T) {
	t.Parallel()

	t.Run("consolidating 100 small UTXOs", func(t *testing.T) {
		t.Parallel()

		// 100 UTXOs of 1000 satoshis each = 100,000 total
		totalInputs := uint64(100000)
		numInputs := 100
		feeRate := uint64(1000)

		// Fee: 10 + (100*148) + 34 = 14844 satoshis
		expectedFee := EstimateFeeForTx(numInputs, 1, feeRate)
		assert.Equal(t, uint64(14844), expectedFee)

		amount, err := CalculateSweepAmount(totalInputs, numInputs, feeRate)
		require.NoError(t, err)
		assert.Equal(t, totalInputs-expectedFee, amount)
	})

	t.Run("consolidating 50 UTXOs at high fee rate", func(t *testing.T) {
		t.Parallel()

		// 50 UTXOs totaling 500,000 satoshis
		totalInputs := uint64(500000)
		numInputs := 50
		feeRate := uint64(50000)

		// Fee: 10 + (50*148) + 34 = 7444 bytes, (7444*50000+999)/1000 = 372,200 satoshis
		expectedFee := EstimateFeeForTx(numInputs, 1, feeRate)

		amount, err := CalculateSweepAmount(totalInputs, numInputs, feeRate)
		require.NoError(t, err)
		assert.Equal(t, totalInputs-expectedFee, amount)
	})

	t.Run("single large UTXO sweep", func(t *testing.T) {
		t.Parallel()

		// 1 large UTXO of 10 BSV
		totalInputs := uint64(1000000000) // 10 BSV
		numInputs := 1
		feeRate := uint64(1000)

		amount, err := CalculateSweepAmount(totalInputs, numInputs, feeRate)
		require.NoError(t, err)

		// Fee should be tiny compared to amount
		fee := totalInputs - amount
		assert.Equal(t, uint64(192), fee)
		assert.Greater(t, amount, uint64(999999000)) // > 9.99999 BSV
	})
}

// TestCalculateSweepAmount_EdgeCases tests edge cases and boundary conditions.
// BSV removed dust limits in 2018, so 1 satoshi is the minimum valid output.
func TestCalculateSweepAmount_EdgeCases(t *testing.T) {
	t.Parallel()

	dustLimit := chain.BSV.DustLimit() // 1 satoshi for BSV

	t.Run("fee exactly equals total inputs", func(t *testing.T) {
		t.Parallel()

		// 1 input fee = 192 satoshis
		totalInputs := uint64(192)
		numInputs := 1
		feeRate := uint64(1000)

		_, err := CalculateSweepAmount(totalInputs, numInputs, feeRate)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "insufficient")
	})

	t.Run("fee exceeds total inputs by one satoshi", func(t *testing.T) {
		t.Parallel()

		totalInputs := uint64(191) // Less than 192 fee
		numInputs := 1
		feeRate := uint64(1000)

		_, err := CalculateSweepAmount(totalInputs, numInputs, feeRate)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "insufficient")
	})

	t.Run("total inputs one satoshi more than fee (BSV allows this)", func(t *testing.T) {
		t.Parallel()

		totalInputs := uint64(193) // 192 fee + 1
		numInputs := 1
		feeRate := uint64(1000)

		// 1 satoshi remaining = BSV dust limit, should succeed
		amount, err := CalculateSweepAmount(totalInputs, numInputs, feeRate)
		require.NoError(t, err)
		assert.Equal(t, dustLimit, amount)
	})

	t.Run("minimum viable sweep (1 satoshi after fee on BSV)", func(t *testing.T) {
		t.Parallel()

		// Need: fee (192) + BSV dust limit (1) = 193 minimum
		totalInputs := uint64(193)
		numInputs := 1
		feeRate := uint64(1000)

		amount, err := CalculateSweepAmount(totalInputs, numInputs, feeRate)
		require.NoError(t, err)
		assert.Equal(t, dustLimit, amount)
	})
}
