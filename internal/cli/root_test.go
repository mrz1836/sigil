package cli

import (
	"testing"

	"github.com/stretchr/testify/assert"

	sigilerr "github.com/mrz1836/sigil/pkg/errors"
)

// errTestRandom is used for testing non-sigil error handling.
var errTestRandom = sigilerr.New("TEST_ERROR", "some random error")

func TestFormatVersion(t *testing.T) {
	tests := []struct {
		name string
		info BuildInfo
		want string
	}{
		{
			name: "all fields populated",
			info: BuildInfo{
				Version: "v1.2.3",
				Commit:  "abc1234",
				Date:    "2024-01-15",
			},
			want: "v1.2.3 (commit: abc1234, built: 2024-01-15)",
		},
		{
			name: "all fields empty",
			info: BuildInfo{},
			want: "dev (commit: unknown, built: unknown)",
		},
		{
			name: "only version empty",
			info: BuildInfo{
				Version: "",
				Commit:  "def5678",
				Date:    "2024-02-20",
			},
			want: "dev (commit: def5678, built: 2024-02-20)",
		},
		{
			name: "only commit empty",
			info: BuildInfo{
				Version: "v2.0.0",
				Commit:  "",
				Date:    "2024-03-25",
			},
			want: "v2.0.0 (commit: unknown, built: 2024-03-25)",
		},
		{
			name: "only date empty",
			info: BuildInfo{
				Version: "v3.0.0",
				Commit:  "ghi9012",
				Date:    "",
			},
			want: "v3.0.0 (commit: ghi9012, built: unknown)",
		},
		{
			name: "version and commit empty",
			info: BuildInfo{
				Version: "",
				Commit:  "",
				Date:    "2024-04-30",
			},
			want: "dev (commit: unknown, built: 2024-04-30)",
		},
		{
			name: "version and date empty",
			info: BuildInfo{
				Version: "",
				Commit:  "jkl3456",
				Date:    "",
			},
			want: "dev (commit: jkl3456, built: unknown)",
		},
		{
			name: "commit and date empty",
			info: BuildInfo{
				Version: "v4.0.0",
				Commit:  "",
				Date:    "",
			},
			want: "v4.0.0 (commit: unknown, built: unknown)",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := formatVersion(tc.info)
			assert.Equal(t, tc.want, got)
		})
	}
}

func TestExitCode(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want int
	}{
		{
			name: "nil error returns success",
			err:  nil,
			want: sigilerr.ExitSuccess,
		},
		{
			name: "general error",
			err:  sigilerr.ErrGeneral,
			want: sigilerr.ExitGeneral,
		},
		{
			name: "invalid input error",
			err:  sigilerr.ErrInvalidInput,
			want: sigilerr.ExitInput,
		},
		{
			name: "authentication error",
			err:  sigilerr.ErrAuthentication,
			want: sigilerr.ExitAuth,
		},
		{
			name: "not found error",
			err:  sigilerr.ErrNotFound,
			want: sigilerr.ExitNotFound,
		},
		{
			name: "permission error",
			err:  sigilerr.ErrPermission,
			want: sigilerr.ExitPermission,
		},
		{
			name: "insufficient funds error",
			err:  sigilerr.ErrInsufficientFunds,
			want: sigilerr.ExitPermission,
		},
		{
			name: "wallet not found error",
			err:  sigilerr.ErrWalletNotFound,
			want: sigilerr.ExitNotFound,
		},
		{
			name: "wallet exists error",
			err:  sigilerr.ErrWalletExists,
			want: sigilerr.ExitInput,
		},
		{
			name: "invalid mnemonic error",
			err:  sigilerr.ErrInvalidMnemonic,
			want: sigilerr.ExitInput,
		},
		{
			name: "decryption failed error",
			err:  sigilerr.ErrDecryptionFailed,
			want: sigilerr.ExitAuth,
		},
		{
			name: "network error",
			err:  sigilerr.ErrNetworkError,
			want: sigilerr.ExitGeneral,
		},
		{
			name: "config not found error",
			err:  sigilerr.ErrConfigNotFound,
			want: sigilerr.ExitNotFound,
		},
		{
			name: "non-sigil error returns general",
			err:  errTestRandom,
			want: sigilerr.ExitGeneral,
		},
		{
			name: "wrapped sigil error preserves exit code",
			err:  sigilerr.Wrap(sigilerr.ErrAuthentication, "failed to authenticate"),
			want: sigilerr.ExitAuth,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := ExitCode(tc.err)
			assert.Equal(t, tc.want, got)
		})
	}
}
