package cli

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/mrz1836/sigil/internal/chain/bsv"
	"github.com/mrz1836/sigil/internal/wallet"
)

func TestEffectiveBSVNetwork(t *testing.T) {
	t.Parallel()

	cfg := &mockConfigProvider{bsvNetwork: "test"}

	// A wallet's stamped network wins over config.
	mainWallet := &wallet.Wallet{Name: "m", Network: "main"}
	assert.Equal(t, "main", effectiveBSVNetwork(mainWallet, cfg))

	// A legacy wallet (empty network) falls back to config.
	legacy := &wallet.Wallet{Name: "l"}
	assert.Equal(t, "test", effectiveBSVNetwork(legacy, cfg))

	// No wallet falls back to config.
	assert.Equal(t, "test", effectiveBSVNetwork(nil, cfg))

	// A testnet wallet on a mainnet-config still resolves to testnet.
	testWallet := &wallet.Wallet{Name: "t", Network: "test"}
	assert.Equal(t, "test", effectiveBSVNetwork(testWallet, &mockConfigProvider{bsvNetwork: "main"}))
}

func TestBsvClientAndWalletNetworkMapping(t *testing.T) {
	t.Parallel()

	assert.Equal(t, bsv.NetworkTestnet, bsvClientNetwork("test"))
	assert.Equal(t, bsv.NetworkMainnet, bsvClientNetwork("main"))
	assert.Equal(t, bsv.NetworkMainnet, bsvClientNetwork(""))

	assert.Equal(t, wallet.Testnet, walletNetwork("test"))
	assert.Equal(t, wallet.Mainnet, walletNetwork("main"))
}

func TestBSVExplorerLinks(t *testing.T) {
	t.Parallel()

	// Mainnet: single WhatsOnChain link.
	mainTx := bsvExplorerTxLinks("main", "deadbeef")
	assert.Equal(t, []string{"https://whatsonchain.com/tx/deadbeef"}, mainTx)

	// Testnet: WhatsOnChain test (primary) + bananablocks test (secondary).
	testTx := bsvExplorerTxLinks("test", "deadbeef")
	assert.Equal(t, []string{
		"https://test.whatsonchain.com/tx/deadbeef",
		"https://test.bananablocks.com/tx/deadbeef",
	}, testTx)

	testAddr := bsvExplorerAddressLinks("test", "mrCDrCybB6J1vRfbwM5hemdJz73FwDBC8r")
	assert.Len(t, testAddr, 2)
	assert.Contains(t, testAddr[0], "test.whatsonchain.com/address/")
	assert.Contains(t, testAddr[1], "test.bananablocks.com/address/")
}
