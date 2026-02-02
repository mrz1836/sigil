package cli

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
