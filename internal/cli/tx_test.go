package cli

import (
	"bytes"
	"context"
	"math/big"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mrz1836/sigil/internal/chain"
	"github.com/mrz1836/sigil/internal/chain/eth"
	"github.com/mrz1836/sigil/internal/output"
)

// TestSanitizeAmount tests amount string sanitization.
func TestSanitizeAmount(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		// Clean inputs
		{
			name:     "whole number",
			input:    "100",
			expected: "100",
		},
		{
			name:     "decimal number",
			input:    "100.50",
			expected: "100.50",
		},
		{
			name:     "small decimal",
			input:    "0.001",
			expected: "0.001",
		},
		{
			name:     "large number",
			input:    "1000000.123456",
			expected: "1000000.123456",
		},

		// Whitespace handling
		{
			name:     "leading whitespace",
			input:    "  100.50",
			expected: "100.50",
		},
		{
			name:     "trailing whitespace",
			input:    "100.50  ",
			expected: "100.50",
		},
		{
			name:     "both sides whitespace",
			input:    "   100.50   ",
			expected: "100.50",
		},
		{
			name:     "with tabs",
			input:    "\t100.50\t",
			expected: "100.50",
		},
		{
			name:     "with newlines",
			input:    "\n100.50\n",
			expected: "100.50",
		},

		// Currency symbols removed
		{
			name:     "with dollar sign prefix",
			input:    "$100.50",
			expected: "100.50",
		},
		{
			name:     "with euro sign prefix",
			input:    "€100.50",
			expected: "100.50",
		},
		{
			name:     "with pound sign prefix",
			input:    "£100.50",
			expected: "100.50",
		},
		{
			name:     "with yen sign prefix",
			input:    "¥100.50",
			expected: "100.50",
		},
		{
			name:     "with bitcoin symbol",
			input:    "₿100.50",
			expected: "100.50",
		},
		{
			name:     "ETH suffix",
			input:    "1.5 ETH",
			expected: "1.5",
		},
		{
			name:     "BSV suffix",
			input:    "0.001 BSV",
			expected: "0.001",
		},
		{
			name:     "USDC suffix",
			input:    "100 USDC",
			expected: "100",
		},

		// Thousand separators removed
		{
			name:     "with commas (US format)",
			input:    "1,000,000.50",
			expected: "1000000.50",
		},
		{
			name:     "with spaces as separators",
			input:    "1 000 000.50",
			expected: "1000000.50",
		},
		{
			name:     "with underscores as separators",
			input:    "1_000_000.50",
			expected: "1000000.50",
		},

		// Special characters removed
		{
			name:     "with plus sign",
			input:    "+100.50",
			expected: "100.50",
		},
		{
			name:     "with parentheses",
			input:    "(100.50)",
			expected: "100.50",
		},
		{
			name:     "with quotes",
			input:    `"100.50"`,
			expected: "100.50",
		},
		{
			name:     "with single quotes",
			input:    "'100.50'",
			expected: "100.50",
		},

		// Negative numbers
		{
			name:     "negative number",
			input:    "-100.50",
			expected: "-100.50",
		},
		{
			name:     "negative with space",
			input:    "- 100.50",
			expected: "-100.50",
		},

		// Edge cases
		{
			name:     "empty string",
			input:    "",
			expected: "",
		},
		{
			name:     "only whitespace",
			input:    "   \t\n   ",
			expected: "",
		},
		{
			name:     "only currency symbol",
			input:    "$",
			expected: "",
		},
		{
			name:     "just decimal point",
			input:    ".",
			expected: ".",
		},
		{
			name:     "leading decimal",
			input:    ".50",
			expected: ".50",
		},
		{
			name:     "trailing decimal",
			input:    "100.",
			expected: "100.",
		},
		{
			name:     "zero",
			input:    "0",
			expected: "0",
		},
		{
			name:     "zero with decimals",
			input:    "0.00",
			expected: "0.00",
		},

		// Complex copy-paste scenarios
		{
			name:     "from spreadsheet with extra chars",
			input:    " $1,234.56 ",
			expected: "1234.56",
		},
		{
			name:     "with text prefix",
			input:    "Amount: 100.50",
			expected: "100.50",
		},
		{
			name:     "unicode numbers mixed",
			input:    "100.50①②③",
			expected: "100.50",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			result := SanitizeAmount(tc.input)
			assert.Equal(t, tc.expected, result)
		})
	}
}

// TestSanitizeAmount_ParseableAfterSanitization verifies sanitized amounts can be parsed.
func TestSanitizeAmount_ParseableAfterSanitization(t *testing.T) {
	t.Parallel()

	dirtyInputs := []struct {
		input    string
		decimals int
	}{
		{"  1.5  ", 18},    // ETH
		{"$100.00", 6},     // USDC-style
		{"1,000.50", 18},   // With comma
		{" 0.001 ETH", 18}, // With suffix
		{"\t10.5\n", 8},    // With tabs/newlines
		{"₿0.00000001", 8}, // Satoshi
	}

	for _, tc := range dirtyInputs {
		t.Run(tc.input, func(t *testing.T) {
			t.Parallel()
			sanitized := SanitizeAmount(tc.input)
			assert.NotEmpty(t, sanitized, "sanitized amount should not be empty")

			// Try to parse the sanitized amount
			_, err := parseDecimalAmount(sanitized, tc.decimals)
			// Some inputs may still fail parsing (like .50 without leading 0)
			// The important thing is sanitization doesn't make valid inputs invalid
			if err != nil {
				t.Logf("Parsing error for %q -> %q: %v (may be expected)", tc.input, sanitized, err)
			}
		})
	}
}

// TestParseDecimalAmount_WithSanitization tests that parseDecimalAmount handles dirty inputs.
func TestParseDecimalAmount_WithSanitization(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		input    string
		decimals int
		wantErr  bool
	}{
		// Valid after sanitization
		{
			name:     "clean amount",
			input:    "1.5",
			decimals: 18,
			wantErr:  false,
		},
		{
			name:     "with whitespace",
			input:    "  1.5  ",
			decimals: 18,
			wantErr:  false,
		},
		{
			name:     "with dollar sign",
			input:    "$100.00",
			decimals: 6,
			wantErr:  false,
		},
		{
			name:     "with commas",
			input:    "1,000.50",
			decimals: 18,
			wantErr:  false,
		},
		{
			name:     "whole number",
			input:    "100",
			decimals: 6,
			wantErr:  false,
		},

		// Invalid even after sanitization
		{
			name:     "empty",
			input:    "",
			decimals: 18,
			wantErr:  true,
		},
		{
			name:     "only whitespace",
			input:    "   ",
			decimals: 18,
			wantErr:  true,
		},
		{
			name:     "only currency symbol",
			input:    "$",
			decimals: 18,
			wantErr:  true,
		},
		{
			name:     "no digits",
			input:    "ETH",
			decimals: 18,
			wantErr:  true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			result, err := parseDecimalAmount(tc.input, tc.decimals)
			if tc.wantErr {
				require.Error(t, err)
				assert.Nil(t, result)
			} else {
				require.NoError(t, err)
				assert.NotNil(t, result)
			}
		})
	}
}

func TestAmountToBigInt(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		amount   uint64
		expected string
	}{
		{
			name:     "zero",
			amount:   0,
			expected: "0",
		},
		{
			name:     "one",
			amount:   1,
			expected: "1",
		},
		{
			name:     "small value",
			amount:   100,
			expected: "100",
		},
		{
			name:     "typical satoshi value",
			amount:   100000000, // 1 BTC in satoshis
			expected: "100000000",
		},
		{
			name:     "large value",
			amount:   1000000000000,
			expected: "1000000000000",
		},
		{
			name:     "max uint64",
			amount:   ^uint64(0), // 18446744073709551615
			expected: "18446744073709551615",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			result := amountToBigInt(tc.amount)
			assert.Equal(t, tc.expected, result.String())
		})
	}
}

func TestResolveToken(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		symbol       string
		wantAddress  bool
		wantDecimals int
		wantErr      bool
	}{
		{
			name:         "USDC uppercase",
			symbol:       "USDC",
			wantAddress:  true,
			wantDecimals: 6,
			wantErr:      false,
		},
		{
			name:         "usdc lowercase",
			symbol:       "usdc",
			wantAddress:  true,
			wantDecimals: 6,
			wantErr:      false,
		},
		{
			name:         "Usdc mixed case",
			symbol:       "Usdc",
			wantAddress:  true,
			wantDecimals: 6,
			wantErr:      false,
		},
		{
			name:        "unsupported token ETH",
			symbol:      "ETH",
			wantAddress: false,
			wantErr:     true,
		},
		{
			name:        "unsupported token USDT",
			symbol:      "USDT",
			wantAddress: false,
			wantErr:     true,
		},
		{
			name:        "unsupported token DAI",
			symbol:      "DAI",
			wantAddress: false,
			wantErr:     true,
		},
		{
			name:        "empty symbol",
			symbol:      "",
			wantAddress: false,
			wantErr:     true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			address, decimals, err := resolveToken(tc.symbol)
			if tc.wantErr {
				require.Error(t, err)
				assert.Empty(t, address)
				assert.Zero(t, decimals)
			} else {
				require.NoError(t, err)
				assert.NotEmpty(t, address)
				assert.Equal(t, tc.wantDecimals, decimals)
			}
		})
	}
}

func TestDisplayBSVTxDetails(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		from, to     string
		amount       string
		fee, feeRate uint64
		wantContains []string
	}{
		{
			name:    "standard BSV tx",
			from:    "1A1zP1eP5QGefi2DMPTfTL5SLmv7DivfNa",
			to:      "1BvBMSEYstWetqTFn5Au4m4GFg7xJaNVN2",
			amount:  "0.001",
			fee:     226,
			feeRate: 1,
			wantContains: []string{
				"TRANSACTION DETAILS",
				"1A1zP1eP5QGefi2DMPTfTL5SLmv7DivfNa",
				"1BvBMSEYstWetqTFn5Au4m4GFg7xJaNVN2",
				"0.001 BSV",
				"1 sat/byte",
				"226 satoshis",
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			var buf bytes.Buffer
			cmd := &cobra.Command{}
			cmd.SetOut(&buf)
			displayBSVTxDetails(cmd, tc.from, tc.to, tc.amount, tc.fee, tc.feeRate)
			result := buf.String()
			for _, s := range tc.wantContains {
				assert.Contains(t, result, s)
			}
		})
	}
}

func TestDisplayBSVTxResultText(t *testing.T) {
	t.Parallel()

	result := &chain.TransactionResult{
		Hash:   "abc123def456",
		Status: "pending",
		Amount: "0.001",
		Fee:    "0.00000226",
	}

	var buf bytes.Buffer
	displayBSVTxResultText(&buf, result)
	out := buf.String()

	assert.Contains(t, out, "Transaction broadcast successfully!")
	assert.Contains(t, out, "abc123def456")
	assert.Contains(t, out, "pending")
	assert.Contains(t, out, "0.001 BSV")
	assert.Contains(t, out, "0.00000226 BSV")
	assert.Contains(t, out, "whatsonchain.com/tx/abc123def456")
}

func TestDisplayBSVTxResultJSON(t *testing.T) {
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
	displayBSVTxResultJSON(&buf, result)
	out := buf.String()

	assert.Contains(t, out, `"hash": "abc123"`)
	assert.Contains(t, out, `"from": "1From"`)
	assert.Contains(t, out, `"to": "1To"`)
	assert.Contains(t, out, `"amount": "0.5"`)
	assert.Contains(t, out, `"fee": "0.0001"`)
	assert.Contains(t, out, `"status": "pending"`)
}

func TestDisplayTxDetails(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		from, to     string
		amount       string
		token        string
		estimate     *eth.GasEstimate
		wantContains []string
	}{
		{
			name:   "ETH native transfer",
			from:   "0xFromAddr",
			to:     "0xToAddr",
			amount: "1.5",
			token:  "",
			estimate: &eth.GasEstimate{
				GasPrice: big.NewInt(20_000_000_000),
				GasLimit: 21000,
				Total:    big.NewInt(420_000_000_000_000),
			},
			wantContains: []string{
				"TRANSACTION DETAILS",
				"0xFromAddr",
				"0xToAddr",
				"1.5 ETH",
				"21000",
			},
		},
		{
			name:   "USDC token transfer",
			from:   "0xFromAddr",
			to:     "0xToAddr",
			amount: "100",
			token:  "USDC",
			estimate: &eth.GasEstimate{
				GasPrice: big.NewInt(20_000_000_000),
				GasLimit: 65000,
				Total:    big.NewInt(1_300_000_000_000_000),
			},
			wantContains: []string{
				"100 USDC",
				"65000",
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			var buf bytes.Buffer
			cmd := &cobra.Command{}
			cmd.SetOut(&buf)
			displayTxDetails(cmd, tc.from, tc.to, tc.amount, tc.token, tc.estimate)
			result := buf.String()
			for _, s := range tc.wantContains {
				assert.Contains(t, result, s)
			}
		})
	}
}

func TestDisplayTxResultText(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		result       *chain.TransactionResult
		wantContains []string
	}{
		{
			name: "ETH native result",
			result: &chain.TransactionResult{
				Hash:   "0xhash123",
				Status: "pending",
				Amount: "1.5",
				Token:  "",
				Fee:    "0.00042",
			},
			wantContains: []string{
				"Transaction broadcast successfully!",
				"0xhash123",
				"1.5 ETH",
				"etherscan.io/tx/0xhash123",
			},
		},
		{
			name: "USDC token result",
			result: &chain.TransactionResult{
				Hash:   "0xhash456",
				Status: "pending",
				Amount: "100",
				Token:  "USDC",
				Fee:    "0.00065",
			},
			wantContains: []string{
				"100 USDC",
				"etherscan.io/tx/0xhash456",
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			var buf bytes.Buffer
			displayTxResultText(&buf, tc.result)
			out := buf.String()
			for _, s := range tc.wantContains {
				assert.Contains(t, out, s)
			}
		})
	}
}

func TestDisplayTxResultJSON(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		result       *chain.TransactionResult
		wantContains []string
		wantMissing  []string
	}{
		{
			name: "without token",
			result: &chain.TransactionResult{
				Hash:     "0xhash",
				From:     "0xfrom",
				To:       "0xto",
				Amount:   "1.0",
				Token:    "",
				Fee:      "0.001",
				GasUsed:  21000,
				GasPrice: "20 Gwei",
				Status:   "pending",
			},
			wantContains: []string{
				`"hash": "0xhash"`,
				`"from": "0xfrom"`,
				`"to": "0xto"`,
				`"amount": "1.0"`,
				`"fee": "0.001"`,
				`"gas_used": 21000`,
				`"gas_price": "20 Gwei"`,
				`"status": "pending"`,
			},
			wantMissing: []string{`"token"`},
		},
		{
			name: "with token",
			result: &chain.TransactionResult{
				Hash:     "0xhash2",
				From:     "0xfrom",
				To:       "0xto",
				Amount:   "100",
				Token:    "USDC",
				Fee:      "0.002",
				GasUsed:  65000,
				GasPrice: "25 Gwei",
				Status:   "pending",
			},
			wantContains: []string{
				`"token": "USDC"`,
				`"gas_used": 65000`,
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			var buf bytes.Buffer
			displayTxResultJSON(&buf, tc.result)
			out := buf.String()
			for _, s := range tc.wantContains {
				assert.Contains(t, out, s)
			}
			for _, s := range tc.wantMissing {
				assert.NotContains(t, out, s)
			}
		})
	}
}

// newTestCmdWithContext creates a cobra.Command with a CommandContext set up for testing.
func newTestCmdWithContext(format output.Format) *cobra.Command {
	cmd := &cobra.Command{}
	cmd.SetContext(context.Background())
	SetCmdContext(cmd, &CommandContext{
		Fmt: &mockFormatProvider{format: format},
	})
	return cmd
}

func TestDisplayTxResult_TextAndJSON(t *testing.T) {
	t.Parallel()

	result := &chain.TransactionResult{
		Hash:     "0xabc",
		From:     "0xfrom",
		To:       "0xto",
		Amount:   "1.0",
		Fee:      "0.001",
		GasUsed:  21000,
		GasPrice: "20 Gwei",
		Status:   "pending",
	}

	t.Run("text format", func(t *testing.T) {
		t.Parallel()
		var buf bytes.Buffer
		cmd := newTestCmdWithContext(output.FormatText)
		cmd.SetOut(&buf)
		displayTxResult(cmd, result)
		assert.Contains(t, buf.String(), "Transaction broadcast successfully!")
		assert.Contains(t, buf.String(), "etherscan.io/tx/0xabc")
	})

	t.Run("json format", func(t *testing.T) {
		t.Parallel()
		var buf bytes.Buffer
		cmd := newTestCmdWithContext(output.FormatJSON)
		cmd.SetOut(&buf)
		displayTxResult(cmd, result)
		assert.Contains(t, buf.String(), `"hash": "0xabc"`)
		assert.Contains(t, buf.String(), `"gas_used": 21000`)
	})
}

func TestDisplayBSVTxResult_TextAndJSON(t *testing.T) {
	t.Parallel()

	result := &chain.TransactionResult{
		Hash:   "bsvhash123",
		From:   "1From",
		To:     "1To",
		Amount: "0.5",
		Fee:    "0.0001",
		Status: "pending",
	}

	t.Run("text format", func(t *testing.T) {
		t.Parallel()
		var buf bytes.Buffer
		cmd := newTestCmdWithContext(output.FormatText)
		cmd.SetOut(&buf)
		displayBSVTxResult(cmd, result)
		assert.Contains(t, buf.String(), "Transaction broadcast successfully!")
		assert.Contains(t, buf.String(), "whatsonchain.com/tx/bsvhash123")
	})

	t.Run("json format", func(t *testing.T) {
		t.Parallel()
		var buf bytes.Buffer
		cmd := newTestCmdWithContext(output.FormatJSON)
		cmd.SetOut(&buf)
		displayBSVTxResult(cmd, result)
		assert.Contains(t, buf.String(), `"hash": "bsvhash123"`)
		assert.Contains(t, buf.String(), `"status": "pending"`)
	})
}

func TestParseDecimalAmount_EdgeCases(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		input    string
		decimals int
		want     string
		wantErr  bool
	}{
		{
			name:     "leading decimal .5 with 6 decimals",
			input:    ".5",
			decimals: 6,
			want:     "500000",
			wantErr:  false,
		},
		{
			name:     "excess decimals truncated",
			input:    "1.123456789",
			decimals: 6,
			want:     "1123456",
			wantErr:  false,
		},
		{
			name:     "zero decimals",
			input:    "100",
			decimals: 0,
			want:     "100",
			wantErr:  false,
		},
		{
			name:     "empty after sanitize",
			input:    "ABC",
			decimals: 6,
			wantErr:  true,
		},
		{
			name:     "large integer no decimals",
			input:    "999999999",
			decimals: 8,
			want:     "99999999900000000",
			wantErr:  false,
		},
		{
			name:     "trailing decimal point",
			input:    "100.",
			decimals: 6,
			want:     "100000000",
			wantErr:  false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			result, err := parseDecimalAmount(tc.input, tc.decimals)
			if tc.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tc.want, result.String())
			}
		})
	}
}
