package cli

import (
	"context"
	"io"
	"os"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mrz1836/sigil/internal/output"
	"github.com/mrz1836/sigil/internal/wallet"
)

// TestWarnNetworkConflict swaps the global os.Stderr to capture output, so it is
// intentionally not parallel.
func TestWarnNetworkConflict(t *testing.T) {
	capture := func(fn func()) string {
		old := os.Stderr
		r, w, err := os.Pipe()
		require.NoError(t, err)
		os.Stderr = w
		fn()
		_ = w.Close()
		os.Stderr = old
		out, _ := io.ReadAll(r)
		return string(out)
	}

	// requestedNet is the effective global network the command "requested".
	newCmd := func(requestedNet string) *cobra.Command {
		cmd := &cobra.Command{}
		cmd.SetContext(context.Background())
		cmd.Flags().String("network", "", "")
		cmd.Flags().Bool("testnet", false, "")
		SetCmdContext(cmd, &CommandContext{
			Cfg: &mockConfigProvider{bsvNetwork: requestedNet},
			Fmt: &mockFormatProvider{format: output.FormatText},
		})
		return cmd
	}

	t.Run("nil wallet is silent", func(t *testing.T) {
		cmd := newCmd("test")
		require.NoError(t, cmd.Flags().Set("network", "test"))
		assert.Empty(t, capture(func() { warnNetworkConflict(cmd, nil) }))
	})

	t.Run("legacy wallet with empty network is silent", func(t *testing.T) {
		cmd := newCmd("test")
		require.NoError(t, cmd.Flags().Set("network", "test"))
		assert.Empty(t, capture(func() { warnNetworkConflict(cmd, &wallet.Wallet{Name: "legacy"}) }))
	})

	t.Run("no network flag changed is silent", func(t *testing.T) {
		cmd := newCmd("test") // flags registered but never Set => not Changed
		assert.Empty(t, capture(func() { warnNetworkConflict(cmd, &wallet.Wallet{Name: "m", Network: "main"}) }))
	})

	t.Run("matching network is silent", func(t *testing.T) {
		cmd := newCmd("main")
		require.NoError(t, cmd.Flags().Set("network", "main"))
		assert.Empty(t, capture(func() { warnNetworkConflict(cmd, &wallet.Wallet{Name: "m", Network: "main"}) }))
	})

	t.Run("conflict warns and names the wallet network", func(t *testing.T) {
		cmd := newCmd("test") // requested "test" conflicts with a "main" wallet
		require.NoError(t, cmd.Flags().Set("network", "test"))
		got := capture(func() { warnNetworkConflict(cmd, &wallet.Wallet{Name: "mainw", Network: "main"}) })
		assert.Contains(t, got, "ignored")
		assert.Contains(t, got, "mainw")
		assert.Contains(t, got, "main wallet")
	})
}
