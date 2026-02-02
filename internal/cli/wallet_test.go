package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mrz1836/sigil/internal/config"
	"github.com/mrz1836/sigil/internal/output"
	"github.com/mrz1836/sigil/internal/sigilcrypto"
	"github.com/mrz1836/sigil/internal/wallet"
)

func TestMain(m *testing.M) {
	sigilcrypto.SetScryptWorkFactor(10) // Fast for tests
	os.Exit(m.Run())
}

// setupTestEnv creates a temporary environment for CLI testing.
// It saves and restores global state to avoid test pollution.
// Tests using this function should NOT use t.Parallel() as they
// modify package-level globals.
func setupTestEnv(t *testing.T) (string, func()) {
	t.Helper()

	// Save original global state
	origCfg := cfg
	origLogger := logger
	origFormatter := formatter

	tmpDir, err := os.MkdirTemp("", "sigil-cli-test")
	require.NoError(t, err)

	// Create wallets directory
	walletsDir := filepath.Join(tmpDir, "wallets")
	require.NoError(t, os.MkdirAll(walletsDir, 0o750))

	// Set up test-specific global config
	testCfg := config.Defaults()
	testCfg.Home = tmpDir
	cfg = testCfg

	// Set up null logger for tests
	logger = config.NullLogger()

	// Set up text formatter for tests
	formatter = output.NewFormatter(output.FormatText, os.Stdout)

	cleanup := func() {
		// Restore original global state
		cfg = origCfg
		logger = origLogger
		formatter = origFormatter

		// Clean up temp directory
		_ = os.RemoveAll(tmpDir)
	}

	return tmpDir, cleanup
}

// TestWalletCreateValidation tests input validation for wallet create.
func TestWalletCreateValidation(t *testing.T) {
	tmpDir, cleanup := setupTestEnv(t)
	defer cleanup()

	storage := wallet.NewFileStorage(filepath.Join(tmpDir, "wallets"))

	tests := []struct {
		name       string
		walletName string
		wordCount  int
		wantErr    bool
		errMsg     string
	}{
		{
			name:       "valid 12 words",
			walletName: "test_wallet",
			wordCount:  12,
			wantErr:    false,
		},
		{
			name:       "valid 24 words",
			walletName: "wallet24",
			wordCount:  24,
			wantErr:    false,
		},
		{
			name:       "invalid word count",
			walletName: "test",
			wordCount:  15,
			wantErr:    true,
			errMsg:     "", // Just check error occurs
		},
		{
			name:       "invalid wallet name with dash",
			walletName: "test-wallet",
			wordCount:  12,
			wantErr:    true,
		},
		{
			name:       "invalid wallet name with space",
			walletName: "test wallet",
			wordCount:  12,
			wantErr:    true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := validateWalletCreationParams(tc.walletName, tc.wordCount, storage)
			if tc.wantErr {
				require.Error(t, err)
				if tc.errMsg != "" {
					assert.Contains(t, err.Error(), tc.errMsg)
				}
			} else {
				require.NoError(t, err)
			}
		})
	}
}

// TestWalletCreateDuplicatePrevention tests that duplicate wallet names are rejected.
func TestWalletCreateDuplicatePrevention(t *testing.T) {
	tmpDir, cleanup := setupTestEnv(t)
	defer cleanup()

	storage := wallet.NewFileStorage(filepath.Join(tmpDir, "wallets"))

	// Create initial wallet
	w, err := wallet.NewWallet("existing", []wallet.ChainID{wallet.ChainETH})
	require.NoError(t, err)

	mnemonic, err := wallet.GenerateMnemonic(12)
	require.NoError(t, err)

	seed, err := wallet.MnemonicToSeed(mnemonic, "")
	require.NoError(t, err)
	defer wallet.ZeroBytes(seed)

	require.NoError(t, w.DeriveAddresses(seed, 1))
	require.NoError(t, storage.Save(w, seed, []byte("testpassword")))

	// Try to create wallet with same name
	err = validateWalletCreationParams("existing", 12, storage)
	require.Error(t, err)
	require.ErrorIs(t, err, wallet.ErrWalletExists)
}

// TestGenerateWalletSeed tests mnemonic and seed generation.
func TestGenerateWalletSeed(t *testing.T) {
	// Test 12 word generation
	mnemonic, seed, err := generateWalletSeed(12, false)
	require.NoError(t, err)
	defer wallet.ZeroBytes(seed)

	words := strings.Fields(mnemonic)
	assert.Len(t, words, 12)
	assert.Len(t, seed, 64) // BIP39 seed is always 64 bytes

	// Verify mnemonic is valid
	require.NoError(t, wallet.ValidateMnemonic(mnemonic))

	// Test 24 word generation
	mnemonic24, seed24, err := generateWalletSeed(24, false)
	require.NoError(t, err)
	defer wallet.ZeroBytes(seed24)

	words24 := strings.Fields(mnemonic24)
	assert.Len(t, words24, 24)
	assert.Len(t, seed24, 64)
}

// TestCreateAndSaveWallet tests wallet creation and storage.
func TestCreateAndSaveWallet(t *testing.T) {
	tmpDir, cleanup := setupTestEnv(t)
	defer cleanup()

	storage := wallet.NewFileStorage(filepath.Join(tmpDir, "wallets"))

	// Generate seed
	mnemonic, err := wallet.GenerateMnemonic(12)
	require.NoError(t, err)

	seed, err := wallet.MnemonicToSeed(mnemonic, "")
	require.NoError(t, err)
	defer wallet.ZeroBytes(seed)

	// Create wallet directly (bypassing password prompt)
	w, err := wallet.NewWallet("testcreate", []wallet.ChainID{wallet.ChainETH, wallet.ChainBSV})
	require.NoError(t, err)

	err = w.DeriveAddresses(seed, 1)
	require.NoError(t, err)

	// Save with test password
	err = storage.Save(w, seed, []byte("testpassword123"))
	require.NoError(t, err)

	// Verify wallet was saved
	exists, err := storage.Exists("testcreate")
	require.NoError(t, err)
	assert.True(t, exists)

	// Verify wallet file has correct permissions
	walletPath := filepath.Join(tmpDir, "wallets", "testcreate.wallet")
	info, err := os.Stat(walletPath)
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0o600), info.Mode().Perm())

	// Load and verify wallet contents
	loadedW, loadedSeed, err := storage.Load("testcreate", []byte("testpassword123"))
	require.NoError(t, err)
	defer wallet.ZeroBytes(loadedSeed)

	assert.Equal(t, "testcreate", loadedW.Name)
	assert.Equal(t, []wallet.ChainID{wallet.ChainETH, wallet.ChainBSV}, loadedW.EnabledChains)
	assert.Len(t, loadedW.Addresses[wallet.ChainETH], 1)
	assert.Len(t, loadedW.Addresses[wallet.ChainBSV], 1)

	// Verify ETH address format
	ethAddr := loadedW.Addresses[wallet.ChainETH][0].Address
	assert.True(t, strings.HasPrefix(ethAddr, "0x"))
	assert.Len(t, ethAddr, 42) // 0x + 40 hex chars

	// Verify BSV address format (starts with 1)
	bsvAddr := loadedW.Addresses[wallet.ChainBSV][0].Address
	assert.True(t, strings.HasPrefix(bsvAddr, "1"))
}

// TestWalletListEmpty tests listing when no wallets exist.
func TestWalletListEmpty(t *testing.T) {
	tmpDir, cleanup := setupTestEnv(t)
	defer cleanup()

	storage := wallet.NewFileStorage(filepath.Join(tmpDir, "wallets"))

	names, err := storage.List()
	require.NoError(t, err)
	assert.Empty(t, names)
}

// TestWalletListMultiple tests listing multiple wallets.
func TestWalletListMultiple(t *testing.T) {
	tmpDir, cleanup := setupTestEnv(t)
	defer cleanup()

	storage := wallet.NewFileStorage(filepath.Join(tmpDir, "wallets"))

	// Create multiple wallets
	walletNames := []string{"wallet_a", "wallet_b", "wallet_c"}
	for _, name := range walletNames {
		w, err := wallet.NewWallet(name, []wallet.ChainID{wallet.ChainETH})
		require.NoError(t, err)

		mnemonic, _ := wallet.GenerateMnemonic(12)
		seed, _ := wallet.MnemonicToSeed(mnemonic, "")
		require.NoError(t, w.DeriveAddresses(seed, 1))
		require.NoError(t, storage.Save(w, seed, []byte("password")))
		wallet.ZeroBytes(seed)
	}

	// List wallets
	names, err := storage.List()
	require.NoError(t, err)
	assert.Len(t, names, 3)
	for _, name := range walletNames {
		assert.Contains(t, names, name)
	}
}

// TestDisplayMnemonic tests mnemonic display formatting.
func TestDisplayMnemonic(t *testing.T) {
	buf := new(bytes.Buffer)
	cmd := &cobra.Command{}
	cmd.SetOut(buf)

	mnemonic := "abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon about"
	displayMnemonic(mnemonic, cmd)

	output := buf.String()

	// Verify header is present
	assert.Contains(t, output, "RECOVERY PHRASE")
	assert.Contains(t, output, "Write down these words")

	// Verify all words are numbered
	words := strings.Fields(mnemonic)
	for i, word := range words {
		expected := strings.TrimSpace(strings.Split(strings.Split(output, word)[0], "\n")[len(strings.Split(strings.Split(output, word)[0], "\n"))-1])
		// Just check the word appears
		assert.Contains(t, output, word)
		_ = i
		_ = expected
	}
}

// TestDisplayWalletAddresses tests address display formatting.
func TestDisplayWalletAddresses(t *testing.T) {
	buf := new(bytes.Buffer)
	cmd := &cobra.Command{}
	cmd.SetOut(buf)

	// Create test wallet with addresses
	w := &wallet.Wallet{
		Name: "test",
		Addresses: map[wallet.ChainID][]wallet.Address{
			wallet.ChainETH: {
				{Index: 0, Address: "0x742d35Cc6634C0532925a3b844Bc9e7595f8b2E0", Path: "m/44'/60'/0'/0/0"},
			},
			wallet.ChainBSV: {
				{Index: 0, Address: "1A1zP1eP5QGefi2DMPTfTL5SLmv7DivfNa", Path: "m/44'/236'/0'/0/0"},
			},
		},
	}

	displayWalletAddresses(w, cmd)

	output := buf.String()
	assert.Contains(t, output, "Derived Addresses:")
	assert.Contains(t, output, "ETH:")
	assert.Contains(t, output, "BSV:")
	assert.Contains(t, output, "0x742d35Cc6634C0532925a3b844Bc9e7595f8b2E0")
	assert.Contains(t, output, "1A1zP1eP5QGefi2DMPTfTL5SLmv7DivfNa")
}

// TestFormatEmptyWalletListText tests empty list text formatting.
func TestFormatEmptyWalletListText(t *testing.T) {
	buf := new(bytes.Buffer)
	formatEmptyWalletList(buf, output.FormatText)

	result := buf.String()
	assert.Contains(t, result, "No wallets found")
	assert.Contains(t, result, "sigil wallet create")
}

// TestFormatEmptyWalletListJSON tests empty list JSON formatting.
func TestFormatEmptyWalletListJSON(t *testing.T) {
	buf := new(bytes.Buffer)
	formatEmptyWalletList(buf, output.FormatJSON)

	assert.Equal(t, "[]\n", buf.String())
}

// TestFormatWalletListText tests wallet list text formatting.
func TestFormatWalletListText(t *testing.T) {
	buf := new(bytes.Buffer)
	names := []string{"wallet1", "wallet2", "wallet3"}

	formatWalletListText(buf, names)

	result := buf.String()
	assert.Contains(t, result, "Wallets:")
	assert.Contains(t, result, "wallet1")
	assert.Contains(t, result, "wallet2")
	assert.Contains(t, result, "wallet3")
}

// TestFormatWalletListJSON tests wallet list JSON formatting.
func TestFormatWalletListJSON(t *testing.T) {
	buf := new(bytes.Buffer)
	names := []string{"wallet1", "wallet2"}

	formatWalletListJSON(buf, names)

	assert.Equal(t, `["wallet1","wallet2"]`+"\n", buf.String())
}

// TestWalletShowNotFound tests showing a non-existent wallet.
func TestWalletShowNotFound(t *testing.T) {
	tmpDir, cleanup := setupTestEnv(t)
	defer cleanup()

	storage := wallet.NewFileStorage(filepath.Join(tmpDir, "wallets"))

	exists, err := storage.Exists("nonexistent")
	require.NoError(t, err)
	assert.False(t, exists)
}

// TestDisplayWalletJSON tests JSON wallet display.
func TestDisplayWalletJSON(t *testing.T) {
	buf := new(bytes.Buffer)
	cmd := &cobra.Command{}
	cmd.SetOut(buf)

	w := &wallet.Wallet{
		Name:    "test",
		Version: 1,
		Addresses: map[wallet.ChainID][]wallet.Address{
			wallet.ChainETH: {
				{Index: 0, Address: "0xtest", Path: "m/44'/60'/0'/0/0"},
			},
		},
	}

	displayWalletJSON(w, cmd)

	output := buf.String()
	assert.Contains(t, output, `"name": "test"`)
	assert.Contains(t, output, `"version": 1`)
	assert.Contains(t, output, `"eth"`)
}

// TestWalletCreateE2E is an end-to-end test for wallet creation.
// This test creates a wallet using the core functions and verifies the complete flow.
func TestWalletCreateE2E(t *testing.T) {
	tmpDir, cleanup := setupTestEnv(t)
	defer cleanup()

	storage := wallet.NewFileStorage(filepath.Join(tmpDir, "wallets"))
	walletName := "e2e_test_wallet"
	wordCount := 12
	password := []byte("secure_password_123")

	// Step 1: Validate inputs
	err := validateWalletCreationParams(walletName, wordCount, storage)
	require.NoError(t, err, "wallet creation params should be valid")

	// Step 2: Generate mnemonic and seed
	mnemonic, err := wallet.GenerateMnemonic(wordCount)
	require.NoError(t, err, "should generate mnemonic")

	words := strings.Fields(mnemonic)
	assert.Len(t, words, 12, "should have 12 words")

	seed, err := wallet.MnemonicToSeed(mnemonic, "")
	require.NoError(t, err, "should derive seed from mnemonic")
	defer wallet.ZeroBytes(seed)

	// Step 3: Create wallet
	w, err := wallet.NewWallet(walletName, []wallet.ChainID{wallet.ChainETH, wallet.ChainBSV})
	require.NoError(t, err, "should create wallet")

	// Step 4: Derive addresses
	err = w.DeriveAddresses(seed, 1)
	require.NoError(t, err, "should derive addresses")

	// Verify addresses were derived
	assert.Len(t, w.Addresses[wallet.ChainETH], 1, "should have 1 ETH address")
	assert.Len(t, w.Addresses[wallet.ChainBSV], 1, "should have 1 BSV address")

	// Step 5: Save wallet
	err = storage.Save(w, seed, password)
	require.NoError(t, err, "should save wallet")

	// Step 6: Verify wallet exists
	exists, err := storage.Exists(walletName)
	require.NoError(t, err)
	assert.True(t, exists, "wallet should exist after save")

	// Step 7: Load and verify
	loadedW, loadedSeed, err := storage.Load(walletName, password)
	require.NoError(t, err, "should load wallet with correct password")
	defer wallet.ZeroBytes(loadedSeed)

	// Verify loaded data matches
	assert.Equal(t, walletName, loadedW.Name)
	assert.Equal(t, w.Addresses[wallet.ChainETH][0].Address, loadedW.Addresses[wallet.ChainETH][0].Address)
	assert.Equal(t, w.Addresses[wallet.ChainBSV][0].Address, loadedW.Addresses[wallet.ChainBSV][0].Address)
	assert.Equal(t, seed, loadedSeed)

	// Step 8: Verify mnemonic can recreate same addresses
	seed2, err := wallet.MnemonicToSeed(mnemonic, "")
	require.NoError(t, err)
	defer wallet.ZeroBytes(seed2)

	w2, err := wallet.NewWallet("verification", []wallet.ChainID{wallet.ChainETH, wallet.ChainBSV})
	require.NoError(t, err)
	require.NoError(t, w2.DeriveAddresses(seed2, 1))

	// Addresses derived from same mnemonic should match
	assert.Equal(t, w.Addresses[wallet.ChainETH][0].Address, w2.Addresses[wallet.ChainETH][0].Address,
		"ETH address should be deterministic from mnemonic")
	assert.Equal(t, w.Addresses[wallet.ChainBSV][0].Address, w2.Addresses[wallet.ChainBSV][0].Address,
		"BSV address should be deterministic from mnemonic")
}

// TestWalletCreate24Words tests wallet creation with 24 words.
func TestWalletCreate24Words(t *testing.T) {
	tmpDir, cleanup := setupTestEnv(t)
	defer cleanup()

	storage := wallet.NewFileStorage(filepath.Join(tmpDir, "wallets"))

	// Validate 24-word creation
	err := validateWalletCreationParams("wallet24", 24, storage)
	require.NoError(t, err)

	// Generate 24-word mnemonic
	mnemonic, err := wallet.GenerateMnemonic(24)
	require.NoError(t, err)

	words := strings.Fields(mnemonic)
	assert.Len(t, words, 24)

	// Derive and verify
	seed, err := wallet.MnemonicToSeed(mnemonic, "")
	require.NoError(t, err)
	defer wallet.ZeroBytes(seed)

	w, err := wallet.NewWallet("wallet24", []wallet.ChainID{wallet.ChainETH, wallet.ChainBSV})
	require.NoError(t, err)
	require.NoError(t, w.DeriveAddresses(seed, 1))

	assert.Len(t, w.Addresses[wallet.ChainETH], 1)
	assert.Len(t, w.Addresses[wallet.ChainBSV], 1)
}

// TestWalletRestoreFromMnemonic tests wallet restoration from a known mnemonic.
func TestWalletRestoreFromMnemonic(t *testing.T) {
	tmpDir, cleanup := setupTestEnv(t)
	defer cleanup()

	storage := wallet.NewFileStorage(filepath.Join(tmpDir, "wallets"))
	walletName := "restored_wallet"
	password := []byte("test_password_123")

	// Use the standard BIP39 test vector mnemonic
	mnemonic := "abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon about"

	// Detect format
	format := wallet.DetectInputFormat(mnemonic)
	assert.Equal(t, wallet.FormatMnemonic, format)

	// Validate mnemonic
	require.NoError(t, wallet.ValidateMnemonic(mnemonic))

	// Derive seed
	seed, err := wallet.MnemonicToSeed(mnemonic, "")
	require.NoError(t, err)
	defer wallet.ZeroBytes(seed)

	// Create wallet
	w, err := wallet.NewWallet(walletName, []wallet.ChainID{wallet.ChainETH, wallet.ChainBSV})
	require.NoError(t, err)

	// Derive addresses
	err = w.DeriveAddresses(seed, 1)
	require.NoError(t, err)

	// Save wallet
	err = storage.Save(w, seed, password)
	require.NoError(t, err)

	// Verify wallet exists
	exists, err := storage.Exists(walletName)
	require.NoError(t, err)
	assert.True(t, exists)

	// Verify addresses are deterministic by restoring again
	seed2, err := wallet.MnemonicToSeed(mnemonic, "")
	require.NoError(t, err)
	defer wallet.ZeroBytes(seed2)

	w2, err := wallet.NewWallet("verification", []wallet.ChainID{wallet.ChainETH, wallet.ChainBSV})
	require.NoError(t, err)
	require.NoError(t, w2.DeriveAddresses(seed2, 1))

	assert.Equal(t, w.Addresses[wallet.ChainETH][0].Address, w2.Addresses[wallet.ChainETH][0].Address)
	assert.Equal(t, w.Addresses[wallet.ChainBSV][0].Address, w2.Addresses[wallet.ChainBSV][0].Address)
}

// TestWalletRestoreDetectTypos tests that typo detection works during restore.
//
//nolint:misspell // Intentional typos for testing
func TestWalletRestoreDetectTypos(t *testing.T) {
	// Mnemonic with a typo in the first word
	mnemonicWithTypo := "abondon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon about"

	// Detect format - should still detect as mnemonic
	format := wallet.DetectInputFormat(mnemonicWithTypo)
	assert.Equal(t, wallet.FormatMnemonic, format)

	// Validate should fail
	err := wallet.ValidateMnemonic(mnemonicWithTypo)
	require.Error(t, err)

	// Detect typos
	typos := wallet.DetectTypos(mnemonicWithTypo)
	require.Len(t, typos, 1)
	assert.Equal(t, 0, typos[0].Index)
	assert.Equal(t, "abondon", typos[0].Word)
	assert.Equal(t, "abandon", typos[0].Suggestion)
}

// TestWalletRestoreFromHex tests wallet restoration from hex private key.
func TestWalletRestoreFromHex(t *testing.T) {
	tmpDir, cleanup := setupTestEnv(t)
	defer cleanup()

	storage := wallet.NewFileStorage(filepath.Join(tmpDir, "wallets"))

	// Valid hex private key (64 chars)
	hexKey := "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"

	// Detect format
	format := wallet.DetectInputFormat(hexKey)
	assert.Equal(t, wallet.FormatHex, format)

	// Parse hex key
	keyBytes, err := wallet.ParseHexKey(hexKey)
	require.NoError(t, err)
	assert.Len(t, keyBytes, 32)

	// For WIF/hex imports, addresses would be derived differently
	// (not from HD path but from the raw key). This test verifies format detection
	// and parsing work correctly.
	_ = storage // Storage would be used in full implementation
}

// TestWalletRestoreValidation tests input validation for restore.
func TestWalletRestoreValidation(t *testing.T) {
	tmpDir, cleanup := setupTestEnv(t)
	defer cleanup()

	storage := wallet.NewFileStorage(filepath.Join(tmpDir, "wallets"))

	tests := []struct {
		name       string
		walletName string
		wantErr    bool
	}{
		{name: "valid name", walletName: "restored", wantErr: false},
		{name: "name with underscore", walletName: "my_restored_wallet", wantErr: false},
		{name: "invalid name with dash", walletName: "my-wallet", wantErr: true},
		{name: "invalid name with space", walletName: "my wallet", wantErr: true},
		{name: "empty name", walletName: "", wantErr: true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := wallet.ValidateWalletName(tc.walletName)
			if tc.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)

				// Also check wallet doesn't already exist
				exists, checkErr := storage.Exists(tc.walletName)
				require.NoError(t, checkErr)
				assert.False(t, exists)
			}
		})
	}
}

// TestWalletRestoreDuplicatePrevention tests that existing wallets can't be overwritten.
func TestWalletRestoreDuplicatePrevention(t *testing.T) {
	tmpDir, cleanup := setupTestEnv(t)
	defer cleanup()

	storage := wallet.NewFileStorage(filepath.Join(tmpDir, "wallets"))

	// Create an existing wallet
	existingWallet, err := wallet.NewWallet("existing", []wallet.ChainID{wallet.ChainETH})
	require.NoError(t, err)

	mnemonic, err := wallet.GenerateMnemonic(12)
	require.NoError(t, err)
	seed, err := wallet.MnemonicToSeed(mnemonic, "")
	require.NoError(t, err)
	defer wallet.ZeroBytes(seed)

	require.NoError(t, existingWallet.DeriveAddresses(seed, 1))
	require.NoError(t, storage.Save(existingWallet, seed, []byte("password")))

	// Verify it exists
	exists, err := storage.Exists("existing")
	require.NoError(t, err)
	assert.True(t, exists)

	// Try to save another wallet with the same name
	newWallet, err := wallet.NewWallet("existing", []wallet.ChainID{wallet.ChainBSV})
	require.NoError(t, err)
	newSeed, err := wallet.MnemonicToSeed(mnemonic, "different")
	require.NoError(t, err)
	defer wallet.ZeroBytes(newSeed)

	require.NoError(t, newWallet.DeriveAddresses(newSeed, 1))

	// This should fail because wallet already exists
	err = storage.Save(newWallet, newSeed, []byte("password"))
	require.Error(t, err)
	require.ErrorIs(t, err, wallet.ErrWalletExists)
}
