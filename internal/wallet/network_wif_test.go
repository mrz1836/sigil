package wallet

import (
	"encoding/hex"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Canonical WIF vectors for private key 0x…0001 (independently verified).
const (
	mainWIFCompressed   = "KwDiBf89QgGbjEhKnhXJuH7LrciVrZi3qYjgd9M7rFU73sVHnoWn"
	mainWIFUncompressed = "5HpHagT65TZzG1PH3CSu63k8DbpvD8s5ip4nEB3kEsreAnchuDf"
	testWIFCompressed   = "cMahea7zqjxrtgAbB7LSGbcQUr1uX1ojuat9jZodMN87JcbXMTcA"
	testWIFUncompressed = "91avARGdfge8E4tZfYLoxeJ5sGBdNJQH4kvjJoQFacbgwmaKkrx"
)

func TestParseWIFAcceptsBothNetworks(t *testing.T) {
	t.Parallel()

	wantKey := hex.EncodeToString(privKeyOne())

	tests := []struct {
		name string
		wif  string
	}{
		{"mainnet compressed", mainWIFCompressed},
		{"mainnet uncompressed", mainWIFUncompressed},
		{"testnet compressed", testWIFCompressed},
		{"testnet uncompressed", testWIFUncompressed},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			key, err := ParseWIF(tc.wif)
			require.NoError(t, err)
			require.Len(t, key, 32)
			assert.Equal(t, wantKey, hex.EncodeToString(key), "WIF must decode to the network-independent private key")
		})
	}
}

func TestIsWIFFormatDetectsTestnetPrefixes(t *testing.T) {
	t.Parallel()

	assert.True(t, isWIFFormat(mainWIFCompressed))   // K/L
	assert.True(t, isWIFFormat(mainWIFUncompressed)) // 5
	assert.True(t, isWIFFormat(testWIFCompressed))   // c
	assert.True(t, isWIFFormat(testWIFUncompressed)) // 9
	assert.False(t, isWIFFormat("not-a-wif"))
	assert.False(t, isWIFFormat(""))
}

func TestBSVTestnetParams(t *testing.T) {
	t.Parallel()

	p := BSVTestnetParams()
	assert.Equal(t, byte(0x6f), p.P2PKHVersion)
	assert.Equal(t, byte(0xc4), p.P2SHVersion)
	assert.Empty(t, p.Bech32HRP)
	assert.Empty(t, p.CashAddrPrefix)
}

func TestNetworkParamsForCoinTypeAndNetwork(t *testing.T) {
	t.Parallel()

	// Mainnet BSV/BTC keep their mainnet version bytes.
	main := NetworkParamsForCoinTypeAndNetwork(236, Mainnet)
	assert.Equal(t, byte(0x00), main.P2PKHVersion)
	assert.Equal(t, byte(0x05), main.P2SHVersion)

	// Testnet BSV/BTC use the Bitcoin testnet version bytes.
	for _, coinType := range []uint32{0, 236} {
		p := NetworkParamsForCoinTypeAndNetwork(coinType, Testnet)
		assert.Equal(t, byte(0x6f), p.P2PKHVersion)
		assert.Equal(t, byte(0xc4), p.P2SHVersion)
	}

	// The mainnet-only wrapper is unchanged.
	assert.Equal(t, NetworkParamsForCoinType(236), NetworkParamsForCoinTypeAndNetwork(236, Mainnet))
}

func TestAllAddressesFromPrivKeyTestnet(t *testing.T) {
	t.Parallel()

	addrs, err := AllAddressesFromPrivKey(privKeyOne(), BSVTestnetParams())
	require.NoError(t, err)
	assert.Equal(t, "mrCDrCybB6J1vRfbwM5hemdJz73FwDBC8r", addrs.P2PKH)
}
