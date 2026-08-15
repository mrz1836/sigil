// Package wallet provides wallet loading and management services.
package wallet

import (
	"context"
	"time"

	"github.com/mrz1836/sigil/internal/config"
	"github.com/mrz1836/sigil/internal/session"
	"github.com/mrz1836/sigil/internal/wallet"
)

// ConfigProvider provides access to security configuration.
type ConfigProvider interface {
	GetSecurity() config.SecurityConfig
}

// SessionManager manages wallet sessions for cached authentication.
type SessionManager interface {
	Available() bool
	HasValidSession(name string) bool
	GetSession(name string) ([]byte, *session.Session, error)
	StartSession(name string, seed []byte, ttl time.Duration) error
}

// StorageProvider provides wallet storage operations.
type StorageProvider interface {
	Exists(name string) (bool, error)
	Load(name string, password []byte) (*wallet.Wallet, []byte, error)
	LoadMetadata(name string) (*wallet.Wallet, error)
	List() ([]string, error)
	// LoadAuthPolicy returns the advisory policy hint and whether a YubiKey
	// envelope is enrolled, without decrypting. Used to dispatch the loader.
	LoadAuthPolicy(name string) (policy string, hasEnvelope bool, err error)
	// LoadEnvelope returns the marshaled tumbler envelope and policy hint.
	LoadEnvelope(name string) (envelope []byte, policy string, err error)
}

// YubiKeyUnlocker recovers a wallet seed from a marshaled tumbler envelope.
// Implemented by internal/yubikey.Store. Kept as an interface here so the
// service package does not depend on the tumbler module directly.
type YubiKeyUnlocker interface {
	// Unlock recovers the seed from envelope. passwordFn is invoked only when
	// the envelope's policy requires a password (it is not called for
	// yubikey-only). The returned seed is a fresh []byte the caller must zero.
	Unlock(ctx context.Context, envelope []byte, passwordFn func() ([]byte, error)) ([]byte, error)
}

// LogWriter provides logging capabilities.
type LogWriter interface {
	Debug(format string, args ...any)
	Error(format string, args ...any)
}
