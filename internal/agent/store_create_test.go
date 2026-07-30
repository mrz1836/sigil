package agent

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestFileStore_Create_ReturnsUseCreateCredential verifies the anonymous-interface
// Create shim always redirects callers to CreateCredential (it exists only to avoid
// an import cycle and must never persist anything itself).
func TestFileStore_Create_ReturnsUseCreateCredential(t *testing.T) {
	t.Parallel()

	store := NewFileStore(t.TempDir())
	cred, err := store.Create("wallet", []byte("seed"), "agent-id", Policy{}, "token", nil, nil)

	require.ErrorIs(t, err, ErrUseCreateCredential)
	assert.Nil(t, cred)
}
