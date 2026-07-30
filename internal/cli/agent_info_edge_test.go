package cli

import (
	"bytes"
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mrz1836/sigil/internal/agent"
	"github.com/mrz1836/sigil/internal/chain"
	"github.com/mrz1836/sigil/internal/wallet"
)

// TestAgentInfo_ExpiredWithLimits covers the runAgentInfo display branches for an
// expired credential with explicit BSV/ETH spend limits and an allowlist — the
// counterparts to the "unlimited"/"active" branches already covered elsewhere.
func TestAgentInfo_ExpiredWithLimits(t *testing.T) {
	tmpDir, cmdCtx, cleanup := setupAgentTest(t) //nolint:govet // matches existing agent test helper usage
	defer cleanup()

	createTestWalletForAgent(t, tmpDir)

	storage := wallet.NewFileStorage(filepath.Join(tmpDir, "wallets"))
	_, seed, err := storage.Load("test-wallet", []byte("testpass123"))
	require.NoError(t, err)
	defer wallet.ZeroBytes(seed)

	token, err := agent.GenerateToken()
	require.NoError(t, err)

	cred := &agent.Credential{
		ID:         agent.TokenID(token),
		Label:      "expired-agent",
		WalletName: "test-wallet",
		Chains:     []chain.ID{chain.BSV, chain.ETH},
		CreatedAt:  time.Now().Add(-60 * 24 * time.Hour),
		ExpiresAt:  time.Now().Add(-24 * time.Hour), // already expired
		Policy: agent.Policy{
			MaxPerTxSat:  50000,
			MaxDailySat:  500000,
			MaxPerTxWei:  "1000000000000000000",
			MaxDailyWei:  "5000000000000000000",
			AllowedAddrs: []string{"1AllowedAddr"},
		},
	}
	require.NoError(t, cmdCtx.AgentStore.CreateCredential(cred, token, seed))

	cmd := agentInfoCmd
	cmd.SetContext(context.Background())
	SetCmdContext(cmd, cmdCtx)
	require.NoError(t, cmd.Flags().Set("wallet", "test-wallet"))
	require.NoError(t, cmd.Flags().Set("id", cred.ID))

	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)

	require.NoError(t, cmd.RunE(cmd, []string{}))

	out := buf.String()
	assert.Contains(t, out, "EXPIRED")
	assert.Contains(t, out, "Per-tx BSV:   50000 sat")
	assert.Contains(t, out, "Daily BSV:    500000 sat")
	assert.Contains(t, out, "1000000000000000000 wei")
	assert.Contains(t, out, "5000000000000000000 wei")
	assert.Contains(t, out, "Allowed addresses:")
	assert.Contains(t, out, "1AllowedAddr")
}
