package cli

import (
	"context"

	"github.com/spf13/cobra"

	"github.com/mrz1836/sigil/internal/cache"
	"github.com/mrz1836/sigil/internal/chain"
	"github.com/mrz1836/sigil/internal/config"
	"github.com/mrz1836/sigil/internal/output"
	"github.com/mrz1836/sigil/internal/wallet"
)

// contextKey is the type for context keys to avoid collisions.
type contextKey string

// cmdCtxKey is the key for storing CommandContext in cobra's context.
const cmdCtxKey contextKey = "sigil-cmd-ctx"

// SetCmdContext stores the CommandContext in the cobra command's context.
func SetCmdContext(cmd *cobra.Command, ctx *CommandContext) {
	cmd.SetContext(context.WithValue(cmd.Context(), cmdCtxKey, ctx))
}

// GetCmdContext retrieves the CommandContext from the cobra command's context.
// Returns nil if no context is set.
func GetCmdContext(cmd *cobra.Command) *CommandContext {
	ctx := cmd.Context()
	if ctx == nil {
		return nil
	}
	if cmdCtx, ok := ctx.Value(cmdCtxKey).(*CommandContext); ok {
		return cmdCtx
	}
	return nil
}

// CommandContext holds dependencies for CLI commands.
// Uses interfaces where possible to enable testing with mocks.
type CommandContext struct {
	// Cfg provides configuration access (interface for testability).
	Cfg ConfigProvider

	// Log provides logging capabilities (interface for testability).
	Log LogWriter

	// Fmt provides output formatting (interface for testability).
	Fmt FormatProvider

	// Storage provides wallet persistence.
	Storage wallet.Storage

	// BalanceCache provides balance caching.
	BalanceCache cache.Cache

	// Factory creates chain clients.
	Factory chain.Factory
}

// NewCommandContext creates a context with the given dependencies.
func NewCommandContext(
	cfg *config.Config,
	logger *config.Logger,
	formatter *output.Formatter,
) *CommandContext {
	return &CommandContext{
		Cfg:     cfg,
		Log:     logger,
		Fmt:     formatter,
		Factory: chain.NewDefaultFactory(),
	}
}

// WithStorage sets the wallet storage.
func (c *CommandContext) WithStorage(s wallet.Storage) *CommandContext {
	c.Storage = s
	return c
}

// WithCache sets the balance cache.
func (c *CommandContext) WithCache(balanceCache cache.Cache) *CommandContext {
	c.BalanceCache = balanceCache
	return c
}

// WithChainFactory sets the chain factory.
func (c *CommandContext) WithChainFactory(f chain.Factory) *CommandContext {
	c.Factory = f
	return c
}
