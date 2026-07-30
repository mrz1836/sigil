package chain

import (
	"testing"

	"github.com/stretchr/testify/require"

	sigilerr "github.com/mrz1836/sigil/pkg/errors"
)

// FuzzParseDecimalAmount ensures the shared decimal parser never panics on
// arbitrary input and that every successful parse yields a non-negative,
// non-nil result. This is the parser every chain funnels amounts into.
func FuzzParseDecimalAmount(f *testing.F) {
	seeds := []string{
		"", "0", "1", "1.5", "0.00000001", "100000000",
		"1.123456789012345678", "  ", ".", "0.", ".5", "1.2.3",
		"-1", "-0.5", "abc", "1e10", "0x10", "1,000", "+1", "00000.0000",
		"999999999999999999999999999999", "1.000000000000000000000",
		"\x00", "NaN", "Inf", "٠", "৪.২",
	}
	for _, s := range seeds {
		f.Add(s)
	}

	f.Fuzz(func(t *testing.T, amount string) {
		// Exercise 0-decimal, satoshi (8) and wei (18) scales.
		for _, decimals := range []int{0, 8, 18} {
			result, err := ParseDecimalAmount(amount, decimals, sigilerr.ErrInvalidAmount)
			if err != nil {
				require.Nil(t, result)
				continue
			}
			require.NotNil(t, result)
			require.GreaterOrEqual(t, result.Sign(), 0)
		}
	})
}
