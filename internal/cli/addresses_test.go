package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mrz1836/sigil/internal/cache"
	"github.com/mrz1836/sigil/internal/chain"
	"github.com/mrz1836/sigil/internal/service/address"
	"github.com/mrz1836/sigil/internal/utxostore"
	"github.com/mrz1836/sigil/internal/wallet"
	sigilerr "github.com/mrz1836/sigil/pkg/errors"
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
			info := address.AddressInfo{HasActivity: tc.addrUsed}
			filtered := address.FilterUsage([]address.AddressInfo{info}, tc.flagUsed, tc.flagUnused)
			got := len(filtered) > 0
			assert.Equal(t, tc.wantIncluded, got)
		})
	}
}

func TestDisplayAddressesText(t *testing.T) {
	tests := []struct {
		name        string
		addresses   []address.AddressInfo
		contains    []string
		notContains []string
	}{
		{
			name:      "empty list",
			addresses: []address.AddressInfo{},
			contains:  []string{"No addresses found"},
		},
		{
			name: "single BSV address",
			addresses: []address.AddressInfo{
				{
					Type:        address.Receive,
					Index:       0,
					Address:     "1A1zP1eP5QGefi2DMPTfTL5SLmv7DivfNa",
					Path:        "m/44'/236'/0'/0/0",
					Label:       "",
					Balance:     "",
					HasActivity: false,
					ChainID:     chain.BSV,
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
			addresses: []address.AddressInfo{
				{
					Type:        address.Receive,
					Index:       0,
					Address:     "1TestAddr",
					Balance:     "1.5",
					HasActivity: true,
					ChainID:     chain.BSV,
				},
			},
			contains: []string{
				"1.5",
				"used",
			},
		},
		{
			name: "address with label",
			addresses: []address.AddressInfo{
				{
					Type:    address.Receive,
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
			addresses: []address.AddressInfo{
				{
					Type:    address.Receive,
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
			addresses: []address.AddressInfo{
				{
					Type:    address.Receive,
					Index:   0,
					Address: "0x742d35Cc6634C0532925a3b844Bc9e7595f8b2E0abc123",
					ChainID: chain.ETH,
				},
			},
			contains: []string{"..."},
		},
		{
			name: "multiple chains",
			addresses: []address.AddressInfo{
				{
					Type:    address.Receive,
					Index:   0,
					Address: "1BSVAddress",
					ChainID: chain.BSV,
				},
				{
					Type:    address.Receive,
					Index:   0,
					Address: "0xETHAddress",
					ChainID: chain.ETH,
				},
			},
			contains: []string{"[BSV]", "[ETH]"},
		},
		{
			name: "change address type",
			addresses: []address.AddressInfo{
				{
					Type:    address.Change,
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
		addresses []address.AddressInfo
		validate  func(t *testing.T, output string)
	}{
		{
			name:      "empty list",
			addresses: []address.AddressInfo{},
			validate: func(t *testing.T, output string) {
				assert.Contains(t, output, `"addresses": [`)
			},
		},
		{
			name: "single address",
			addresses: []address.AddressInfo{
				{
					Type:        address.Receive,
					Index:       0,
					Address:     "1TestAddress",
					Path:        "m/44'/236'/0'/0/0",
					Label:       "Test",
					Balance:     "0.00001",
					HasActivity: true,
					ChainID:     chain.BSV,
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
			addresses: []address.AddressInfo{
				{
					Type:        address.Receive,
					Index:       0,
					Address:     "1TestAddress",
					Path:        "m/44'/236'/0'/0/0",
					Balance:     "1.5",
					Unconfirmed: "0.5",
					HasActivity: true,
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
			addresses: []address.AddressInfo{
				{
					Type:    address.Receive,
					Index:   0,
					Address: "addr1",
					ChainID: chain.BSV,
				},
				{
					Type:    address.Receive,
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
	addresses := []address.AddressInfo{
		{
			Type:        address.Receive,
			Index:       5,
			Address:     "0x742d35Cc6634C0532925a3b844Bc9e7595f8b2E0",
			Path:        "m/44'/60'/0'/0/5",
			Label:       "Main",
			Balance:     "1.0",
			Unconfirmed: "0.5",
			HasActivity: true,
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
	addresses := []address.AddressInfo{
		{
			Type:    address.Receive,
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
	// Test that address.AddressInfo correctly holds all fields
	info := address.AddressInfo{
		Type:        address.Change,
		Index:       42,
		Address:     "1TestAddress123456789012345678901234",
		Path:        "m/44'/236'/0'/1/42",
		Label:       "My Change",
		Balance:     "5.0",
		Unconfirmed: "-0.5",
		HasActivity: true,
		ChainID:     chain.BSV,
	}

	assert.Equal(t, address.Change, info.Type)
	assert.Equal(t, uint32(42), info.Index)
	assert.Equal(t, "1TestAddress123456789012345678901234", info.Address)
	assert.Equal(t, "m/44'/236'/0'/1/42", info.Path)
	assert.Equal(t, "My Change", info.Label)
	assert.Equal(t, "5.0", info.Balance)
	assert.Equal(t, "-0.5", info.Unconfirmed)
	assert.True(t, info.HasActivity)
	assert.Equal(t, chain.BSV, info.ChainID)
}

func TestDisplayAddressesTextTableFormat(t *testing.T) {
	// Verify table formatting with expected column widths
	addresses := []address.AddressInfo{
		{
			Type:        address.Receive,
			Index:       0,
			Address:     "1A1zP1eP5QGefi2DMPTfTL5SLmv7DivfNa",
			Label:       "Genesis",
			Balance:     "50.0",
			HasActivity: true,
			ChainID:     chain.BSV,
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
	addresses := []address.AddressInfo{
		{
			Type:    address.Receive,
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
	addresses := []address.AddressInfo{
		{
			Type:    address.Receive,
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
		name         string
		addr         wallet.Address
		chainID      chain.ID
		wantActivity bool
		wantLabel    string
	}{
		{
			name:         "address with no store data",
			addr:         wallet.Address{Index: 0, Address: "1UnknownAddr", Path: "m/44'/236'/0'/0/0"},
			chainID:      chain.BSV,
			wantActivity: false,
			wantLabel:    "",
		},
		{
			name:         "address with activity",
			addr:         wallet.Address{Index: 1, Address: "1ActiveAddr", Path: "m/44'/236'/0'/0/1"},
			chainID:      chain.BSV,
			wantActivity: true,
			wantLabel:    "Active",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			// Use address service to build address info
			addressService := address.NewService(address.NewMetadataAdapter(store))
			wlt := &wallet.Wallet{
				Addresses: map[chain.ID][]wallet.Address{
					tc.chainID: {tc.addr},
				},
			}
			infos := addressService.Collect(&address.CollectionRequest{
				Wallet:      wlt,
				ChainFilter: tc.chainID,
				TypeFilter:  address.Receive,
			})
			require.Len(t, infos, 1)
			info := infos[0]

			assert.Equal(t, address.Receive, info.Type)
			assert.Equal(t, tc.addr.Index, info.Index)
			assert.Equal(t, tc.addr.Address, info.Address)
			assert.Equal(t, tc.chainID, info.ChainID)
			assert.Equal(t, tc.wantActivity, info.HasActivity)
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
		addresses []address.AddressInfo
		want      bool
	}{
		{
			name:      "empty list",
			addresses: []address.AddressInfo{},
			want:      false,
		},
		{
			name: "no unconfirmed",
			addresses: []address.AddressInfo{
				{Balance: "1.0"},
				{Balance: "2.0"},
			},
			want: false,
		},
		{
			name: "has unconfirmed",
			addresses: []address.AddressInfo{
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
	addresses := []address.AddressInfo{
		{
			Type:        address.Receive,
			Index:       0,
			Address:     "1TestAddr1",
			Balance:     "1.5",
			Unconfirmed: "0.5",
			HasActivity: true,
			ChainID:     chain.BSV,
		},
		{
			Type:        address.Receive,
			Index:       1,
			Address:     "1TestAddr2",
			Balance:     "2.0",
			HasActivity: true,
			ChainID:     chain.BSV,
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
	addresses := []address.AddressInfo{
		{
			Type:        address.Receive,
			Index:       0,
			Address:     "1TestAddr",
			Balance:     "1.5",
			HasActivity: true,
			ChainID:     chain.BSV,
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

// mockLogWriter implements LogWriter for testing.
type mockLogWriter struct {
	errorCalls []string
	debugCalls []string
}

func (m *mockLogWriter) Debug(format string, _ ...any) {
	m.debugCalls = append(m.debugCalls, format)
}

func (m *mockLogWriter) Error(format string, _ ...any) {
	m.errorCalls = append(m.errorCalls, format)
}

func (m *mockLogWriter) Close() error { return nil }

func TestLoadOrCreateBalanceCache(t *testing.T) {
	t.Parallel()

	t.Run("refresh=true returns fresh cache ignoring disk", func(t *testing.T) {
		t.Parallel()

		tmpDir := t.TempDir()
		cachePath := filepath.Join(tmpDir, "balances.json")
		storage := cache.NewFileStorage(cachePath)

		// Pre-populate cache on disk
		bc := cache.NewBalanceCache()
		bc.Set(cache.BalanceCacheEntry{
			Chain: chain.BSV, Address: "1abc", Balance: "5.0", Symbol: "BSV", Decimals: 8,
		})
		require.NoError(t, storage.Save(bc))

		cmd := &cobra.Command{}
		var errBuf bytes.Buffer
		cmd.SetErr(&errBuf)

		result := loadOrCreateBalanceCache(storage, true, cmd, nil)
		require.NotNil(t, result)
		assert.Equal(t, 0, result.Size(), "refresh should return empty cache")
	})

	t.Run("valid cache file loads from disk", func(t *testing.T) {
		t.Parallel()

		tmpDir := t.TempDir()
		cachePath := filepath.Join(tmpDir, "balances.json")
		storage := cache.NewFileStorage(cachePath)

		bc := cache.NewBalanceCache()
		bc.Set(cache.BalanceCacheEntry{
			Chain: chain.ETH, Address: "0x123", Balance: "2.0", Symbol: "ETH", Decimals: 18,
		})
		require.NoError(t, storage.Save(bc))

		cmd := &cobra.Command{}

		result := loadOrCreateBalanceCache(storage, false, cmd, nil)
		require.NotNil(t, result)
		assert.Equal(t, 1, result.Size())
		entry, exists, _ := result.Get(chain.ETH, "0x123", "")
		require.True(t, exists)
		assert.Equal(t, "2.0", entry.Balance)
	})

	t.Run("corrupt cache file with logger returns fresh cache and warns", func(t *testing.T) {
		t.Parallel()

		tmpDir := t.TempDir()
		cachePath := filepath.Join(tmpDir, "balances.json")

		// Write invalid JSON
		require.NoError(t, os.MkdirAll(filepath.Dir(cachePath), 0o750))
		require.NoError(t, os.WriteFile(cachePath, []byte("{invalid json!"), 0o600))

		storage := cache.NewFileStorage(cachePath)
		cmd := &cobra.Command{}
		var errBuf bytes.Buffer
		cmd.SetErr(&errBuf)

		logger := &mockLogWriter{}

		result := loadOrCreateBalanceCache(storage, false, cmd, logger)
		require.NotNil(t, result)
		assert.Equal(t, 0, result.Size(), "corrupt cache should return fresh cache")
		assert.Contains(t, errBuf.String(), "Warning")
		assert.NotEmpty(t, logger.errorCalls, "should log error for corrupt cache")
	})

	t.Run("corrupt cache file nil logger does not panic", func(t *testing.T) {
		t.Parallel()

		tmpDir := t.TempDir()
		cachePath := filepath.Join(tmpDir, "balances.json")

		require.NoError(t, os.MkdirAll(filepath.Dir(cachePath), 0o750))
		require.NoError(t, os.WriteFile(cachePath, []byte("{bad json"), 0o600))

		storage := cache.NewFileStorage(cachePath)
		cmd := &cobra.Command{}
		var errBuf bytes.Buffer
		cmd.SetErr(&errBuf)

		assert.NotPanics(t, func() {
			result := loadOrCreateBalanceCache(storage, false, cmd, nil)
			require.NotNil(t, result)
			assert.Equal(t, 0, result.Size())
		})
		assert.Contains(t, errBuf.String(), "Warning")
	})

	t.Run("missing cache file returns fresh cache", func(t *testing.T) {
		t.Parallel()

		tmpDir := t.TempDir()
		cachePath := filepath.Join(tmpDir, "nonexistent", "balances.json")
		storage := cache.NewFileStorage(cachePath)

		cmd := &cobra.Command{}
		logger := &mockLogWriter{}

		result := loadOrCreateBalanceCache(storage, false, cmd, logger)
		require.NotNil(t, result)
		assert.Equal(t, 0, result.Size())
	})
}

func TestRunAddressesLabel(t *testing.T) {
	// Tests modify the global addressesWallet variable so cannot be parallel at top level.
	origWallet := addressesWallet
	defer func() { addressesWallet = origWallet }()

	t.Run("set label", func(t *testing.T) {
		testHome := t.TempDir()
		walletName := "testwallet"
		walletDir := filepath.Join(testHome, "wallets", walletName)
		require.NoError(t, os.MkdirAll(walletDir, 0o750))

		// Create UTXO store with an address
		store := utxostore.New(walletDir)
		store.AddAddress(&utxostore.AddressMetadata{
			Address: "1TestLabelAddr",
			ChainID: chain.BSV,
		})
		require.NoError(t, store.Save())

		cmd := &cobra.Command{}
		cmd.SetContext(context.Background())
		SetCmdContext(cmd, &CommandContext{
			Cfg: &mockConfigProvider{home: testHome},
		})
		addressesWallet = walletName

		var buf bytes.Buffer
		cmd.SetOut(&buf)

		err := runAddressesLabel(cmd, []string{"1TestLabelAddr", "MyLabel"})
		require.NoError(t, err)
		assert.Contains(t, buf.String(), `Label set to "MyLabel"`)
		assert.Contains(t, buf.String(), "1TestLabelAddr")

		// Verify label was persisted
		store2 := utxostore.New(walletDir)
		require.NoError(t, store2.Load())
		meta := store2.GetAddress(chain.BSV, "1TestLabelAddr")
		require.NotNil(t, meta)
		assert.Equal(t, "MyLabel", meta.Label)
	})

	t.Run("clear label", func(t *testing.T) {
		testHome := t.TempDir()
		walletName := "testwallet2"
		walletDir := filepath.Join(testHome, "wallets", walletName)
		require.NoError(t, os.MkdirAll(walletDir, 0o750))

		store := utxostore.New(walletDir)
		store.AddAddress(&utxostore.AddressMetadata{
			Address: "1ClearLabelAddr",
			ChainID: chain.BSV,
			Label:   "OldLabel",
		})
		require.NoError(t, store.Save())

		cmd := &cobra.Command{}
		cmd.SetContext(context.Background())
		SetCmdContext(cmd, &CommandContext{
			Cfg: &mockConfigProvider{home: testHome},
		})
		addressesWallet = walletName

		var buf bytes.Buffer
		cmd.SetOut(&buf)

		err := runAddressesLabel(cmd, []string{"1ClearLabelAddr", ""})
		require.NoError(t, err)
		assert.Contains(t, buf.String(), "Label cleared")
	})

	t.Run("address not found", func(t *testing.T) {
		testHome := t.TempDir()
		walletName := "testwallet3"
		walletDir := filepath.Join(testHome, "wallets", walletName)
		require.NoError(t, os.MkdirAll(walletDir, 0o750))

		// Create an empty store (no addresses)
		store := utxostore.New(walletDir)
		require.NoError(t, store.Save())

		cmd := &cobra.Command{}
		cmd.SetContext(context.Background())
		SetCmdContext(cmd, &CommandContext{
			Cfg: &mockConfigProvider{home: testHome},
		})
		addressesWallet = walletName

		err := runAddressesLabel(cmd, []string{"1NonExistent", "Label"})
		require.Error(t, err)
		require.ErrorIs(t, err, sigilerr.ErrInvalidInput)
	})
}

func TestDisplayAddressesRefreshJSON(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		addresses  []address.AddressInfo
		errorCount int
		validate   func(t *testing.T, output string)
	}{
		{
			name:       "empty list with no errors",
			addresses:  []address.AddressInfo{},
			errorCount: 0,
			validate: func(t *testing.T, output string) {
				assert.Contains(t, output, `"refreshed": 0`)
				assert.Contains(t, output, `"errors": 0`)
				assert.Contains(t, output, `"addresses": [`)
			},
		},
		{
			name: "single address",
			addresses: []address.AddressInfo{
				{
					Type:        address.Receive,
					Index:       0,
					Address:     "1TestAddress",
					Path:        "m/44'/236'/0'/0/0",
					Label:       "Test",
					Balance:     "0.00001",
					HasActivity: true,
					ChainID:     chain.BSV,
				},
			},
			errorCount: 0,
			validate: func(t *testing.T, output string) {
				assert.Contains(t, output, `"refreshed": 1`)
				assert.Contains(t, output, `"errors": 0`)
				assert.Contains(t, output, `"chain": "bsv"`)
				assert.Contains(t, output, `"address": "1TestAddress"`)
				assert.Contains(t, output, `"balance": "0.00001"`)
			},
		},
		{
			name: "with errors",
			addresses: []address.AddressInfo{
				{
					Type:    address.Receive,
					Index:   0,
					Address: "1Addr",
					ChainID: chain.BSV,
				},
			},
			errorCount: 2,
			validate: func(t *testing.T, output string) {
				assert.Contains(t, output, `"refreshed": 1`)
				assert.Contains(t, output, `"errors": 2`)
			},
		},
		{
			name: "with unconfirmed balance",
			addresses: []address.AddressInfo{
				{
					Type:        address.Receive,
					Index:       0,
					Address:     "1Addr",
					Balance:     "1.5",
					Unconfirmed: "0.5",
					ChainID:     chain.BSV,
				},
			},
			errorCount: 0,
			validate: func(t *testing.T, output string) {
				assert.Contains(t, output, `"balance": "1.5"`)
				assert.Contains(t, output, `"unconfirmed": "0.5"`)
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			buf := new(bytes.Buffer)
			cmd := &cobra.Command{}
			cmd.SetOut(buf)

			displayAddressesRefreshJSON(cmd, tc.addresses, tc.errorCount)

			output := buf.String()
			tc.validate(t, output)

			// Verify it's valid JSON
			var result map[string]interface{}
			err := json.Unmarshal([]byte(output), &result)
			require.NoError(t, err, "output should be valid JSON: %s", output)

			// Verify structure
			require.Contains(t, result, "refreshed")
			require.Contains(t, result, "errors")
			require.Contains(t, result, "addresses")
		})
	}
}

func TestAddressesRefreshCmd_FlagRegistration(t *testing.T) {
	t.Parallel()

	// Verify the command is registered
	require.NotNil(t, addressesRefreshCmd)
	assert.Equal(t, "refresh", addressesRefreshCmd.Use)

	// Verify flags exist
	walletFlag := addressesRefreshCmd.Flags().Lookup("wallet")
	require.NotNil(t, walletFlag, "wallet flag should exist")
	assert.Equal(t, "w", walletFlag.Shorthand)

	chainFlag := addressesRefreshCmd.Flags().Lookup("chain")
	require.NotNil(t, chainFlag, "chain flag should exist")
	assert.Equal(t, "c", chainFlag.Shorthand)

	addressFlag := addressesRefreshCmd.Flags().Lookup("address")
	require.NotNil(t, addressFlag, "address flag should exist")

	// Verify wallet is required
	err := addressesRefreshCmd.ValidateRequiredFlags()
	require.Error(t, err, "wallet flag should be required")
}

func TestBuildRefreshTargets(t *testing.T) {
	t.Parallel()

	// Create a test wallet
	wlt := &wallet.Wallet{
		Name:          "test",
		EnabledChains: []chain.ID{chain.BSV, chain.ETH},
		Addresses: map[chain.ID][]wallet.Address{
			chain.BSV: {
				{Index: 0, Address: "1BSVAddr1", Path: "m/44'/236'/0'/0/0"},
				{Index: 1, Address: "1BSVAddr2", Path: "m/44'/236'/0'/0/1"},
			},
			chain.ETH: {
				{Index: 0, Address: "0xETHAddr1", Path: "m/44'/60'/0'/0/0"},
			},
		},
		ChangeAddresses: map[chain.ID][]wallet.Address{
			chain.BSV: {
				{Index: 0, Address: "1BSVChange1", Path: "m/44'/236'/0'/1/0"},
			},
		},
	}

	t.Run("all addresses", func(t *testing.T) {
		t.Parallel()

		targets, err := buildRefreshTargets(wlt, []chain.ID{chain.BSV, chain.ETH}, nil)
		require.NoError(t, err)
		assert.Len(t, targets, 4) // 2 BSV receive + 1 BSV change + 1 ETH
	})

	t.Run("specific chain", func(t *testing.T) {
		t.Parallel()

		targets, err := buildRefreshTargets(wlt, []chain.ID{chain.BSV}, nil)
		require.NoError(t, err)
		assert.Len(t, targets, 3) // 2 BSV receive + 1 BSV change
	})

	t.Run("specific valid address", func(t *testing.T) {
		t.Parallel()

		targets, err := buildRefreshTargets(wlt, []chain.ID{chain.BSV}, []string{"1BSVAddr1"})
		require.NoError(t, err)
		assert.Len(t, targets, 1)
		assert.Equal(t, "1BSVAddr1", targets[0].address)
		assert.Equal(t, chain.BSV, targets[0].chainID)
	})

	t.Run("specific change address", func(t *testing.T) {
		t.Parallel()

		targets, err := buildRefreshTargets(wlt, []chain.ID{chain.BSV}, []string{"1BSVChange1"})
		require.NoError(t, err)
		assert.Len(t, targets, 1)
		assert.Equal(t, "1BSVChange1", targets[0].address)
	})

	t.Run("invalid address", func(t *testing.T) {
		t.Parallel()

		targets, err := buildRefreshTargets(wlt, []chain.ID{chain.BSV}, []string{"1NonExistent"})
		require.Error(t, err)
		require.Nil(t, targets)
		require.ErrorIs(t, err, sigilerr.ErrInvalidInput)
	})

	t.Run("multiple addresses", func(t *testing.T) {
		t.Parallel()

		targets, err := buildRefreshTargets(wlt, []chain.ID{chain.BSV, chain.ETH}, []string{"1BSVAddr1", "0xETHAddr1"})
		require.NoError(t, err)
		assert.Len(t, targets, 2)
	})
}

func TestFindInAddresses(t *testing.T) {
	t.Parallel()

	addresses := []wallet.Address{
		{Index: 0, Address: "1Addr1"},
		{Index: 1, Address: "1Addr2"},
		{Index: 2, Address: "1Addr3"},
	}

	assert.True(t, findInAddresses(addresses, "1Addr1"))
	assert.True(t, findInAddresses(addresses, "1Addr2"))
	assert.True(t, findInAddresses(addresses, "1Addr3"))
	assert.False(t, findInAddresses(addresses, "1NotFound"))
	assert.False(t, findInAddresses(nil, "1Addr1"))
	assert.False(t, findInAddresses([]wallet.Address{}, "1Addr1"))
}
