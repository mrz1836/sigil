package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/mrz1836/sigil/internal/chain/bsv"
	"github.com/mrz1836/sigil/internal/wallet"
)

// effectiveBSVNetwork resolves the network for a wallet-scoped operation.
// A wallet's own stamped network takes precedence over the global config value,
// so a mainnet wallet is never queried or spent on testnet (and vice versa).
// Legacy wallets (empty Network) fall back to the global config network.
func effectiveBSVNetwork(w *wallet.Wallet, cfg ConfigProvider) string {
	if w != nil && w.Network != "" {
		return normalizeNetworkString(w.Network)
	}
	if cfg != nil {
		return cfg.GetBSVNetwork()
	}
	return "main"
}

// bsvNetworkForCmd returns the effective global BSV network string for a command
// that has no wallet in scope (e.g. keygen, discover). It reads the resolved
// config from the command context, defaulting to mainnet.
func bsvNetworkForCmd(cmd *cobra.Command) string {
	cc := GetCmdContext(cmd)
	if cc == nil || cc.Cfg == nil {
		return "main"
	}
	return cc.Cfg.GetBSVNetwork()
}

// bsvClientNetwork maps a network string ("main"/"test") to the bsv package's
// Network type used by bsv.ClientOptions.
func bsvClientNetwork(network string) bsv.Network {
	if network == "test" {
		return bsv.NetworkTestnet
	}
	return bsv.NetworkMainnet
}

// walletNetwork maps a network string to the wallet package's encoding Network.
func walletNetwork(network string) wallet.Network {
	return wallet.NetworkFromString(network)
}

// normalizeNetworkString normalizes a raw network string to "main"/"test",
// defaulting unknown/empty values to "main".
func normalizeNetworkString(s string) string {
	if s == "test" || s == "testnet" {
		return "test"
	}
	return "main"
}

// bsvExplorerTxLinks returns explorer URLs for a BSV transaction. On testnet it
// returns WhatsOnChain test (primary) and bananablocks test (secondary); on
// mainnet it returns the single WhatsOnChain link.
func bsvExplorerTxLinks(network, txid string) []string {
	if network == "test" {
		return []string{
			"https://test.whatsonchain.com/tx/" + txid,
			"https://test.bananablocks.com/tx/" + txid,
		}
	}
	return []string{"https://whatsonchain.com/tx/" + txid}
}

// bsvExplorerAddressLinks returns explorer URLs for a BSV address (see bsvExplorerTxLinks).
func bsvExplorerAddressLinks(network, address string) []string {
	if network == "test" {
		return []string{
			"https://test.whatsonchain.com/address/" + address,
			"https://test.bananablocks.com/address/" + address,
		}
	}
	return []string{"https://whatsonchain.com/address/" + address}
}

// warnNetworkConflict prints a fail-closed warning when a --network/--testnet flag
// disagrees with a loaded wallet's stamped network. The wallet's network is honored.
func warnNetworkConflict(cmd *cobra.Command, w *wallet.Wallet) {
	if w == nil || w.Network == "" {
		return
	}
	if !cmd.Flags().Changed("network") && !cmd.Flags().Changed("testnet") {
		return
	}
	requested := bsvNetworkForCmd(cmd)
	walletNet := normalizeNetworkString(w.Network)
	if requested != walletNet {
		fmt.Fprintf(os.Stderr,
			"Warning: --network %s ignored; wallet %q is a %s wallet\n",
			requested, w.Name, walletNet)
	}
}
