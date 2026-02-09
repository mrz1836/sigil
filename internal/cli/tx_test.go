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
