package wallet

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mrz1836/sigil/internal/sigilcrypto"
)

func TestMain(m *testing.M) {
	sigilcrypto.SetScryptWorkFactor(10) // Fast for tests
	os.Exit(m.Run())
}

func TestStorage_SaveAndLoad(t *testing.T) {
	t.Parallel()
	// Create temp directory for test
	tmpDir, err := os.MkdirTemp("", "sigil-wallet-test")
	require.NoError(t, err)
	defer func() { _ = os.RemoveAll(tmpDir) }()

	storage := NewFileStorage(tmpDir)
	password := []byte("test-password-123")

	// Create test wallet
	wallet, err := NewWallet("test", []ChainID{ChainETH, ChainBSV})
	require.NoError(t, err)

	// Generate test seed
	mnemonic, err := GenerateMnemonic(12)
	require.NoError(t, err)

	seed, err := MnemonicToSeed(mnemonic, "")
	require.NoError(t, err)

	// Derive addresses
	err = wallet.DeriveAddresses(seed, 1)
	require.NoError(t, err)

	// Save wallet
	err = storage.Save(wallet, seed, password)
	require.NoError(t, err)

	// Verify file exists with correct permissions
	walletPath := filepath.Join(tmpDir, "test.wallet")
	info, err := os.Stat(walletPath)
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0o600), info.Mode().Perm())

	// Load wallet
	loadedWallet, loadedSeed, err := storage.Load("test", password)
	require.NoError(t, err)

	// Verify wallet data
	assert.Equal(t, wallet.Name, loadedWallet.Name)
	assert.Equal(t, wallet.EnabledChains, loadedWallet.EnabledChains)
	assert.Equal(t, wallet.Version, loadedWallet.Version)
	assert.Len(t, loadedWallet.Addresses, len(wallet.Addresses))

	// Verify seed
	assert.Equal(t, seed, loadedSeed)

	// Clean up loaded seed
	ZeroBytes(loadedSeed)
}

func TestStorage_LoadWrongPassword(t *testing.T) {
	t.Parallel()
	tmpDir, err := os.MkdirTemp("", "sigil-wallet-test")
	require.NoError(t, err)
	defer func() { _ = os.RemoveAll(tmpDir) }()

	storage := NewFileStorage(tmpDir)

	// Create and save wallet
	wallet, err := NewWallet("test", []ChainID{ChainETH})
	require.NoError(t, err)

	mnemonic, _ := GenerateMnemonic(12)
	seed, _ := MnemonicToSeed(mnemonic, "")
	require.NoError(t, wallet.DeriveAddresses(seed, 1))

	err = storage.Save(wallet, seed, []byte("correct-password"))
	require.NoError(t, err)

	// Try to load with wrong password
	_, _, err = storage.Load("test", []byte("wrong-password"))
	assert.Error(t, err)
}

func TestStorage_LoadNotFound(t *testing.T) {
	t.Parallel()
	tmpDir, err := os.MkdirTemp("", "sigil-wallet-test")
	require.NoError(t, err)
	defer func() { _ = os.RemoveAll(tmpDir) }()

	storage := NewFileStorage(tmpDir)

	_, _, err = storage.Load("nonexistent", []byte("password"))
	assert.ErrorIs(t, err, ErrWalletNotFound)
}

func TestStorage_Exists(t *testing.T) {
	t.Parallel()
	tmpDir, err := os.MkdirTemp("", "sigil-wallet-test")
	require.NoError(t, err)
	defer func() { _ = os.RemoveAll(tmpDir) }()

	storage := NewFileStorage(tmpDir)

	// Should not exist initially
	exists, err := storage.Exists("test")
	require.NoError(t, err)
	assert.False(t, exists)

	// Create wallet
	wallet, _ := NewWallet("test", []ChainID{ChainETH})
	mnemonic, _ := GenerateMnemonic(12)
	seed, _ := MnemonicToSeed(mnemonic, "")
	require.NoError(t, wallet.DeriveAddresses(seed, 1))
	require.NoError(t, storage.Save(wallet, seed, []byte("password")))

	// Should exist now
	exists, err = storage.Exists("test")
	require.NoError(t, err)
	assert.True(t, exists)
}

func TestStorage_List(t *testing.T) {
	t.Parallel()
	tmpDir, err := os.MkdirTemp("", "sigil-wallet-test")
	require.NoError(t, err)
	defer func() { _ = os.RemoveAll(tmpDir) }()

	storage := NewFileStorage(tmpDir)

	// Should be empty initially
	names, err := storage.List()
	require.NoError(t, err)
	assert.Empty(t, names)

	// Create wallets
	for _, name := range []string{"wallet1", "wallet2", "wallet3"} {
		w, _ := NewWallet(name, []ChainID{ChainETH})
		mnemonic, _ := GenerateMnemonic(12)
		seed, _ := MnemonicToSeed(mnemonic, "")
		require.NoError(t, w.DeriveAddresses(seed, 1))
		require.NoError(t, storage.Save(w, seed, []byte("password")))
	}

	// Should list all wallets
	names, err = storage.List()
	require.NoError(t, err)
	assert.Len(t, names, 3)
	assert.Contains(t, names, "wallet1")
	assert.Contains(t, names, "wallet2")
	assert.Contains(t, names, "wallet3")
}

func TestStorage_Delete(t *testing.T) {
	t.Parallel()
	tmpDir, err := os.MkdirTemp("", "sigil-wallet-test")
	require.NoError(t, err)
	defer func() { _ = os.RemoveAll(tmpDir) }()

	storage := NewFileStorage(tmpDir)

	// Create wallet
	w, _ := NewWallet("test", []ChainID{ChainETH})
	mnemonic, _ := GenerateMnemonic(12)
	seed, _ := MnemonicToSeed(mnemonic, "")
	require.NoError(t, w.DeriveAddresses(seed, 1))
	require.NoError(t, storage.Save(w, seed, []byte("password")))

	// Verify exists
	exists, _ := storage.Exists("test")
	assert.True(t, exists)

	// Delete
	err = storage.Delete("test")
	require.NoError(t, err)

	// Verify deleted
	exists, _ = storage.Exists("test")
	assert.False(t, exists)
}

func TestStorage_DeleteNotFound(t *testing.T) {
	t.Parallel()
	tmpDir, err := os.MkdirTemp("", "sigil-wallet-test")
	require.NoError(t, err)
	defer func() { _ = os.RemoveAll(tmpDir) }()

	storage := NewFileStorage(tmpDir)

	err = storage.Delete("nonexistent")
	assert.ErrorIs(t, err, ErrWalletNotFound)
}

func TestStorage_SaveOverwritePrevented(t *testing.T) {
	t.Parallel()
	tmpDir, err := os.MkdirTemp("", "sigil-wallet-test")
	require.NoError(t, err)
	defer func() { _ = os.RemoveAll(tmpDir) }()

	storage := NewFileStorage(tmpDir)

	// Create first wallet
	wallet1, _ := NewWallet("test", []ChainID{ChainETH})
	mnemonic1, _ := GenerateMnemonic(12)
	seed1, _ := MnemonicToSeed(mnemonic1, "")
	require.NoError(t, wallet1.DeriveAddresses(seed1, 1))

	err = storage.Save(wallet1, seed1, []byte("password"))
	require.NoError(t, err)

	// Try to save another wallet with same name
	wallet2, _ := NewWallet("test", []ChainID{ChainBSV})
	mnemonic2, _ := GenerateMnemonic(12)
	seed2, _ := MnemonicToSeed(mnemonic2, "")
	require.NoError(t, wallet2.DeriveAddresses(seed2, 1))

	err = storage.Save(wallet2, seed2, []byte("password"))
	assert.ErrorIs(t, err, ErrWalletExists)
}

func TestStorage_UpdateMetadata_PersistsDerivedAddresses(t *testing.T) {
	t.Parallel()

	tmpDir, err := os.MkdirTemp("", "sigil-wallet-test")
	require.NoError(t, err)
	defer func() { _ = os.RemoveAll(tmpDir) }()

	storage := NewFileStorage(tmpDir)
	password := []byte("test-password-123")

	w, err := NewWallet("test", []ChainID{ChainBSV})
	require.NoError(t, err)

	mnemonic, err := GenerateMnemonic(12)
	require.NoError(t, err)
	seed, err := MnemonicToSeed(mnemonic, "")
	require.NoError(t, err)
	defer ZeroBytes(seed)

	require.NoError(t, w.DeriveAddresses(seed, 1))
	require.NoError(t, storage.Save(w, seed, password))

	walletPath := filepath.Join(tmpDir, "test.wallet")
	initialData, err := os.ReadFile(walletPath) //nolint:gosec // G304: Test path from controlled test input
	require.NoError(t, err)

	var initialFile walletFile
	require.NoError(t, json.Unmarshal(initialData, &initialFile))
	initialCount := len(initialFile.Wallet.Addresses[ChainBSV])
	initialEncryptedSeed := append([]byte(nil), initialFile.EncryptedSeed...)

	_, err = w.DeriveNextReceiveAddress(seed, ChainBSV)
	require.NoError(t, err)
	require.NoError(t, storage.UpdateMetadata(w))

	updatedData, err := os.ReadFile(walletPath) //nolint:gosec // G304: Test path from controlled test input
	require.NoError(t, err)

	var updatedFile walletFile
	require.NoError(t, json.Unmarshal(updatedData, &updatedFile))
	assert.Len(t, updatedFile.Wallet.Addresses[ChainBSV], initialCount+1)
	assert.True(t, bytes.Equal(initialEncryptedSeed, updatedFile.EncryptedSeed))

	loadedWallet, loadedSeed, err := storage.Load("test", password)
	require.NoError(t, err)
	defer ZeroBytes(loadedSeed)
	assert.Len(t, loadedWallet.Addresses[ChainBSV], initialCount+1)
}

func TestStorage_UpdateMetadata_WalletNotFound(t *testing.T) {
	t.Parallel()

	tmpDir, err := os.MkdirTemp("", "sigil-wallet-test")
	require.NoError(t, err)
	defer func() { _ = os.RemoveAll(tmpDir) }()

	storage := NewFileStorage(tmpDir)
	w, err := NewWallet("missing", []ChainID{ChainETH})
	require.NoError(t, err)

	err = storage.UpdateMetadata(w)
	assert.ErrorIs(t, err, ErrWalletNotFound)
}

func TestStorage_PermissionDenied(t *testing.T) {
	t.Parallel()

	tmpDir, err := os.MkdirTemp("", "sigil-wallet-test")
	require.NoError(t, err)
	defer func() { _ = os.RemoveAll(tmpDir) }()

	// Create a read-only directory
	readOnlyDir := filepath.Join(tmpDir, "readonly")
	err = os.Mkdir(readOnlyDir, 0o500) // r-x------ (read and execute only)
	require.NoError(t, err)

	storage := NewFileStorage(readOnlyDir)

	// Create test wallet
	wallet, err := NewWallet("test", []ChainID{ChainBSV})
	require.NoError(t, err)

	mnemonic, err := GenerateMnemonic(12)
	require.NoError(t, err)
	seed, err := MnemonicToSeed(mnemonic, "")
	require.NoError(t, err)

	// Attempt to save should fail due to permission denied
	err = storage.Save(wallet, seed, []byte("password"))
	require.Error(t, err, "Should fail to save in read-only directory")

	// Restore write permission for cleanup
	_ = os.Chmod(readOnlyDir, 0o700) //nolint:gosec // directory needs execute permission
}

func TestStorage_ConcurrentLoadSave(t *testing.T) { //nolint:gocognit // concurrency test complexity acceptable
	t.Parallel()

	tmpDir, err := os.MkdirTemp("", "sigil-wallet-test")
	require.NoError(t, err)
	defer func() { _ = os.RemoveAll(tmpDir) }()

	storage := NewFileStorage(tmpDir)
	password := []byte("test-password")

	// Create and save initial wallet
	wallet, err := NewWallet("concurrent", []ChainID{ChainBSV})
	require.NoError(t, err)

	mnemonic, err := GenerateMnemonic(12)
	require.NoError(t, err)
	seed, err := MnemonicToSeed(mnemonic, "")
	require.NoError(t, err)

	err = wallet.DeriveAddresses(seed, 1)
	require.NoError(t, err)

	err = storage.Save(wallet, seed, password)
	require.NoError(t, err)

	// Run concurrent operations
	const numGoroutines = 10
	done := make(chan bool, numGoroutines)

	// Half goroutines do Load, half do Save
	for i := 0; i < numGoroutines; i++ {
		if i%2 == 0 {
			// Load operation
			go func() {
				defer func() { done <- true }()
				_, loadedSeed, loadErr := storage.Load("concurrent", password)
				if loadErr == nil {
					ZeroBytes(loadedSeed)
				}
			}()
		} else {
			// UpdateMetadata operation (doesn't modify seed)
			go func() {
				defer func() { done <- true }()
				w, walletErr := NewWallet("concurrent", []ChainID{ChainBSV})
				if walletErr == nil {
					_ = storage.UpdateMetadata(w)
				}
			}()
		}
	}

	// Wait for all goroutines to complete
	for i := 0; i < numGoroutines; i++ {
		<-done
	}

	// Verify wallet is still intact and loadable
	finalWallet, finalSeed, err := storage.Load("concurrent", password)
	require.NoError(t, err)
	assert.NotNil(t, finalWallet)
	assert.NotNil(t, finalSeed)
	ZeroBytes(finalSeed)
}

func TestStorage_NonexistentDirectory(t *testing.T) {
	t.Parallel()

	// Use a directory that doesn't exist
	tmpDir, err := os.MkdirTemp("", "sigil-wallet-test")
	require.NoError(t, err)
	nonexistentDir := filepath.Join(tmpDir, "nonexistent", "nested", "path")
	// Don't create it
	defer func() { _ = os.RemoveAll(tmpDir) }()

	storage := NewFileStorage(nonexistentDir)

	// Create test wallet
	wallet, err := NewWallet("test", []ChainID{ChainBSV})
	require.NoError(t, err)

	mnemonic, err := GenerateMnemonic(12)
	require.NoError(t, err)
	seed, err := MnemonicToSeed(mnemonic, "")
	require.NoError(t, err)

	// Save should create the directory structure
	err = storage.Save(wallet, seed, []byte("password"))
	require.NoError(t, err, "Save should create directory structure")

	// Verify directory was created with correct permissions
	info, err := os.Stat(nonexistentDir)
	require.NoError(t, err)
	assert.True(t, info.IsDir())
	assert.Equal(t, os.FileMode(0o700), info.Mode().Perm())
}

func TestValidateWalletName(t *testing.T) {
	t.Parallel()
	// Create valid 64-char name
	longValid := "a234567890123456789012345678901234567890123456789012345678901234"
	// Create invalid 65-char name
	longInvalid := "a2345678901234567890123456789012345678901234567890123456789012345"

	tests := []struct {
		name    string
		wantErr bool
	}{
		{"valid", false},
		{"Valid123", false},
		{"wallet_name", false},
		{"with-dash", false},
		{"ALLCAPS", false},
		{"a", false},
		{longValid, false}, // 64 chars

		{"", true},
		{longInvalid, true}, // 65 chars
		{"with space", true},
		{"with.dot", true},
		{"with@symbol", true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := ValidateWalletName(tc.name)
			if tc.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
