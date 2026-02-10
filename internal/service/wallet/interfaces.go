// Package wallet provides wallet loading and management services.
package wallet

import (
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
}

// LogWriter provides logging capabilities.
type LogWriter interface {
	Debug(format string, args ...interface{})
	Error(format string, args ...interface{})
}
