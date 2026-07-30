package cli

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestDownloadToString_HTTP exercises downloadToString against a local httptest
// server for both the success and non-200 branches (no real network).
func TestDownloadToString_HTTP(t *testing.T) {
	t.Parallel()

	t.Run("success returns body", func(t *testing.T) {
		t.Parallel()
		const body = "deadbeef  sigil_1.0.0_darwin_arm64.tar.gz\n"
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			_, _ = w.Write([]byte(body))
		}))
		defer server.Close()

		got, err := downloadToString(server.URL)
		require.NoError(t, err)
		assert.Equal(t, body, got)
	})

	t.Run("non-200 yields ErrHTTPStatus", func(t *testing.T) {
		t.Parallel()
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusNotFound)
		}))
		defer server.Close()

		got, err := downloadToString(server.URL)
		require.ErrorIs(t, err, ErrHTTPStatus)
		assert.Empty(t, got)
	})

	t.Run("bad url errors", func(t *testing.T) {
		t.Parallel()
		// A control character makes http.NewRequestWithContext fail before any dial.
		_, err := downloadToString("http://\x7f invalid")
		require.Error(t, err)
	})
}

// TestDownloadToFile_HTTP exercises downloadToFile against a local httptest server,
// verifying the file is written on success and not created on a non-200 response.
func TestDownloadToFile_HTTP(t *testing.T) {
	t.Parallel()

	t.Run("success writes file", func(t *testing.T) {
		t.Parallel()
		payload := []byte("binary-archive-bytes")
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			_, _ = w.Write(payload)
		}))
		defer server.Close()

		dest := filepath.Join(t.TempDir(), "archive.tar.gz")
		err := downloadToFile(server.URL, dest)
		require.NoError(t, err)

		got, readErr := os.ReadFile(dest) //nolint:gosec // Test reads from temp dir
		require.NoError(t, readErr)
		assert.Equal(t, payload, got)
	})

	t.Run("non-200 yields ErrHTTPStatus and no file", func(t *testing.T) {
		t.Parallel()
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
		}))
		defer server.Close()

		dest := filepath.Join(t.TempDir(), "archive.tar.gz")
		err := downloadToFile(server.URL, dest)
		require.ErrorIs(t, err, ErrHTTPStatus)

		// The status check happens before os.Create, so no file exists.
		_, statErr := os.Stat(dest)
		assert.True(t, os.IsNotExist(statErr), "destination file should not be created")
	})
}

// TestReplaceBinary_Cases covers the backup/copy/cleanup flow of replaceBinary,
// including both failure branches, all without touching the network.
func TestReplaceBinary_Cases(t *testing.T) {
	t.Parallel()

	t.Run("success swaps content and removes backup", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		current := filepath.Join(dir, "sigil")
		newBin := filepath.Join(dir, "sigil-new")
		require.NoError(t, os.WriteFile(current, []byte("OLD"), 0o755)) //nolint:gosec // test binary perms
		require.NoError(t, os.WriteFile(newBin, []byte("NEW"), 0o755))  //nolint:gosec // test binary perms

		require.NoError(t, replaceBinary(current, newBin))

		got, err := os.ReadFile(current) //nolint:gosec // Test reads from temp dir
		require.NoError(t, err)
		assert.Equal(t, []byte("NEW"), got)

		// Backup must be cleaned up on success.
		_, statErr := os.Stat(current + ".backup")
		assert.True(t, os.IsNotExist(statErr), "backup should be removed after success")
	})

	t.Run("missing source restores original", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		current := filepath.Join(dir, "sigil")
		require.NoError(t, os.WriteFile(current, []byte("ORIGINAL"), 0o755)) //nolint:gosec // test binary perms
		missingNew := filepath.Join(dir, "does-not-exist")

		err := replaceBinary(current, missingNew)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "could not replace binary")

		// The original binary must be restored from the backup.
		got, readErr := os.ReadFile(current) //nolint:gosec // Test reads from temp dir
		require.NoError(t, readErr)
		assert.Equal(t, []byte("ORIGINAL"), got)
		_, statErr := os.Stat(current + ".backup")
		assert.True(t, os.IsNotExist(statErr), "backup should be renamed back to current")
	})

	t.Run("missing current fails at backup", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		current := filepath.Join(dir, "nonexistent-sigil")
		newBin := filepath.Join(dir, "sigil-new")
		require.NoError(t, os.WriteFile(newBin, []byte("NEW"), 0o755)) //nolint:gosec // test binary perms

		err := replaceBinary(current, newBin)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "could not backup current binary")
	})
}
