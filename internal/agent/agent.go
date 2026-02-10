// Package agent provides agent-token-based authentication for programmatic wallet access.
// Agent tokens allow AI agents and bots to use wallets non-interactively with
// policy-limited access — spending caps, chain restrictions, address allowlists,
// and expiration.
package agent

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"runtime"
	"time"

	"github.com/mrz1836/sigil/internal/chain"
)

// Sentinel errors for token validation.
var (
	ErrTokenTooShort    = errors.New("token too short")
	ErrTokenBadPrefix   = errors.New("invalid token prefix")
	ErrTokenBadLength   = errors.New("invalid token length")
	ErrChainDenied      = errors.New("agent not authorized for chain")
	ErrAddrDenied       = errors.New("destination address not in agent allowlist")
	ErrPerTxLimit       = errors.New("amount exceeds per-transaction limit")
	ErrDailyLimitExceed = errors.New("amount would exceed daily limit")
	ErrDailyOverflow    = errors.New("daily limit overflow")
	ErrInvalidWallet    = errors.New("invalid wallet name")
	ErrAgentNotFound    = errors.New("agent not found")
	ErrInvalidAgentPath = errors.New("invalid agent path")
	ErrPolicyTampered   = errors.New("policy integrity check failed: possible tampering")
	ErrAgentExpired     = errors.New("agent has expired")
	ErrDecryptFailed    = errors.New("decrypting seed: wrong token or corrupted agent file")
	ErrTokenNoMatch     = errors.New("token does not match any agent")
)

// Token prefix for agent tokens.
const tokenPrefix = "sigil_agt_" //nolint:gosec // G101: Not a credential, this is a format prefix

// tokenLength is the length of the random token in bytes.
const tokenLength = 32

// agentIDLength is the number of hex chars from the token hash used as the agent ID.
// 12 hex chars = 48 bits, reducing birthday collision probability significantly
// compared to the original 6 chars (24 bits).
const agentIDLength = 12

// Credential represents a stored agent credential with encrypted seed and policy.
type Credential struct {
	// ID is the short agent identifier (e.g., "agt_7f3a2b1c9d0e").
	ID string `json:"id"`

	// Label is a human-readable label for this agent.
	Label string `json:"label"`

	// WalletName is the wallet this agent has access to.
	WalletName string `json:"wallet_name"`

	// Chains lists the chains this agent is authorized to use.
	Chains []chain.ID `json:"chains"`

	// EncryptedSeed is the wallet seed encrypted with the agent token.
	EncryptedSeed []byte `json:"encrypted_seed"`

	// Policy defines the spending limits for this agent.
	Policy Policy `json:"policy"`

	// PolicyHMAC is the HMAC-SHA256 of the serialized policy, keyed with the token.
	// Used to detect policy tampering.
	PolicyHMAC string `json:"policy_hmac"`

	// CreatedAt is when the agent was created.
	CreatedAt time.Time `json:"created_at"`

	// ExpiresAt is when the agent token expires.
	ExpiresAt time.Time `json:"expires_at"`

	// Xpubs maps chain IDs to their xpub strings for read-only access.
	Xpubs map[chain.ID]string `json:"xpubs,omitempty"`
}

// IsExpired returns true if the agent credential has expired.
func (c *Credential) IsExpired() bool {
	return time.Now().After(c.ExpiresAt)
}

// TTL returns the remaining time until the credential expires.
// Returns 0 if already expired.
func (c *Credential) TTL() time.Duration {
	remaining := time.Until(c.ExpiresAt)
	if remaining < 0 {
		return 0
	}
	return remaining
}

// HasChain returns true if the agent is authorized for the given chain.
func (c *Credential) HasChain(id chain.ID) bool {
	for _, ch := range c.Chains {
		if ch == id {
			return true
		}
	}
	return false
}

// Policy defines spending limits and restrictions for an agent.
type Policy struct {
	// MaxPerTxSat is the maximum BSV amount per transaction in satoshis (0=unlimited).
	MaxPerTxSat uint64 `json:"max_per_tx_sat"`

	// MaxPerTxWei is the maximum ETH amount per transaction in wei (0=unlimited).
	// Stored as string to avoid JSON precision loss with large numbers.
	MaxPerTxWei string `json:"max_per_tx_wei"`

	// MaxDailySat is the maximum daily BSV spending limit in satoshis (0=unlimited).
	MaxDailySat uint64 `json:"max_daily_sat"`

	// MaxDailyWei is the maximum daily ETH spending limit in wei (0=unlimited).
	MaxDailyWei string `json:"max_daily_wei"`

	// AllowedAddrs is a list of allowed destination addresses. Empty means any address.
	AllowedAddrs []string `json:"allowed_addrs,omitempty"`
}

// MaxPerTxWeiBig returns MaxPerTxWei as a *big.Int. Returns nil if unset or zero.
func (p *Policy) MaxPerTxWeiBig() *big.Int {
	if p.MaxPerTxWei == "" || p.MaxPerTxWei == "0" {
		return nil
	}
	v, ok := new(big.Int).SetString(p.MaxPerTxWei, 10)
	if !ok {
		return nil
	}
	return v
}

// MaxDailyWeiBig returns MaxDailyWei as a *big.Int. Returns nil if unset or zero.
func (p *Policy) MaxDailyWeiBig() *big.Int {
	if p.MaxDailyWei == "" || p.MaxDailyWei == "0" {
		return nil
	}
	v, ok := new(big.Int).SetString(p.MaxDailyWei, 10)
	if !ok {
		return nil
	}
	return v
}

// GenerateToken generates a new random agent token.
// Returns the formatted token string (sigil_agt_<base64>).
func GenerateToken() (string, error) {
	tokenBytes := make([]byte, tokenLength)
	if _, err := rand.Read(tokenBytes); err != nil {
		return "", fmt.Errorf("generating random token: %w", err)
	}
	encoded := base64.RawURLEncoding.EncodeToString(tokenBytes)
	return tokenPrefix + encoded, nil
}

// TokenID derives a short, deterministic ID from a token.
// Format: "agt_" + first 6 hex chars of SHA256(token).
func TokenID(token string) string {
	h := sha256.Sum256([]byte(token))
	return "agt_" + hex.EncodeToString(h[:])[:agentIDLength]
}

// ParseToken validates and extracts the raw bytes from a token string.
// Returns an error if the token format is invalid.
func ParseToken(token string) ([]byte, error) {
	if len(token) <= len(tokenPrefix) {
		return nil, ErrTokenTooShort
	}
	if token[:len(tokenPrefix)] != tokenPrefix {
		return nil, fmt.Errorf("%w: expected %q", ErrTokenBadPrefix, tokenPrefix)
	}

	raw, err := base64.RawURLEncoding.DecodeString(token[len(tokenPrefix):])
	if err != nil {
		return nil, fmt.Errorf("invalid token encoding: %w", err)
	}

	if len(raw) != tokenLength {
		return nil, fmt.Errorf("%w: got %d, want %d", ErrTokenBadLength, len(raw), tokenLength)
	}

	return raw, nil
}

// ComputePolicyHMAC computes the HMAC-SHA256 of the policy, keyed with the token.
// This is used to detect if the policy has been tampered with.
func ComputePolicyHMAC(policy *Policy, token string) (string, error) {
	policyJSON, err := json.Marshal(policy)
	if err != nil {
		return "", fmt.Errorf("marshaling policy: %w", err)
	}

	mac := hmac.New(sha256.New, []byte(token))
	mac.Write(policyJSON)
	return hex.EncodeToString(mac.Sum(nil)), nil
}

// VerifyPolicyHMAC verifies the HMAC of a policy against the expected value.
func VerifyPolicyHMAC(policy *Policy, token, expectedHMAC string) (bool, error) {
	computed, err := ComputePolicyHMAC(policy, token)
	if err != nil {
		return false, err
	}
	return hmac.Equal([]byte(computed), []byte(expectedHMAC)), nil
}

// zeroBytes securely zeros a byte slice.
// runtime.KeepAlive prevents the compiler from optimizing away the zeroing
// as a dead store when the slice is not used afterward.
func zeroBytes(b []byte) {
	for i := range b {
		b[i] = 0
	}
	runtime.KeepAlive(b)
}
