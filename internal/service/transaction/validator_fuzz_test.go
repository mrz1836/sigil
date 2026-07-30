package transaction

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// FuzzParseDecimalAmount ensures the send-path amount validator never panics on
// arbitrary input and that every successful parse yields a non-negative result.
// This guards the CLI transaction entry point where user-supplied amounts enter.
func FuzzParseDecimalAmount(f *testing.F) {
	seeds := []string{
		"", "0", "1", "1.5", "0.00000001", "21000000",
		" 1.5 ", "\t1\n", ".", "0.", ".5", "1.2.3", "1..2",
		"-1", "-0.5", "abc", "1e10", "0x10", "1,000", "+1",
		"999999999999999999999999999999", "\x00", "NaN", "Inf",
	}
	for _, s := range seeds {
		f.Add(s)
	}

	f.Fuzz(func(t *testing.T, amount string) {
		for _, decimals := range []int{0, 8, 18} {
			result, err := parseDecimalAmount(amount, decimals)
			if err != nil {
				require.Nil(t, result)
				continue
			}
			require.NotNil(t, result)
			require.GreaterOrEqual(t, result.Sign(), 0)
		}
	})
}
