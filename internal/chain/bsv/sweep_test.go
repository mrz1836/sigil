package bsv

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
			name:        "single UTXO sweep at 1 sat/byte",
			totalInputs: 10000,
			numInputs:   1,
			feeRate:     1,
			// Fee: 10 + 148 + 34 = 192 bytes * 1 = 192 satoshis
			expectedAmount: 10000 - 192,
			expectError:    false,
		},
		{
			name:        "multiple UTXO sweep at 1 sat/byte",
			totalInputs: 10000,
			numInputs:   2,
			feeRate:     1,
			// Fee: 10 + (2*148) + 34 = 340 bytes * 1 = 340 satoshis
			expectedAmount: 10000 - 340,
			expectError:    false,
		},
		{
			name:        "high fee rate sweep",
			totalInputs: 100000,
			numInputs:   1,
			feeRate:     50,
			// Fee: 192 bytes * 50 = 9600 satoshis
			expectedAmount: 100000 - 9600,
			expectError:    false,
		},
		{
			name:        "tiny UTXO (fee exceeds value)",
			totalInputs: 200,
			numInputs:   1,
			feeRate:     1,
			// Fee: 192 satoshis > 200 total, so error
			expectedAmount: 0,
			expectError:    true,
			errContain:     "insufficient",
		},
		{
			name:        "dust UTXO only",
			totalInputs: 546,
			numInputs:   1,
			feeRate:     1,
			// Fee: 192 satoshis, remaining 354 < dust limit
			expectedAmount: 0,
			expectError:    true,
			errContain:     "dust",
		},
		{
			name:        "exactly covers fee but below dust",
			totalInputs: 600,
			numInputs:   1,
			feeRate:     1,
			// Fee: 192 satoshis, remaining 408 < dust limit 546
			expectedAmount: 0,
			expectError:    true,
			errContain:     "dust",
		},
		{
			name:        "exactly at dust limit after fee",
			totalInputs: 738,
			numInputs:   1,
			feeRate:     1,
			// Fee: 192 satoshis, remaining 738-192 = 546 = dust limit
			expectedAmount: 546,
			expectError:    false,
		},
		{
			name:        "just above dust limit after fee",
			totalInputs: 739,
			numInputs:   1,
			feeRate:     1,
			// Fee: 192 satoshis, remaining 739-192 = 547 > dust limit
			expectedAmount: 547,
			expectError:    false,
		},
		{
			name:        "many small UTXOs",
			totalInputs: 50000,
			numInputs:   10,
			feeRate:     1,
			// Fee: 10 + (10*148) + 34 = 1524 bytes * 1 = 1524 satoshis
			expectedAmount: 50000 - 1524,
			expectError:    false,
		},
		{
			name:        "large sweep 1 BSV",
			totalInputs: 100000000, // 1 BSV
			numInputs:   1,
			feeRate:     1,
			// Fee: 192 satoshis
			expectedAmount: 100000000 - 192,
			expectError:    false,
		},
		{
			name:        "large sweep 21 million BSV",
			totalInputs: 2100000000000000, // 21 million BSV
			numInputs:   1,
			feeRate:     1,
			// Fee: 192 satoshis
			expectedAmount: 2100000000000000 - 192,
			expectError:    false,
		},
		{
			name:        "zero total inputs",
			totalInputs: 0,
			numInputs:   0,
			feeRate:     1,
			// Fee: 10 + 0 + 34 = 44 > 0, so error
			expectedAmount: 0,
			expectError:    true,
			errContain:     "insufficient",
		},
		{
			name:        "zero fee rate uses minimum",
			totalInputs: 10000,
			numInputs:   1,
			feeRate:     0, // Should be clamped to 1
			// Fee: 192 * 1 = 192 satoshis
			expectedAmount: 10000 - 192,
			expectError:    false,
		},
		{
			name:        "very high fee rate clamped to max",
			totalInputs: 100000,
			numInputs:   1,
			feeRate:     1000, // Should be clamped to 50
			// Fee: 192 * 50 = 9600 satoshis
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
		{1, 1},
		{2, 1},
		{5, 1},
		{10, 1},
		{1, 10},
		{1, 50},
		{5, 25},
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
		feeRate := uint64(1)

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
		feeRate := uint64(50)

		// Fee: 10 + (50*148) + 34 = 7444 bytes * 50 = 372,200 satoshis
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
		feeRate := uint64(1)

		amount, err := CalculateSweepAmount(totalInputs, numInputs, feeRate)
		require.NoError(t, err)

		// Fee should be tiny compared to amount
		fee := totalInputs - amount
		assert.Equal(t, uint64(192), fee)
		assert.Greater(t, amount, uint64(999999000)) // > 9.99999 BSV
	})
}

// TestCalculateSweepAmount_EdgeCases tests edge cases and boundary conditions.
func TestCalculateSweepAmount_EdgeCases(t *testing.T) {
	t.Parallel()

	t.Run("fee exactly equals total inputs", func(t *testing.T) {
		t.Parallel()

		// 1 input fee = 192 satoshis
		totalInputs := uint64(192)
		numInputs := 1
		feeRate := uint64(1)

		_, err := CalculateSweepAmount(totalInputs, numInputs, feeRate)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "insufficient")
	})

	t.Run("fee exceeds total inputs by one satoshi", func(t *testing.T) {
		t.Parallel()

		totalInputs := uint64(191) // Less than 192 fee
		numInputs := 1
		feeRate := uint64(1)

		_, err := CalculateSweepAmount(totalInputs, numInputs, feeRate)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "insufficient")
	})

	t.Run("total inputs one satoshi more than fee", func(t *testing.T) {
		t.Parallel()

		totalInputs := uint64(193) // 192 fee + 1
		numInputs := 1
		feeRate := uint64(1)

		// 1 satoshi remaining < dust limit
		_, err := CalculateSweepAmount(totalInputs, numInputs, feeRate)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "dust")
	})

	t.Run("minimum viable sweep (dust limit after fee)", func(t *testing.T) {
		t.Parallel()

		// Need: fee (192) + dust limit (546) = 738 minimum
		totalInputs := uint64(738)
		numInputs := 1
		feeRate := uint64(1)

		amount, err := CalculateSweepAmount(totalInputs, numInputs, feeRate)
		require.NoError(t, err)
		assert.Equal(t, uint64(546), amount)
	})
}
