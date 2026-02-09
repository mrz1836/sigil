package cli

import (
	"bytes"
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mrz1836/sigil/internal/utxostore"
	"github.com/mrz1836/sigil/internal/wallet"
)

var (
	errConnectionTimeout = errors.New("connection timeout")
	errRateLimited       = errors.New("rate limited")
)

func TestDisplayScanResults(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		result      *utxostore.ScanResult
		contains    []string
		notContains []string
	}{
		{
			name: "no errors, with UTXOs",
			result: &utxostore.ScanResult{
				AddressesScanned: 5,
				UTXOsFound:       3,
				TotalBalance:     150000000, // 1.5 BSV
			},
			contains: []string{
				"Scan Results",
				"5",
				"3",
				"150000000 satoshis",
				"1.50000000 BSV",
			},
		},
		{
			name: "with errors",
			result: &utxostore.ScanResult{
				AddressesScanned: 2,
				UTXOsFound:       0,
				TotalBalance:     0,
				Errors:           []error{errConnectionTimeout, errRateLimited},
			},
			contains: []string{
				"Scan completed with errors",
				"connection timeout",
				"rate limited",
			},
		},
		{
			name: "zero balance",
			result: &utxostore.ScanResult{
				AddressesScanned: 3,
				UTXOsFound:       0,
				TotalBalance:     0,
			},
			contains: []string{
				"0 satoshis",
				"0.00000000 BSV",
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			var buf bytes.Buffer
			cmd := &cobra.Command{}
			cmd.SetOut(&buf)

			displayScanResults(tc.result, cmd)

			output := buf.String()
			for _, s := range tc.contains {
				assert.Contains(t, output, s, "output should contain %q", s)
			}
			for _, s := range tc.notContains {
				assert.NotContains(t, output, s, "output should not contain %q", s)
			}
		})
	}
}

// --- Tests for createAndSaveWallet ---

func TestCreateAndSaveWallet_HappyPath(t *testing.T) {
	tmpDir, cleanup := setupTestEnv(t)
	defer cleanup()
	withMockPrompts(t, []byte("testpassword123"), true)

	storage := wallet.NewFileStorage(filepath.Join(tmpDir, "wallets"))

	mnemonic, err := wallet.GenerateMnemonic(12)
	require.NoError(t, err)

	seed, err := wallet.MnemonicToSeed(mnemonic, "")
	require.NoError(t, err)
	defer wallet.ZeroBytes(seed)

	w, err := createAndSaveWallet("create_test", seed, storage)
	require.NoError(t, err)
	require.NotNil(t, w)

	assert.Equal(t, "create_test", w.Name)
	assert.NotEmpty(t, w.Addresses[wallet.ChainETH])
	assert.NotEmpty(t, w.Addresses[wallet.ChainBSV])

	// Verify wallet was persisted
	exists, err := storage.Exists("create_test")
	require.NoError(t, err)
	assert.True(t, exists)
}

func TestCreateAndSaveWallet_InvalidSeed(t *testing.T) {
	tmpDir, cleanup := setupTestEnv(t)
	defer cleanup()
	withMockPrompts(t, []byte("testpassword123"), true)

	storage := wallet.NewFileStorage(filepath.Join(tmpDir, "wallets"))

	// An empty seed should cause DeriveAddresses to fail
	_, err := createAndSaveWallet("bad_seed", []byte{}, storage)
	require.Error(t, err)
}

// --- Tests for generateWalletSeed with passphrase ---

func TestGenerateWalletSeed_WithPassphrase(t *testing.T) {
	_, cleanup := setupTestEnv(t)
	defer cleanup()
	withMockPrompts(t, []byte("testpassword123"), true)

	mnemonic, seed, err := generateWalletSeed(12, true)
	require.NoError(t, err)
	defer wallet.ZeroBytes(seed)

	words := strings.Fields(mnemonic)
	assert.Len(t, words, 12)
	assert.Len(t, seed, 64)

	// The passphrase version should produce a different seed than no passphrase
	seedNoPassphrase, err := wallet.MnemonicToSeed(mnemonic, "")
	require.NoError(t, err)
	defer wallet.ZeroBytes(seedNoPassphrase)

	assert.NotEqual(t, seedNoPassphrase, seed, "passphrase should produce different seed")
}
