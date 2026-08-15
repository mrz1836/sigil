package wallet

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestLoad_LegacyGoldenWallet guards that the EncryptBytes/DecryptBytes
// migration did not change the on-disk format: a wallet file created by the
// prior code path (age scrypt at logN=10; see testdata/legacy.wallet) must still
// decrypt. The wallet test binary pins the work factor to 10 (TestMain), which
// matches the golden.
func TestLoad_LegacyGoldenWallet(t *testing.T) {
	golden, err := os.ReadFile("testdata/legacy.wallet")
	require.NoError(t, err)

	dir := t.TempDir()
	//nolint:gosec // G703: test writes the golden into its own temp dir
	require.NoError(t, os.WriteFile(filepath.Join(dir, "legacy"+walletFileExtension), golden, 0o600))

	s := NewFileStorage(dir)
	w, seed, err := s.Load("legacy", []byte("legacy-wallet-pass"))
	require.NoError(t, err)
	assert.Equal(t, "legacy", w.Name)

	want := make([]byte, 64)
	for i := range want {
		want[i] = byte(i + 1)
	}
	assert.Equal(t, want, seed, "legacy golden seed must decrypt unchanged")

	// Wrong password still fails closed.
	_, _, err = s.Load("legacy", []byte("wrong-pass"))
	require.ErrorIs(t, err, ErrDecryptionFailed)
}
