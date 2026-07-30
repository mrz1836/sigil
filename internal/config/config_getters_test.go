package config_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/mrz1836/sigil/internal/config"
)

func TestConfig_GetBSVBroadcast(t *testing.T) {
	t.Parallel()
	cfg := config.Defaults()
	cfg.Networks.BSV.Broadcast = "https://custom-broadcast.example.com"
	assert.Equal(t, "https://custom-broadcast.example.com", cfg.GetBSVBroadcast())
}

func TestConfig_GetBSVFeeStrategy(t *testing.T) {
	t.Parallel()
	cfg := config.Defaults()
	cfg.Fees.BSVFeeStrategy = "priority"
	assert.Equal(t, "priority", cfg.GetBSVFeeStrategy())
}

func TestConfig_GetBSVMinMiners(t *testing.T) {
	t.Parallel()
	cfg := config.Defaults()
	cfg.Fees.BSVMinMiners = 5
	assert.Equal(t, 5, cfg.GetBSVMinMiners())
}
