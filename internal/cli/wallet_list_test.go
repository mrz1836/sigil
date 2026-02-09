package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mrz1836/sigil/internal/config"
	"github.com/mrz1836/sigil/internal/output"
	"github.com/mrz1836/sigil/internal/session"
	"github.com/mrz1836/sigil/internal/wallet"
)

var (
	errSessionCorrupt      = errors.New("session corrupt")
	errKeychainUnavailable = errors.New("keychain unavailable")
)

// TestDisplayWalletText tests text display formatting for wallet details.
func TestDisplayWalletText(t *testing.T) {
	t.Parallel()

	wlt := &wallet.Wallet{
		Name:      "test_wallet",
		CreatedAt: time.Date(2026, 1, 15, 10, 30, 0, 0, time.UTC),
		Version:   1,
		Addresses: map[wallet.ChainID][]wallet.Address{
			wallet.ChainBSV: {
				{
					Index:   0,
					Address: "1A1zP1eP5QGefi2DMPTfTL5SLmv7DivfNa",
					Path:    "m/44'/236'/0'/0/0",
				},
				{
					Index:   1,
					Address: "1BvBMSEYstWetqTFn5Au4m4GFg7xJaNVN2",
					Path:    "m/44'/236'/0'/0/1",
				},
			},
			wallet.ChainETH: {
				{
					Index:   0,
					Address: "0x742d35Cc6634C0532925a3b844Bc454e4438f44e",
					Path:    "m/44'/60'/0'/0/0",
				},
			},
		},
	}

	cmd := &cobra.Command{}
	var buf bytes.Buffer
	cmd.SetOut(&buf)

	displayWalletText(wlt, cmd)

	result := buf.String()
	assert.Contains(t, result, "Wallet: test_wallet")
	assert.Contains(t, result, "Created: 2026-01-15 10:30:00")
	assert.Contains(t, result, "Version: 1")
	assert.Contains(t, result, "Addresses:")
	assert.Contains(t, result, "1A1zP1eP5QGefi2DMPTfTL5SLmv7DivfNa")
	assert.Contains(t, result, "m/44'/236'/0'/0/0")
	assert.Contains(t, result, "0x742d35Cc6634C0532925a3b844Bc454e4438f44e")
}

// TestDisplayWalletTextMultipleAddresses tests text display with multiple addresses per chain.
func TestDisplayWalletTextMultipleAddresses(t *testing.T) {
	t.Parallel()

	wlt := &wallet.Wallet{
		Name:      "multi_addr",
		CreatedAt: time.Date(2026, 1, 20, 15, 45, 0, 0, time.UTC),
		Version:   1,
		Addresses: map[wallet.ChainID][]wallet.Address{
			wallet.ChainBSV: {
				{Index: 0, Address: "1BSVaddr1", Path: "m/44'/236'/0'/0/0"},
				{Index: 1, Address: "1BSVaddr2", Path: "m/44'/236'/0'/0/1"},
				{Index: 2, Address: "1BSVaddr3", Path: "m/44'/236'/0'/0/2"},
			},
		},
	}

	cmd := &cobra.Command{}
	var buf bytes.Buffer
	cmd.SetOut(&buf)

	displayWalletText(wlt, cmd)

	result := buf.String()
	assert.Contains(t, result, "Wallet: multi_addr")
	assert.Contains(t, result, "[0] 1BSVaddr1")
	assert.Contains(t, result, "[1] 1BSVaddr2")
	assert.Contains(t, result, "[2] 1BSVaddr3")
	assert.Contains(t, result, "Path: m/44'/236'/0'/0/0")
	assert.Contains(t, result, "Path: m/44'/236'/0'/0/1")
	assert.Contains(t, result, "Path: m/44'/236'/0'/0/2")
}

// TestDisplayWalletTextEmptyAddresses tests text display when wallet has no addresses.
func TestDisplayWalletTextEmptyAddresses(t *testing.T) {
	t.Parallel()

	wlt := &wallet.Wallet{
		Name:      "empty_wallet",
		CreatedAt: time.Date(2026, 2, 1, 8, 0, 0, 0, time.UTC),
		Version:   1,
		Addresses: map[wallet.ChainID][]wallet.Address{},
	}

	cmd := &cobra.Command{}
	var buf bytes.Buffer
	cmd.SetOut(&buf)

	displayWalletText(wlt, cmd)

	result := buf.String()
	assert.Contains(t, result, "Wallet: empty_wallet")
	assert.Contains(t, result, "Addresses:")
}

// TestDisplayWalletJSONMultipleChains tests JSON display with multiple chains.
func TestDisplayWalletJSONMultipleChains(t *testing.T) {
	t.Parallel()

	wlt := &wallet.Wallet{
		Name:      "multi_chain",
		CreatedAt: time.Date(2026, 1, 20, 15, 45, 0, 0, time.UTC),
		Version:   1,
		Addresses: map[wallet.ChainID][]wallet.Address{
			wallet.ChainBSV: {
				{Index: 0, Address: "1BSVaddr1", Path: "m/44'/236'/0'/0/0"},
				{Index: 1, Address: "1BSVaddr2", Path: "m/44'/236'/0'/0/1"},
			},
			wallet.ChainETH: {
				{Index: 0, Address: "0xETHaddr1", Path: "m/44'/60'/0'/0/0"},
			},
		},
	}

	cmd := &cobra.Command{}
	var buf bytes.Buffer
	cmd.SetOut(&buf)

	displayWalletJSON(wlt, cmd)

	var parsed struct {
		Name      string                      `json:"name"`
		Addresses map[string][]map[string]any `json:"addresses"`
	}
	require.NoError(t, json.Unmarshal(buf.Bytes(), &parsed))
	assert.Equal(t, "multi_chain", parsed.Name)
	require.Len(t, parsed.Addresses["bsv"], 2)
	require.Len(t, parsed.Addresses["eth"], 1)
}

// --- Tests for runWalletList ---

// newWalletListTestCmd creates a cobra.Command with CommandContext for runWalletList testing.
func newWalletListTestCmd(home string, format output.Format) (*cobra.Command, *bytes.Buffer) {
	cmd := &cobra.Command{}
	cmd.SetContext(context.Background())
	SetCmdContext(cmd, &CommandContext{
		Cfg: &mockConfigProvider{home: home},
		Fmt: &mockFormatProvider{format: format},
	})
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	return cmd, &buf
}

// createTestWallet creates a real wallet file in the given wallets directory.
func createTestWallet(t *testing.T, walletsDir, name string) {
	t.Helper()
	storage := wallet.NewFileStorage(walletsDir)
	w, err := wallet.NewWallet(name, []wallet.ChainID{wallet.ChainETH})
	require.NoError(t, err)
	mnemonic, err := wallet.GenerateMnemonic(12)
	require.NoError(t, err)
	seed, err := wallet.MnemonicToSeed(mnemonic, "")
	require.NoError(t, err)
	defer wallet.ZeroBytes(seed)
	require.NoError(t, w.DeriveAddresses(seed, 1))
	require.NoError(t, storage.Save(w, seed, []byte("password")))
}

func TestRunWalletList_EmptyText(t *testing.T) {
	tmpDir, cleanup := setupTestEnv(t)
	defer cleanup()

	cmd, buf := newWalletListTestCmd(tmpDir, output.FormatText)
	err := runWalletList(cmd, nil)
	require.NoError(t, err)
	assert.Contains(t, buf.String(), "No wallets found")
}

func TestRunWalletList_EmptyJSON(t *testing.T) {
	tmpDir, cleanup := setupTestEnv(t)
	defer cleanup()

	cmd, buf := newWalletListTestCmd(tmpDir, output.FormatJSON)
	err := runWalletList(cmd, nil)
	require.NoError(t, err)
	assert.Equal(t, "[]\n", buf.String())
}

func TestRunWalletList_WithWalletsText(t *testing.T) {
	tmpDir, cleanup := setupTestEnv(t)
	defer cleanup()

	walletsDir := filepath.Join(tmpDir, "wallets")
	createTestWallet(t, walletsDir, "alpha")
	createTestWallet(t, walletsDir, "bravo")

	cmd, buf := newWalletListTestCmd(tmpDir, output.FormatText)
	err := runWalletList(cmd, nil)
	require.NoError(t, err)

	result := buf.String()
	assert.Contains(t, result, "Wallets:")
	assert.Contains(t, result, "alpha")
	assert.Contains(t, result, "bravo")
}

func TestRunWalletList_WithWalletsJSON(t *testing.T) {
	tmpDir, cleanup := setupTestEnv(t)
	defer cleanup()

	walletsDir := filepath.Join(tmpDir, "wallets")
	createTestWallet(t, walletsDir, "charlie")

	cmd, buf := newWalletListTestCmd(tmpDir, output.FormatJSON)
	err := runWalletList(cmd, nil)
	require.NoError(t, err)

	result := buf.String()
	var parsed []string
	require.NoError(t, json.Unmarshal([]byte(result), &parsed))
	assert.Contains(t, parsed, "charlie")
}

// --- Tests for loadWalletWithSession ---

// walletTestSessionMgr is a configurable mock session manager for wallet loading tests.
type walletTestSessionMgr struct {
	available   bool
	hasValid    bool
	getSeed     []byte
	getSess     *session.Session
	getErr      error
	startErr    error
	startCalled bool
}

func (m *walletTestSessionMgr) Available() bool { return m.available }
func (m *walletTestSessionMgr) StartSession(_ string, _ []byte, _ time.Duration) error {
	m.startCalled = true
	return m.startErr
}

func (m *walletTestSessionMgr) GetSession(_ string) ([]byte, *session.Session, error) {
	if m.getErr != nil {
		return nil, nil, m.getErr
	}
	cp := make([]byte, len(m.getSeed))
	copy(cp, m.getSeed)
	return cp, m.getSess, nil
}
func (m *walletTestSessionMgr) HasValidSession(_ string) bool             { return m.hasValid }
func (m *walletTestSessionMgr) EndSession(_ string) error                 { return nil }
func (m *walletTestSessionMgr) EndAllSessions() int                       { return 0 }
func (m *walletTestSessionMgr) ListSessions() ([]*session.Session, error) { return nil, nil }

func TestLoadWalletWithSession_WalletNotFound(t *testing.T) {
	tmpDir, cleanup := setupTestEnv(t)
	defer cleanup()

	storage := wallet.NewFileStorage(filepath.Join(tmpDir, "wallets"))
	cmd := &cobra.Command{}
	cmd.SetContext(context.Background())
	SetCmdContext(cmd, &CommandContext{
		Cfg: &mockConfigProvider{home: tmpDir},
		Fmt: &mockFormatProvider{format: output.FormatText},
		Log: config.NullLogger(),
	})

	_, _, err := loadWalletWithSession("nonexistent", storage, cmd)
	require.Error(t, err)
	require.ErrorIs(t, err, wallet.ErrWalletNotFound)
}

func TestLoadWalletWithSession_PasswordPrompt(t *testing.T) {
	tmpDir, cleanup := setupTestEnv(t)
	defer cleanup()
	withMockPrompts(t, []byte("password"), true)

	walletsDir := filepath.Join(tmpDir, "wallets")
	createTestWallet(t, walletsDir, "testpw")

	storage := wallet.NewFileStorage(walletsDir)
	cmd := &cobra.Command{}
	cmd.SetContext(context.Background())
	var errBuf bytes.Buffer
	cmd.SetErr(&errBuf)
	SetCmdContext(cmd, &CommandContext{
		Cfg: &mockConfigProvider{home: tmpDir},
		Fmt: &mockFormatProvider{format: output.FormatText},
		Log: config.NullLogger(),
	})

	wlt, seed, err := loadWalletWithSession("testpw", storage, cmd)
	require.NoError(t, err)
	require.NotNil(t, wlt)
	require.NotNil(t, seed)
	defer wallet.ZeroBytes(seed)
	assert.Equal(t, "testpw", wlt.Name)
}

func TestLoadWalletWithSession_SessionValid(t *testing.T) {
	tmpDir, cleanup := setupTestEnv(t)
	defer cleanup()

	walletsDir := filepath.Join(tmpDir, "wallets")
	createTestWallet(t, walletsDir, "sessiontest")

	storage := wallet.NewFileStorage(walletsDir)

	// We need a real seed to test session retrieval
	mnemonic, err := wallet.GenerateMnemonic(12)
	require.NoError(t, err)
	fakeSeed, err := wallet.MnemonicToSeed(mnemonic, "")
	require.NoError(t, err)

	sess := &session.Session{
		WalletName: "sessiontest",
		CreatedAt:  time.Now(),
		ExpiresAt:  time.Now().Add(15 * time.Minute),
	}
	mgr := &walletTestSessionMgr{
		available: true,
		hasValid:  true,
		getSeed:   fakeSeed,
		getSess:   sess,
	}

	cmd := &cobra.Command{}
	cmd.SetContext(context.Background())
	var errBuf bytes.Buffer
	cmd.SetErr(&errBuf)
	SetCmdContext(cmd, &CommandContext{
		Cfg:        &mockConfigProvider{home: tmpDir, security: config.SecurityConfig{SessionEnabled: true}},
		Fmt:        &mockFormatProvider{format: output.FormatText},
		Log:        config.NullLogger(),
		SessionMgr: mgr,
	})

	wlt, seed, err := loadWalletWithSession("sessiontest", storage, cmd)
	require.NoError(t, err)
	require.NotNil(t, wlt)
	require.NotNil(t, seed)
	defer wallet.ZeroBytes(seed)
	assert.Equal(t, "sessiontest", wlt.Name)
	assert.Contains(t, errBuf.String(), "Using cached session")
}

func TestLoadWalletWithSession_SessionGetError(t *testing.T) {
	tmpDir, cleanup := setupTestEnv(t)
	defer cleanup()
	withMockPrompts(t, []byte("password"), true)

	walletsDir := filepath.Join(tmpDir, "wallets")
	createTestWallet(t, walletsDir, "sessionerr")

	storage := wallet.NewFileStorage(walletsDir)

	mgr := &walletTestSessionMgr{
		available: true,
		hasValid:  true,
		getErr:    errSessionCorrupt,
	}

	cmd := &cobra.Command{}
	cmd.SetContext(context.Background())
	var errBuf bytes.Buffer
	cmd.SetErr(&errBuf)
	SetCmdContext(cmd, &CommandContext{
		Cfg:        &mockConfigProvider{home: tmpDir, security: config.SecurityConfig{SessionEnabled: true}},
		Fmt:        &mockFormatProvider{format: output.FormatText},
		Log:        config.NullLogger(),
		SessionMgr: mgr,
	})

	// Falls through to password prompt
	wlt, seed, err := loadWalletWithSession("sessionerr", storage, cmd)
	require.NoError(t, err)
	require.NotNil(t, wlt)
	require.NotNil(t, seed)
	defer wallet.ZeroBytes(seed)
	assert.Equal(t, "sessionerr", wlt.Name)
}

func TestLoadWalletWithSession_StartSessionFailure(t *testing.T) {
	tmpDir, cleanup := setupTestEnv(t)
	defer cleanup()
	withMockPrompts(t, []byte("password"), true)

	walletsDir := filepath.Join(tmpDir, "wallets")
	createTestWallet(t, walletsDir, "startfail")

	storage := wallet.NewFileStorage(walletsDir)

	mgr := &walletTestSessionMgr{
		available: true,
		hasValid:  false,
		startErr:  errKeychainUnavailable,
	}

	cmd := &cobra.Command{}
	cmd.SetContext(context.Background())
	var errBuf bytes.Buffer
	cmd.SetErr(&errBuf)
	SetCmdContext(cmd, &CommandContext{
		Cfg:        &mockConfigProvider{home: tmpDir, security: config.SecurityConfig{SessionEnabled: true, SessionTTLMinutes: 15}},
		Fmt:        &mockFormatProvider{format: output.FormatText},
		Log:        config.NullLogger(),
		SessionMgr: mgr,
	})

	wlt, seed, err := loadWalletWithSession("startfail", storage, cmd)
	require.NoError(t, err)
	require.NotNil(t, wlt)
	require.NotNil(t, seed)
	defer wallet.ZeroBytes(seed)
	assert.True(t, mgr.startCalled)
	// Should not contain "Session started" since start failed
	assert.NotContains(t, errBuf.String(), "Session started")
}

func TestLoadWalletWithSession_StartSessionSuccess(t *testing.T) {
	tmpDir, cleanup := setupTestEnv(t)
	defer cleanup()
	withMockPrompts(t, []byte("password"), true)

	walletsDir := filepath.Join(tmpDir, "wallets")
	createTestWallet(t, walletsDir, "startsuccess")

	storage := wallet.NewFileStorage(walletsDir)

	mgr := &walletTestSessionMgr{
		available: true,
		hasValid:  false,
	}

	cmd := &cobra.Command{}
	cmd.SetContext(context.Background())
	var errBuf bytes.Buffer
	cmd.SetErr(&errBuf)
	SetCmdContext(cmd, &CommandContext{
		Cfg:        &mockConfigProvider{home: tmpDir, security: config.SecurityConfig{SessionEnabled: true, SessionTTLMinutes: 15}},
		Fmt:        &mockFormatProvider{format: output.FormatText},
		Log:        config.NullLogger(),
		SessionMgr: mgr,
	})

	wlt, seed, err := loadWalletWithSession("startsuccess", storage, cmd)
	require.NoError(t, err)
	require.NotNil(t, wlt)
	require.NotNil(t, seed)
	defer wallet.ZeroBytes(seed)
	assert.True(t, mgr.startCalled)
	assert.Contains(t, errBuf.String(), "Session started")
}

// --- Tests for runWalletShow ---

func TestRunWalletShow_Text(t *testing.T) {
	tmpDir, cleanup := setupTestEnv(t)
	defer cleanup()
	withMockPrompts(t, []byte("password"), true)

	walletsDir := filepath.Join(tmpDir, "wallets")
	createTestWallet(t, walletsDir, "showtest")

	cmd := &cobra.Command{}
	cmd.SetContext(context.Background())
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	var errBuf bytes.Buffer
	cmd.SetErr(&errBuf)
	SetCmdContext(cmd, &CommandContext{
		Cfg: &mockConfigProvider{home: tmpDir},
		Fmt: &mockFormatProvider{format: output.FormatText},
		Log: config.NullLogger(),
	})

	err := runWalletShow(cmd, []string{"showtest"})
	require.NoError(t, err)

	result := buf.String()
	assert.Contains(t, result, "Wallet: showtest")
	assert.Contains(t, result, "Addresses:")
}

func TestRunWalletShow_JSON(t *testing.T) {
	tmpDir, cleanup := setupTestEnv(t)
	defer cleanup()
	withMockPrompts(t, []byte("password"), true)

	walletsDir := filepath.Join(tmpDir, "wallets")
	createTestWallet(t, walletsDir, "showjson")

	cmd := &cobra.Command{}
	cmd.SetContext(context.Background())
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	var errBuf bytes.Buffer
	cmd.SetErr(&errBuf)
	SetCmdContext(cmd, &CommandContext{
		Cfg: &mockConfigProvider{home: tmpDir},
		Fmt: &mockFormatProvider{format: output.FormatJSON},
		Log: config.NullLogger(),
	})

	err := runWalletShow(cmd, []string{"showjson"})
	require.NoError(t, err)

	var parsed map[string]any
	require.NoError(t, json.Unmarshal(buf.Bytes(), &parsed))
	assert.Equal(t, "showjson", parsed["name"])
}

func TestRunWalletShow_NotFound(t *testing.T) {
	tmpDir, cleanup := setupTestEnv(t)
	defer cleanup()

	cmd := &cobra.Command{}
	cmd.SetContext(context.Background())
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	SetCmdContext(cmd, &CommandContext{
		Cfg: &mockConfigProvider{home: tmpDir},
		Fmt: &mockFormatProvider{format: output.FormatText},
		Log: config.NullLogger(),
	})

	err := runWalletShow(cmd, []string{"nonexistent"})
	require.Error(t, err)
	require.ErrorIs(t, err, wallet.ErrWalletNotFound)
}
