package output

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"rsc.io/qr"
)

func TestDefaultQRConfig(t *testing.T) {
	cfg := DefaultQRConfig()

	assert.Equal(t, qr.L, cfg.Level, "default level should be L (low)")
	assert.Equal(t, 1, cfg.QuietZone, "default quiet zone should be 1")
	assert.True(t, cfg.HalfBlocks, "half blocks should be enabled by default")
}

func TestCanRenderQR_Buffer(t *testing.T) {
	var buf bytes.Buffer
	assert.False(t, CanRenderQR(&buf), "bytes.Buffer should not be a terminal")
}

func TestCanRenderQR_Nil(t *testing.T) {
	assert.False(t, CanRenderQR(nil), "nil writer should not be a terminal")
}

func TestRenderQR_NonTerminal(t *testing.T) {
	var buf bytes.Buffer
	cfg := DefaultQRConfig()

	err := RenderQR(&buf, "1A1zP1eP5QGefi2DMPTfTL5SLmv7DivfNa", cfg)

	require.NoError(t, err, "RenderQR should not error for non-terminal")
	assert.Empty(t, buf.String(), "no output should be produced for non-terminal")
}

func TestRenderQR_ValidAddress(t *testing.T) {
	// This test verifies that RenderQR doesn't panic or error with valid input.
	// We can't test actual output without a real terminal.
	var buf bytes.Buffer
	cfg := DefaultQRConfig()

	testAddresses := []string{
		"1A1zP1eP5QGefi2DMPTfTL5SLmv7DivfNa",         // BSV/BTC P2PKH
		"0x742d35Cc6634C0532925a3b844Bc9e7595f8b2E0", // ETH
	}

	for _, addr := range testAddresses {
		err := RenderQR(&buf, addr, cfg)
		require.NoError(t, err, "RenderQR should not error for address: %s", addr)
	}
}
