package btc

import (
	"bytes"
	"context"
	"math/big"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mrz1836/sigil/internal/chain"
	sigilerr "github.com/mrz1836/sigil/pkg/errors"
)

func TestClient_ID(t *testing.T) {
	t.Parallel()
	c := &Client{network: NetworkMainnet}
	assert.Equal(t, chain.BTC, c.ID())
}

func TestClient_GetTokenBalanceNotSupported(t *testing.T) {
	t.Parallel()
	c := &Client{network: NetworkMainnet}
	_, err := c.GetTokenBalance(context.Background(), "a", "b")
	assert.ErrorIs(t, err, sigilerr.ErrNotSupported)
}

func TestNewClient_BroadcasterSetup(t *testing.T) {
	t.Parallel()

	// Mainnet: mempool + Blockstream fallback.
	main := NewClient(context.Background(), &ClientOptions{Network: NetworkMainnet})
	require.Len(t, main.broadcasters, 2)
	assert.Equal(t, "mempool.space", main.broadcasters[0].Name())
	assert.Equal(t, "blockstream.info", main.broadcasters[1].Name())

	// Testnet4: mempool only (Blockstream testnet is a different chain).
	test := NewClient(context.Background(), &ClientOptions{Network: NetworkTestnet})
	require.Len(t, test.broadcasters, 1)
	assert.Equal(t, "mempool.space", test.broadcasters[0].Name())
}

func TestClient_ListUTXOs(t *testing.T) {
	t.Parallel()

	addr := validMainnetAddr(0x01)
	c := &Client{
		network: NetworkMainnet,
		provider: &mockEsplora{
			utxosFn: func(_ context.Context, _ string) ([]EsploraUTXO, error) {
				return []EsploraUTXO{
					{TxID: fixedTxID, Vout: 0, Value: 100000, Status: EsploraUTXOStatus{Confirmed: true}},
					{TxID: fixedTxID2, Vout: 2, Value: 2500, Status: EsploraUTXOStatus{Confirmed: false}},
				}, nil
			},
		},
	}

	utxos, err := c.ListUTXOs(context.Background(), addr)
	require.NoError(t, err)
	require.Len(t, utxos, 2)
	assert.Equal(t, uint64(100000), utxos[0].Amount)
	assert.Equal(t, addr, utxos[0].Address)
	assert.Equal(t, uint32(1), utxos[0].Confirmations)
	assert.Equal(t, uint32(0), utxos[1].Confirmations)
}

func TestClient_SelectUTXOs(t *testing.T) {
	t.Parallel()

	c := &Client{network: NetworkMainnet}
	utxos := []UTXO{
		{TxID: fixedTxID, Vout: 0, Amount: 100000, Address: "a"},
		{TxID: fixedTxID2, Vout: 1, Amount: 50000, Address: "b"},
		{TxID: fixedTxID, Vout: 2, Amount: 10000, Address: "c"},
	}

	selected, change, err := c.SelectUTXOs(utxos, 120000, 1)
	require.NoError(t, err)
	require.Len(t, selected, 2, "largest-first should pick 100000 + 50000")
	// fee for 2-in/2-out @ 1 sat/vB = (10 + 296 + 68) = 374.
	assert.Equal(t, uint64(150000-120000-374), change)
}

func TestClient_SelectUTXOs_Insufficient(t *testing.T) {
	t.Parallel()

	c := &Client{network: NetworkMainnet}
	utxos := []UTXO{{TxID: fixedTxID, Vout: 0, Amount: 1000, Address: "a"}}
	_, _, err := c.SelectUTXOs(utxos, 100000, 1)
	assert.ErrorIs(t, err, ErrInsufficientFunds)
}

func TestClient_EstimateFee(t *testing.T) {
	t.Parallel()

	c := &Client{
		network:     NetworkMainnet,
		feeStrategy: FeeStrategyNormal,
		provider: &mockEsplora{feesFn: func(_ context.Context) (*FeeEstimates, error) {
			return &FeeEstimates{HalfHourFee: 10, MinimumFee: 1}, nil
		}},
	}
	fee, err := c.EstimateFee(context.Background(), "", "", nil)
	require.NoError(t, err)
	// 1-in/2-out @ 10 sat/vB = (10 + 148 + 68) * 10 = 2260.
	assert.Equal(t, int64(2260), fee.Int64())
}

// sendClient builds a Client whose provider serves the given UTXOs for the from
// address and a fixed fee, broadcasting via a recording broadcaster.
func sendClient(t *testing.T, utxos []EsploraUTXO, rec *recordingBroadcaster) *Client {
	t.Helper()
	return &Client{
		network: NetworkMainnet,
		provider: &mockEsplora{
			utxosFn: func(_ context.Context, _ string) ([]EsploraUTXO, error) { return utxos, nil },
			feesFn: func(_ context.Context) (*FeeEstimates, error) {
				return &FeeEstimates{HalfHourFee: 5, MinimumFee: 1}, nil
			},
		},
		broadcasters: []Broadcaster{rec},
		feeStrategy:  FeeStrategyNormal,
	}
}

func TestClient_Send_NormalZeroesKey(t *testing.T) {
	t.Parallel()

	from := p2pkhAddressForKey(t, fixedKeyHex)
	to := validMainnetAddr(0x55)
	rec := &recordingBroadcaster{name: "rec", returnID: testTxID}

	c := sendClient(t, []EsploraUTXO{
		{TxID: fixedTxID, Vout: 0, Value: 200000, Status: EsploraUTXOStatus{Confirmed: true}},
	}, rec)

	key := mustDecodeHex(t, fixedKeyHex)
	result, err := c.Send(context.Background(), chain.SendRequest{
		From:       from,
		To:         to,
		Amount:     big.NewInt(50000),
		PrivateKey: key,
	})
	require.NoError(t, err)
	assert.Equal(t, testTxID, result.Hash)
	assert.Equal(t, to, result.To)
	assert.NotEmpty(t, rec.lastHex, "raw tx must have been broadcast")

	// The signing key must be zeroed after the send.
	assert.True(t, bytes.Equal(key, make([]byte, 32)), "private key must be zeroed after Send")
}

func TestClient_Send_SweepNoChange(t *testing.T) {
	t.Parallel()

	from := p2pkhAddressForKey(t, fixedKeyHex)
	to := validMainnetAddr(0x66)
	rec := &recordingBroadcaster{name: "rec", returnID: testTxID}

	c := sendClient(t, []EsploraUTXO{
		{TxID: fixedTxID, Vout: 0, Value: 100000, Status: EsploraUTXOStatus{Confirmed: true}},
		{TxID: fixedTxID2, Vout: 1, Value: 100000, Status: EsploraUTXOStatus{Confirmed: true}},
	}, rec)

	key := mustDecodeHex(t, fixedKeyHex)
	result, err := c.Send(context.Background(), chain.SendRequest{
		From:       from,
		To:         to,
		PrivateKey: key,
		SweepAll:   true,
	})
	require.NoError(t, err)
	assert.Equal(t, testTxID, result.Hash)
	assert.True(t, bytes.Equal(key, make([]byte, 32)), "private key must be zeroed after sweep")
}

func TestClient_Send_MultiKeyZeroesAllKeys(t *testing.T) {
	t.Parallel()

	addr1 := p2pkhAddressForKey(t, fixedKeyHex)
	addr2 := p2pkhAddressForKey(t, fixedKey2Hex)
	to := validMainnetAddr(0x77)
	rec := &recordingBroadcaster{name: "rec", returnID: testTxID}

	c := &Client{
		network: NetworkMainnet,
		provider: &mockEsplora{feesFn: func(_ context.Context) (*FeeEstimates, error) {
			return &FeeEstimates{HalfHourFee: 2, MinimumFee: 1}, nil
		}},
		broadcasters: []Broadcaster{rec},
		feeStrategy:  FeeStrategyNormal,
	}

	key1 := mustDecodeHex(t, fixedKeyHex)
	key2 := mustDecodeHex(t, fixedKey2Hex)
	result, err := c.Send(context.Background(), chain.SendRequest{
		From:   addr1,
		To:     to,
		Amount: big.NewInt(90000),
		UTXOs: []UTXO{
			{TxID: fixedTxID, Vout: 0, Amount: 60000, Address: addr1},
			{TxID: fixedTxID2, Vout: 1, Amount: 60000, Address: addr2},
		},
		PrivateKeys: map[string][]byte{addr1: key1, addr2: key2},
	})
	require.NoError(t, err)
	assert.Equal(t, testTxID, result.Hash)
	assert.True(t, bytes.Equal(key1, make([]byte, 32)), "key1 must be zeroed")
	assert.True(t, bytes.Equal(key2, make([]byte, 32)), "key2 must be zeroed")
}

func TestClient_Send_RejectsCrossNetworkRecipient(t *testing.T) {
	t.Parallel()

	from := p2pkhAddressForKey(t, fixedKeyHex)
	testnetTo, _ := encodeAddr(AddrP2PKH, NetworkTestnet)
	rec := &recordingBroadcaster{name: "rec", returnID: testTxID}
	c := sendClient(t, nil, rec)

	_, err := c.Send(context.Background(), chain.SendRequest{
		From:       from,
		To:         testnetTo,
		Amount:     big.NewInt(50000),
		PrivateKey: mustDecodeHex(t, fixedKeyHex),
	})
	require.Error(t, err)
	assert.Empty(t, rec.lastHex, "must not broadcast on validation failure")
}

func TestClient_FormatParseAmount(t *testing.T) {
	t.Parallel()

	c := &Client{network: NetworkMainnet}
	assert.Equal(t, "1.50000000", c.FormatAmount(big.NewInt(150000000)))
	assert.Equal(t, "0.00000001", c.FormatAmount(big.NewInt(1)))

	amt, err := c.ParseAmount("0.001")
	require.NoError(t, err)
	assert.Equal(t, int64(100000), amt.Int64())

	_, err = c.ParseAmount("not-a-number")
	assert.ErrorIs(t, err, ErrInvalidAmount)
}
