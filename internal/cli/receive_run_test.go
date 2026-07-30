package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mrz1836/sigil/internal/output"
)

// TestRunReceive_LocalDisplay drives runReceive end-to-end for the non-check path,
// which loads the wallet and derives/displays an address entirely locally (no
// network). It mutates package-level receive flags, so it is not parallel.
func TestRunReceive_LocalDisplay(t *testing.T) {
	origWallet, origChain, origNew := receiveWallet, receiveChain, receiveNew
	origCheck, origQR, origLabel := receiveCheck, receiveQR, receiveLabel
	origAddr, origAll := receiveAddress, receiveAll
	defer func() {
		receiveWallet, receiveChain, receiveNew = origWallet, origChain, origNew
		receiveCheck, receiveQR, receiveLabel = origCheck, origQR, origLabel
		receiveAddress, receiveAll = origAddr, origAll
	}()

	// setup builds a command backed by a real (mock-password) wallet with BSV+ETH.
	setup := func(t *testing.T, format output.Format) (*cobra.Command, *bytes.Buffer) {
		t.Helper()
		tmpDir, cmdCtx, cleanup := setupAgentTest(t) //nolint:govet // local variable, not shadowing
		t.Cleanup(cleanup)
		createTestWalletForAgent(t, tmpDir)
		withMockPrompts(t, []byte("testpass123"), true)
		cmdCtx.Fmt = &mockFormatProvider{format: format}
		cmd := &cobra.Command{}
		cmd.SetContext(context.Background())
		SetCmdContext(cmd, cmdCtx)
		var buf bytes.Buffer
		cmd.SetOut(&buf)
		cmd.SetErr(&buf)
		return cmd, &buf
	}

	t.Run("shows existing unused bsv address in text", func(t *testing.T) {
		receiveWallet, receiveChain, receiveNew = "test-wallet", "bsv", false
		receiveCheck, receiveQR, receiveLabel = false, false, ""
		receiveAddress, receiveAll = "", false

		cmd, buf := setup(t, output.FormatText)
		require.NoError(t, runReceive(cmd, nil))

		out := buf.String()
		assert.Contains(t, out, "Receiving address:")
		assert.Contains(t, out, "Chain:   bsv")
		// Mainnet BSV address explorer link.
		assert.Contains(t, out, "whatsonchain.com/address/")
	})

	t.Run("derives a new eth address with label in json", func(t *testing.T) {
		receiveWallet, receiveChain, receiveNew = "test-wallet", "eth", true
		receiveCheck, receiveQR, receiveLabel = false, false, "savings"
		receiveAddress, receiveAll = "", false

		cmd, buf := setup(t, output.FormatJSON)
		require.NoError(t, runReceive(cmd, nil))

		var parsed map[string]any
		require.NoError(t, json.Unmarshal(buf.Bytes(), &parsed))
		assert.Equal(t, "eth", parsed["chain"])
		assert.Equal(t, true, parsed["is_new"])
		assert.Equal(t, "savings", parsed["label"])
		assert.NotEmpty(t, parsed["address"])
	})
}
