package cli

import (
	"bytes"
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mrz1836/sigil/internal/output"
	"github.com/mrz1836/sigil/internal/wallet"
)

// TestDisplayWalletText tests text display formatting for wallet details.
func TestDisplayWalletText(t *testing.T) {
	t.Parallel()

	wlt := &wallet.Wallet{
		Name:      "test_wallet",
		CreatedAt: time.Date(2026, 1, 15, 10, 30, 0, 0, time.UTC),
		Version:   1,
		Addresses: map[wallet.ChainID][]wallet.Address{
			wallet.ChainBSV: {
				{
					Index:   0,
					Address: "1A1zP1eP5QGefi2DMPTfTL5SLmv7DivfNa",
					Path:    "m/44'/236'/0'/0/0",
				},
				{
					Index:   1,
					Address: "1BvBMSEYstWetqTFn5Au4m4GFg7xJaNVN2",
					Path:    "m/44'/236'/0'/0/1",
				},
			},
			wallet.ChainETH: {
				{
					Index:   0,
					Address: "0x742d35Cc6634C0532925a3b844Bc454e4438f44e",
					Path:    "m/44'/60'/0'/0/0",
				},
			},
		},
	}

	cmd := &cobra.Command{}
	var buf bytes.Buffer
	cmd.SetOut(&buf)

	displayWalletText(wlt, cmd)

	result := buf.String()
	assert.Contains(t, result, "Wallet: test_wallet")
	assert.Contains(t, result, "Created: 2026-01-15 10:30:00")
	assert.Contains(t, result, "Version: 1")
	assert.Contains(t, result, "Addresses:")
	assert.Contains(t, result, "1A1zP1eP5QGefi2DMPTfTL5SLmv7DivfNa")
	assert.Contains(t, result, "m/44'/236'/0'/0/0")
	assert.Contains(t, result, "0x742d35Cc6634C0532925a3b844Bc454e4438f44e")
}

// TestDisplayWalletTextMultipleAddresses tests text display with multiple addresses per chain.
func TestDisplayWalletTextMultipleAddresses(t *testing.T) {
	t.Parallel()

	wlt := &wallet.Wallet{
		Name:      "multi_addr",
		CreatedAt: time.Date(2026, 1, 20, 15, 45, 0, 0, time.UTC),
		Version:   1,
		Addresses: map[wallet.ChainID][]wallet.Address{
			wallet.ChainBSV: {
				{Index: 0, Address: "1BSVaddr1", Path: "m/44'/236'/0'/0/0"},
				{Index: 1, Address: "1BSVaddr2", Path: "m/44'/236'/0'/0/1"},
				{Index: 2, Address: "1BSVaddr3", Path: "m/44'/236'/0'/0/2"},
			},
		},
	}

	cmd := &cobra.Command{}
	var buf bytes.Buffer
	cmd.SetOut(&buf)

	displayWalletText(wlt, cmd)

	result := buf.String()
	assert.Contains(t, result, "Wallet: multi_addr")
	assert.Contains(t, result, "[0] 1BSVaddr1")
	assert.Contains(t, result, "[1] 1BSVaddr2")
	assert.Contains(t, result, "[2] 1BSVaddr3")
	assert.Contains(t, result, "Path: m/44'/236'/0'/0/0")
	assert.Contains(t, result, "Path: m/44'/236'/0'/0/1")
	assert.Contains(t, result, "Path: m/44'/236'/0'/0/2")
}

// TestDisplayWalletTextEmptyAddresses tests text display when wallet has no addresses.
func TestDisplayWalletTextEmptyAddresses(t *testing.T) {
	t.Parallel()

	wlt := &wallet.Wallet{
		Name:      "empty_wallet",
		CreatedAt: time.Date(2026, 2, 1, 8, 0, 0, 0, time.UTC),
		Version:   1,
		Addresses: map[wallet.ChainID][]wallet.Address{},
	}

	cmd := &cobra.Command{}
	var buf bytes.Buffer
	cmd.SetOut(&buf)

	displayWalletText(wlt, cmd)

	result := buf.String()
	assert.Contains(t, result, "Wallet: empty_wallet")
	assert.Contains(t, result, "Addresses:")
}

// TestDisplayWalletJSONMultipleChains tests JSON display with multiple chains.
func TestDisplayWalletJSONMultipleChains(t *testing.T) {
	t.Parallel()

	wlt := &wallet.Wallet{
		Name:      "multi_chain",
		CreatedAt: time.Date(2026, 1, 20, 15, 45, 0, 0, time.UTC),
		Version:   1,
		Addresses: map[wallet.ChainID][]wallet.Address{
			wallet.ChainBSV: {
				{Index: 0, Address: "1BSVaddr1", Path: "m/44'/236'/0'/0/0"},
				{Index: 1, Address: "1BSVaddr2", Path: "m/44'/236'/0'/0/1"},
			},
			wallet.ChainETH: {
				{Index: 0, Address: "0xETHaddr1", Path: "m/44'/60'/0'/0/0"},
			},
		},
	}

	cmd := &cobra.Command{}
	var buf bytes.Buffer
	cmd.SetOut(&buf)

	displayWalletJSON(wlt, cmd)

	result := buf.String()
	assert.Contains(t, result, `"name": "multi_chain"`)
	assert.Contains(t, result, `"1BSVaddr1"`)
	assert.Contains(t, result, `"1BSVaddr2"`)
	assert.Contains(t, result, `"0xETHaddr1"`)
}

// --- Tests for runWalletList ---

// newWalletListTestCmd creates a cobra.Command with CommandContext for runWalletList testing.
func newWalletListTestCmd(home string, format output.Format) (*cobra.Command, *bytes.Buffer) {
	cmd := &cobra.Command{}
	cmd.SetContext(context.Background())
	SetCmdContext(cmd, &CommandContext{
		Cfg: &mockConfigProvider{home: home},
		Fmt: &mockFormatProvider{format: format},
	})
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	return cmd, &buf
}

// createTestWallet creates a real wallet file in the given wallets directory.
func createTestWallet(t *testing.T, walletsDir, name string) {
	t.Helper()
	storage := wallet.NewFileStorage(walletsDir)
	w, err := wallet.NewWallet(name, []wallet.ChainID{wallet.ChainETH})
	require.NoError(t, err)
	mnemonic, err := wallet.GenerateMnemonic(12)
	require.NoError(t, err)
	seed, err := wallet.MnemonicToSeed(mnemonic, "")
	require.NoError(t, err)
	defer wallet.ZeroBytes(seed)
	require.NoError(t, w.DeriveAddresses(seed, 1))
	require.NoError(t, storage.Save(w, seed, []byte("password")))
}

func TestRunWalletList_EmptyText(t *testing.T) {
	tmpDir, cleanup := setupTestEnv(t)
	defer cleanup()

	cmd, buf := newWalletListTestCmd(tmpDir, output.FormatText)
	err := runWalletList(cmd, nil)
	require.NoError(t, err)
	assert.Contains(t, buf.String(), "No wallets found")
}

func TestRunWalletList_EmptyJSON(t *testing.T) {
	tmpDir, cleanup := setupTestEnv(t)
	defer cleanup()

	cmd, buf := newWalletListTestCmd(tmpDir, output.FormatJSON)
	err := runWalletList(cmd, nil)
	require.NoError(t, err)
	assert.Equal(t, "[]\n", buf.String())
}

func TestRunWalletList_WithWalletsText(t *testing.T) {
	tmpDir, cleanup := setupTestEnv(t)
	defer cleanup()

	walletsDir := filepath.Join(tmpDir, "wallets")
	createTestWallet(t, walletsDir, "alpha")
	createTestWallet(t, walletsDir, "bravo")

	cmd, buf := newWalletListTestCmd(tmpDir, output.FormatText)
	err := runWalletList(cmd, nil)
	require.NoError(t, err)

	result := buf.String()
	assert.Contains(t, result, "Wallets:")
	assert.Contains(t, result, "alpha")
	assert.Contains(t, result, "bravo")
}

func TestRunWalletList_WithWalletsJSON(t *testing.T) {
	tmpDir, cleanup := setupTestEnv(t)
	defer cleanup()

	walletsDir := filepath.Join(tmpDir, "wallets")
	createTestWallet(t, walletsDir, "charlie")

	cmd, buf := newWalletListTestCmd(tmpDir, output.FormatJSON)
	err := runWalletList(cmd, nil)
	require.NoError(t, err)

	result := buf.String()
	assert.Contains(t, result, `"charlie"`)
}
