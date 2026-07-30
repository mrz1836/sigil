package bsv

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestIsValidAddressForNetwork verifies the boolean network-scoped validator,
// including cross-network rejection (a mainnet address is invalid on testnet).
func TestIsValidAddressForNetwork(t *testing.T) {
	t.Parallel()

	const mainnetAddr = "1A1zP1eP5QGefi2DMPTfTL5SLmv7DivfNa"
	const testnetAddr = "mrCDrCybB6J1vRfbwM5hemdJz73FwDBC8r"

	tests := []struct {
		name    string
		address string
		network Network
		want    bool
	}{
		{name: "mainnet address on mainnet", address: mainnetAddr, network: NetworkMainnet, want: true},
		{name: "testnet address on testnet", address: testnetAddr, network: NetworkTestnet, want: true},
		{name: "mainnet address on testnet is rejected", address: mainnetAddr, network: NetworkTestnet, want: false},
		{name: "testnet address on mainnet is rejected", address: testnetAddr, network: NetworkMainnet, want: false},
		{name: "garbage on mainnet", address: "not-an-address", network: NetworkMainnet, want: false},
		{name: "garbage on testnet", address: "not-an-address", network: NetworkTestnet, want: false},
		{name: "empty on mainnet", address: "", network: NetworkMainnet, want: false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tc.want, IsValidAddressForNetwork(tc.address, tc.network))
		})
	}
}
