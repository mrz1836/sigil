package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"math/big"
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mrz1836/sigil/internal/cache"
	"github.com/mrz1836/sigil/internal/chain"
	"github.com/mrz1836/sigil/internal/chain/eth"
	"github.com/mrz1836/sigil/internal/output"
	"github.com/mrz1836/sigil/internal/utxostore"
	"github.com/mrz1836/sigil/internal/wallet"
)

// TestIsAmountAll tests the isAmountAll helper function.
func TestIsAmountAll(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		input    string
		expected bool
	}{
		// Positive cases
		{name: "lowercase all", input: "all", expected: true},
		{name: "uppercase ALL", input: "ALL", expected: true},
		{name: "mixed case All", input: "All", expected: true},
		{name: "mixed case aLl", input: "aLl", expected: true},
		{name: "with leading space", input: "  all", expected: true},
		{name: "with trailing space", input: "all  ", expected: true},
		{name: "with surrounding space", input: "  all  ", expected: true},
		{name: "with tab", input: "\tall\t", expected: true},

		// Negative cases
		{name: "numeric amount", input: "0.5", expected: false},
		{name: "empty string", input: "", expected: false},
		{name: "partial match alll", input: "alll", expected: false},
		{name: "partial match all1", input: "all1", expected: false},
		{name: "partial match al", input: "al", expected: false},
		{name: "word containing all", input: "wallet", expected: false},
		{name: "max keyword", input: "max", expected: false},
		{name: "zero", input: "0", expected: false},
		{name: "whitespace only", input: "   ", expected: false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			result := isAmountAll(tc.input)
			assert.Equal(t, tc.expected, result)
		})
	}
}

// TestAmountAllConstant verifies the amountAll constant value.
func TestAmountAllConstant(t *testing.T) {
	t.Parallel()
	assert.Equal(t, "all", amountAll)
}

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
			name:     "with tabs and newlines",
			input:    "\t100.50\n",
			expected: "100.50",
		},

		// Non-numeric characters are preserved (validation happens later)
		{
			name:     "currency symbol preserved",
			input:    "$100.50",
			expected: "$100.50",
		},
		{
			name:     "negative preserved",
			input:    "-100.50",
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
			name:     "just decimal point",
			input:    ".",
			expected: ".",
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

	inputs := []struct {
		input    string
		decimals int
	}{
		{"  1.5  ", 18}, // ETH
		{"\t10.5\n", 8}, // With tabs/newlines
		{"0.001", 8},    // Clean value
	}

	for _, tc := range inputs {
		t.Run(tc.input, func(t *testing.T) {
			t.Parallel()
			sanitized := SanitizeAmount(tc.input)
			assert.NotEmpty(t, sanitized, "sanitized amount should not be empty")

			// Try to parse the sanitized amount
			_, err := parseDecimalAmount(sanitized, tc.decimals)
			require.NoError(t, err)
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
		// Valid inputs
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
			name:     "whole number",
			input:    "100",
			decimals: 6,
			wantErr:  false,
		},
		{
			name:     "leading decimal",
			input:    ".50",
			decimals: 6,
			wantErr:  false,
		},
		{
			name:     "trailing decimal",
			input:    "100.",
			decimals: 6,
			wantErr:  false,
		},

		// Invalid inputs
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
		{
			name:     "currency symbol",
			input:    "$100.00",
			decimals: 6,
			wantErr:  true,
		},
		{
			name:     "commas",
			input:    "1,000.50",
			decimals: 18,
			wantErr:  true,
		},
		{
			name:     "negative amount",
			input:    "-1.5",
			decimals: 18,
			wantErr:  true,
		},
		{
			name:     "scientific notation",
			input:    "1e3",
			decimals: 18,
			wantErr:  true,
		},
		{
			name:     "multiple decimals",
			input:    "1.2.3",
			decimals: 18,
			wantErr:  true,
		},
		{
			name:     "just decimal point",
			input:    ".",
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
			result := chain.AmountToBigInt(tc.amount)
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
				"1 sat/KB",
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

	var parsed map[string]any
	require.NoError(t, json.Unmarshal(buf.Bytes(), &parsed))
	assert.Equal(t, "abc123", parsed["hash"])
	assert.Equal(t, "1From", parsed["from"])
	assert.Equal(t, "1To", parsed["to"])
	assert.Equal(t, "0.5", parsed["amount"])
	assert.Equal(t, "0.0001", parsed["fee"])
	assert.Equal(t, "pending", parsed["status"])
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

			var parsed map[string]any
			require.NoError(t, json.Unmarshal(buf.Bytes(), &parsed))
			assert.Equal(t, tc.result.Hash, parsed["hash"])
			assert.Equal(t, tc.result.From, parsed["from"])
			assert.Equal(t, tc.result.To, parsed["to"])
			assert.Equal(t, tc.result.Amount, parsed["amount"])
			assert.Equal(t, tc.result.Fee, parsed["fee"])
			assert.Equal(t, tc.result.GasPrice, parsed["gas_price"])

			gasUsed, ok := parsed["gas_used"].(float64)
			require.True(t, ok)
			assert.InDelta(t, float64(tc.result.GasUsed), gasUsed, 0)

			if tc.result.Token == "" {
				_, hasToken := parsed["token"]
				assert.False(t, hasToken)
			} else {
				assert.Equal(t, tc.result.Token, parsed["token"])
			}
		})
	}
}

func TestDisplayTxResultJSON_Escaping(t *testing.T) {
	t.Parallel()

	result := &chain.TransactionResult{
		Hash:     "0x\"hash",
		From:     "0xfrom\nline",
		To:       "0xto\u2713",
		Amount:   "1.0",
		Token:    "USDC",
		Fee:      "0.001",
		GasUsed:  21000,
		GasPrice: "20 \"Gwei\"",
		Status:   "pending",
	}

	var buf bytes.Buffer
	displayTxResultJSON(&buf, result)

	var parsed chain.TransactionResult
	require.NoError(t, json.Unmarshal(buf.Bytes(), &parsed))
	assert.Equal(t, result.Hash, parsed.Hash)
	assert.Equal(t, result.From, parsed.From)
	assert.Equal(t, result.To, parsed.To)
	assert.Equal(t, result.GasPrice, parsed.GasPrice)
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

// TestLogCacheError tests the logCacheError helper with nil and non-nil loggers.
func TestLogCacheError(t *testing.T) {
	t.Parallel()

	t.Run("nil logger does not panic", func(t *testing.T) {
		t.Parallel()

		cc := &CommandContext{Log: nil}
		assert.NotPanics(t, func() {
			logCacheError(cc, "test error: %v", "some detail")
		})
	})

	t.Run("non-nil logger records error", func(t *testing.T) {
		t.Parallel()

		logger := &txTestLogWriter{}
		cc := &CommandContext{Log: logger}

		logCacheError(cc, "failed to load cache: %v", "disk full")

		require.Len(t, logger.errorCalls, 1)
		assert.Equal(t, "failed to load cache: %v", logger.errorCalls[0])
	})
}

// txTestLogWriter implements LogWriter for tx_test.go.
type txTestLogWriter struct {
	errorCalls []string
}

func (m *txTestLogWriter) Debug(_ string, _ ...any) {}
func (m *txTestLogWriter) Error(format string, _ ...any) {
	m.errorCalls = append(m.errorCalls, format)
}
func (m *txTestLogWriter) Close() error { return nil }

// TestInvalidateBalanceCache_SweepAll verifies that after a sweep-all send the
// cached balance is set to "0.0" and the entry is preserved on disk.
func TestInvalidateBalanceCache_SweepAll(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()

	// Pre-populate cache with a BSV balance.
	cachePath := filepath.Join(tmpDir, "cache", "balances.json")
	storage := cache.NewFileStorage(cachePath)
	bc := cache.NewBalanceCache()
	bc.Set(cache.BalanceCacheEntry{
		Chain:    chain.BSV,
		Address:  "1abc",
		Balance:  "0.5",
		Symbol:   "BSV",
		Decimals: 8,
	})
	require.NoError(t, storage.Save(bc))

	cc := &CommandContext{
		Cfg: &mockConfigProvider{home: tmpDir},
	}

	// Sweep-all: expected balance is "0.0".
	invalidateBalanceCache(cc, chain.BSV, "1abc", "", "0.0")

	loaded, err := storage.Load()
	require.NoError(t, err)
	entry, exists, _ := loaded.Get(chain.BSV, "1abc", "")
	require.True(t, exists, "cache entry should still exist")
	assert.Equal(t, "0.0", entry.Balance)
	assert.Equal(t, "BSV", entry.Symbol)
	assert.Equal(t, 8, entry.Decimals)
}

// TestInvalidateBalanceCache_PartialSend verifies that a partial send deletes
// the cache entry so the next balance query must fetch from the network.
func TestInvalidateBalanceCache_PartialSend(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()

	cachePath := filepath.Join(tmpDir, "cache", "balances.json")
	storage := cache.NewFileStorage(cachePath)
	bc := cache.NewBalanceCache()
	bc.Set(cache.BalanceCacheEntry{
		Chain:    chain.BSV,
		Address:  "1abc",
		Balance:  "1.0",
		Symbol:   "BSV",
		Decimals: 8,
	})
	require.NoError(t, storage.Save(bc))

	cc := &CommandContext{
		Cfg: &mockConfigProvider{home: tmpDir},
	}

	// Partial send: empty expectedBalance triggers deletion.
	invalidateBalanceCache(cc, chain.BSV, "1abc", "", "")

	loaded, err := storage.Load()
	require.NoError(t, err)
	_, exists, _ := loaded.Get(chain.BSV, "1abc", "")
	assert.False(t, exists, "cache entry should be deleted after partial send")
}

// TestInvalidateBalanceCache_PreservesOtherEntries verifies that invalidating
// one address does not affect other cached entries.
func TestInvalidateBalanceCache_PreservesOtherEntries(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()

	cachePath := filepath.Join(tmpDir, "cache", "balances.json")
	storage := cache.NewFileStorage(cachePath)
	bc := cache.NewBalanceCache()
	bc.Set(cache.BalanceCacheEntry{
		Chain: chain.BSV, Address: "1abc", Balance: "0.5", Symbol: "BSV", Decimals: 8,
	})
	bc.Set(cache.BalanceCacheEntry{
		Chain: chain.ETH, Address: "0x123", Balance: "2.0", Symbol: "ETH", Decimals: 18,
	})
	require.NoError(t, storage.Save(bc))

	cc := &CommandContext{
		Cfg: &mockConfigProvider{home: tmpDir},
	}

	invalidateBalanceCache(cc, chain.BSV, "1abc", "", "0.0")

	loaded, err := storage.Load()
	require.NoError(t, err)

	// BSV entry updated.
	bsvEntry, exists, _ := loaded.Get(chain.BSV, "1abc", "")
	require.True(t, exists)
	assert.Equal(t, "0.0", bsvEntry.Balance)

	// ETH entry untouched.
	ethEntry, exists, _ := loaded.Get(chain.ETH, "0x123", "")
	require.True(t, exists)
	assert.Equal(t, "2.0", ethEntry.Balance)
}

// TestInvalidateBalanceCache_NoCacheFile verifies no panic or error when the
// cache file doesn't exist yet.
func TestInvalidateBalanceCache_NoCacheFile(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	cc := &CommandContext{
		Cfg: &mockConfigProvider{home: tmpDir},
	}

	// Should not panic — Load returns an empty cache when file is absent.
	invalidateBalanceCache(cc, chain.BSV, "1abc", "", "0.0")

	// Verify the cache file was created with the entry.
	cachePath := filepath.Join(tmpDir, "cache", "balances.json")
	storage := cache.NewFileStorage(cachePath)
	loaded, err := storage.Load()
	require.NoError(t, err)
	entry, exists, _ := loaded.Get(chain.BSV, "1abc", "")
	require.True(t, exists)
	assert.Equal(t, "0.0", entry.Balance)
}

// TestInvalidateBalanceCache_ETHTokenSweep verifies both native and token
// entries are handled for an ERC-20 sweep.
func TestInvalidateBalanceCache_ETHTokenSweep(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()

	cachePath := filepath.Join(tmpDir, "cache", "balances.json")
	storage := cache.NewFileStorage(cachePath)
	bc := cache.NewBalanceCache()
	bc.Set(cache.BalanceCacheEntry{
		Chain: chain.ETH, Address: "0xABC", Balance: "1.5", Symbol: "ETH", Decimals: 18,
	})
	bc.Set(cache.BalanceCacheEntry{
		Chain: chain.ETH, Address: "0xABC", Token: eth.USDCMainnet,
		Balance: "500.0", Symbol: "USDC", Decimals: 6,
	})
	require.NoError(t, storage.Save(bc))

	cc := &CommandContext{
		Cfg: &mockConfigProvider{home: tmpDir},
	}

	// Token sweep: zero out token, delete native (gas spent, amount unknown).
	invalidateBalanceCache(cc, chain.ETH, "0xABC", eth.USDCMainnet, "0.0")
	invalidateBalanceCache(cc, chain.ETH, "0xABC", "", "")

	loaded, err := storage.Load()
	require.NoError(t, err)

	// USDC set to 0.0.
	usdcEntry, exists, _ := loaded.Get(chain.ETH, "0xABC", eth.USDCMainnet)
	require.True(t, exists)
	assert.Equal(t, "0.0", usdcEntry.Balance)

	// ETH native deleted.
	_, exists, _ = loaded.Get(chain.ETH, "0xABC", "")
	assert.False(t, exists, "native ETH entry should be deleted")
}

// TestInvalidateBalanceCache_InvalidHome verifies graceful handling when the
// home path is invalid (e.g. points to a file instead of a directory).
func TestInvalidateBalanceCache_InvalidHome(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	// Create a regular file where the "cache" directory would need to be.
	blocker := filepath.Join(tmpDir, "cache")
	require.NoError(t, os.WriteFile(blocker, []byte("block"), 0o600))

	cc := &CommandContext{
		Cfg: &mockConfigProvider{home: tmpDir},
	}

	// Should not panic even when the cache directory cannot be created.
	assert.NotPanics(t, func() {
		invalidateBalanceCache(cc, chain.BSV, "1abc", "", "0.0")
	})
}

// TestDeriveKeysForUTXOs tests that private keys are derived only for addresses with UTXOs.
func TestDeriveKeysForUTXOs(t *testing.T) {
	t.Parallel()

	// Create a real seed for derivation
	seed, err := wallet.MnemonicToSeed("abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon about", "")
	require.NoError(t, err)

	// Derive three addresses so we know their expected indices
	a0, err := wallet.DeriveAddress(seed, wallet.ChainBSV, 0, 0)
	require.NoError(t, err)
	a1, err := wallet.DeriveAddress(seed, wallet.ChainBSV, 0, 1)
	require.NoError(t, err)
	a2, err := wallet.DeriveAddress(seed, wallet.ChainBSV, 0, 2)
	require.NoError(t, err)

	addresses := []wallet.Address{
		{Address: a0.Address, Index: 0},
		{Address: a1.Address, Index: 1},
		{Address: a2.Address, Index: 2},
	}

	t.Run("derives keys only for addresses with UTXOs", func(t *testing.T) {
		t.Parallel()

		// UTXOs only on address 0 and 2
		utxos := []chain.UTXO{
			{TxID: "aa", Amount: 50000, Address: a0.Address},
			{TxID: "bb", Amount: 30000, Address: a2.Address},
		}

		keys, err := deriveKeysForUTXOs(utxos, addresses, seed)
		require.NoError(t, err)
		defer func() {
			for _, k := range keys {
				wallet.ZeroBytes(k)
			}
		}()

		// Should have keys for a0 and a2 but NOT a1
		assert.Len(t, keys, 2)
		assert.Contains(t, keys, a0.Address)
		assert.Contains(t, keys, a2.Address)
		assert.NotContains(t, keys, a1.Address)

		// Keys should be 32 bytes
		assert.Len(t, keys[a0.Address], 32)
		assert.Len(t, keys[a2.Address], 32)
	})

	t.Run("single address with multiple UTXOs derives one key", func(t *testing.T) {
		t.Parallel()

		utxos := []chain.UTXO{
			{TxID: "cc", Amount: 10000, Address: a1.Address},
			{TxID: "dd", Amount: 20000, Address: a1.Address},
			{TxID: "ee", Amount: 30000, Address: a1.Address},
		}

		keys, err := deriveKeysForUTXOs(utxos, addresses, seed)
		require.NoError(t, err)
		defer func() {
			for _, k := range keys {
				wallet.ZeroBytes(k)
			}
		}()

		assert.Len(t, keys, 1)
		assert.Contains(t, keys, a1.Address)
	})

	t.Run("unknown address returns error and zeros keys", func(t *testing.T) {
		t.Parallel()

		utxos := []chain.UTXO{
			{TxID: "ff", Amount: 50000, Address: a0.Address},
			{TxID: "gg", Amount: 30000, Address: "1UnknownAddressNotInWallet"},
		}

		keys, err := deriveKeysForUTXOs(utxos, addresses, seed)
		require.Error(t, err)
		assert.Nil(t, keys)
		assert.Contains(t, err.Error(), "not found in wallet")
	})

	t.Run("empty UTXOs returns empty key map", func(t *testing.T) {
		t.Parallel()

		keys, err := deriveKeysForUTXOs([]chain.UTXO{}, addresses, seed)
		require.NoError(t, err)
		assert.Empty(t, keys)
	})
}

// TestUniqueUTXOAddrs tests the unique address extraction from UTXOs.
func TestUniqueUTXOAddrs(t *testing.T) {
	t.Parallel()

	t.Run("deduplicates addresses", func(t *testing.T) {
		t.Parallel()

		utxos := []chain.UTXO{
			{Address: "addr1"},
			{Address: "addr2"},
			{Address: "addr1"}, // duplicate
			{Address: "addr3"},
			{Address: "addr2"}, // duplicate
		}

		result := uniqueUTXOAddrs(utxos)
		assert.Len(t, result, 3)
		assert.Contains(t, result, "addr1")
		assert.Contains(t, result, "addr2")
		assert.Contains(t, result, "addr3")
	})

	t.Run("empty input returns empty map", func(t *testing.T) {
		t.Parallel()

		result := uniqueUTXOAddrs([]chain.UTXO{})
		assert.Empty(t, result)
	})

	t.Run("single UTXO returns single address", func(t *testing.T) {
		t.Parallel()

		result := uniqueUTXOAddrs([]chain.UTXO{{Address: "only_one"}})
		assert.Len(t, result, 1)
		assert.Contains(t, result, "only_one")
	})
}

// TestFilterSpentBSVUTXOs tests that locally-spent UTXOs are excluded from the API result set.
func TestFilterSpentBSVUTXOs(t *testing.T) {
	t.Parallel()

	t.Run("filters out spent UTXOs", func(t *testing.T) {
		t.Parallel()

		store := utxostore.New(t.TempDir())
		store.AddUTXO(&utxostore.StoredUTXO{
			ChainID: chain.BSV,
			TxID:    "tx1",
			Vout:    0,
			Amount:  1000,
			Spent:   false,
		})
		store.AddUTXO(&utxostore.StoredUTXO{
			ChainID: chain.BSV,
			TxID:    "tx2",
			Vout:    0,
			Amount:  2000,
			Spent:   false,
		})
		store.MarkSpent(chain.BSV, "tx1", 0, "spending-tx")

		utxos := []chain.UTXO{
			{TxID: "tx1", Vout: 0, Amount: 1000},
			{TxID: "tx2", Vout: 0, Amount: 2000},
			{TxID: "tx3", Vout: 0, Amount: 3000}, // not in store at all
		}

		filtered := filterSpentBSVUTXOs(utxos, store)

		assert.Len(t, filtered, 2)
		assert.Equal(t, "tx2", filtered[0].TxID)
		assert.Equal(t, "tx3", filtered[1].TxID)
	})

	t.Run("keeps all UTXOs when none are spent", func(t *testing.T) {
		t.Parallel()

		store := utxostore.New(t.TempDir())

		utxos := []chain.UTXO{
			{TxID: "tx1", Vout: 0, Amount: 1000},
			{TxID: "tx2", Vout: 1, Amount: 2000},
		}

		filtered := filterSpentBSVUTXOs(utxos, store)

		assert.Len(t, filtered, 2)
	})

	t.Run("returns empty slice when all are spent", func(t *testing.T) {
		t.Parallel()

		store := utxostore.New(t.TempDir())
		store.AddUTXO(&utxostore.StoredUTXO{
			ChainID: chain.BSV,
			TxID:    "tx1",
			Vout:    0,
			Amount:  1000,
			Spent:   true,
		})
		store.AddUTXO(&utxostore.StoredUTXO{
			ChainID: chain.BSV,
			TxID:    "tx2",
			Vout:    0,
			Amount:  2000,
			Spent:   true,
		})

		utxos := []chain.UTXO{
			{TxID: "tx1", Vout: 0, Amount: 1000},
			{TxID: "tx2", Vout: 0, Amount: 2000},
		}

		filtered := filterSpentBSVUTXOs(utxos, store)

		assert.Empty(t, filtered)
	})

	t.Run("empty input returns empty slice", func(t *testing.T) {
		t.Parallel()

		store := utxostore.New(t.TempDir())

		filtered := filterSpentBSVUTXOs([]chain.UTXO{}, store)

		assert.Empty(t, filtered)
	})

	t.Run("distinguishes by vout", func(t *testing.T) {
		t.Parallel()

		store := utxostore.New(t.TempDir())
		store.AddUTXO(&utxostore.StoredUTXO{
			ChainID: chain.BSV,
			TxID:    "tx1",
			Vout:    0,
			Amount:  1000,
			Spent:   true,
		})
		store.AddUTXO(&utxostore.StoredUTXO{
			ChainID: chain.BSV,
			TxID:    "tx1",
			Vout:    1,
			Amount:  2000,
			Spent:   false,
		})

		utxos := []chain.UTXO{
			{TxID: "tx1", Vout: 0, Amount: 1000},
			{TxID: "tx1", Vout: 1, Amount: 2000},
		}

		filtered := filterSpentBSVUTXOs(utxos, store)

		require.Len(t, filtered, 1)
		assert.Equal(t, uint32(1), filtered[0].Vout)
	})
}

// TestMarkSpentBSVUTXOs tests that UTXOs are marked spent in the store after broadcast.
func TestMarkSpentBSVUTXOs(t *testing.T) {
	t.Parallel()

	t.Run("marks all UTXOs as spent and persists", func(t *testing.T) {
		t.Parallel()

		tmpDir := t.TempDir()
		store := utxostore.New(tmpDir)

		utxos := []chain.UTXO{
			{TxID: "tx1", Vout: 0, Amount: 1000, ScriptPubKey: "76a914...", Address: "addr1"},
			{TxID: "tx2", Vout: 1, Amount: 2000, ScriptPubKey: "76a914...", Address: "addr2"},
		}

		cc := &CommandContext{Log: nil}
		markSpentBSVUTXOs(cc, store, utxos, "broadcast-txid")

		// Verify UTXOs are marked spent in memory
		assert.True(t, store.IsSpent(chain.BSV, "tx1", 0))
		assert.True(t, store.IsSpent(chain.BSV, "tx2", 1))

		// Verify persisted to disk
		store2 := utxostore.New(tmpDir)
		require.NoError(t, store2.Load())
		assert.True(t, store2.IsSpent(chain.BSV, "tx1", 0))
		assert.True(t, store2.IsSpent(chain.BSV, "tx2", 1))

		// Verify balance is zero (all spent)
		assert.Equal(t, uint64(0), store2.GetBalance(chain.BSV))
	})

	t.Run("handles nil store gracefully", func(t *testing.T) {
		t.Parallel()

		utxos := []chain.UTXO{
			{TxID: "tx1", Vout: 0, Amount: 1000},
		}

		cc := &CommandContext{Log: nil}
		assert.NotPanics(t, func() {
			markSpentBSVUTXOs(cc, nil, utxos, "broadcast-txid")
		})
	})

	t.Run("adds unknown UTXOs before marking spent", func(t *testing.T) {
		t.Parallel()

		tmpDir := t.TempDir()
		store := utxostore.New(tmpDir)
		// Store is empty — UTXOs from API are not in the local store

		utxos := []chain.UTXO{
			{TxID: "api-tx1", Vout: 0, Amount: 5000, Address: "addr1"},
		}

		cc := &CommandContext{Log: nil}
		markSpentBSVUTXOs(cc, store, utxos, "broadcast-txid")

		// Should be added and marked spent
		assert.True(t, store.IsSpent(chain.BSV, "api-tx1", 0))
		assert.Equal(t, uint64(0), store.GetBalance(chain.BSV))
	})

	t.Run("handles empty UTXOs slice", func(t *testing.T) {
		t.Parallel()

		tmpDir := t.TempDir()
		store := utxostore.New(tmpDir)

		cc := &CommandContext{Log: nil}
		assert.NotPanics(t, func() {
			markSpentBSVUTXOs(cc, store, []chain.UTXO{}, "broadcast-txid")
		})
	})

	t.Run("logs error on save failure but does not panic", func(t *testing.T) {
		t.Parallel()

		// Use a path that will fail on save (directory doesn't allow write)
		store := utxostore.New("/dev/null/invalid")

		utxos := []chain.UTXO{
			{TxID: "tx1", Vout: 0, Amount: 1000},
		}

		logger := &txTestLogWriter{}
		cc := &CommandContext{Log: logger}

		assert.NotPanics(t, func() {
			markSpentBSVUTXOs(cc, store, utxos, "broadcast-txid")
		})
		assert.NotEmpty(t, logger.errorCalls, "should log an error on save failure")
	})
}
