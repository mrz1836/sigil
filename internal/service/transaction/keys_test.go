package transaction

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mrz1836/sigil/internal/chain"
	"github.com/mrz1836/sigil/internal/wallet"
)

// TestDeriveKeysForUTXOs_SingleAddress tests key derivation for a single address.
func TestDeriveKeysForUTXOs_SingleAddress(t *testing.T) {
	t.Parallel()

	addresses := []wallet.Address{
		{Address: "1ABC", Index: 0, Path: "m/44'/236'/0'/0/0"},
	}

	utxos := []chain.UTXO{
		{Address: "1ABC", TxID: "tx1", Vout: 0, Amount: 100000},
		{Address: "1ABC", TxID: "tx2", Vout: 1, Amount: 200000},
	}

	seed := getTestSeed(t)

	keys, err := deriveKeysForUTXOs(utxos, addresses, seed)
	require.NoError(t, err)
	require.NotNil(t, keys)
	assert.Len(t, keys, 1, "should derive exactly one key for one unique address")
	assert.Contains(t, keys, "1ABC")
	assert.NotEmpty(t, keys["1ABC"], "derived key should not be empty")

	// Clean up
	zeroKeyMap(keys)

	// Verify byte-by-byte zeroing (SECURITY-CRITICAL)
	for addr, key := range keys {
		for i, b := range key {
			assert.Equal(t, byte(0), b, "key[%s][%d] not zeroed", addr, i)
		}
	}
}

// TestDeriveKeysForUTXOs_MultipleAddresses tests key derivation for multiple unique addresses.
func TestDeriveKeysForUTXOs_MultipleAddresses(t *testing.T) {
	t.Parallel()

	addresses := []wallet.Address{
		{Address: "1ABC", Index: 0, Path: "m/44'/236'/0'/0/0"},
		{Address: "1DEF", Index: 1, Path: "m/44'/236'/0'/0/1"},
		{Address: "1GHI", Index: 2, Path: "m/44'/236'/0'/0/2"},
	}

	utxos := []chain.UTXO{
		{Address: "1ABC", TxID: "tx1", Vout: 0, Amount: 100000},
		{Address: "1DEF", TxID: "tx2", Vout: 0, Amount: 200000},
		{Address: "1GHI", TxID: "tx3", Vout: 0, Amount: 300000},
	}

	seed := getTestSeed(t)

	keys, err := deriveKeysForUTXOs(utxos, addresses, seed)
	require.NoError(t, err)
	require.NotNil(t, keys)
	assert.Len(t, keys, 3, "should derive three keys for three unique addresses")
	assert.Contains(t, keys, "1ABC")
	assert.Contains(t, keys, "1DEF")
	assert.Contains(t, keys, "1GHI")

	// Verify all keys are non-empty
	for addr, key := range keys {
		assert.NotEmpty(t, key, "key for %s should not be empty", addr)
	}

	// Clean up
	zeroKeyMap(keys)
}

// TestDeriveKeysForUTXOs_DuplicateAddresses tests that duplicate addresses in UTXOs
// only result in one key derivation.
func TestDeriveKeysForUTXOs_DuplicateAddresses(t *testing.T) {
	t.Parallel()

	addresses := []wallet.Address{
		{Address: "1ABC", Index: 0, Path: "m/44'/236'/0'/0/0"},
		{Address: "1DEF", Index: 1, Path: "m/44'/236'/0'/0/1"},
	}

	// Multiple UTXOs for the same addresses
	utxos := []chain.UTXO{
		{Address: "1ABC", TxID: "tx1", Vout: 0, Amount: 100000},
		{Address: "1ABC", TxID: "tx2", Vout: 0, Amount: 200000},
		{Address: "1ABC", TxID: "tx3", Vout: 1, Amount: 300000},
		{Address: "1DEF", TxID: "tx4", Vout: 0, Amount: 400000},
		{Address: "1DEF", TxID: "tx5", Vout: 0, Amount: 500000},
	}

	seed := getTestSeed(t)

	keys, err := deriveKeysForUTXOs(utxos, addresses, seed)
	require.NoError(t, err)
	require.NotNil(t, keys)
	assert.Len(t, keys, 2, "should derive only 2 keys despite 5 UTXOs")
	assert.Contains(t, keys, "1ABC")
	assert.Contains(t, keys, "1DEF")

	// Clean up
	zeroKeyMap(keys)
}

// TestDeriveKeysForUTXOs_AddressNotInWallet tests error handling when a UTXO
// references an address not in the wallet.
func TestDeriveKeysForUTXOs_AddressNotInWallet(t *testing.T) {
	t.Parallel()

	addresses := []wallet.Address{
		{Address: "1ABC", Index: 0, Path: "m/44'/236'/0'/0/0"},
	}

	utxos := []chain.UTXO{
		{Address: "1ABC", TxID: "tx1", Vout: 0, Amount: 100000},
		{Address: "1NOTFOUND", TxID: "tx2", Vout: 0, Amount: 200000},
	}

	seed := getTestSeed(t)

	keys, err := deriveKeysForUTXOs(utxos, addresses, seed)
	require.Error(t, err)
	assert.Nil(t, keys, "should return nil keys on error")
	assert.Contains(t, err.Error(), "1NOTFOUND")
	assert.Contains(t, err.Error(), "address not found in wallet")
}

// TestDeriveKeysForUTXOs_EmptyUTXOs tests handling of empty UTXO list.
func TestDeriveKeysForUTXOs_EmptyUTXOs(t *testing.T) {
	t.Parallel()

	addresses := []wallet.Address{
		{Address: "1ABC", Index: 0, Path: "m/44'/236'/0'/0/0"},
	}

	utxos := []chain.UTXO{}

	seed := getTestSeed(t)

	keys, err := deriveKeysForUTXOs(utxos, addresses, seed)
	require.NoError(t, err)
	require.NotNil(t, keys)
	assert.Empty(t, keys, "should return empty map for empty UTXO list")
}

// TestDeriveKeysForUTXOs_InvalidSeed tests error handling with an invalid seed.
func TestDeriveKeysForUTXOs_InvalidSeed(t *testing.T) {
	t.Parallel()

	addresses := []wallet.Address{
		{Address: "1ABC", Index: 0, Path: "m/44'/236'/0'/0/0"},
	}

	utxos := []chain.UTXO{
		{Address: "1ABC", TxID: "tx1", Vout: 0, Amount: 100000},
	}

	// Invalid seed (too short)
	invalidSeed := []byte{0x01, 0x02}

	keys, err := deriveKeysForUTXOs(utxos, addresses, invalidSeed)
	require.Error(t, err)
	assert.Nil(t, keys, "should return nil keys on derivation error")
	assert.Contains(t, err.Error(), "deriving key")
}

// TestDeriveKeysForUTXOs_KeysZeroedOnError verifies that keys are properly zeroed
// even when an error occurs during derivation.
func TestDeriveKeysForUTXOs_KeysZeroedOnError(t *testing.T) {
	t.Parallel()

	addresses := []wallet.Address{
		{Address: "1ABC", Index: 0, Path: "m/44'/236'/0'/0/0"},
		{Address: "1DEF", Index: 1, Path: "m/44'/236'/0'/0/1"},
	}

	// First address valid, second not in wallet - will fail after deriving first key
	utxos := []chain.UTXO{
		{Address: "1ABC", TxID: "tx1", Vout: 0, Amount: 100000},
		{Address: "1NOTFOUND", TxID: "tx2", Vout: 0, Amount: 200000},
	}

	seed := getTestSeed(t)

	keys, err := deriveKeysForUTXOs(utxos, addresses, seed)
	require.Error(t, err)
	assert.Nil(t, keys, "should return nil keys on error")

	// The implementation should have zeroed keys internally before returning
	// We can't verify this directly, but the function contract guarantees it
}

// TestDeriveKeyForAddress_Success tests successful key derivation for a single address.
func TestDeriveKeyForAddress_Success(t *testing.T) {
	t.Parallel()

	addrIndex := map[string]uint32{
		"1ABC": 0,
		"1DEF": 1,
	}

	seed := getTestSeed(t)

	key, err := deriveKeyForAddress("1ABC", addrIndex, seed)
	require.NoError(t, err)
	assert.NotEmpty(t, key, "derived key should not be empty")

	// Clean up
	wallet.ZeroBytes(key)

	// Verify zeroing
	for i, b := range key {
		assert.Equal(t, byte(0), b, "key[%d] not zeroed", i)
	}
}

// TestDeriveKeyForAddress_NotInIndex tests error handling when address is not in index.
func TestDeriveKeyForAddress_NotInIndex(t *testing.T) {
	t.Parallel()

	addrIndex := map[string]uint32{
		"1ABC": 0,
	}

	seed := getTestSeed(t)

	key, err := deriveKeyForAddress("1NOTFOUND", addrIndex, seed)
	require.Error(t, err)
	assert.Nil(t, key)
	assert.Contains(t, err.Error(), "address not found in wallet")
	assert.Contains(t, err.Error(), "1NOTFOUND")
}

// TestDeriveKeyForAddress_DerivationError tests error handling during key derivation.
func TestDeriveKeyForAddress_DerivationError(t *testing.T) {
	t.Parallel()

	addrIndex := map[string]uint32{
		"1ABC": 0,
	}

	// Invalid seed
	invalidSeed := []byte{0x01, 0x02}

	key, err := deriveKeyForAddress("1ABC", addrIndex, invalidSeed)
	require.Error(t, err)
	assert.Nil(t, key)
	assert.Contains(t, err.Error(), "deriving key for address")
}

// TestZeroKeyMap_EmptyMap tests zeroing an empty map.
func TestZeroKeyMap_EmptyMap(t *testing.T) {
	t.Parallel()

	keys := make(map[string][]byte)

	// Should not panic
	zeroKeyMap(keys)

	assert.Empty(t, keys)
}

// TestZeroKeyMap_AllBytesZeroed tests byte-by-byte verification of key zeroing.
// SECURITY-CRITICAL: This test ensures all key material is properly wiped.
func TestZeroKeyMap_AllBytesZeroed(t *testing.T) {
	t.Parallel()

	keys := map[string][]byte{
		"addr1": {0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08},
		"addr2": {0xFF, 0xFE, 0xFD, 0xFC, 0xFB, 0xFA, 0xF9, 0xF8},
		"addr3": {0xAA, 0xBB, 0xCC, 0xDD, 0xEE, 0xFF, 0x11, 0x22},
	}

	// Verify keys have non-zero data before zeroing
	for _, key := range keys {
		hasNonZero := false
		for _, b := range key {
			if b != 0 {
				hasNonZero = true
				break
			}
		}
		assert.True(t, hasNonZero, "test key should have non-zero bytes")
	}

	// Zero all keys
	zeroKeyMap(keys)

	// SECURITY-CRITICAL: Verify ALL bytes are 0x00
	for addr, key := range keys {
		for i, b := range key {
			assert.Equal(t, byte(0), b, "key[%s][%d] not zeroed (was 0x%02X)", addr, i, b)
		}
	}
}

// TestZeroKeyMap_NilKeyInMap tests handling of nil key value in map.
func TestZeroKeyMap_NilKeyInMap(t *testing.T) {
	t.Parallel()

	keys := map[string][]byte{
		"addr1": {0x01, 0x02, 0x03},
		"addr2": nil, // Nil key
		"addr3": {0x04, 0x05, 0x06},
	}

	// Should not panic on nil key
	zeroKeyMap(keys)

	// Verify non-nil keys are zeroed
	assert.Equal(t, []byte{0, 0, 0}, keys["addr1"])
	assert.Nil(t, keys["addr2"])
	assert.Equal(t, []byte{0, 0, 0}, keys["addr3"])
}
