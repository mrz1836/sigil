// Package session provides session-based authentication for wallet operations.
// After entering a password once, credentials are cached for a configurable time.
// The session key is stored in the OS keychain, with the encrypted seed stored
// in a session file.
package session

import (
	"errors"
	"time"
)

// Default session configuration values.
const (
	// DefaultTTL is the default session duration (15 minutes).
	DefaultTTL = 15 * time.Minute

	// MaxTTL is the maximum allowed session duration (60 minutes).
	MaxTTL = 60 * time.Minute

	// MinTTL is the minimum allowed session duration (1 minute).
	MinTTL = 1 * time.Minute

	// ServiceName is the keyring service name for sigil sessions.
	ServiceName = "sigil-session"
)

// Session errors.
var (
	// ErrSessionNotFound indicates no session exists for the wallet.
	ErrSessionNotFound = errors.New("session not found")

	// ErrSessionExpired indicates the session has expired.
	ErrSessionExpired = errors.New("session expired")

	// ErrKeyringUnavailable indicates the OS keyring is not available.
	ErrKeyringUnavailable = errors.New("keyring unavailable")

	// ErrSessionCorrupted indicates the session file is corrupted.
	ErrSessionCorrupted = errors.New("session corrupted")
)

// Session represents an active authentication session for a wallet.
type Session struct {
	WalletName string    `json:"wallet_name"`
	CreatedAt  time.Time `json:"created_at"`
	ExpiresAt  time.Time `json:"expires_at"`
}

// IsValid returns true if the session has not expired.
func (s *Session) IsValid() bool {
	return time.Now().Before(s.ExpiresAt)
}

// TTL returns the remaining time until the session expires.
// Returns 0 if the session has already expired.
func (s *Session) TTL() time.Duration {
	remaining := time.Until(s.ExpiresAt)
	if remaining < 0 {
		return 0
	}
	return remaining
}

// Manager defines the interface for session management.
type Manager interface {
	// Available returns true if session caching is available (keyring accessible).
	Available() bool

	// StartSession creates a new session for the wallet with the given seed.
	// The seed is encrypted with a random key stored in the OS keyring.
	StartSession(wallet string, seed []byte, ttl time.Duration) error

	// GetSession retrieves the decrypted seed for an active session.
	// Returns ErrSessionNotFound if no session exists.
	// Returns ErrSessionExpired if the session has expired.
	GetSession(wallet string) ([]byte, *Session, error)

	// HasValidSession returns true if a valid (non-expired) session exists.
	HasValidSession(wallet string) bool

	// EndSession removes the session for a wallet.
	EndSession(wallet string) error

	// EndAllSessions removes all active sessions and returns the count.
	EndAllSessions() int

	// ListSessions returns all active sessions.
	ListSessions() ([]*Session, error)
}

// Keyring defines the interface for secure key storage.
// This abstraction allows for testing with mock implementations.
type Keyring interface {
	// Set stores a secret in the keyring.
	Set(service, user, password string) error

	// Get retrieves a secret from the keyring.
	Get(service, user string) (string, error)

	// Delete removes a secret from the keyring.
	Delete(service, user string) error
}
