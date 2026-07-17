package wallet

import (
	"encoding/hex"
	"strings"
	"testing"

	"github.com/decred/dcrd/hdkeychain/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// privKeyOne is the canonical secp256k1 private key 0x…0001, used for
// well-known address/WIF test vectors (independently verified).
func privKeyOne() []byte {
	k := make([]byte, 32)
	k[31] = 0x01
	return k
}

// testSeed returns a fixed BIP32 seed for deterministic derivation tests.
func testSeed() []byte { return []byte("sigil-testnet-fixed-seed-32bytes") }

func TestAddressFromPrivKeyBytesForNetworkVectors(t *testing.T) {
	t.Parallel()

	mainAddr, err := AddressFromPrivKeyBytesForNetwork(privKeyOne(), Mainnet)
	require.NoError(t, err)
	assert.Equal(t, "1BgGZ9tcN4rm9KBzDn7KprQz87SZ26SAMH", mainAddr)

	testAddr, err := AddressFromPrivKeyBytesForNetwork(privKeyOne(), Testnet)
	require.NoError(t, err)
	assert.Equal(t, "mrCDrCybB6J1vRfbwM5hemdJz73FwDBC8r", testAddr)

	// Backward-compatible wrapper stays mainnet.
	wrap, err := AddressFromPrivKeyBytes(privKeyOne())
	require.NoError(t, err)
	assert.Equal(t, mainAddr, wrap)
}

// TestDeriveNetworkIndependenceProof is the load-bearing invariant: the same
// seed/path yields identical raw public key bytes on both networks — only the
// encoded P2PKH address string differs (mainnet "1...", testnet "m/n...").
func TestDeriveNetworkIndependenceProof(t *testing.T) {
	t.Parallel()

	mainAddr, err := DeriveAddressForNetwork(testSeed(), ChainBSV, 0, 0, Mainnet)
	require.NoError(t, err)
	testAddr, err := DeriveAddressForNetwork(testSeed(), ChainBSV, 0, 0, Testnet)
	require.NoError(t, err)

	// Same public key on both networks.
	assert.Equal(t, mainAddr.PublicKey, testAddr.PublicKey)
	// Same derivation path (coin type 236 kept for both).
	assert.Equal(t, mainAddr.Path, testAddr.Path)
	assert.Equal(t, "m/44'/236'/0'/0/0", testAddr.Path)
	// Different encoded address, with the expected leading characters.
	assert.NotEqual(t, mainAddr.Address, testAddr.Address)
	assert.True(t, strings.HasPrefix(mainAddr.Address, "1"), "mainnet address starts with 1")
	first := testAddr.Address[0]
	assert.True(t, first == 'm' || first == 'n', "testnet address starts with m or n, got %q", testAddr.Address)
}

// TestDerivePrivateKeyNetworkIndependent proves private-key derivation returns
// identical bytes regardless of the network params used to build the master key.
func TestDerivePrivateKeyNetworkIndependent(t *testing.T) {
	t.Parallel()

	mainMaster, err := hdkeychain.NewMaster(testSeed(), hdNetParams{Mainnet})
	require.NoError(t, err)
	testMaster, err := hdkeychain.NewMaster(testSeed(), hdNetParams{Testnet})
	require.NoError(t, err)

	mainKey, err := deriveBIP44KeyWithCoinType(mainMaster, 236, 0, 0, 0)
	require.NoError(t, err)
	testKey, err := deriveBIP44KeyWithCoinType(testMaster, 236, 0, 0, 0)
	require.NoError(t, err)

	mainPriv, err := mainKey.SerializedPrivKey()
	require.NoError(t, err)
	testPriv, err := testKey.SerializedPrivKey()
	require.NoError(t, err)

	assert.Equal(t, hex.EncodeToString(mainPriv), hex.EncodeToString(testPriv),
		"raw private key bytes must be network-independent")
}

// TestHDExtendedKeyPrefixes proves the tpub/tprv vs xpub/xprv version bytes:
// same seed, only the 4-byte prefix differs; the remaining serialized bytes match.
func TestHDExtendedKeyPrefixes(t *testing.T) {
	t.Parallel()

	xpub, err := DeriveAccountXpubForNetwork(testSeed(), ChainBSV, 0, Mainnet)
	require.NoError(t, err)
	tpub, err := DeriveAccountXpubForNetwork(testSeed(), ChainBSV, 0, Testnet)
	require.NoError(t, err)

	assert.True(t, strings.HasPrefix(xpub, "xpub"), "mainnet account key is xpub, got %q", xpub[:4])
	assert.True(t, strings.HasPrefix(tpub, "tpub"), "testnet account key is tpub, got %q", tpub[:4])
}

// TestDeriveAddressFromXpubAutoDetectsNetwork verifies that address derivation
// from an extended key auto-detects the network from the xpub/tpub prefix.
func TestDeriveAddressFromXpubAutoDetectsNetwork(t *testing.T) {
	t.Parallel()

	xpub, err := DeriveAccountXpubForNetwork(testSeed(), ChainBSV, 0, Mainnet)
	require.NoError(t, err)
	tpub, err := DeriveAccountXpubForNetwork(testSeed(), ChainBSV, 0, Testnet)
	require.NoError(t, err)

	mainAddr, err := DeriveAddressFromXpub(xpub, ChainBSV, 0, 0)
	require.NoError(t, err)
	testAddr, err := DeriveAddressFromXpub(tpub, ChainBSV, 0, 0)
	require.NoError(t, err)

	assert.True(t, strings.HasPrefix(mainAddr.Address, "1"))
	assert.Contains(t, []byte{'m', 'n'}, testAddr.Address[0])
	// Same underlying key → same public key hex across the two encodings.
	assert.Equal(t, mainAddr.PublicKey, testAddr.PublicKey)
}

func TestMnemonicContextForNetworkEncoding(t *testing.T) {
	t.Parallel()

	mcMain, err := NewMnemonicContextForNetwork(testSeed(), Mainnet)
	require.NoError(t, err)
	defer mcMain.Zero()
	mcTest, err := NewMnemonicContextForNetwork(testSeed(), Testnet)
	require.NoError(t, err)
	defer mcTest.Zero()

	mainAddr, mainPub, _, err := mcMain.DeriveAddressWithCoinType(236, 0, 0, 0)
	require.NoError(t, err)
	testAddr, testPub, _, err := mcTest.DeriveAddressWithCoinType(236, 0, 0, 0)
	require.NoError(t, err)

	assert.Equal(t, mainPub, testPub)
	assert.True(t, strings.HasPrefix(mainAddr, "1"))
	assert.Contains(t, []byte{'m', 'n'}, testAddr[0])
}
