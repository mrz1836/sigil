package bitcoin

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBase58Error_Error(t *testing.T) {
	t.Parallel()

	// The sentinel's Error() string is stable and descriptive.
	assert.Equal(t, "invalid Base58 encoding", ErrInvalidBase58.Error())

	// It is returned (and thus its Error() is reachable) for empty input.
	_, err := Base58Decode("")
	require.ErrorIs(t, err, ErrInvalidBase58)
	assert.Equal(t, "invalid Base58 encoding", err.Error())
}
