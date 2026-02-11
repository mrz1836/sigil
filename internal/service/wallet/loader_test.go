package wallet

import (
	"errors"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mrz1836/sigil/internal/agent"
	"github.com/mrz1836/sigil/internal/chain"
	"github.com/mrz1836/sigil/internal/config"
	"github.com/mrz1836/sigil/internal/session"
	"github.com/mrz1836/sigil/internal/wallet"
)

const testMnemonic = "abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon about"

func getTestSeed(t *testing.T) []byte {
	t.Helper()
	seed, err := wallet.MnemonicToSeed(testMnemonic, "")
	require.NoError(t, err)
	return seed
}

func TestLoad_SessionAuth_Success(t *testing.T) {
	t.Parallel()

	// Ensure no agent environment variables are set
	_ = os.Unsetenv(config.EnvAgentToken)
	_ = os.Unsetenv(config.EnvAgentXpub)

	testWallet := &wallet.Wallet{
		Name:          "test",
		EnabledChains: []chain.ID{chain.BSV},
	}
	seed := getTestSeed(t)

	storage := newMockStorageProvider()
	storage.addWallet(testWallet, seed)

	sessionMgr := newMockSessionManager()
	require.NoError(t, sessionMgr.StartSession("test", seed, 30*time.Minute))

	cfg := newMockConfigProvider()
	cfg.security.SessionEnabled = true

	service := NewService(&Config{
		Storage:    storage,
		SessionMgr: sessionMgr,
		Config:     cfg,
	})

	req := &LoadRequest{
		Name: "test",
	}

	result, sessInfo, err := service.Load(req, nil)
	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.NotNil(t, sessInfo)
	assert.Equal(t, AuthSession, sessInfo.Mode)
	assert.NotNil(t, result.Seed)
	assert.Equal(t, "test", result.Wallet.Name)
}

func TestLoad_SessionAuth_Expired(t *testing.T) {
	t.Parallel()

	// Ensure no agent environment variables are set
	_ = os.Unsetenv(config.EnvAgentToken)
	_ = os.Unsetenv(config.EnvAgentXpub)

	testWallet := &wallet.Wallet{
		Name:          "test",
		EnabledChains: []chain.ID{chain.BSV},
	}
	seed := getTestSeed(t)

	storage := newMockStorageProvider()
	storage.addWallet(testWallet, seed)

	sessionMgr := newMockSessionManager()
	// Start session with negative duration (already expired)
	sessionMgr.sessions["test"] = &sessionData{
		seed: seed,
		session: &session.Session{
			CreatedAt: time.Now().Add(-1 * time.Hour),
			ExpiresAt: time.Now().Add(-30 * time.Minute),
		},
	}

	cfg := newMockConfigProvider()
	cfg.security.SessionEnabled = true

	service := NewService(&Config{
		Storage:    storage,
		SessionMgr: sessionMgr,
		Config:     cfg,
	})

	req := &LoadRequest{
		Name: "test",
		PasswordFunc: func(_ string) (string, error) {
			return "password", nil
		},
	}

	// Should fall back to password auth since session is expired
	result, sessInfo, err := service.Load(req, nil)
	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, AuthPassword, sessInfo.Mode)
}

func TestLoad_Password_Success(t *testing.T) {
	t.Parallel()

	// Ensure no agent environment variables are set
	_ = os.Unsetenv(config.EnvAgentToken)
	_ = os.Unsetenv(config.EnvAgentXpub)

	testWallet := &wallet.Wallet{
		Name:          "test",
		EnabledChains: []chain.ID{chain.BSV},
	}
	seed := getTestSeed(t)

	storage := newMockStorageProvider()
	storage.addWallet(testWallet, seed)

	service := NewService(&Config{
		Storage: storage,
	})

	req := &LoadRequest{
		Name: "test",
		PasswordFunc: func(_ string) (string, error) {
			return "correct-password", nil
		},
	}

	result, sessInfo, err := service.Load(req, nil)
	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.NotNil(t, sessInfo)
	assert.Equal(t, AuthPassword, sessInfo.Mode)
	assert.NotNil(t, result.Seed)
	assert.Equal(t, "test", result.Wallet.Name)
}

func TestLoad_Password_WrongPassword(t *testing.T) {
	t.Parallel()

	// Ensure no agent environment variables are set
	_ = os.Unsetenv(config.EnvAgentToken)
	_ = os.Unsetenv(config.EnvAgentXpub)

	testWallet := &wallet.Wallet{
		Name:          "test",
		EnabledChains: []chain.ID{chain.BSV},
	}

	storage := newMockStorageProvider()
	storage.addWallet(testWallet, nil)
	storage.loadErr = errors.New("invalid password") //nolint:err113 // test error

	service := NewService(&Config{
		Storage: storage,
	})

	req := &LoadRequest{
		Name: "test",
		PasswordFunc: func(_ string) (string, error) {
			return "wrong-password", nil
		},
	}

	result, sessInfo, err := service.Load(req, nil)
	require.Error(t, err)
	assert.Nil(t, result)
	assert.Nil(t, sessInfo)
	assert.Contains(t, err.Error(), "invalid password")
}

func TestLoad_Password_EmptyPassphrase(t *testing.T) {
	t.Parallel()

	// Ensure no agent environment variables are set
	_ = os.Unsetenv(config.EnvAgentToken)
	_ = os.Unsetenv(config.EnvAgentXpub)

	testWallet := &wallet.Wallet{
		Name:          "test",
		EnabledChains: []chain.ID{chain.BSV},
	}
	seed := getTestSeed(t)

	storage := newMockStorageProvider()
	storage.addWallet(testWallet, seed)

	service := NewService(&Config{
		Storage: storage,
	})

	req := &LoadRequest{
		Name: "test",
		PasswordFunc: func(_ string) (string, error) {
			return "", nil // Empty password
		},
	}

	result, sessInfo, err := service.Load(req, nil)
	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, AuthPassword, sessInfo.Mode)
}

func TestLoad_NoPasswordFunc(t *testing.T) {
	t.Parallel()

	// Ensure no agent environment variables are set
	_ = os.Unsetenv(config.EnvAgentToken)
	_ = os.Unsetenv(config.EnvAgentXpub)

	testWallet := &wallet.Wallet{
		Name:          "test",
		EnabledChains: []chain.ID{chain.BSV},
	}

	storage := newMockStorageProvider()
	storage.addWallet(testWallet, nil)

	service := NewService(&Config{
		Storage: storage,
	})

	req := &LoadRequest{
		Name: "test",
		// No PasswordFunc provided
	}

	result, sessInfo, err := service.Load(req, nil)
	require.Error(t, err)
	assert.Nil(t, result)
	assert.Nil(t, sessInfo)
	assert.Contains(t, err.Error(), "invalid input")
}

func TestLoad_WalletNotFound(t *testing.T) {
	t.Parallel()

	// Ensure no agent environment variables are set
	_ = os.Unsetenv(config.EnvAgentToken)
	_ = os.Unsetenv(config.EnvAgentXpub)

	storage := newMockStorageProvider()

	service := NewService(&Config{
		Storage: storage,
	})

	req := &LoadRequest{
		Name: "nonexistent",
		PasswordFunc: func(_ string) (string, error) {
			return "password", nil
		},
	}

	result, sessInfo, err := service.Load(req, nil)
	require.Error(t, err)
	assert.Nil(t, result)
	assert.Nil(t, sessInfo)
	assert.ErrorIs(t, err, wallet.ErrWalletNotFound)
}

func TestLoad_SessionCreation(t *testing.T) {
	t.Parallel()

	// Ensure no agent environment variables are set
	_ = os.Unsetenv(config.EnvAgentToken)
	_ = os.Unsetenv(config.EnvAgentXpub)

	testWallet := &wallet.Wallet{
		Name:          "test",
		EnabledChains: []chain.ID{chain.BSV},
	}
	seed := getTestSeed(t)

	storage := newMockStorageProvider()
	storage.addWallet(testWallet, seed)

	sessionMgr := newMockSessionManager()

	cfg := newMockConfigProvider()
	cfg.security.SessionEnabled = true
	cfg.security.SessionTTLMinutes = 30

	service := NewService(&Config{
		Storage:    storage,
		SessionMgr: sessionMgr,
		Config:     cfg,
	})

	req := &LoadRequest{
		Name: "test",
		PasswordFunc: func(_ string) (string, error) {
			return "password", nil
		},
	}

	result, sessInfo, err := service.Load(req, nil)
	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, AuthPassword, sessInfo.Mode)

	// Verify session was created
	assert.True(t, sessionMgr.HasValidSession("test"))
}

func TestLoad_AgentToken_Success(t *testing.T) {
	// Not using t.Parallel() because this test modifies environment variables

	// Clean up environment first to avoid interference from other tests
	require.NoError(t, os.Unsetenv(config.EnvAgentToken))
	require.NoError(t, os.Unsetenv(config.EnvAgentXpub))

	// Set up agent token in environment
	token := "test-token-12345"
	require.NoError(t, os.Setenv(config.EnvAgentToken, token))
	t.Cleanup(func() { _ = os.Unsetenv(config.EnvAgentToken) })

	testWallet := &wallet.Wallet{
		Name:          "test",
		EnabledChains: []chain.ID{chain.BSV},
	}
	seed := getTestSeed(t)

	storage := newMockStorageProvider()
	storage.addWallet(testWallet, seed)

	// Create mock agent store
	agentStore := newMockAgentStore(t.TempDir())
	cred := &agent.Credential{
		Label:     "Test Agent",
		ExpiresAt: time.Now().Add(24 * time.Hour),
		Chains:    []chain.ID{chain.BSV},
	}
	agentStore.addAgent("test", token, seed, cred)

	service := NewService(&Config{
		Storage: storage,
	})

	ctx := &LoadContext{
		AgentStore: agentStore.store,
	}

	req := &LoadRequest{
		Name: "test",
	}

	result, sessInfo, err := service.Load(req, ctx)
	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.NotNil(t, sessInfo)
	assert.Equal(t, AuthAgentToken, sessInfo.Mode)
	assert.NotNil(t, result.Seed)
}

func TestLoad_AgentToken_InvalidToken(t *testing.T) {
	// Not using t.Parallel() because this test modifies environment variables

	// Clean up environment first to avoid interference from other tests
	require.NoError(t, os.Unsetenv(config.EnvAgentToken))
	require.NoError(t, os.Unsetenv(config.EnvAgentXpub))

	// Set up invalid agent token in environment
	token := "invalid-token"
	require.NoError(t, os.Setenv(config.EnvAgentToken, token))
	t.Cleanup(func() { _ = os.Unsetenv(config.EnvAgentToken) })

	testWallet := &wallet.Wallet{
		Name:          "test",
		EnabledChains: []chain.ID{chain.BSV},
	}

	storage := newMockStorageProvider()
	storage.addWallet(testWallet, nil)

	// Create empty agent store
	agentStore := newMockAgentStore(t.TempDir())

	service := NewService(&Config{
		Storage: storage,
	})

	ctx := &LoadContext{
		AgentStore: agentStore.store,
	}

	req := &LoadRequest{
		Name: "test",
	}

	result, sessInfo, err := service.Load(req, ctx)
	require.Error(t, err)
	assert.Nil(t, result)
	assert.Nil(t, sessInfo)
	assert.Contains(t, err.Error(), "agent token is invalid or does not match any agent")
}

func TestLoad_AgentToken_Expired(t *testing.T) {
	// Not using t.Parallel() because this test modifies environment variables

	// Clean up environment first to avoid interference from other tests
	require.NoError(t, os.Unsetenv(config.EnvAgentToken))
	require.NoError(t, os.Unsetenv(config.EnvAgentXpub))

	// Set up agent token in environment
	token := "test-token-expired-12345" //nolint:gosec // test credential, not real
	require.NoError(t, os.Setenv(config.EnvAgentToken, token))
	t.Cleanup(func() { _ = os.Unsetenv(config.EnvAgentToken) })

	testWallet := &wallet.Wallet{
		Name:          "test",
		EnabledChains: []chain.ID{chain.BSV},
	}
	seed := getTestSeed(t)

	storage := newMockStorageProvider()
	storage.addWallet(testWallet, seed)

	// Create mock agent store with expired credential
	agentStore := newMockAgentStore(t.TempDir())
	cred := &agent.Credential{
		Label:     "Test Agent",
		ExpiresAt: time.Now().Add(-1 * time.Hour), // Expired
		Chains:    []chain.ID{chain.BSV},
	}
	agentStore.addAgent("test", token, seed, cred)

	service := NewService(&Config{
		Storage: storage,
	})

	ctx := &LoadContext{
		AgentStore: agentStore.store,
	}

	req := &LoadRequest{
		Name: "test",
	}

	result, sessInfo, err := service.Load(req, ctx)
	require.Error(t, err)
	assert.Nil(t, result)
	assert.Nil(t, sessInfo)
	// FileStore.Load checks expiry and returns error, causing LoadByToken to fail
	// with "does not match" rather than "expired". This is the actual behavior.
	assert.Contains(t, err.Error(), "agent token is invalid or does not match any agent")
}

func TestLoad_AgentXpub_Success(t *testing.T) {
	// Not using t.Parallel() because this test modifies environment variables

	// Clean up environment first
	require.NoError(t, os.Unsetenv(config.EnvAgentToken))
	require.NoError(t, os.Unsetenv(config.EnvAgentXpub))

	// Derive a test xpub
	seed := getTestSeed(t)
	xpub, err := wallet.DeriveAccountXpub(seed, chain.BSV, 0)
	require.NoError(t, err)

	// Set up agent xpub in environment
	require.NoError(t, os.Setenv(config.EnvAgentXpub, xpub))
	t.Cleanup(func() { _ = os.Unsetenv(config.EnvAgentXpub) })

	testWallet := &wallet.Wallet{
		Name:          "test",
		EnabledChains: []chain.ID{chain.BSV},
	}

	storage := newMockStorageProvider()
	storage.addWallet(testWallet, nil)

	service := NewService(&Config{
		Storage: storage,
	})

	ctx := &LoadContext{}

	req := &LoadRequest{
		Name: "test",
	}

	result, sessInfo, err := service.Load(req, ctx)
	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.NotNil(t, sessInfo)
	assert.Equal(t, AuthXpub, sessInfo.Mode)
	assert.Nil(t, result.Seed) // xpub mode has no seed
	assert.Contains(t, sessInfo.Message, "read-only")
}

func TestLoad_OnAuthMessage_Callback(t *testing.T) {
	t.Parallel()

	// Ensure no agent environment variables are set
	_ = os.Unsetenv(config.EnvAgentToken)
	_ = os.Unsetenv(config.EnvAgentXpub)

	testWallet := &wallet.Wallet{
		Name:          "test",
		EnabledChains: []chain.ID{chain.BSV},
	}
	seed := getTestSeed(t)

	storage := newMockStorageProvider()
	storage.addWallet(testWallet, seed)

	sessionMgr := newMockSessionManager()
	require.NoError(t, sessionMgr.StartSession("test", seed, 30*time.Minute))

	cfg := newMockConfigProvider()
	cfg.security.SessionEnabled = true

	service := NewService(&Config{
		Storage:    storage,
		SessionMgr: sessionMgr,
		Config:     cfg,
	})

	var authMessage string
	ctx := &LoadContext{
		OnAuthMessage: func(msg string) {
			authMessage = msg
		},
	}

	req := &LoadRequest{
		Name: "test",
	}

	result, _, err := service.Load(req, ctx)
	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.NotEmpty(t, authMessage)
	assert.Contains(t, authMessage, "session")
}

func TestCheckAgentToken(t *testing.T) {
	// Test with no token
	_ = os.Unsetenv(config.EnvAgentToken)
	token := CheckAgentToken()
	assert.Empty(t, token)

	// Test with token
	require.NoError(t, os.Setenv(config.EnvAgentToken, "test-token"))
	defer func() { _ = os.Unsetenv(config.EnvAgentToken) }()
	token = CheckAgentToken()
	assert.Equal(t, "test-token", token)
}

func TestCheckAgentXpub(t *testing.T) {
	// Test with no xpub
	_ = os.Unsetenv(config.EnvAgentXpub)
	xpub := CheckAgentXpub()
	assert.Empty(t, xpub)

	// Test with xpub
	require.NoError(t, os.Setenv(config.EnvAgentXpub, "xpub123"))
	defer func() { _ = os.Unsetenv(config.EnvAgentXpub) }()
	xpub = CheckAgentXpub()
	assert.Equal(t, "xpub123", xpub)
}

func TestFormatDuration(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		duration time.Duration
		want     string
	}{
		{"30 seconds", 30 * time.Second, "30 seconds"},
		{"1 minute", 1 * time.Minute, "1 minutes"},
		{"30 minutes", 30 * time.Minute, "30 minutes"},
		{"1 hour", 1 * time.Hour, "1 hours"},
		{"12 hours", 12 * time.Hour, "12 hours"},
		{"1 day", 24 * time.Hour, "1 day"},
		{"2 days", 48 * time.Hour, "2 days"},
		{"7 days", 7 * 24 * time.Hour, "7 days"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatDuration(tt.duration)
			assert.Equal(t, tt.want, got)
		})
	}
}

// Mock implementations

type mockSessionManager struct {
	available bool
	sessions  map[string]*sessionData
}

type sessionData struct {
	seed    []byte
	session *session.Session
}

func newMockSessionManager() *mockSessionManager {
	return &mockSessionManager{
		available: true,
		sessions:  make(map[string]*sessionData),
	}
}

func (m *mockSessionManager) Available() bool {
	return m.available
}

func (m *mockSessionManager) HasValidSession(name string) bool {
	sd, exists := m.sessions[name]
	if !exists {
		return false
	}
	return time.Now().Before(sd.session.ExpiresAt)
}

func (m *mockSessionManager) GetSession(name string) ([]byte, *session.Session, error) {
	sd, exists := m.sessions[name]
	if !exists {
		return nil, nil, session.ErrSessionNotFound
	}
	if time.Now().After(sd.session.ExpiresAt) {
		return nil, nil, session.ErrSessionExpired
	}
	return sd.seed, sd.session, nil
}

func (m *mockSessionManager) StartSession(name string, seed []byte, ttl time.Duration) error {
	m.sessions[name] = &sessionData{
		seed: seed,
		session: &session.Session{
			CreatedAt: time.Now(),
			ExpiresAt: time.Now().Add(ttl),
		},
	}
	return nil
}

type mockConfigProvider struct {
	security config.SecurityConfig
}

func newMockConfigProvider() *mockConfigProvider {
	return &mockConfigProvider{
		security: config.SecurityConfig{
			SessionEnabled:    false,
			SessionTTLMinutes: 30,
		},
	}
}

func (m *mockConfigProvider) GetSecurity() config.SecurityConfig {
	return m.security
}

type mockAgentStore struct {
	store   *agent.FileStore
	agents  map[string]mockAgent
	tempDir string
}

type mockAgent struct {
	token      string
	seed       []byte
	credential *agent.Credential
}

func newMockAgentStore(tempDir string) *mockAgentStore {
	store := agent.NewFileStore(tempDir)
	return &mockAgentStore{
		store:   store,
		agents:  make(map[string]mockAgent),
		tempDir: tempDir,
	}
}

func (m *mockAgentStore) addAgent(walletName, token string, seed []byte, cred *agent.Credential) {
	key := walletName + ":" + token
	m.agents[key] = mockAgent{
		token:      token,
		seed:       seed,
		credential: cred,
	}

	// Set wallet name in credential if not already set
	if cred.WalletName == "" {
		cred.WalletName = walletName
	}

	// Set the agent ID to match TokenID(token) so LoadByToken can find it
	if cred.ID == "" {
		cred.ID = agent.TokenID(token)
	}

	// Actually save the agent to the file store for realistic testing
	if err := m.store.CreateCredential(cred, token, seed); err != nil {
		panic(fmt.Sprintf("failed to create credential: %v", err))
	}
}
