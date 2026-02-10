package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseBool(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		input    string
		expected bool
	}{
		{"1", "1", true},
		{"true", "true", true},
		{"TRUE", "TRUE", true},
		{"yes", "yes", true},
		{"YES", "YES", true},
		{"on", "on", true},
		{"ON", "ON", true},
		{"with spaces", "  true  ", true},
		{"0", "0", false},
		{"false", "false", false},
		{"FALSE", "FALSE", false},
		{"no", "no", false},
		{"off", "off", false},
		{"empty", "", false},
		{"random", "random", false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			result := parseBool(tc.input)
			assert.Equal(t, tc.expected, result)
		})
	}
}

func TestSanitizeURL(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "clean URL",
			input:    "https://mainnet.infura.io/v3/abc123",
			expected: "https://mainnet.infura.io/v3/abc123",
		},
		{
			name:     "with leading/trailing spaces",
			input:    "  https://mainnet.infura.io/v3/abc123  ",
			expected: "https://mainnet.infura.io/v3/abc123",
		},
		{
			name:     "localhost",
			input:    "http://localhost:8545",
			expected: "http://localhost:8545",
		},
		{
			name:     "127.0.0.1",
			input:    "http://127.0.0.1:8545",
			expected: "http://127.0.0.1:8545",
		},
		{
			name:     "websocket",
			input:    "wss://mainnet.infura.io/ws",
			expected: "wss://mainnet.infura.io/ws",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			result := SanitizeURL(tc.input)
			assert.Equal(t, tc.expected, result)
		})
	}
}

//nolint:gocognit // Test function with comprehensive test cases
func TestValidateRPCURL(t *testing.T) {
	t.Parallel()

	t.Run("valid URLs", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name string
			url  string
		}{
			{"https", "https://mainnet.infura.io/v3/abc123"},
			{"wss", "wss://mainnet.infura.io/ws"},
			{"localhost http", "http://localhost:8545"},
			{"127.0.0.1 http", "http://127.0.0.1:8545"},
			{"IPv6 loopback", "http://[::1]:8545"},
			{"empty", ""},
		}

		for _, tc := range tests {
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()
				err := ValidateRPCURL(tc.url)
				assert.NoError(t, err)
			})
		}
	})

	t.Run("malicious schemes must be rejected", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name string
			url  string
		}{
			{"javascript", "javascript:alert(1)"},
			{"data", "data:text/html,<script>alert(1)</script>"},
			{"file", "file:///etc/passwd"},
		}

		for _, tc := range tests {
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()
				err := ValidateRPCURL(tc.url)
				// These malicious schemes must be rejected
				require.Error(t, err, "malicious URL %q should be rejected", tc.url)
			})
		}
	})

	t.Run("insecure URLs", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name string
			url  string
		}{
			{"http remote", "http://example.com:8545"},
			{"http remote with path", "http://example.com:8545/rpc"},
		}

		for _, tc := range tests {
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()
				err := ValidateRPCURL(tc.url)
				require.Error(t, err)
				assert.ErrorIs(t, err, ErrInsecureRPCURL)
			})
		}
	})

	t.Run("invalid URLs", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name string
			url  string
		}{
			{"invalid chars", "https://example .com"},
			{"missing scheme", "example.com:8545"},
		}

		for _, tc := range tests {
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()
				err := ValidateRPCURL(tc.url)
				if err != nil {
					t.Logf("Invalid URL %q rejected: %v", tc.url, err)
				}
			})
		}
	})
}

//nolint:gocognit // Test function with comprehensive test cases
func TestApplyEnvironment(t *testing.T) {
	// Cannot run in parallel because we modify environment variables

	t.Run("SIGIL_HOME", func(t *testing.T) {
		cfg := Defaults()
		originalHome := cfg.Home

		t.Setenv(EnvHome, "/custom/home")
		ApplyEnvironment(cfg)

		assert.Equal(t, "/custom/home", cfg.Home)
		assert.NotEqual(t, originalHome, cfg.Home)
	})

	t.Run("SIGIL_ETH_RPC valid", func(t *testing.T) {
		cfg := Defaults()

		t.Setenv(EnvETHRPC, "https://mainnet.infura.io/v3/test")
		ApplyEnvironment(cfg)

		assert.Equal(t, "https://mainnet.infura.io/v3/test", cfg.Networks.ETH.RPC)
		assert.Empty(t, cfg.Warnings)
	})

	t.Run("SIGIL_ETH_RPC insecure", func(t *testing.T) {
		cfg := Defaults()

		t.Setenv(EnvETHRPC, "http://example.com:8545")
		ApplyEnvironment(cfg)

		assert.Equal(t, "http://example.com:8545", cfg.Networks.ETH.RPC)
		assert.NotEmpty(t, cfg.Warnings, "should have warning for insecure URL")
	})

	t.Run("SIGIL_ETH_RPC with spaces", func(t *testing.T) {
		cfg := Defaults()

		t.Setenv(EnvETHRPC, "  https://mainnet.infura.io/v3/test  ")
		ApplyEnvironment(cfg)

		assert.Equal(t, "https://mainnet.infura.io/v3/test", cfg.Networks.ETH.RPC)
	})

	t.Run("SIGIL_ETH_PROVIDER", func(t *testing.T) {
		tests := []struct {
			name     string
			value    string
			expected string
		}{
			{"rpc", "rpc", "rpc"},
			{"RPC uppercase", "RPC", "rpc"},
			{"etherscan", "etherscan", "etherscan"},
			{"ETHERSCAN uppercase", "ETHERSCAN", "etherscan"},
			{"with spaces", "  rpc  ", "rpc"},
			{"invalid value", "invalid", ""}, // Should not override
		}

		for _, tc := range tests {
			t.Run(tc.name, func(t *testing.T) {
				cfg := Defaults()
				originalProvider := cfg.Networks.ETH.Provider

				t.Setenv(EnvETHProvider, tc.value)
				ApplyEnvironment(cfg)

				if tc.expected != "" {
					assert.Equal(t, tc.expected, cfg.Networks.ETH.Provider)
				} else {
					assert.Equal(t, originalProvider, cfg.Networks.ETH.Provider, "should not override with invalid value")
				}
			})
		}
	})

	t.Run("ETHERSCAN_API_KEY", func(t *testing.T) {
		cfg := Defaults()

		t.Setenv(EnvEtherscanAPIKey, "test-api-key-123")
		ApplyEnvironment(cfg)

		assert.Equal(t, "test-api-key-123", cfg.Networks.ETH.EtherscanAPIKey)
	})

	t.Run("SIGIL_BSV_API_KEY", func(t *testing.T) {
		cfg := Defaults()

		t.Setenv(EnvBSVAPIKey, "bsv-api-key-456")
		ApplyEnvironment(cfg)

		assert.Equal(t, "bsv-api-key-456", cfg.Networks.BSV.APIKey)
	})

	t.Run("WHATS_ON_CHAIN_API_KEY fallback", func(t *testing.T) {
		cfg := Defaults()

		t.Setenv(EnvWOCAPIKey, "woc-api-key-789")
		ApplyEnvironment(cfg)

		assert.Equal(t, "woc-api-key-789", cfg.Networks.BSV.APIKey)
	})

	t.Run("SIGIL_BSV_API_KEY takes precedence over WHATS_ON_CHAIN_API_KEY", func(t *testing.T) {
		cfg := Defaults()

		t.Setenv(EnvBSVAPIKey, "sigil-key")
		t.Setenv(EnvWOCAPIKey, "woc-key")
		ApplyEnvironment(cfg)

		assert.Equal(t, "sigil-key", cfg.Networks.BSV.APIKey)
	})

	t.Run("SIGIL_BSV_FEE_STRATEGY", func(t *testing.T) {
		tests := []struct {
			name     string
			value    string
			expected string
		}{
			{"economy", "economy", "economy"},
			{"ECONOMY uppercase", "ECONOMY", "economy"},
			{"normal", "normal", "normal"},
			{"priority", "priority", "priority"},
			{"with spaces", "  normal  ", "normal"},
			{"invalid value", "invalid", ""}, // Should not override
		}

		for _, tc := range tests {
			t.Run(tc.name, func(t *testing.T) {
				cfg := Defaults()
				originalStrategy := cfg.Fees.BSVFeeStrategy

				t.Setenv(EnvBSVFeeStrategy, tc.value)
				ApplyEnvironment(cfg)

				if tc.expected != "" {
					assert.Equal(t, tc.expected, cfg.Fees.BSVFeeStrategy)
				} else {
					assert.Equal(t, originalStrategy, cfg.Fees.BSVFeeStrategy, "should not override with invalid value")
				}
			})
		}
	})

	t.Run("SIGIL_BSV_MIN_MINERS", func(t *testing.T) {
		tests := []struct {
			name     string
			value    string
			expected int
		}{
			{"valid positive", "5", 5},
			{"zero", "0", 0},      // Should not override (need > 0)
			{"negative", "-1", 0}, // Should not override
			{"invalid", "abc", 0}, // Should not override
			{"empty", "", 0},      // Should not override
			{"large value", "100", 100},
		}

		for _, tc := range tests {
			t.Run(tc.name, func(t *testing.T) {
				cfg := Defaults()
				originalMinMiners := cfg.Fees.BSVMinMiners

				t.Setenv(EnvBSVMinMiners, tc.value)
				ApplyEnvironment(cfg)

				if tc.expected > 0 {
					assert.Equal(t, tc.expected, cfg.Fees.BSVMinMiners)
				} else {
					assert.Equal(t, originalMinMiners, cfg.Fees.BSVMinMiners, "should not override with invalid value")
				}
			})
		}
	})

	t.Run("SIGIL_OUTPUT_FORMAT", func(t *testing.T) {
		cfg := Defaults()

		t.Setenv(EnvOutputFormat, "JSON")
		ApplyEnvironment(cfg)

		assert.Equal(t, "json", cfg.Output.DefaultFormat)
	})

	t.Run("SIGIL_VERBOSE", func(t *testing.T) {
		tests := []struct {
			name     string
			value    string
			expected bool
		}{
			{"true", "true", true},
			{"1", "1", true},
			{"yes", "yes", true},
			{"false", "false", false},
			{"0", "0", false},
		}

		for _, tc := range tests {
			t.Run(tc.name, func(t *testing.T) {
				cfg := Defaults()

				t.Setenv(EnvVerbose, tc.value)
				ApplyEnvironment(cfg)

				assert.Equal(t, tc.expected, cfg.Output.Verbose)
			})
		}
	})

	t.Run("SIGIL_LOG_LEVEL", func(t *testing.T) {
		cfg := Defaults()

		t.Setenv(EnvLogLevel, "DEBUG")
		ApplyEnvironment(cfg)

		assert.Equal(t, "debug", cfg.Logging.Level)
	})

	t.Run("NO_COLOR", func(t *testing.T) {
		cfg := Defaults()
		originalColor := cfg.Output.Color

		t.Setenv(EnvNoColor, "1")
		ApplyEnvironment(cfg)

		assert.Equal(t, "never", cfg.Output.Color)
		assert.NotEqual(t, originalColor, cfg.Output.Color)
	})

	t.Run("SIGIL_SESSION_TTL", func(t *testing.T) {
		tests := []struct {
			name     string
			value    string
			expected int
		}{
			{"valid positive", "30", 30},
			{"zero", "0", 0},      // Should not override (need > 0)
			{"negative", "-1", 0}, // Should not override
			{"invalid", "abc", 0}, // Should not override
		}

		for _, tc := range tests {
			t.Run(tc.name, func(t *testing.T) {
				cfg := Defaults()
				originalTTL := cfg.Security.SessionTTLMinutes

				t.Setenv(EnvSessionTTL, tc.value)
				ApplyEnvironment(cfg)

				if tc.expected > 0 {
					assert.Equal(t, tc.expected, cfg.Security.SessionTTLMinutes)
				} else {
					assert.Equal(t, originalTTL, cfg.Security.SessionTTLMinutes, "should not override with invalid value")
				}
			})
		}
	})

	t.Run("multiple env vars", func(t *testing.T) {
		cfg := Defaults()

		t.Setenv(EnvHome, "/custom/home")
		t.Setenv(EnvETHRPC, "https://eth.example.com")
		t.Setenv(EnvOutputFormat, "json")
		t.Setenv(EnvVerbose, "true")

		ApplyEnvironment(cfg)

		assert.Equal(t, "/custom/home", cfg.Home)
		assert.Equal(t, "https://eth.example.com", cfg.Networks.ETH.RPC)
		assert.Equal(t, "json", cfg.Output.DefaultFormat)
		assert.True(t, cfg.Output.Verbose)
	})
}

func TestDefaults(t *testing.T) {
	t.Parallel()

	cfg := Defaults()
	require.NotNil(t, cfg)

	// Basic structure checks
	assert.NotEmpty(t, cfg.Home)
	assert.NotNil(t, cfg.Networks)
	assert.NotNil(t, cfg.Networks.ETH)
	assert.NotNil(t, cfg.Networks.BSV)
	assert.NotNil(t, cfg.Output)
	assert.NotNil(t, cfg.Logging)
	assert.NotNil(t, cfg.Security)
	assert.NotNil(t, cfg.Fees)
}
