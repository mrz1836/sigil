package cli

import (
	"bytes"
	"context"
	"path/filepath"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mrz1836/sigil/internal/shamir"
	"github.com/mrz1836/sigil/internal/wallet"
)

// saveShamirFlags saves the current state of Shamir-related flags and returns a restore function.
func saveShamirFlags() func() {
	origCreateShamir := createShamir
	origCreateThreshold := createThreshold
	origCreateShareCount := createShareCount
	origCreateWords := createWords
	origCreatePassphrase := createPassphrase
	origCreateScan := createScan

	origRestoreShamir := restoreShamir
	origRestoreInput := restoreInput
	origRestorePassphrase := restorePassphrase
	origRestoreScan := restoreScan

	return func() {
		createShamir = origCreateShamir
		createThreshold = origCreateThreshold
		createShareCount = origCreateShareCount
		createWords = origCreateWords
		createPassphrase = origCreatePassphrase
		createScan = origCreateScan

		restoreShamir = origRestoreShamir
		restoreInput = origRestoreInput
		restorePassphrase = origRestorePassphrase
		restoreScan = origRestoreScan
	}
}

func TestWalletCreate_Shamir(t *testing.T) {
	tmpDir, cleanup := setupTestEnv(t)
	defer cleanup()
	defer saveShamirFlags()()

	withMockPrompts(t, []byte("testpassword123"), true)

	// Set flags for Shamir creation
	createShamir = true
	createThreshold = 2
	createShareCount = 3
	createWords = 12
	createPassphrase = false
	createScan = false // skip scan for speed

	// Capture output
	buf := new(bytes.Buffer)
	cmd := &cobra.Command{}
	cmd.SetOut(buf)
	cmd.SetErr(buf)

	// Inject CommandContext - REQUIRED to avoid panic
	cmd.SetContext(context.Background())
	ctx := NewCommandContext(cfg, logger, formatter)
	SetCmdContext(cmd, ctx)

	err := runWalletCreate(cmd, []string{"shamir_create_test"})
	require.NoError(t, err)

	output := buf.String()

	// Verify output contains Shamir headers
	assert.Contains(t, output, "SHAMIR SECRET SHARES")
	assert.Contains(t, output, "Your wallet seed has been split into 3 shares")
	assert.Contains(t, output, "need any 2 of them")

	// Verify shares are printed
	assert.Contains(t, output, "Share 1:")
	assert.Contains(t, output, "Share 2:")
	assert.Contains(t, output, "Share 3:")

	// Check for share prefix format
	assert.Contains(t, output, "sigil-v1-2-1-")
	assert.Contains(t, output, "sigil-v1-2-2-")
	assert.Contains(t, output, "sigil-v1-2-3-")

	// Verify wallet was created
	storage := wallet.NewFileStorage(filepath.Join(tmpDir, "wallets"))
	exists, err := storage.Exists("shamir_create_test")
	require.NoError(t, err)
	assert.True(t, exists)
}

func TestWalletRestore_Shamir(t *testing.T) {
	tmpDir, cleanup := setupTestEnv(t)
	defer cleanup()
	defer saveShamirFlags()()

	withMockPrompts(t, []byte("testpassword123"), true)

	// We need valid shares to restore. Let's generate them programmatically.
	mnemonic := "abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon about"
	shares, err := shamir.Split([]byte(mnemonic), 3, 2)
	require.NoError(t, err)

	// Set flags for Restore
	restoreShamir = true
	restorePassphrase = false
	restoreScan = false // skip scan for speed
	restoreInput = ""

	// Prepare input buffer simulating user typing shares
	// We need k=2 shares. Let's use share 1 and share 3.
	input := shares[0] + "\n" + shares[2] + "\n\n" // empty line to finish

	bufOut := new(bytes.Buffer)
	bufIn := bytes.NewBufferString(input)

	cmd := &cobra.Command{}
	cmd.SetOut(bufOut)
	cmd.SetErr(bufOut)
	cmd.SetIn(bufIn)

	// Inject CommandContext
	cmd.SetContext(context.Background())
	ctx := NewCommandContext(cfg, logger, formatter)
	SetCmdContext(cmd, ctx)

	err = runWalletRestore(cmd, []string{"shamir_restore_test"})
	require.NoError(t, err)

	output := bufOut.String()
	assert.Contains(t, output, "Enter your Shamir shares one by one")
	assert.Contains(t, output, "Share 1:")
	assert.Contains(t, output, "Share 2:")
	assert.Contains(t, output, "Share 3:")

	assert.Contains(t, output, "restored successfully")

	// Verify restoration works
	storage := wallet.NewFileStorage(filepath.Join(tmpDir, "wallets"))
	loadedW, loadedSeed, err := storage.Load("shamir_restore_test", []byte("testpassword123"))
	require.NoError(t, err)
	defer wallet.ZeroBytes(loadedSeed)

	// Check if seed matches original mnemonic
	originalSeed, _ := wallet.MnemonicToSeed(mnemonic, "")
	defer wallet.ZeroBytes(originalSeed)

	assert.Equal(t, originalSeed, loadedSeed)
	assert.Equal(t, "shamir_restore_test", loadedW.Name)
}
