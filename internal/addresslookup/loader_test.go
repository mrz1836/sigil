package addresslookup

import (
	"compress/gzip"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoad_CSVFormat(t *testing.T) {
	t.Parallel()
	content := "address,balance\n1ABC,100.50\n1DEF,200.75\n1GHI,0.01\n"
	path := writeTempFile(t, "test.csv", content)

	set, stats, err := Load(path)
	require.NoError(t, err)

	assert.Equal(t, 3, set.Count())
	assert.Equal(t, 3, stats.Count)
	assert.Positive(t, stats.LoadTime)

	assert.True(t, set.Contains("1ABC"))
	assert.True(t, set.Contains("1DEF"))
	assert.True(t, set.Contains("1GHI"))
	assert.False(t, set.Contains("1XYZ"))

	result := set.Lookup("1DEF")
	assert.True(t, result.Found)
	assert.Equal(t, "200.75", result.Balance)
}

func TestLoad_TSVFormat(t *testing.T) {
	t.Parallel()
	content := "address\tbalance\n1ABC\t100.50\n1DEF\t200.75\n"
	path := writeTempFile(t, "test.tsv", content)

	set, _, err := Load(path)
	require.NoError(t, err)

	assert.Equal(t, 2, set.Count())

	result := set.Lookup("1ABC")
	assert.True(t, result.Found)
	assert.Equal(t, "100.50", result.Balance)
}

func TestLoad_PlainFormat(t *testing.T) {
	t.Parallel()
	content := "1ABC\n1DEF\n1GHI\n"
	path := writeTempFile(t, "plain.txt", content)

	set, _, err := Load(path)
	require.NoError(t, err)

	assert.Equal(t, 3, set.Count())
	assert.True(t, set.Contains("1ABC"))
	assert.True(t, set.Contains("1DEF"))
	assert.True(t, set.Contains("1GHI"))

	result := set.Lookup("1ABC")
	assert.True(t, result.Found)
	assert.Empty(t, result.Balance)
}

func TestLoad_CRLFLineEndings(t *testing.T) {
	t.Parallel()
	content := "address,balance\r\n1ABC,100\r\n1DEF,200\r\n"
	path := writeTempFile(t, "crlf.csv", content)

	set, _, err := Load(path)
	require.NoError(t, err)

	assert.Equal(t, 2, set.Count())
	assert.True(t, set.Contains("1ABC"))
	assert.True(t, set.Contains("1DEF"))

	result := set.Lookup("1ABC")
	assert.True(t, result.Found)
	assert.Equal(t, "100", result.Balance)
}

func TestLoad_GzipFile(t *testing.T) {
	t.Parallel()
	content := "address,balance\n1ABC,100\n1DEF,200\n"
	path := writeTempGzipFile(t, "test.csv.gz", content)

	set, _, err := Load(path)
	require.NoError(t, err)

	assert.Equal(t, 2, set.Count())
	assert.True(t, set.Contains("1ABC"))
}

func TestLoad_EmptyLinesAndComments(t *testing.T) {
	t.Parallel()
	content := "# This is a comment\n\n1ABC\n\n# Another comment\n1DEF\n\n"
	path := writeTempFile(t, "comments.txt", content)

	set, _, err := Load(path)
	require.NoError(t, err)

	assert.Equal(t, 2, set.Count())
	assert.True(t, set.Contains("1ABC"))
	assert.True(t, set.Contains("1DEF"))
}

func TestLoad_NonexistentFile(t *testing.T) {
	t.Parallel()
	_, _, err := Load("/nonexistent/file.csv")
	assert.Error(t, err)
}

func TestLoad_EmptyFile(t *testing.T) {
	t.Parallel()
	path := writeTempFile(t, "empty.csv", "")

	set, stats, err := Load(path)
	require.NoError(t, err)

	assert.Equal(t, 0, set.Count())
	assert.Equal(t, 0, stats.Count)
}

func TestLoad_HeaderOnly(t *testing.T) {
	t.Parallel()
	path := writeTempFile(t, "header.csv", "address,balance\n")

	set, _, err := Load(path)
	require.NoError(t, err)

	assert.Equal(t, 0, set.Count())
}

func TestParseCSVLine(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		line string
		addr string
		bal  string
	}{
		{"comma separated", "1ABC,100.50", "1ABC", "100.50"},
		{"tab separated", "1ABC\t100.50", "1ABC", "100.50"},
		{"with spaces", " 1ABC , 100.50 ", "1ABC", "100.50"},
		{"no separator", "1ABC", "1ABC", ""},
		{"multiple commas", "1ABC,100.50,extra", "1ABC", "100.50"},
		{"empty balance", "1ABC,", "1ABC", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			addr, bal := parseCSVLine(tt.line)
			assert.Equal(t, tt.addr, addr)
			assert.Equal(t, tt.bal, bal)
		})
	}
}

func TestLoad_Stats(t *testing.T) {
	t.Parallel()
	content := "address,balance\n1ABC,100\n1DEF,200\n1GHI,300\n"
	path := writeTempFile(t, "stats.csv", content)

	set, stats, err := Load(path)
	require.NoError(t, err)

	assert.Equal(t, 3, stats.Count)
	assert.Positive(t, stats.SlotWidth)
	assert.Positive(t, stats.MemBytes)
	assert.Positive(t, stats.LoadTime)
	assert.Equal(t, set.Count(), stats.Count)
}

func TestLoadDir_MultipleSubdirs(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	// Create btc/ subdir with TSV
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "btc"), 0o750))
	require.NoError(t, os.WriteFile(
		filepath.Join(dir, "btc", "btc-addresses.tsv"),
		[]byte("address\tbalance\n1ABC\t100\nbc1qtest\t200\n3DEF\t300\n"),
		0o600,
	))

	// Create ltc/ subdir with CSV
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "ltc"), 0o750))
	require.NoError(t, os.WriteFile(
		filepath.Join(dir, "ltc", "ltc-addresses.csv"),
		[]byte("address,balance\nLtcAddr1,400\nltc1qtest,500\n"),
		0o600,
	))

	set, stats, err := LoadDir(dir)
	require.NoError(t, err)

	// 3 from btc + 2 from ltc = 5 total
	assert.Equal(t, 5, set.Count())
	assert.Equal(t, 5, stats.Count)
	assert.Positive(t, stats.MemBytes)
	assert.Positive(t, stats.LoadTime)

	// Check individual lookups
	assert.True(t, set.Contains("1ABC"))
	assert.True(t, set.Contains("bc1qtest"))
	assert.True(t, set.Contains("3DEF"))
	assert.True(t, set.Contains("LtcAddr1"))
	assert.True(t, set.Contains("ltc1qtest"))
	assert.False(t, set.Contains("notfound"))

	// Verify balances
	result := set.Lookup("1ABC")
	assert.True(t, result.Found)
	assert.Equal(t, "100", result.Balance)

	result = set.Lookup("ltc1qtest")
	assert.True(t, result.Found)
	assert.Equal(t, "500", result.Balance)
}

func TestLoadDir_EmptyDir(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	set, stats, err := LoadDir(dir)
	require.NoError(t, err)

	assert.Equal(t, 0, set.Count())
	assert.Equal(t, 0, stats.Count)
}

func TestLoadDir_SkipsNonAddressFiles(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	// Write a valid address file
	require.NoError(t, os.WriteFile(
		filepath.Join(dir, "addresses.tsv"),
		[]byte("addr1\t100\n"),
		0o600,
	))

	// Write a non-address file (should be skipped)
	require.NoError(t, os.WriteFile(
		filepath.Join(dir, "README.md"),
		[]byte("This is not an address file\n"),
		0o600,
	))
	require.NoError(t, os.WriteFile(
		filepath.Join(dir, "data.json"),
		[]byte("{}\n"),
		0o600,
	))

	set, _, err := LoadDir(dir)
	require.NoError(t, err)

	assert.Equal(t, 1, set.Count())
	assert.True(t, set.Contains("addr1"))
}

func TestLoadDir_GzipFiles(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	// Create a gzip address file
	gzPath := filepath.Join(dir, "addresses.csv.gz")
	f, err := os.Create(gzPath) //nolint:gosec // G304: test creates files in temp dir
	require.NoError(t, err)
	gz := gzip.NewWriter(f)
	_, err = gz.Write([]byte("addr1,100\naddr2,200\n"))
	require.NoError(t, err)
	require.NoError(t, gz.Close())
	require.NoError(t, f.Close())

	set, _, err := LoadDir(dir)
	require.NoError(t, err)

	assert.Equal(t, 2, set.Count())
	assert.True(t, set.Contains("addr1"))
	assert.True(t, set.Contains("addr2"))
}

func TestLoadDir_NonexistentDir(t *testing.T) {
	t.Parallel()
	_, _, err := LoadDir("/nonexistent/dir")
	assert.Error(t, err)
}

func TestLoadDir_BackwardCompatWithSingleFile(t *testing.T) {
	t.Parallel()
	// LoadDir with a directory containing a single file should produce
	// the same results as Load with that file directly
	content := "address,balance\n1ABC,100\n1DEF,200\n"

	// Test with Load
	filePath := writeTempFile(t, "test.csv", content)
	setFile, _, err := Load(filePath)
	require.NoError(t, err)

	// Test with LoadDir
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "test.csv"), []byte(content), 0o600))
	setDir, _, err := LoadDir(dir)
	require.NoError(t, err)

	// Same count and same addresses
	assert.Equal(t, setFile.Count(), setDir.Count())
	assert.Equal(t, setFile.Contains("1ABC"), setDir.Contains("1ABC"))
	assert.Equal(t, setFile.Contains("1DEF"), setDir.Contains("1DEF"))
}

func TestIsAddressFile(t *testing.T) {
	t.Parallel()
	assert.True(t, isAddressFile("addresses.tsv"))
	assert.True(t, isAddressFile("addresses.csv"))
	assert.True(t, isAddressFile("addresses.txt"))
	assert.True(t, isAddressFile("addresses.csv.gz"))
	assert.True(t, isAddressFile("ADDRESSES.TSV"))
	assert.False(t, isAddressFile("README.md"))
	assert.False(t, isAddressFile("data.json"))
	assert.False(t, isAddressFile("script.sh"))
}

func TestLoadDirWithProgress_CallbackSequence(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	// Create 3 address files
	require.NoError(t, os.WriteFile(filepath.Join(dir, "a.tsv"), []byte("addr1\t100\naddr2\t200\n"), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "b.csv"), []byte("addr3,300\n"), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "c.txt"), []byte("addr4\naddr5\n"), 0o600))

	var events []LoadProgress
	cb := func(p LoadProgress) {
		events = append(events, p)
	}

	set, stats, err := LoadDirWithProgress(dir, cb)
	require.NoError(t, err)
	assert.Equal(t, 5, set.Count())
	assert.Equal(t, 5, stats.Count)

	// Expect 3 "loading_file" events + 1 "building_index" event
	require.Len(t, events, 4)

	for i := 0; i < 3; i++ {
		assert.Equal(t, "loading_file", events[i].Phase)
		assert.Equal(t, 3, events[i].FilesTotal)
		assert.Equal(t, i, events[i].FilesLoaded)
		assert.NotEmpty(t, events[i].FileName)
	}

	last := events[3]
	assert.Equal(t, "building_index", last.Phase)
	assert.Equal(t, 3, last.FilesTotal)
	assert.Equal(t, 3, last.FilesLoaded)
	assert.Equal(t, 5, last.PairsLoaded)
}

func TestLoadWithProgress_CallbackSequence(t *testing.T) {
	t.Parallel()
	content := "address,balance\n1ABC,100\n1DEF,200\n"
	path := writeTempFile(t, "progress.csv", content)

	var events []LoadProgress
	cb := func(p LoadProgress) {
		events = append(events, p)
	}

	set, _, err := LoadWithProgress(path, cb)
	require.NoError(t, err)
	assert.Equal(t, 2, set.Count())

	require.Len(t, events, 2)
	assert.Equal(t, "loading_file", events[0].Phase)
	assert.Equal(t, 1, events[0].FilesTotal)
	assert.Equal(t, "progress.csv", events[0].FileName)

	assert.Equal(t, "building_index", events[1].Phase)
	assert.Equal(t, 2, events[1].PairsLoaded)
}

func TestLoadDirWithProgress_NilCallback(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "test.txt"), []byte("addr1\n"), 0o600))

	set, stats, err := LoadDirWithProgress(dir, nil)
	require.NoError(t, err)
	assert.Equal(t, 1, set.Count())
	assert.Equal(t, 1, stats.Count)
}

// Helper functions

func writeTempFile(t *testing.T, name, content string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, name)
	err := os.WriteFile(path, []byte(content), 0o600)
	require.NoError(t, err)
	return path
}

func writeTempGzipFile(t *testing.T, name, content string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, name)

	f, err := os.Create(path) //nolint:gosec // G304: test creates files in temp dir
	require.NoError(t, err)

	gz := gzip.NewWriter(f)
	_, err = gz.Write([]byte(content))
	require.NoError(t, err)
	require.NoError(t, gz.Close())
	require.NoError(t, f.Close())

	return path
}
