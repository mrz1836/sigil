package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/pflag"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mrz1836/sigil/internal/addresslookup"
	"github.com/mrz1836/sigil/internal/discovery"
	"github.com/mrz1836/sigil/internal/wallet"
)

// knownTestWIF is a valid WIF private key for testing (uncompressed format).
// The compressed P2PKH address derived from this key is 1LoVGDgRs9hTfTNJNuXKSpywcbdvwRXpmK.
const (
	knownTestWIF     = "5HueCGU8rMjxEXxiPuD5BDku4MkFqeZyd4dZ1jvhTVqvbTLvyTJ"
	knownTestAddress = "1LoVGDgRs9hTfTNJNuXKSpywcbdvwRXpmK"
)

// knownTestHex is the hex private key for the same key as above.
const knownTestHex = "0c28fca386c7a227600b2fe50b7cae11ec86d3bf1fbe471be89827e19d72aa1d"

// resetLookupGlobals resets all lookup command globals and Cobra flag state.
func resetLookupGlobals(t *testing.T) {
	t.Helper()
	origInput := lookupInput
	origKeysFile := lookupKeysFile
	origFormat := lookupFormat
	origPassphrase := lookupPassphrase
	origFile := lookupFile
	origGap := lookupGap
	origScheme := lookupScheme
	origWorkers := lookupWorkers
	origOutputFormat := outputFormat
	t.Cleanup(func() {
		lookupInput = origInput
		lookupKeysFile = origKeysFile
		lookupFormat = origFormat
		lookupPassphrase = origPassphrase
		lookupFile = origFile
		lookupGap = origGap
		lookupScheme = origScheme
		lookupWorkers = origWorkers
		outputFormat = origOutputFormat
		// Reset Cobra flag Changed state on cleanup too
		lookupCmd.Flags().VisitAll(func(f *pflag.Flag) {
			f.Changed = false
		})
		rootCmd.PersistentFlags().VisitAll(func(f *pflag.Flag) {
			f.Changed = false
		})
		// Clear rootCmd.SetArgs so subsequent tests using os.Args aren't affected
		rootCmd.SetArgs(nil)
	})
	lookupInput = ""
	lookupKeysFile = ""
	lookupFormat = "auto"
	lookupPassphrase = false
	lookupFile = ""
	lookupGap = 20
	lookupScheme = ""
	lookupWorkers = 1
	outputFormat = "auto"

	// Reset Cobra flag "Changed" state so MarkFlagsOneRequired / MutuallyExclusive work
	lookupCmd.Flags().VisitAll(func(f *pflag.Flag) {
		f.Changed = false
	})
	rootCmd.PersistentFlags().VisitAll(func(f *pflag.Flag) {
		f.Changed = false
	})
}

// createTestAddressFile creates a CSV address file with the given addresses and balances.
func createTestAddressFile(t *testing.T, entries map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "addresses.csv")

	var sb strings.Builder
	sb.WriteString("address,balance\n")
	for addr, bal := range entries {
		sb.WriteString(addr)
		sb.WriteString(",")
		sb.WriteString(bal)
		sb.WriteString("\n")
	}

	err := os.WriteFile(path, []byte(sb.String()), 0o600)
	require.NoError(t, err)
	return path
}

// createTestKeysFile creates a keys file with one key per line.
func createTestKeysFile(t *testing.T, keys []string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "keys.txt")
	content := strings.Join(keys, "\n") + "\n"
	err := os.WriteFile(path, []byte(content), 0o600)
	require.NoError(t, err)
	return path
}

func TestLookupCmd_Registration(t *testing.T) {
	t.Parallel()

	// Verify lookup command is registered under root
	found := false
	for _, cmd := range rootCmd.Commands() {
		if cmd.Name() == "lookup" {
			found = true
			assert.Equal(t, "utility", cmd.GroupID)
			assert.NotEmpty(t, cmd.Short)
			assert.NotEmpty(t, cmd.Long)
			assert.NotEmpty(t, cmd.Example)
			break
		}
	}
	assert.True(t, found, "lookup command should be registered")
}

func TestLookupCmd_FlagDefaults(t *testing.T) {
	t.Parallel()

	f := lookupCmd.Flags()

	inputFlag := f.Lookup("input")
	require.NotNil(t, inputFlag)
	assert.Empty(t, inputFlag.DefValue)

	keysFileFlag := f.Lookup("keys-file")
	require.NotNil(t, keysFileFlag)
	assert.Empty(t, keysFileFlag.DefValue)

	formatFlag := f.Lookup("format")
	require.NotNil(t, formatFlag)
	assert.Equal(t, "auto", formatFlag.DefValue)

	gapFlag := f.Lookup("gap")
	require.NotNil(t, gapFlag)
	assert.Equal(t, "20", gapFlag.DefValue)

	fileFlag := f.Lookup("file")
	require.NotNil(t, fileFlag)

	workersFlag := f.Lookup("workers")
	require.NotNil(t, workersFlag)
}

func TestLookupCmd_RequiresFileFlag(t *testing.T) {
	resetLookupGlobals(t)
	buf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetErr(buf)
	rootCmd.SetArgs([]string{"lookup", "--input", "test"})

	err := rootCmd.Execute()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "file")
}

func TestLookupCmd_RequiresInputOrKeysFile(t *testing.T) {
	resetLookupGlobals(t)
	addrFile := createTestAddressFile(t, map[string]string{
		"1ABC": "100",
	})

	buf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetErr(buf)
	rootCmd.SetArgs([]string{"lookup", "--file", addrFile})

	err := rootCmd.Execute()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "--input or --keys-file")
}

func TestLookupCmd_SingleMode_WIF_Match(t *testing.T) {
	resetLookupGlobals(t)
	addrFile := createTestAddressFile(t, map[string]string{
		knownTestAddress: "12345.67",
	})

	buf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetErr(new(bytes.Buffer))
	rootCmd.SetArgs([]string{"lookup", "--input", knownTestWIF, "--file", addrFile, "-o", "text"})

	err := rootCmd.Execute()
	require.NoError(t, err)

	output := buf.String()
	assert.Contains(t, output, "MATCH")
	assert.Contains(t, output, knownTestAddress)
	assert.Contains(t, output, "12345.67")
}

func TestLookupCmd_SingleMode_WIF_NoMatch(t *testing.T) {
	resetLookupGlobals(t)
	addrFile := createTestAddressFile(t, map[string]string{
		"1SomeOtherAddress": "100",
	})

	buf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetErr(new(bytes.Buffer))
	rootCmd.SetArgs([]string{"lookup", "--input", knownTestWIF, "--file", addrFile, "-o", "text"})

	err := rootCmd.Execute()
	require.NoError(t, err)

	output := buf.String()
	assert.Contains(t, output, "No matches found")
}

func TestLookupCmd_SingleMode_Hex_Match(t *testing.T) {
	resetLookupGlobals(t)
	addrFile := createTestAddressFile(t, map[string]string{
		knownTestAddress: "99.99",
	})

	buf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetErr(new(bytes.Buffer))
	rootCmd.SetArgs([]string{
		"lookup",
		"--input", knownTestHex,
		"--format", "hex",
		"--file", addrFile,
		"-o", "text",
	})

	err := rootCmd.Execute()
	require.NoError(t, err)

	output := buf.String()
	assert.Contains(t, output, "MATCH")
	assert.Contains(t, output, knownTestAddress)
}

func TestLookupCmd_SingleMode_JSON_Output(t *testing.T) {
	resetLookupGlobals(t)
	addrFile := createTestAddressFile(t, map[string]string{
		knownTestAddress: "500.00",
	})

	buf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetErr(new(bytes.Buffer))
	rootCmd.SetArgs([]string{
		"lookup",
		"--input", knownTestWIF,
		"--file", addrFile,
		"-o", "json",
	})

	err := rootCmd.Execute()
	require.NoError(t, err)

	var result lookupOutput
	err = json.Unmarshal(buf.Bytes(), &result)
	require.NoError(t, err)

	assert.Equal(t, int64(1), result.Stats.KeysProcessed)
	// P2PKH address 1... matches both BTC and BCH (same version byte 0x00)
	assert.GreaterOrEqual(t, result.Stats.MatchesFound, 1)
	require.GreaterOrEqual(t, len(result.Results), 1)
	assert.Equal(t, knownTestAddress, result.Results[0].Address)
	assert.Equal(t, "500.00", result.Results[0].Balance)
	assert.NotEmpty(t, result.Results[0].Format)
}

func TestLookupCmd_BatchMode_WIF(t *testing.T) {
	resetLookupGlobals(t)
	addrFile := createTestAddressFile(t, map[string]string{
		knownTestAddress: "42.00",
	})

	keysFile := createTestKeysFile(t, []string{
		knownTestWIF,
		"# this is a comment",
		"",
		"5JNotARealWIFKeyButItWillBeSkippedAnyway",
	})

	buf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetErr(new(bytes.Buffer))
	rootCmd.SetArgs([]string{
		"lookup",
		"--keys-file", keysFile,
		"--file", addrFile,
		"--workers", "1",
		"-o", "text",
	})

	err := rootCmd.Execute()
	require.NoError(t, err)

	output := buf.String()
	assert.Contains(t, output, "MATCH")
	assert.Contains(t, output, knownTestAddress)
	assert.Contains(t, output, "Done.")
}

func TestLookupCmd_BatchMode_JSON(t *testing.T) {
	resetLookupGlobals(t)
	addrFile := createTestAddressFile(t, map[string]string{
		knownTestAddress: "10.00",
	})

	keysFile := createTestKeysFile(t, []string{knownTestWIF})

	buf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetErr(new(bytes.Buffer))
	rootCmd.SetArgs([]string{
		"lookup",
		"--keys-file", keysFile,
		"--file", addrFile,
		"--workers", "1",
		"-o", "json",
	})

	err := rootCmd.Execute()
	require.NoError(t, err)

	var result lookupOutput
	err = json.Unmarshal(buf.Bytes(), &result)
	require.NoError(t, err)

	// P2PKH address 1... matches both BTC and BCH (same version byte 0x00)
	assert.GreaterOrEqual(t, result.Stats.MatchesFound, 1)
	assert.Positive(t, result.Stats.KeysProcessed)
	require.GreaterOrEqual(t, len(result.Results), 1)
	assert.Equal(t, knownTestAddress, result.Results[0].Address)
	assert.NotEmpty(t, result.Results[0].Format)
}

func TestLookupCmd_BatchMode_NoMatches(t *testing.T) {
	resetLookupGlobals(t)
	addrFile := createTestAddressFile(t, map[string]string{
		"1SomeRandomAddress": "100",
	})

	keysFile := createTestKeysFile(t, []string{knownTestWIF})

	buf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetErr(new(bytes.Buffer))
	rootCmd.SetArgs([]string{
		"lookup",
		"--keys-file", keysFile,
		"--file", addrFile,
		"--workers", "1",
		"-o", "text",
	})

	err := rootCmd.Execute()
	require.NoError(t, err)

	output := buf.String()
	assert.Contains(t, output, "Found 0 match")
}

func TestLookupCmd_NonexistentAddressFile(t *testing.T) {
	resetLookupGlobals(t)
	buf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetErr(buf)
	rootCmd.SetArgs([]string{
		"lookup",
		"--input", knownTestWIF,
		"--file", "/nonexistent/addresses.csv",
	})

	err := rootCmd.Execute()
	assert.Error(t, err)
}

func TestLookupCmd_NonexistentKeysFile(t *testing.T) {
	resetLookupGlobals(t)
	addrFile := createTestAddressFile(t, map[string]string{
		"1ABC": "100",
	})

	buf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetErr(buf)
	rootCmd.SetArgs([]string{
		"lookup",
		"--keys-file", "/nonexistent/keys.txt",
		"--file", addrFile,
	})

	err := rootCmd.Execute()
	assert.Error(t, err)
}

func TestLookupCmd_MutuallyExclusive_InputAndKeysFile(t *testing.T) {
	resetLookupGlobals(t)
	addrFile := createTestAddressFile(t, map[string]string{
		"1ABC": "100",
	})

	buf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetErr(buf)
	rootCmd.SetArgs([]string{
		"lookup",
		"--input", "test",
		"--keys-file", "keys.txt",
		"--file", addrFile,
	})

	err := rootCmd.Execute()
	assert.Error(t, err)
}

func TestFormatCount(t *testing.T) {
	t.Parallel()
	tests := []struct {
		input    int
		expected string
	}{
		{0, "0"},
		{1, "1"},
		{999, "999"},
		{1000, "1,000"},
		{1234, "1,234"},
		{12345, "12,345"},
		{123456, "123,456"},
		{1234567, "1,234,567"},
		{55792054, "55,792,054"},
	}

	for _, tt := range tests {
		result := formatCount(tt.input)
		assert.Equal(t, tt.expected, result)
	}
}

func TestDetectFormat(t *testing.T) {
	t.Parallel()

	// Save and restore the global
	origFormat := lookupFormat
	t.Cleanup(func() { lookupFormat = origFormat })

	lookupFormat = "wif"
	assert.Equal(t, "wif", detectFormat("anything").String())

	lookupFormat = "hex"
	assert.Equal(t, "hex", detectFormat("anything").String())

	lookupFormat = "mnemonic"
	assert.Equal(t, "mnemonic", detectFormat("anything").String())

	lookupFormat = "auto"
	assert.Equal(t, "wif", detectFormat(knownTestWIF).String())
	assert.Equal(t, "hex", detectFormat(knownTestHex).String())
}

// Known addresses for the test WIF key (0c28fca...):
const (
	knownTestP2SH      = "3D9iyFHi1Zs9KoyynUfrL82rGhJfYTfSG4"
	knownTestBech32    = "bc1qmy63mjadtw8nhzl69ukdepwzsyvv4yex5qlmkd"
	knownTestCashAddr  = "qrvn28wt44dc7wutlghjehy9c2q33j5nyctecw50xm"
	knownTestLTCP2PKH  = "Lf2SXRzFwowWvG4TZ3Wcir3hpp1D6zsqGn"
	knownTestLTCP2SH   = "MKMsH8hfxgia8KFstMfC9mHFbPu7UgrWfo"
	knownTestLTCBech   = "ltc1qmy63mjadtw8nhzl69ukdepwzsyvv4yexsu9lwa"
	knownTestDOGEP2PKH = "DQwaoUd5AZbkCTYu7VWszb9YVjNEFtT2DQ"
)

// createTestAddressDir creates a directory structure with address files in subdirectories.
func createTestAddressDir(t *testing.T, files map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	for name, content := range files {
		path := filepath.Join(dir, name)
		require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o750))
		require.NoError(t, os.WriteFile(path, []byte(content), 0o600))
	}
	return dir
}

func TestLookupCmd_SingleMode_P2SH_Match(t *testing.T) {
	resetLookupGlobals(t)
	addrFile := createTestAddressFile(t, map[string]string{
		knownTestP2SH: "777.00",
	})

	buf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetErr(new(bytes.Buffer))
	rootCmd.SetArgs([]string{"lookup", "--input", knownTestWIF, "--file", addrFile, "-o", "text"})

	err := rootCmd.Execute()
	require.NoError(t, err)

	output := buf.String()
	assert.Contains(t, output, "MATCH")
	assert.Contains(t, output, knownTestP2SH)
	assert.Contains(t, output, "777.00")
	assert.Contains(t, output, "P2SH-P2WPKH")
}

func TestLookupCmd_SingleMode_Bech32_Match(t *testing.T) {
	resetLookupGlobals(t)
	addrFile := createTestAddressFile(t, map[string]string{
		knownTestBech32: "888.00",
	})

	buf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetErr(new(bytes.Buffer))
	rootCmd.SetArgs([]string{"lookup", "--input", knownTestWIF, "--file", addrFile, "-o", "text"})

	err := rootCmd.Execute()
	require.NoError(t, err)

	output := buf.String()
	assert.Contains(t, output, "MATCH")
	assert.Contains(t, output, knownTestBech32)
	assert.Contains(t, output, "888.00")
	assert.Contains(t, output, "Bech32")
}

func TestLookupCmd_SingleMode_CashAddr_Match(t *testing.T) {
	resetLookupGlobals(t)
	addrFile := createTestAddressFile(t, map[string]string{
		knownTestCashAddr: "999.00",
	})

	buf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetErr(new(bytes.Buffer))
	rootCmd.SetArgs([]string{"lookup", "--input", knownTestWIF, "--file", addrFile, "-o", "text"})

	err := rootCmd.Execute()
	require.NoError(t, err)

	output := buf.String()
	assert.Contains(t, output, "MATCH")
	assert.Contains(t, output, knownTestCashAddr)
	assert.Contains(t, output, "999.00")
	assert.Contains(t, output, "CashAddr")
}

func TestLookupCmd_SingleMode_LTC_Match(t *testing.T) {
	resetLookupGlobals(t)
	addrFile := createTestAddressFile(t, map[string]string{
		knownTestLTCP2PKH: "111.00",
	})

	buf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetErr(new(bytes.Buffer))
	rootCmd.SetArgs([]string{"lookup", "--input", knownTestWIF, "--file", addrFile, "-o", "text"})

	err := rootCmd.Execute()
	require.NoError(t, err)

	output := buf.String()
	assert.Contains(t, output, "MATCH")
	assert.Contains(t, output, knownTestLTCP2PKH)
	assert.Contains(t, output, "LTC P2PKH")
}

func TestLookupCmd_SingleMode_LTC_Bech32_Match(t *testing.T) {
	resetLookupGlobals(t)
	addrFile := createTestAddressFile(t, map[string]string{
		knownTestLTCBech: "222.00",
	})

	buf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetErr(new(bytes.Buffer))
	rootCmd.SetArgs([]string{"lookup", "--input", knownTestWIF, "--file", addrFile, "-o", "text"})

	err := rootCmd.Execute()
	require.NoError(t, err)

	output := buf.String()
	assert.Contains(t, output, "MATCH")
	assert.Contains(t, output, knownTestLTCBech)
	assert.Contains(t, output, "LTC Bech32")
}

func TestLookupCmd_SingleMode_DOGE_Match(t *testing.T) {
	resetLookupGlobals(t)
	addrFile := createTestAddressFile(t, map[string]string{
		knownTestDOGEP2PKH: "333.00",
	})

	buf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetErr(new(bytes.Buffer))
	rootCmd.SetArgs([]string{"lookup", "--input", knownTestWIF, "--file", addrFile, "-o", "text"})

	err := rootCmd.Execute()
	require.NoError(t, err)

	output := buf.String()
	assert.Contains(t, output, "MATCH")
	assert.Contains(t, output, knownTestDOGEP2PKH)
	assert.Contains(t, output, "DOGE P2PKH")
}

func TestLookupCmd_SingleMode_MultiFormat_AllMatch(t *testing.T) {
	resetLookupGlobals(t)

	// Put all address formats for the same key into the dataset
	addrFile := createTestAddressFile(t, map[string]string{
		knownTestAddress:   "100.00",
		knownTestP2SH:      "200.00",
		knownTestBech32:    "300.00",
		knownTestCashAddr:  "400.00",
		knownTestLTCP2PKH:  "500.00",
		knownTestLTCP2SH:   "600.00",
		knownTestLTCBech:   "700.00",
		knownTestDOGEP2PKH: "800.00",
	})

	buf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetErr(new(bytes.Buffer))
	rootCmd.SetArgs([]string{
		"lookup",
		"--input", knownTestWIF,
		"--file", addrFile,
		"-o", "json",
	})

	err := rootCmd.Execute()
	require.NoError(t, err)

	var result lookupOutput
	err = json.Unmarshal(buf.Bytes(), &result)
	require.NoError(t, err)

	// Should find matches for all formats
	// BTC: P2PKH, P2SH, Bech32 (3)
	// LTC: P2PKH, P2SH, Bech32 (3)
	// BCH: P2PKH (same as BTC), CashAddr (1 unique = CashAddr)
	// DOGE: P2PKH (1)
	// Total: 8 unique format matches (BTC P2PKH + BCH P2PKH are separate hits on the same address)
	assert.GreaterOrEqual(t, len(result.Results), 8, "expected at least 8 address format matches")

	// Verify each format is represented
	foundFormats := make(map[string]bool)
	for _, r := range result.Results {
		foundFormats[r.Format] = true
	}
	assert.True(t, foundFormats["BTC P2PKH"], "missing BTC P2PKH")
	assert.True(t, foundFormats["BTC P2SH-P2WPKH"], "missing BTC P2SH-P2WPKH")
	assert.True(t, foundFormats["BTC Bech32"], "missing BTC Bech32")
	assert.True(t, foundFormats["LTC P2PKH"], "missing LTC P2PKH")
	assert.True(t, foundFormats["LTC P2SH-P2WPKH"], "missing LTC P2SH-P2WPKH")
	assert.True(t, foundFormats["LTC Bech32"], "missing LTC Bech32")
	assert.True(t, foundFormats["BCH CashAddr"], "missing BCH CashAddr")
	assert.True(t, foundFormats["DOGE P2PKH"], "missing DOGE P2PKH")
}

func TestLookupCmd_DirectoryLoading(t *testing.T) {
	resetLookupGlobals(t)

	dir := createTestAddressDir(t, map[string]string{
		"btc/btc-addresses.tsv": "address\tbalance\n" + knownTestBech32 + "\t1000.00\n",
		"ltc/ltc-addresses.csv": "address,balance\n" + knownTestLTCBech + ",2000.00\n",
	})

	buf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetErr(new(bytes.Buffer))
	rootCmd.SetArgs([]string{
		"lookup",
		"--input", knownTestWIF,
		"--file", dir,
		"-o", "json",
	})

	err := rootCmd.Execute()
	require.NoError(t, err)

	var result lookupOutput
	err = json.Unmarshal(buf.Bytes(), &result)
	require.NoError(t, err)

	// Should find both bech32 addresses from separate subdirectory files
	assert.GreaterOrEqual(t, len(result.Results), 2)

	foundAddresses := make(map[string]bool)
	for _, r := range result.Results {
		foundAddresses[r.Address] = true
	}
	assert.True(t, foundAddresses[knownTestBech32], "should find BTC bech32 from btc/ subdir")
	assert.True(t, foundAddresses[knownTestLTCBech], "should find LTC bech32 from ltc/ subdir")
}

func TestLookupCmd_DirectoryLoading_EmptyDir(t *testing.T) {
	resetLookupGlobals(t)

	dir := t.TempDir() // empty directory

	buf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetErr(new(bytes.Buffer))
	rootCmd.SetArgs([]string{
		"lookup",
		"--input", knownTestWIF,
		"--file", dir,
		"-o", "text",
	})

	err := rootCmd.Execute()
	require.NoError(t, err)

	output := buf.String()
	assert.Contains(t, output, "No matches found")
}

func TestLookupCmd_BatchMode_MultiFormat(t *testing.T) {
	resetLookupGlobals(t)

	// Use a bech32 address that only matches via segwit derivation
	addrFile := createTestAddressFile(t, map[string]string{
		knownTestBech32: "555.00",
	})

	keysFile := createTestKeysFile(t, []string{knownTestHex})

	buf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetErr(new(bytes.Buffer))
	rootCmd.SetArgs([]string{
		"lookup",
		"--keys-file", keysFile,
		"--format", "hex",
		"--file", addrFile,
		"--workers", "1",
		"-o", "json",
	})

	err := rootCmd.Execute()
	require.NoError(t, err)

	var result lookupOutput
	err = json.Unmarshal(buf.Bytes(), &result)
	require.NoError(t, err)

	require.Len(t, result.Results, 1)
	assert.Equal(t, knownTestBech32, result.Results[0].Address)
	assert.Equal(t, "BTC Bech32", result.Results[0].Format)
}

func TestLookupCmd_WorkersZero(t *testing.T) {
	resetLookupGlobals(t)
	addrFile := createTestAddressFile(t, map[string]string{"1ABC": "100"})
	keysFile := createTestKeysFile(t, []string{knownTestWIF})

	buf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetErr(buf)
	rootCmd.SetArgs([]string{
		"lookup",
		"--keys-file", keysFile,
		"--file", addrFile,
		"--workers", "0",
	})

	err := rootCmd.Execute()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "at least 1")
}

func TestLookupCmd_UnknownScheme(t *testing.T) {
	resetLookupGlobals(t)
	addrFile := createTestAddressFile(t, map[string]string{knownTestAddress: "1.00"})

	buf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetErr(buf)
	rootCmd.SetArgs([]string{
		"lookup",
		"--input", knownTestWIF,
		"--file", addrFile,
		"--scheme", "Nonexistent Scheme",
	})

	err := rootCmd.Execute()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown scheme")
	assert.Contains(t, err.Error(), "Nonexistent Scheme")
	assert.Contains(t, err.Error(), "available:")
}

func TestLookupCmd_GapTooLarge(t *testing.T) {
	resetLookupGlobals(t)
	addrFile := createTestAddressFile(t, map[string]string{knownTestAddress: "1.00"})

	buf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetErr(buf)
	rootCmd.SetArgs([]string{
		"lookup",
		"--input", knownTestWIF,
		"--file", addrFile,
		"--gap", "10001",
	})

	err := rootCmd.Execute()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "--gap")
}

func TestLookupCmd_Help(t *testing.T) {
	buf := new(bytes.Buffer)
	lookupCmd.SetOut(buf)
	require.NoError(t, lookupCmd.Help())

	output := buf.String()
	assert.Contains(t, output, "lookup")
	assert.Contains(t, output, "--input")
	assert.Contains(t, output, "--keys-file")
	assert.Contains(t, output, "--format")
	assert.Contains(t, output, "--file")
	assert.Contains(t, output, "--gap")
	assert.Contains(t, output, "--scheme")
	assert.Contains(t, output, "--workers")
	assert.Contains(t, output, "Examples:")
}

// testMnemonic is the standard BIP39 test vector.
const testMnemonic = "abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon about"

// precomputeMnemonicAddress derives a known address from the test mnemonic for a given coin type.
func precomputeMnemonicAddress(t *testing.T, coinType, account, change, index uint32) string {
	t.Helper()
	seed, err := wallet.MnemonicToSeed(testMnemonic, "")
	require.NoError(t, err)
	addr, _, _, err := wallet.DeriveAddressWithCoinType(seed, coinType, account, change, index)
	require.NoError(t, err)
	return addr
}

func TestLookupCmd_SingleMode_MnemonicMatch(t *testing.T) {
	resetLookupGlobals(t)

	// Pre-compute the BTC P2PKH address at m/44'/0'/0'/0/0
	knownAddr := precomputeMnemonicAddress(t, 0, 0, 0, 0)
	addrFile := createTestAddressFile(t, map[string]string{
		knownAddr: "1000.00",
	})

	buf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	lookupCmd.SetOut(nil) // clear any stale output from prior tests
	rootCmd.SetErr(new(bytes.Buffer))
	rootCmd.SetArgs([]string{
		"lookup",
		"--input", testMnemonic,
		"--file", addrFile,
		"-o", "json",
	})

	err := rootCmd.Execute()
	require.NoError(t, err)

	var result lookupOutput
	err = json.Unmarshal(buf.Bytes(), &result)
	require.NoError(t, err)

	assert.Equal(t, int64(1), result.Stats.KeysProcessed)
	require.GreaterOrEqual(t, len(result.Results), 1)

	// Find the match for our known address
	var found bool
	for _, r := range result.Results {
		if r.Address == knownAddr {
			found = true
			assert.NotEmpty(t, r.Scheme, "mnemonic match should have scheme")
			assert.NotEmpty(t, r.Path, "mnemonic match should have path")
			assert.Equal(t, "1000.00", r.Balance)
			break
		}
	}
	assert.True(t, found, "should find the pre-computed mnemonic address %s", knownAddr)
}

func TestLookupCmd_SingleMode_MnemonicWithScheme(t *testing.T) {
	resetLookupGlobals(t)

	// Pre-compute address for BSV coin type (236)
	knownAddr := precomputeMnemonicAddress(t, 236, 0, 0, 0)
	addrFile := createTestAddressFile(t, map[string]string{
		knownAddr: "2000.00",
	})

	buf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	lookupCmd.SetOut(nil)
	rootCmd.SetErr(new(bytes.Buffer))
	rootCmd.SetArgs([]string{
		"lookup",
		"--input", testMnemonic,
		"--format", "mnemonic",
		"--scheme", "BSV Standard",
		"--file", addrFile,
		"-o", "json",
	})

	err := rootCmd.Execute()
	require.NoError(t, err)

	var result lookupOutput
	err = json.Unmarshal(buf.Bytes(), &result)
	require.NoError(t, err)

	require.GreaterOrEqual(t, len(result.Results), 1)

	var found bool
	for _, r := range result.Results {
		if r.Address == knownAddr {
			found = true
			assert.Equal(t, "BSV Standard", r.Scheme)
			assert.Contains(t, r.Path, "m/44'/236'")
			break
		}
	}
	assert.True(t, found, "should find BSV address with BSV Standard scheme")
}

func TestLookupCmd_BatchMode_MnemonicMultipleWorkers(t *testing.T) {
	resetLookupGlobals(t)

	knownAddr := precomputeMnemonicAddress(t, 0, 0, 0, 0)
	addrFile := createTestAddressFile(t, map[string]string{
		knownAddr: "3000.00",
	})

	keysFile := createTestKeysFile(t, []string{testMnemonic})

	buf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	lookupCmd.SetOut(nil)
	rootCmd.SetErr(new(bytes.Buffer))
	rootCmd.SetArgs([]string{
		"lookup",
		"--keys-file", keysFile,
		"--file", addrFile,
		"--workers", "2",
		"-o", "json",
	})

	err := rootCmd.Execute()
	require.NoError(t, err)

	var result lookupOutput
	err = json.Unmarshal(buf.Bytes(), &result)
	require.NoError(t, err)

	assert.Positive(t, result.Stats.KeysProcessed)
	require.GreaterOrEqual(t, len(result.Results), 1)

	var found bool
	for _, r := range result.Results {
		if r.Address == knownAddr {
			found = true
			break
		}
	}
	assert.True(t, found, "batch mnemonic lookup should find pre-computed address")
}

func TestValidateLookupFlags_GapBoundaries(t *testing.T) {
	resetLookupGlobals(t)

	tests := []struct {
		name    string
		gap     int
		wantErr bool
	}{
		{"gap=0", 0, true},
		{"gap=-1", -1, true},
		{"gap=1", 1, false},
		{"gap=10000", 10000, false},
		{"gap=10001", 10001, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set valid base state
			lookupInput = "test"
			lookupKeysFile = ""
			lookupWorkers = 1
			lookupGap = tt.gap

			err := validateLookupFlags()
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestPrintMatch_AllFields(t *testing.T) {
	t.Parallel()

	buf := new(bytes.Buffer)
	cmd := lookupCmd
	cmd.SetOut(buf)

	m := lookupMatch{
		Address: "1TestAddress",
		Balance: "42.00",
		Format:  "BTC P2PKH",
		KeyLine: 7,
		Scheme:  "BSV Standard",
		Path:    "m/44'/236'/0'/0/0",
	}
	printMatch(cmd, m)

	output := buf.String()
	assert.Contains(t, output, "MATCH")
	assert.Contains(t, output, "1TestAddress")
	assert.Contains(t, output, "42.00")
	assert.Contains(t, output, "BTC P2PKH")
	assert.Contains(t, output, "key_line=7")
	assert.Contains(t, output, `scheme="BSV Standard"`)
	assert.Contains(t, output, "path=m/44'/236'/0'/0/0")
}

func TestDeriveAndLookup_FormatUnknown(t *testing.T) {
	resetLookupGlobals(t)
	lookupFormat = "auto"

	// Create a minimal address set
	addrSet := addresslookup.NewAddressSet(nil)
	schemes := discovery.DefaultSchemes()

	// "not-a-key-or-mnemonic" should be detected as FormatUnknown and return nil
	matches := deriveAndLookup("not-a-key-or-mnemonic", addrSet, schemes, 20, nil)
	assert.Nil(t, matches)
}

func TestDetectFormat_Mnemonic(t *testing.T) {
	origFormat := lookupFormat
	t.Cleanup(func() { lookupFormat = origFormat })

	// Auto-detect mnemonic
	lookupFormat = "auto"
	result := detectFormat(testMnemonic)
	assert.Equal(t, wallet.FormatMnemonic, result, "should auto-detect mnemonic format")
	assert.Equal(t, "mnemonic", result.String())

	// Explicit mnemonic format
	lookupFormat = "mnemonic"
	result = detectFormat("anything")
	assert.Equal(t, wallet.FormatMnemonic, result)
}

func TestValidateLookupFlags_WorkersBoundaries(t *testing.T) {
	resetLookupGlobals(t)

	tests := []struct {
		name    string
		workers int
		wantErr bool
	}{
		{"workers=0", 0, true},
		{"workers=-1", -1, true},
		{"workers=1", 1, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			lookupInput = "test"
			lookupKeysFile = ""
			lookupGap = 20
			lookupWorkers = tt.workers

			err := validateLookupFlags()
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestGetSchemes_ValidScheme(t *testing.T) {
	resetLookupGlobals(t)

	lookupScheme = "BSV Standard"
	schemes, err := getSchemes()
	require.NoError(t, err)
	require.Len(t, schemes, 1)
	assert.Equal(t, "BSV Standard", schemes[0].Name)
	assert.Equal(t, uint32(236), schemes[0].CoinType)
}

func TestGetSchemes_DefaultSchemes(t *testing.T) {
	resetLookupGlobals(t)

	lookupScheme = ""
	schemes, err := getSchemes()
	require.NoError(t, err)
	assert.Greater(t, len(schemes), 1, "default schemes should return multiple schemes")
}

func TestFormatCount_Comprehensive(t *testing.T) {
	t.Parallel()
	// Verify formatting does not panic on edge values
	assert.Equal(t, "0", formatCount(0))
	assert.Equal(t, "100", formatCount(100))
	assert.Equal(t, "999", formatCount(999))
	assert.Equal(t, "1,000", formatCount(1000))
	assert.Equal(t, "1,000,000", formatCount(1000000))
	assert.Equal(t, "999,999,999", formatCount(999999999))
}
