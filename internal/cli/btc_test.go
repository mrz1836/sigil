package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mrz1836/sigil/internal/chain"
	"github.com/mrz1836/sigil/internal/output"
	"github.com/mrz1836/sigil/internal/service/transaction"
)

func TestConvertToBTCTransactionResult(t *testing.T) {
	t.Parallel()

	// GasUsed/GasPrice/Token are ETH-only and must not leak into a BTC result.
	result := &transaction.SendResult{
		Hash:       "btc-hash-123",
		From:       "1From",
		To:         "1To",
		Amount:     "0.001",
		Fee:        "0.00000226",
		Token:      "USDC",
		Status:     "pending",
		GasUsed:    21000,
		GasPrice:   "20 Gwei",
		UTXOsSpent: 3,
	}

	converted := convertToBTCTransactionResult(result)

	assert.Equal(t, "btc-hash-123", converted.Hash)
	assert.Equal(t, "1From", converted.From)
	assert.Equal(t, "1To", converted.To)
	assert.Equal(t, "0.001", converted.Amount)
	assert.Equal(t, "0.00000226", converted.Fee)
	assert.Equal(t, "pending", converted.Status)
	// BTC results carry no ETH gas fields or token.
	assert.Empty(t, converted.Token)
	assert.Zero(t, converted.GasUsed)
	assert.Empty(t, converted.GasPrice)
}

func TestDisplayBTCTxResultText(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		network      string
		wantContains []string
	}{
		{
			name:    "mainnet uses mempool.space",
			network: "main",
			wantContains: []string{
				"Transaction broadcast successfully!",
				"abc123def456",
				"pending",
				"0.001 BTC",
				"0.00000226 BTC",
				"https://mempool.space/tx/abc123def456",
			},
		},
		{
			name:    "testnet uses testnet4 path",
			network: "test",
			wantContains: []string{
				"https://mempool.space/testnet4/tx/abc123def456",
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			result := &chain.TransactionResult{
				Hash:   "abc123def456",
				Status: "pending",
				Amount: "0.001",
				Fee:    "0.00000226",
			}

			var buf bytes.Buffer
			displayBTCTxResultText(&buf, result, tc.network)
			out := buf.String()

			for _, s := range tc.wantContains {
				assert.Contains(t, out, s)
			}
		})
	}
}

func TestDisplayBTCTxResultJSON(t *testing.T) {
	t.Parallel()

	result := &chain.TransactionResult{
		Hash:   "abc123",
		From:   "1From",
		To:     "1To",
		Amount: "0.5",
		Fee:    "0.0001",
		Status: "pending",
	}

	var buf bytes.Buffer
	displayBTCTxResultJSON(&buf, result)

	var parsed map[string]any
	require.NoError(t, json.Unmarshal(buf.Bytes(), &parsed))
	assert.Equal(t, "abc123", parsed["hash"])
	assert.Equal(t, "1From", parsed["from"])
	assert.Equal(t, "1To", parsed["to"])
	assert.Equal(t, "0.5", parsed["amount"])
	assert.Equal(t, "0.0001", parsed["fee"])
	assert.Equal(t, "pending", parsed["status"])
}

func TestDisplayBTCTxResult_TextAndJSON(t *testing.T) {
	t.Parallel()

	result := &chain.TransactionResult{
		Hash:   "deadbeef",
		From:   "1From",
		To:     "1To",
		Amount: "0.25",
		Fee:    "0.00000200",
		Status: "pending",
	}

	t.Run("text format", func(t *testing.T) {
		t.Parallel()
		var buf bytes.Buffer
		cmd := newTestCmdWithContext(output.FormatText)
		cmd.SetOut(&buf)
		displayBTCTxResult(cmd, result, "main")
		assert.Contains(t, buf.String(), "Transaction broadcast successfully!")
		assert.Contains(t, buf.String(), "https://mempool.space/tx/deadbeef")
	})

	t.Run("json format", func(t *testing.T) {
		t.Parallel()
		var buf bytes.Buffer
		cmd := newTestCmdWithContext(output.FormatJSON)
		cmd.SetOut(&buf)
		displayBTCTxResult(cmd, result, "main")

		var parsed map[string]any
		require.NoError(t, json.Unmarshal(buf.Bytes(), &parsed))
		assert.Equal(t, "deadbeef", parsed["hash"])
		assert.Equal(t, "pending", parsed["status"])
	})
}

func TestDisplayBTCTxDetailsEnhanced(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		details      *btcConfirmationDetails
		wantContains []string
	}{
		{
			name: "single address normal send",
			details: &btcConfirmationDetails{
				To:              "1RecipientAddr",
				AmountSats:      150000,
				IsSweep:         false,
				EstimatedFee:    2260,
				FeeRate:         10,
				TotalUTXOs:      2,
				SourceAddresses: []string{"1SourceAddr"},
				AddressUTXOs:    map[string]int{"1SourceAddr": 2},
			},
			wantContains: []string{
				"TRANSACTION DETAILS",
				"From:      1SourceAddr",
				"To:        1RecipientAddr",
				"150,000 sats BTC",
				"UTXOs:     2",
				"Fee Rate:  10 sat/vB",
				"Est. Fee:  2,260 satoshis",
			},
		},
		{
			name: "multi-address sweep all",
			details: &btcConfirmationDetails{
				To:              "1RecipientAddr",
				AmountSats:      999774,
				IsSweep:         true,
				EstimatedFee:    226,
				FeeRate:         5,
				TotalUTXOs:      3,
				SourceAddresses: []string{"1AddrOne", "1AddrTwo"},
				AddressUTXOs: map[string]int{
					"1AddrOne": 1,
					"1AddrTwo": 2,
				},
			},
			wantContains: []string{
				"From:      2 addresses with UTXOs:",
				"• 1AddrOne (1 UTXO)",
				"• 1AddrTwo (2 UTXOs)",
				"999,774 sats (sweep all) BTC",
				"Total:     3 UTXOs",
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			var buf bytes.Buffer
			cmd := &cobra.Command{}
			cmd.SetContext(context.Background())
			cmd.SetOut(&buf)
			displayBTCTxDetailsEnhanced(cmd, tc.details)
			result := buf.String()

			for _, s := range tc.wantContains {
				assert.Contains(t, result, s)
			}
		})
	}
}
