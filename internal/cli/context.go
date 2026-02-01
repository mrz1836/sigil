package cli

import (
	"github.com/mrz1836/sigil/internal/cache"
	"github.com/mrz1836/sigil/internal/chain"
	"github.com/mrz1836/sigil/internal/config"
	"github.com/mrz1836/sigil/internal/output"
	"github.com/mrz1836/sigil/internal/wallet"
)

// CommandContext holds dependencies for CLI commands.
type CommandContext struct {
	Config       *config.Config
	Logger       *config.Logger
	Formatter    *output.Formatter
	Storage      wallet.Storage
	Cache        cache.Cache
	ChainFactory chain.Factory
}

// NewCommandContext creates a context with the given dependencies.
func NewCommandContext(
	cfg *config.Config,
	logger *config.Logger,
	formatter *output.Formatter,
) *CommandContext {
	return &CommandContext{
		Config:       cfg,
		Logger:       logger,
		Formatter:    formatter,
		ChainFactory: chain.NewDefaultFactory(),
	}
}

// WithStorage sets the wallet storage.
func (c *CommandContext) WithStorage(s wallet.Storage) *CommandContext {
	c.Storage = s
	return c
}

// WithCache sets the balance cache.
func (c *CommandContext) WithCache(balanceCache cache.Cache) *CommandContext {
	c.Cache = balanceCache
	return c
}

// WithChainFactory sets the chain factory.
func (c *CommandContext) WithChainFactory(f chain.Factory) *CommandContext {
	c.ChainFactory = f
	return c
}
