package ethcrypto

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAddress_Bytes(t *testing.T) {
	t.Parallel()

	addr, err := HexToAddress("0x742d35Cc6634C0532925a3b844Bc454e4438f44e")
	require.NoError(t, err)

	b := addr.Bytes()
	require.Len(t, b, AddressLength)
	// The bytes must match the decoded hex.
	assert.Equal(t, "742d35cc6634c0532925a3b844bc454e4438f44e",
		hexString(b))
}

func TestAddress_String_IsEIP55Checksummed(t *testing.T) {
	t.Parallel()

	// A canonical EIP-55 vector round-trips through String() to its checksummed form.
	addr, err := HexToAddress("0x5aAeb6053F3E94C9b9A09f33669435E7Ef1BeAed")
	require.NoError(t, err)
	assert.Equal(t, "0x5aAeb6053F3E94C9b9A09f33669435E7Ef1BeAed", addr.String())
	// String() is defined as the checksum of Hex().
	assert.Equal(t, ToChecksumAddress(addr.Hex()), addr.String())
}

func TestBytesToAddress_PadAndTruncate(t *testing.T) {
	t.Parallel()

	t.Run("short slice is left-padded with zeros", func(t *testing.T) {
		t.Parallel()
		a := BytesToAddress([]byte{0x01, 0x02})
		b := a.Bytes()
		require.Len(t, b, AddressLength)
		// Value lands in the low-order bytes; the rest is zero.
		assert.Equal(t, byte(0x01), b[AddressLength-2])
		assert.Equal(t, byte(0x02), b[AddressLength-1])
		assert.True(t, bytes.Equal(make([]byte, AddressLength-2), b[:AddressLength-2]))
	})

	t.Run("long slice keeps the last 20 bytes", func(t *testing.T) {
		t.Parallel()
		in := make([]byte, AddressLength+4)
		for i := range in {
			in[i] = byte(i)
		}
		a := BytesToAddress(in)
		assert.Equal(t, in[len(in)-AddressLength:], a.Bytes())
	})
}

// hexString is a tiny local helper to avoid importing encoding/hex noise into assertions.
func hexString(b []byte) string {
	const hexdigits = "0123456789abcdef"
	out := make([]byte, len(b)*2)
	for i, c := range b {
		out[i*2] = hexdigits[c>>4]
		out[i*2+1] = hexdigits[c&0x0F]
	}
	return string(out)
}
