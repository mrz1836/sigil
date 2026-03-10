package cli

import (
	"bufio"
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/spf13/pflag"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mrz1836/sigil/internal/wallet"
)

// resetKeygenGlobals saves all keygen command globals and restores them after the test.
func resetKeygenGlobals(t *testing.T) {
	t.Helper()
	origFormat := keygenFormat
	origCount := keygenCount
	origFile := keygenFile
	origWorkers := keygenWorkers
	t.Cleanup(func() {
		keygenFormat = origFormat
		keygenCount = origCount
		keygenFile = origFile
		keygenWorkers = origWorkers
		keygenCmd.Flags().VisitAll(func(f *pflag.Flag) {
			f.Changed = false
		})
		rootCmd.SetArgs(nil)
	})
	keygenFormat = ""
	keygenCount = 0
	keygenFile = ""
	keygenWorkers = 1
	keygenCmd.Flags().VisitAll(func(f *pflag.Flag) {
		f.Changed = false
	})
}

func TestKeygenCmd_Registration(t *testing.T) {
	t.Parallel()

	found := false
	for _, cmd := range rootCmd.Commands() {
		if cmd.Name() == "keygen" {
			found = true
			assert.Equal(t, "utility", cmd.GroupID)
			assert.NotEmpty(t, cmd.Short)
			assert.NotEmpty(t, cmd.Long)
			assert.NotEmpty(t, cmd.Example)
			break
		}
	}
	assert.True(t, found, "keygen command should be registered under root")
}

func TestKeygenCmd_FlagDefaults(t *testing.T) {
	t.Parallel()

	f := keygenCmd.Flags()

	formatFlag := f.Lookup("format")
	require.NotNil(t, formatFlag)
	assert.Empty(t, formatFlag.DefValue)

	countFlag := f.Lookup("count")
	require.NotNil(t, countFlag)
	assert.Equal(t, "0", countFlag.DefValue)

	fileFlag := f.Lookup("file")
	require.NotNil(t, fileFlag)
	assert.Empty(t, fileFlag.DefValue)

	workersFlag := f.Lookup("workers")
	require.NotNil(t, workersFlag)
	assert.Equal(t, fmt.Sprintf("%d", runtime.NumCPU()), workersFlag.DefValue)
}

func TestKeygenCmd_RequiredFlags_MissingFormat(t *testing.T) {
	resetKeygenGlobals(t)
	dir := t.TempDir()
	outFile := filepath.Join(dir, "out.txt")

	buf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetErr(buf)
	rootCmd.SetArgs([]string{"keygen", "--count", "1", "--file", outFile})

	err := rootCmd.Execute()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "format")
}

func TestKeygenCmd_RequiredFlags_MissingCount(t *testing.T) {
	resetKeygenGlobals(t)
	dir := t.TempDir()
	outFile := filepath.Join(dir, "out.txt")

	buf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetErr(buf)
	rootCmd.SetArgs([]string{"keygen", "--format", "hex", "--file", outFile})

	err := rootCmd.Execute()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "count")
}

func TestKeygenCmd_RequiredFlags_MissingFile(t *testing.T) {
	resetKeygenGlobals(t)

	buf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetErr(buf)
	rootCmd.SetArgs([]string{"keygen", "--format", "hex", "--count", "1"})

	err := rootCmd.Execute()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "file")
}

func TestKeygenCmd_UnknownFormat(t *testing.T) {
	resetKeygenGlobals(t)
	dir := t.TempDir()
	outFile := filepath.Join(dir, "out.txt")

	buf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetErr(buf)
	rootCmd.SetArgs([]string{"keygen", "--format", "invalid", "--count", "1", "--file", outFile})

	err := rootCmd.Execute()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown format")
	assert.Contains(t, err.Error(), "invalid")
}

func TestKeygenCmd_CountZero(t *testing.T) {
	resetKeygenGlobals(t)
	dir := t.TempDir()
	outFile := filepath.Join(dir, "out.txt")

	buf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetErr(buf)
	rootCmd.SetArgs([]string{"keygen", "--format", "hex", "--count", "0", "--file", outFile})

	err := rootCmd.Execute()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "--count")
}

func TestKeygenCmd_WorkersZero(t *testing.T) {
	resetKeygenGlobals(t)
	dir := t.TempDir()
	outFile := filepath.Join(dir, "out.txt")

	buf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetErr(buf)
	rootCmd.SetArgs([]string{"keygen", "--format", "hex", "--count", "1", "--file", outFile, "--workers", "0"})

	err := rootCmd.Execute()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "--workers")
}

func TestKeygenCmd_Hex(t *testing.T) {
	resetKeygenGlobals(t)
	dir := t.TempDir()
	outFile := filepath.Join(dir, "hex_keys.txt")

	buf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetErr(new(bytes.Buffer))
	rootCmd.SetArgs([]string{"keygen", "--format", "hex", "--count", "10", "--file", outFile, "--workers", "1"})

	err := rootCmd.Execute()
	require.NoError(t, err)

	lines := readLines(t, outFile)
	assert.Len(t, lines, 10)
	for _, line := range lines {
		assert.Len(t, line, 64, "hex key should be 64 hex chars")
		assert.Regexp(t, `^[0-9a-f]{64}$`, line, "hex key should be lowercase hex")
	}
}

func TestKeygenCmd_WIF(t *testing.T) {
	resetKeygenGlobals(t)
	dir := t.TempDir()
	outFile := filepath.Join(dir, "wif_keys.txt")

	buf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetErr(new(bytes.Buffer))
	rootCmd.SetArgs([]string{"keygen", "--format", "wif", "--count", "10", "--file", outFile, "--workers", "1"})

	err := rootCmd.Execute()
	require.NoError(t, err)

	lines := readLines(t, outFile)
	assert.Len(t, lines, 10)
	for _, line := range lines {
		// Compressed WIF starts with K or L
		assert.True(t, strings.HasPrefix(line, "K") || strings.HasPrefix(line, "L"),
			"compressed WIF should start with K or L, got: %s", line)
		assert.Greater(t, len(line), 50, "WIF key should be at least 51 chars")
		privKey, err := wallet.ParseWIF(line)
		require.NoError(t, err, "WIF should decode without error")
		assert.Len(t, privKey, 32, "decoded private key should be 32 bytes")
	}
}

func TestKeygenCmd_WIFUncompressed(t *testing.T) {
	resetKeygenGlobals(t)
	dir := t.TempDir()
	outFile := filepath.Join(dir, "wif_old.txt")

	buf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetErr(new(bytes.Buffer))
	rootCmd.SetArgs([]string{"keygen", "--format", "wif-uncompressed", "--count", "10", "--file", outFile, "--workers", "1"})

	err := rootCmd.Execute()
	require.NoError(t, err)

	lines := readLines(t, outFile)
	assert.Len(t, lines, 10)
	for _, line := range lines {
		// Uncompressed WIF starts with 5
		assert.True(t, strings.HasPrefix(line, "5"),
			"uncompressed WIF should start with 5, got: %s", line)
		assert.Greater(t, len(line), 50, "WIF key should be at least 51 chars")
		privKey, err := wallet.ParseWIF(line)
		require.NoError(t, err, "WIF should decode without error")
		assert.Len(t, privKey, 32, "decoded private key should be 32 bytes")
	}
}

func TestKeygenCmd_Mnemonic12(t *testing.T) {
	resetKeygenGlobals(t)
	dir := t.TempDir()
	outFile := filepath.Join(dir, "mnemonics12.txt")

	buf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetErr(new(bytes.Buffer))
	rootCmd.SetArgs([]string{"keygen", "--format", "mnemonic12", "--count", "5", "--file", outFile, "--workers", "1"})

	err := rootCmd.Execute()
	require.NoError(t, err)

	lines := readLines(t, outFile)
	assert.Len(t, lines, 5)
	for _, line := range lines {
		words := strings.Fields(line)
		assert.Len(t, words, 12, "mnemonic12 should have 12 words")
		assert.NoError(t, wallet.ValidateMnemonic(line), "mnemonic should be BIP39-valid")
	}
}

func TestKeygenCmd_Mnemonic24(t *testing.T) {
	resetKeygenGlobals(t)
	dir := t.TempDir()
	outFile := filepath.Join(dir, "mnemonics24.txt")

	buf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetErr(new(bytes.Buffer))
	rootCmd.SetArgs([]string{"keygen", "--format", "mnemonic24", "--count", "5", "--file", outFile, "--workers", "1"})

	err := rootCmd.Execute()
	require.NoError(t, err)

	lines := readLines(t, outFile)
	assert.Len(t, lines, 5)
	for _, line := range lines {
		words := strings.Fields(line)
		assert.Len(t, words, 24, "mnemonic24 should have 24 words")
		assert.NoError(t, wallet.ValidateMnemonic(line), "mnemonic should be BIP39-valid")
	}
}

func TestKeygenCmd_MultipleWorkers(t *testing.T) {
	resetKeygenGlobals(t)
	dir := t.TempDir()
	outFile := filepath.Join(dir, "hex_multi.txt")

	buf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetErr(new(bytes.Buffer))
	rootCmd.SetArgs([]string{"keygen", "--format", "hex", "--count", "50", "--file", outFile, "--workers", "4"})

	err := rootCmd.Execute()
	require.NoError(t, err)

	lines := readLines(t, outFile)
	assert.Len(t, lines, 50, "should generate exactly 50 keys with 4 workers")
}

func TestKeygenCmd_OutputContainsSuccess(t *testing.T) {
	resetKeygenGlobals(t)
	dir := t.TempDir()
	outFile := filepath.Join(dir, "keys.txt")

	buf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetErr(new(bytes.Buffer))
	rootCmd.SetArgs([]string{"keygen", "--format", "hex", "--count", "3", "--file", outFile, "--workers", "1"})

	err := rootCmd.Execute()
	require.NoError(t, err)

	output := buf.String()
	assert.Contains(t, output, "Generated")
	assert.Contains(t, output, "3")
}

func TestKeygenCmd_NonexistentOutputDir(t *testing.T) {
	resetKeygenGlobals(t)

	buf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetErr(buf)
	rootCmd.SetArgs([]string{"keygen", "--format", "hex", "--count", "1", "--file", "/nonexistent/dir/keys.txt"})

	err := rootCmd.Execute()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "create output file")
}

func TestKeygenCmd_Help(t *testing.T) {
	buf := new(bytes.Buffer)
	keygenCmd.SetOut(buf)
	require.NoError(t, keygenCmd.Help())

	output := buf.String()
	assert.Contains(t, output, "keygen")
	assert.Contains(t, output, "--format")
	assert.Contains(t, output, "--count")
	assert.Contains(t, output, "--file")
	assert.Contains(t, output, "--workers")
}

func TestGenerateOneKey_AllFormats(t *testing.T) {
	t.Parallel()

	formats := []struct {
		format   string
		validate func(t *testing.T, key string)
	}{
		{
			format: "hex",
			validate: func(t *testing.T, key string) {
				t.Helper()
				assert.Len(t, key, 64)
				assert.Regexp(t, `^[0-9a-f]{64}$`, key)
			},
		},
		{
			format: "wif",
			validate: func(t *testing.T, key string) {
				t.Helper()
				assert.True(t, strings.HasPrefix(key, "K") || strings.HasPrefix(key, "L"))
				privKey, err := wallet.ParseWIF(key)
				require.NoError(t, err, "WIF should decode without error")
				assert.Len(t, privKey, 32, "decoded private key should be 32 bytes")
			},
		},
		{
			format: "wif-uncompressed",
			validate: func(t *testing.T, key string) {
				t.Helper()
				assert.True(t, strings.HasPrefix(key, "5"))
				privKey, err := wallet.ParseWIF(key)
				require.NoError(t, err, "WIF should decode without error")
				assert.Len(t, privKey, 32, "decoded private key should be 32 bytes")
			},
		},
		{
			format: "mnemonic12",
			validate: func(t *testing.T, key string) {
				t.Helper()
				assert.Len(t, strings.Fields(key), 12)
				assert.NoError(t, wallet.ValidateMnemonic(key), "mnemonic should be BIP39-valid")
			},
		},
		{
			format: "mnemonic24",
			validate: func(t *testing.T, key string) {
				t.Helper()
				assert.Len(t, strings.Fields(key), 24)
				assert.NoError(t, wallet.ValidateMnemonic(key), "mnemonic should be BIP39-valid")
			},
		},
	}

	for _, tt := range formats {
		t.Run(tt.format, func(t *testing.T) {
			t.Parallel()
			key, err := generateOneKey(tt.format)
			require.NoError(t, err)
			assert.NotEmpty(t, key)
			tt.validate(t, key)
		})
	}
}

func TestGenerateOneKey_UnknownFormat(t *testing.T) {
	t.Parallel()
	_, err := generateOneKey("unknown")
	require.Error(t, err)
}

func BenchmarkGenerateOneKey(b *testing.B) {
	for _, format := range []string{"hex", "wif", "wif-uncompressed", "mnemonic12", "mnemonic24"} {
		b.Run(format, func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				_, _ = generateOneKey(format)
			}
		})
	}
}

func TestValidateKeygenFlags(t *testing.T) {
	resetKeygenGlobals(t)

	tests := []struct {
		name    string
		format  string
		count   int
		workers int
		wantErr string
	}{
		{"hex valid", "hex", 1, 1, ""},
		{"wif valid", "wif", 1, 1, ""},
		{"wif-uncompressed valid", "wif-uncompressed", 1, 1, ""},
		{"mnemonic12 valid", "mnemonic12", 1, 1, ""},
		{"mnemonic24 valid", "mnemonic24", 1, 1, ""},
		{"invalid format", "invalid", 1, 1, "unknown format"},
		{"count zero", "hex", 0, 1, errKeygenCountMin.Error()},
		{"count negative", "hex", -1, 1, errKeygenCountMin.Error()},
		{"workers zero", "hex", 1, 0, errKeygenWorkersMin.Error()},
		{"workers negative", "hex", 1, -1, errKeygenWorkersMin.Error()},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			keygenFormat = tt.format
			keygenCount = tt.count
			keygenWorkers = tt.workers

			err := validateKeygenFlags()
			if tt.wantErr == "" {
				assert.NoError(t, err)
			} else {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantErr)
			}
		})
	}
}

func TestKeygenCmd_NegativeCount(t *testing.T) {
	resetKeygenGlobals(t)
	dir := t.TempDir()
	outFile := filepath.Join(dir, "neg.txt")

	buf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetErr(buf)
	rootCmd.SetArgs([]string{"keygen", "--count", "-1", "--format", "hex", "--file", outFile})

	err := rootCmd.Execute()
	require.Error(t, err)
}

func TestKeygenCmd_KeysAreUnique(t *testing.T) {
	resetKeygenGlobals(t)
	dir := t.TempDir()
	outFile := filepath.Join(dir, "unique.txt")

	buf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetErr(new(bytes.Buffer))
	rootCmd.SetArgs([]string{"keygen", "--format", "hex", "--count", "200", "--file", outFile, "--workers", "4"})

	err := rootCmd.Execute()
	require.NoError(t, err)

	lines := readLines(t, outFile)
	assert.Len(t, lines, 200)

	seen := make(map[string]struct{}, len(lines))
	for _, line := range lines {
		seen[line] = struct{}{}
	}
	assert.Len(t, seen, 200, "all 200 keys should be unique")
}

// readLines reads a file and returns non-empty lines.
func readLines(t *testing.T, path string) []string {
	t.Helper()
	f, err := os.Open(path) //nolint:gosec // G304: test helper, path is controlled
	require.NoError(t, err)
	defer func() { _ = f.Close() }()

	var lines []string
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		if line != "" {
			lines = append(lines, line)
		}
	}
	require.NoError(t, scanner.Err())
	return lines
}
