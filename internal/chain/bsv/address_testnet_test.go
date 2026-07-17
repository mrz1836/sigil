package bsv

import (
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Known Base58Check vectors for private key 0x…0001 (independently verified).
const (
	mainnetP2PKH = "1BgGZ9tcN4rm9KBzDn7KprQz87SZ26SAMH" // version 0x00
	testnetP2PKH = "mrCDrCybB6J1vRfbwM5hemdJz73FwDBC8r" // version 0x6f
)

func TestValidateBase58CheckAddressTestnet(t *testing.T) {
	t.Parallel()

	// Accepts a well-formed testnet P2PKH address.
	require.NoError(t, ValidateBase58CheckAddressTestnet(testnetP2PKH))

	// Rejects a mainnet address on testnet (cross-network protection).
	require.Error(t, ValidateBase58CheckAddressTestnet(mainnetP2PKH))

	// Mainnet validator rejects the testnet address.
	require.Error(t, ValidateBase58CheckAddress(testnetP2PKH))
	require.NoError(t, ValidateBase58CheckAddress(mainnetP2PKH))
}

func TestValidateBase58CheckAddressForNetwork(t *testing.T) {
	t.Parallel()

	require.NoError(t, ValidateBase58CheckAddressForNetwork(mainnetP2PKH, NetworkMainnet))
	require.NoError(t, ValidateBase58CheckAddressForNetwork(testnetP2PKH, NetworkTestnet))
	require.Error(t, ValidateBase58CheckAddressForNetwork(mainnetP2PKH, NetworkTestnet))
	require.Error(t, ValidateBase58CheckAddressForNetwork(testnetP2PKH, NetworkMainnet))
}

// TestTestnetP2SHRoundTrip generates a testnet P2SH (version 0xc4) payload and
// verifies it round-trips and begins with "2".
func TestTestnetP2SHRoundTrip(t *testing.T) {
	t.Parallel()

	payload := make([]byte, payloadLen) // 20 zero bytes is a valid script hash for encoding
	addr := EncodeBase58Check(versionP2SHTestnet, payload)
	assert.True(t, strings.HasPrefix(addr, "2"), "testnet P2SH should start with 2, got %q", addr)

	version, decoded, err := DecodeBase58Check(addr)
	require.NoError(t, err)
	assert.Equal(t, byte(versionP2SHTestnet), version)
	assert.Equal(t, payload, decoded)

	// It validates on testnet (P2SH allowed) but not on mainnet.
	require.NoError(t, ValidateBase58CheckAddressTestnet(addr))
	assert.Error(t, ValidateBase58CheckAddress(addr))
}

func TestClientValidateAddressByNetwork(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	mainClient := NewClient(ctx, &ClientOptions{Network: NetworkMainnet, WOCClient: &mockWOCClient{}})
	testClient := NewClient(ctx, &ClientOptions{Network: NetworkTestnet, WOCClient: &mockWOCClient{}})

	// Mainnet client: accepts mainnet, rejects testnet.
	require.NoError(t, mainClient.ValidateAddress(mainnetP2PKH))
	require.ErrorIs(t, mainClient.ValidateAddress(testnetP2PKH), ErrInvalidAddress)

	// Testnet client: accepts testnet, rejects mainnet.
	require.NoError(t, testClient.ValidateAddress(testnetP2PKH))
	require.ErrorIs(t, testClient.ValidateAddress(mainnetP2PKH), ErrInvalidAddress)
}

// TestTestnetBroadcasterSkipsARC verifies that a testnet client uses only the
// WhatsOnChain SDK broadcaster (GorillaPool ARC is mainnet-only).
func TestTestnetBroadcasterSkipsARC(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	testClient := NewClient(ctx, &ClientOptions{Network: NetworkTestnet})
	require.Len(t, testClient.broadcasters, 1)
	assert.Equal(t, "whatsonchain", testClient.broadcasters[0].Name())

	mainClient := NewClient(ctx, &ClientOptions{Network: NetworkMainnet})
	require.Len(t, mainClient.broadcasters, 2)
	assert.Equal(t, "whatsonchain", mainClient.broadcasters[0].Name())
	assert.Equal(t, "gorillapool", mainClient.broadcasters[1].Name())
}
