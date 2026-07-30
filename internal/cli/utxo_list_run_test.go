package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"path/filepath"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mrz1836/sigil/internal/output"
)

// TestRunUTXOList_Display drives the local (no-network) UTXO listing path. It
// mutates package-level utxo flags, so it is not parallel.
func TestRunUTXOList_Display(t *testing.T) {
	origWallet, origChain := utxoWallet, utxoChain
	defer func() { utxoWallet, utxoChain = origWallet, origChain }()

	newCmd := func(home string, format output.Format) (*cobra.Command, *bytes.Buffer) {
		cmd := &cobra.Command{}
		cmd.SetContext(context.Background())
		SetCmdContext(cmd, &CommandContext{
			Cfg: &mockConfigProvider{home: home},
			Fmt: &mockFormatProvider{format: format},
		})
		var buf bytes.Buffer
		cmd.SetOut(&buf)
		return cmd, &buf
	}

	t.Run("lists stored utxos in text", func(t *testing.T) {
		tmpDir, cleanup := setupTestEnv(t)
		defer cleanup()
		seedUTXOStore(t, tmpDir, "listw", 250000)

		utxoWallet, utxoChain = "listw", "bsv"
		cmd, buf := newCmd(tmpDir, output.FormatText)
		require.NoError(t, runUTXOList(cmd, nil))

		out := buf.String()
		assert.Contains(t, out, "abc123")         // txid column
		assert.Contains(t, out, "250000")         // amount
		assert.Contains(t, out, "Total: 1 UTXOs") // summary line
	})

	t.Run("lists stored utxos in json", func(t *testing.T) {
		tmpDir, cleanup := setupTestEnv(t)
		defer cleanup()
		seedUTXOStore(t, tmpDir, "listwj", 250000)

		utxoWallet, utxoChain = "listwj", "bsv"
		cmd, buf := newCmd(tmpDir, output.FormatJSON)
		require.NoError(t, runUTXOList(cmd, nil))

		var parsed []map[string]any
		require.NoError(t, json.Unmarshal(buf.Bytes(), &parsed))
		require.Len(t, parsed, 1)
		assert.Equal(t, "abc123", parsed[0]["txid"])
		assert.InDelta(t, float64(250000), parsed[0]["amount"], 0)
	})

	t.Run("empty store reports none", func(t *testing.T) {
		tmpDir, cleanup := setupTestEnv(t)
		defer cleanup()
		createTestWallet(t, filepath.Join(tmpDir, "wallets"), "emptyw")

		utxoWallet, utxoChain = "emptyw", "bsv"
		cmd, buf := newCmd(tmpDir, output.FormatText)
		require.NoError(t, runUTXOList(cmd, nil))

		assert.Contains(t, buf.String(), "No UTXOs stored")
	})
}
