package bsv

import (
	"context"
	"math/big"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestClient_EstimateFee(t *testing.T) {
	t.Parallel()

	client := NewClient(context.Background(), &ClientOptions{WOCClient: &mockWOCClient{}})

	// Fee = ceil(estimatedTxSize(225) * DefaultFeeRate(250) / 1000) = 57 sats.
	fee, err := client.EstimateFee(context.Background(), "from", "to", big.NewInt(1000))
	require.NoError(t, err)
	require.NotNil(t, fee)
	assert.Equal(t, int64(57), fee.Int64())

	// It is deterministic and independent of the (ignored) address/amount inputs.
	fee2, err := client.EstimateFee(context.Background(), "other", "dest", big.NewInt(999999))
	require.NoError(t, err)
	assert.Equal(t, fee, fee2)
}
