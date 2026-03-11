package cli

import (
	"context"
	"time"

	"github.com/spf13/cobra"
)

// contextWithTimeout returns a timeout context rooted in the command context.
func contextWithTimeout(cmd *cobra.Command, d time.Duration) (context.Context, context.CancelFunc) {
	base := cmd.Context()
	if base == nil {
		base = context.Background()
	}
	return context.WithTimeout(base, d)
}
