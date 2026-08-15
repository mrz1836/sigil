package wallet

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// makeTestWallet writes a password-only wallet and returns its storage + name.
func makeTestWallet(t *testing.T) (*FileStorage, string) {
	t.Helper()
	dir := t.TempDir()
	s := NewFileStorage(dir)
	name := "envwallet"
	w := &Wallet{Name: name}
	seed := make([]byte, 64)
	for i := range seed {
		seed[i] = byte(i)
	}
	require.NoError(t, s.Save(w, seed, []byte("password")))
	return s, name
}

func TestEnvelope_LegacyWalletHasNoEnvelope(t *testing.T) {
	s, name := makeTestWallet(t)

	policy, hasEnv, err := s.LoadAuthPolicy(name)
	require.NoError(t, err)
	assert.False(t, hasEnv)
	assert.Empty(t, policy)

	_, _, err = s.LoadEnvelope(name)
	require.ErrorIs(t, err, ErrNoEnvelope)

	// The legacy password path still works.
	_, seed, err := s.Load(name, []byte("password"))
	require.NoError(t, err)
	assert.Len(t, seed, 64)
}

func TestEnvelope_EnrollClearsEncryptedSeed(t *testing.T) {
	s, name := makeTestWallet(t)

	envelope := []byte("marshaled-tumbler-envelope-bytes")
	require.NoError(t, s.EnrollEnvelope(name, envelope, "yubikey-only"))

	policy, hasEnv, err := s.LoadAuthPolicy(name)
	require.NoError(t, err)
	assert.True(t, hasEnv)
	assert.Equal(t, "yubikey-only", policy)

	gotEnv, gotPolicy, err := s.LoadEnvelope(name)
	require.NoError(t, err)
	assert.Equal(t, envelope, gotEnv)
	assert.Equal(t, "yubikey-only", gotPolicy)

	// The legacy password path must no longer work (EncryptedSeed cleared),
	// so a stolen file cannot be opened with the password alone.
	_, _, err = s.Load(name, []byte("password"))
	require.Error(t, err)

	// Metadata still loads.
	w, err := s.LoadMetadata(name)
	require.NoError(t, err)
	assert.Equal(t, name, w.Name)
}

func TestEnvelope_UpdateEnvelope(t *testing.T) {
	s, name := makeTestWallet(t)
	require.NoError(t, s.EnrollEnvelope(name, []byte("v1"), "yubikey-only"))
	require.NoError(t, s.UpdateEnvelope(name, []byte("v2-with-backup-slot")))

	gotEnv, _, err := s.LoadEnvelope(name)
	require.NoError(t, err)
	assert.Equal(t, []byte("v2-with-backup-slot"), gotEnv)
}

func TestEnvelope_UpdateWithoutEnrollFails(t *testing.T) {
	s, name := makeTestWallet(t)
	err := s.UpdateEnvelope(name, []byte("x"))
	assert.ErrorIs(t, err, ErrNoEnvelope)
}

func TestEnvelope_PersistsAcrossReload(t *testing.T) {
	s, name := makeTestWallet(t)
	require.NoError(t, s.EnrollEnvelope(name, []byte("persist-me"), "password-and-yubikey"))

	// Re-open the file with a fresh storage instance (reads from disk).
	env, policy, err := s.LoadEnvelope(name)
	require.NoError(t, err)
	assert.Equal(t, []byte("persist-me"), env)
	assert.Equal(t, "password-and-yubikey", policy)

	// The on-disk JSON must not contain a non-empty encrypted_seed value.
	//nolint:gosec // G304: test reads a wallet file it just wrote under a temp dir
	raw, err := os.ReadFile(filepath.Join(s.basePath, name+walletFileExtension))
	require.NoError(t, err)
	assert.NotContains(t, string(raw), "\"encrypted_seed\"")
}
