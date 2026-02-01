package cache

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

const (
	// cacheFilePermissions is the permission mode for cache files.
	cacheFilePermissions = 0o640

	// cacheDirPermissions is the permission mode for cache directories.
	cacheDirPermissions = 0o750
)

// FileStorage implements cache persistence using the filesystem.
type FileStorage struct {
	path string
}

// NewFileStorage creates a new file-based cache storage.
func NewFileStorage(path string) *FileStorage {
	return &FileStorage{path: path}
}

// Save writes the cache to the filesystem.
func (s *FileStorage) Save(cache *BalanceCache) error {
	// Ensure directory exists
	dir := filepath.Dir(s.path)
	if err := os.MkdirAll(dir, cacheDirPermissions); err != nil {
		return fmt.Errorf("creating cache directory: %w", err)
	}

	// Marshal to JSON
	data, err := json.MarshalIndent(cache, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling cache: %w", err)
	}

	// Write to file
	if err := os.WriteFile(s.path, data, cacheFilePermissions); err != nil {
		return fmt.Errorf("writing cache file: %w", err)
	}

	return nil
}

// Load reads the cache from the filesystem.
// Returns an empty cache if the file doesn't exist.
func (s *FileStorage) Load() (*BalanceCache, error) {
	// Check if file exists
	if _, err := os.Stat(s.path); os.IsNotExist(err) {
		return NewBalanceCache(), nil
	}

	// Read file
	data, err := os.ReadFile(s.path)
	if err != nil {
		return nil, fmt.Errorf("reading cache file: %w", err)
	}

	// Unmarshal JSON
	var cache BalanceCache
	if err := json.Unmarshal(data, &cache); err != nil {
		// If JSON is corrupted, return empty cache (intentionally ignoring parse error)
		return NewBalanceCache(), nil //nolint:nilerr // intentionally ignoring corrupted cache
	}

	// Ensure map is initialized
	if cache.Entries == nil {
		cache.Entries = make(map[string]BalanceCacheEntry)
	}

	return &cache, nil
}

// Delete removes the cache file.
func (s *FileStorage) Delete() error {
	if _, err := os.Stat(s.path); os.IsNotExist(err) {
		return nil // Already doesn't exist
	}

	if err := os.Remove(s.path); err != nil {
		return fmt.Errorf("removing cache file: %w", err)
	}

	return nil
}

// Exists checks if the cache file exists.
func (s *FileStorage) Exists() bool {
	_, err := os.Stat(s.path)
	return err == nil
}

// Path returns the cache file path.
func (s *FileStorage) Path() string {
	return s.path
}
