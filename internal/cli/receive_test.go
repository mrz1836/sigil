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

func TestDisplayReceiveText_WithNonEmptyLabel(t *testing.T) {
	t.Parallel()

	addr := &wallet.Address{
		Index:   2,
		Address: "1LabelTestAddr",
		Path:    "m/44'/236'/0'/0/2",
	}

	cmd := &cobra.Command{}
	var buf bytes.Buffer
	cmd.SetOut(&buf)

	displayReceiveText(cmd, addr, chain.BSV, "MyLabel", false)

	result := buf.String()
	assert.Contains(t, result, "Label:   MyLabel")
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

func TestDisplayReceiveCheckText(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		addr     *wallet.Address
		chainID  chain.ID
		label    string
		balance  uint64
		utxos    []*utxostore.StoredUTXO
		contains []string
		excludes []string
	}{
		{
			name: "funds received",
			addr: &wallet.Address{
				Index:   0,
				Address: "1TestAddr12345678901234567890ABC",
				Path:    "m/44'/236'/0'/0/0",
			},
			chainID: chain.BSV,
			label:   "test label",
			balance: 150000,
			utxos: []*utxostore.StoredUTXO{
				{TxID: "abc123", Vout: 0, Amount: 100000},
				{TxID: "def456", Vout: 1, Amount: 50000},
			},
			contains: []string{
				"Receive address check:",
				"Chain:   bsv",
				"Address: 1TestAddr12345678901234567890ABC",
				"Path:    m/44'/236'/0'/0/0",
				"Index:   0",
				"Label:   test label",
				"Status:  Funds received",
				"UTXOs:   2",
				"150000 satoshis",
				"0.00150000 BSV",
				"WhatsOnChain",
			},
		},
		{
			name: "no funds yet",
			addr: &wallet.Address{
				Index:   1,
				Address: "1EmptyAddr",
				Path:    "m/44'/236'/0'/0/1",
			},
			chainID: chain.BSV,
			label:   "",
			balance: 0,
			utxos:   []*utxostore.StoredUTXO{},
			contains: []string{
				"Status:  No funds received yet",
			},
			excludes: []string{"Label:", "UTXOs:"},
		},
		{
			name: "funds without label",
			addr: &wallet.Address{
				Index:   2,
				Address: "1NoLabel",
				Path:    "m/44'/236'/0'/0/2",
			},
			chainID: chain.BSV,
			label:   "",
			balance: 50000,
			utxos: []*utxostore.StoredUTXO{
				{TxID: "tx1", Vout: 0, Amount: 50000},
			},
			contains: []string{
				"Status:  Funds received",
				"UTXOs:   1",
			},
			excludes: []string{"Label:"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			var buf bytes.Buffer
			displayReceiveCheckText(&buf, tc.addr, tc.chainID, tc.label, tc.balance, tc.utxos)

			result := buf.String()
			for _, s := range tc.contains {
				assert.Contains(t, result, s)
			}
			for _, s := range tc.excludes {
				assert.NotContains(t, result, s)
			}
		})
	}
}

func TestDisplayReceiveCheckJSON(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		addr     *wallet.Address
		chainID  chain.ID
		label    string
		balance  uint64
		utxos    []*utxostore.StoredUTXO
		hasFunds bool
	}{
		{
			name: "with funds and label",
			addr: &wallet.Address{
				Index:   0,
				Address: "1TestAddr",
				Path:    "m/44'/236'/0'/0/0",
			},
			chainID:  chain.BSV,
			label:    "test label",
			balance:  100000,
			utxos:    []*utxostore.StoredUTXO{{TxID: "tx1", Vout: 0, Amount: 100000, Confirmations: 3}},
			hasFunds: true,
		},
		{
			name: "empty no label",
			addr: &wallet.Address{
				Index:   1,
				Address: "1EmptyAddr",
				Path:    "m/44'/236'/0'/0/1",
			},
			chainID:  chain.BSV,
			label:    "",
			balance:  0,
			utxos:    []*utxostore.StoredUTXO{},
			hasFunds: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			var buf bytes.Buffer
			displayReceiveCheckJSON(&buf, tc.addr, tc.chainID, tc.label, tc.balance, tc.utxos)

			var parsed map[string]any
			require.NoError(t, json.Unmarshal(buf.Bytes(), &parsed))
			assert.Equal(t, string(tc.chainID), parsed["chain"])
			assert.Equal(t, tc.addr.Address, parsed["address"])
			assert.Equal(t, tc.addr.Path, parsed["path"])
			assert.InDelta(t, float64(tc.addr.Index), parsed["index"], 0)
			assert.Equal(t, tc.hasFunds, parsed["has_funds"])
			assert.InDelta(t, float64(tc.balance), parsed["balance"], 0)
			assert.InDelta(t, float64(tc.balance)/1e8, parsed["balance_bsv"], 1e-12)
			assert.InDelta(t, float64(len(tc.utxos)), parsed["utxo_count"], 0)

			if tc.label == "" {
				_, hasLabel := parsed["label"]
				assert.False(t, hasLabel, "label should be omitted when empty")
			} else {
				assert.Equal(t, tc.label, parsed["label"])
			}

			utxoArr, ok := parsed["utxos"].([]any)
			require.True(t, ok, "utxos should be an array")
			assert.Len(t, utxoArr, len(tc.utxos))
		})
	}
}

func TestDisplayReceiveCheckJSON_EmptyUTXOsNotNull(t *testing.T) {
	t.Parallel()

	addr := &wallet.Address{
		Index:   0,
		Address: "1TestAddr",
		Path:    "m/44'/236'/0'/0/0",
	}

	var buf bytes.Buffer
	displayReceiveCheckJSON(&buf, addr, chain.BSV, "", 0, []*utxostore.StoredUTXO{})

	result := buf.String()
	// Verify utxos is [] not null
	assert.Contains(t, result, `"utxos": []`)
}

func TestDisplayReceiveCheckAllText(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		chainID  chain.ID
		results  []addressCheckResult
		contains []string
		excludes []string
	}{
		{
			name:    "mixed balances",
			chainID: chain.BSV,
			results: []addressCheckResult{
				{
					Addr:    &wallet.Address{Index: 0, Address: "1AAAA12345678901234567890ABCDEF", Path: "m/44'/236'/0'/0/0"},
					Balance: 100000,
					UTXOs:   []*utxostore.StoredUTXO{{TxID: "tx1", Amount: 100000}},
				},
				{
					Addr:    &wallet.Address{Index: 1, Address: "1BBBB12345678901234567890ABCDEF", Path: "m/44'/236'/0'/0/1"},
					Balance: 0,
					UTXOs:   []*utxostore.StoredUTXO{},
				},
				{
					Addr:    &wallet.Address{Index: 2, Address: "1CCCC12345678901234567890ABCDEF", Path: "m/44'/236'/0'/0/2"},
					Label:   "savings",
					Balance: 50000,
					UTXOs:   []*utxostore.StoredUTXO{{TxID: "tx2", Amount: 50000}},
				},
			},
			contains: []string{
				"Checking 3 receiving address(es)",
				"(1 UTXO)",
				"[savings]",
				"0.00150000 BSV",
				"2 UTXOs across 2 addresses",
			},
		},
		{
			name:    "all empty",
			chainID: chain.BSV,
			results: []addressCheckResult{
				{
					Addr:    &wallet.Address{Index: 0, Address: "1Empty1", Path: "m/44'/236'/0'/0/0"},
					Balance: 0,
					UTXOs:   []*utxostore.StoredUTXO{},
				},
			},
			contains: []string{
				"Checking 1 receiving address(es)",
				"0.00000000 BSV (0 UTXOs across 0 addresses)",
			},
		},
		{
			name:    "with errors",
			chainID: chain.BSV,
			results: []addressCheckResult{
				{
					Addr: &wallet.Address{Index: 0, Address: "1Err1", Path: "m/44'/236'/0'/0/0"},
					Err:  errNetworkTimeout,
				},
				{
					Addr:    &wallet.Address{Index: 1, Address: "1OK1", Path: "m/44'/236'/0'/0/1"},
					Balance: 50000,
					UTXOs:   []*utxostore.StoredUTXO{{TxID: "tx1", Amount: 50000}},
				},
			},
			contains: []string{
				"ERROR: network timeout",
				"Errors: 1 address(es) failed to check",
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			var buf bytes.Buffer
			displayReceiveCheckAllText(&buf, tc.chainID, tc.results)

			result := buf.String()
			for _, s := range tc.contains {
				assert.Contains(t, result, s)
			}
			for _, s := range tc.excludes {
				assert.NotContains(t, result, s)
			}
		})
	}
}

func TestDisplayReceiveCheckAllJSON(t *testing.T) {
	t.Parallel()

	results := []addressCheckResult{
		{
			Addr:    &wallet.Address{Index: 0, Address: "1Addr0", Path: "m/44'/236'/0'/0/0"},
			Label:   "primary",
			Balance: 100000,
			UTXOs: []*utxostore.StoredUTXO{
				{TxID: "tx1", Vout: 0, Amount: 60000, Confirmations: 10},
				{TxID: "tx2", Vout: 1, Amount: 40000, Confirmations: 5},
			},
		},
		{
			Addr:    &wallet.Address{Index: 1, Address: "1Addr1", Path: "m/44'/236'/0'/0/1"},
			Balance: 0,
			UTXOs:   []*utxostore.StoredUTXO{},
		},
	}

	var buf bytes.Buffer
	displayReceiveCheckAllJSON(&buf, chain.BSV, results)

	var parsed map[string]any
	require.NoError(t, json.Unmarshal(buf.Bytes(), &parsed))

	assert.Equal(t, "bsv", parsed["chain"])
	assert.InDelta(t, 2.0, parsed["addresses_checked"], 0)
	assert.InDelta(t, 100000.0, parsed["total_balance"], 0)
	assert.InDelta(t, 0.001, parsed["total_balance_bsv"], 1e-12)
	assert.InDelta(t, 2.0, parsed["total_utxo_count"], 0)

	addrs, ok := parsed["addresses"].([]any)
	require.True(t, ok, "addresses should be an array")
	assert.Len(t, addrs, 2)

	// First address has funds
	first, ok := addrs[0].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "1Addr0", first["address"])
	assert.Equal(t, "primary", first["label"])
	assert.Equal(t, true, first["has_funds"])
	firstUTXOs, ok := first["utxos"].([]any)
	require.True(t, ok)
	assert.Len(t, firstUTXOs, 2)

	// Second address is empty
	second, ok := addrs[1].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "1Addr1", second["address"])
	assert.Equal(t, false, second["has_funds"])
	secondUTXOs, ok := second["utxos"].([]any)
	require.True(t, ok)
	assert.Empty(t, secondUTXOs) // [] not null
}

func TestFindWalletAddress(t *testing.T) {
	t.Parallel()

	wlt := &wallet.Wallet{
		Name: "test",
		Addresses: map[wallet.ChainID][]wallet.Address{
			wallet.ChainBSV: {
				{Index: 0, Address: "1Addr0", Path: "m/44'/236'/0'/0/0"},
				{Index: 1, Address: "1Addr1", Path: "m/44'/236'/0'/0/1"},
				{Index: 2, Address: "1Addr2", Path: "m/44'/236'/0'/0/2"},
			},
		},
	}

	tests := []struct {
		name        string
		chainID     chain.ID
		address     string
		expectErr   bool
		expectIndex uint32
	}{
		{
			name:        "found first",
			chainID:     chain.BSV,
			address:     "1Addr0",
			expectErr:   false,
			expectIndex: 0,
		},
		{
			name:        "found last",
			chainID:     chain.BSV,
			address:     "1Addr2",
			expectErr:   false,
			expectIndex: 2,
		},
		{
			name:      "not found",
			chainID:   chain.BSV,
			address:   "1NotInWallet",
			expectErr: true,
		},
		{
			name:      "wrong chain",
			chainID:   chain.ETH,
			address:   "1Addr0",
			expectErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			result, err := findWalletAddress(wlt, tc.chainID, tc.address)
			if tc.expectErr {
				require.Error(t, err)
				assert.Nil(t, result)
			} else {
				require.NoError(t, err)
				require.NotNil(t, result)
				assert.Equal(t, tc.expectIndex, result.Index)
				assert.Equal(t, tc.address, result.Address)
			}
		})
	}
}

func TestFindWalletAddress_EmptyWallet(t *testing.T) {
	t.Parallel()

	wlt := &wallet.Wallet{
		Name:      "empty",
		Addresses: map[wallet.ChainID][]wallet.Address{},
	}

	result, err := findWalletAddress(wlt, chain.BSV, "1Anything")
	require.Error(t, err)
	assert.Nil(t, result)
}

func TestTruncateAddr(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		addr     string
		expected string
	}{
		{
			name:     "long address",
			addr:     "1A1zP1eP5QGefi2DMPTfTL5SLmv7DivfNa",
			expected: "1A1zP1eP...ivfNa",
		},
		{
			name:     "short address",
			addr:     "1Short",
			expected: "1Short",
		},
		{
			name:     "exactly 16 chars",
			addr:     "1234567890123456",
			expected: "1234567890123456",
		},
		{
			name:     "17 chars truncated",
			addr:     "12345678901234567",
			expected: "12345678...34567",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tc.expected, truncateAddr(tc.addr))
		})
	}
}

func TestDisplayReceiveCheckAllETHText(t *testing.T) {
	t.Parallel()

	t.Run("displays ETH balances", func(t *testing.T) {
		t.Parallel()
		results := []ethCheckResult{
			{
				Addr:       &wallet.Address{Index: 0, Address: "0x742d35Cc6634C0532925a3b844Bc454e4438f44e", Path: "m/44'/60'/0'/0/0"},
				ETHBalance: "1.500000000000000000",
			},
			{
				Addr:       &wallet.Address{Index: 1, Address: "0x593D8959A6C95Ad6Db52aA66315bBB93b01CD484", Path: "m/44'/60'/0'/0/1"},
				ETHBalance: "0.000000000000000000",
			},
		}

		var buf bytes.Buffer
		displayReceiveCheckAllETHText(&buf, results)

		result := buf.String()
		assert.Contains(t, result, "Checking 2 receiving address(es) on eth...")
		assert.Contains(t, result, "1.500000000000000000 ETH")
		assert.Contains(t, result, "0.000000000000000000 ETH")
		assert.Contains(t, result, "m/44'/60'/0'/0/0")
		assert.NotContains(t, result, "Errors:")
	})

	t.Run("displays errors", func(t *testing.T) {
		t.Parallel()
		results := []ethCheckResult{
			{
				Addr: &wallet.Address{Index: 0, Address: "0x742d35Cc6634C0532925a3b844Bc454e4438f44e", Path: "m/44'/60'/0'/0/0"},
				Err:  errNetworkTimeout,
			},
		}

		var buf bytes.Buffer
		displayReceiveCheckAllETHText(&buf, results)

		result := buf.String()
		assert.Contains(t, result, "ERROR: network timeout")
		assert.Contains(t, result, "Errors: 1 address(es) failed to check")
	})
}

func TestDisplayReceiveCheckAllETHJSON(t *testing.T) {
	t.Parallel()

	results := []ethCheckResult{
		{
			Addr:       &wallet.Address{Index: 0, Address: "0x742d35Cc6634C0532925a3b844Bc454e4438f44e", Path: "m/44'/60'/0'/0/0"},
			ETHBalance: "1.500000000000000000",
		},
		{
			Addr: &wallet.Address{Index: 1, Address: "0x593D8959A6C95Ad6Db52aA66315bBB93b01CD484", Path: "m/44'/60'/0'/0/1"},
			Err:  errNetworkTimeout,
		},
	}

	var buf bytes.Buffer
	displayReceiveCheckAllETHJSON(&buf, results)

	var parsed map[string]any
	require.NoError(t, json.Unmarshal(buf.Bytes(), &parsed))

	assert.Equal(t, "eth", parsed["chain"])
	assert.InDelta(t, 2.0, parsed["addresses_checked"], 0)

	addrs, ok := parsed["addresses"].([]any)
	require.True(t, ok)
	assert.Len(t, addrs, 2)

	// First address has balance
	first, ok := addrs[0].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "0x742d35Cc6634C0532925a3b844Bc454e4438f44e", first["address"])
	assert.Equal(t, "1.500000000000000000", first["balance"])
	assert.Equal(t, "ETH", first["symbol"])
	_, hasError := first["error"]
	assert.False(t, hasError)

	// Second address has error
	second, ok := addrs[1].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "network timeout", second["error"])
}

func TestFormatUTXOCount(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		count    int
		expected string
	}{
		{
			name:     "zero UTXOs",
			count:    0,
			expected: "-",
		},
		{
			name:     "one UTXO singular",
			count:    1,
			expected: "(1 UTXO)",
		},
		{
			name:     "five UTXOs plural",
			count:    5,
			expected: "(5 UTXOs)",
		},
		{
			name:     "large count",
			count:    100,
			expected: "(100 UTXOs)",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			result := formatUTXOCount(tc.count)
			assert.Equal(t, tc.expected, result)
		})
	}
}
