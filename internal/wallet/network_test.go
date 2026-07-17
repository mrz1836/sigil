package wallet

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNetworkFromStringVariants(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input string
		want  Network
	}{
		{"empty defaults mainnet", "", Mainnet},
		{"main", "main", Mainnet},
		{"test", "test", Testnet},
		{"testnet alias", "testnet", Testnet},
		{"unknown defaults mainnet", "garbage", Mainnet},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tc.want, NetworkFromString(tc.input))
		})
	}
}

func TestNetworkStringAndIsTestnet(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "main", Mainnet.String())
	assert.Equal(t, "test", Testnet.String())
	assert.False(t, Mainnet.IsTestnet())
	assert.True(t, Testnet.IsTestnet())
}

func TestNetworkVersionBytes(t *testing.T) {
	t.Parallel()

	// Mainnet version bytes.
	assert.Equal(t, byte(0x00), Mainnet.P2PKHVersion())
	assert.Equal(t, byte(0x05), Mainnet.P2SHVersion())
	assert.Equal(t, byte(0x80), Mainnet.WIFVersion())
	assert.Equal(t, [4]byte{0x04, 0x88, 0xAD, 0xE4}, Mainnet.HDPrivVersion())
	assert.Equal(t, [4]byte{0x04, 0x88, 0xB2, 0x1E}, Mainnet.HDPubVersion())

	// Testnet version bytes.
	assert.Equal(t, byte(0x6f), Testnet.P2PKHVersion())
	assert.Equal(t, byte(0xc4), Testnet.P2SHVersion())
	assert.Equal(t, byte(0xef), Testnet.WIFVersion())
	assert.Equal(t, [4]byte{0x04, 0x35, 0x83, 0x94}, Testnet.HDPrivVersion())
	assert.Equal(t, [4]byte{0x04, 0x35, 0x87, 0xCF}, Testnet.HDPubVersion())
}

// TestZeroValueIsMainnet documents the load-bearing invariant that the zero
// value of Network is Mainnet, so defaulted call sites keep mainnet behavior.
func TestZeroValueIsMainnet(t *testing.T) {
	t.Parallel()

	var n Network
	assert.Equal(t, Mainnet, n)
	assert.Equal(t, byte(0x00), n.P2PKHVersion())
}
