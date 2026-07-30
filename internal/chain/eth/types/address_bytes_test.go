package ethtypes

import (
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
	// Bytes returns the raw 20-byte value; Hex re-encodes the same bytes.
	assert.Equal(t, "0x742d35cc6634c0532925a3b844bc454e4438f44e", addr.Hex())
	assert.Equal(t, addr[:], b)
}
