package cli

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mrz1836/sigil/internal/chain"
	sigilerr "github.com/mrz1836/sigil/pkg/errors"
)

// TestPromptTransactionConfirmation_UnsupportedChains covers the dispatcher's
// early-return validation branches (unsupported BCH/LTC and unknown chains). These
// return before any client is created, so cmd/req/addresses are unused and no
// network is touched.
func TestPromptTransactionConfirmation_UnsupportedChains(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		id   chain.ID
	}{
		{"BCH not yet supported", chain.BCH},
		{"LTC not yet supported", chain.LTC},
		{"unknown chain", chain.ID("dogecoin")},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			ok, err := promptTransactionConfirmation(context.Background(), nil, tc.id, nil, nil)
			require.Error(t, err)
			assert.False(t, ok)
			assert.ErrorIs(t, err, sigilerr.ErrInvalidInput)
		})
	}
}
