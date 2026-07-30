package wallet

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHasValidEIP55Checksum(t *testing.T) {
	t.Parallel()

	// Canonical mixed-case checksummed addresses from the EIP-55 specification.
	valid := []string{
		"0x5aAeb6053F3E94C9b9A09f33669435E7Ef1BeAed",
		"0xfB6916095ca1df60bB79Ce92cE3Ea74c37c5d359",
		"0xdbF03B407c01E7cD3CBea99509d93f8DDDC8C6FB",
		"0xD1220A0cf47c7B9Be7A2E6BA89F429762e7b9aDb",
	}
	for _, addr := range valid {
		assert.True(t, HasValidEIP55Checksum(addr), "expected valid checksum: %s", addr)
	}

	// Same bytes, all-lowercase: the canonical form has uppercase letters, so the
	// downcased variant must fail the checksum.
	assert.False(t, HasValidEIP55Checksum("0x5aaeb6053f3e94c9b9a09f33669435e7ef1beaed"))
	// All-uppercase variant likewise fails.
	assert.False(t, HasValidEIP55Checksum("0x5AAEB6053F3E94C9B9A09F33669435E7EF1BEAED"))

	// Structurally invalid inputs are rejected before the checksum step.
	assert.False(t, HasValidEIP55Checksum("not-an-address"))
	assert.False(t, HasValidEIP55Checksum("0x123"))
	assert.False(t, HasValidEIP55Checksum(""))
}

func TestDerivePrivateKeyForChain(t *testing.T) {
	t.Parallel()
	seed := getTestSeed(t)

	key0, err := DerivePrivateKeyForChain(seed, ChainETH, 0)
	require.NoError(t, err)
	defer ZeroBytes(key0)
	assert.Len(t, key0, 32)

	// It is a thin wrapper over DerivePrivateKey with account 0.
	direct, err := DerivePrivateKey(seed, ChainETH, 0, 0)
	require.NoError(t, err)
	defer ZeroBytes(direct)
	assert.Equal(t, direct, key0)

	// Distinct indices produce distinct keys.
	key1, err := DerivePrivateKeyForChain(seed, ChainETH, 1)
	require.NoError(t, err)
	defer ZeroBytes(key1)
	assert.NotEqual(t, key0, key1)

	// Distinct chains (different coin types) produce distinct keys at the same index.
	bsvKey, err := DerivePrivateKeyForChain(seed, ChainBSV, 0)
	require.NoError(t, err)
	defer ZeroBytes(bsvKey)
	assert.NotEqual(t, key0, bsvKey)
}
