package cli

import (
	"bytes"
	"context"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mrz1836/sigil/internal/chain"
	"github.com/mrz1836/sigil/internal/chain/eth/etherscan"
	"github.com/mrz1836/sigil/internal/output"
	"github.com/mrz1836/sigil/internal/wallet"
	sigilerr "github.com/mrz1836/sigil/pkg/errors"
)

func TestRunReceiveCheckSingleETH_MissingAPIKey(t *testing.T) {
	t.Parallel()

	// Empty Etherscan API key must fail fast before any network client is built.
	cmdCtx := &CommandContext{ //nolint:govet // local variable, not shadowing
		Cfg: &mockConfigProvider{ethEtherscanAPIKey: ""},
		Fmt: &mockFormatProvider{format: output.FormatText},
	}
	addr := &wallet.Address{Address: "0xabc", Path: "m/44'/60'/0'/0/0", Index: 0}

	var buf bytes.Buffer
	err := runReceiveCheckSingleETH(context.Background(), &buf, cmdCtx, addr)
	require.Error(t, err)
	assert.ErrorIs(t, err, etherscan.ErrAPIKeyRequired)
}

func TestRunReceiveCheckAllETH_NoAddresses(t *testing.T) {
	t.Parallel()

	// A wallet with no ETH addresses short-circuits with a friendly message, no error.
	cmdCtx := &CommandContext{ //nolint:govet // local variable, not shadowing
		Cfg: &mockConfigProvider{ethEtherscanAPIKey: ""},
		Fmt: &mockFormatProvider{format: output.FormatText},
	}
	wlt := &wallet.Wallet{Name: "empty", Addresses: map[chain.ID][]wallet.Address{}}

	var buf bytes.Buffer
	err := runReceiveCheckAllETH(context.Background(), &buf, cmdCtx, wlt)
	require.NoError(t, err)
	assert.Contains(t, buf.String(), "No receiving addresses found for eth")
}

func TestRunReceiveCheckAllETH_MissingAPIKey(t *testing.T) {
	t.Parallel()

	// With ETH addresses present but no API key, it fails fast before any fetch.
	cmdCtx := &CommandContext{ //nolint:govet // local variable, not shadowing
		Cfg: &mockConfigProvider{ethEtherscanAPIKey: ""},
		Fmt: &mockFormatProvider{format: output.FormatText},
	}
	wlt := &wallet.Wallet{
		Name: "eth1",
		Addresses: map[chain.ID][]wallet.Address{
			chain.ETH: {{Address: "0xabc", Path: "m/44'/60'/0'/0/0", Index: 0}},
		},
	}

	var buf bytes.Buffer
	err := runReceiveCheckAllETH(context.Background(), &buf, cmdCtx, wlt)
	require.Error(t, err)
	assert.ErrorIs(t, err, etherscan.ErrAPIKeyRequired)
}

// TestRunReceive_ValidationErrors covers the flag/chain validation branches that
// return before any wallet load or network call. It mutates package-level receive
// flags, so it is intentionally not parallel.
func TestRunReceive_ValidationErrors(t *testing.T) {
	origChain, origCheck, origAddr, origAll := receiveChain, receiveCheck, receiveAddress, receiveAll
	defer func() {
		receiveChain, receiveCheck, receiveAddress, receiveAll = origChain, origCheck, origAddr, origAll
	}()

	newCmd := func() *cobra.Command {
		cmd := &cobra.Command{}
		cmd.SetContext(context.Background())
		SetCmdContext(cmd, &CommandContext{
			Cfg: &mockConfigProvider{home: t.TempDir()},
			Fmt: &mockFormatProvider{format: output.FormatText},
		})
		return cmd
	}

	t.Run("--address without --check", func(t *testing.T) {
		receiveChain, receiveCheck, receiveAddress, receiveAll = "bsv", false, "1abc", false
		err := runReceive(newCmd(), nil)
		require.Error(t, err)
		assert.ErrorIs(t, err, sigilerr.ErrInvalidInput)
	})

	t.Run("--all without --check", func(t *testing.T) {
		receiveChain, receiveCheck, receiveAddress, receiveAll = "bsv", false, "", true
		err := runReceive(newCmd(), nil)
		require.Error(t, err)
		assert.ErrorIs(t, err, sigilerr.ErrInvalidInput)
	})

	t.Run("invalid chain", func(t *testing.T) {
		receiveChain, receiveCheck, receiveAddress, receiveAll = "dogecoin", false, "", false
		err := runReceive(newCmd(), nil)
		require.Error(t, err)
		assert.ErrorIs(t, err, sigilerr.ErrInvalidInput)
	})
}
