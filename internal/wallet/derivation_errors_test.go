package wallet

import (
	"encoding/hex"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestDeriveAddressUnsupportedChain verifies an unknown chain ID reaches the
// ErrUnsupportedChain guard rather than silently deriving an address.
func TestDeriveAddressUnsupportedChain(t *testing.T) {
	seed := getTestSeed(t)
	addr, err := DeriveAddress(seed, ChainID("dogecoin-xyz"), 0, 0)
	require.ErrorIs(t, err, ErrUnsupportedChain)
	require.Nil(t, addr)
}

// TestDecompressPublicKey covers the invalid-input branch and a valid round trip.
func TestDecompressPublicKey(t *testing.T) {
	t.Run("valid compressed key decompresses", func(t *testing.T) {
		// secp256k1 generator point G in compressed form.
		compressed, err := hex.DecodeString("0279be667ef9dcbbac55a06295ce870b07029bfcdb2dce28d959f2815b16f81798")
		require.NoError(t, err)

		uncompressed, err := decompressPublicKey(compressed)
		require.NoError(t, err)
		require.Len(t, uncompressed, 65)
		require.Equal(t, byte(0x04), uncompressed[0])
	})

	invalid := map[string][]byte{
		"empty":               {},
		"too short":           {0x02, 0xaa},
		"invalid format byte": make([]byte, 33), // leading 0x00 is not a valid prefix
		"wrong length":        make([]byte, 34),
	}
	for name, input := range invalid {
		t.Run("invalid: "+name, func(t *testing.T) {
			out, err := decompressPublicKey(input)
			require.Error(t, err)
			require.Nil(t, out)
		})
	}
}

// TestToChecksumAddressLengthGuard verifies the length guard rejects malformed
// byte slices and a valid 20-byte address encodes to a 42-char 0x string.
func TestToChecksumAddressLengthGuard(t *testing.T) {
	for _, n := range []int{0, 19, 21, 32} {
		out, err := toChecksumAddress(make([]byte, n))
		require.ErrorIs(t, err, ErrInvalidAddressLength)
		require.Empty(t, out)
	}

	out, err := toChecksumAddress(make([]byte, 20))
	require.NoError(t, err)
	require.Len(t, out, 42)
	require.True(t, strings.HasPrefix(out, "0x"))
}
