package btc

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEstimateFeeForTx_SizeTimesRate(t *testing.T) {
	t.Parallel()

	// 1 input, 2 outputs @ 10 sat/vB: (10 + 148 + 2*34) * 10 = 2260.
	assert.Equal(t, uint64((10+148+2*34)*10), EstimateFeeForTx(1, 2, 10))
	// vsize is multiplied directly (no per-kilobyte division).
	assert.Equal(t, EstimateTxSize(3, 2)*7, EstimateFeeForTx(3, 2, 7))
}

func TestOutputVBytes_ExactSizes(t *testing.T) {
	t.Parallel()

	// scriptLen → serialized output size (8 value + 1 varint + scriptLen).
	assert.Equal(t, uint64(34), outputVBytes(25), "P2PKH output")
	assert.Equal(t, uint64(32), outputVBytes(23), "P2SH output")
	assert.Equal(t, uint64(31), outputVBytes(22), "P2WPKH output")
	assert.Equal(t, uint64(43), outputVBytes(34), "P2WSH output")
}

// TestEstimateFeeExact_P2WSHShortfall proves the exact estimator sizes a P2WSH
// recipient 9 vBytes larger than the flat 34-byte assumption — the bug the exact
// sizing fixes.
func TestEstimateFeeExact_P2WSHShortfall(t *testing.T) {
	t.Parallel()

	const rate = 50
	flat := EstimateFeeForTx(1, 2, rate) // 2 flat P2PKH outputs (recipient + change)

	// Exact: P2WSH recipient (34-byte script → 43 vB) + P2PKH change (25 → 34 vB).
	exact := estimateFeeExact(1, []int{34, 25}, rate)

	// P2WSH output (43) is 9 bytes larger than the flat P2PKH assumption (34).
	assert.Equal(t, flat+9*rate, exact)
}

func TestValidateFeeRate(t *testing.T) {
	t.Parallel()

	assert.Equal(t, uint64(MinFeeRate), ValidateFeeRate(0))
	assert.Equal(t, uint64(MaxFeeRate), ValidateFeeRate(MaxFeeRate+1000))
	assert.Equal(t, uint64(10), ValidateFeeRate(10))
}

func newFeeClient(strategy FeeStrategy, fees *FeeEstimates, feesErr error) *Client {
	return &Client{
		provider: &mockEsplora{
			feesFn: func(_ context.Context) (*FeeEstimates, error) {
				return fees, feesErr
			},
		},
		network:     NetworkMainnet,
		feeStrategy: strategy,
	}
}

func TestFeeRate_TierMapping(t *testing.T) {
	t.Parallel()

	fees := &FeeEstimates{
		FastestFee:  30,
		HalfHourFee: 20,
		HourFee:     15,
		EconomyFee:  5,
		MinimumFee:  1,
	}

	tests := []struct {
		strategy FeeStrategy
		want     uint64
	}{
		{FeeStrategyEconomy, 5},
		{FeeStrategyNormal, 20},
		{FeeStrategyPriority, 30},
	}
	for _, tt := range tests {
		t.Run(string(tt.strategy), func(t *testing.T) {
			t.Parallel()
			c := newFeeClient(tt.strategy, fees, nil)
			assert.Equal(t, tt.want, c.FeeRate(context.Background()))
		})
	}
}

func TestFeeRate_FallbackOnError(t *testing.T) {
	t.Parallel()

	c := newFeeClient(FeeStrategyNormal, nil, errTestProvider)
	assert.Equal(t, uint64(DefaultFeeRate), c.FeeRate(context.Background()))
}

func TestFeeRate_FlooredAtMinimum(t *testing.T) {
	t.Parallel()

	// economyFee below the network minimumFee → floored to minimumFee.
	fees := &FeeEstimates{EconomyFee: 1, HalfHourFee: 2, FastestFee: 3, MinimumFee: 4}
	c := newFeeClient(FeeStrategyEconomy, fees, nil)
	assert.Equal(t, uint64(4), c.FeeRate(context.Background()))
}

func TestFeeRate_ClampedToMax(t *testing.T) {
	t.Parallel()

	fees := &FeeEstimates{FastestFee: MaxFeeRate + 10_000, HalfHourFee: 5, EconomyFee: 5, MinimumFee: 1}
	c := newFeeClient(FeeStrategyPriority, fees, nil)
	assert.Equal(t, uint64(MaxFeeRate), c.FeeRate(context.Background()))
}

func TestFeeRate_FloorAtLeastOne(t *testing.T) {
	t.Parallel()

	// All-zero estimates → floor of 1 sat/vB (min-relay).
	fees := &FeeEstimates{}
	c := newFeeClient(FeeStrategyNormal, fees, nil)
	require.Equal(t, uint64(1), c.FeeRate(context.Background()))
}
