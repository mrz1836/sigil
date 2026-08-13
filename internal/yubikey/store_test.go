package yubikey_test

import (
	"bytes"
	"context"
	"testing"

	tumbler "github.com/mrz1836/go-tumbler"
	"github.com/mrz1836/go-tumbler/transport"
	"github.com/mrz1836/sigil/internal/yubikey"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// seed64 is a fixed 64-byte wallet seed.
func seed64() []byte {
	s := make([]byte, 64)
	for i := range s {
		s[i] = byte(i + 1)
	}
	return s
}

func newStore() (*yubikey.Store, *transport.FakeTransport) {
	fake := transport.NewFakeTransport(2, []byte("device-hmac-secret-20"))
	return yubikey.NewStore(fake, 2), fake
}

func TestStore_TwoFactor_EnrollUnlock(t *testing.T) {
	ctx := context.Background()
	store, _ := newStore()
	seed := seed64()
	password := []byte("wallet-password")

	res, err := store.Enroll(ctx, seed, tumbler.PolicyPasswordAndYubiKey, yubikey.EnrollOptions{Password: password})
	require.NoError(t, err)
	require.NotEmpty(t, res.Envelope)
	assert.Empty(t, res.RecoveryCode)

	// Effective policy is 2FA.
	pol, err := store.EffectivePolicy(res.Envelope)
	require.NoError(t, err)
	assert.Equal(t, tumbler.PolicyPasswordAndYubiKey, pol)

	got, err := store.Unlock(ctx, res.Envelope, func() ([]byte, error) {
		return []byte("wallet-password"), nil
	})
	require.NoError(t, err)
	assert.True(t, bytes.Equal(seed64(), got), "recovered seed must match")

	// Wrong password fails.
	_, err = store.Unlock(ctx, res.Envelope, func() ([]byte, error) {
		return []byte("wrong-password"), nil
	})
	assert.ErrorIs(t, err, tumbler.ErrAuthFailed)
}

func TestStore_YubiKeyOnly_EnrollUnlock(t *testing.T) {
	ctx := context.Background()
	store, _ := newStore()
	seed := seed64()

	res, err := store.Enroll(ctx, seed, tumbler.PolicyYubiKeyOnly, yubikey.EnrollOptions{})
	require.NoError(t, err)

	pol, err := store.EffectivePolicy(res.Envelope)
	require.NoError(t, err)
	assert.Equal(t, tumbler.PolicyYubiKeyOnly, pol)

	// No password needed; passwordFn must not be called.
	got, err := store.Unlock(ctx, res.Envelope, func() ([]byte, error) {
		t.Fatal("passwordFn must not be called for yubikey-only")
		return nil, nil
	})
	require.NoError(t, err)
	assert.True(t, bytes.Equal(seed64(), got))
}

func TestStore_TwoFactorRequiresPassword(t *testing.T) {
	ctx := context.Background()
	store, _ := newStore()
	_, err := store.Enroll(ctx, seed64(), tumbler.PolicyPasswordAndYubiKey, yubikey.EnrollOptions{})
	assert.ErrorIs(t, err, yubikey.ErrPasswordRequired)
}

func TestStore_RecoveryCode(t *testing.T) {
	ctx := context.Background()
	store, _ := newStore()
	seed := seed64()

	res, err := store.Enroll(ctx, seed, tumbler.PolicyYubiKeyOnly, yubikey.EnrollOptions{WithRecovery: true})
	require.NoError(t, err)
	require.NotEmpty(t, res.RecoveryCode)

	got, err := store.UnlockWithRecovery(ctx, res.Envelope, res.RecoveryCode)
	require.NoError(t, err)
	assert.True(t, bytes.Equal(seed64(), got))

	// A wrong recovery code fails.
	_, err = store.UnlockWithRecovery(ctx, res.Envelope, "AAAA-AAAA-AAAA-AAAA-AAAA-AAAA-AAAA")
	require.Error(t, err)
}

func TestStore_AddBackupKey(t *testing.T) {
	ctx := context.Background()
	fake := transport.NewFakeTransport(2, []byte("primary-secret"))
	store := yubikey.NewStore(fake, 2)
	seed := seed64()

	res, err := store.Enroll(ctx, seed, tumbler.PolicyYubiKeyOnly, yubikey.EnrollOptions{})
	require.NoError(t, err)

	// Add a backup key on a different device secret.
	backupStore := yubikey.NewStore(transport.NewFakeTransport(2, []byte("backup-secret")), 2)
	updated, err := backupStore.AddBackupKey(ctx, res.Envelope, seed, nil, "backup")
	require.NoError(t, err)

	// Backup device unlocks the same seed.
	got, err := backupStore.Unlock(ctx, updated, nil)
	require.NoError(t, err)
	assert.True(t, bytes.Equal(seed64(), got))
}

func TestStore_TouchTimeoutSurfaces(t *testing.T) {
	ctx := context.Background()
	store, fake := newStore()
	res, err := store.Enroll(ctx, seed64(), tumbler.PolicyYubiKeyOnly, yubikey.EnrollOptions{})
	require.NoError(t, err)

	fake.TouchFn = func() error { return transport.ErrTouchTimeout }
	_, err = store.Unlock(ctx, res.Envelope, nil)
	assert.ErrorIs(t, err, transport.ErrTouchTimeout)
}
