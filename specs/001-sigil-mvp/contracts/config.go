// Package contracts defines the interface contracts for Sigil MVP.
// These are design artifacts - not compiled code.
// Actual implementations go in internal/config/

package contracts

import (
	"context"
)

// ConfigService defines the interface for configuration management.
type ConfigService interface {
	// Load reads configuration from file and environment.
	Load(ctx context.Context) (*Config, error)

	// Save writes configuration to file.
	Save(ctx context.Context, config *Config) error

	// Get retrieves a configuration value by path (e.g., "networks.eth.rpc").
	Get(path string) (interface{}, error)

	// Set updates a configuration value by path.
	Set(path string, value interface{}) error

	// Init creates a default configuration file.
	Init(ctx context.Context) error

	// Validate checks configuration for errors.
	Validate(config *Config) error
}

// Config represents the complete application configuration.
// See data-model.md for full structure definition.
type Config struct {
	Version    int              `yaml:"version"`
	Home       string           `yaml:"home"`
	Encryption EncryptionConfig `yaml:"encryption"`
	Networks   NetworksConfig   `yaml:"networks"`
	Fees       FeesConfig       `yaml:"fees"`
	Derivation DerivationConfig `yaml:"derivation"`
	Security   SecurityConfig   `yaml:"security"`
	Output     OutputConfig     `yaml:"output"`
	Logging    LoggingConfig    `yaml:"logging"`
}

// EncryptionConfig defines encryption settings.
type EncryptionConfig struct {
	Method        string `yaml:"method"`
	IdentityFile  string `yaml:"identity_file"`
	KeyDerivation string `yaml:"key_derivation"`
}

// NetworksConfig defines per-chain network settings.
type NetworksConfig struct {
	ETH ETHNetworkConfig `yaml:"eth"`
	BSV BSVNetworkConfig `yaml:"bsv"`
	BTC BTCNetworkConfig `yaml:"btc"`
	BCH BCHNetworkConfig `yaml:"bch"`
}

// ETHNetworkConfig defines Ethereum network settings.
type ETHNetworkConfig struct {
	Enabled bool          `yaml:"enabled"`
	RPC     string        `yaml:"rpc"`
	ChainID int           `yaml:"chain_id"`
	Tokens  []TokenConfig `yaml:"tokens"`
}

// TokenConfig defines an ERC-20 token to track.
type TokenConfig struct {
	Symbol   string `yaml:"symbol"`
	Address  string `yaml:"address"`
	Decimals int    `yaml:"decimals"`
}

// BSVNetworkConfig defines BSV network settings.
type BSVNetworkConfig struct {
	Enabled   bool   `yaml:"enabled"`
	API       string `yaml:"api"`
	Broadcast string `yaml:"broadcast"`
	APIKey    string `yaml:"api_key"`
}

// BTCNetworkConfig defines BTC network settings.
type BTCNetworkConfig struct {
	Enabled bool   `yaml:"enabled"`
	API     string `yaml:"api"`
}

// BCHNetworkConfig defines BCH network settings.
type BCHNetworkConfig struct {
	Enabled bool   `yaml:"enabled"`
	API     string `yaml:"api"`
}

// FeesConfig defines fee estimation settings.
type FeesConfig struct {
	Provider            string `yaml:"provider"`
	FallbackSatsPerByte int    `yaml:"fallback_sats_per_byte"`
	MaxSatsPerByte      int    `yaml:"max_sats_per_byte"`
	ETHGasStrategy      string `yaml:"eth_gas_strategy"`
}

// DerivationConfig defines key derivation settings.
type DerivationConfig struct {
	DefaultAccount int               `yaml:"default_account"`
	AddressGap     int               `yaml:"address_gap"`
	Paths          map[string]string `yaml:"paths"`
}

// SecurityConfig defines security settings.
type SecurityConfig struct {
	AutoLockSeconds     int     `yaml:"auto_lock_seconds"`
	RequireConfirmAbove float64 `yaml:"require_confirm_above"`
	MemoryLock          bool    `yaml:"memory_lock"`
}

// OutputConfig defines output formatting settings.
type OutputConfig struct {
	DefaultFormat string `yaml:"default_format"`
	Color         string `yaml:"color"`
	Verbose       bool   `yaml:"verbose"`
}

// LoggingConfig defines logging settings.
type LoggingConfig struct {
	Level string `yaml:"level"`
	File  string `yaml:"file"`
}

// ConfigDefaults returns the default configuration.
func ConfigDefaults() *Config {
	return &Config{
		Version: 1,
		Home:    "~/.sigil",
		Encryption: EncryptionConfig{
			Method:        "age",
			IdentityFile:  "~/.sigil/identity.age",
			KeyDerivation: "argon2id",
		},
		Networks: NetworksConfig{
			ETH: ETHNetworkConfig{
				Enabled: true,
				RPC:     "", // User must configure
				ChainID: 1,
				Tokens: []TokenConfig{
					{
						Symbol:   "USDC",
						Address:  "0xA0b86991c6218b36c1d19D4a2e9Eb0cE3606eB48",
						Decimals: 6,
					},
				},
			},
			BSV: BSVNetworkConfig{
				Enabled:   true,
				API:       "whatsonchain",
				Broadcast: "taal",
				APIKey:    "",
			},
			BTC: BTCNetworkConfig{
				Enabled: false, // Phase 2
				API:     "mempool",
			},
			BCH: BCHNetworkConfig{
				Enabled: false, // Phase 2
				API:     "fullstack",
			},
		},
		Fees: FeesConfig{
			Provider:            "taal",
			FallbackSatsPerByte: 1,
			MaxSatsPerByte:      100,
			ETHGasStrategy:      "medium",
		},
		Derivation: DerivationConfig{
			DefaultAccount: 0,
			AddressGap:     20,
			Paths:          map[string]string{},
		},
		Security: SecurityConfig{
			AutoLockSeconds:     0, // Disabled for MVP
			RequireConfirmAbove: 0, // Disabled for MVP
			MemoryLock:          true,
		},
		Output: OutputConfig{
			DefaultFormat: "auto",
			Color:         "auto",
			Verbose:       false,
		},
		Logging: LoggingConfig{
			Level: "error",
			File:  "~/.sigil/sigil.log",
		},
	}
}

// Config-related errors.
var (
	ErrConfigNotFound = Error{Code: "CONFIG_NOT_FOUND", Message: "configuration file not found"}
	ErrConfigInvalid  = Error{Code: "CONFIG_INVALID", Message: "configuration file is invalid"}
	ErrConfigPath     = Error{Code: "CONFIG_PATH_INVALID", Message: "configuration path does not exist"}
)
