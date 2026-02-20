package cli

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUpgradeFormatVersion(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{name: "DevVersion", input: "dev", expected: "dev"},
		{name: "EmptyString", input: "", expected: "dev"},
		{name: "SemverWithoutPrefix", input: "1.2.3", expected: "v1.2.3"},
		{name: "SemverWithPrefix", input: "v1.2.3", expected: "v1.2.3"},
		{name: "MajorOnly", input: "2", expected: "v2"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result := upgradeFormatVersion(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestIsLikelyCommitHash(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		input    string
		expected bool
	}{
		{name: "ShortHash", input: "abc123d", expected: true},
		{name: "LongHash", input: "abc123def456789012345678901234567890abcd", expected: true},
		{name: "DirtySuffix", input: "abc123d-dirty", expected: true},
		{name: "MixedCase", input: "AbC123DeF456", expected: true},
		{name: "TooShort", input: "abc12", expected: false},
		{name: "TooLong", input: "abc123def456789012345678901234567890abcdef", expected: false},
		{name: "InvalidChars", input: "abc123xyz", expected: false},
		{name: "Empty", input: "", expected: false},
		{name: "SemverVersion", input: "1.2.3", expected: false},
		{name: "DevString", input: "dev", expected: false},
		{name: "PureNumbers", input: "1234567", expected: false},
		{name: "AllZeros", input: "0000000", expected: false},
		{name: "HexWithLetters", input: "1a2b3c4d", expected: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result := isLikelyCommitHash(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestGetCurrentVersion(t *testing.T) {
	// Save and restore buildInfo
	origBuildInfo := buildInfo
	defer func() { buildInfo = origBuildInfo }()

	t.Run("WithVersion", func(t *testing.T) {
		buildInfo = BuildInfo{Version: "1.2.3"}
		assert.Equal(t, "1.2.3", GetCurrentVersion())
	})

	t.Run("EmptyVersion", func(t *testing.T) {
		buildInfo = BuildInfo{Version: ""}
		assert.Equal(t, "dev", GetCurrentVersion())
	})

	t.Run("DevVersion", func(t *testing.T) {
		buildInfo = BuildInfo{Version: "dev"}
		assert.Equal(t, "dev", GetCurrentVersion())
	})
}

func TestUpgradeCmd_Flags(t *testing.T) {
	t.Parallel()

	cmd := newUpgradeCmd()

	// Verify flags exist
	assert.NotNil(t, cmd.Flags().Lookup("force"))
	assert.NotNil(t, cmd.Flags().Lookup("check"))
	assert.NotNil(t, cmd.Flags().Lookup("use-go-install"))

	// Verify flag shorthand
	f := cmd.Flags().ShorthandLookup("f")
	assert.NotNil(t, f)
	assert.Equal(t, "force", f.Name)

	// Verify command metadata
	assert.Equal(t, "upgrade", cmd.Use)
	assert.Contains(t, cmd.Short, "Upgrade")
}

func TestUpgradeCmd_Registration(t *testing.T) {
	t.Parallel()

	// Verify the upgrade command is registered on rootCmd
	found := false
	for _, cmd := range rootCmd.Commands() {
		if cmd.Use == "upgrade" {
			found = true
			assert.Equal(t, "config", cmd.GroupID)
			break
		}
	}
	assert.True(t, found, "upgrade command should be registered on rootCmd")
}

func TestUpgradeCheckOnly(t *testing.T) {
	// Save and restore buildInfo
	origBuildInfo := buildInfo
	defer func() { buildInfo = origBuildInfo }()

	// Set a known version for testing
	buildInfo = BuildInfo{Version: "0.0.1", Commit: "test", Date: "test"}

	// Create a mock GitHub API server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "/releases/latest") {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{
				"tag_name": "v99.99.99",
				"name": "Release v99.99.99",
				"draft": false,
				"prerelease": false,
				"published_at": "2025-01-01T12:00:00Z",
				"body": "Test release"
			}`))
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	// Create an upgrade command and verify flag setting works
	cmd := newUpgradeCmd()
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})

	err := cmd.Flags().Set("check", "true")
	require.NoError(t, err)
}

func TestVerifyChecksum(t *testing.T) {
	t.Parallel()

	// Create a temp file with known content
	tempDir := t.TempDir()
	testFile := filepath.Join(tempDir, "test.tar.gz")
	testContent := []byte("hello world test content")
	require.NoError(t, os.WriteFile(testFile, testContent, 0o600))

	// Calculate expected checksum
	hasher := sha256.New()
	hasher.Write(testContent)
	expectedChecksum := hex.EncodeToString(hasher.Sum(nil))

	t.Run("ValidChecksum", func(t *testing.T) {
		t.Parallel()
		checksums := fmt.Sprintf("%s  test.tar.gz\n", expectedChecksum)
		err := verifyChecksum(testFile, "test.tar.gz", checksums)
		assert.NoError(t, err)
	})

	t.Run("InvalidChecksum", func(t *testing.T) {
		t.Parallel()
		checksums := "0000000000000000000000000000000000000000000000000000000000000000  test.tar.gz\n"
		err := verifyChecksum(testFile, "test.tar.gz", checksums)
		assert.ErrorIs(t, err, ErrChecksumMismatch)
	})

	t.Run("FileNotInChecksums", func(t *testing.T) {
		t.Parallel()
		checksums := fmt.Sprintf("%s  other.tar.gz\n", expectedChecksum)
		err := verifyChecksum(testFile, "test.tar.gz", checksums)
		require.Error(t, err)
		assert.ErrorIs(t, err, ErrChecksumNotFound)
	})

	t.Run("MultipleEntries", func(t *testing.T) {
		t.Parallel()
		checksums := fmt.Sprintf("aaaa  other1.tar.gz\n%s  test.tar.gz\nbbbb  other2.tar.gz\n", expectedChecksum)
		err := verifyChecksum(testFile, "test.tar.gz", checksums)
		assert.NoError(t, err)
	})

	t.Run("NonexistentFile", func(t *testing.T) {
		t.Parallel()
		err := verifyChecksum(filepath.Join(tempDir, "nonexistent"), "test.tar.gz", "checksum  test.tar.gz")
		assert.Error(t, err)
	})
}

func TestExtractBinaryFromArchive(t *testing.T) {
	t.Parallel()

	t.Run("ValidArchive", func(t *testing.T) {
		t.Parallel()
		tempDir := t.TempDir()

		// Create a tar.gz archive with a sigil binary
		archivePath := filepath.Join(tempDir, "sigil.tar.gz")
		binaryContent := []byte("#!/bin/sh\necho hello")
		createTestArchive(t, archivePath, "sigil", binaryContent)

		// Extract
		extractDir := filepath.Join(tempDir, "extract")
		require.NoError(t, os.MkdirAll(extractDir, 0o750))

		result, err := extractBinaryFromArchive(archivePath, extractDir)
		require.NoError(t, err)
		assert.Equal(t, filepath.Join(extractDir, "sigil"), result)

		// Verify content
		content, err := os.ReadFile(result) //nolint:gosec // Test reads from temp dir
		require.NoError(t, err)
		assert.Equal(t, binaryContent, content)
	})

	t.Run("BinaryNotFound", func(t *testing.T) {
		t.Parallel()
		tempDir := t.TempDir()

		// Create a tar.gz archive without the sigil binary
		archivePath := filepath.Join(tempDir, "archive.tar.gz")
		createTestArchive(t, archivePath, "other-binary", []byte("content"))

		extractDir := filepath.Join(tempDir, "extract")
		require.NoError(t, os.MkdirAll(extractDir, 0o750))

		_, err := extractBinaryFromArchive(archivePath, extractDir)
		assert.ErrorIs(t, err, ErrBinaryNotFoundInArchive)
	})

	t.Run("InvalidArchive", func(t *testing.T) {
		t.Parallel()
		tempDir := t.TempDir()

		// Create an invalid archive file
		archivePath := filepath.Join(tempDir, "invalid.tar.gz")
		require.NoError(t, os.WriteFile(archivePath, []byte("not a real archive"), 0o600))

		extractDir := filepath.Join(tempDir, "extract")
		require.NoError(t, os.MkdirAll(extractDir, 0o750))

		_, err := extractBinaryFromArchive(archivePath, extractDir)
		assert.Error(t, err)
	})
}

func TestCopyFile(t *testing.T) {
	t.Parallel()

	tempDir := t.TempDir()
	srcPath := filepath.Join(tempDir, "src")
	dstPath := filepath.Join(tempDir, "dst")

	content := []byte("test binary content")
	require.NoError(t, os.WriteFile(srcPath, content, 0o600))

	err := copyFile(srcPath, dstPath, 0o755)
	require.NoError(t, err)

	// Verify content
	result, err := os.ReadFile(dstPath) //nolint:gosec // Test reads from temp dir
	require.NoError(t, err)
	assert.Equal(t, content, result)

	// Verify permissions
	info, err := os.Stat(dstPath)
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0o755), info.Mode().Perm())
}

func TestUpgradeDevVersionNoForce(t *testing.T) {
	// Save and restore buildInfo
	origBuildInfo := buildInfo
	defer func() { buildInfo = origBuildInfo }()

	buildInfo = BuildInfo{Version: "dev"}

	cmd := newUpgradeCmd()
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})

	// Running without --force should fail for dev version
	err := cmd.RunE(cmd, nil)
	assert.ErrorIs(t, err, ErrDevVersionNoForce)
}

func TestUpgradeDevVersionWithCheck(t *testing.T) {
	// Save and restore buildInfo
	origBuildInfo := buildInfo
	defer func() { buildInfo = origBuildInfo }()

	buildInfo = BuildInfo{Version: "dev"}

	// Create a mock server for the version check
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{
			"tag_name": "v1.0.0",
			"name": "Release v1.0.0",
			"draft": false,
			"prerelease": false,
			"published_at": "2025-01-01T12:00:00Z",
			"body": "Test release"
		}`))
	}))
	defer server.Close()

	// --check with dev version should not return ErrDevVersionNoForce
	cmd := newUpgradeCmd()
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})

	require.NoError(t, cmd.Flags().Set("check", "true"))
	// This will try to hit the real GitHub API since we can't easily inject
	// the mock URL, so we just verify the flag behavior avoids dev version error
	err := cmd.RunE(cmd, nil)
	if err != nil {
		assert.NotErrorIs(t, err, ErrDevVersionNoForce)
	}
}

func TestFindChecksumEntry(t *testing.T) {
	t.Parallel()

	checksums := "abc123  sigil_1.0.0_darwin_arm64.tar.gz\ndef456  sigil_1.0.0_linux_amd64.tar.gz\n"

	t.Run("Found", func(t *testing.T) {
		t.Parallel()
		result := findChecksumEntry("sigil_1.0.0_darwin_arm64.tar.gz", checksums)
		assert.Equal(t, "abc123", result)
	})

	t.Run("NotFound", func(t *testing.T) {
		t.Parallel()
		result := findChecksumEntry("sigil_1.0.0_windows_amd64.tar.gz", checksums)
		assert.Empty(t, result)
	})
}

// createTestArchive creates a tar.gz archive with a single file for testing
func createTestArchive(t *testing.T, archivePath, fileName string, content []byte) {
	t.Helper()

	file, err := os.Create(archivePath) //nolint:gosec // Test creates files in temp dir
	require.NoError(t, err)
	defer func() { _ = file.Close() }()

	gzWriter := gzip.NewWriter(file)
	defer func() { _ = gzWriter.Close() }()

	tarWriter := tar.NewWriter(gzWriter)
	defer func() { _ = tarWriter.Close() }()

	header := &tar.Header{
		Name:     fileName,
		Size:     int64(len(content)),
		Mode:     0o755,
		Typeflag: tar.TypeReg,
	}
	require.NoError(t, tarWriter.WriteHeader(header))
	_, err = tarWriter.Write(content)
	require.NoError(t, err)
}
