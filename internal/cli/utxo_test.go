package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mrz1836/sigil/internal/output"
	"github.com/mrz1836/sigil/internal/utxostore"
	"github.com/mrz1836/sigil/internal/wallet"
)

// Test error sentinels for display function tests.
var (
	errScanAddressFailed = errors.New("failed to scan address 1abc")
	errNetworkTimeout    = errors.New("network timeout")
	errSingleError       = errors.New("single error")
)

func TestDisplayUTXOsText(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		utxos    []*utxostore.StoredUTXO
		contains []string
	}{
		{
			name: "single UTXO",
			utxos: []*utxostore.StoredUTXO{
				{
					TxID:          "abc123def456abc123def456abc123def456abc123def456abc123def456abc1",
					Vout:          0,
					Amount:        100000,
					Confirmations: 6,
					Address:       "1TestAddress",
				},
			},
			contains: []string{
				"TXID",
				"VOUT",
				"AMOUNT",
				"ADDRESS",
				"abc123def456abc123def456abc123def456abc123def456abc123def456abc1",
				"100000",
				"1TestAddress",
				"Total: 1 UTXOs",
				"0.00100000 BSV",
			},
		},
		{
			name: "multiple UTXOs",
			utxos: []*utxostore.StoredUTXO{
				{
					TxID:          "txid1111111111111111111111111111111111111111111111111111111111111111",
					Vout:          0,
					Amount:        50000,
					Confirmations: 10,
					Address:       "1MultiUTXOAddress",
				},
				{
					TxID:          "txid2222222222222222222222222222222222222222222222222222222222222222",
					Vout:          1,
					Amount:        150000,
					Confirmations: 5,
					Address:       "1OtherAddress",
				},
			},
			contains: []string{
				"1MultiUTXOAddress",
				"1OtherAddress",
				"Total: 2 UTXOs",
				"200000 satoshis",
				"0.00200000 BSV",
			},
		},
		{
			name: "large amounts",
			utxos: []*utxostore.StoredUTXO{
				{
					TxID:          "largetx111111111111111111111111111111111111111111111111111111111111",
					Vout:          0,
					Amount:        100000000, // 1 BSV
					Confirmations: 100,
					Address:       "1WhaleAddress",
				},
			},
			contains: []string{
				"Total: 1 UTXOs",
				"100000000 satoshis",
				"1.00000000 BSV",
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			var buf bytes.Buffer
			displayUTXOsText(&buf, tc.utxos)

			result := buf.String()
			for _, s := range tc.contains {
				assert.Contains(t, result, s)
			}
		})
	}
}

func TestDisplayUTXOsJSON(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		utxos    []*utxostore.StoredUTXO
		contains []string
	}{
		{
			name: "single UTXO",
			utxos: []*utxostore.StoredUTXO{
				{
					TxID:          "abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890",
					Vout:          0,
					Amount:        50000,
					Confirmations: 3,
					Address:       "1TestAddr",
				},
			},
			contains: []string{
				`"txid": "abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890"`,
				`"vout": 0`,
				`"amount": 50000`,
				`"address": "1TestAddr"`,
				`"confirmations": 3`,
			},
		},
		{
			name: "multiple UTXOs",
			utxos: []*utxostore.StoredUTXO{
				{
					TxID:          "first00000000000000000000000000000000000000000000000000000000001",
					Vout:          0,
					Amount:        10000,
					Confirmations: 1,
					Address:       "1AddrA",
				},
				{
					TxID:          "second0000000000000000000000000000000000000000000000000000000002",
					Vout:          2,
					Amount:        20000,
					Confirmations: 2,
					Address:       "1AddrB",
				},
			},
			contains: []string{
				`"txid": "first00000000000000000000000000000000000000000000000000000000001"`,
				`"txid": "second0000000000000000000000000000000000000000000000000000000002"`,
				`"vout": 2`,
				`"amount": 20000`,
				`"address": "1AddrB"`,
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			var buf bytes.Buffer
			displayUTXOsJSON(&buf, tc.utxos)

			result := buf.String()
			for _, s := range tc.contains {
				assert.Contains(t, result, s)
			}

			var parsed []map[string]any
			require.NoError(t, json.Unmarshal(buf.Bytes(), &parsed))
			require.Len(t, parsed, len(tc.utxos))
		})
	}
}

func TestDisplayUTXOsJSON_ArrayFormat(t *testing.T) {
	t.Parallel()

	// Test that commas are correct between elements
	utxos := []*utxostore.StoredUTXO{
		{TxID: "tx1", Vout: 0, Amount: 100, Confirmations: 1, Address: "1A"},
		{TxID: "tx2", Vout: 1, Amount: 200, Confirmations: 2, Address: "1B"},
		{TxID: "tx3", Vout: 2, Amount: 300, Confirmations: 3, Address: "1C"},
	}

	var buf bytes.Buffer
	displayUTXOsJSON(&buf, utxos)

	var parsed []map[string]any
	require.NoError(t, json.Unmarshal(buf.Bytes(), &parsed))
	require.Len(t, parsed, 3)
	assert.Equal(t, "tx1", parsed[0]["txid"])
	assert.Equal(t, "tx3", parsed[2]["txid"])
}

func TestDisplayRefreshResults(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		result   *utxostore.ScanResult
		contains []string
	}{
		{
			name: "successful refresh with UTXOs",
			result: &utxostore.ScanResult{
				AddressesScanned: 10,
				UTXOsFound:       5,
				TotalBalance:     500000,
			},
			contains: []string{
				"Addresses scanned: 10",
				"UTXOs found:       5",
				"Total balance:     500000 satoshis",
				"0.00500000 BSV",
			},
		},
		{
			name: "refresh with no UTXOs",
			result: &utxostore.ScanResult{
				AddressesScanned: 20,
				UTXOsFound:       0,
				TotalBalance:     0,
			},
			contains: []string{
				"Addresses scanned: 20",
				"UTXOs found:       0",
				"Total balance:     0 satoshis",
			},
		},
		{
			name: "refresh with errors",
			result: &utxostore.ScanResult{
				AddressesScanned: 5,
				UTXOsFound:       2,
				TotalBalance:     100000,
				Errors: []error{
					errScanAddressFailed,
					errNetworkTimeout,
				},
			},
			contains: []string{
				"Addresses scanned: 5",
				"UTXOs found:       2",
				"Errors (2):",
				"failed to scan address 1abc",
				"network timeout",
			},
		},
		{
			name: "large balance",
			result: &utxostore.ScanResult{
				AddressesScanned: 1,
				UTXOsFound:       1,
				TotalBalance:     2100000000000000, // 21 million BSV
			},
			contains: []string{
				"21000000.00000000 BSV",
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			var buf bytes.Buffer
			displayRefreshResults(&buf, tc.result)

			result := buf.String()
			for _, s := range tc.contains {
				assert.Contains(t, result, s)
			}
		})
	}
}

func TestDisplayRefreshResults_NoErrors(t *testing.T) {
	t.Parallel()

	result := &utxostore.ScanResult{
		AddressesScanned: 10,
		UTXOsFound:       3,
		TotalBalance:     300000,
		Errors:           nil, // No errors
	}

	var buf bytes.Buffer
	displayRefreshResults(&buf, result)

	output := buf.String()
	assert.NotContains(t, output, "Errors")
}

func TestDisplayRefreshResults_SingleError(t *testing.T) {
	t.Parallel()

	result := &utxostore.ScanResult{
		AddressesScanned: 5,
		UTXOsFound:       1,
		TotalBalance:     50000,
		Errors:           []error{errSingleError},
	}

	var buf bytes.Buffer
	displayRefreshResults(&buf, result)

	output := buf.String()
	assert.Contains(t, output, "Errors (1):")
	assert.Contains(t, output, "single error")
}

// --- Tests for runUTXOBalance ---

// newUTXOBalanceTestCmd creates a cobra.Command with CommandContext for runUTXOBalance testing.
func newUTXOBalanceTestCmd(home string, format output.Format, walletName string) (*cobra.Command, *bytes.Buffer) {
	cmd := &cobra.Command{}
	cmd.SetContext(context.Background())
	SetCmdContext(cmd, &CommandContext{
		Cfg: &mockConfigProvider{home: home},
		Fmt: &mockFormatProvider{format: format},
	})
	var buf bytes.Buffer
	cmd.SetOut(&buf)

	// Set the global flag used by runUTXOBalance
	utxoWallet = walletName

	return cmd, &buf
}

func TestRunUTXOBalance_WalletNotFound(t *testing.T) {
	tmpDir, testCleanup := setupTestEnv(t)
	defer testCleanup()

	cmd, _ := newUTXOBalanceTestCmd(tmpDir, output.FormatText, "nonexistent")
	err := runUTXOBalance(cmd, nil)
	require.Error(t, err)
	assert.ErrorIs(t, err, wallet.ErrWalletNotFound)
}

func TestRunUTXOBalance_EmptyStore_Text(t *testing.T) {
	tmpDir, testCleanup := setupTestEnv(t)
	defer testCleanup()

	// Create a real wallet so Exists() returns true
	walletsDir := filepath.Join(tmpDir, "wallets")
	createTestWallet(t, walletsDir, "testbal")

	// Ensure the UTXO store directory exists (wallet directory, not the .wallet file)
	utxoDir := filepath.Join(walletsDir, "testbal")
	require.NoError(t, os.MkdirAll(utxoDir, 0o750))

	cmd, buf := newUTXOBalanceTestCmd(tmpDir, output.FormatText, "testbal")
	err := runUTXOBalance(cmd, nil)
	require.NoError(t, err)
	assert.Contains(t, buf.String(), "No UTXOs stored")
}

func TestRunUTXOBalance_EmptyStore_JSON(t *testing.T) {
	tmpDir, testCleanup := setupTestEnv(t)
	defer testCleanup()

	// Create a real wallet so Exists() returns true
	walletsDir := filepath.Join(tmpDir, "wallets")
	createTestWallet(t, walletsDir, "testjson")

	// Ensure the UTXO store directory exists
	utxoDir := filepath.Join(walletsDir, "testjson")
	require.NoError(t, os.MkdirAll(utxoDir, 0o750))

	cmd, buf := newUTXOBalanceTestCmd(tmpDir, output.FormatJSON, "testjson")
	err := runUTXOBalance(cmd, nil)
	require.NoError(t, err)

	result := buf.String()
	var parsed map[string]any
	require.NoError(t, json.Unmarshal([]byte(result), &parsed))
	assert.InDelta(t, float64(0), parsed["balance"], 0)
	assert.InDelta(t, float64(0), parsed["utxos"], 0)
}
