package wallet

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

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
		{"ALLCAPS", false},
		{"a", false},
		{longValid, false}, // 64 chars

		{"", true},
		{longInvalid, true}, // 65 chars
		{"with-dash", true},
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
