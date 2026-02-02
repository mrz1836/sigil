package cli

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestReceiveQRFlagRegistered(t *testing.T) {
	flag := receiveCmd.Flags().Lookup("qr")
	assert.NotNil(t, flag, "--qr flag should be registered")
	assert.Equal(t, "false", flag.DefValue, "default value should be false")
	assert.Equal(t, "bool", flag.Value.Type(), "flag type should be bool")
}

func TestFormatQRData_BSV(t *testing.T) {
	// BSV P2PKH address
	address := "1A1zP1eP5QGefi2DMPTfTL5SLmv7DivfNa"
	result := formatQRData(address)

	// Plain address for maximum wallet compatibility
	assert.Equal(t, address, result, "formatQRData should return plain address")
}

func TestFormatQRData_ETH(t *testing.T) {
	// ETH address
	address := "0x742d35Cc6634C0532925a3b844Bc9e7595f8b2E0"
	result := formatQRData(address)

	// Plain address for maximum wallet compatibility
	assert.Equal(t, address, result, "formatQRData should return plain address")
}
