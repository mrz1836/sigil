package shamir

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// TestCombineEmptyReturnsErrNoShares pins the empty-input sentinel for Combine.
func TestCombineEmptyReturnsErrNoShares(t *testing.T) {
	for _, name := range []string{"nil", "empty"} {
		t.Run(name, func(t *testing.T) {
			var shares []string
			if name == "empty" {
				shares = []string{}
			}
			secret, err := Combine(shares)
			require.ErrorIs(t, err, ErrNoShares)
			require.Nil(t, secret)
		})
	}
}

// TestSplitSentinels pins each validation sentinel returned by Split.
func TestSplitSentinels(t *testing.T) {
	tests := []struct {
		name    string
		secret  []byte
		n, k    int
		wantErr error
	}{
		{name: "threshold below two", secret: []byte("s"), n: 3, k: 1, wantErr: ErrThresholdInvalid},
		{name: "shares fewer than threshold", secret: []byte("s"), n: 2, k: 3, wantErr: ErrSharesInsufficient},
		{name: "shares exceed maximum", secret: []byte("s"), n: 256, k: 2, wantErr: ErrSharesExceedMax},
		{name: "empty secret", secret: []byte{}, n: 3, k: 2, wantErr: ErrSecretEmpty},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			shares, err := Split(tt.secret, tt.n, tt.k)
			require.ErrorIs(t, err, tt.wantErr)
			require.Nil(t, shares)
		})
	}
}

// TestParseShareSentinels pins each malformed-share sentinel from parseShare.
func TestParseShareSentinels(t *testing.T) {
	tests := []struct {
		name    string
		share   string
		wantErr error
	}{
		{name: "wrong field count", share: "sigil-v1-2-1", wantErr: ErrInvalidShareFormat},
		{name: "unknown version", share: "sigil-v2-2-1-aabb", wantErr: ErrUnsupportedVersion},
		{name: "bad prefix", share: "xxx-v1-2-1-aabb", wantErr: ErrUnsupportedVersion},
		{name: "invalid threshold", share: "sigil-v1-x-1-aabb", wantErr: ErrInvalidThreshold},
		{name: "non-numeric index", share: "sigil-v1-2-x-aabb", wantErr: ErrInvalidIndex},
		{name: "index below one", share: "sigil-v1-2-0-aabb", wantErr: ErrInvalidIndex},
		{name: "index above max", share: "sigil-v1-2-256-aabb", wantErr: ErrInvalidIndex},
		{name: "invalid hex", share: "sigil-v1-2-1-zz", wantErr: ErrInvalidHex},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, _, err := parseShare(tt.share)
			require.ErrorIs(t, err, tt.wantErr)
		})
	}
}
