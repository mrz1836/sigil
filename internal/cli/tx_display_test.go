package cli

import (
	"bytes"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"

	"github.com/mrz1836/sigil/internal/chain"
	"github.com/mrz1836/sigil/internal/output"
	"github.com/mrz1836/sigil/internal/service/transaction"
)

func TestConvertToETHTransactionResult(t *testing.T) {
	t.Parallel()

	result := &transaction.SendResult{
		Hash:     "0xhash",
		From:     "0xfrom",
		To:       "0xto",
		Amount:   "100",
		Fee:      "0.00065",
		Token:    "USDC",
		Status:   "pending",
		GasUsed:  65000,
		GasPrice: "25 Gwei",
	}

	converted := convertToETHTransactionResult(result)

	// ETH conversions preserve gas + token fields.
	assert.Equal(t, "0xhash", converted.Hash)
	assert.Equal(t, "0xfrom", converted.From)
	assert.Equal(t, "0xto", converted.To)
	assert.Equal(t, "100", converted.Amount)
	assert.Equal(t, "0.00065", converted.Fee)
	assert.Equal(t, "USDC", converted.Token)
	assert.Equal(t, "pending", converted.Status)
	assert.Equal(t, uint64(65000), converted.GasUsed)
	assert.Equal(t, "25 Gwei", converted.GasPrice)
}

func TestConvertToBSVTransactionResult(t *testing.T) {
	t.Parallel()

	// Gas + token are ETH-only and must be dropped for BSV.
	result := &transaction.SendResult{
		Hash:     "bsvhash",
		From:     "1From",
		To:       "1To",
		Amount:   "0.5",
		Fee:      "0.0001",
		Token:    "USDC",
		Status:   "pending",
		GasUsed:  21000,
		GasPrice: "20 Gwei",
	}

	converted := convertToBSVTransactionResult(result)

	assert.Equal(t, "bsvhash", converted.Hash)
	assert.Equal(t, "1From", converted.From)
	assert.Equal(t, "1To", converted.To)
	assert.Equal(t, "0.5", converted.Amount)
	assert.Equal(t, "0.0001", converted.Fee)
	assert.Equal(t, "pending", converted.Status)
	assert.Empty(t, converted.Token)
	assert.Zero(t, converted.GasUsed)
	assert.Empty(t, converted.GasPrice)
}

func TestDisplayBSVTxDetailsEnhanced(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		details      *bsvConfirmationDetails
		wantContains []string
	}{
		{
			name: "single address normal send",
			details: &bsvConfirmationDetails{
				To:              "1RecipientAddr",
				AmountSats:      100000,
				IsSweep:         false,
				EstimatedFee:    226,
				FeeRate:         500,
				TotalUTXOs:      1,
				SourceAddresses: []string{"1SourceAddr"},
				AddressUTXOs:    map[string]int{"1SourceAddr": 1},
			},
			wantContains: []string{
				"TRANSACTION DETAILS",
				"From:      1SourceAddr",
				"To:        1RecipientAddr",
				"100,000 sats BSV",
				"UTXOs:     1",
				"Fee Rate:  500 sat/KB",
				"Est. Fee:  226 satoshis",
			},
		},
		{
			name: "multi address sweep",
			details: &bsvConfirmationDetails{
				To:              "1RecipientAddr",
				AmountSats:      2500000,
				IsSweep:         true,
				EstimatedFee:    452,
				FeeRate:         500,
				TotalUTXOs:      4,
				SourceAddresses: []string{"1AddrA", "1AddrB"},
				AddressUTXOs: map[string]int{
					"1AddrA": 1,
					"1AddrB": 3,
				},
			},
			wantContains: []string{
				"From:      2 addresses with UTXOs:",
				"• 1AddrA (1 UTXO)",
				"• 1AddrB (3 UTXOs)",
				"2,500,000 sats (sweep all) BSV",
				"Total:     4 UTXOs",
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			var buf bytes.Buffer
			cmd := &cobra.Command{}
			cmd.SetOut(&buf)
			displayBSVTxDetailsEnhanced(cmd, tc.details)
			result := buf.String()

			for _, s := range tc.wantContains {
				assert.Contains(t, result, s)
			}
		})
	}
}

func TestFormatSatsWithCommas(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		sats uint64
		want string
	}{
		{name: "zero", sats: 0, want: "0"},
		{name: "single digit", sats: 7, want: "7"},
		{name: "hundreds", sats: 100, want: "100"},
		{name: "exactly one thousand", sats: 1000, want: "1,000"},
		{name: "five digits", sats: 12345, want: "12,345"},
		{name: "seven digits", sats: 1234567, want: "1,234,567"},
		{name: "one hundred million", sats: 100000000, want: "100,000,000"},
		{name: "max uint64", sats: 18446744073709551615, want: "18,446,744,073,709,551,615"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tc.want, formatSatsWithCommas(tc.sats))
		})
	}
}

func TestPluralize(t *testing.T) {
	t.Parallel()

	tests := []struct {
		count int
		want  string
	}{
		{count: 0, want: "s"},
		{count: 1, want: ""},
		{count: 2, want: "s"},
		{count: 100, want: "s"},
	}

	for _, tc := range tests {
		assert.Equal(t, tc.want, pluralize(tc.count), "count=%d", tc.count)
	}
}

func TestDisplaySendResult_RoutesByChain(t *testing.T) {
	t.Parallel()

	result := &transaction.SendResult{
		Hash:   "routehash",
		From:   "from",
		To:     "to",
		Amount: "1.0",
		Fee:    "0.001",
		Status: "pending",
	}

	tests := []struct {
		name     string
		chainID  chain.ID
		network  string
		wantLink string
	}{
		{name: "BSV routes to whatsonchain", chainID: chain.BSV, network: "main", wantLink: "whatsonchain.com/tx/routehash"},
		{name: "BTC routes to mempool.space", chainID: chain.BTC, network: "main", wantLink: "mempool.space/tx/routehash"},
		{name: "ETH routes to etherscan", chainID: chain.ETH, network: "main", wantLink: "etherscan.io/tx/routehash"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			var buf bytes.Buffer
			cmd := newTestCmdWithContext(output.FormatText)
			cmd.SetOut(&buf)
			displaySendResult(cmd, tc.chainID, result, tc.network)
			out := buf.String()

			assert.Contains(t, out, "Transaction broadcast successfully!")
			assert.Contains(t, out, tc.wantLink)
		})
	}
}
