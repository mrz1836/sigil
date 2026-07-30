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
	"github.com/mrz1836/sigil/internal/utxostore"
	"github.com/mrz1836/sigil/internal/wallet"
	sigilerr "github.com/mrz1836/sigil/pkg/errors"
)

// TestRunReceiveCheckAll_NoAddresses hits the early-return branch when a wallet has
// no addresses for the requested chain. The store and discovery service are unused
// on this path, so nil is safe and no network is touched.
func TestRunReceiveCheckAll_NoAddresses(t *testing.T) {
	t.Parallel()

	cmdCtx := &CommandContext{ //nolint:govet // local variable, not shadowing
		Cfg: &mockConfigProvider{},
		Fmt: &mockFormatProvider{format: output.FormatText},
	}
	wlt := &wallet.Wallet{Name: "empty", Addresses: map[chain.ID][]wallet.Address{}}

	var buf bytes.Buffer
	runReceiveCheckAll(context.Background(), &buf, cmdCtx, wlt, nil, nil, chain.BSV)

	assert.Contains(t, buf.String(), "No receiving addresses found for")
	assert.Contains(t, buf.String(), "bsv")
}

// TestRunReceiveCheckAllChains_EmptyWallet drives runReceiveCheckAllChains with a
// wallet that has no addresses on any chain: it builds the balance/discovery
// services (constructors only) but takes none of the per-chain branches, so it
// returns nil without any network call.
func TestRunReceiveCheckAllChains_EmptyWallet(t *testing.T) {
	t.Parallel()

	cmdCtx := &CommandContext{ //nolint:govet // local variable, not shadowing
		Cfg: &mockConfigProvider{home: t.TempDir()},
		Fmt: &mockFormatProvider{format: output.FormatText},
	}
	cmd := &cobra.Command{}
	cmd.SetContext(context.Background())
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)

	store := utxostore.New(t.TempDir())
	wlt := &wallet.Wallet{Name: "empty", Addresses: map[chain.ID][]wallet.Address{}}

	require.NoError(t, runReceiveCheckAllChains(cmd, cmdCtx, wlt, store))
}

// TestRunReceiveCheckETH_DispatchMissingAPIKey drives the single-address ETH path
// through runReceiveCheckETH; with no Etherscan key it fails fast before any fetch.
// Mutates package-level receive flags, so it is not parallel.
func TestRunReceiveCheckETH_DispatchMissingAPIKey(t *testing.T) {
	origAll, origAddr := receiveAll, receiveAddress
	defer func() { receiveAll, receiveAddress = origAll, origAddr }()
	receiveAll, receiveAddress = false, ""

	cmdCtx := &CommandContext{ //nolint:govet // local variable, not shadowing
		Cfg: &mockConfigProvider{ethEtherscanAPIKey: ""},
		Fmt: &mockFormatProvider{format: output.FormatText},
	}
	addr := &wallet.Address{Address: "0xabc", Path: "m/44'/60'/0'/0/0", Index: 0}

	var buf bytes.Buffer
	err := runReceiveCheckETH(context.Background(), &buf, cmdCtx, &wallet.Wallet{Name: "w"}, addr)
	require.Error(t, err)
	assert.ErrorIs(t, err, etherscan.ErrAPIKeyRequired)
}

// TestRunReceiveCheckETH_AddressNotFound covers the --address branch of
// runReceiveCheckETH where the requested address is absent from the wallet, so
// findWalletAddress returns before any client is built. Not parallel.
func TestRunReceiveCheckETH_AddressNotFound(t *testing.T) {
	origAll, origAddr := receiveAll, receiveAddress
	defer func() { receiveAll, receiveAddress = origAll, origAddr }()
	receiveAll, receiveAddress = false, "0xnotinwallet"

	cmdCtx := &CommandContext{ //nolint:govet // local variable, not shadowing
		Cfg: &mockConfigProvider{},
		Fmt: &mockFormatProvider{format: output.FormatText},
	}
	wlt := &wallet.Wallet{
		Name:      "w",
		Addresses: map[chain.ID][]wallet.Address{chain.ETH: {{Address: "0xreal"}}},
	}

	var buf bytes.Buffer
	err := runReceiveCheckETH(context.Background(), &buf, cmdCtx, wlt, nil)
	require.Error(t, err)
	assert.ErrorIs(t, err, sigilerr.ErrInvalidInput)
}

// TestRunReceiveCheck_ETHMissingAPIKey drives the top-level runReceiveCheck
// dispatcher for the ETH chain; it routes to the ETH path which fails fast on the
// missing API key. Not parallel (reads package-level receive flags).
func TestRunReceiveCheck_ETHMissingAPIKey(t *testing.T) {
	origAll, origAddr := receiveAll, receiveAddress
	defer func() { receiveAll, receiveAddress = origAll, origAddr }()
	receiveAll, receiveAddress = false, ""

	cmdCtx := &CommandContext{ //nolint:govet // local variable, not shadowing
		Cfg: &mockConfigProvider{ethEtherscanAPIKey: ""},
		Fmt: &mockFormatProvider{format: output.FormatText},
	}
	cmd := &cobra.Command{}
	cmd.SetContext(context.Background())
	var buf bytes.Buffer
	cmd.SetOut(&buf)

	addr := &wallet.Address{Address: "0xabc", Path: "m/44'/60'/0'/0/0", Index: 0}
	err := runReceiveCheck(cmd, cmdCtx, &wallet.Wallet{Name: "w"}, nil, addr, chain.ETH)
	require.Error(t, err)
	assert.ErrorIs(t, err, etherscan.ErrAPIKeyRequired)
}

// TestRunReceiveCheckBSV_AddressNotFound covers the BSV UTXO path: the balance and
// discovery services are constructed, then the missing --address short-circuits via
// findWalletAddress before CheckAddress makes any network call. Not parallel.
func TestRunReceiveCheckBSV_AddressNotFound(t *testing.T) {
	origAll, origAddr := receiveAll, receiveAddress
	defer func() { receiveAll, receiveAddress = origAll, origAddr }()
	receiveAll, receiveAddress = false, "1notinwallet"

	cmdCtx := &CommandContext{ //nolint:govet // local variable, not shadowing
		Cfg: &mockConfigProvider{},
		Fmt: &mockFormatProvider{format: output.FormatText},
	}
	store := utxostore.New(t.TempDir())
	wlt := &wallet.Wallet{
		Name:      "w",
		Addresses: map[chain.ID][]wallet.Address{chain.BSV: {{Address: "1real"}}},
	}

	var buf bytes.Buffer
	err := runReceiveCheckBSV(context.Background(), &buf, cmdCtx, wlt, store, nil, chain.BSV)
	require.Error(t, err)
	assert.ErrorIs(t, err, sigilerr.ErrInvalidInput)
}
