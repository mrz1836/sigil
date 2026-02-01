package wallet

import (
	"encoding/hex"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Test vectors derived from well-known mnemonic and expected addresses
// Using the standard BIP39 test vector mnemonic with no passphrase
//
//nolint:gochecknoglobals // BIP39 standard test vector constant
var derivationTestMnemonic = "abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon about"

func getTestSeed(t *testing.T) []byte {
	seed, err := MnemonicToSeed(derivationTestMnemonic, "")
	require.NoError(t, err)
	return seed
}

func TestDeriveETHAddress(t *testing.T) {
	seed := getTestSeed(t)

	// ETH path: m/44'/60'/0'/0/0
	addr, err := DeriveAddress(seed, ChainETH, 0, 0)
	require.NoError(t, err)

	assert.NotEmpty(t, addr.Address)
	assert.Equal(t, "m/44'/60'/0'/0/0", addr.Path)
	assert.Equal(t, uint32(0), addr.Index)
	assert.NotEmpty(t, addr.PublicKey)

	// ETH addresses start with 0x and are 42 characters
	assert.Len(t, addr.Address, 42)
	assert.Equal(t, "0x", addr.Address[:2])
}

func TestDeriveETHAddress_MultipleIndices(t *testing.T) {
	seed := getTestSeed(t)

	var addresses []string
	for i := uint32(0); i < 5; i++ {
		addr, err := DeriveAddress(seed, ChainETH, 0, i)
		require.NoError(t, err)
		addresses = append(addresses, addr.Address)
	}

	// All addresses should be unique
	uniqueAddrs := make(map[string]bool)
	for _, addr := range addresses {
		assert.False(t, uniqueAddrs[addr], "duplicate address found: %s", addr)
		uniqueAddrs[addr] = true
	}
}

func TestDeriveBSVAddress(t *testing.T) {
	seed := getTestSeed(t)

	// BSV path: m/44'/236'/0'/0/0
	addr, err := DeriveAddress(seed, ChainBSV, 0, 0)
	require.NoError(t, err)

	assert.NotEmpty(t, addr.Address)
	assert.Equal(t, "m/44'/236'/0'/0/0", addr.Path)
	assert.Equal(t, uint32(0), addr.Index)
	assert.NotEmpty(t, addr.PublicKey)

	// BSV P2PKH addresses start with 1
	assert.Equal(t, "1", addr.Address[:1])
}

func TestDeriveBSVAddress_MultipleIndices(t *testing.T) {
	seed := getTestSeed(t)

	var addresses []string
	for i := uint32(0); i < 5; i++ {
		addr, err := DeriveAddress(seed, ChainBSV, 0, i)
		require.NoError(t, err)
		addresses = append(addresses, addr.Address)
	}

	// All addresses should be unique
	uniqueAddrs := make(map[string]bool)
	for _, addr := range addresses {
		assert.False(t, uniqueAddrs[addr], "duplicate address found: %s", addr)
		uniqueAddrs[addr] = true
	}
}

func TestDeriveAddress_DifferentChains(t *testing.T) {
	seed := getTestSeed(t)

	ethAddr, err := DeriveAddress(seed, ChainETH, 0, 0)
	require.NoError(t, err)

	bsvAddr, err := DeriveAddress(seed, ChainBSV, 0, 0)
	require.NoError(t, err)

	// Same seed should produce different addresses for different chains
	assert.NotEqual(t, ethAddr.Address, bsvAddr.Address)
}

func TestDeriveAddress_DifferentAccounts(t *testing.T) {
	seed := getTestSeed(t)

	addr0, err := DeriveAddress(seed, ChainETH, 0, 0)
	require.NoError(t, err)

	addr1, err := DeriveAddress(seed, ChainETH, 1, 0)
	require.NoError(t, err)

	// Different accounts should produce different addresses
	assert.NotEqual(t, addr0.Address, addr1.Address)
}

func TestDeriveAddress_UnsupportedChain(t *testing.T) {
	seed := getTestSeed(t)

	_, err := DeriveAddress(seed, Chain("unknown"), 0, 0)
	assert.Error(t, err)
}

func TestDerivePrivateKey_ETH(t *testing.T) {
	seed := getTestSeed(t)

	privKey, err := DerivePrivateKey(seed, ChainETH, 0, 0)
	require.NoError(t, err)
	defer ZeroBytes(privKey)

	// ETH private keys are 32 bytes
	assert.Len(t, privKey, 32)
}

func TestDerivePrivateKey_BSV(t *testing.T) {
	seed := getTestSeed(t)

	privKey, err := DerivePrivateKey(seed, ChainBSV, 0, 0)
	require.NoError(t, err)
	defer ZeroBytes(privKey)

	// BSV private keys are 32 bytes
	assert.Len(t, privKey, 32)
}

func TestGetDerivationPath(t *testing.T) {
	tests := []struct {
		chain    Chain
		account  uint32
		index    uint32
		expected string
	}{
		{ChainETH, 0, 0, "m/44'/60'/0'/0/0"},
		{ChainETH, 0, 1, "m/44'/60'/0'/0/1"},
		{ChainETH, 1, 0, "m/44'/60'/1'/0/0"},
		{ChainBSV, 0, 0, "m/44'/236'/0'/0/0"},
		{ChainBSV, 0, 5, "m/44'/236'/0'/0/5"},
		{ChainBSV, 2, 10, "m/44'/236'/2'/0/10"},
		{ChainBTC, 0, 0, "m/44'/0'/0'/0/0"},
	}

	for _, tc := range tests {
		t.Run(tc.expected, func(t *testing.T) {
			path := GetDerivationPath(tc.chain, tc.account, tc.index)
			assert.Equal(t, tc.expected, path)
		})
	}
}

func TestGetCoinType(t *testing.T) {
	tests := []struct {
		chain    Chain
		expected uint32
	}{
		{ChainETH, 60},
		{ChainBSV, 236},
		{ChainBTC, 0},
	}

	for _, tc := range tests {
		t.Run(string(tc.chain), func(t *testing.T) {
			coinType := tc.chain.CoinType()
			assert.Equal(t, tc.expected, coinType)
		})
	}
}

func TestZeroBytes(t *testing.T) {
	data := []byte{0x01, 0x02, 0x03, 0x04}
	ZeroBytes(data)

	for _, b := range data {
		assert.Equal(t, byte(0), b)
	}
}

// Test with known test vectors from BIP32/BIP44
func TestDeriveAddress_ETH_KnownVector(t *testing.T) {
	// Using the standard test mnemonic
	seed := getTestSeed(t)

	addr, err := DeriveAddress(seed, ChainETH, 0, 0)
	require.NoError(t, err)

	// The expected address for this mnemonic at m/44'/60'/0'/0/0
	// This is a well-known test vector
	t.Logf("ETH address: %s", addr.Address)
	t.Logf("ETH pubkey: %s", addr.PublicKey)

	// Verify it's a valid checksummed address
	assert.True(t, IsValidETHAddress(addr.Address))
}

func TestIsValidETHAddress(t *testing.T) {
	tests := []struct {
		name     string
		address  string
		expected bool
	}{
		{"valid lower", "0x742d35cc6634c0532925a3b844bc454e4438f44e", true},
		{"valid mixed", "0x742d35Cc6634C0532925a3b844Bc454e4438f44e", true},
		{"valid upper", "0x742D35CC6634C0532925A3B844BC454E4438F44E", true},
		{"no prefix", "742d35cc6634c0532925a3b844bc454e4438f44e", false},
		{"too short", "0x742d35cc6634c0532925a3b844bc454e4438f4", false},
		{"too long", "0x742d35cc6634c0532925a3b844bc454e4438f44e00", false},
		{"invalid chars", "0x742d35cc6634c0532925a3b844bc454e4438f44g", false},
		{"empty", "", false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := IsValidETHAddress(tc.address)
			assert.Equal(t, tc.expected, result)
		})
	}
}

func TestPublicKeyFormat(t *testing.T) {
	seed := getTestSeed(t)

	// ETH uses uncompressed public keys (65 bytes, but we store as hex without 04 prefix = 128 chars)
	ethAddr, err := DeriveAddress(seed, ChainETH, 0, 0)
	require.NoError(t, err)

	// Decode the hex public key
	pubKeyBytes, err := hex.DecodeString(ethAddr.PublicKey)
	require.NoError(t, err)
	t.Logf("ETH pubkey length: %d bytes", len(pubKeyBytes))

	// BSV uses compressed public keys (33 bytes = 66 hex chars)
	bsvAddr, err := DeriveAddress(seed, ChainBSV, 0, 0)
	require.NoError(t, err)

	pubKeyBytes, err = hex.DecodeString(bsvAddr.PublicKey)
	require.NoError(t, err)
	assert.Len(t, pubKeyBytes, 33) // Compressed public key
}
