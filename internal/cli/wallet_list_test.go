package cli

import (
	"bytes"
	"testing"
	"time"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"

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
