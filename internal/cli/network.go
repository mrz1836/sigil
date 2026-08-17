package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/mrz1836/sigil/internal/chain"
	"github.com/mrz1836/sigil/internal/chain/bsv"
	"github.com/mrz1836/sigil/internal/chain/btc"
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

// effectiveBTCNetwork resolves the network for a wallet-scoped BTC operation.
// A wallet's own stamped network takes precedence over the global config value.
func effectiveBTCNetwork(w *wallet.Wallet, cfg ConfigProvider) string {
	if w != nil && w.Network != "" {
		return normalizeNetworkString(w.Network)
	}
	if cfg != nil {
		return cfg.GetBTCNetwork()
	}
	return "main"
}

// effectiveNetworkForChain resolves the wallet-scoped network for a UTXO chain,
// dispatching to the chain-specific resolver so BTC honors its own config
// fallback. For stamped wallets both resolvers return the wallet's network.
func effectiveNetworkForChain(w *wallet.Wallet, cfg ConfigProvider, chainID chain.ID) string {
	if chainID == chain.BTC {
		return effectiveBTCNetwork(w, cfg)
	}
	return effectiveBSVNetwork(w, cfg)
}

// btcClientNetwork maps a network string ("main"/"test") to the btc package's
// Network type used by btc.ClientOptions.
func btcClientNetwork(network string) btc.Network {
	if network == "test" {
		return btc.NetworkTestnet
	}
	return btc.NetworkMainnet
}

// btcExplorerTxLinks returns explorer URLs for a BTC transaction on mempool.space.
// Testnet4 lives under the /testnet4/ path prefix.
func btcExplorerTxLinks(network, txid string) []string {
	if network == "test" {
		return []string{"https://mempool.space/testnet4/tx/" + txid}
	}
	return []string{"https://mempool.space/tx/" + txid}
}

// btcExplorerAddressLinks returns explorer URLs for a BTC address (see btcExplorerTxLinks).
func btcExplorerAddressLinks(network, address string) []string {
	if network == "test" {
		return []string{"https://mempool.space/testnet4/address/" + address}
	}
	return []string{"https://mempool.space/address/" + address}
}

// ethExplorerTxLinks returns the Etherscan URL for an ETH transaction. Ethereum
// surfaces a single mainnet explorer link (testnet links are not shown).
func ethExplorerTxLinks(txid string) []string {
	return []string{"https://etherscan.io/tx/" + txid}
}

// ethExplorerAddressLinks returns the Etherscan URL for an ETH address (see ethExplorerTxLinks).
func ethExplorerAddressLinks(address string) []string {
	return []string{"https://etherscan.io/address/" + address}
}

// explorerAddressLinks returns block-explorer URLs for an address on the given
// chain and network, dispatching to the per-chain builders. Unknown chains
// return nil.
func explorerAddressLinks(chainID chain.ID, network, address string) []string {
	switch chainID {
	case chain.BSV:
		return bsvExplorerAddressLinks(network, address)
	case chain.BTC:
		return btcExplorerAddressLinks(network, address)
	case chain.ETH:
		return ethExplorerAddressLinks(address)
	case chain.BCH, chain.LTC:
		return nil // Future chains - no explorer link yet
	default:
		return nil
	}
}

// explorerTxLinks returns block-explorer URLs for a transaction (see explorerAddressLinks).
func explorerTxLinks(chainID chain.ID, network, txid string) []string {
	switch chainID {
	case chain.BSV:
		return bsvExplorerTxLinks(network, txid)
	case chain.BTC:
		return btcExplorerTxLinks(network, txid)
	case chain.ETH:
		return ethExplorerTxLinks(txid)
	case chain.BCH, chain.LTC:
		return nil // Future chains - no explorer link yet
	default:
		return nil
	}
}

// explorerLabel returns the human-readable header printed before a chain's
// explorer links. ETH links point at Etherscan; other chains use a generic label.
func explorerLabel(chainID chain.ID) string {
	if chainID == chain.ETH {
		return "View on Etherscan:"
	}
	return "View on block explorer:"
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
