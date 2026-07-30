package shamir

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// FuzzCombine ensures Combine never panics on arbitrary share input. In
// particular it proves the gfDiv divide-by-zero guard (gf256.go) stays
// unreachable: distinct-index shares always yield a non-zero denominator, and
// duplicate indices are deduplicated before interpolation.
func FuzzCombine(f *testing.F) {
	valid, err := Split([]byte("fuzz-secret"), 3, 2)
	require.NoError(f, err)

	// Seed with pairs that reach interpolation, duplicates, and garbage.
	f.Add(valid[0], valid[1])                       // valid, reaches interpolation
	f.Add(valid[0], valid[0])                       // duplicate index -> dedup
	f.Add("sigil-v1-2-1-aabb", "sigil-v1-2-2-ccdd") // distinct indices
	f.Add("sigil-v1-2-1-aabb", "sigil-v1-2-1-ccdd") // same index, diff value
	f.Add("sigil-v1-2-1-zz", "sigil-v1-2-2-yy")     // invalid hex
	f.Add("sigil-v1-2-1-aa", "sigil-v1-3-2-bb")     // mismatched threshold
	f.Add("", "")                                   // empty
	f.Add("garbage", "sigil-v1-2-2-ccdd")           // malformed + valid

	f.Fuzz(func(t *testing.T, a, b string) {
		// Must never panic, regardless of input.
		secret, cErr := Combine([]string{a, b})
		if cErr != nil {
			require.Nil(t, secret)
			return
		}
		require.NotNil(t, secret)
	})
}
