package cli

import (
	"context"
	"testing"
	"time"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mrz1836/sigil/internal/agent"
	"github.com/mrz1836/sigil/internal/cache"
	"github.com/mrz1836/sigil/internal/chain"
	"github.com/mrz1836/sigil/internal/config"
	"github.com/mrz1836/sigil/internal/output"
	"github.com/mrz1836/sigil/internal/service/balance"
	"github.com/mrz1836/sigil/internal/service/transaction"
	walletservice "github.com/mrz1836/sigil/internal/service/wallet"
	"github.com/mrz1836/sigil/internal/session"
	"github.com/mrz1836/sigil/internal/wallet"
)

// mockStorage implements wallet.Storage for testing.
type mockStorage struct{}

func (m *mockStorage) Save(_ *wallet.Wallet, _, _ []byte) error { return nil }
func (m *mockStorage) Load(_ string, _ []byte) (*wallet.Wallet, []byte, error) {
	return nil, nil, nil
}

//nolint:nilnil // Mock implementation returns nil for testing
func (m *mockStorage) LoadMetadata(_ string) (*wallet.Wallet, error) { return nil, nil }
func (m *mockStorage) Exists(_ string) (bool, error)                 { return false, nil }
func (m *mockStorage) List() ([]string, error)                       { return nil, nil }
func (m *mockStorage) Delete(_ string) error                         { return nil }

// mockCache implements cache.Cache for testing.
type mockCache struct{}

func (m *mockCache) Get(_ chain.ID, _, _ string) (*cache.BalanceCacheEntry, bool, time.Duration) {
	return nil, false, 0
}
func (m *mockCache) Set(_ cache.BalanceCacheEntry)                                     {}
func (m *mockCache) IsStale(_ chain.ID, _, _ string) bool                              { return true }
func (m *mockCache) IsStaleWithDuration(_ chain.ID, _, _ string, _ time.Duration) bool { return true }
func (m *mockCache) Delete(_ chain.ID, _, _ string)                                    {}
func (m *mockCache) Clear()                                                            {}
func (m *mockCache) Size() int                                                         { return 0 }
func (m *mockCache) GetAllForAddress(_ string) []cache.BalanceCacheEntry               { return nil }
func (m *mockCache) Prune(_ time.Duration) int                                         { return 0 }

// mockFactory implements chain.Factory for testing.
type mockFactory struct{}

//nolint:nilnil // Mock implementation returns nil for testing
func (m *mockFactory) NewChain(_ context.Context, _ chain.ID, _ string) (chain.Chain, error) {
	return nil, nil
}

// mockSessionManager implements session.Manager for testing.
type mockSessionManager struct {
	available bool
}

func (m *mockSessionManager) Available() bool                                        { return m.available }
func (m *mockSessionManager) StartSession(_ string, _ []byte, _ time.Duration) error { return nil }
func (m *mockSessionManager) GetSession(_ string) ([]byte, *session.Session, error) {
	return nil, nil, nil
}
func (m *mockSessionManager) HasValidSession(_ string) bool             { return false }
func (m *mockSessionManager) EndSession(_ string) error                 { return nil }
func (m *mockSessionManager) EndAllSessions() int                       { return 0 }
func (m *mockSessionManager) ListSessions() ([]*session.Session, error) { return nil, nil }

func TestNewCommandContext(t *testing.T) {
	tests := []struct {
		name   string
		config *config.Config
		log    *config.Logger
		fmt    *output.Formatter
	}{
		{
			name:   "with all values",
			config: config.Defaults(),
			log:    config.NullLogger(),
			fmt:    output.NewFormatter(output.FormatText, nil),
		},
		{
			name:   "with nil config",
			config: nil,
			log:    config.NullLogger(),
			fmt:    output.NewFormatter(output.FormatText, nil),
		},
		{
			name:   "with nil logger",
			config: config.Defaults(),
			log:    nil,
			fmt:    output.NewFormatter(output.FormatText, nil),
		},
		{
			name:   "with nil formatter",
			config: config.Defaults(),
			log:    config.NullLogger(),
			fmt:    nil,
		},
		{
			name:   "all nil",
			config: nil,
			log:    nil,
			fmt:    nil,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctx := NewCommandContext(tc.config, tc.log, tc.fmt)
			require.NotNil(t, ctx)

			// Check fields are assigned correctly
			assert.Equal(t, tc.config, ctx.Cfg)
			assert.Equal(t, tc.log, ctx.Log)
			assert.Equal(t, tc.fmt, ctx.Fmt)

			// Factory should always be set to default
			assert.NotNil(t, ctx.Factory)
		})
	}
}

func TestSetCmdContext_GetCmdContext_Roundtrip(t *testing.T) {
	testCfg := config.Defaults()
	testLogger := config.NullLogger()
	testFormatter := output.NewFormatter(output.FormatText, nil)

	cc := NewCommandContext(testCfg, testLogger, testFormatter)

	cmd := &cobra.Command{}
	// Initialize the command's context (required before SetCmdContext)
	cmd.SetContext(context.Background())

	// Set the context
	SetCmdContext(cmd, cc)

	// Get it back
	retrieved := GetCmdContext(cmd)
	require.NotNil(t, retrieved)

	// Verify it's the same context
	assert.Equal(t, cc, retrieved)
	assert.Equal(t, testCfg, retrieved.Cfg)
	assert.Equal(t, testLogger, retrieved.Log)
	assert.Equal(t, testFormatter, retrieved.Fmt)
}

func TestGetCmdContext_NilContext(t *testing.T) {
	cmd := &cobra.Command{}

	// Command with no context set
	ctx := GetCmdContext(cmd)
	assert.Nil(t, ctx)
}

func TestGetCmdContext_WrongContextType(t *testing.T) {
	cmd := &cobra.Command{}

	// Set a context with wrong type value
	cmd.SetContext(cmd.Context())

	ctx := GetCmdContext(cmd)
	assert.Nil(t, ctx)
}

func TestCommandContext_WithStorage(t *testing.T) {
	ctx := NewCommandContext(nil, nil, nil)

	// Initially nil
	assert.Nil(t, ctx.Storage)

	// Set storage
	storage := &mockStorage{}
	result := ctx.WithStorage(storage)

	// Returns same context for chaining
	assert.Equal(t, ctx, result)
	assert.Equal(t, storage, ctx.Storage)
}

func TestCommandContext_WithCache(t *testing.T) {
	ctx := NewCommandContext(nil, nil, nil)

	// Initially nil
	assert.Nil(t, ctx.BalanceCache)

	// Set cache
	balanceCache := &mockCache{}
	result := ctx.WithCache(balanceCache)

	// Returns same context for chaining
	assert.Equal(t, ctx, result)
	assert.Equal(t, balanceCache, ctx.BalanceCache)
}

func TestCommandContext_WithChainFactory(t *testing.T) {
	ctx := NewCommandContext(nil, nil, nil)

	// Default factory is set
	assert.NotNil(t, ctx.Factory)

	// Replace with mock
	factory := &mockFactory{}
	result := ctx.WithChainFactory(factory)

	// Returns same context for chaining
	assert.Equal(t, ctx, result)
	assert.Equal(t, factory, ctx.Factory)
}

func TestCommandContext_WithSessionManager(t *testing.T) {
	ctx := NewCommandContext(nil, nil, nil)

	// Initially nil
	assert.Nil(t, ctx.SessionMgr)

	// Set session manager
	mgr := &mockSessionManager{available: true}
	result := ctx.WithSessionManager(mgr)

	// Returns same context for chaining
	assert.Equal(t, ctx, result)
	assert.Equal(t, mgr, ctx.SessionMgr)
}

func TestCommandContext_WithAgentStore(t *testing.T) {
	ctx := NewCommandContext(nil, nil, nil)
	assert.Nil(t, ctx.AgentStore)
	store := &agent.FileStore{}
	result := ctx.WithAgentStore(store)
	assert.Equal(t, ctx, result)
	assert.Equal(t, store, ctx.AgentStore)
}

func TestCommandContext_WithBalanceService(t *testing.T) {
	ctx := NewCommandContext(nil, nil, nil)
	assert.Nil(t, ctx.BalanceService)
	svc := &balance.Service{}
	result := ctx.WithBalanceService(svc)
	assert.Equal(t, ctx, result)
	assert.Equal(t, svc, ctx.BalanceService)
}

func TestCommandContext_WithWalletService(t *testing.T) {
	ctx := NewCommandContext(nil, nil, nil)
	assert.Nil(t, ctx.WalletService)
	svc := &walletservice.Service{}
	result := ctx.WithWalletService(svc)
	assert.Equal(t, ctx, result)
	assert.Equal(t, svc, ctx.WalletService)
}

func TestCommandContext_WithTransactionService(t *testing.T) {
	ctx := NewCommandContext(nil, nil, nil)
	assert.Nil(t, ctx.TransactionService)
	svc := &transaction.Service{}
	result := ctx.WithTransactionService(svc)
	assert.Equal(t, ctx, result)
	assert.Equal(t, svc, ctx.TransactionService)
}

// mockFormatProvider implements FormatProvider for testing.
type mockFormatProvider struct{ format output.Format }

func (m *mockFormatProvider) Format() output.Format { return m.format }

// Compile-time check that mock types implement interfaces.
var (
	_ wallet.Storage  = (*mockStorage)(nil)
	_ cache.Cache     = (*mockCache)(nil)
	_ chain.Factory   = (*mockFactory)(nil)
	_ session.Manager = (*mockSessionManager)(nil)
	_ FormatProvider  = (*mockFormatProvider)(nil)
)
