// Package config provides configuration management for Sigil.
package config

import (
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// Config represents the application configuration.
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
	Enabled      bool          `yaml:"enabled"`
	RPC          string        `yaml:"rpc"`
	FallbackRPCs []string      `yaml:"fallback_rpcs,omitempty"`
	ChainID      int           `yaml:"chain_id"`
	Tokens       []TokenConfig `yaml:"tokens"`
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
	SessionEnabled      bool    `yaml:"session_enabled"`
	SessionTTLMinutes   int     `yaml:"session_ttl_minutes"`
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

// Load reads configuration from the specified file.
func Load(path string) (*Config, error) {
	// #nosec G304 -- config file path is from validated user input
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	cfg := Defaults()
	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, err
	}

	return cfg, nil
}

// Save writes configuration to the specified file.
func Save(cfg *Config, path string) error {
	// Ensure directory exists
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return err
	}

	data, err := yaml.Marshal(cfg)
	if err != nil {
		return err
	}

	return os.WriteFile(path, data, 0o600)
}

// Path returns the default config file path.
func Path(home string) string {
	return filepath.Join(home, "config.yaml")
}

// GetHome returns the sigil home directory path.
func (c *Config) GetHome() string {
	return c.Home
}

// GetETHRPC returns the Ethereum RPC URL.
func (c *Config) GetETHRPC() string {
	return c.Networks.ETH.RPC
}

// GetETHFallbackRPCs returns the fallback Ethereum RPC URLs.
func (c *Config) GetETHFallbackRPCs() []string {
	return c.Networks.ETH.FallbackRPCs
}

// GetBSVAPIKey returns the BSV API key.
func (c *Config) GetBSVAPIKey() string {
	return c.Networks.BSV.APIKey
}

// GetLoggingLevel returns the configured logging level.
func (c *Config) GetLoggingLevel() string {
	return c.Logging.Level
}

// GetLoggingFile returns the configured log file path.
func (c *Config) GetLoggingFile() string {
	return c.Logging.File
}

// GetOutputFormat returns the default output format.
func (c *Config) GetOutputFormat() string {
	return c.Output.DefaultFormat
}

// IsVerbose returns true if verbose output is enabled.
func (c *Config) IsVerbose() bool {
	return c.Output.Verbose
}

// GetSecurity returns the security configuration.
func (c *Config) GetSecurity() SecurityConfig {
	return c.Security
}

// DefaultHome returns the default sigil home directory.
func DefaultHome() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ".sigil"
	}
	return filepath.Join(home, ".sigil")
}
