package cli

import (
	"context"
	"testing"
	"time"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/require"
)

func TestContextWithTimeout_UsesCommandContext(t *testing.T) {
	t.Parallel()

	parent, parentCancel := context.WithCancel(context.Background())
	cmd := &cobra.Command{}
	cmd.SetContext(parent)

	ctx, cancel := contextWithTimeout(cmd, time.Second)
	defer cancel()

	parentCancel()

	select {
	case <-ctx.Done():
		require.ErrorIs(t, ctx.Err(), context.Canceled)
	case <-time.After(100 * time.Millisecond):
		t.Fatal("expected derived context to cancel when parent command context is canceled")
	}
}

func TestContextWithTimeout_FallbackBackground(t *testing.T) {
	t.Parallel()

	cmd := &cobra.Command{}
	ctx, cancel := contextWithTimeout(cmd, 25*time.Millisecond)
	defer cancel()

	select {
	case <-ctx.Done():
		require.ErrorIs(t, ctx.Err(), context.DeadlineExceeded)
	case <-time.After(200 * time.Millisecond):
		t.Fatal("expected derived context deadline to trigger")
	}
}
