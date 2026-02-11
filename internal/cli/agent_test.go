package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mrz1836/sigil/internal/agent"
	"github.com/mrz1836/sigil/internal/chain"
	"github.com/mrz1836/sigil/internal/config"
	"github.com/mrz1836/sigil/internal/output"
	"github.com/mrz1836/sigil/internal/wallet"
)

// resetAgentFlags resets all package-level agent flag variables to their defaults.
func resetAgentFlags() {
	agentWallet = ""
	agentChains = ""
	agentMaxPerTx = "0"
	agentMaxDaily = "0"
	agentMaxPerTxETH = "0"
	agentMaxDailyETH = "0"
	agentAllowedAddr = ""
	agentExpires = ""
	agentLabel = ""
	agentID = ""
	agentRevokeAll = false
}

// setupAgentTest creates a test environment for agent commands.
func setupAgentTest(t *testing.T) (string, *CommandContext, func()) {
	t.Helper()

	// Reset all package-level flag variables to avoid test pollution
	resetAgentFlags()

	tmpDir := t.TempDir()

	// Create wallets and agents directories
	walletsDir := filepath.Join(tmpDir, "wallets")
	agentsDir := filepath.Join(tmpDir, "agents")
	require.NoError(t, os.MkdirAll(walletsDir, 0o750))
	require.NoError(t, os.MkdirAll(agentsDir, 0o750))

	// Create test config
	cfg := &mockConfigProvider{
		home: tmpDir,
	}

	// Create command context
	cmdCtx := &CommandContext{ //nolint:govet // local variable, not shadowing
		Cfg:        cfg,
		Fmt:        &mockFormatProvider{format: output.FormatText},
		Log:        config.NullLogger(),
		AgentStore: agent.NewFileStore(agentsDir),
	}

	cleanup := func() { //nolint:govet // local variable, shadows package-level cleanup intentionally
		// Cleanup is handled by t.TempDir()
	}

	return tmpDir, cmdCtx, cleanup
}

// createTestWalletForAgent creates a test wallet with a known seed.
func createTestWalletForAgent(t *testing.T, tmpDir string) {
	t.Helper()

	storage := wallet.NewFileStorage(filepath.Join(tmpDir, "wallets"))

	// Create wallet with BSV and ETH chains
	w, err := wallet.NewWallet("test-wallet", []wallet.ChainID{wallet.ChainBSV, wallet.ChainETH})
	require.NoError(t, err)

	// Generate a test mnemonic
	mnemonic := "abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon about"
	seed, err := wallet.MnemonicToSeed(mnemonic, "")
	require.NoError(t, err)
	defer wallet.ZeroBytes(seed)

	// Derive addresses
	require.NoError(t, w.DeriveAddresses(seed, 1))

	// Save wallet
	err = storage.Save(w, seed, []byte("testpass123"))
	require.NoError(t, err)
}

// TestParseChainList tests chain list parsing.
func TestParseChainList(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		input   string
		want    []chain.ID
		wantErr bool
	}{
		{
			name:    "single bsv",
			input:   "bsv",
			want:    []chain.ID{chain.BSV},
			wantErr: false,
		},
		{
			name:    "single eth",
			input:   "eth",
			want:    []chain.ID{chain.ETH},
			wantErr: false,
		},
		{
			name:    "multiple chains",
			input:   "bsv,eth",
			want:    []chain.ID{chain.BSV, chain.ETH},
			wantErr: false,
		},
		{
			name:    "uppercase",
			input:   "BSV,ETH",
			want:    []chain.ID{chain.BSV, chain.ETH},
			wantErr: false,
		},
		{
			name:    "with spaces",
			input:   "bsv, eth",
			want:    []chain.ID{chain.BSV, chain.ETH},
			wantErr: false,
		},
		{
			name:    "invalid chain",
			input:   "invalid",
			want:    nil,
			wantErr: true,
		},
		{
			name:    "empty",
			input:   "",
			want:    nil,
			wantErr: true,
		},
		{
			name:    "only spaces",
			input:   "  ,  ",
			want:    nil,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := parseChainList(tt.input)
			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.want, got)
			}
		})
	}
}

// TestFormatChainList tests chain list formatting.
func TestFormatChainList(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input []chain.ID
		want  string
	}{
		{
			name:  "single",
			input: []chain.ID{chain.BSV},
			want:  "bsv",
		},
		{
			name:  "multiple",
			input: []chain.ID{chain.BSV, chain.ETH},
			want:  "bsv, eth",
		},
		{
			name:  "empty",
			input: []chain.ID{},
			want:  "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := formatChainList(tt.input)
			assert.Equal(t, tt.want, got)
		})
	}
}

// TestParseDuration tests duration parsing.
func TestParseDuration(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		input   string
		want    time.Duration
		wantErr bool
	}{
		{
			name:    "1 day",
			input:   "1d",
			want:    24 * time.Hour,
			wantErr: false,
		},
		{
			name:    "7 days",
			input:   "7d",
			want:    7 * 24 * time.Hour,
			wantErr: false,
		},
		{
			name:    "30 days",
			input:   "30d",
			want:    30 * 24 * time.Hour,
			wantErr: false,
		},
		{
			name:    "365 days",
			input:   "365d",
			want:    365 * 24 * time.Hour,
			wantErr: false,
		},
		{
			name:    "1 hour",
			input:   "1h",
			want:    1 * time.Hour,
			wantErr: false,
		},
		{
			name:    "24 hours",
			input:   "24h",
			want:    24 * time.Hour,
			wantErr: false,
		},
		{
			name:    "uppercase D",
			input:   "7D",
			want:    7 * 24 * time.Hour,
			wantErr: false,
		},
		{
			name:    "with spaces",
			input:   " 7d ",
			want:    7 * 24 * time.Hour,
			wantErr: false,
		},
		{
			name:    "invalid format",
			input:   "7x",
			want:    0,
			wantErr: true,
		},
		{
			name:    "negative",
			input:   "-1d",
			want:    0,
			wantErr: true,
		},
		{
			name:    "zero",
			input:   "0d",
			want:    0,
			wantErr: true,
		},
		{
			name:    "empty",
			input:   "",
			want:    0,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := parseDuration(tt.input)
			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.want, got)
			}
		})
	}
}

// TestParseSatAmount tests satoshi amount parsing.
func TestParseSatAmount(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		input   string
		want    uint64
		wantErr bool
	}{
		{
			name:    "with sat suffix",
			input:   "50000sat",
			want:    50000,
			wantErr: false,
		},
		{
			name:    "with SAT suffix",
			input:   "50000SAT",
			want:    50000,
			wantErr: false,
		},
		{
			name:    "integer satoshis",
			input:   "50000",
			want:    50000,
			wantErr: false,
		},
		{
			name:    "decimal BSV",
			input:   "0.0005",
			want:    50000,
			wantErr: false,
		},
		{
			name:    "1 satoshi",
			input:   "1",
			want:    1,
			wantErr: false,
		},
		{
			name:    "decimal 1 BSV",
			input:   "1.0",
			want:    100000000,
			wantErr: false,
		},
		{
			name:    "zero",
			input:   "0",
			want:    0,
			wantErr: false,
		},
		{
			name:    "empty",
			input:   "",
			want:    0,
			wantErr: false,
		},
		{
			name:    "invalid",
			input:   "invalid",
			want:    0,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := parseSatAmount(tt.input)
			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.want, got)
			}
		})
	}
}

// TestParseWeiAmount tests wei amount parsing.
func TestParseWeiAmount(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "zero",
			input: "0",
			want:  "0",
		},
		{
			name:  "empty",
			input: "",
			want:  "0",
		},
		{
			name:  "decimal ETH",
			input: "0.001",
			want:  "1000000000000000", // 0.001 ETH in wei
		},
		{
			name:  "1 ETH",
			input: "1.0",
			want:  "1000000000000000000", // 1 ETH in wei
		},
		{
			name:  "integer wei",
			input: "1000000",
			want:  "1000000",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := parseWeiAmount(tt.input)
			assert.Equal(t, tt.want, got)
		})
	}
}

// TestAgentCreate_Success tests successful agent creation.
func TestAgentCreate_Success(t *testing.T) {
	tmpDir, cmdCtx, cleanup := setupAgentTest(t) //nolint:govet // test helper returns
	defer cleanup()

	// Create test wallet
	createTestWalletForAgent(t, tmpDir)

	// Mock password prompt
	withMockPrompts(t, []byte("testpass123"), true)

	// Create command
	cmd := agentCreateCmd
	cmd.SetContext(context.Background())
	SetCmdContext(cmd, cmdCtx)

	// Set flags
	require.NoError(t, cmd.Flags().Set("wallet", "test-wallet"))
	require.NoError(t, cmd.Flags().Set("chains", "bsv"))
	require.NoError(t, cmd.Flags().Set("max-per-tx", "50000sat"))
	require.NoError(t, cmd.Flags().Set("max-daily", "500000sat"))
	require.NoError(t, cmd.Flags().Set("expires", "30d"))
	require.NoError(t, cmd.Flags().Set("label", "test-agent"))

	// Capture output
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)

	// Execute
	err := cmd.RunE(cmd, []string{})
	require.NoError(t, err)

	// Verify output
	output := buf.String()
	assert.Contains(t, output, "Agent created")
	assert.Contains(t, output, "test-wallet")
	assert.Contains(t, output, "SIGIL_AGENT_TOKEN=")
}

// TestAgentCreate_JSONOutput tests agent creation with JSON output.
func TestAgentCreate_JSONOutput(t *testing.T) {
	tmpDir, cmdCtx, cleanup := setupAgentTest(t) //nolint:govet // test helper returns
	defer cleanup()

	// Use JSON format
	cmdCtx.Fmt = &mockFormatProvider{format: output.FormatJSON}

	// Create test wallet
	createTestWalletForAgent(t, tmpDir)

	// Mock password prompt
	withMockPrompts(t, []byte("testpass123"), true)

	// Create command
	cmd := agentCreateCmd
	cmd.SetContext(context.Background())
	SetCmdContext(cmd, cmdCtx)

	// Set flags
	require.NoError(t, cmd.Flags().Set("wallet", "test-wallet"))
	require.NoError(t, cmd.Flags().Set("chains", "bsv"))
	require.NoError(t, cmd.Flags().Set("expires", "7d"))
	require.NoError(t, cmd.Flags().Set("label", "json-test"))

	// Capture output
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)

	// Execute
	err := cmd.RunE(cmd, []string{})
	require.NoError(t, err)

	// Parse JSON output
	var result map[string]interface{}
	err = json.Unmarshal(buf.Bytes(), &result)
	require.NoError(t, err)

	// Verify structure
	assert.Contains(t, result, "id")
	assert.Contains(t, result, "label")
	assert.Contains(t, result, "token")
	assert.Contains(t, result, "wallet")
	assert.Equal(t, "json-test", result["label"])
	assert.Equal(t, "test-wallet", result["wallet"])
}

// TestAgentCreate_MissingWallet tests error when wallet doesn't exist.
func TestAgentCreate_MissingWallet(t *testing.T) {
	_, cmdCtx, cleanup := setupAgentTest(t) //nolint:govet // test helper returns
	defer cleanup()

	// Mock password prompt
	withMockPrompts(t, []byte("testpass123"), true)

	// Create command
	cmd := agentCreateCmd
	cmd.SetContext(context.Background())
	SetCmdContext(cmd, cmdCtx)

	// Set flags for non-existent wallet
	require.NoError(t, cmd.Flags().Set("wallet", "nonexistent"))
	require.NoError(t, cmd.Flags().Set("chains", "bsv"))
	require.NoError(t, cmd.Flags().Set("expires", "30d"))
	require.NoError(t, cmd.Flags().Set("label", "test"))

	// Capture output
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)

	// Execute - should fail
	err := cmd.RunE(cmd, []string{})
	require.Error(t, err)
}

// TestAgentCreate_InvalidChains tests error with invalid chain.
func TestAgentCreate_InvalidChains(t *testing.T) {
	tmpDir, cmdCtx, cleanup := setupAgentTest(t) //nolint:govet // test helper returns
	defer cleanup()

	createTestWalletForAgent(t, tmpDir)
	withMockPrompts(t, []byte("testpass123"), true)

	cmd := agentCreateCmd
	cmd.SetContext(context.Background())
	SetCmdContext(cmd, cmdCtx)

	require.NoError(t, cmd.Flags().Set("wallet", "test-wallet"))
	require.NoError(t, cmd.Flags().Set("chains", "INVALID"))
	require.NoError(t, cmd.Flags().Set("expires", "30d"))
	require.NoError(t, cmd.Flags().Set("label", "test"))

	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)

	err := cmd.RunE(cmd, []string{})
	require.Error(t, err)
}

// TestAgentCreate_InvalidDuration tests error with bad expiry format.
func TestAgentCreate_InvalidDuration(t *testing.T) {
	tmpDir, cmdCtx, cleanup := setupAgentTest(t) //nolint:govet // test helper returns
	defer cleanup()

	createTestWalletForAgent(t, tmpDir)
	withMockPrompts(t, []byte("testpass123"), true)

	cmd := agentCreateCmd
	cmd.SetContext(context.Background())
	SetCmdContext(cmd, cmdCtx)

	require.NoError(t, cmd.Flags().Set("wallet", "test-wallet"))
	require.NoError(t, cmd.Flags().Set("chains", "bsv"))
	require.NoError(t, cmd.Flags().Set("expires", "invalid"))
	require.NoError(t, cmd.Flags().Set("label", "test"))

	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)

	err := cmd.RunE(cmd, []string{})
	require.Error(t, err)
}

// TestAgentCreate_InvalidAmount tests error with bad spending limit.
func TestAgentCreate_InvalidAmount(t *testing.T) {
	tmpDir, cmdCtx, cleanup := setupAgentTest(t) //nolint:govet // test helper returns
	defer cleanup()

	createTestWalletForAgent(t, tmpDir)
	withMockPrompts(t, []byte("testpass123"), true)

	cmd := agentCreateCmd
	cmd.SetContext(context.Background())
	SetCmdContext(cmd, cmdCtx)

	require.NoError(t, cmd.Flags().Set("wallet", "test-wallet"))
	require.NoError(t, cmd.Flags().Set("chains", "bsv"))
	require.NoError(t, cmd.Flags().Set("max-per-tx", "invalid"))
	require.NoError(t, cmd.Flags().Set("expires", "30d"))
	require.NoError(t, cmd.Flags().Set("label", "test"))

	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)

	err := cmd.RunE(cmd, []string{})
	require.Error(t, err)
}

// TestAgentCreate_MultipleChains tests agent with multiple chains.
func TestAgentCreate_MultipleChains(t *testing.T) {
	tmpDir, cmdCtx, cleanup := setupAgentTest(t) //nolint:govet // test helper returns
	defer cleanup()

	createTestWalletForAgent(t, tmpDir)
	withMockPrompts(t, []byte("testpass123"), true)

	cmd := agentCreateCmd
	cmd.SetContext(context.Background())
	SetCmdContext(cmd, cmdCtx)

	require.NoError(t, cmd.Flags().Set("wallet", "test-wallet"))
	require.NoError(t, cmd.Flags().Set("chains", "bsv,eth"))
	require.NoError(t, cmd.Flags().Set("expires", "30d"))
	require.NoError(t, cmd.Flags().Set("label", "multi-chain"))

	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)

	err := cmd.RunE(cmd, []string{})
	require.NoError(t, err)

	output := buf.String()
	assert.Contains(t, output, "bsv")
	assert.Contains(t, output, "eth")
}

// TestAgentCreate_AllowedAddresses tests agent with address allowlist.
func TestAgentCreate_AllowedAddresses(t *testing.T) {
	tmpDir, cmdCtx, cleanup := setupAgentTest(t) //nolint:govet // test helper returns
	defer cleanup()

	createTestWalletForAgent(t, tmpDir)
	withMockPrompts(t, []byte("testpass123"), true)

	cmd := agentCreateCmd
	cmd.SetContext(context.Background())
	SetCmdContext(cmd, cmdCtx)

	require.NoError(t, cmd.Flags().Set("wallet", "test-wallet"))
	require.NoError(t, cmd.Flags().Set("chains", "bsv"))
	require.NoError(t, cmd.Flags().Set("allowed-addrs", "1ABC,1DEF"))
	require.NoError(t, cmd.Flags().Set("expires", "30d"))
	require.NoError(t, cmd.Flags().Set("label", "restricted"))

	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)

	err := cmd.RunE(cmd, []string{})
	require.NoError(t, err)

	output := buf.String()
	assert.Contains(t, output, "1ABC")
	assert.Contains(t, output, "1DEF")
}

// TestAgentList_MultipleAgents tests listing multiple agents.
func TestAgentList_MultipleAgents(t *testing.T) {
	tmpDir, cmdCtx, cleanup := setupAgentTest(t) //nolint:govet // test helper returns
	defer cleanup()

	createTestWalletForAgent(t, tmpDir)

	// Create multiple agents
	storage := wallet.NewFileStorage(filepath.Join(tmpDir, "wallets"))
	password := []byte("testpass123")
	_, seed, err := storage.Load("test-wallet", password)
	require.NoError(t, err)
	defer wallet.ZeroBytes(seed)

	for i := 1; i <= 3; i++ {
		token, genErr := agent.GenerateToken()
		require.NoError(t, genErr)

		cred := &agent.Credential{
			ID:         agent.TokenID(token),
			Label:      strings.Replace("agent-X", "X", string(rune('0'+i)), 1),
			WalletName: "test-wallet",
			Chains:     []chain.ID{chain.BSV},
			CreatedAt:  time.Now(),
			ExpiresAt:  time.Now().Add(30 * 24 * time.Hour),
		}

		err = cmdCtx.AgentStore.CreateCredential(cred, token, seed)
		require.NoError(t, err)
	}

	// List agents
	cmd := agentListCmd
	cmd.SetContext(context.Background())
	SetCmdContext(cmd, cmdCtx)
	require.NoError(t, cmd.Flags().Set("wallet", "test-wallet"))

	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)

	err = cmd.RunE(cmd, []string{})
	require.NoError(t, err)

	output := buf.String()
	assert.Contains(t, output, "agent-1")
	assert.Contains(t, output, "agent-2")
	assert.Contains(t, output, "agent-3")
}

// TestAgentList_NoAgents tests listing when no agents exist.
func TestAgentList_NoAgents(t *testing.T) {
	tmpDir, cmdCtx, cleanup := setupAgentTest(t) //nolint:govet // test helper returns
	defer cleanup()

	createTestWalletForAgent(t, tmpDir)

	cmd := agentListCmd
	cmd.SetContext(context.Background())
	SetCmdContext(cmd, cmdCtx)
	require.NoError(t, cmd.Flags().Set("wallet", "test-wallet"))

	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)

	err := cmd.RunE(cmd, []string{})
	require.NoError(t, err)

	output := buf.String()
	assert.Contains(t, output, "No agents found")
}

// TestAgentList_JSONOutput tests agent list with JSON output.
func TestAgentList_JSONOutput(t *testing.T) {
	tmpDir, cmdCtx, cleanup := setupAgentTest(t) //nolint:govet // test helper returns
	defer cleanup()

	cmdCtx.Fmt = &mockFormatProvider{format: output.FormatJSON}
	createTestWalletForAgent(t, tmpDir)

	// Create an agent
	storage := wallet.NewFileStorage(filepath.Join(tmpDir, "wallets"))
	password := []byte("testpass123")
	_, seed, err := storage.Load("test-wallet", password)
	require.NoError(t, err)
	defer wallet.ZeroBytes(seed)

	token, err := agent.GenerateToken()
	require.NoError(t, err)

	cred := &agent.Credential{
		ID:         agent.TokenID(token),
		Label:      "json-agent",
		WalletName: "test-wallet",
		Chains:     []chain.ID{chain.BSV},
		CreatedAt:  time.Now(),
		ExpiresAt:  time.Now().Add(30 * 24 * time.Hour),
	}

	err = cmdCtx.AgentStore.CreateCredential(cred, token, seed)
	require.NoError(t, err)

	// List with JSON
	cmd := agentListCmd
	cmd.SetContext(context.Background())
	SetCmdContext(cmd, cmdCtx)
	require.NoError(t, cmd.Flags().Set("wallet", "test-wallet"))

	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)

	err = cmd.RunE(cmd, []string{})
	require.NoError(t, err)

	// Parse JSON
	var result map[string]interface{}
	err = json.Unmarshal(buf.Bytes(), &result)
	require.NoError(t, err)

	assert.Contains(t, result, "agents")
	agents := result["agents"].([]interface{})
	assert.Len(t, agents, 1)
}

// TestAgentInfo_Success tests showing agent info.
func TestAgentInfo_Success(t *testing.T) {
	tmpDir, cmdCtx, cleanup := setupAgentTest(t) //nolint:govet // test helper returns
	defer cleanup()

	createTestWalletForAgent(t, tmpDir)

	// Create agent
	storage := wallet.NewFileStorage(filepath.Join(tmpDir, "wallets"))
	password := []byte("testpass123")
	_, seed, err := storage.Load("test-wallet", password)
	require.NoError(t, err)
	defer wallet.ZeroBytes(seed)

	token, err := agent.GenerateToken()
	require.NoError(t, err)

	cred := &agent.Credential{
		ID:         agent.TokenID(token),
		Label:      "info-agent",
		WalletName: "test-wallet",
		Chains:     []chain.ID{chain.BSV},
		CreatedAt:  time.Now(),
		ExpiresAt:  time.Now().Add(30 * 24 * time.Hour),
	}

	err = cmdCtx.AgentStore.CreateCredential(cred, token, seed)
	require.NoError(t, err)

	// Show info
	cmd := agentInfoCmd
	cmd.SetContext(context.Background())
	SetCmdContext(cmd, cmdCtx)
	require.NoError(t, cmd.Flags().Set("wallet", "test-wallet"))
	require.NoError(t, cmd.Flags().Set("id", cred.ID))

	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)

	err = cmd.RunE(cmd, []string{})
	require.NoError(t, err)

	output := buf.String()
	assert.Contains(t, output, cred.ID)
	assert.Contains(t, output, "info-agent")
	assert.Contains(t, output, "bsv")
}

// TestAgentInfo_NotFound tests error when agent doesn't exist.
func TestAgentInfo_NotFound(t *testing.T) {
	tmpDir, cmdCtx, cleanup := setupAgentTest(t) //nolint:govet // test helper returns
	defer cleanup()

	createTestWalletForAgent(t, tmpDir)

	cmd := agentInfoCmd
	cmd.SetContext(context.Background())
	SetCmdContext(cmd, cmdCtx)
	require.NoError(t, cmd.Flags().Set("wallet", "test-wallet"))
	require.NoError(t, cmd.Flags().Set("id", "agt_nonexistent"))

	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)

	err := cmd.RunE(cmd, []string{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

// TestAgentInfo_JSONOutput tests agent info with JSON output.
func TestAgentInfo_JSONOutput(t *testing.T) {
	tmpDir, cmdCtx, cleanup := setupAgentTest(t) //nolint:govet // test helper returns
	defer cleanup()

	cmdCtx.Fmt = &mockFormatProvider{format: output.FormatJSON}
	createTestWalletForAgent(t, tmpDir)

	// Create agent
	storage := wallet.NewFileStorage(filepath.Join(tmpDir, "wallets"))
	password := []byte("testpass123")
	_, seed, err := storage.Load("test-wallet", password)
	require.NoError(t, err)
	defer wallet.ZeroBytes(seed)

	token, err := agent.GenerateToken()
	require.NoError(t, err)

	cred := &agent.Credential{
		ID:         agent.TokenID(token),
		Label:      "json-info",
		WalletName: "test-wallet",
		Chains:     []chain.ID{chain.BSV},
		CreatedAt:  time.Now(),
		ExpiresAt:  time.Now().Add(30 * 24 * time.Hour),
	}

	err = cmdCtx.AgentStore.CreateCredential(cred, token, seed)
	require.NoError(t, err)

	// Show info
	cmd := agentInfoCmd
	cmd.SetContext(context.Background())
	SetCmdContext(cmd, cmdCtx)
	require.NoError(t, cmd.Flags().Set("wallet", "test-wallet"))
	require.NoError(t, cmd.Flags().Set("id", cred.ID))

	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)

	err = cmd.RunE(cmd, []string{})
	require.NoError(t, err)

	// Parse JSON
	var result map[string]interface{}
	err = json.Unmarshal(buf.Bytes(), &result)
	require.NoError(t, err)

	assert.Equal(t, cred.ID, result["id"])
	assert.Equal(t, "json-info", result["label"])
}

// TestAgentRevoke_SingleAgent tests revoking a specific agent.
func TestAgentRevoke_SingleAgent(t *testing.T) {
	tmpDir, cmdCtx, cleanup := setupAgentTest(t) //nolint:govet // test helper returns
	defer cleanup()

	createTestWalletForAgent(t, tmpDir)

	// Create agent
	storage := wallet.NewFileStorage(filepath.Join(tmpDir, "wallets"))
	password := []byte("testpass123")
	_, seed, err := storage.Load("test-wallet", password)
	require.NoError(t, err)
	defer wallet.ZeroBytes(seed)

	token, err := agent.GenerateToken()
	require.NoError(t, err)

	cred := &agent.Credential{
		ID:         agent.TokenID(token),
		Label:      "revoke-me",
		WalletName: "test-wallet",
		Chains:     []chain.ID{chain.BSV},
		CreatedAt:  time.Now(),
		ExpiresAt:  time.Now().Add(30 * 24 * time.Hour),
	}

	err = cmdCtx.AgentStore.CreateCredential(cred, token, seed)
	require.NoError(t, err)

	// Revoke
	cmd := agentRevokeCmd
	cmd.SetContext(context.Background())
	SetCmdContext(cmd, cmdCtx)
	require.NoError(t, cmd.Flags().Set("wallet", "test-wallet"))
	require.NoError(t, cmd.Flags().Set("id", cred.ID))

	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)

	err = cmd.RunE(cmd, []string{})
	require.NoError(t, err)

	output := buf.String()
	assert.Contains(t, output, "revoked")

	// Verify agent is gone
	agents, err := cmdCtx.AgentStore.List("test-wallet")
	require.NoError(t, err)
	assert.Empty(t, agents)
}

// TestAgentRevoke_AllAgents tests revoking all agents.
func TestAgentRevoke_AllAgents(t *testing.T) {
	tmpDir, cmdCtx, cleanup := setupAgentTest(t) //nolint:govet // test helper returns
	defer cleanup()

	createTestWalletForAgent(t, tmpDir)

	// Create multiple agents
	storage := wallet.NewFileStorage(filepath.Join(tmpDir, "wallets"))
	password := []byte("testpass123")
	_, seed, err := storage.Load("test-wallet", password)
	require.NoError(t, err)
	defer wallet.ZeroBytes(seed)

	for i := 1; i <= 3; i++ {
		token, genErr := agent.GenerateToken()
		require.NoError(t, genErr)

		cred := &agent.Credential{
			ID:         agent.TokenID(token),
			Label:      "agent",
			WalletName: "test-wallet",
			Chains:     []chain.ID{chain.BSV},
			CreatedAt:  time.Now(),
			ExpiresAt:  time.Now().Add(30 * 24 * time.Hour),
		}

		err = cmdCtx.AgentStore.CreateCredential(cred, token, seed)
		require.NoError(t, err)
	}

	// Revoke all
	cmd := agentRevokeCmd
	cmd.SetContext(context.Background())
	SetCmdContext(cmd, cmdCtx)
	require.NoError(t, cmd.Flags().Set("wallet", "test-wallet"))
	require.NoError(t, cmd.Flags().Set("all", "true"))

	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)

	err = cmd.RunE(cmd, []string{})
	require.NoError(t, err)

	output := buf.String()
	assert.Contains(t, output, "Revoked 3")

	// Verify all agents are gone
	agents, err := cmdCtx.AgentStore.List("test-wallet")
	require.NoError(t, err)
	assert.Empty(t, agents)
}

// TestAgentRevoke_NotFound tests error when agent doesn't exist.
func TestAgentRevoke_NotFound(t *testing.T) {
	tmpDir, cmdCtx, cleanup := setupAgentTest(t) //nolint:govet // test helper returns
	defer cleanup()

	createTestWalletForAgent(t, tmpDir)

	cmd := agentRevokeCmd
	cmd.SetContext(context.Background())
	SetCmdContext(cmd, cmdCtx)
	require.NoError(t, cmd.Flags().Set("wallet", "test-wallet"))
	require.NoError(t, cmd.Flags().Set("id", "agt_nonexistent"))

	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)

	err := cmd.RunE(cmd, []string{})
	require.Error(t, err)
}
