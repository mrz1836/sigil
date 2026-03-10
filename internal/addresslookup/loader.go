package addresslookup

import (
	"bufio"
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// LoadProgress reports the current state of address data loading.
type LoadProgress struct {
	Phase       string // "loading_file" or "building_index"
	FilesTotal  int
	FilesLoaded int
	FileName    string
	PairsLoaded int
}

// LoadProgressCallback is called during loading to report progress.
type LoadProgressCallback func(LoadProgress)

// Load reads an address data file and builds an AddressSet.
// Supports CSV (address,balance), plain (one address per line),
// CRLF line endings, and gzip-compressed files (.gz extension).
func Load(path string) (*AddressSet, Stats, error) {
	return LoadWithProgress(path, nil)
}

// LoadWithProgress is like Load but reports progress via the callback.
func LoadWithProgress(path string, cb LoadProgressCallback) (*AddressSet, Stats, error) {
	start := time.Now()

	if cb != nil {
		cb(LoadProgress{
			Phase:      "loading_file",
			FilesTotal: 1, FilesLoaded: 0,
			FileName: filepath.Base(path),
		})
	}

	pairs, err := loadPairs(path)
	if err != nil {
		return nil, Stats{}, err
	}

	if cb != nil {
		cb(LoadProgress{
			Phase:       "building_index",
			FilesTotal:  1,
			FilesLoaded: 1,
			PairsLoaded: len(pairs),
		})
	}

	set := NewAddressSet(pairs)
	loadTime := time.Since(start)

	stats := Stats{
		Count:    set.Count(),
		MemBytes: set.MemBytes(),
		LoadTime: loadTime,
	}

	return set, stats, nil
}

// loadPairs reads an address data file and returns address-balance pairs.
// This is the internal implementation shared by Load and LoadDir.
//
//nolint:gocognit,gocyclo // File parsing with format detection has inherent branching
func loadPairs(path string) ([]addrBal, error) {
	f, err := os.Open(path) //nolint:gosec // G304: path comes from validated CLI flag or directory walk
	if err != nil {
		return nil, fmt.Errorf("open address file: %w", err)
	}
	defer func() { _ = f.Close() }()

	var reader io.Reader = f

	if strings.HasSuffix(path, ".gz") {
		gz, gzErr := gzip.NewReader(f)
		if gzErr != nil {
			return nil, fmt.Errorf("gzip reader: %w", gzErr)
		}
		defer func() { _ = gz.Close() }()
		reader = gz
	}

	scanner := bufio.NewScanner(reader)
	scanner.Buffer(make([]byte, 0, 1024*1024), 1024*1024)

	var pairs []addrBal
	isCSV := false
	formatDetected := false

	for scanner.Scan() {
		line := scanner.Text()
		line = strings.TrimRight(line, "\r\n")
		line = strings.TrimSpace(line)

		if line == "" || line[0] == '#' {
			continue
		}

		if !formatDetected {
			lower := strings.ToLower(line)
			if strings.Contains(lower, "address") && strings.Contains(lower, "balance") {
				isCSV = true
				formatDetected = true
				continue
			}
			if strings.Contains(line, ",") || strings.Contains(line, "\t") {
				isCSV = true
			}
			formatDetected = true
		}

		var addr, bal string
		if isCSV {
			addr, bal = parseCSVLine(line)
		} else {
			addr = line
		}

		if addr == "" {
			continue
		}

		pairs = append(pairs, addrBal{addr: addr, bal: bal})
	}

	if err = scanner.Err(); err != nil {
		return nil, fmt.Errorf("reading address file: %w", err)
	}

	return pairs, nil
}

// isAddressFile returns true if the file extension suggests it contains address data.
func isAddressFile(name string) bool {
	lower := strings.ToLower(name)
	for _, ext := range []string{".tsv", ".csv", ".txt", ".gz"} {
		if strings.HasSuffix(lower, ext) {
			return true
		}
	}
	return false
}

// collectAddressFiles walks a directory recursively and returns paths of all address data files.
func collectAddressFiles(dirPath string) ([]string, error) {
	var files []string
	err := filepath.Walk(dirPath, func(path string, info os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if !info.IsDir() && isAddressFile(info.Name()) {
			files = append(files, path)
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("walking directory: %w", err)
	}
	return files, nil
}

// LoadDir walks a directory recursively, loads all address data files
// (.tsv, .csv, .txt, .gz), and builds a single unified AddressSet.
func LoadDir(dirPath string) (*AddressSet, Stats, error) {
	return LoadDirWithProgress(dirPath, nil)
}

// LoadDirWithProgress is like LoadDir but reports progress via the callback.
func LoadDirWithProgress(dirPath string, cb LoadProgressCallback) (*AddressSet, Stats, error) {
	start := time.Now()

	files, err := collectAddressFiles(dirPath)
	if err != nil {
		return nil, Stats{}, err
	}

	// Two-pass approach: collect per-file slices first, then do a single
	// capacity-hinted allocation for the combined slice. This avoids the
	// repeated doubling that append causes for large multi-file loads and
	// halves peak memory compared to in-place appending.
	allSlices := make([][]addrBal, 0, len(files))
	totalPairs := 0
	for i, path := range files {
		if cb != nil {
			cb(LoadProgress{
				Phase:       "loading_file",
				FilesTotal:  len(files),
				FilesLoaded: i,
				FileName:    filepath.Base(path),
				PairsLoaded: totalPairs,
			})
		}
		pairs, loadErr := loadPairs(path)
		if loadErr != nil {
			return nil, Stats{}, fmt.Errorf("loading %s: %w", path, loadErr)
		}
		allSlices = append(allSlices, pairs)
		totalPairs += len(pairs)
	}

	if cb != nil {
		cb(LoadProgress{
			Phase:       "building_index",
			FilesTotal:  len(files),
			FilesLoaded: len(files),
			PairsLoaded: totalPairs,
		})
	}

	allPairs := make([]addrBal, 0, totalPairs)
	for _, s := range allSlices {
		allPairs = append(allPairs, s...)
	}

	set := NewAddressSet(allPairs)
	loadTime := time.Since(start)

	stats := Stats{
		Count:    set.Count(),
		MemBytes: set.MemBytes(),
		LoadTime: loadTime,
	}

	return set, stats, nil
}

// parseCSVLine splits a CSV or TSV line into address and balance.
func parseCSVLine(line string) (addr, bal string) {
	// Try tab first (TSV), then comma
	sep := "\t"
	idx := strings.Index(line, sep)
	if idx < 0 {
		sep = ","
		idx = strings.Index(line, sep)
	}

	if idx < 0 {
		// No separator, treat whole line as address
		return strings.TrimSpace(line), ""
	}

	addr = strings.TrimSpace(line[:idx])
	bal = strings.TrimSpace(line[idx+len(sep):])

	// If balance contains more separators, take only the first field
	if nextSep := strings.IndexAny(bal, "\t,"); nextSep >= 0 {
		bal = strings.TrimSpace(bal[:nextSep])
	}

	return addr, bal
}
