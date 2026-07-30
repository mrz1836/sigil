package transaction

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mrz1836/sigil/internal/chain/btc"
	"github.com/mrz1836/sigil/internal/wallet"
)

// fakeEsplora implements btc.EsploraProvider so AggregateBTCUTXOs can run its
// bounded-concurrency fan-out against a BTC client with no network.
type fakeEsplora struct {
	utxosByAddr map[string][]btc.EsploraUTXO
}

func (f *fakeEsplora) AddressStats(_ context.Context, _ string) (*btc.AddressStats, error) {
	return &btc.AddressStats{}, nil
}

func (f *fakeEsplora) AddressUTXOs(_ context.Context, address string) ([]btc.EsploraUTXO, error) {
	return f.utxosByAddr[address], nil
}

func (f *fakeEsplora) FeeEstimates(_ context.Context) (*btc.FeeEstimates, error) {
	return &btc.FeeEstimates{FastestFee: 10}, nil
}

func TestAggregateBTCUTXOs(t *testing.T) {
	t.Parallel()

	const addrA = "1A1zP1eP5QGefi2DMPTfTL5SLmv7DivfNa"
	const addrB = "1BvBMSEYstWetqTFn5Au4m4GFg7xJaNVN2"

	provider := &fakeEsplora{
		utxosByAddr: map[string][]btc.EsploraUTXO{
			addrA: {
				{TxID: "aa", Vout: 0, Value: 100000, Status: btc.EsploraUTXOStatus{Confirmed: true, BlockHeight: 800000}},
				{TxID: "ab", Vout: 1, Value: 50000, Status: btc.EsploraUTXOStatus{Confirmed: true, BlockHeight: 800001}},
			},
			addrB: {
				{TxID: "ba", Vout: 0, Value: 25000, Status: btc.EsploraUTXOStatus{Confirmed: true, BlockHeight: 800002}},
			},
		},
	}

	client := btc.NewClient(context.Background(), &btc.ClientOptions{
		Network:  btc.NetworkMainnet,
		Provider: provider,
	})

	addresses := []wallet.Address{{Address: addrA}, {Address: addrB}}
	utxos, err := AggregateBTCUTXOs(context.Background(), client, addresses)
	require.NoError(t, err)

	// All UTXOs from every address are merged.
	require.Len(t, utxos, 3)
	var total uint64
	for _, u := range utxos {
		total += u.Amount
	}
	assert.Equal(t, uint64(175000), total)
}
