package btc

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// errEsploraUnavailable is a static sentinel for the provider-error test path.
var errEsploraUnavailable = errors.New("esplora unavailable")

func TestClient_GetBalance(t *testing.T) {
	t.Parallel()

	const addr = "1A1zP1eP5QGefi2DMPTfTL5SLmv7DivfNa"

	t.Run("returns confirmed amount (funded minus spent)", func(t *testing.T) {
		t.Parallel()
		client := NewClient(context.Background(), &ClientOptions{
			Network: NetworkMainnet,
			Provider: &mockEsplora{
				statsFn: func(_ context.Context, _ string) (*AddressStats, error) {
					return &AddressStats{
						ChainStats:   ChainStats{FundedTxoSum: 150000, SpentTxoSum: 50000},
						MempoolStats: ChainStats{},
					}, nil
				},
			},
		})

		bal, err := client.GetBalance(context.Background(), addr)
		require.NoError(t, err)
		require.NotNil(t, bal)
		assert.Equal(t, int64(100000), bal.Int64())
	})

	t.Run("propagates provider error", func(t *testing.T) {
		t.Parallel()
		client := NewClient(context.Background(), &ClientOptions{
			Network: NetworkMainnet,
			Provider: &mockEsplora{
				statsFn: func(_ context.Context, _ string) (*AddressStats, error) {
					return nil, errEsploraUnavailable
				},
			},
		})

		bal, err := client.GetBalance(context.Background(), addr)
		require.Error(t, err)
		assert.Nil(t, bal)
	})
}
