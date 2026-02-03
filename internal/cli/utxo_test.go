package cli

import (
	"bytes"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/mrz1836/sigil/internal/chain/bsv"
	"github.com/mrz1836/sigil/internal/utxostore"
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
		address  string
		utxos    []bsv.UTXO
		contains []string
	}{
		{
			name:    "single UTXO",
			address: "1TestAddress",
			utxos: []bsv.UTXO{
				{
					TxID:          "abc123def456abc123def456abc123def456abc123def456abc123def456abc1",
					Vout:          0,
					Amount:        100000,
					Confirmations: 6,
				},
			},
			contains: []string{
				"UTXOs for 1TestAddress",
				"TXID",
				"VOUT",
				"AMOUNT",
				"abc123def456abc123def456abc123def456abc123def456abc123def456abc1",
				"100000",
				"Total: 1 UTXOs",
				"0.00100000 BSV",
			},
		},
		{
			name:    "multiple UTXOs",
			address: "1MultiUTXOAddress",
			utxos: []bsv.UTXO{
				{
					TxID:          "txid1111111111111111111111111111111111111111111111111111111111111111",
					Vout:          0,
					Amount:        50000,
					Confirmations: 10,
				},
				{
					TxID:          "txid2222222222222222222222222222222222222222222222222222222222222222",
					Vout:          1,
					Amount:        150000,
					Confirmations: 5,
				},
			},
			contains: []string{
				"UTXOs for 1MultiUTXOAddress",
				"Total: 2 UTXOs",
				"200000 satoshis",
				"0.00200000 BSV",
			},
		},
		{
			name:    "large amounts",
			address: "1WhaleAddress",
			utxos: []bsv.UTXO{
				{
					TxID:          "largetx111111111111111111111111111111111111111111111111111111111111",
					Vout:          0,
					Amount:        100000000, // 1 BSV
					Confirmations: 100,
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
			displayUTXOsText(&buf, tc.address, tc.utxos)

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
		utxos    []bsv.UTXO
		contains []string
	}{
		{
			name: "single UTXO",
			utxos: []bsv.UTXO{
				{
					TxID:          "abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890",
					Vout:          0,
					Amount:        50000,
					Confirmations: 3,
				},
			},
			contains: []string{
				`"txid": "abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890"`,
				`"vout": 0`,
				`"amount": 50000`,
				`"confirmations": 3`,
			},
		},
		{
			name: "multiple UTXOs",
			utxos: []bsv.UTXO{
				{
					TxID:          "first00000000000000000000000000000000000000000000000000000000001",
					Vout:          0,
					Amount:        10000,
					Confirmations: 1,
				},
				{
					TxID:          "second0000000000000000000000000000000000000000000000000000000002",
					Vout:          2,
					Amount:        20000,
					Confirmations: 2,
				},
			},
			contains: []string{
				`"txid": "first00000000000000000000000000000000000000000000000000000000001"`,
				`"txid": "second0000000000000000000000000000000000000000000000000000000002"`,
				`"vout": 2`,
				`"amount": 20000`,
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

			// Verify it's valid JSON array structure
			assert.NotEmpty(t, result)
			assert.Equal(t, '[', rune(result[0]))
		})
	}
}

func TestDisplayUTXOsJSON_ArrayFormat(t *testing.T) {
	t.Parallel()

	// Test that commas are correct between elements
	utxos := []bsv.UTXO{
		{TxID: "tx1", Vout: 0, Amount: 100, Confirmations: 1},
		{TxID: "tx2", Vout: 1, Amount: 200, Confirmations: 2},
		{TxID: "tx3", Vout: 2, Amount: 300, Confirmations: 3},
	}

	var buf bytes.Buffer
	displayUTXOsJSON(&buf, utxos)

	result := buf.String()

	// Should start with [ and end with ]
	assert.Contains(t, result, "[")
	assert.Contains(t, result, "]")

	// Check for proper comma placement (no trailing comma on last element)
	assert.NotContains(t, result, `}]`) // Last element should not have comma before ]
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
