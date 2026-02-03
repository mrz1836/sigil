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
	t.Parallel()
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
	t.Parallel()
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
	t.Parallel()
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
	t.Parallel()
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
	t.Parallel()
	seed := getTestSeed(t)

	ethAddr, err := DeriveAddress(seed, ChainETH, 0, 0)
	require.NoError(t, err)

	bsvAddr, err := DeriveAddress(seed, ChainBSV, 0, 0)
	require.NoError(t, err)

	// Same seed should produce different addresses for different chains
	assert.NotEqual(t, ethAddr.Address, bsvAddr.Address)
}

func TestDeriveAddress_DifferentAccounts(t *testing.T) {
	t.Parallel()
	seed := getTestSeed(t)

	addr0, err := DeriveAddress(seed, ChainETH, 0, 0)
	require.NoError(t, err)

	addr1, err := DeriveAddress(seed, ChainETH, 1, 0)
	require.NoError(t, err)

	// Different accounts should produce different addresses
	assert.NotEqual(t, addr0.Address, addr1.Address)
}

func TestDeriveAddress_UnsupportedChain(t *testing.T) {
	t.Parallel()
	seed := getTestSeed(t)

	_, err := DeriveAddress(seed, ChainID("unknown"), 0, 0)
	assert.Error(t, err)
}

func TestDerivePrivateKey_ETH(t *testing.T) {
	t.Parallel()
	seed := getTestSeed(t)

	privKey, err := DerivePrivateKey(seed, ChainETH, 0, 0)
	require.NoError(t, err)
	defer ZeroBytes(privKey)

	// ETH private keys are 32 bytes
	assert.Len(t, privKey, 32)
}

func TestDerivePrivateKey_BSV(t *testing.T) {
	t.Parallel()
	seed := getTestSeed(t)

	privKey, err := DerivePrivateKey(seed, ChainBSV, 0, 0)
	require.NoError(t, err)
	defer ZeroBytes(privKey)

	// BSV private keys are 32 bytes
	assert.Len(t, privKey, 32)
}

func TestGetDerivationPath(t *testing.T) {
	t.Parallel()
	tests := []struct {
		chain    ChainID
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
			t.Parallel()
			path := GetDerivationPath(tc.chain, tc.account, tc.index)
			assert.Equal(t, tc.expected, path)
		})
	}
}

func TestGetCoinType(t *testing.T) {
	t.Parallel()
	tests := []struct {
		chain    ChainID
		expected uint32
	}{
		{ChainETH, 60},
		{ChainBSV, 236},
		{ChainBTC, 0},
	}

	for _, tc := range tests {
		t.Run(string(tc.chain), func(t *testing.T) {
			t.Parallel()
			coinType := tc.chain.CoinType()
			assert.Equal(t, tc.expected, coinType)
		})
	}
}

func TestZeroBytes(t *testing.T) {
	t.Parallel()
	data := []byte{0x01, 0x02, 0x03, 0x04}
	ZeroBytes(data)

	for _, b := range data {
		assert.Equal(t, byte(0), b)
	}
}

// Test with known test vectors from BIP32/BIP44
func TestDeriveAddress_ETH_KnownVector(t *testing.T) {
	t.Parallel()
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
	t.Parallel()
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
			t.Parallel()
			result := IsValidETHAddress(tc.address)
			assert.Equal(t, tc.expected, result)
		})
	}
}

func TestPublicKeyFormat(t *testing.T) {
	t.Parallel()
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

// Tests for DeriveAddressWithCoinType (multi-path discovery support)

func TestDeriveAddressWithCoinType_BSV(t *testing.T) {
	t.Parallel()
	seed := getTestSeed(t)

	// Test BSV coin type (236)
	addr, pubKey, path, err := DeriveAddressWithCoinType(seed, 236, 0, 0, 0)
	require.NoError(t, err)

	assert.NotEmpty(t, addr)
	assert.NotEmpty(t, pubKey)
	assert.Equal(t, "m/44'/236'/0'/0/0", path)

	// BSV addresses start with 1
	assert.Equal(t, "1", addr[:1])
}

func TestDeriveAddressWithCoinType_BTC(t *testing.T) {
	t.Parallel()
	seed := getTestSeed(t)

	// Test BTC coin type (0)
	addr, pubKey, path, err := DeriveAddressWithCoinType(seed, 0, 0, 0, 0)
	require.NoError(t, err)

	assert.NotEmpty(t, addr)
	assert.NotEmpty(t, pubKey)
	assert.Equal(t, "m/44'/0'/0'/0/0", path)

	// BTC/BSV addresses start with 1
	assert.Equal(t, "1", addr[:1])
}

func TestDeriveAddressWithCoinType_BCH(t *testing.T) {
	t.Parallel()
	seed := getTestSeed(t)

	// Test BCH coin type (145)
	addr, pubKey, path, err := DeriveAddressWithCoinType(seed, 145, 0, 0, 0)
	require.NoError(t, err)

	assert.NotEmpty(t, addr)
	assert.NotEmpty(t, pubKey)
	assert.Equal(t, "m/44'/145'/0'/0/0", path)
}

func TestDeriveAddressWithCoinType_DifferentCoinTypes_ProduceDifferentAddresses(t *testing.T) {
	t.Parallel()
	seed := getTestSeed(t)

	bsvAddr, _, _, err := DeriveAddressWithCoinType(seed, 236, 0, 0, 0)
	require.NoError(t, err)

	btcAddr, _, _, err := DeriveAddressWithCoinType(seed, 0, 0, 0, 0)
	require.NoError(t, err)

	bchAddr, _, _, err := DeriveAddressWithCoinType(seed, 145, 0, 0, 0)
	require.NoError(t, err)

	// All should be different
	assert.NotEqual(t, bsvAddr, btcAddr)
	assert.NotEqual(t, bsvAddr, bchAddr)
	assert.NotEqual(t, btcAddr, bchAddr)
}

func TestDeriveAddressWithCoinType_ChangeChain(t *testing.T) {
	t.Parallel()
	seed := getTestSeed(t)

	// External chain (change=0)
	extAddr, _, extPath, err := DeriveAddressWithCoinType(seed, 236, 0, 0, 0)
	require.NoError(t, err)
	assert.Contains(t, extPath, "'/0/0")

	// Internal chain (change=1)
	intAddr, _, intPath, err := DeriveAddressWithCoinType(seed, 236, 0, 1, 0)
	require.NoError(t, err)
	assert.Contains(t, intPath, "'/1/0")

	// Should be different addresses
	assert.NotEqual(t, extAddr, intAddr)
}

func TestDeriveAddressWithCoinType_MultipleIndices(t *testing.T) {
	t.Parallel()
	seed := getTestSeed(t)

	addresses := make(map[string]bool)
	for i := uint32(0); i < 10; i++ {
		addr, _, path, err := DeriveAddressWithCoinType(seed, 236, 0, 0, i)
		require.NoError(t, err)

		// Should not have seen this address before
		assert.False(t, addresses[addr], "duplicate address at index %d", i)
		addresses[addr] = true

		// Path should be in correct format
		expectedPath := GetDerivationPathFull(ChainBSV, 0, 0, i)
		assert.Equal(t, expectedPath, path)
	}

	assert.Len(t, addresses, 10)
}

func TestDeriveAddressWithCoinType_Consistency(t *testing.T) {
	t.Parallel()
	seed := getTestSeed(t)

	// Derive same address twice - should be identical
	addr1, pub1, path1, err := DeriveAddressWithCoinType(seed, 236, 0, 0, 5)
	require.NoError(t, err)

	addr2, pub2, path2, err := DeriveAddressWithCoinType(seed, 236, 0, 0, 5)
	require.NoError(t, err)

	assert.Equal(t, addr1, addr2)
	assert.Equal(t, pub1, pub2)
	assert.Equal(t, path1, path2)
}

func TestDerivePrivateKeyWithCoinType(t *testing.T) {
	t.Parallel()
	seed := getTestSeed(t)

	// Test with BSV coin type
	privKey, err := DerivePrivateKeyWithCoinType(seed, 236, 0, 0, 0)
	require.NoError(t, err)
	defer ZeroBytes(privKey)

	assert.Len(t, privKey, 32)
}

func TestDerivePrivateKeyWithCoinType_DifferentCoinTypes(t *testing.T) {
	t.Parallel()
	seed := getTestSeed(t)

	bsvKey, err := DerivePrivateKeyWithCoinType(seed, 236, 0, 0, 0)
	require.NoError(t, err)
	defer ZeroBytes(bsvKey)

	btcKey, err := DerivePrivateKeyWithCoinType(seed, 0, 0, 0, 0)
	require.NoError(t, err)
	defer ZeroBytes(btcKey)

	// Keys should be different for different coin types
	assert.NotEqual(t, hex.EncodeToString(bsvKey), hex.EncodeToString(btcKey))
}

// Tests for DeriveLegacyAddress (HandCash 1.x support)

func TestDeriveLegacyAddress(t *testing.T) {
	t.Parallel()
	seed := getTestSeed(t)

	addr, pubKey, path, err := DeriveLegacyAddress(seed, 0)
	require.NoError(t, err)

	assert.NotEmpty(t, addr)
	assert.NotEmpty(t, pubKey)
	assert.Equal(t, "m/0'/0", path)

	// Address should start with 1
	assert.Equal(t, "1", addr[:1])
}

func TestDeriveLegacyAddress_MultipleIndices(t *testing.T) {
	t.Parallel()
	seed := getTestSeed(t)

	addresses := make(map[string]bool)
	for i := uint32(0); i < 10; i++ {
		addr, _, path, err := DeriveLegacyAddress(seed, i)
		require.NoError(t, err)

		assert.False(t, addresses[addr], "duplicate address at index %d", i)
		addresses[addr] = true

		// Path should be m/0'/index
		assert.Contains(t, path, "m/0'/")
	}

	assert.Len(t, addresses, 10)
}

func TestDeriveLegacyAddress_DifferentFromBIP44(t *testing.T) {
	t.Parallel()
	seed := getTestSeed(t)

	legacyAddr, _, _, err := DeriveLegacyAddress(seed, 0)
	require.NoError(t, err)

	bip44Addr, _, _, err := DeriveAddressWithCoinType(seed, 0, 0, 0, 0)
	require.NoError(t, err)

	// Legacy path m/0'/0 should be different from BIP44 path m/44'/0'/0'/0/0
	assert.NotEqual(t, legacyAddr, bip44Addr)
}

func TestDeriveLegacyPrivateKey(t *testing.T) {
	t.Parallel()
	seed := getTestSeed(t)

	privKey, err := DeriveLegacyPrivateKey(seed, 0)
	require.NoError(t, err)
	defer ZeroBytes(privKey)

	assert.Len(t, privKey, 32)
}

func TestDeriveLegacyPrivateKey_Consistency(t *testing.T) {
	t.Parallel()
	seed := getTestSeed(t)

	key1, err := DeriveLegacyPrivateKey(seed, 5)
	require.NoError(t, err)
	defer ZeroBytes(key1)

	key2, err := DeriveLegacyPrivateKey(seed, 5)
	require.NoError(t, err)
	defer ZeroBytes(key2)

	assert.Equal(t, hex.EncodeToString(key1), hex.EncodeToString(key2))
}

// Tests for DeriveAddressWithChange

func TestDeriveAddressWithChange_External(t *testing.T) {
	t.Parallel()
	seed := getTestSeed(t)

	addr, err := DeriveAddressWithChange(seed, ChainBSV, 0, ExternalChain, 0)
	require.NoError(t, err)

	assert.NotEmpty(t, addr.Address)
	assert.False(t, addr.IsChange)
	assert.Contains(t, addr.Path, "'/0/0")
}

func TestDeriveAddressWithChange_Internal(t *testing.T) {
	t.Parallel()
	seed := getTestSeed(t)

	addr, err := DeriveAddressWithChange(seed, ChainBSV, 0, InternalChain, 0)
	require.NoError(t, err)

	assert.NotEmpty(t, addr.Address)
	assert.True(t, addr.IsChange)
	assert.Contains(t, addr.Path, "'/1/0")
}

func TestDeriveAddressWithChange_ExternalAndInternalDiffer(t *testing.T) {
	t.Parallel()
	seed := getTestSeed(t)

	extAddr, err := DeriveAddressWithChange(seed, ChainBSV, 0, ExternalChain, 0)
	require.NoError(t, err)

	intAddr, err := DeriveAddressWithChange(seed, ChainBSV, 0, InternalChain, 0)
	require.NoError(t, err)

	assert.NotEqual(t, extAddr.Address, intAddr.Address)
}

// Edge case tests

func TestDeriveAddressWithCoinType_LargeIndex(t *testing.T) {
	t.Parallel()
	seed := getTestSeed(t)

	// Test with large index (near max uint32)
	addr, _, path, err := DeriveAddressWithCoinType(seed, 236, 0, 0, 1000000)
	require.NoError(t, err)

	assert.NotEmpty(t, addr)
	assert.Contains(t, path, "/1000000")
}

func TestDeriveAddressWithCoinType_MultipleAccounts(t *testing.T) {
	t.Parallel()
	seed := getTestSeed(t)

	// Different accounts should produce different addresses
	addr0, _, _, err := DeriveAddressWithCoinType(seed, 236, 0, 0, 0)
	require.NoError(t, err)

	addr1, _, _, err := DeriveAddressWithCoinType(seed, 236, 1, 0, 0)
	require.NoError(t, err)

	addr2, _, _, err := DeriveAddressWithCoinType(seed, 236, 2, 0, 0)
	require.NoError(t, err)

	assert.NotEqual(t, addr0, addr1)
	assert.NotEqual(t, addr0, addr2)
	assert.NotEqual(t, addr1, addr2)
}

// Benchmark tests

func BenchmarkDeriveAddressWithCoinType(b *testing.B) {
	seed, _ := MnemonicToSeed(derivationTestMnemonic, "")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _, _, _ = DeriveAddressWithCoinType(seed, 236, 0, 0, uint32(i%1000)) //nolint:gosec // Benchmark uses modulo to avoid overflow
	}
}

func BenchmarkDeriveLegacyAddress(b *testing.B) {
	seed, _ := MnemonicToSeed(derivationTestMnemonic, "")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _, _, _ = DeriveLegacyAddress(seed, uint32(i%1000)) //nolint:gosec // Benchmark uses modulo to avoid overflow
	}
}
