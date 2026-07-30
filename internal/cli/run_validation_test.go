package cli

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mrz1836/sigil/internal/output"
	"github.com/mrz1836/sigil/internal/wallet"
	sigilerr "github.com/mrz1836/sigil/pkg/errors"
)

// newValidationCmd builds a command with a home dir whose wallets directory
// exists (so storage.Exists resolves to "not found" rather than erroring).
func newValidationCmd(t *testing.T) *cobra.Command {
	t.Helper()
	home := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(home, "wallets"), 0o750))
	cmd := &cobra.Command{}
	cmd.SetContext(context.Background())
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	SetCmdContext(cmd, &CommandContext{
		Cfg: &mockConfigProvider{home: home},
		Fmt: &mockFormatProvider{format: output.FormatText},
	})
	return cmd
}

// TestRunUTXOList_FailFast mutates the package-level utxo flags, so it is not parallel.
func TestRunUTXOList_FailFast(t *testing.T) {
	origChain, origWallet := utxoChain, utxoWallet
	defer func() { utxoChain, utxoWallet = origChain, origWallet }()

	t.Run("non-bsv chain rejected", func(t *testing.T) {
		utxoChain, utxoWallet = "eth", "main"
		err := runUTXOList(newValidationCmd(t), nil)
		require.Error(t, err)
		assert.ErrorIs(t, err, sigilerr.ErrInvalidInput)
	})

	t.Run("wallet not found", func(t *testing.T) {
		utxoChain, utxoWallet = "bsv", "does-not-exist"
		err := runUTXOList(newValidationCmd(t), nil)
		require.Error(t, err)
		assert.ErrorIs(t, err, wallet.ErrWalletNotFound)
	})
}

// TestRunUTXORefresh_WalletNotFound mutates package-level utxo flags, so it is not parallel.
func TestRunUTXORefresh_WalletNotFound(t *testing.T) {
	origWallet, origAddrs := utxoWallet, utxoAddresses
	defer func() { utxoWallet, utxoAddresses = origWallet, origAddrs }()

	utxoWallet, utxoAddresses = "ghost", nil
	err := runUTXORefresh(newValidationCmd(t), nil)
	require.Error(t, err)
	assert.ErrorIs(t, err, wallet.ErrWalletNotFound)
}

// TestRunAddressesList_ValidationErrors covers the flag-validation branches that
// return before any wallet load. It mutates package-level addresses flags, so it
// is not parallel.
func TestRunAddressesList_ValidationErrors(t *testing.T) {
	origType, origUsed, origUnused := addressesType, addressesUsed, addressesUnused
	defer func() { addressesType, addressesUsed, addressesUnused = origType, origUsed, origUnused }()

	t.Run("invalid type filter", func(t *testing.T) {
		addressesType, addressesUsed, addressesUnused = "bogus", false, false
		err := runAddressesList(newValidationCmd(t), nil)
		require.Error(t, err)
		assert.ErrorIs(t, err, sigilerr.ErrInvalidInput)
	})

	t.Run("used and unused are mutually exclusive", func(t *testing.T) {
		addressesType, addressesUsed, addressesUnused = "all", true, true
		err := runAddressesList(newValidationCmd(t), nil)
		require.Error(t, err)
		assert.ErrorIs(t, err, sigilerr.ErrInvalidInput)
	})
}

// TestRunTxSend_ValidationErrors mutates package-level tx flags, so it is not parallel.
func TestRunTxSend_ValidationErrors(t *testing.T) {
	origChain, origToken, origWallet := txChain, txToken, txWallet
	defer func() { txChain, txToken, txWallet = origChain, origToken, origWallet }()

	t.Run("invalid chain", func(t *testing.T) {
		txChain, txToken, txWallet = "dogecoin", "", "main"
		err := runTxSend(newValidationCmd(t), nil)
		require.Error(t, err)
		assert.ErrorIs(t, err, sigilerr.ErrInvalidInput)
	})

	t.Run("token on non-eth chain rejected", func(t *testing.T) {
		txChain, txToken, txWallet = "bsv", "USDC", "main"
		err := runTxSend(newValidationCmd(t), nil)
		require.ErrorIs(t, err, sigilerr.ErrInvalidInput)
		// The suggestion (not the base message) identifies the token-specific branch.
		var sigErr *sigilerr.SigilError
		require.ErrorAs(t, err, &sigErr)
		assert.Contains(t, sigErr.Suggestion, "token")
	})
}
