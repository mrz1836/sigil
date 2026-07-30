package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mrz1836/sigil/internal/chain"
	"github.com/mrz1836/sigil/internal/output"
	"github.com/mrz1836/sigil/internal/utxostore"
)

// seedUTXOStore creates a wallet plus a UTXO store holding a single BSV UTXO.
func seedUTXOStore(t *testing.T, tmpDir, name string, sats uint64) {
	t.Helper()
	walletsDir := filepath.Join(tmpDir, "wallets")
	createTestWallet(t, walletsDir, name)

	storeDir := filepath.Join(walletsDir, name)
	require.NoError(t, os.MkdirAll(storeDir, 0o750))
	store := utxostore.New(storeDir)
	store.AddUTXO(&utxostore.StoredUTXO{
		ChainID: chain.BSV,
		TxID:    "abc123",
		Vout:    0,
		Amount:  sats,
		Address: "1SeedAddr",
	})
	require.NoError(t, store.Save())
}

func TestRunUTXOBalance_NonEmpty(t *testing.T) {
	// Mutates the utxoWallet global (via newUTXOBalanceTestCmd) and cfg/logger
	// globals (via setupTestEnv), so this test is intentionally not parallel.
	origWallet := utxoWallet
	defer func() { utxoWallet = origWallet }()

	t.Run("text shows satoshi and BSV balance", func(t *testing.T) {
		tmpDir, cleanup := setupTestEnv(t)
		defer cleanup()

		seedUTXOStore(t, tmpDir, "hasfunds", 150000)

		cmd, buf := newUTXOBalanceTestCmd(tmpDir, output.FormatText, "hasfunds")
		require.NoError(t, runUTXOBalance(cmd, nil))

		out := buf.String()
		assert.Contains(t, out, "Offline Balance")
		assert.Contains(t, out, "UTXOs:   1")
		assert.Contains(t, out, "150000 satoshis (0.00150000 BSV)")
	})

	t.Run("json reports balance and utxo count", func(t *testing.T) {
		tmpDir, cleanup := setupTestEnv(t)
		defer cleanup()

		seedUTXOStore(t, tmpDir, "hasfundsjson", 150000)

		cmd, buf := newUTXOBalanceTestCmd(tmpDir, output.FormatJSON, "hasfundsjson")
		require.NoError(t, runUTXOBalance(cmd, nil))

		var parsed map[string]any
		require.NoError(t, json.Unmarshal(buf.Bytes(), &parsed))
		assert.InDelta(t, float64(150000), parsed["balance"], 0)
		assert.InDelta(t, float64(1), parsed["utxos"], 0)
		assert.InDelta(t, 0.0015, parsed["bsv"], 1e-9)
	})
}
