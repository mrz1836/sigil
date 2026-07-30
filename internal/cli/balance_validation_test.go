package cli

import (
	"bytes"
	"context"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mrz1836/sigil/internal/output"
)

// TestRunBalanceShow_ValidationErrors covers the mutually-exclusive flag guards
// that return before any wallet load or network call. It mutates package-level
// balance flags, so it is not parallel.
func TestRunBalanceShow_ValidationErrors(t *testing.T) {
	origCached, origAsync, origRefresh := balanceCachedOnly, balanceAsync, balanceRefresh
	defer func() {
		balanceCachedOnly, balanceAsync, balanceRefresh = origCached, origAsync, origRefresh
	}()

	newCmd := func() *cobra.Command {
		cmd := &cobra.Command{}
		cmd.SetContext(context.Background())
		var buf bytes.Buffer
		cmd.SetOut(&buf)
		cmd.SetErr(&buf)
		SetCmdContext(cmd, &CommandContext{
			Cfg: &mockConfigProvider{home: t.TempDir()},
			Fmt: &mockFormatProvider{format: output.FormatText},
		})
		return cmd
	}

	t.Run("cached and async are mutually exclusive", func(t *testing.T) {
		balanceCachedOnly, balanceAsync, balanceRefresh = true, true, false
		err := runBalanceShow(newCmd(), nil)
		require.Error(t, err)
		assert.ErrorIs(t, err, ErrCachedAndAsync)
	})

	t.Run("refresh and async are mutually exclusive", func(t *testing.T) {
		balanceCachedOnly, balanceAsync, balanceRefresh = false, true, true
		err := runBalanceShow(newCmd(), nil)
		require.Error(t, err)
		assert.ErrorIs(t, err, ErrRefreshAndAsync)
	})

	t.Run("refresh and cached are mutually exclusive", func(t *testing.T) {
		balanceCachedOnly, balanceAsync, balanceRefresh = true, false, true
		err := runBalanceShow(newCmd(), nil)
		require.Error(t, err)
		assert.ErrorIs(t, err, ErrRefreshAndCached)
	})
}
