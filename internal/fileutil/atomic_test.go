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

func TestWriteAtomic_Concurrent(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	target := filepath.Join(tmpDir, "concurrent.json")

	const numGoroutines = 10
	done := make(chan int, numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		go func(id int) {
			data := []byte("writer-" + string(rune('0'+id)))
			err := WriteAtomic(target, data, 0o600)
			assert.NoError(t, err)
			done <- id
		}(i)
	}

	// Wait for all goroutines to complete
	for i := 0; i < numGoroutines; i++ {
		<-done
	}

	// Verify file exists and is readable
	data, err := os.ReadFile(target) //nolint:gosec // G304: Test path from t.TempDir()
	require.NoError(t, err)
	assert.NotEmpty(t, data, "file should have content from one of the writers")

	// Verify no temp files left behind
	entries, err := os.ReadDir(tmpDir)
	require.NoError(t, err)
	assert.Len(t, entries, 1, "should only have the target file, no temp files")
}

func TestWriteAtomic_LargeFile(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	target := filepath.Join(tmpDir, "large.dat")

	// Create 10MB of data
	data := make([]byte, 10*1024*1024)
	for i := range data {
		data[i] = byte(i % 256)
	}

	err := WriteAtomic(target, data, 0o600)
	require.NoError(t, err)

	// Verify the data
	readData, err := os.ReadFile(target) //nolint:gosec // G304: Test path from t.TempDir()
	require.NoError(t, err)
	assert.Equal(t, data, readData)
}

func TestWriteAtomic_TempFileCleanup(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	target := filepath.Join(tmpDir, "cleanup.json")

	// Write multiple times
	for i := 0; i < 5; i++ {
		data := []byte("iteration-" + string(rune('0'+i)))
		err := WriteAtomic(target, data, 0o600)
		require.NoError(t, err)
	}

	// Verify no temp files left behind
	entries, err := os.ReadDir(tmpDir)
	require.NoError(t, err)

	for _, entry := range entries {
		assert.NotEqual(t, ".tmp", filepath.Ext(entry.Name()), "found temp file: %s", entry.Name())
		assert.NotContains(t, entry.Name(), ".tmp-", "found temp file: %s", entry.Name())
	}
}

func TestWriteAtomic_PermissionPreservation(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		perm os.FileMode
	}{
		{"0600", 0o600},
		{"0640", 0o640},
		{"0644", 0o644},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			tmpDir := t.TempDir()
			target := filepath.Join(tmpDir, "perms.dat")

			err := WriteAtomic(target, []byte("test"), tc.perm)
			require.NoError(t, err)

			info, err := os.Stat(target)
			require.NoError(t, err)
			assert.Equal(t, tc.perm, info.Mode().Perm(), "permissions should match requested")
		})
	}
}

func TestWriteAtomic_ZeroLength(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	target := filepath.Join(tmpDir, "empty.dat")

	// Write empty file
	err := WriteAtomic(target, []byte{}, 0o600)
	require.NoError(t, err)

	// Verify file exists and is empty
	data, err := os.ReadFile(target) //nolint:gosec // G304: Test path from t.TempDir()
	require.NoError(t, err)
	assert.Empty(t, data)

	info, err := os.Stat(target)
	require.NoError(t, err)
	assert.Equal(t, int64(0), info.Size())
}

func TestWriteAtomic_NestedDirectories(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	nestedPath := filepath.Join(tmpDir, "level1", "level2", "level3")

	// Create nested directories
	err := os.MkdirAll(nestedPath, 0o750)
	require.NoError(t, err)

	target := filepath.Join(nestedPath, "nested.json")

	err = WriteAtomic(target, []byte("nested data"), 0o600)
	require.NoError(t, err)

	data, err := os.ReadFile(target) //nolint:gosec // G304: Test path from t.TempDir()
	require.NoError(t, err)
	assert.Equal(t, "nested data", string(data))
}

func TestWriteAtomic_OverwriteExisting(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	target := filepath.Join(tmpDir, "overwrite.dat")

	// Write initial file with different permissions
	require.NoError(t, os.WriteFile(target, []byte("original"), 0o644)) //nolint:gosec // G306: Test file, relaxed perms OK

	// Overwrite with atomic write and new permissions
	err := WriteAtomic(target, []byte("updated"), 0o600)
	require.NoError(t, err)

	data, err := os.ReadFile(target) //nolint:gosec // G304: Test path from t.TempDir()
	require.NoError(t, err)
	assert.Equal(t, "updated", string(data))

	// Verify permissions were updated
	info, err := os.Stat(target)
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0o600), info.Mode().Perm())
}
