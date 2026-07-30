package wallet

import (
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mrz1836/sigil/internal/agent"
	"github.com/mrz1836/sigil/internal/chain"
	"github.com/mrz1836/sigil/internal/wallet"
	sigilerr "github.com/mrz1836/sigil/pkg/errors"
)

// The tests below exercise loadWithAgentToken and loadWithXpub directly
// (white-box, same package) to cover error/validation branches that do not
// require any live infrastructure.

func TestLoadWithAgentToken_NilContext(t *testing.T) {
	t.Parallel()

	service := NewService(&Config{Storage: newMockStorageProvider()})

	// A nil context means no agent store is available.
	result, sessInfo, err := service.loadWithAgentToken("test", "agent-token-x", nil)
	require.Error(t, err)
	assert.Nil(t, result)
	assert.Nil(t, sessInfo)
	assert.ErrorIs(t, err, sigilerr.ErrAgentTokenInvalid)
}

func TestLoadWithAgentToken_NilAgentStore(t *testing.T) {
	t.Parallel()

	service := NewService(&Config{Storage: newMockStorageProvider()})

	// Context present but AgentStore is nil -> same "not initialized" guard.
	result, sessInfo, err := service.loadWithAgentToken("test", "agent-token-x", &LoadContext{})
	require.Error(t, err)
	assert.Nil(t, result)
	assert.Nil(t, sessInfo)
	assert.ErrorIs(t, err, sigilerr.ErrAgentTokenInvalid)
}

func TestLoadWithAgentToken_LoadMetadataError(t *testing.T) {
	t.Parallel()

	token := "agent-token-meta"
	seed := getTestSeed(t)

	storage := newMockStorageProvider()
	storage.addWallet(&wallet.Wallet{Name: "test", EnabledChains: []chain.ID{chain.BSV}}, seed)
	storage.loadMetaErr = errors.New("metadata load failure") //nolint:err113 // test error

	agentStore := newMockAgentStore(t.TempDir())
	agentStore.addAgent("test", token, seed, &agent.Credential{
		Label:     "Test Agent",
		ExpiresAt: time.Now().Add(24 * time.Hour),
		Chains:    []chain.ID{chain.BSV},
	})

	service := NewService(&Config{Storage: storage})

	result, sessInfo, err := service.loadWithAgentToken("test", token, &LoadContext{AgentStore: agentStore.store})
	require.Error(t, err)
	assert.Nil(t, result)
	assert.Nil(t, sessInfo)
	// The token decrypts successfully; loading wallet metadata is what fails.
	assert.Contains(t, err.Error(), "metadata load failure")
}

func TestLoadWithAgentToken_Success_Callbacks(t *testing.T) {
	t.Parallel()

	token := "agent-token-cb" //nolint:gosec // test credential, not real
	seed := getTestSeed(t)

	storage := newMockStorageProvider()
	storage.addWallet(&wallet.Wallet{Name: "test", EnabledChains: []chain.ID{chain.BSV}}, seed)

	agentStore := newMockAgentStore(t.TempDir())
	agentStore.addAgent("test", token, seed, &agent.Credential{
		Label:     "Test Agent",
		ExpiresAt: time.Now().Add(24 * time.Hour),
		Chains:    []chain.ID{chain.BSV},
	})

	service := NewService(&Config{Storage: storage})

	var gotInfo *AgentSessionInfo
	var authMsg string
	ctx := &LoadContext{
		AgentStore:    agentStore.store,
		OnSessionInfo: func(info *AgentSessionInfo) { gotInfo = info },
		OnAuthMessage: func(msg string) { authMsg = msg },
	}

	result, sessInfo, err := service.loadWithAgentToken("test", token, ctx)
	require.NoError(t, err)
	require.NotNil(t, result)
	require.NotNil(t, sessInfo)
	assert.Equal(t, AuthAgentToken, sessInfo.Mode)
	assert.NotNil(t, result.Seed)

	// OnSessionInfo fires with the token, decrypted credential, and counter path.
	require.NotNil(t, gotInfo)
	assert.Equal(t, token, gotInfo.Token)
	require.NotNil(t, gotInfo.Credential)
	assert.Equal(t, "Test Agent", gotInfo.Credential.Label)
	assert.NotEmpty(t, gotInfo.CounterPath)

	// OnAuthMessage fires with a human-readable agent message.
	assert.Contains(t, authMsg, "Test Agent")
}

func TestLoadWithXpub_LoadMetadataError(t *testing.T) {
	t.Parallel()

	storage := newMockStorageProvider()
	storage.addWallet(&wallet.Wallet{Name: "test"}, nil)
	storage.loadMetaErr = errors.New("metadata load failure") //nolint:err113 // test error

	service := NewService(&Config{Storage: storage})

	result, sessInfo, err := service.loadWithXpub("test", "test-xpub", &LoadContext{})
	require.Error(t, err)
	assert.Nil(t, result)
	assert.Nil(t, sessInfo)
	assert.Contains(t, err.Error(), "metadata load failure")
}

func TestLoadWithXpub_NilContext(t *testing.T) {
	t.Parallel()

	storage := newMockStorageProvider()
	storage.addWallet(&wallet.Wallet{Name: "test", EnabledChains: []chain.ID{chain.BSV}}, nil)

	service := NewService(&Config{Storage: storage})

	// A nil context exercises the nil-guards wrapping the callbacks.
	result, sessInfo, err := service.loadWithXpub("test", "test-xpub", nil)
	require.NoError(t, err)
	require.NotNil(t, result)
	require.NotNil(t, sessInfo)
	assert.Nil(t, result.Seed) // read-only mode: no seed is returned
	assert.Equal(t, AuthXpub, sessInfo.Mode)
	assert.Contains(t, sessInfo.Message, "read-only")
}

func TestLoadWithXpub_Success_Callbacks(t *testing.T) {
	t.Parallel()

	xpub := "test-xpub-readonly"

	storage := newMockStorageProvider()
	storage.addWallet(&wallet.Wallet{Name: "test", EnabledChains: []chain.ID{chain.BSV}}, nil)

	service := NewService(&Config{Storage: storage})

	var gotInfo *AgentSessionInfo
	var authMsg string
	ctx := &LoadContext{
		OnSessionInfo: func(info *AgentSessionInfo) { gotInfo = info },
		OnAuthMessage: func(msg string) { authMsg = msg },
	}

	result, sessInfo, err := service.loadWithXpub("test", xpub, ctx)
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Nil(t, result.Seed)
	assert.Equal(t, AuthXpub, sessInfo.Mode)

	// OnSessionInfo receives xpub read-only session info verbatim.
	require.NotNil(t, gotInfo)
	assert.True(t, gotInfo.XpubReadOnly)
	assert.Equal(t, xpub, gotInfo.Xpub)

	// OnAuthMessage announces read-only mode.
	assert.Contains(t, authMsg, "read-only")
}
