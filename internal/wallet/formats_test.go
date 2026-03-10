package wallet

import (
	"encoding/hex"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Test key: the generator point private key (0x01)
// Compressed public key: 0279BE667EF9DCBBAC55A06295CE870B07029BFCDB2DCE28D959F2815B16F81798
// Hash160: 751e76e8199196d454941c45d1b3a323f1433bd6
// P2PKH (BTC): 1BgGZ9tcN4rm9KBzDn7KprQz87SZ26SAMH
// Bech32 (BTC): bc1qw508d6qejxtdg4y5r3zarvary0c5xw7kv8f3t4

func TestAllAddressesFromPubKey_BTCMainnet(t *testing.T) {
	t.Parallel()

	pubKey, err := hex.DecodeString("0279BE667EF9DCBBAC55A06295CE870B07029BFCDB2DCE28D959F2815B16F81798")
	require.NoError(t, err)

	addrs, err := AllAddressesFromPubKey(pubKey, BTCMainnetParams())
	require.NoError(t, err)

	assert.Equal(t, "1BgGZ9tcN4rm9KBzDn7KprQz87SZ26SAMH", addrs.P2PKH)
	assert.Equal(t, "bc1qw508d6qejxtdg4y5r3zarvary0c5xw7kv8f3t4", addrs.Bech32)
	assert.NotEmpty(t, addrs.P2SH, "P2SH should be derived for BTC")
	// P2SH-P2WPKH for generator point pubkey
	assert.Equal(t, "3JvL6Ymt8MVWiCNHC7oWU6nLeHNJKLZGLN", addrs.P2SH)
	assert.Empty(t, addrs.CashAddr, "BTC should not have CashAddr")
}

func TestAllAddressesFromPubKey_BTCMainnet_KnownWIF(t *testing.T) {
	t.Parallel()

	// Known WIF: 5HueCGU8rMjxEXxiPuD5BDku4MkFqeZyd4dZ1jvhTVqvbTLvyTJ
	// Private key hex: 0c28fca386c7a227600b2fe50b7cae11ec86d3bf1fbe471be89827e19d72aa1d
	// Compressed P2PKH: 1LoVGDgRs9hTfTNJNuXKSpywcbdvwRXpmK
	privKey, err := hex.DecodeString("0c28fca386c7a227600b2fe50b7cae11ec86d3bf1fbe471be89827e19d72aa1d")
	require.NoError(t, err)

	addrs, err := AllAddressesFromPrivKey(privKey, BTCMainnetParams())
	require.NoError(t, err)

	assert.Equal(t, "1LoVGDgRs9hTfTNJNuXKSpywcbdvwRXpmK", addrs.P2PKH)
	assert.NotEmpty(t, addrs.P2SH)
	assert.NotEmpty(t, addrs.Bech32)
	assert.Empty(t, addrs.CashAddr)

	// Verify bech32 starts with bc1q
	require.Greater(t, len(addrs.Bech32), 4)
	assert.Equal(t, "bc1q", addrs.Bech32[:4], "BTC bech32 should start with bc1q")

	// Verify P2SH starts with 3
	assert.Equal(t, byte('3'), addrs.P2SH[0], "BTC P2SH should start with 3")
}

func TestAllAddressesFromPubKey_LTCMainnet(t *testing.T) {
	t.Parallel()

	pubKey, err := hex.DecodeString("0279BE667EF9DCBBAC55A06295CE870B07029BFCDB2DCE28D959F2815B16F81798")
	require.NoError(t, err)

	addrs, err := AllAddressesFromPubKey(pubKey, LTCMainnetParams())
	require.NoError(t, err)

	// LTC P2PKH starts with L (version byte 0x30)
	assert.Equal(t, byte('L'), addrs.P2PKH[0], "LTC P2PKH should start with L")

	// LTC P2SH starts with M (version byte 0x32)
	assert.Equal(t, byte('M'), addrs.P2SH[0], "LTC P2SH should start with M")

	// LTC bech32 starts with ltc1q
	require.Greater(t, len(addrs.Bech32), 5)
	assert.Equal(t, "ltc1q", addrs.Bech32[:5], "LTC bech32 should start with ltc1q")

	// No CashAddr for LTC
	assert.Empty(t, addrs.CashAddr)
}

func TestAllAddressesFromPubKey_BCHMainnet(t *testing.T) {
	t.Parallel()

	pubKey, err := hex.DecodeString("0279BE667EF9DCBBAC55A06295CE870B07029BFCDB2DCE28D959F2815B16F81798")
	require.NoError(t, err)

	addrs, err := AllAddressesFromPubKey(pubKey, BCHMainnetParams())
	require.NoError(t, err)

	// BCH P2PKH is same as BTC P2PKH (same version byte 0x00)
	assert.Equal(t, "1BgGZ9tcN4rm9KBzDn7KprQz87SZ26SAMH", addrs.P2PKH)

	// BCH has CashAddr starting with q (P2PKH)
	assert.NotEmpty(t, addrs.CashAddr)
	assert.Equal(t, byte('q'), addrs.CashAddr[0], "BCH cashaddr should start with q")

	// No segwit for BCH
	assert.Empty(t, addrs.P2SH)
	assert.Empty(t, addrs.Bech32)
}

func TestAllAddressesFromPrivKey_BTCMainnet(t *testing.T) {
	t.Parallel()

	// Generator point private key (0x01)
	privKey := make([]byte, 32)
	privKey[31] = 0x01

	addrs, err := AllAddressesFromPrivKey(privKey, BTCMainnetParams())
	require.NoError(t, err)

	assert.Equal(t, "1BgGZ9tcN4rm9KBzDn7KprQz87SZ26SAMH", addrs.P2PKH)
	assert.Equal(t, "bc1qw508d6qejxtdg4y5r3zarvary0c5xw7kv8f3t4", addrs.Bech32)
	assert.Equal(t, "3JvL6Ymt8MVWiCNHC7oWU6nLeHNJKLZGLN", addrs.P2SH)
}

func TestAllAddressesFromPrivKey_InvalidLength(t *testing.T) {
	t.Parallel()

	_, err := AllAddressesFromPrivKey(make([]byte, 16), BTCMainnetParams())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid private key length")
}

func TestAllAddressesFromPubKey_InvalidLength(t *testing.T) {
	t.Parallel()

	_, err := AllAddressesFromPubKey(make([]byte, 32), BTCMainnetParams())
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrInvalidPubKeyLen)
}

func TestDerivedAddresses_Addresses(t *testing.T) {
	t.Parallel()

	pubKey, _ := hex.DecodeString("0279BE667EF9DCBBAC55A06295CE870B07029BFCDB2DCE28D959F2815B16F81798")

	// BTC: should have 3 addresses (P2PKH, P2SH, Bech32)
	btcAddrs, err := AllAddressesFromPubKey(pubKey, BTCMainnetParams())
	require.NoError(t, err)
	assert.Len(t, btcAddrs.Addresses(), 3)

	// BCH: should have 2 addresses (P2PKH, CashAddr)
	bchAddrs, err := AllAddressesFromPubKey(pubKey, BCHMainnetParams())
	require.NoError(t, err)
	assert.Len(t, bchAddrs.Addresses(), 2)

	// LTC: should have 3 addresses (P2PKH, P2SH, Bech32)
	ltcAddrs, err := AllAddressesFromPubKey(pubKey, LTCMainnetParams())
	require.NoError(t, err)
	assert.Len(t, ltcAddrs.Addresses(), 3)

	// DOGE: should have 1 address (P2PKH only)
	dogeAddrs, err := AllAddressesFromPubKey(pubKey, DOGEMainnetParams())
	require.NoError(t, err)
	assert.Len(t, dogeAddrs.Addresses(), 1)
}

func TestDerivedAddresses_FormatLabel(t *testing.T) {
	t.Parallel()

	pubKey, _ := hex.DecodeString("0279BE667EF9DCBBAC55A06295CE870B07029BFCDB2DCE28D959F2815B16F81798")

	addrs, err := AllAddressesFromPubKey(pubKey, BTCMainnetParams())
	require.NoError(t, err)

	assert.Equal(t, "P2PKH", addrs.FormatLabel(addrs.P2PKH))
	assert.Equal(t, "P2SH-P2WPKH", addrs.FormatLabel(addrs.P2SH))
	assert.Equal(t, "Bech32", addrs.FormatLabel(addrs.Bech32))
	assert.Empty(t, addrs.FormatLabel("unknown"))
}

func TestAllAddressesFromPrivKey_ConsistencyWithExisting(t *testing.T) {
	t.Parallel()

	// Verify that the P2PKH address from AllAddressesFromPrivKey matches
	// the existing AddressFromPrivKeyBytes function
	privKey, _ := hex.DecodeString("0c28fca386c7a227600b2fe50b7cae11ec86d3bf1fbe471be89827e19d72aa1d")

	// Existing function
	existingAddr, err := AddressFromPrivKeyBytes(privKey)
	require.NoError(t, err)

	// New function
	newAddrs, err := AllAddressesFromPrivKey(privKey, BTCMainnetParams())
	require.NoError(t, err)

	assert.Equal(t, existingAddr, newAddrs.P2PKH,
		"P2PKH from AllAddressesFromPrivKey should match AddressFromPrivKeyBytes")
}

func TestAllAddressesFromPrivKey_MultipleKeys(t *testing.T) {
	t.Parallel()

	keys := []struct {
		name string
		hex  string
	}{
		{"key1", "0000000000000000000000000000000000000000000000000000000000000001"},
		{"key2", "0c28fca386c7a227600b2fe50b7cae11ec86d3bf1fbe471be89827e19d72aa1d"},
		{"key3", "e8f32e723decf4051aefac8e2c93c9c5b214313817cdb01a1494b917c8436b35"},
	}

	for _, kk := range keys {
		t.Run(kk.name, func(t *testing.T) {
			t.Parallel()
			privKey, err := hex.DecodeString(kk.hex)
			require.NoError(t, err)

			// All networks should produce valid results
			for _, params := range []struct {
				name string
				p    NetworkParams
			}{
				{"BTC", BTCMainnetParams()},
				{"LTC", LTCMainnetParams()},
				{"BCH", BCHMainnetParams()},
				{"DOGE", DOGEMainnetParams()},
			} {
				addrs, err := AllAddressesFromPrivKey(privKey, params.p)
				require.NoError(t, err, "%s/%s", kk.name, params.name)
				assert.NotEmpty(t, addrs.P2PKH, "%s/%s P2PKH", kk.name, params.name)
			}
		})
	}
}

func TestBTCMainnetParams(t *testing.T) {
	t.Parallel()
	p := BTCMainnetParams()
	assert.Equal(t, byte(0x00), p.P2PKHVersion)
	assert.Equal(t, byte(0x05), p.P2SHVersion)
	assert.Equal(t, "bc", p.Bech32HRP)
	assert.Empty(t, p.CashAddrPrefix)
}

func TestLTCMainnetParams(t *testing.T) {
	t.Parallel()
	p := LTCMainnetParams()
	assert.Equal(t, byte(0x30), p.P2PKHVersion)
	assert.Equal(t, byte(0x32), p.P2SHVersion)
	assert.Equal(t, "ltc", p.Bech32HRP)
	assert.Empty(t, p.CashAddrPrefix)
}

func TestBCHMainnetParams(t *testing.T) {
	t.Parallel()
	p := BCHMainnetParams()
	assert.Equal(t, byte(0x00), p.P2PKHVersion)
	assert.Equal(t, byte(0x05), p.P2SHVersion)
	assert.Empty(t, p.Bech32HRP)
	assert.Equal(t, "bitcoincash", p.CashAddrPrefix)
}

func TestDOGEMainnetParams(t *testing.T) {
	t.Parallel()
	p := DOGEMainnetParams()
	assert.Equal(t, byte(0x1E), p.P2PKHVersion)
	assert.Equal(t, byte(0x16), p.P2SHVersion)
	assert.Empty(t, p.Bech32HRP)
	assert.Empty(t, p.CashAddrPrefix)
}

func TestAllAddressesFromPubKey_DOGEMainnet(t *testing.T) {
	t.Parallel()

	pubKey, err := hex.DecodeString("0279BE667EF9DCBBAC55A06295CE870B07029BFCDB2DCE28D959F2815B16F81798")
	require.NoError(t, err)

	addrs, err := AllAddressesFromPubKey(pubKey, DOGEMainnetParams())
	require.NoError(t, err)

	// DOGE P2PKH starts with D (version byte 0x1E)
	assert.Equal(t, byte('D'), addrs.P2PKH[0], "DOGE P2PKH should start with D")

	// DOGE has no SegWit or CashAddr
	assert.Empty(t, addrs.P2SH)
	assert.Empty(t, addrs.Bech32)
	assert.Empty(t, addrs.CashAddr)
}
