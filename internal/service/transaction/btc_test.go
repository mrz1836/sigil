package transaction

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mrz1836/sigil/internal/chain"
)

func TestRecipientScriptLength(t *testing.T) {
	t.Parallel()

	t.Run("mainnet P2PKH is 25 bytes", func(t *testing.T) {
		t.Parallel()
		// OP_DUP OP_HASH160 <20-byte hash> OP_EQUALVERIFY OP_CHECKSIG = 25 bytes.
		n, err := recipientScriptLength("1A1zP1eP5QGefi2DMPTfTL5SLmv7DivfNa", "main")
		require.NoError(t, err)
		assert.Equal(t, 25, n)
	})

	t.Run("mainnet P2WPKH is 22 bytes", func(t *testing.T) {
		t.Parallel()
		// OP_0 <20-byte program> = 22 bytes.
		n, err := recipientScriptLength("bc1qw508d6qejxtdg4y5r3zarvary0c5xw7kv8f3t4", "main")
		require.NoError(t, err)
		assert.Equal(t, 22, n)
	})

	t.Run("invalid address errors", func(t *testing.T) {
		t.Parallel()
		_, err := recipientScriptLength("not-a-valid-address", "main")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "recipient script")
	})

	t.Run("network mismatch errors", func(t *testing.T) {
		t.Parallel()
		// A mainnet address parsed under the testnet params is rejected.
		_, err := recipientScriptLength("1A1zP1eP5QGefi2DMPTfTL5SLmv7DivfNa", "test")
		require.Error(t, err)
	})
}

func TestFilterSpentBTCUTXOs(t *testing.T) {
	t.Parallel()

	t.Run("nil store keeps all", func(t *testing.T) {
		t.Parallel()
		utxos := []chain.UTXO{
			{TxID: "tx1", Vout: 0, Amount: 100000, Address: "1ABC"},
			{TxID: "tx2", Vout: 1, Amount: 200000, Address: "1DEF"},
		}
		filtered := FilterSpentBTCUTXOs(utxos, nil)
		assert.Equal(t, utxos, filtered)
	})

	t.Run("removes only BTC-spent outputs", func(t *testing.T) {
		t.Parallel()
		utxos := []chain.UTXO{
			{TxID: "tx1", Vout: 0, Amount: 100000, Address: "1ABC"},
			{TxID: "tx2", Vout: 0, Amount: 200000, Address: "1DEF"},
			{TxID: "tx3", Vout: 1, Amount: 300000, Address: "1GHI"},
		}
		store := newMockUTXOProvider()
		store.MarkSpent(chain.BTC, "tx1", 0, "spend-a")
		store.MarkSpent(chain.BTC, "tx3", 1, "spend-b")

		filtered := FilterSpentBTCUTXOs(utxos, store)
		require.Len(t, filtered, 1)
		assert.Equal(t, "tx2", filtered[0].TxID)
	})

	t.Run("spent flag is chain-scoped", func(t *testing.T) {
		t.Parallel()
		utxos := []chain.UTXO{{TxID: "tx1", Vout: 0, Amount: 100000, Address: "1ABC"}}
		store := newMockUTXOProvider()
		// Marked spent on BSV — must not filter the BTC UTXO.
		store.MarkSpent(chain.BSV, "tx1", 0, "spend-bsv")

		filtered := FilterSpentBTCUTXOs(utxos, store)
		assert.Len(t, filtered, 1)
	})
}

func TestMarkSpentBTCUTXOs(t *testing.T) {
	t.Parallel()

	t.Run("nil store is a no-op", func(t *testing.T) {
		t.Parallel()
		logger := newMockLogWriter()
		utxos := []chain.UTXO{{TxID: "tx1", Vout: 0, Amount: 100000, Address: "1ABC"}}
		MarkSpentBTCUTXOs(logger, nil, utxos, "spend-tx")
		assert.Empty(t, logger.errorMessages)
	})

	t.Run("marks each UTXO spent on the BTC chain", func(t *testing.T) {
		t.Parallel()
		logger := newMockLogWriter()
		store := newMockUTXOProvider()
		utxos := []chain.UTXO{
			{TxID: "tx1", Vout: 0, Amount: 100000, Address: "1ABC"},
			{TxID: "tx2", Vout: 1, Amount: 200000, Address: "1DEF"},
		}

		MarkSpentBTCUTXOs(logger, store, utxos, "broadcast-hash")

		assert.True(t, store.IsSpent(chain.BTC, "tx1", 0))
		assert.True(t, store.IsSpent(chain.BTC, "tx2", 1))
		// Marking spent does not filter another chain's identical outpoint.
		assert.False(t, store.IsSpent(chain.BSV, "tx1", 0))
		assert.Empty(t, logger.errorMessages)
	})
}

func TestUniqueUTXOAddrs_Exported(t *testing.T) {
	t.Parallel()

	utxos := []chain.UTXO{
		{TxID: "tx1", Vout: 0, Address: "1ABC"},
		{TxID: "tx2", Vout: 1, Address: "1ABC"}, // duplicate address
		{TxID: "tx3", Vout: 0, Address: "1DEF"},
	}
	addrs := UniqueUTXOAddrs(utxos)
	assert.Len(t, addrs, 2)
	assert.Contains(t, addrs, "1ABC")
	assert.Contains(t, addrs, "1DEF")
}

// TestSend_Dispatch_BTC_InvalidAddress covers sendBTC's recipient validation,
// which fails fast before any network client is used.
func TestSend_Dispatch_BTC_InvalidAddress(t *testing.T) {
	t.Parallel()

	service := NewService(&Config{
		Config:  newMockConfigProvider(),
		Storage: newMockStorageProvider(),
		Logger:  newMockLogWriter(),
	})

	result, err := service.Send(context.Background(), &SendRequest{
		ChainID:   chain.BTC,
		To:        "not-a-btc-address",
		AmountStr: "0.001",
	})

	require.Error(t, err)
	assert.Nil(t, result)
}

// TestSend_Dispatch_BTC_InvalidAmount drives sendBTC through network resolution,
// address validation, client construction, and the local UTXO-store load, then
// fails at amount parsing — all before any network round-trip.
func TestSend_Dispatch_BTC_InvalidAmount(t *testing.T) {
	t.Parallel()

	service := NewService(&Config{
		Config:  newMockConfigProvider(),
		Storage: newMockStorageProvider(),
		Logger:  newMockLogWriter(),
	})

	result, err := service.Send(context.Background(), &SendRequest{
		ChainID: chain.BTC,
		// Valid mainnet P2PKH address so validation passes and parsing is reached.
		To:        "1A1zP1eP5QGefi2DMPTfTL5SLmv7DivfNa",
		AmountStr: "not-a-number",
	})

	require.Error(t, err)
	assert.Nil(t, result)
}
