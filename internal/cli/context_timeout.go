package cli

import (
	"context"
	"time"

	"github.com/spf13/cobra"
)

// Shared CLI timeouts. commandTimeout bounds an entire command's context;
// fetchTimeout bounds a single network fetch (balance/UTXO request).
const (
	commandTimeout = 60 * time.Second
	fetchTimeout   = 30 * time.Second
)

// contextWithTimeout returns a timeout context rooted in the command context.
func contextWithTimeout(cmd *cobra.Command, d time.Duration) (context.Context, context.CancelFunc) {
	base := cmd.Context()
	if base == nil {
		base = context.Background()
	}
	return context.WithTimeout(base, d)
}
