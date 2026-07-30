package wallet

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFileStorage_LoadMetadata(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	storage := NewFileStorage(tmpDir)

	w, err := NewWallet("meta", []ChainID{ChainETH, ChainBSV})
	require.NoError(t, err)
	seed := getTestSeed(t)
	require.NoError(t, w.DeriveAddresses(seed, 1))
	require.NoError(t, storage.Save(w, seed, []byte("password-123")))

	// LoadMetadata returns wallet info without requiring the decryption password.
	meta, err := storage.LoadMetadata("meta")
	require.NoError(t, err)
	assert.Equal(t, "meta", meta.Name)
	assert.Equal(t, w.EnabledChains, meta.EnabledChains)
	assert.Equal(t, w.Version, meta.Version)
	require.NotEmpty(t, meta.Addresses[ChainETH])
	assert.Equal(t, w.Addresses[ChainETH][0].Address, meta.Addresses[ChainETH][0].Address)
}

func TestFileStorage_LoadMetadata_Errors(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	storage := NewFileStorage(tmpDir)

	// Unknown wallet → ErrWalletNotFound.
	_, err := storage.LoadMetadata("nonexistent")
	require.ErrorIs(t, err, ErrWalletNotFound)

	// Invalid name is rejected before any filesystem access.
	_, err = storage.LoadMetadata("bad name!")
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrInvalidWalletName)
}
