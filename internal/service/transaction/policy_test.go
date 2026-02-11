package transaction

import (
	"math/big"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mrz1836/sigil/internal/agent"
	"github.com/mrz1836/sigil/internal/chain"
)

// TestEnforceAgentPolicy_NotInAgentMode tests behavior when credential is nil.
func TestEnforceAgentPolicy_NotInAgentMode(t *testing.T) {
	t.Parallel()

	// Nil credential means not in agent mode - should pass immediately
	err := enforceAgentPolicy(nil, "/tmp/counter", "token", chain.BSV, "1ABC", big.NewInt(100000))
	require.NoError(t, err, "should pass when not in agent mode")
}

// TestEnforceAgentPolicy_ChainDenied tests when chain is not in allowlist.
func TestEnforceAgentPolicy_ChainDenied(t *testing.T) {
	t.Parallel()

	cred := &agent.Credential{
		ID:         "test-agent",
		WalletName: "test-wallet",
		Chains:     []chain.ID{chain.ETH}, // Only ETH allowed
		Policy: agent.Policy{
			MaxPerTxSat: 100000,
		},
	}

	// Try to send BSV when only ETH is allowed
	err := enforceAgentPolicy(cred, "/tmp/counter", "token", chain.BSV, "1ABC", big.NewInt(50000))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "transaction violates agent spending policy")
}

// TestEnforceAgentPolicy_PerTxLimitExceeded tests per-transaction limit enforcement.
func TestEnforceAgentPolicy_PerTxLimitExceeded(t *testing.T) {
	t.Parallel()

	cred := &agent.Credential{
		ID:         "test-agent",
		WalletName: "test-wallet",
		Chains:     []chain.ID{chain.BSV},
		Policy: agent.Policy{
			MaxPerTxSat: 50000, // 50k sat limit
		},
	}

	// Try to send more than limit
	err := enforceAgentPolicy(cred, "/tmp/counter", "token", chain.BSV, "1ABC", big.NewInt(100000))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "transaction violates agent spending policy")
}

// TestEnforceAgentPolicy_AddressNotAllowed tests address allowlist enforcement.
func TestEnforceAgentPolicy_AddressNotAllowed(t *testing.T) {
	t.Parallel()

	cred := &agent.Credential{
		ID:         "test-agent",
		WalletName: "test-wallet",
		Chains:     []chain.ID{chain.BSV},
		Policy: agent.Policy{
			MaxPerTxSat:  100000,
			AllowedAddrs: []string{"1ALLOWED", "1APPROVED"},
		},
	}

	// Try to send to address not in allowlist
	err := enforceAgentPolicy(cred, "/tmp/counter", "token", chain.BSV, "1NOTALLOWED", big.NewInt(50000))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "transaction violates agent spending policy")
}

// TestEnforceAgentPolicy_DailyLimitExceeded tests daily spending limit.
func TestEnforceAgentPolicy_DailyLimitExceeded(t *testing.T) {
	t.Parallel()

	// Create temp counter file
	tmpDir := t.TempDir()
	counterPath := filepath.Join(tmpDir, "counter.json")
	token := "test-token"

	cred := &agent.Credential{
		ID:         "test-agent",
		WalletName: "test-wallet",
		Chains:     []chain.ID{chain.BSV},
		Policy: agent.Policy{
			MaxPerTxSat: 100000,
			MaxDailySat: 100000, // 100k sat daily limit
		},
	}

	// First transaction - should pass
	err := enforceAgentPolicy(cred, counterPath, token, chain.BSV, "1ABC", big.NewInt(50000))
	require.NoError(t, err)

	// Record the spend
	err = agent.RecordSpend(counterPath, token, chain.BSV, big.NewInt(50000))
	require.NoError(t, err)

	// Second transaction would exceed daily limit
	err = enforceAgentPolicy(cred, counterPath, token, chain.BSV, "1ABC", big.NewInt(60000))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "daily spending limit")
}

// TestEnforceAgentPolicy_SuccessWithinLimits tests successful policy enforcement.
func TestEnforceAgentPolicy_SuccessWithinLimits(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	counterPath := filepath.Join(tmpDir, "counter.json")
	token := "test-token"

	cred := &agent.Credential{
		ID:         "test-agent",
		WalletName: "test-wallet",
		Chains:     []chain.ID{chain.BSV},
		Policy: agent.Policy{
			MaxPerTxSat: 100000,
			MaxDailySat: 500000,
		},
	}

	// Transaction within limits - should pass
	err := enforceAgentPolicy(cred, counterPath, token, chain.BSV, "1ABC", big.NewInt(50000))
	require.NoError(t, err)
}

// TestEnforceAgentPolicy_SuccessWithAllowedAddress tests allowlist pass.
func TestEnforceAgentPolicy_SuccessWithAllowedAddress(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	counterPath := filepath.Join(tmpDir, "counter.json")
	token := "test-token"

	cred := &agent.Credential{
		ID:         "test-agent",
		WalletName: "test-wallet",
		Chains:     []chain.ID{chain.BSV},
		Policy: agent.Policy{
			MaxPerTxSat:  100000,
			AllowedAddrs: []string{"1ALLOWED", "1APPROVED"},
		},
	}

	// Send to allowed address - should pass
	err := enforceAgentPolicy(cred, counterPath, token, chain.BSV, "1ALLOWED", big.NewInt(50000))
	require.NoError(t, err)
}

// TestEnforceAgentPolicy_ETHWithWeiLimits tests ETH policy with wei limits.
func TestEnforceAgentPolicy_ETHWithWeiLimits(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	counterPath := filepath.Join(tmpDir, "counter.json")
	token := "test-token"

	cred := &agent.Credential{
		ID:         "test-agent",
		WalletName: "test-wallet",
		Chains:     []chain.ID{chain.ETH},
		Policy: agent.Policy{
			MaxPerTxWei: "1000000000000000000", // 1 ETH
			MaxDailyWei: "5000000000000000000", // 5 ETH
		},
	}

	// Transaction within limits - should pass
	amount, _ := new(big.Int).SetString("500000000000000000", 10) // 0.5 ETH
	err := enforceAgentPolicy(cred, counterPath, token, chain.ETH, "0xABC", amount)
	require.NoError(t, err)

	// Transaction exceeding per-tx limit - should fail
	amount2, _ := new(big.Int).SetString("2000000000000000000", 10) // 2 ETH
	err = enforceAgentPolicy(cred, counterPath, token, chain.ETH, "0xABC", amount2)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "transaction violates agent spending policy")
}

// TestRecordAgentSpend_NotInAgentMode tests behavior when not in agent mode.
func TestRecordAgentSpend_NotInAgentMode(t *testing.T) {
	t.Parallel()

	logger := newMockLogWriter()

	// Empty counterPath and token means not in agent mode
	recordAgentSpend(logger, "", "", chain.BSV, big.NewInt(50000))

	// Should return immediately without error or logging
	assert.Empty(t, logger.debugMessages)
	assert.Empty(t, logger.errorMessages)
}

// TestRecordAgentSpend_Success tests successful spend recording.
func TestRecordAgentSpend_Success(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	counterPath := filepath.Join(tmpDir, "counter.json")
	token := "test-token"
	logger := newMockLogWriter()

	// Record a spend
	recordAgentSpend(logger, counterPath, token, chain.BSV, big.NewInt(50000))

	// Verify counter file was created and spend recorded
	counter, err := os.ReadFile(counterPath) //nolint:gosec // Test file path
	require.NoError(t, err)
	assert.Contains(t, string(counter), "50000")
	assert.Empty(t, logger.debugMessages, "should not log on success")
}

// TestRecordAgentSpend_Error tests error handling during recording.
func TestRecordAgentSpend_Error(t *testing.T) {
	t.Parallel()

	// Use invalid path to trigger error
	invalidPath := "/nonexistent/dir/counter.json"
	token := "test-token"
	logger := newMockLogWriter()

	// Should handle error gracefully
	recordAgentSpend(logger, invalidPath, token, chain.BSV, big.NewInt(50000))

	// Verify error was logged
	require.Len(t, logger.debugMessages, 1)
	assert.Contains(t, logger.debugMessages[0], "failed to record agent spending")
}

// TestRecordAgentSpend_NilLogger tests that nil logger doesn't cause panic.
func TestRecordAgentSpend_NilLogger(t *testing.T) {
	t.Parallel()

	invalidPath := "/nonexistent/dir/counter.json"
	token := "test-token"

	// Should not panic with nil logger
	recordAgentSpend(nil, invalidPath, token, chain.BSV, big.NewInt(50000))
}

// TestRecordAgentSpend_MultipleSpends tests recording multiple spends.
func TestRecordAgentSpend_MultipleSpends(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	counterPath := filepath.Join(tmpDir, "counter.json")
	token := "test-token"
	logger := newMockLogWriter()

	// Record multiple spends
	recordAgentSpend(logger, counterPath, token, chain.BSV, big.NewInt(10000))
	recordAgentSpend(logger, counterPath, token, chain.BSV, big.NewInt(20000))
	recordAgentSpend(logger, counterPath, token, chain.BSV, big.NewInt(30000))

	// Load counter and verify total
	counter, err := os.ReadFile(counterPath) //nolint:gosec // Test file path
	require.NoError(t, err)
	assert.Contains(t, string(counter), "60000", "total should be 60000")
	assert.Empty(t, logger.debugMessages)
}

// TestRecordAgentSpend_ETHWithWei tests recording ETH spends in wei.
func TestRecordAgentSpend_ETHWithWei(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	counterPath := filepath.Join(tmpDir, "counter.json")
	token := "test-token"
	logger := newMockLogWriter()

	// Record ETH spend
	amount, _ := new(big.Int).SetString("1000000000000000000", 10) // 1 ETH
	recordAgentSpend(logger, counterPath, token, chain.ETH, amount)

	// Verify counter file contains wei amount
	counter, err := os.ReadFile(counterPath) //nolint:gosec // Test file path
	require.NoError(t, err)
	assert.Contains(t, string(counter), "1000000000000000000")
	assert.Empty(t, logger.debugMessages)
}

// TestRecordAgentSpend_EmptyCounterPath tests handling of empty counter path.
func TestRecordAgentSpend_EmptyCounterPath(t *testing.T) {
	t.Parallel()

	logger := newMockLogWriter()

	// Empty counter path with non-empty token
	recordAgentSpend(logger, "", "token", chain.BSV, big.NewInt(50000))

	// Should return immediately without logging
	assert.Empty(t, logger.debugMessages)
	assert.Empty(t, logger.errorMessages)
}

// TestRecordAgentSpend_EmptyToken tests handling of empty token.
func TestRecordAgentSpend_EmptyToken(t *testing.T) {
	t.Parallel()

	logger := newMockLogWriter()

	// Non-empty counter path with empty token
	recordAgentSpend(logger, "/tmp/counter.json", "", chain.BSV, big.NewInt(50000))

	// Should return immediately without logging
	assert.Empty(t, logger.debugMessages)
	assert.Empty(t, logger.errorMessages)
}
