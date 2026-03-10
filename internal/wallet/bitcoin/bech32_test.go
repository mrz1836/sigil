package bitcoin

import (
	"encoding/hex"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConvertBits_8to5(t *testing.T) {
	t.Parallel()
	data := []byte{0x00, 0x14}
	result, err := ConvertBits(data, 8, 5, true)
	require.NoError(t, err)
	assert.NotEmpty(t, result)
}

func TestConvertBits_RoundTrip(t *testing.T) {
	t.Parallel()
	original := []byte{0xab, 0xcd, 0xef, 0x01, 0x23}
	fiveBit, err := ConvertBits(original, 8, 5, true)
	require.NoError(t, err)

	eightBit, err := ConvertBits(fiveBit, 5, 8, false)
	require.NoError(t, err)
	assert.Equal(t, original, eightBit)
}

func TestConvertBits_InvalidValue(t *testing.T) {
	t.Parallel()
	// Value 32 is out of range for 5-bit input
	_, err := ConvertBits([]byte{32}, 5, 8, true)
	assert.Error(t, err)
}

func TestBech32Encode_Basic(t *testing.T) {
	t.Parallel()
	// Encode with a simple HRP and data
	data := []byte{0, 1, 2, 3, 4, 5}
	result, err := Bech32Encode("test", data)
	require.NoError(t, err)
	assert.NotEmpty(t, result)
	assert.Contains(t, result, "test1")
}

func TestBech32Encode_InvalidData(t *testing.T) {
	t.Parallel()
	// Data value >= 32 is invalid for 5-bit encoding
	_, err := Bech32Encode("test", []byte{32})
	assert.Error(t, err)
}

func TestSegwitEncode_BIP173_Vectors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		hrp     string
		version byte
		program string // hex
		want    string
	}{
		{
			name:    "BTC P2WPKH v0 generator point",
			hrp:     "bc",
			version: 0,
			program: "751e76e8199196d454941c45d1b3a323f1433bd6",
			want:    "bc1qw508d6qejxtdg4y5r3zarvary0c5xw7kv8f3t4",
		},
		{
			name:    "BTC P2WSH v0",
			hrp:     "bc",
			version: 0,
			program: "1863143c14c5166804bd19203356da136c985678cd4d27a1b8c6329604903262",
			want:    "bc1qrp33g0q5c5txsp9arysrx4k6zdkfs4nce4xj0gdcccefvpysxf3qccfmv3",
		},
		{
			name:    "Testnet P2WPKH v0",
			hrp:     "tb",
			version: 0,
			program: "751e76e8199196d454941c45d1b3a323f1433bd6",
			want:    "tb1qw508d6qejxtdg4y5r3zarvary0c5xw7kxpjzsx",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			program, err := hex.DecodeString(tt.program)
			require.NoError(t, err)
			got, err := SegwitEncode(tt.hrp, tt.version, program)
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestSegwitEncode_LTC(t *testing.T) {
	t.Parallel()

	program, err := hex.DecodeString("751e76e8199196d454941c45d1b3a323f1433bd6")
	require.NoError(t, err)
	got, err := SegwitEncode("ltc", 0, program)
	require.NoError(t, err)

	assert.NotEmpty(t, got)
	assert.Equal(t, "ltc1", got[:4], "should start with ltc1")
}

func TestSegwitEncode_InvalidVersion(t *testing.T) {
	t.Parallel()
	program, _ := hex.DecodeString("751e76e8199196d454941c45d1b3a323f1433bd6")
	_, err := SegwitEncode("bc", 17, program)
	assert.Error(t, err)
}

func TestSegwitEncode_InvalidProgramLength(t *testing.T) {
	t.Parallel()
	// Too short
	_, err := SegwitEncode("bc", 0, []byte{0x01})
	require.Error(t, err)

	// Invalid for v0 (must be 20 or 32 bytes)
	_, err = SegwitEncode("bc", 0, make([]byte, 25))
	require.Error(t, err)
}

func TestSegwitEncode_P2WPKH_KnownVector(t *testing.T) {
	t.Parallel()

	// From BIP173: key 0279BE667EF9DCBBAC55A06295CE870B07029BFCDB2DCE28D959F2815B16F81798
	// Hash160 = 751e76e8199196d454941c45d1b3a323f1433bd6
	// Expected: bc1qw508d6qejxtdg4y5r3zarvary0c5xw7kv8f3t4
	pubKeyHash, _ := hex.DecodeString("751e76e8199196d454941c45d1b3a323f1433bd6")
	addr, err := SegwitEncode("bc", 0, pubKeyHash)
	require.NoError(t, err)
	assert.Equal(t, "bc1qw508d6qejxtdg4y5r3zarvary0c5xw7kv8f3t4", addr)
}
