package wallet

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDeriveAccountXpub(t *testing.T) {
	t.Parallel()
	seed := getTestSeed(t)

	xpub, err := DeriveAccountXpub(seed, ChainETH, 0)
	require.NoError(t, err)
	assert.True(t, strings.HasPrefix(xpub, "xpub"), "mainnet account xpub, got %q", xpub)

	// Derivation is deterministic.
	xpubAgain, err := DeriveAccountXpub(seed, ChainETH, 0)
	require.NoError(t, err)
	assert.Equal(t, xpub, xpubAgain)

	// A different account index yields a different xpub.
	xpubAcct1, err := DeriveAccountXpub(seed, ChainETH, 1)
	require.NoError(t, err)
	assert.NotEqual(t, xpub, xpubAcct1)
}

// TestDeriveAccountXpub_MatchesSeedDerivation verifies the account xpub can
// regenerate the exact same external address the seed derives directly — proving
// the neutered key path is consistent with full seed derivation.
func TestDeriveAccountXpub_MatchesSeedDerivation(t *testing.T) {
	t.Parallel()
	seed := getTestSeed(t)

	xpub, err := DeriveAccountXpub(seed, ChainETH, 0)
	require.NoError(t, err)

	fromXpub, err := DeriveAddressFromXpub(xpub, ChainETH, ExternalChain, 0)
	require.NoError(t, err)
	require.NotNil(t, fromXpub)

	fromSeed, err := DeriveAddress(seed, ChainETH, 0, 0)
	require.NoError(t, err)

	assert.Equal(t, fromSeed.Address, fromXpub.Address)
	assert.Equal(t, fromSeed.PublicKey, fromXpub.PublicKey)
}
