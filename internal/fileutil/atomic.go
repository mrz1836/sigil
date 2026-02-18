// Package fileutil provides filesystem helpers for robust file operations.
package fileutil

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// ErrEmptyPath indicates an empty file path was provided.
var ErrEmptyPath = errors.New("path is empty")

// WriteAtomic writes data to path atomically with the provided permissions.
// It writes to a temp file in the same directory, fsyncs, then renames.
func WriteAtomic(path string, data []byte, perm os.FileMode) error {
	if path == "" {
		return ErrEmptyPath
	}

	dir := filepath.Dir(path)
	base := filepath.Base(path)

	tmpFile, err := os.CreateTemp(dir, base+".tmp-*")
	if err != nil {
		return fmt.Errorf("creating temp file: %w", err)
	}

	tmpPath := tmpFile.Name()
	closed := false
	defer func() {
		if !closed {
			_ = tmpFile.Close()
		}
		_ = os.Remove(tmpPath)
	}()

	if _, err := tmpFile.Write(data); err != nil {
		return fmt.Errorf("writing temp file: %w", err)
	}

	if err := tmpFile.Chmod(perm); err != nil {
		return fmt.Errorf("setting temp file permissions: %w", err)
	}

	if err := tmpFile.Sync(); err != nil {
		return fmt.Errorf("syncing temp file: %w", err)
	}

	if err := tmpFile.Close(); err != nil {
		return fmt.Errorf("closing temp file: %w", err)
	}
	closed = true

	if err := os.Rename(tmpPath, path); err != nil { //nolint:gosec // G703: path is validated by caller, not from user input
		return fmt.Errorf("renaming temp file: %w", err)
	}

	// Best effort directory sync for rename durability.
	if dirFile, err := os.Open(dir); err == nil { //nolint:gosec // G304: dir is derived from validated path
		_ = dirFile.Sync()
		_ = dirFile.Close()
	}

	return nil
}
