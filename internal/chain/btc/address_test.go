package btc

import (
	"encoding/hex"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mrz1836/sigil/internal/wallet/bitcoin"
)

// encodeAddr builds an address of the given type/network from the fixed known
// programs. It does not need a *testing.T, so it is reusable in fuzz seeding.
func encodeAddr(typ AddrType, network Network) (addr string, program []byte) {
	hash20, _ := hex.DecodeString(program20Hex)
	hash32, _ := hex.DecodeString(program32Hex)

	verP2PKH, verP2SH := byte(versionP2PKHMainnet), byte(versionP2SHMainnet)
	hrp := hrpMainnet
	if network == NetworkTestnet {
		verP2PKH, verP2SH = versionP2PKHTestnet, versionP2SHTestnet
		hrp = hrpTestnet
	}

	switch typ {
	case AddrP2PKH:
		return bitcoin.Base58CheckEncode(verP2PKH, hash20), hash20
	case AddrP2SH:
		return bitcoin.Base58CheckEncode(verP2SH, hash20), hash20
	case AddrP2WPKH:
		a, _ := bitcoin.SegwitEncode(hrp, 0, hash20)
		return a, hash20
	case AddrP2WSH:
		a, _ := bitcoin.SegwitEncode(hrp, 0, hash32)
		return a, hash32
	default:
		return "", nil
	}
}

func TestDecodeAddress_AllTypesBothNetworks(t *testing.T) {
	t.Parallel()

	types := []AddrType{AddrP2PKH, AddrP2SH, AddrP2WPKH, AddrP2WSH}
	networks := []Network{NetworkMainnet, NetworkTestnet}

	for _, network := range networks {
		for _, typ := range types {
			t.Run(string(network)+"/"+typ.String(), func(t *testing.T) {
				t.Parallel()
				addr, wantProgram := encodeAddr(typ, network)
				require.NotEmpty(t, addr)

				gotType, gotProgram, err := DecodeAddress(addr, network)
				require.NoError(t, err)
				assert.Equal(t, typ, gotType)
				assert.Equal(t, wantProgram, gotProgram)
			})
		}
	}
}

func TestDecodeAddress_CrossNetworkRejected(t *testing.T) {
	t.Parallel()

	for _, typ := range []AddrType{AddrP2PKH, AddrP2SH, AddrP2WPKH, AddrP2WSH} {
		t.Run("mainnet addr on testnet client: "+typ.String(), func(t *testing.T) {
			t.Parallel()
			addr, _ := encodeAddr(typ, NetworkMainnet)
			_, _, err := DecodeAddress(addr, NetworkTestnet)
			require.ErrorIs(t, err, ErrInvalidAddress)
		})
		t.Run("testnet addr on mainnet client: "+typ.String(), func(t *testing.T) {
			t.Parallel()
			addr, _ := encodeAddr(typ, NetworkTestnet)
			_, _, err := DecodeAddress(addr, NetworkMainnet)
			require.ErrorIs(t, err, ErrInvalidAddress)
		})
	}
}

func TestDecodeAddress_Rejects(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		addr string
	}{
		{"empty", ""},
		{"garbage", "not-an-address"},
		{"too short for bech32", "bc1"},
		{"bad base58 checksum", "1BvBMSEYstWetqTFn5Au4m4GFg7xJaNVN3"},
		{"bech32 bad checksum", "bc1qw508d6qejxtdg4y5r3zarvary0c5xw7kv8f3t5"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			_, _, err := DecodeAddress(tt.addr, NetworkMainnet)
			require.ErrorIs(t, err, ErrInvalidAddress)
		})
	}
}

func TestDecodeAddress_UppercaseBech32(t *testing.T) {
	t.Parallel()

	// BIP173 allows all-uppercase bech32; DecodeAddress must accept "BC1…".
	addr, program := encodeAddr(AddrP2WPKH, NetworkMainnet)
	typ, gotProgram, err := DecodeAddress(strings.ToUpper(addr), NetworkMainnet)
	require.NoError(t, err)
	assert.Equal(t, AddrP2WPKH, typ)
	assert.Equal(t, program, gotProgram)
}

func TestValidateBTCAddressForNetwork(t *testing.T) {
	t.Parallel()

	addr, _ := encodeAddr(AddrP2PKH, NetworkMainnet)
	require.NoError(t, ValidateBTCAddressForNetwork(addr, NetworkMainnet))
	require.ErrorIs(t, ValidateBTCAddressForNetwork(addr, NetworkTestnet), ErrInvalidAddress)
	require.ErrorIs(t, ValidateBTCAddressForNetwork("", NetworkMainnet), ErrInvalidAddress)
}

func TestAddressToScript_ByteLayouts(t *testing.T) {
	t.Parallel()

	tests := []struct {
		typ     AddrType
		network Network
		want    string
	}{
		{AddrP2PKH, NetworkMainnet, "76a914751e76e8199196d454941c45d1b3a323f1433bd688ac"},
		{AddrP2SH, NetworkMainnet, "a914751e76e8199196d454941c45d1b3a323f1433bd687"},
		{AddrP2WPKH, NetworkMainnet, "0014751e76e8199196d454941c45d1b3a323f1433bd6"},
		{AddrP2WSH, NetworkMainnet, "00201863143c14c5166804bd19203356da136c985678cd4d27a1b8c6329604903262"},
		{AddrP2PKH, NetworkTestnet, "76a914751e76e8199196d454941c45d1b3a323f1433bd688ac"},
		{AddrP2WPKH, NetworkTestnet, "0014751e76e8199196d454941c45d1b3a323f1433bd6"},
	}
	for _, tt := range tests {
		t.Run(tt.typ.String()+"/"+string(tt.network), func(t *testing.T) {
			t.Parallel()
			addr, _ := encodeAddr(tt.typ, tt.network)
			s, err := AddressToScript(addr, tt.network)
			require.NoError(t, err)
			assert.Equal(t, tt.want, hex.EncodeToString(*s))
		})
	}
}

func TestAddressToScript_InvalidRejected(t *testing.T) {
	t.Parallel()
	_, err := AddressToScript("not-an-address", NetworkMainnet)
	require.ErrorIs(t, err, ErrInvalidAddress)
}

func FuzzDecodeAddress(f *testing.F) {
	mainP2PKH, _ := encodeAddr(AddrP2PKH, NetworkMainnet)
	mainSeg, _ := encodeAddr(AddrP2WPKH, NetworkMainnet)
	testP2PKH, _ := encodeAddr(AddrP2PKH, NetworkTestnet)
	f.Add(mainP2PKH, "main")
	f.Add(mainSeg, "main")
	f.Add(testP2PKH, "test")
	f.Add("bc1qw508d6qejxtdg4y5r3zarvary0c5xw7kv8f3t4", "test") // cross-network
	f.Add("", "main")

	f.Fuzz(func(t *testing.T, addr, netStr string) {
		network := NetworkMainnet
		if netStr == "test" {
			network = NetworkTestnet
		}
		// Must never panic.
		typ, program, err := DecodeAddress(addr, network)
		if err != nil {
			return
		}
		// On success: canonical program length and AddressToScript must succeed.
		require.True(t, len(program) == 20 || len(program) == 32)
		require.Contains(t, []AddrType{AddrP2PKH, AddrP2SH, AddrP2WPKH, AddrP2WSH}, typ)
		_, sErr := AddressToScript(addr, network)
		require.NoError(t, sErr)
	})
}
