package transaction

import (
	"math/big"

	"github.com/mrz1836/sigil/internal/agent"
	"github.com/mrz1836/sigil/internal/chain"
	sigilerr "github.com/mrz1836/sigil/pkg/errors"
)

// enforceAgentPolicy checks per-transaction and daily limits when running in agent mode.
// Returns nil if not in agent mode or if the transaction is within policy limits.
// Migrated from cli/tx.go lines 615-640
func enforceAgentPolicy(cred *agent.Credential, counterPath, token string, chainID chain.ID, to string, amount *big.Int) error {
	if cred == nil {
		return nil // Not in agent mode
	}

	// Per-transaction limit and address allowlist check
	if err := agent.ValidateTransaction(cred, chainID, to, amount); err != nil {
		return sigilerr.WithSuggestion(
			sigilerr.ErrAgentPolicyViolation,
			err.Error(),
		)
	}

	// Daily limit check
	if err := agent.CheckDailyLimit(counterPath, token, cred, chainID, amount); err != nil {
		return sigilerr.WithSuggestion(
			sigilerr.ErrAgentDailyLimit,
			err.Error(),
		)
	}

	return nil
}

// EnforceAgentPolicy is the exported version for external use.
func EnforceAgentPolicy(cred *agent.Credential, counterPath, token string, chainID chain.ID, to string, amount *big.Int) error {
	return enforceAgentPolicy(cred, counterPath, token, chainID, to, amount)
}

// recordAgentSpend records a completed transaction in the agent's daily spending counter.
// No-op if not in agent mode.
// Migrated from cli/tx.go lines 642-655
func recordAgentSpend(logger LogWriter, counterPath, token string, chainID chain.ID, amount *big.Int) {
	if counterPath == "" || token == "" {
		return // Not in agent mode
	}

	if err := agent.RecordSpend(counterPath, token, chainID, amount); err != nil {
		if logger != nil {
			logger.Debug("failed to record agent spending: %v", err)
		}
	}
}

// RecordAgentSpend is the exported version for external use.
func RecordAgentSpend(logger LogWriter, counterPath, token string, chainID chain.ID, amount *big.Int) {
	recordAgentSpend(logger, counterPath, token, chainID, amount)
}
