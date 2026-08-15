package wallet

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mrz1836/sigil/internal/wallet"
	sigilerr "github.com/mrz1836/sigil/pkg/errors"
)

// errTouchTimeout is a static sentinel for the touch-timeout failure path.
var errTouchTimeout = errors.New("touch timeout")

// mockYubiKeyStore is a test double for YubiKeyUnlocker.
type mockYubiKeyStore struct {
	seed        []byte
	err         error
	sawPassword bool
}

func (m *mockYubiKeyStore) Unlock(_ context.Context, _ []byte, passwordFn func() ([]byte, error)) ([]byte, error) {
	if m.err != nil {
		return nil, m.err
	}
	// Simulate a 2FA envelope by invoking passwordFn when provided.
	if passwordFn != nil {
		if _, err := passwordFn(); err != nil {
			return nil, err
		}
		m.sawPassword = true
	}
	out := make([]byte, len(m.seed))
	copy(out, m.seed)
	return out, nil
}

func TestLoad_YubiKey_DispatchesAfterSession(t *testing.T) {
	seed := getTestSeed(t)

	storage := newMockStorageProvider()
	storage.addWallet(&wallet.Wallet{Name: "yk"}, nil) // no plaintext seed; envelope only
	storage.addEnvelope("yk", []byte("envelope-bytes"), "password-and-yubikey")

	cfg := newMockConfigProvider()
	cfg.security.SessionEnabled = false // isolate the yubikey path

	service := NewService(&Config{Storage: storage, Config: cfg})

	yk := &mockYubiKeyStore{seed: seed}
	ctx := &LoadContext{
		YubiKeyStore: yk,
	}
	req := &LoadRequest{
		Name:         "yk",
		PasswordFunc: func(string) (string, error) { return "the-password", nil },
	}

	result, sessInfo, err := service.Load(req, ctx)
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, AuthYubiKey, sessInfo.Mode)
	assert.Equal(t, seed, result.Seed)
	assert.True(t, yk.sawPassword, "2FA envelope should prompt for the password")
	wallet.ZeroBytes(result.Seed)
}

func TestLoad_YubiKey_StoreNotInitialized(t *testing.T) {
	storage := newMockStorageProvider()
	storage.addWallet(&wallet.Wallet{Name: "yk"}, nil)
	storage.addEnvelope("yk", []byte("envelope-bytes"), "yubikey-only")

	cfg := newMockConfigProvider()
	cfg.security.SessionEnabled = false
	service := NewService(&Config{Storage: storage, Config: cfg})

	// No YubiKeyStore in the context -> clear error, no silent password fallback.
	req := &LoadRequest{Name: "yk"}
	_, _, err := service.Load(req, &LoadContext{})
	require.Error(t, err)
	assert.ErrorIs(t, err, sigilerr.ErrInvalidInput)
}

func TestLoad_YubiKey_UnlockError(t *testing.T) {
	seed := getTestSeed(t)
	storage := newMockStorageProvider()
	storage.addWallet(&wallet.Wallet{Name: "yk"}, nil)
	storage.addEnvelope("yk", []byte("envelope-bytes"), "yubikey-only")

	cfg := newMockConfigProvider()
	cfg.security.SessionEnabled = false
	service := NewService(&Config{Storage: storage, Config: cfg})

	yk := &mockYubiKeyStore{seed: seed, err: errTouchTimeout}
	_, _, err := service.Load(&LoadRequest{Name: "yk"}, &LoadContext{YubiKeyStore: yk})
	assert.ErrorIs(t, err, errTouchTimeout)
}

func TestLoad_PasswordWallet_UnaffectedByYubiKeyPath(t *testing.T) {
	// A wallet with no envelope must still take the legacy password path even
	// when a YubiKey store is present in the context.
	seed := getTestSeed(t)
	storage := newMockStorageProvider()
	storage.addWallet(&wallet.Wallet{Name: "legacy"}, seed)

	cfg := newMockConfigProvider()
	cfg.security.SessionEnabled = false
	service := NewService(&Config{Storage: storage, Config: cfg})

	yk := &mockYubiKeyStore{seed: []byte("should-not-be-used")}
	req := &LoadRequest{
		Name:         "legacy",
		PasswordFunc: func(string) (string, error) { return "pw", nil },
	}
	result, sessInfo, err := service.Load(req, &LoadContext{YubiKeyStore: yk})
	require.NoError(t, err)
	assert.Equal(t, AuthPassword, sessInfo.Mode)
	assert.Equal(t, seed, result.Seed)
}
