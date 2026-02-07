package fileutil

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWriteAtomic_Success(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	target := filepath.Join(tmpDir, "state.json")

	require.NoError(t, os.WriteFile(target, []byte("old"), 0o644)) //nolint:gosec // G306: Test file, relaxed perms OK
	require.NoError(t, WriteAtomic(target, []byte("new"), 0o600))

	data, err := os.ReadFile(target) //nolint:gosec // G304: Test path from t.TempDir()
	require.NoError(t, err)
	assert.Equal(t, "new", string(data))

	info, err := os.Stat(target)
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0o600), info.Mode().Perm())
}

func TestWriteAtomic_FailureLeavesOriginalFile(t *testing.T) {
	tmpDir := t.TempDir()
	target := filepath.Join(tmpDir, "state.json")
	require.NoError(t, os.WriteFile(target, []byte("original"), 0o644)) //nolint:gosec // G306: Test file, relaxed perms OK

	require.NoError(t, os.Chmod(tmpDir, 0o500)) //nolint:gosec // G302: Test uses intentionally restrictive perms
	defer func() {
		_ = os.Chmod(tmpDir, 0o700) //nolint:gosec // G302: Restoring perms in test cleanup
	}()

	err := WriteAtomic(target, []byte("replacement"), 0o600)
	require.Error(t, err)

	data, readErr := os.ReadFile(target) //nolint:gosec // G304: Test path from t.TempDir()
	require.NoError(t, readErr)
	assert.Equal(t, "original", string(data))
}

func TestWriteAtomic_EmptyPath(t *testing.T) {
	t.Parallel()

	err := WriteAtomic("", []byte("data"), 0o600)
	require.Error(t, err)
}
