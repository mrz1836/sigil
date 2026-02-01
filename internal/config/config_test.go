package config_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mrz1836/sigil/internal/config"
)

func TestLoadSave_RoundTrip(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "config.yaml")

	// Create config with custom values
	cfg := config.Defaults()
	cfg.Networks.ETH.RPC = "https://mainnet.infura.io/v3/YOUR-KEY"
	cfg.Networks.BSV.APIKey = "test-api-key"
	cfg.Output.Verbose = true

	// Save
	err := config.Save(cfg, path)
	require.NoError(t, err)

	// Verify file exists
	_, err = os.Stat(path)
	require.NoError(t, err)

	// Load
	loaded, err := config.Load(path)
	require.NoError(t, err)

	// Verify values
	assert.Equal(t, cfg.Version, loaded.Version)
	assert.Equal(t, cfg.Networks.ETH.RPC, loaded.Networks.ETH.RPC)
	assert.Equal(t, cfg.Networks.BSV.APIKey, loaded.Networks.BSV.APIKey)
	assert.Equal(t, cfg.Output.Verbose, loaded.Output.Verbose)
}

func TestDefaults(t *testing.T) {
	t.Parallel()
	cfg := config.Defaults()

	assert.Equal(t, 1, cfg.Version)
	assert.Equal(t, "~/.sigil", cfg.Home)
	assert.Equal(t, "age", cfg.Encryption.Method)
	assert.True(t, cfg.Networks.ETH.Enabled)
	assert.True(t, cfg.Networks.BSV.Enabled)
	assert.False(t, cfg.Networks.BTC.Enabled)
	assert.False(t, cfg.Networks.BCH.Enabled)
	assert.Equal(t, "whatsonchain", cfg.Networks.BSV.API)
	assert.Equal(t, "taal", cfg.Networks.BSV.Broadcast)
	assert.Equal(t, 20, cfg.Derivation.AddressGap)
	assert.True(t, cfg.Security.MemoryLock)
	assert.True(t, cfg.Security.SessionEnabled)
	assert.Equal(t, 15, cfg.Security.SessionTTLMinutes)
	assert.Equal(t, "auto", cfg.Output.DefaultFormat)
	assert.Equal(t, "error", cfg.Logging.Level)
}

func TestDefaults_USDCToken(t *testing.T) {
	t.Parallel()
	cfg := config.Defaults()

	require.Len(t, cfg.Networks.ETH.Tokens, 1)
	assert.Equal(t, "USDC", cfg.Networks.ETH.Tokens[0].Symbol)
	assert.Equal(t, "0xA0b86991c6218b36c1d19D4a2e9Eb0cE3606eB48", cfg.Networks.ETH.Tokens[0].Address)
	assert.Equal(t, 6, cfg.Networks.ETH.Tokens[0].Decimals)
}

func TestDefaults_ETHRPCDefault(t *testing.T) {
	t.Parallel()
	cfg := config.Defaults()
	assert.Equal(t, config.DefaultETHRPCURL, cfg.Networks.ETH.RPC)
	assert.Equal(t, "https://ethereum-rpc.publicnode.com", cfg.Networks.ETH.RPC)
}

func TestDefaults_ETHFallbackRPCs(t *testing.T) {
	t.Parallel()
	cfg := config.Defaults()
	assert.Equal(t, config.DefaultETHFallbackRPCs, cfg.Networks.ETH.FallbackRPCs)
	require.Len(t, cfg.Networks.ETH.FallbackRPCs, 2)
	assert.Equal(t, "https://rpc.ankr.com/eth", cfg.Networks.ETH.FallbackRPCs[0])
	assert.Equal(t, "https://1rpc.io/eth", cfg.Networks.ETH.FallbackRPCs[1])
}

func TestConfig_GetETHFallbackRPCs(t *testing.T) {
	t.Parallel()
	cfg := config.Defaults()
	fallbacks := cfg.GetETHFallbackRPCs()
	require.Len(t, fallbacks, 2)
	assert.Equal(t, "https://rpc.ankr.com/eth", fallbacks[0])
	assert.Equal(t, "https://1rpc.io/eth", fallbacks[1])
}

func TestConfig_GetETHFallbackRPCs_Empty(t *testing.T) {
	t.Parallel()
	cfg := config.Defaults()
	cfg.Networks.ETH.FallbackRPCs = nil
	fallbacks := cfg.GetETHFallbackRPCs()
	assert.Empty(t, fallbacks)
}

func TestLoad_FileNotFound(t *testing.T) {
	t.Parallel()
	_, err := config.Load("/nonexistent/config.yaml")
	assert.Error(t, err)
}

func TestLoad_InvalidYAML(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "config.yaml")

	err := os.WriteFile(path, []byte("invalid: yaml: content: ["), 0o600)
	require.NoError(t, err)

	_, err = config.Load(path)
	assert.Error(t, err)
}

func TestSave_CreatesDirectory(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "subdir", "config.yaml")

	cfg := config.Defaults()
	err := config.Save(cfg, path)
	require.NoError(t, err)

	// Verify directory was created
	info, err := os.Stat(filepath.Dir(path))
	require.NoError(t, err)
	assert.True(t, info.IsDir())
}

func TestApplyEnvironment(t *testing.T) {
	cfg := config.Defaults()

	// Set environment variables
	t.Setenv("SIGIL_HOME", "/custom/home")
	t.Setenv("SIGIL_ETH_RPC", "https://custom-rpc.example.com")
	t.Setenv("SIGIL_BSV_API_KEY", "custom-api-key")
	t.Setenv("SIGIL_OUTPUT_FORMAT", "json")
	t.Setenv("SIGIL_VERBOSE", "true")
	t.Setenv("SIGIL_LOG_LEVEL", "debug")

	config.ApplyEnvironment(cfg)

	assert.Equal(t, "/custom/home", cfg.Home)
	assert.Equal(t, "https://custom-rpc.example.com", cfg.Networks.ETH.RPC)
	assert.Equal(t, "custom-api-key", cfg.Networks.BSV.APIKey)
	assert.Equal(t, "json", cfg.Output.DefaultFormat)
	assert.True(t, cfg.Output.Verbose)
	assert.Equal(t, "debug", cfg.Logging.Level)
}

func TestApplyEnvironment_NoColor(t *testing.T) {
	// Can't use t.Parallel() with t.Setenv()
	cfg := config.Defaults()

	t.Setenv("NO_COLOR", "1")
	config.ApplyEnvironment(cfg)

	assert.Equal(t, "never", cfg.Output.Color)
}

func TestApplyEnvironment_VerboseValues(t *testing.T) {
	// Can't use t.Parallel() with t.Setenv()
	tests := []struct {
		value    string
		expected bool
	}{
		{"true", true},
		{"TRUE", true},
		{"1", true},
		{"yes", true},
		{"on", true},
		{"false", false},
		{"0", false},
		{"no", false},
		{"", false},
	}

	for _, tt := range tests {
		t.Run(tt.value, func(t *testing.T) {
			cfg := config.Defaults()
			t.Setenv("SIGIL_VERBOSE", tt.value)
			config.ApplyEnvironment(cfg)
			assert.Equal(t, tt.expected, cfg.Output.Verbose)
		})
	}
}

func TestConfigPath(t *testing.T) {
	t.Parallel()
	path := config.Path("/home/user/.sigil")
	assert.Equal(t, "/home/user/.sigil/config.yaml", path)
}

func TestDefaultHome(t *testing.T) {
	t.Parallel()
	home := config.DefaultHome()
	assert.Contains(t, home, ".sigil")
}

func TestApplyEnvironment_SessionTTL(t *testing.T) {
	cfg := config.Defaults()

	t.Setenv("SIGIL_SESSION_TTL", "30")
	config.ApplyEnvironment(cfg)

	assert.Equal(t, 30, cfg.Security.SessionTTLMinutes)
}

func TestApplyEnvironment_SessionTTL_InvalidValues(t *testing.T) {
	tests := []struct {
		name     string
		value    string
		expected int
	}{
		{"invalid string", "abc", 15},
		{"zero", "0", 15},
		{"negative", "-5", 15},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := config.Defaults()
			t.Setenv("SIGIL_SESSION_TTL", tt.value)
			config.ApplyEnvironment(cfg)
			assert.Equal(t, tt.expected, cfg.Security.SessionTTLMinutes)
		})
	}
}
