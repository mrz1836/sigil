package backup_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mrz1836/sigil/internal/backup"
	"github.com/mrz1836/sigil/internal/sigilcrypto"
	"github.com/mrz1836/sigil/internal/wallet"
)

func TestMain(m *testing.M) {
	sigilcrypto.SetScryptWorkFactor(10) // Fast for tests
	os.Exit(m.Run())
}

// mockStorage implements wallet.Storage for testing.
type mockStorage struct {
	wallet  *wallet.Wallet
	seed    []byte
	loadErr error
	saveErr error
	saved   *wallet.Wallet
}

func (m *mockStorage) Save(w *wallet.Wallet, _, _ []byte) error {
	if m.saveErr != nil {
		return m.saveErr
	}
	m.saved = w
	return nil
}

func (m *mockStorage) Load(_ string, _ []byte) (*wallet.Wallet, []byte, error) {
	if m.loadErr != nil {
		return nil, nil, m.loadErr
	}
	// Return a copy of the seed to simulate real behavior
	seedCopy := make([]byte, len(m.seed))
	copy(seedCopy, m.seed)
	return m.wallet, seedCopy, nil
}

func (m *mockStorage) Exists(_ string) (bool, error) {
	return m.wallet != nil, nil
}

func (m *mockStorage) List() ([]string, error) {
	if m.wallet != nil {
		return []string{m.wallet.Name}, nil
	}
	return nil, nil
}

func (m *mockStorage) Delete(_ string) error {
	return nil
}

// testWallet creates a wallet for testing.
func testWallet(t *testing.T) (*wallet.Wallet, []byte) {
	t.Helper()
	w, err := wallet.NewWallet("testwallet", []wallet.ChainID{wallet.ChainETH, wallet.ChainBSV})
	require.NoError(t, err)

	mnemonic, err := wallet.GenerateMnemonic(12)
	require.NoError(t, err)

	seed, err := wallet.MnemonicToSeed(mnemonic, "")
	require.NoError(t, err)

	err = w.DeriveAddresses(seed, 1)
	require.NoError(t, err)

	return w, seed
}

// --- manifest.go tests ---

func TestNewManifest(t *testing.T) {
	t.Parallel()

	chains := []string{"eth", "bsv"}
	addressCount := map[string]int{"eth": 5, "bsv": 3}

	before := time.Now().UTC()
	manifest := backup.NewManifest("mywallet", chains, addressCount)
	after := time.Now().UTC()

	assert.Equal(t, "mywallet", manifest.WalletName)
	assert.Equal(t, chains, manifest.Chains)
	assert.Equal(t, addressCount, manifest.AddressCount)
	assert.Equal(t, "age", manifest.EncryptionMethod)
	assert.True(t, manifest.CreatedAt.Equal(manifest.CreatedAt.UTC()), "CreatedAt should be UTC")
	assert.True(t, !manifest.CreatedAt.Before(before) && !manifest.CreatedAt.After(after),
		"CreatedAt should be between before and after")
}

func TestCalculateChecksum(t *testing.T) {
	t.Parallel()

	t.Run("deterministic output", func(t *testing.T) {
		t.Parallel()
		data := []byte("test data for checksum")
		checksum1 := backup.CalculateChecksum(data)
		checksum2 := backup.CalculateChecksum(data)
		assert.Equal(t, checksum1, checksum2)
		assert.Len(t, checksum1, 64) // SHA256 hex is 64 chars
	})

	t.Run("different data different checksum", func(t *testing.T) {
		t.Parallel()
		checksum1 := backup.CalculateChecksum([]byte("data one"))
		checksum2 := backup.CalculateChecksum([]byte("data two"))
		assert.NotEqual(t, checksum1, checksum2)
	})
}

func TestVerifyChecksum(t *testing.T) {
	t.Parallel()

	t.Run("matching checksum passes", func(t *testing.T) {
		t.Parallel()
		data := []byte("verify me")
		checksum := backup.CalculateChecksum(data)
		err := backup.VerifyChecksum(data, checksum)
		assert.NoError(t, err)
	})

	t.Run("mismatched checksum returns error", func(t *testing.T) {
		t.Parallel()
		data := []byte("original data")
		wrongChecksum := backup.CalculateChecksum([]byte("different data"))
		err := backup.VerifyChecksum(data, wrongChecksum)
		assert.ErrorIs(t, err, backup.ErrBackupCorrupted)
	})
}

func TestNewBackup(t *testing.T) {
	t.Parallel()

	manifest := backup.NewManifest("wallet", []string{"eth"}, map[string]int{"eth": 1})
	encryptedData := []byte("encrypted-content")

	b := backup.NewBackup(manifest, encryptedData)

	assert.Equal(t, backup.BackupVersion, b.Version)
	assert.Equal(t, manifest, b.Manifest)
	assert.Equal(t, encryptedData, b.EncryptedData)
	assert.Equal(t, backup.CalculateChecksum(encryptedData), b.Checksum)
}

func TestBackup_Validate(t *testing.T) {
	t.Parallel()

	t.Run("valid backup passes", func(t *testing.T) {
		t.Parallel()
		manifest := backup.NewManifest("wallet", []string{"eth"}, map[string]int{"eth": 1})
		b := backup.NewBackup(manifest, []byte("data"))
		err := b.Validate()
		assert.NoError(t, err)
	})

	t.Run("wrong version fails", func(t *testing.T) {
		t.Parallel()
		manifest := backup.NewManifest("wallet", []string{"eth"}, map[string]int{"eth": 1})
		b := backup.NewBackup(manifest, []byte("data"))
		b.Version = 999
		err := b.Validate()
		require.ErrorIs(t, err, backup.ErrInvalidFormat)
		assert.Contains(t, err.Error(), "unsupported version")
	})

	t.Run("empty wallet name fails", func(t *testing.T) {
		t.Parallel()
		manifest := backup.NewManifest("", []string{"eth"}, map[string]int{"eth": 1})
		b := backup.NewBackup(manifest, []byte("data"))
		err := b.Validate()
		require.ErrorIs(t, err, backup.ErrInvalidFormat)
		assert.Contains(t, err.Error(), "missing wallet name")
	})

	t.Run("empty data fails", func(t *testing.T) {
		t.Parallel()
		manifest := backup.NewManifest("wallet", []string{"eth"}, map[string]int{"eth": 1})
		b := backup.NewBackup(manifest, []byte{})
		err := b.Validate()
		require.ErrorIs(t, err, backup.ErrInvalidFormat)
		assert.Contains(t, err.Error(), "no encrypted data")
	})

	t.Run("bad checksum fails", func(t *testing.T) {
		t.Parallel()
		manifest := backup.NewManifest("wallet", []string{"eth"}, map[string]int{"eth": 1})
		b := backup.NewBackup(manifest, []byte("data"))
		b.Checksum = "wrong-checksum"
		err := b.Validate()
		assert.ErrorIs(t, err, backup.ErrBackupCorrupted)
	})
}

// --- backup.go Service tests ---

func TestNewService(t *testing.T) {
	t.Parallel()

	storage := &mockStorage{}
	svc := backup.NewService("/tmp/backups", storage)
	assert.NotNil(t, svc)
}

func TestService_Create(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()

	w, seed := testWallet(t)
	storage := &mockStorage{wallet: w, seed: seed}
	svc := backup.NewService(tmpDir, storage)
	password := []byte("test-password-123") // gitleaks:allow

	b, backupPath, err := svc.Create("testwallet", password)

	require.NoError(t, err)
	assert.NotNil(t, b)
	assert.NotEmpty(t, backupPath)
	assert.Equal(t, "testwallet", b.Manifest.WalletName)
	assert.Equal(t, backup.BackupVersion, b.Version)
	assert.NotEmpty(t, b.EncryptedData)
	assert.Equal(t, backup.CalculateChecksum(b.EncryptedData), b.Checksum)

	// Verify file was created with correct permissions
	info, err := os.Stat(backupPath)
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0o600), info.Mode().Perm())

	// Verify manifest contains chain info
	assert.Contains(t, b.Manifest.Chains, "eth")
	assert.Contains(t, b.Manifest.Chains, "bsv")
}

func TestService_Create_StorageLoadError(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()

	storage := &mockStorage{loadErr: assert.AnError}
	svc := backup.NewService(tmpDir, storage)

	_, _, err := svc.Create("wallet", []byte("password")) // gitleaks:allow
	require.Error(t, err)
	assert.Contains(t, err.Error(), "loading wallet")
}

func TestService_Create_WriteFailure(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	w, seed := testWallet(t)
	storage := &mockStorage{wallet: w, seed: seed}
	svc := backup.NewService(tmpDir, storage)

	require.NoError(t, os.Chmod(tmpDir, 0o500)) //nolint:gosec // G302: Test uses intentionally restrictive perms
	defer func() {
		_ = os.Chmod(tmpDir, 0o700) //nolint:gosec // G302: Restoring perms in test cleanup
	}()

	_, _, err := svc.Create("testwallet", []byte("test-password-123")) // gitleaks:allow
	require.Error(t, err)
	assert.Contains(t, err.Error(), "writing backup")
}

func TestService_Verify(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()

	w, seed := testWallet(t)
	storage := &mockStorage{wallet: w, seed: seed}
	svc := backup.NewService(tmpDir, storage)
	password := []byte("test-password-123") // gitleaks:allow

	// Create a backup first
	_, backupPath, err := svc.Create("testwallet", password)
	require.NoError(t, err)

	// Verify it
	manifest, err := svc.Verify(backupPath)
	require.NoError(t, err)
	assert.Equal(t, "testwallet", manifest.WalletName)
}

func TestService_Verify_Errors(t *testing.T) {
	t.Parallel()

	t.Run("file not found", func(t *testing.T) {
		t.Parallel()
		tmpDir := t.TempDir()
		storage := &mockStorage{}
		svc := backup.NewService(tmpDir, storage)
		_, err := svc.Verify(filepath.Join(tmpDir, "nonexistent.sigil"))
		assert.ErrorIs(t, err, backup.ErrBackupNotFound)
	})

	t.Run("invalid JSON", func(t *testing.T) {
		t.Parallel()
		tmpDir := t.TempDir()
		storage := &mockStorage{}
		svc := backup.NewService(tmpDir, storage)

		badPath := filepath.Join(tmpDir, "bad.sigil")
		err := os.WriteFile(badPath, []byte("not json"), 0o600)
		require.NoError(t, err)

		_, err = svc.Verify(badPath)
		assert.ErrorIs(t, err, backup.ErrInvalidFormat)
	})

	t.Run("validation failure", func(t *testing.T) {
		t.Parallel()
		tmpDir := t.TempDir()
		storage := &mockStorage{}
		svc := backup.NewService(tmpDir, storage)

		invalidBackup := backup.Backup{
			Version: 999, // Invalid version
			Manifest: backup.Manifest{
				WalletName: "test",
			},
			EncryptedData: []byte("data"),
			Checksum:      backup.CalculateChecksum([]byte("data")),
		}
		data, _ := json.Marshal(invalidBackup)
		invalidPath := filepath.Join(tmpDir, "invalid.sigil")
		err := os.WriteFile(invalidPath, data, 0o600)
		require.NoError(t, err)

		_, err = svc.Verify(invalidPath)
		assert.ErrorIs(t, err, backup.ErrInvalidFormat)
	})
}

func TestService_VerifyWithDecryption(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()

	w, seed := testWallet(t)
	storage := &mockStorage{wallet: w, seed: seed}
	svc := backup.NewService(tmpDir, storage)
	password := []byte("test-password-123") // gitleaks:allow

	// Create a backup first
	_, backupPath, err := svc.Create("testwallet", password)
	require.NoError(t, err)

	t.Run("correct password works", func(t *testing.T) {
		manifest, err := svc.VerifyWithDecryption(backupPath, password)
		require.NoError(t, err)
		assert.Equal(t, "testwallet", manifest.WalletName)
	})

	t.Run("wrong password fails", func(t *testing.T) {
		_, err := svc.VerifyWithDecryption(backupPath, []byte("wrong-password"))
		assert.ErrorIs(t, err, backup.ErrDecryptionFailed)
	})
}

func TestService_Restore(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()

	w, seed := testWallet(t)
	storage := &mockStorage{wallet: w, seed: seed}
	svc := backup.NewService(tmpDir, storage)
	password := []byte("test-password-123") // gitleaks:allow

	// Create a backup
	_, backupPath, err := svc.Create("testwallet", password)
	require.NoError(t, err)

	t.Run("restore with same name", func(t *testing.T) {
		err := svc.Restore(backupPath, password, "")
		require.NoError(t, err)
		assert.NotNil(t, storage.saved)
		assert.Equal(t, "testwallet", storage.saved.Name)
	})

	t.Run("restore with new name", func(t *testing.T) {
		err := svc.Restore(backupPath, password, "newwalletname")
		require.NoError(t, err)
		assert.NotNil(t, storage.saved)
		assert.Equal(t, "newwalletname", storage.saved.Name)
	})
}

func TestService_Restore_WrongPassword(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	w, seed := testWallet(t)
	storage := &mockStorage{wallet: w, seed: seed}
	svc := backup.NewService(tmpDir, storage)
	password := []byte("test-password-123") // gitleaks:allow

	_, backupPath, err := svc.Create("testwallet", password)
	require.NoError(t, err)

	err = svc.Restore(backupPath, []byte("wrong-password"), "")
	assert.ErrorIs(t, err, backup.ErrDecryptionFailed)
}

func TestService_Restore_StorageSaveError(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	w, seed := testWallet(t)
	password := []byte("test-password-123") // gitleaks:allow

	// Use working storage to create backup
	goodStorage := &mockStorage{wallet: w, seed: seed}
	goodSvc := backup.NewService(tmpDir, goodStorage)
	_, backupPath, err := goodSvc.Create("testwallet", password)
	require.NoError(t, err)

	// Use failing storage to attempt restore
	failStorage := &mockStorage{wallet: w, seed: seed, saveErr: assert.AnError}
	failSvc := backup.NewService(tmpDir, failStorage)

	err = failSvc.Restore(backupPath, password, "")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "saving restored wallet")
}

func TestService_List(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()

	storage := &mockStorage{}
	svc := backup.NewService(tmpDir, storage)

	t.Run("empty directory", func(t *testing.T) {
		backups, err := svc.List()
		require.NoError(t, err)
		assert.Empty(t, backups)
	})

	t.Run("filters by extension and ignores directories", func(t *testing.T) {
		// Create some backup files
		require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "wallet1.sigil"), []byte("{}"), 0o600))
		require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "wallet2.sigil"), []byte("{}"), 0o600))

		// Create non-backup files that should be ignored
		require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "readme.txt"), []byte("hi"), 0o600))
		require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "backup.json"), []byte("{}"), 0o600))

		// Create a directory that should be ignored
		require.NoError(t, os.Mkdir(filepath.Join(tmpDir, "subdir.sigil"), 0o750))

		backups, err := svc.List()
		require.NoError(t, err)
		assert.Len(t, backups, 2)
		assert.Contains(t, backups, "wallet1.sigil")
		assert.Contains(t, backups, "wallet2.sigil")
	})
}

func TestService_BackupPath(t *testing.T) {
	t.Parallel()

	storage := &mockStorage{}
	svc := backup.NewService("/var/backups", storage)

	path := svc.BackupPath("mybackup.sigil")
	assert.Equal(t, "/var/backups/mybackup.sigil", path)
}
