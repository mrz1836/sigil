//go:build integration
// +build integration

package btc

import (
	"context"
	"math/big"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mrz1836/sigil/internal/chain"
	"github.com/mrz1836/sigil/internal/wallet"
	"github.com/mrz1836/sigil/internal/wallet/bitcoin"
)

// TestIntegration_Testnet4_BalanceAndUTXOs exercises the live mempool.space
// testnet4 endpoints through a testnet-configured client and confirms
// cross-network address rejection.
//
//	SIGIL_RUN_INTEGRATION_TESTS=1 SIGIL_BTC_TESTNET_ADDRESS=tb1q... \
//	  go test -tags=integration ./internal/chain/btc/ -run Testnet4 -v
//
// If SIGIL_BTC_TESTNET_ADDRESS is unset the balance/UTXO assertions are skipped
// (faucet coins move), but routing and validation are still checked.
func TestIntegration_Testnet4_BalanceAndUTXOs(t *testing.T) {
	if os.Getenv("SIGIL_RUN_INTEGRATION_TESTS") == "" {
		t.Skip("Skipping integration test. Set SIGIL_RUN_INTEGRATION_TESTS=1 to run.")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()

	client := NewClient(ctx, &ClientOptions{Network: NetworkTestnet})

	// Cross-network safety: a mainnet address must be rejected on a testnet client.
	t.Run("rejects mainnet address", func(t *testing.T) {
		assert.ErrorIs(t, client.ValidateAddress("1BvBMSEYstWetqTFn5Au4m4GFg7xJaNVN2"), ErrInvalidAddress)
		assert.ErrorIs(t, client.ValidateAddress("bc1qw508d6qejxtdg4y5r3zarvary0c5xw7kv8f3t4"), ErrInvalidAddress)
	})

	addr := os.Getenv("SIGIL_BTC_TESTNET_ADDRESS")
	if addr == "" {
		t.Skip("Set SIGIL_BTC_TESTNET_ADDRESS to a funded testnet4 address to check balance/UTXOs.")
	}

	require.NoError(t, client.ValidateAddress(addr), "testnet address should validate on a testnet client")

	t.Run("balance routes to testnet4", func(t *testing.T) {
		bal, err := client.GetNativeBalance(ctx, addr)
		require.NoError(t, err)
		t.Logf("testnet4 balance for %s: %s satoshis (unconfirmed=%v)", addr, bal.Amount.String(), bal.Unconfirmed)
	})

	t.Run("fee estimates available", func(t *testing.T) {
		rate := client.FeeRate(ctx)
		assert.Positive(t, rate, "testnet4 fee rate should be at least 1 sat/vB")
		t.Logf("testnet4 fee rate: %d sat/vB", rate)
	})

	t.Run("list utxos", func(t *testing.T) {
		utxos, err := client.ListUTXOs(ctx, addr)
		require.NoError(t, err)
		t.Logf("testnet4 UTXOs for %s: %d", addr, len(utxos))
	})
}

// TestIntegration_Testnet4_BuildSignBroadcast broadcasts a real testnet4
// transaction — the ultimate proof that the legacy sighash matches Bitcoin Core
// consensus. It is opt-in and never runs in CI because it spends faucet coins.
//
//	SIGIL_RUN_INTEGRATION_TESTS=1 SIGIL_BTC_TESTNET_WIF=c... SIGIL_BTC_TESTNET_TO=tb1q... \
//	  go test -tags=integration ./internal/chain/btc/ -run Testnet4_BuildSignBroadcast -v
func TestIntegration_Testnet4_BuildSignBroadcast(t *testing.T) {
	if os.Getenv("SIGIL_RUN_INTEGRATION_TESTS") == "" {
		t.Skip("Skipping integration test. Set SIGIL_RUN_INTEGRATION_TESTS=1 to run.")
	}
	wif := os.Getenv("SIGIL_BTC_TESTNET_WIF")
	to := os.Getenv("SIGIL_BTC_TESTNET_TO")
	if wif == "" || to == "" {
		t.Skip("Set SIGIL_BTC_TESTNET_WIF and SIGIL_BTC_TESTNET_TO to run the funded testnet4 send.")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	// Decode the WIF to raw private-key bytes (version + 32-byte key [+ 0x01]).
	payload, err := bitcoin.Base58CheckDecode(wif)
	require.NoError(t, err, "WIF must Base58Check-decode")
	require.GreaterOrEqual(t, len(payload), 33, "WIF payload must contain a 32-byte key")
	key := make([]byte, 32)
	copy(key, payload[1:33])

	from, err := wallet.AddressFromPrivKeyBytesForNetwork(key, wallet.Testnet)
	require.NoError(t, err)
	t.Logf("testnet4 from-address: %s", from)

	client := NewClient(ctx, &ClientOptions{Network: NetworkTestnet})
	require.NoError(t, client.ValidateAddress(from), "from address must validate on testnet client")
	require.NoError(t, client.ValidateAddress(to), "recipient must be a valid testnet address")

	// Send a tiny amount (1000 sats) — spends the legacy P2PKH UTXOs at `from`.
	result, err := client.Send(ctx, chain.SendRequest{
		From:       from,
		To:         to,
		Amount:     big.NewInt(1000),
		PrivateKey: key,
	})
	require.NoError(t, err, "build/sign/broadcast must succeed on testnet4")
	require.NotEmpty(t, result.Hash)
	t.Logf("testnet4 broadcast OK: https://mempool.space/testnet4/tx/%s", result.Hash)
}
