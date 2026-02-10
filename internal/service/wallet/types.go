package wallet

import (
	"time"

	"github.com/mrz1836/sigil/internal/wallet"
)

// LoadRequest contains parameters for loading a wallet.
type LoadRequest struct {
	Name         string
	PasswordFunc func(string) (string, error) // Injected password prompt function
}

// LoadResult contains the loaded wallet and seed material.
// Caller MUST zero the seed after use with wallet.ZeroBytes(result.Seed).
type LoadResult struct {
	Wallet *wallet.Wallet
	Seed   []byte // Caller must zero after use
}

// AuthMode represents the authentication method used to load a wallet.
type AuthMode int

const (
	// AuthSession uses a cached session for authentication.
	AuthSession AuthMode = iota
	// AuthAgentToken uses an agent token for authentication.
	AuthAgentToken
	// AuthXpub uses extended public key (read-only mode, no seed).
	AuthXpub
	// AuthPassword uses password-based authentication.
	AuthPassword
)

// String returns the string representation of the auth mode.
func (a AuthMode) String() string {
	switch a {
	case AuthSession:
		return "session"
	case AuthAgentToken:
		return "agent_token"
	case AuthXpub:
		return "xpub"
	case AuthPassword:
		return "password"
	default:
		return "unknown"
	}
}

// SessionInfo contains information about the authentication session used.
type SessionInfo struct {
	Mode      AuthMode
	ExpiresIn time.Duration // Time until expiration (for session/agent modes)
	Message   string        // User-facing message about auth method
}
