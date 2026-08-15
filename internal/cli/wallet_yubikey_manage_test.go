package cli

import (
	"bytes"
	"context"
	"encoding/hex"
	"path/filepath"
	"regexp"
	"testing"

	tumbler "github.com/mrz1836/go-tumbler"
	"github.com/mrz1836/go-tumbler/transport"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mrz1836/sigil/internal/wallet"
	"github.com/mrz1836/sigil/internal/yubikey"
)

// slotIDPattern matches a rendered 16-hex slot ID for golden normalization —
// slot IDs are random per enrollment, so the golden pins the layout, not the ID.
var slotIDPattern = regexp.MustCompile(`\b[0-9a-f]{16}\b`)

// newYubiKeyManageCmd builds a cobra command wired to a home dir, capturing
// stdout and stderr separately.
func newYubiKeyManageCmd(t *testing.T, home string) (*cobra.Command, *bytes.Buffer, *bytes.Buffer) {
	t.Helper()
	cmd := &cobra.Command{}
	cmd.SetContext(context.Background())
	SetCmdContext(cmd, &CommandContext{Cfg: &mockConfigProvider{home: home}})
	var stdout, stderr bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	return cmd, &stdout, &stderr
}

// enrollTestWallet writes a password wallet then enrolls a real multi-slot
// envelope (password+yubikey primary + recovery slot) via the fake transport,
// returning the wallet name and its slot metadata.
func enrollTestWallet(t *testing.T, home string, recovery bool) (string, []tumbler.SlotInfo) {
	t.Helper()
	ctx := context.Background()
	name := "envwallet"
	seed := make([]byte, 64)
	for i := range seed {
		seed[i] = byte(i + 1)
	}
	pw := []byte("wallet-password")

	storage := wallet.NewFileStorage(filepath.Join(home, "wallets"))
	require.NoError(t, storage.Save(&wallet.Wallet{Name: name}, seed, pw))

	store := yubikey.NewStore(transport.NewFakeTransport(2, []byte("device-hmac-secret-20")), 2)
	res, err := store.Enroll(ctx, seed, tumbler.PolicyPasswordAndYubiKey,
		yubikey.EnrollOptions{Password: pw, WithRecovery: recovery})
	require.NoError(t, err)
	require.NoError(t, storage.EnrollEnvelope(name, res.Envelope, tumbler.PolicyPasswordAndYubiKey.String()))

	env, err := tumbler.ParseEnvelope(res.Envelope)
	require.NoError(t, err)
	return name, env.SlotInfos()
}

func TestRunWalletYubiKeyList_ShowsSlotsAndPolicy(t *testing.T) {
	home := t.TempDir()
	name, infos := enrollTestWallet(t, home, true)
	require.Len(t, infos, 2, "expected a 2FA primary + recovery slot")

	cmd, stdout, _ := newYubiKeyManageCmd(t, home)
	require.NoError(t, runWalletYubiKeyList(cmd, []string{name}))

	got := stdout.String()
	// Normalize the random slot IDs with a same-width (16-char) placeholder so
	// the golden pins the column layout, not the per-enrollment IDs.
	normalized := slotIDPattern.ReplaceAllString(got, "<slot-id-16-hex>")
	const golden = `Wallet "envwallet" — YubiKey keyslots
Policy: password-and-yubikey

SLOT ID           METHOD            LABEL
<slot-id-16-hex>  password+yubikey  password+yubikey
<slot-id-16-hex>  recovery          recovery-code
`
	assert.Equal(t, golden, normalized)

	// Every rendered ID is a real 16-hex handle for `remove`.
	assert.Len(t, slotIDPattern.FindAllString(got, -1), 2)
}

func TestRunWalletYubiKeyList_WarnsWhenNoRecovery(t *testing.T) {
	home := t.TempDir()
	name, _ := enrollTestWallet(t, home, false)

	cmd, stdout, _ := newYubiKeyManageCmd(t, home)
	require.NoError(t, runWalletYubiKeyList(cmd, []string{name}))

	got := stdout.String()
	assert.Contains(t, got, "Warnings:")
	assert.Contains(t, got, "no recovery code enrolled")
}

func TestRunWalletYubiKeyList_NoEnvelope(t *testing.T) {
	home := t.TempDir()
	storage := wallet.NewFileStorage(filepath.Join(home, "wallets"))
	seed := make([]byte, 64)
	require.NoError(t, storage.Save(&wallet.Wallet{Name: "plain"}, seed, []byte("pw")))

	cmd, _, _ := newYubiKeyManageCmd(t, home)
	err := runWalletYubiKeyList(cmd, []string{"plain"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no YubiKey envelope")
	assert.Contains(t, err.Error(), "enroll-yubikey")
}

func TestRunWalletYubiKeyRemove_PersistsBackupSlotRemoval(t *testing.T) {
	home := t.TempDir()
	name, infos := enrollTestWallet(t, home, true)

	// Target the recovery slot (a backup unlock method) — removing it is safe.
	var recoveryID string
	for _, info := range infos {
		if info.Type == tumbler.MethodRecovery {
			recoveryID = hexID(info.ID)
		}
	}
	require.NotEmpty(t, recoveryID)

	cmd, stdout, _ := newYubiKeyManageCmd(t, home)
	walletYubiKeyRemoveForce = true
	t.Cleanup(func() { walletYubiKeyRemoveForce = false })

	require.NoError(t, runWalletYubiKeyRemove(cmd, []string{name, recoveryID}))
	assert.Contains(t, stdout.String(), "✓ Removed keyslot")

	// The envelope now has one slot fewer and no recovery.
	storage := wallet.NewFileStorage(filepath.Join(home, "wallets"))
	blob, _, err := storage.LoadEnvelope(name)
	require.NoError(t, err)
	env, err := tumbler.ParseEnvelope(blob)
	require.NoError(t, err)
	assert.Equal(t, 1, env.SlotCount())
	_, found := findSlot(env.SlotInfos(), mustSlotID(t, recoveryID))
	assert.False(t, found, "removed slot must be gone")
}

func TestRunWalletYubiKeyRemove_WarnsOnWeakenedPosture(t *testing.T) {
	home := t.TempDir()
	name, infos := enrollTestWallet(t, home, true)

	var recoveryID string
	for _, info := range infos {
		if info.Type == tumbler.MethodRecovery {
			recoveryID = hexID(info.ID)
		}
	}

	cmd, _, stderr := newYubiKeyManageCmd(t, home)
	walletYubiKeyRemoveForce = true
	t.Cleanup(func() { walletYubiKeyRemoveForce = false })

	require.NoError(t, runWalletYubiKeyRemove(cmd, []string{name, recoveryID}))
	errOut := stderr.String()
	// Dropping the recovery slot leaves one unlock method — warn loudly.
	assert.Contains(t, errOut, "only one unlock method enrolled")
	assert.Contains(t, errOut, "no recovery code enrolled")
	assert.Contains(t, errOut, "SECURITY.md")
}

func TestRunWalletYubiKeyRemove_RefusesLastSlot(t *testing.T) {
	home := t.TempDir()
	name, infos := enrollTestWallet(t, home, false) // 2FA primary only, no recovery
	require.Len(t, infos, 1)
	primaryID := hexID(infos[0].ID)

	cmd, _, _ := newYubiKeyManageCmd(t, home)
	walletYubiKeyRemoveForce = true
	t.Cleanup(func() { walletYubiKeyRemoveForce = false })

	err := runWalletYubiKeyRemove(cmd, []string{name, primaryID})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "only remaining keyslot")

	// The envelope is untouched.
	storage := wallet.NewFileStorage(filepath.Join(home, "wallets"))
	blob, _, loadErr := storage.LoadEnvelope(name)
	require.NoError(t, loadErr)
	env, parseErr := tumbler.ParseEnvelope(blob)
	require.NoError(t, parseErr)
	assert.Equal(t, 1, env.SlotCount())
}

func TestRunWalletYubiKeyRemove_UnknownSlotID(t *testing.T) {
	home := t.TempDir()
	name, _ := enrollTestWallet(t, home, true)

	cmd, _, _ := newYubiKeyManageCmd(t, home)
	walletYubiKeyRemoveForce = true
	t.Cleanup(func() { walletYubiKeyRemoveForce = false })

	err := runWalletYubiKeyRemove(cmd, []string{name, "00000000deadbeef"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no keyslot")
}

func TestRunWalletYubiKeyRemove_InvalidSlotID(t *testing.T) {
	home := t.TempDir()
	name, _ := enrollTestWallet(t, home, true)

	cmd, _, _ := newYubiKeyManageCmd(t, home)
	for _, bad := range []string{"xyz", "abcd", "00000000deadbeefaa"} {
		err := runWalletYubiKeyRemove(cmd, []string{name, bad})
		require.Error(t, err, "input %q", bad)
		assert.Contains(t, err.Error(), "16 hexadecimal characters")
	}
}

func TestRunWalletYubiKeyRemove_AbortsWithoutConfirm(t *testing.T) {
	home := t.TempDir()
	name, infos := enrollTestWallet(t, home, true)
	var recoveryID string
	for _, info := range infos {
		if info.Type == tumbler.MethodRecovery {
			recoveryID = hexID(info.ID)
		}
	}

	cmd, stdout, _ := newYubiKeyManageCmd(t, home)
	origConfirm := confirmYubiKeyRemoveFn
	confirmYubiKeyRemoveFn = func() bool { return false }
	t.Cleanup(func() { confirmYubiKeyRemoveFn = origConfirm })

	require.NoError(t, runWalletYubiKeyRemove(cmd, []string{name, recoveryID}))
	assert.Contains(t, stdout.String(), "Aborted")

	// Nothing changed on disk.
	storage := wallet.NewFileStorage(filepath.Join(home, "wallets"))
	blob, _, err := storage.LoadEnvelope(name)
	require.NoError(t, err)
	env, err := tumbler.ParseEnvelope(blob)
	require.NoError(t, err)
	assert.Equal(t, 2, env.SlotCount())
}

// hexID renders a slot ID the way the CLI does.
func hexID(id [8]byte) string { return hex.EncodeToString(id[:]) }

func mustSlotID(t *testing.T, s string) [8]byte {
	t.Helper()
	id, err := parseSlotID(s)
	require.NoError(t, err)
	return id
}
