package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNormalizeBSVNetwork(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		input  string
		want   string
		wantOK bool
	}{
		{"empty defaults main", "", "main", true},
		{"main", "main", "main", true},
		{"mainnet alias", "mainnet", "main", true},
		{"test", "test", "test", true},
		{"testnet alias", "testnet", "test", true},
		{"uppercase TEST", "TEST", "test", true},
		{"whitespace", "  test  ", "test", true},
		{"invalid", "regtest", "main", false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, ok := NormalizeBSVNetwork(tc.input)
			assert.Equal(t, tc.want, got)
			assert.Equal(t, tc.wantOK, ok)
		})
	}
}

func TestGetBSVNetwork(t *testing.T) {
	t.Parallel()

	cfg := Defaults()
	assert.Equal(t, "main", cfg.GetBSVNetwork(), "default is mainnet")

	cfg.Networks.BSV.Network = "test"
	assert.Equal(t, "test", cfg.GetBSVNetwork())

	cfg.Networks.BSV.Network = "garbage"
	assert.Equal(t, "main", cfg.GetBSVNetwork(), "invalid value falls back to mainnet")
}

func TestNormalizeBTCNetwork(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		input  string
		want   string
		wantOK bool
	}{
		{"empty defaults main", "", "main", true},
		{"main", "main", "main", true},
		{"mainnet alias", "mainnet", "main", true},
		{"test", "test", "test", true},
		{"testnet alias", "testnet", "test", true},
		{"testnet4 alias", "testnet4", "test", true},
		{"uppercase TEST", "TEST", "test", true},
		{"whitespace", "  test  ", "test", true},
		{"invalid", "regtest", "main", false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, ok := NormalizeBTCNetwork(tc.input)
			assert.Equal(t, tc.want, got)
			assert.Equal(t, tc.wantOK, ok)
		})
	}
}

func TestGetBTCNetwork(t *testing.T) {
	t.Parallel()

	cfg := Defaults()
	assert.Equal(t, "main", cfg.GetBTCNetwork(), "default is mainnet")

	cfg.Networks.BTC.Network = "testnet4"
	assert.Equal(t, "test", cfg.GetBTCNetwork(), "testnet4 normalizes to test")

	cfg.Networks.BTC.Network = "garbage"
	assert.Equal(t, "main", cfg.GetBTCNetwork(), "invalid value falls back to mainnet")
}

func TestApplyEnvironmentBTC(t *testing.T) {
	t.Run("network testnet4", func(t *testing.T) {
		t.Setenv(EnvBTCNetwork, "testnet4")
		cfg := Defaults()
		ApplyEnvironment(cfg)
		assert.Equal(t, "test", cfg.GetBTCNetwork())
	})

	t.Run("invalid network warns", func(t *testing.T) {
		t.Setenv(EnvBTCNetwork, "regtest")
		cfg := Defaults()
		ApplyEnvironment(cfg)
		assert.Equal(t, "main", cfg.Networks.BTC.Network)
		require.NotEmpty(t, cfg.Warnings)
	})

	t.Run("api key and fee strategy", func(t *testing.T) {
		t.Setenv(EnvBTCAPIKey, "btc-secret")
		t.Setenv(EnvBTCFeeStrategy, "priority")
		cfg := Defaults()
		ApplyEnvironment(cfg)
		assert.Equal(t, "btc-secret", cfg.GetBTCAPIKey())
		assert.Equal(t, "priority", cfg.GetBTCFeeStrategy())
	})

	t.Run("invalid fee strategy ignored", func(t *testing.T) {
		t.Setenv(EnvBTCFeeStrategy, "bogus")
		cfg := Defaults()
		ApplyEnvironment(cfg)
		assert.Equal(t, "normal", cfg.GetBTCFeeStrategy(), "invalid value keeps default")
	})
}

func TestApplyEnvironmentBSVNetwork(t *testing.T) {
	tests := []struct {
		name        string
		env         string
		want        string
		wantWarning bool
	}{
		{"test", "test", "test", false},
		{"testnet alias", "testnet", "test", false},
		{"main", "main", "main", false},
		{"invalid warns and defaults main", "regtest", "main", true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv(EnvBSVNetwork, tc.env)
			cfg := Defaults()
			ApplyEnvironment(cfg)
			assert.Equal(t, tc.want, cfg.Networks.BSV.Network)
			if tc.wantWarning {
				require.NotEmpty(t, cfg.Warnings)
			}
		})
	}
}
