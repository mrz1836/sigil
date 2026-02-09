package cli

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mrz1836/sigil/internal/chain"
	"github.com/mrz1836/sigil/internal/utxostore"
	"github.com/mrz1836/sigil/internal/wallet"
)

func TestFormatSatoshis(t *testing.T) {
	tests := []struct {
		name string
		sats uint64
		want string
	}{
		{
			name: "zero",
			sats: 0,
			want: "0 sat",
		},
		{
			name: "one satoshi",
			sats: 1,
			want: "1 sat",
		},
		{
			name: "small amount",
			sats: 1000,
			want: "1000 sat",
		},
		{
			name: "near threshold",
			sats: 99999999,
			want: "99999999 sat",
		},
		{
			name: "exactly 1 BSV",
			sats: 100000000,
			want: "1.0000",
		},
		{
			name: "1.5 BSV",
			sats: 150000000,
			want: "1.5000",
		},
		{
			name: "fractional BSV with rounding",
			sats: 123456789,
			want: "1.2346",
		},
		{
			name: "large amount",
			sats: 1234567890,
			want: "12.3457",
		},
		{
			name: "very large amount",
			sats: 2100000000000000,
			want: "21000000.0000",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := formatSatoshis(tc.sats)
			assert.Equal(t, tc.want, got)
		})
	}
}

func TestShouldIncludeAddress(t *testing.T) {
	// Save original flag values
	origUsed := addressesUsed
	origUnused := addressesUnused
	defer func() {
		addressesUsed = origUsed
		addressesUnused = origUnused
	}()

	tests := []struct {
		name         string
		addrUsed     bool
		flagUsed     bool
		flagUnused   bool
		wantIncluded bool
	}{
		{
			name:         "no filters - used address",
			addrUsed:     true,
			flagUsed:     false,
			flagUnused:   false,
			wantIncluded: true,
		},
		{
			name:         "no filters - unused address",
			addrUsed:     false,
			flagUsed:     false,
			flagUnused:   false,
			wantIncluded: true,
		},
		{
			name:         "used filter - used address",
			addrUsed:     true,
			flagUsed:     true,
			flagUnused:   false,
			wantIncluded: true,
		},
		{
			name:         "used filter - unused address",
			addrUsed:     false,
			flagUsed:     true,
			flagUnused:   false,
			wantIncluded: false,
		},
		{
			name:         "unused filter - used address",
			addrUsed:     true,
			flagUsed:     false,
			flagUnused:   true,
			wantIncluded: false,
		},
		{
			name:         "unused filter - unused address",
			addrUsed:     false,
			flagUsed:     false,
			flagUnused:   true,
			wantIncluded: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Set global flags
			addressesUsed = tc.flagUsed
			addressesUnused = tc.flagUnused

			info := addressInfo{Used: tc.addrUsed}
			got := shouldIncludeAddress(info)
			assert.Equal(t, tc.wantIncluded, got)
		})
	}
}

func TestDisplayAddressesText(t *testing.T) {
	tests := []struct {
		name        string
		addresses   []addressInfo
		contains    []string
		notContains []string
	}{
		{
			name:      "empty list",
			addresses: []addressInfo{},
			contains:  []string{"No addresses found"},
		},
		{
			name: "single BSV address",
			addresses: []addressInfo{
				{
					Type:    "receive",
					Index:   0,
					Address: "1A1zP1eP5QGefi2DMPTfTL5SLmv7DivfNa",
					Path:    "m/44'/236'/0'/0/0",
					Label:   "",
					Balance: "",
					Used:    false,
					ChainID: chain.BSV,
				},
			},
			contains: []string{
				"Addresses:",
				"[BSV]",
				"receive",
				"1A1zP1eP5QGefi2DMPTfTL5SLmv7DivfNa",
				"unused",
			},
		},
		{
			name: "address with balance",
			addresses: []addressInfo{
				{
					Type:    "receive",
					Index:   0,
					Address: "1TestAddr",
					Balance: "1.5",
					Used:    true,
					ChainID: chain.BSV,
				},
			},
			contains: []string{
				"1.5",
				"used",
			},
		},
		{
			name: "address with label",
			addresses: []addressInfo{
				{
					Type:    "receive",
					Index:   0,
					Address: "1TestAddr",
					Label:   "Savings",
					ChainID: chain.BSV,
				},
			},
			contains: []string{"Savings"},
		},
		{
			name: "label truncation",
			addresses: []addressInfo{
				{
					Type:    "receive",
					Index:   0,
					Address: "1TestAddr",
					Label:   "VeryLongLabelThatShouldBeTruncated",
					ChainID: chain.BSV,
				},
			},
			contains:    []string{"VeryLongLab..."},
			notContains: []string{"VeryLongLabelThatShouldBeTruncated"},
		},
		{
			name: "long address truncation",
			addresses: []addressInfo{
				{
					Type:    "receive",
					Index:   0,
					Address: "0x742d35Cc6634C0532925a3b844Bc9e7595f8b2E0abc123",
					ChainID: chain.ETH,
				},
			},
			contains: []string{"..."},
		},
		{
			name: "multiple chains",
			addresses: []addressInfo{
				{
					Type:    "receive",
					Index:   0,
					Address: "1BSVAddress",
					ChainID: chain.BSV,
				},
				{
					Type:    "receive",
					Index:   0,
					Address: "0xETHAddress",
					ChainID: chain.ETH,
				},
			},
			contains: []string{"[BSV]", "[ETH]"},
		},
		{
			name: "change address type",
			addresses: []addressInfo{
				{
					Type:    "change",
					Index:   3,
					Address: "1ChangeAddr",
					ChainID: chain.BSV,
				},
			},
			contains: []string{"change", "3"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			buf := new(bytes.Buffer)
			cmd := &cobra.Command{}
			cmd.SetOut(buf)

			displayAddressesText(cmd, tc.addresses)

			output := buf.String()
			for _, s := range tc.contains {
				assert.Contains(t, output, s, "output should contain %q", s)
			}
			for _, s := range tc.notContains {
				assert.NotContains(t, output, s, "output should not contain %q", s)
			}
		})
	}
}

func TestDisplayAddressesJSON(t *testing.T) {
	tests := []struct {
		name      string
		addresses []addressInfo
		validate  func(t *testing.T, output string)
	}{
		{
			name:      "empty list",
			addresses: []addressInfo{},
			validate: func(t *testing.T, output string) {
				assert.Contains(t, output, `"addresses": [`)
			},
		},
		{
			name: "single address",
			addresses: []addressInfo{
				{
					Type:    "receive",
					Index:   0,
					Address: "1TestAddress",
					Path:    "m/44'/236'/0'/0/0",
					Label:   "Test",
					Balance: "0.00001",
					Used:    true,
					ChainID: chain.BSV,
				},
			},
			validate: func(t *testing.T, output string) {
				assert.Contains(t, output, `"chain": "bsv"`)
				assert.Contains(t, output, `"type": "receive"`)
				assert.Contains(t, output, `"index": 0`)
				assert.Contains(t, output, `"address": "1TestAddress"`)
				assert.Contains(t, output, `"path": "m/44'/236'/0'/0/0"`)
				assert.Contains(t, output, `"label": "Test"`)
				assert.Contains(t, output, `"balance": "0.00001"`)
				assert.Contains(t, output, `"used": true`)
			},
		},
		{
			name: "address with unconfirmed",
			addresses: []addressInfo{
				{
					Type:        "receive",
					Index:       0,
					Address:     "1TestAddress",
					Path:        "m/44'/236'/0'/0/0",
					Balance:     "1.5",
					Unconfirmed: "0.5",
					Used:        true,
					ChainID:     chain.BSV,
				},
			},
			validate: func(t *testing.T, output string) {
				assert.Contains(t, output, `"balance": "1.5"`)
				assert.Contains(t, output, `"unconfirmed": "0.5"`)
			},
		},
		{
			name: "multiple addresses",
			addresses: []addressInfo{
				{
					Type:    "receive",
					Index:   0,
					Address: "addr1",
					ChainID: chain.BSV,
				},
				{
					Type:    "receive",
					Index:   1,
					Address: "addr2",
					ChainID: chain.BSV,
				},
			},
			validate: func(t *testing.T, output string) {
				// Count address entries
				count := strings.Count(output, `"address":`)
				assert.Equal(t, 2, count)
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			buf := new(bytes.Buffer)
			cmd := &cobra.Command{}
			cmd.SetOut(buf)

			displayAddressesJSON(cmd, tc.addresses)

			output := buf.String()
			tc.validate(t, output)

			// Verify it's valid JSON
			var result map[string]interface{}
			err := json.Unmarshal([]byte(output), &result)
			require.NoError(t, err, "output should be valid JSON: %s", output)
		})
	}
}

func TestDisplayAddressesJSONStructure(t *testing.T) {
	addresses := []addressInfo{
		{
			Type:        "receive",
			Index:       5,
			Address:     "0x742d35Cc6634C0532925a3b844Bc9e7595f8b2E0",
			Path:        "m/44'/60'/0'/0/5",
			Label:       "Main",
			Balance:     "1.0",
			Unconfirmed: "0.5",
			Used:        true,
			ChainID:     chain.ETH,
		},
	}

	buf := new(bytes.Buffer)
	cmd := &cobra.Command{}
	cmd.SetOut(buf)

	displayAddressesJSON(cmd, addresses)

	var result struct {
		Addresses []struct {
			Chain       string `json:"chain"`
			Type        string `json:"type"`
			Index       int    `json:"index"`
			Address     string `json:"address"`
			Path        string `json:"path"`
			Label       string `json:"label"`
			Balance     string `json:"balance"`
			Unconfirmed string `json:"unconfirmed"`
			Used        bool   `json:"used"`
		} `json:"addresses"`
	}

	err := json.Unmarshal(buf.Bytes(), &result)
	require.NoError(t, err)

	require.Len(t, result.Addresses, 1)
	addr := result.Addresses[0]
	assert.Equal(t, "eth", addr.Chain)
	assert.Equal(t, "receive", addr.Type)
	assert.Equal(t, 5, addr.Index)
	assert.Equal(t, "0x742d35Cc6634C0532925a3b844Bc9e7595f8b2E0", addr.Address)
	assert.Equal(t, "m/44'/60'/0'/0/5", addr.Path)
	assert.Equal(t, "Main", addr.Label)
	assert.Equal(t, "1.0", addr.Balance)
	assert.Equal(t, "0.5", addr.Unconfirmed)
	assert.True(t, addr.Used)
}

func TestDisplayAddressesJSON_Escaping(t *testing.T) {
	addresses := []addressInfo{
		{
			Type:    "receive",
			Index:   1,
			Address: "1Addr\"Test",
			Path:    "m/44'/236'/0'/0/1",
			Label:   "line1\nline2 \"quoted\" \u2713",
			ChainID: chain.BSV,
		},
	}

	buf := new(bytes.Buffer)
	cmd := &cobra.Command{}
	cmd.SetOut(buf)

	displayAddressesJSON(cmd, addresses)

	var parsed struct {
		Addresses []struct {
			Address string `json:"address"`
			Label   string `json:"label"`
		} `json:"addresses"`
	}
	err := json.Unmarshal(buf.Bytes(), &parsed)
	require.NoError(t, err)
	require.Len(t, parsed.Addresses, 1)
	assert.Equal(t, addresses[0].Address, parsed.Addresses[0].Address)
	assert.Equal(t, "line1\nline2 \"quoted\" \u2713", parsed.Addresses[0].Label)
}

func TestAddressInfoStruct(t *testing.T) {
	// Test that addressInfo correctly holds all fields
	info := addressInfo{
		Type:        "change",
		Index:       42,
		Address:     "1TestAddress123456789012345678901234",
		Path:        "m/44'/236'/0'/1/42",
		Label:       "My Change",
		Balance:     "5.0",
		Unconfirmed: "-0.5",
		Used:        true,
		ChainID:     chain.BSV,
	}

	assert.Equal(t, "change", info.Type)
	assert.Equal(t, uint32(42), info.Index)
	assert.Equal(t, "1TestAddress123456789012345678901234", info.Address)
	assert.Equal(t, "m/44'/236'/0'/1/42", info.Path)
	assert.Equal(t, "My Change", info.Label)
	assert.Equal(t, "5.0", info.Balance)
	assert.Equal(t, "-0.5", info.Unconfirmed)
	assert.True(t, info.Used)
	assert.Equal(t, chain.BSV, info.ChainID)
}

func TestDisplayAddressesTextTableFormat(t *testing.T) {
	// Verify table formatting with expected column widths
	addresses := []addressInfo{
		{
			Type:    "receive",
			Index:   0,
			Address: "1A1zP1eP5QGefi2DMPTfTL5SLmv7DivfNa",
			Label:   "Genesis",
			Balance: "50.0",
			Used:    true,
			ChainID: chain.BSV,
		},
	}

	buf := new(bytes.Buffer)
	cmd := &cobra.Command{}
	cmd.SetOut(buf)

	displayAddressesText(cmd, addresses)

	output := buf.String()

	// Verify header row exists
	assert.Contains(t, output, "Type")
	assert.Contains(t, output, "Index")
	assert.Contains(t, output, "Address")
	assert.Contains(t, output, "Label")
	assert.Contains(t, output, "Balance")
	assert.Contains(t, output, "Status")

	// Verify separator line with box-drawing characters
	assert.Contains(t, output, "───")

	// Verify balance value is shown
	assert.Contains(t, output, "50.0")
}

func TestDisplayAddressesTextEmptyLabel(t *testing.T) {
	// Empty label should display as "-"
	addresses := []addressInfo{
		{
			Type:    "receive",
			Index:   0,
			Address: "1TestAddr",
			Label:   "",
			ChainID: chain.BSV,
		},
	}

	buf := new(bytes.Buffer)
	cmd := &cobra.Command{}
	cmd.SetOut(buf)

	displayAddressesText(cmd, addresses)

	output := buf.String()
	// The dash should appear in the label column
	lines := strings.Split(output, "\n")
	for _, line := range lines {
		if strings.Contains(line, "1TestAddr") {
			// Find dash that's not part of separator
			assert.Contains(t, line, "-")
			break
		}
	}
}

func TestDisplayAddressesTextZeroBalance(t *testing.T) {
	// Empty balance should display as "-"
	addresses := []addressInfo{
		{
			Type:    "receive",
			Index:   0,
			Address: "1TestAddr",
			Balance: "",
			ChainID: chain.BSV,
		},
	}

	buf := new(bytes.Buffer)
	cmd := &cobra.Command{}
	cmd.SetOut(buf)

	displayAddressesText(cmd, addresses)

	output := buf.String()
	lines := strings.Split(output, "\n")
	for _, line := range lines {
		if strings.Contains(line, "1TestAddr") {
			// Balance column should show "-" for empty balance
			assert.Contains(t, line, "-")
			break
		}
	}
}

func TestBuildAddressInfo(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	store := utxostore.New(tmpDir)

	// Add an address with activity to the store
	store.AddAddress(&utxostore.AddressMetadata{
		Address:     "1ActiveAddr",
		ChainID:     chain.BSV,
		HasActivity: true,
		Label:       "Active",
	})

	tests := []struct {
		name      string
		addr      wallet.Address
		chainID   chain.ID
		wantUsed  bool
		wantLabel string
	}{
		{
			name:      "address with no store data",
			addr:      wallet.Address{Index: 0, Address: "1UnknownAddr", Path: "m/44'/236'/0'/0/0"},
			chainID:   chain.BSV,
			wantUsed:  false,
			wantLabel: "",
		},
		{
			name:      "address with activity",
			addr:      wallet.Address{Index: 1, Address: "1ActiveAddr", Path: "m/44'/236'/0'/0/1"},
			chainID:   chain.BSV,
			wantUsed:  true,
			wantLabel: "Active",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			info := buildAddressInfo("receive", &tc.addr, tc.chainID, store)
			assert.Equal(t, "receive", info.Type)
			assert.Equal(t, tc.addr.Index, info.Index)
			assert.Equal(t, tc.addr.Address, info.Address)
			assert.Equal(t, tc.chainID, info.ChainID)
			assert.Equal(t, tc.wantUsed, info.Used)
			assert.Empty(t, info.Balance, "balance should be empty before network fetch")
			assert.Equal(t, tc.wantLabel, info.Label)
		})
	}
}

func TestIsNonZeroBalance(t *testing.T) {
	tests := []struct {
		name string
		s    string
		want bool
	}{
		{"empty string", "", false},
		{"zero", "0", false},
		{"zero with decimal", "0.0", false},
		{"zero BSV format", "0.00000000", false},
		{"negative zero", "-0.0", false},
		{"positive amount", "1.5", true},
		{"small amount", "0.00000001", true},
		{"negative amount", "-0.5", true},
		{"integer", "100", true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := isNonZeroBalance(tc.s)
			assert.Equal(t, tc.want, got)
		})
	}
}

func TestHasUnconfirmedAddressData(t *testing.T) {
	tests := []struct {
		name      string
		addresses []addressInfo
		want      bool
	}{
		{
			name:      "empty list",
			addresses: []addressInfo{},
			want:      false,
		},
		{
			name: "no unconfirmed",
			addresses: []addressInfo{
				{Balance: "1.0"},
				{Balance: "2.0"},
			},
			want: false,
		},
		{
			name: "has unconfirmed",
			addresses: []addressInfo{
				{Balance: "1.0"},
				{Balance: "2.0", Unconfirmed: "0.5"},
			},
			want: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := hasUnconfirmedAddressData(tc.addresses)
			assert.Equal(t, tc.want, got)
		})
	}
}

func TestDisplayAddressesTextWithUnconfirmed(t *testing.T) {
	// When any address has unconfirmed data, wide table should show
	addresses := []addressInfo{
		{
			Type:        "receive",
			Index:       0,
			Address:     "1TestAddr1",
			Balance:     "1.5",
			Unconfirmed: "0.5",
			Used:        true,
			ChainID:     chain.BSV,
		},
		{
			Type:    "receive",
			Index:   1,
			Address: "1TestAddr2",
			Balance: "2.0",
			Used:    true,
			ChainID: chain.BSV,
		},
	}

	buf := new(bytes.Buffer)
	cmd := &cobra.Command{}
	cmd.SetOut(buf)

	displayAddressesText(cmd, addresses)

	output := buf.String()

	// Wide table should show Confirmed and Unconfirmed columns
	assert.Contains(t, output, "Confirmed")
	assert.Contains(t, output, "Unconfirmed")
	assert.NotContains(t, output, "  Balance  ") // Should NOT show single Balance column
	assert.Contains(t, output, "1.5")
	assert.Contains(t, output, "0.5")
	assert.Contains(t, output, "2.0")
}

func TestDisplayAddressesTextNoUnconfirmed(t *testing.T) {
	// When no address has unconfirmed data, narrow table should show
	addresses := []addressInfo{
		{
			Type:    "receive",
			Index:   0,
			Address: "1TestAddr",
			Balance: "1.5",
			Used:    true,
			ChainID: chain.BSV,
		},
	}

	buf := new(bytes.Buffer)
	cmd := &cobra.Command{}
	cmd.SetOut(buf)

	displayAddressesText(cmd, addresses)

	output := buf.String()

	// Narrow table should show Balance column, not Confirmed/Unconfirmed
	assert.Contains(t, output, "Balance")
	assert.NotContains(t, output, "Confirmed")
	assert.Contains(t, output, "1.5")
}

func TestFormatHelpers(t *testing.T) {
	// truncateAddressDisplay
	assert.Equal(t, "short", truncateAddressDisplay("short"))
	long := "0x742d35Cc6634C0532925a3b844Bc9e7595f8b2E0abc123"
	truncated := truncateAddressDisplay(long)
	assert.Contains(t, truncated, "...")
	assert.Less(t, len(truncated), len(long))

	// formatLabel
	assert.Equal(t, "-", formatLabel(""))
	assert.Equal(t, "Short", formatLabel("Short"))
	assert.Equal(t, "VeryLongLab...", formatLabel("VeryLongLabelThatIsTooLong"))

	// formatBalanceDisplay
	assert.Equal(t, "-", formatBalanceDisplay(""))
	assert.Equal(t, "1.5", formatBalanceDisplay("1.5"))

	// formatStatus
	assert.Equal(t, "used", formatStatus(true))
	assert.Equal(t, "unused", formatStatus(false))
}
