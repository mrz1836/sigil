package cli

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mrz1836/sigil/internal/chain"
	"github.com/mrz1836/sigil/internal/utxostore"
	"github.com/mrz1836/sigil/internal/wallet"
)

func TestFormatQRData(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		address  string
		expected string
	}{
		{
			name:     "BSV address",
			address:  "1A1zP1eP5QGefi2DMPTfTL5SLmv7DivfNa",
			expected: "1A1zP1eP5QGefi2DMPTfTL5SLmv7DivfNa",
		},
		{
			name:     "ETH address",
			address:  "0x742d35Cc6634C0532925a3b844Bc454e4438f44e",
			expected: "0x742d35Cc6634C0532925a3b844Bc454e4438f44e",
		},
		{
			name:     "empty address",
			address:  "",
			expected: "",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			result := formatQRData(tc.address)
			assert.Equal(t, tc.expected, result)
		})
	}
}

func TestFindUnusedReceiveAddress(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		addresses     []wallet.Address
		usedAddresses map[string]bool // addresses with HasActivity=true
		expectedAddr  string
		expectedNil   bool
	}{
		{
			name: "first address unused",
			addresses: []wallet.Address{
				{Index: 0, Address: "addr0", Path: "m/44'/236'/0'/0/0"},
				{Index: 1, Address: "addr1", Path: "m/44'/236'/0'/0/1"},
			},
			usedAddresses: map[string]bool{},
			expectedAddr:  "addr0",
		},
		{
			name: "first address used, second unused",
			addresses: []wallet.Address{
				{Index: 0, Address: "addr0", Path: "m/44'/236'/0'/0/0"},
				{Index: 1, Address: "addr1", Path: "m/44'/236'/0'/0/1"},
			},
			usedAddresses: map[string]bool{"addr0": true},
			expectedAddr:  "addr1",
		},
		{
			name: "all addresses used",
			addresses: []wallet.Address{
				{Index: 0, Address: "addr0", Path: "m/44'/236'/0'/0/0"},
				{Index: 1, Address: "addr1", Path: "m/44'/236'/0'/0/1"},
			},
			usedAddresses: map[string]bool{"addr0": true, "addr1": true},
			expectedNil:   true,
		},
		{
			name:        "no addresses",
			addresses:   []wallet.Address{},
			expectedNil: true,
		},
		{
			name: "middle address unused",
			addresses: []wallet.Address{
				{Index: 0, Address: "addr0", Path: "m/44'/236'/0'/0/0"},
				{Index: 1, Address: "addr1", Path: "m/44'/236'/0'/0/1"},
				{Index: 2, Address: "addr2", Path: "m/44'/236'/0'/0/2"},
			},
			usedAddresses: map[string]bool{"addr0": true, "addr2": true},
			expectedAddr:  "addr1",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			// Create wallet with addresses
			wlt := &wallet.Wallet{
				Name: "test",
				Addresses: map[wallet.ChainID][]wallet.Address{
					wallet.ChainBSV: tc.addresses,
				},
			}

			// Create UTXO store with used addresses marked
			store := utxostore.New(t.TempDir())
			for addr, used := range tc.usedAddresses {
				store.AddAddress(&utxostore.AddressMetadata{
					Address:     addr,
					ChainID:     chain.BSV,
					HasActivity: used,
				})
			}

			result := findUnusedReceiveAddress(wlt, wallet.ChainBSV, store)

			if tc.expectedNil {
				assert.Nil(t, result)
			} else {
				assert.NotNil(t, result)
				assert.Equal(t, tc.expectedAddr, result.Address)
			}
		})
	}
}

func TestDisplayReceiveText(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		addr     *wallet.Address
		chainID  chain.ID
		label    string
		isNew    bool
		contains []string
	}{
		{
			name: "new BSV address with label",
			addr: &wallet.Address{
				Index:   5,
				Address: "1TestAddress12345678901234567890",
				Path:    "m/44'/236'/0'/0/5",
			},
			chainID: chain.BSV,
			label:   "Payment from Alice",
			isNew:   true,
			contains: []string{
				"New receiving address generated:",
				"Chain:   bsv",
				"Address: 1TestAddress12345678901234567890",
				"Path:    m/44'/236'/0'/0/5",
				"Index:   5",
				"Label:   Payment from Alice",
				"WhatsOnChain",
			},
		},
		{
			name: "existing BSV address without label",
			addr: &wallet.Address{
				Index:   0,
				Address: "1ExistingAddress",
				Path:    "m/44'/236'/0'/0/0",
			},
			chainID: chain.BSV,
			label:   "",
			isNew:   false,
			contains: []string{
				"Receiving address:",
				"Chain:   bsv",
				"Address: 1ExistingAddress",
			},
		},
		{
			name: "ETH address",
			addr: &wallet.Address{
				Index:   0,
				Address: "0x742d35Cc6634C0532925a3b844Bc454e4438f44e",
				Path:    "m/44'/60'/0'/0/0",
			},
			chainID: chain.ETH,
			label:   "",
			isNew:   false,
			contains: []string{
				"Receiving address:",
				"Chain:   eth",
				"0x742d35Cc6634C0532925a3b844Bc454e4438f44e",
				"Etherscan",
			},
		},
		{
			name: "BTC address no explorer link",
			addr: &wallet.Address{
				Index:   0,
				Address: "bc1qtest",
				Path:    "m/44'/0'/0'/0/0",
			},
			chainID: chain.BTC,
			label:   "",
			isNew:   false,
			contains: []string{
				"Receiving address:",
				"Chain:   btc",
				"bc1qtest",
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			cmd := &cobra.Command{}
			var buf bytes.Buffer
			cmd.SetOut(&buf)

			displayReceiveText(cmd, tc.addr, tc.chainID, tc.label, tc.isNew)

			result := buf.String()
			for _, s := range tc.contains {
				assert.Contains(t, result, s)
			}
		})
	}
}

func TestDisplayReceiveText_NoLabelExcluded(t *testing.T) {
	t.Parallel()

	addr := &wallet.Address{
		Index:   0,
		Address: "1TestAddress",
		Path:    "m/44'/236'/0'/0/0",
	}

	cmd := &cobra.Command{}
	var buf bytes.Buffer
	cmd.SetOut(&buf)

	displayReceiveText(cmd, addr, chain.BSV, "", false)

	result := buf.String()
	assert.NotContains(t, result, "Label:")
}

func TestDisplayReceiveJSON(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		addr     *wallet.Address
		chainID  chain.ID
		label    string
		isNew    bool
		contains []string
	}{
		{
			name: "new address with label",
			addr: &wallet.Address{
				Index:   3,
				Address: "1TestJSONAddress",
				Path:    "m/44'/236'/0'/0/3",
			},
			chainID: chain.BSV,
			label:   "Test label",
			isNew:   true,
			contains: []string{
				`"chain": "bsv"`,
				`"address": "1TestJSONAddress"`,
				`"path": "m/44'/236'/0'/0/3"`,
				`"index": 3`,
				`"label": "Test label"`,
				`"is_new": true`,
			},
		},
		{
			name: "existing address without label",
			addr: &wallet.Address{
				Index:   0,
				Address: "1ExistingAddr",
				Path:    "m/44'/236'/0'/0/0",
			},
			chainID: chain.BSV,
			label:   "",
			isNew:   false,
			contains: []string{
				`"chain": "bsv"`,
				`"address": "1ExistingAddr"`,
				`"is_new": false`,
			},
		},
		{
			name: "ETH address",
			addr: &wallet.Address{
				Index:   0,
				Address: "0xETHAddress",
				Path:    "m/44'/60'/0'/0/0",
			},
			chainID: chain.ETH,
			label:   "",
			isNew:   true,
			contains: []string{
				`"chain": "eth"`,
				`"address": "0xETHAddress"`,
				`"is_new": true`,
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			cmd := &cobra.Command{}
			var buf bytes.Buffer
			cmd.SetOut(&buf)

			displayReceiveJSON(cmd, tc.addr, tc.chainID, tc.label, tc.isNew)

			var parsed map[string]any
			require.NoError(t, json.Unmarshal(buf.Bytes(), &parsed))
			assert.Equal(t, string(tc.chainID), parsed["chain"])
			assert.Equal(t, tc.addr.Address, parsed["address"])
			assert.Equal(t, tc.addr.Path, parsed["path"])
			assert.InDelta(t, float64(tc.addr.Index), parsed["index"], 0)
			assert.Equal(t, tc.isNew, parsed["is_new"])
			if tc.label == "" {
				_, hasLabel := parsed["label"]
				assert.False(t, hasLabel)
			} else {
				assert.Equal(t, tc.label, parsed["label"])
			}
		})
	}
}

func TestDisplayReceiveJSON_NoLabel(t *testing.T) {
	t.Parallel()

	addr := &wallet.Address{
		Index:   0,
		Address: "1TestAddress",
		Path:    "m/44'/236'/0'/0/0",
	}

	cmd := &cobra.Command{}
	var buf bytes.Buffer
	cmd.SetOut(&buf)

	displayReceiveJSON(cmd, addr, chain.BSV, "", false)

	result := buf.String()
	assert.NotContains(t, result, `"label"`)
}

func TestDisplayReceiveJSON_Escaping(t *testing.T) {
	t.Parallel()

	addr := &wallet.Address{
		Index:   12,
		Address: "1Addr\"Line",
		Path:    "m/44'/236'/0'/0/12",
	}

	cmd := &cobra.Command{}
	var buf bytes.Buffer
	cmd.SetOut(&buf)

	displayReceiveJSON(cmd, addr, chain.BSV, "line1\nline2 \"quoted\" \u2713", true)

	var parsed map[string]any
	require.NoError(t, json.Unmarshal(buf.Bytes(), &parsed))
	assert.Equal(t, addr.Address, parsed["address"])
	assert.Equal(t, "line1\nline2 \"quoted\" \u2713", parsed["label"])
}
