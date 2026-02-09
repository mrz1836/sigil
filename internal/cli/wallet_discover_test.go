package cli

import (
	"bytes"
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mrz1836/sigil/internal/discovery"
	"github.com/mrz1836/sigil/internal/wallet"
)

func TestTruncateString(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		input    string
		maxLen   int
		expected string
	}{
		// String shorter than maxLen - no truncation
		{
			name:     "short string unchanged",
			input:    "hello",
			maxLen:   10,
			expected: "hello",
		},
		{
			name:     "empty string unchanged",
			input:    "",
			maxLen:   10,
			expected: "",
		},
		{
			name:     "exact length unchanged",
			input:    "hello",
			maxLen:   5,
			expected: "hello",
		},

		// String longer than maxLen - truncated with ellipsis
		{
			name:     "long string truncated with ellipsis",
			input:    "hello world",
			maxLen:   8,
			expected: "hello...",
		},
		{
			name:     "long string truncated at 10",
			input:    "this is a very long string",
			maxLen:   10,
			expected: "this is...",
		},
		{
			name:     "address truncation typical use",
			input:    "0x742d35Cc6634C0532925a3b844Bc454e4438f44e",
			maxLen:   15,
			expected: "0x742d35Cc66...",
		},

		// Edge cases with small maxLen
		{
			name:     "maxLen 3 no ellipsis",
			input:    "hello",
			maxLen:   3,
			expected: "hel",
		},
		{
			name:     "maxLen 2 no ellipsis",
			input:    "hello",
			maxLen:   2,
			expected: "he",
		},
		{
			name:     "maxLen 1 no ellipsis",
			input:    "hello",
			maxLen:   1,
			expected: "h",
		},
		{
			name:     "maxLen 0 empty result",
			input:    "hello",
			maxLen:   0,
			expected: "",
		},

		// Edge case: maxLen 4 means only 1 char + "..."
		{
			name:     "maxLen 4 gives 1 char plus ellipsis",
			input:    "hello",
			maxLen:   4,
			expected: "h...",
		},

		// Unicode strings (bytes, not runes)
		{
			name:     "unicode string truncated by bytes",
			input:    "hello\xc2\xa9world", // hello©world
			maxLen:   8,
			expected: "hello...",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			result := truncateString(tc.input, tc.maxLen)
			assert.Equal(t, tc.expected, result)
		})
	}
}

func TestBuildDiscoverResponse(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		result *discovery.Result
		check  func(t *testing.T, resp DiscoverResponse)
	}{
		{
			name: "empty result",
			result: &discovery.Result{
				FoundAddresses:   map[string][]discovery.DiscoveredAddress{},
				SchemesScanned:   []string{"BSV Standard"},
				AddressesScanned: 20,
				Duration:         2 * time.Second,
			},
			check: func(t *testing.T, resp DiscoverResponse) {
				assert.Equal(t, uint64(0), resp.TotalBalance)
				assert.Equal(t, 0, resp.TotalUTXOs)
				assert.Empty(t, resp.Addresses)
				assert.Equal(t, []string{"BSV Standard"}, resp.SchemesScanned)
				assert.Equal(t, 20, resp.AddressesScanned)
				assert.Equal(t, int64(2000), resp.DurationMs)
				assert.False(t, resp.PassphraseUsed)
				assert.Nil(t, resp.Errors)
			},
		},
		{
			name: "with addresses and errors",
			result: &discovery.Result{
				FoundAddresses: map[string][]discovery.DiscoveredAddress{
					"BSV Standard": {
						{
							Address:    "1abc",
							Path:       "m/44'/236'/0'/0/0",
							SchemeName: "BSV Standard",
							Balance:    50000,
							UTXOCount:  2,
							IsChange:   false,
						},
					},
					"Bitcoin Legacy": {
						{
							Address:    "1def",
							Path:       "m/44'/0'/0'/1/3",
							SchemeName: "Bitcoin Legacy",
							Balance:    10000,
							UTXOCount:  1,
							IsChange:   true,
						},
					},
				},
				TotalBalance:     60000,
				TotalUTXOs:       3,
				SchemesScanned:   []string{"BSV Standard", "Bitcoin Legacy"},
				AddressesScanned: 40,
				Duration:         5 * time.Second,
				PassphraseUsed:   true,
				Errors:           []string{"timeout on HandCash Legacy"},
			},
			check: func(t *testing.T, resp DiscoverResponse) {
				assert.Equal(t, uint64(60000), resp.TotalBalance)
				assert.Equal(t, 3, resp.TotalUTXOs)
				assert.Len(t, resp.Addresses, 2)
				assert.True(t, resp.PassphraseUsed)
				assert.Equal(t, []string{"timeout on HandCash Legacy"}, resp.Errors)
				assert.Equal(t, int64(5000), resp.DurationMs)

				// Verify addresses are mapped correctly
				found := map[string]bool{}
				for _, addr := range resp.Addresses {
					found[addr.Address] = true
					if addr.Address == "1def" {
						assert.True(t, addr.IsChange)
						assert.Equal(t, "Bitcoin Legacy", addr.Scheme)
						assert.Equal(t, uint64(10000), addr.Balance)
					}
				}
				assert.True(t, found["1abc"])
				assert.True(t, found["1def"])
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			resp := buildDiscoverResponse(tc.result)
			tc.check(t, resp)
		})
	}
}

func TestOutputDiscoverJSON(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		response     DiscoverResponse
		wantContains []string
		wantMissing  []string
	}{
		{
			name: "empty no addresses",
			response: DiscoverResponse{
				TotalBalance:     0,
				TotalUTXOs:       0,
				SchemesScanned:   []string{"BSV Standard"},
				AddressesScanned: 20,
				DurationMs:       1500,
			},
			wantContains: []string{
				`"total_balance": 0`,
				`"total_utxos": 0`,
				`"addresses_scanned": 20`,
				`"BSV Standard"`,
				`"addresses": [`,
			},
			wantMissing: []string{`"errors"`, `"migration"`},
		},
		{
			name: "with addresses",
			response: DiscoverResponse{
				TotalBalance:     50000,
				TotalUTXOs:       2,
				SchemesScanned:   []string{"BSV Standard"},
				AddressesScanned: 10,
				DurationMs:       3000,
				Addresses: []DiscoverAddressResponse{
					{
						Scheme:    "BSV Standard",
						Address:   "1abc",
						Path:      "m/44'/236'/0'/0/0",
						Balance:   50000,
						UTXOCount: 2,
					},
				},
			},
			wantContains: []string{
				`"scheme": "BSV Standard"`,
				`"address": "1abc"`,
				`"path": "m/44'/236'/0'/0/0"`,
				`"balance": 50000`,
				`"utxo_count": 2`,
			},
		},
		{
			name: "with errors",
			response: DiscoverResponse{
				SchemesScanned: []string{},
				Errors:         []string{"scan timeout", "api error"},
			},
			wantContains: []string{
				`"errors": [`,
				`"scan timeout"`,
				`"api error"`,
			},
		},
		{
			name: "with migration",
			response: DiscoverResponse{
				SchemesScanned: []string{},
				Migration: &DiscoverMigrationResponse{
					Destination:  "1dest",
					TotalInput:   100000,
					EstimatedFee: 500,
					NetAmount:    99500,
				},
			},
			wantContains: []string{
				`"migration": {`,
				`"destination": "1dest"`,
				`"total_input": 100000`,
				`"estimated_fee": 500`,
				`"net_amount": 99500`,
			},
		},
		{
			name: "with passphrase_used",
			response: DiscoverResponse{
				SchemesScanned: []string{},
				PassphraseUsed: true,
			},
			wantContains: []string{
				`"passphrase_used": true`,
			},
		},
		{
			name: "with migration and tx_id",
			response: DiscoverResponse{
				SchemesScanned: []string{},
				Migration: &DiscoverMigrationResponse{
					Destination:  "1dest",
					TotalInput:   200000,
					EstimatedFee: 1000,
					NetAmount:    199000,
					TxID:         "txid123abc",
				},
			},
			wantContains: []string{
				`"tx_id": "txid123abc"`,
			},
		},
		{
			name: "with change address",
			response: DiscoverResponse{
				SchemesScanned: []string{},
				Addresses: []DiscoverAddressResponse{
					{
						Scheme:    "BSV Standard",
						Address:   "1change",
						Path:      "m/44'/236'/0'/1/0",
						Balance:   1000,
						UTXOCount: 1,
						IsChange:  true,
					},
				},
			},
			wantContains: []string{
				`"is_change": true`,
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			var buf bytes.Buffer
			outputDiscoverJSON(&buf, tc.response)
			out := buf.String()
			var parsed map[string]any
			require.NoError(t, json.Unmarshal(buf.Bytes(), &parsed))
			for _, s := range tc.wantContains {
				assert.Contains(t, out, s)
			}
			for _, s := range tc.wantMissing {
				assert.NotContains(t, out, s)
			}
		})
	}
}

func TestOutputDiscoverJSON_NilSlicesNormalized(t *testing.T) {
	t.Parallel()

	response := DiscoverResponse{
		SchemesScanned: nil,
		Addresses:      nil,
	}

	var buf bytes.Buffer
	outputDiscoverJSON(&buf, response)

	var parsed map[string]any
	require.NoError(t, json.Unmarshal(buf.Bytes(), &parsed))

	// nil slices should be normalized to empty arrays, not null
	schemes, ok := parsed["schemes_scanned"].([]any)
	require.True(t, ok, "schemes_scanned should be an array, not null")
	assert.Empty(t, schemes)

	addrs, ok := parsed["addresses"].([]any)
	require.True(t, ok, "addresses should be an array, not null")
	assert.Empty(t, addrs)
}

func TestOutputDiscoverJSON_Escaping(t *testing.T) {
	t.Parallel()

	response := DiscoverResponse{
		SchemesScanned: []string{"Scheme \"One\""},
		Errors:         []string{"line1\nline2 \u2713"},
		Addresses: []DiscoverAddressResponse{
			{
				Scheme:    "Scheme \"One\"",
				Address:   "1Addr\"Test",
				Path:      "m/44'/236'/0'/0/0",
				Balance:   1,
				UTXOCount: 1,
			},
		},
	}

	var buf bytes.Buffer
	outputDiscoverJSON(&buf, response)

	var parsed DiscoverResponse
	require.NoError(t, json.Unmarshal(buf.Bytes(), &parsed))
	require.Len(t, parsed.Addresses, 1)
	assert.Equal(t, response.Addresses[0].Address, parsed.Addresses[0].Address)
	assert.Equal(t, response.Errors[0], parsed.Errors[0])
}

func TestOutputDiscoverText(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		response      DiscoverResponse
		showMigration bool
		wantContains  []string
		wantMissing   []string
	}{
		{
			name: "empty no funds",
			response: DiscoverResponse{
				Addresses: nil,
			},
			showMigration: false,
			wantContains: []string{
				"No funds discovered",
				"Check if you entered the correct mnemonic",
				"--passphrase",
				"--gap 50",
			},
		},
		{
			name: "with addresses shows table and totals",
			response: DiscoverResponse{
				TotalBalance: 50000,
				TotalUTXOs:   2,
				DurationMs:   3000,
				Addresses: []DiscoverAddressResponse{
					{
						Scheme:    "BSV Standard",
						Address:   "1abc",
						Path:      "m/44'/236'/0'/0/0",
						Balance:   50000,
						UTXOCount: 2,
					},
				},
			},
			showMigration: false,
			wantContains: []string{
				"DISCOVERED FUNDS",
				"BSV Standard",
				"50000 sat",
				"Total:",
				"Use --migrate --wallet",
			},
		},
		{
			name: "with errors shows warnings",
			response: DiscoverResponse{
				TotalBalance: 10000,
				TotalUTXOs:   1,
				DurationMs:   2000,
				Addresses: []DiscoverAddressResponse{
					{
						Scheme:    "BSV Standard",
						Address:   "1abc",
						Path:      "m/44'/236'/0'/0/0",
						Balance:   10000,
						UTXOCount: 1,
					},
				},
				Errors: []string{"timeout on scheme X"},
			},
			showMigration: false,
			wantContains: []string{
				"Warnings:",
				"timeout on scheme X",
			},
		},
		{
			name: "showMigration true hides migrate hint",
			response: DiscoverResponse{
				TotalBalance: 10000,
				TotalUTXOs:   1,
				DurationMs:   1000,
				Addresses: []DiscoverAddressResponse{
					{
						Scheme:    "BSV Standard",
						Address:   "1abc",
						Path:      "m/44'/236'/0'/0/0",
						Balance:   10000,
						UTXOCount: 1,
					},
				},
			},
			showMigration: true,
			wantMissing: []string{
				"Use --migrate",
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			var buf bytes.Buffer
			outputDiscoverText(&buf, tc.response, tc.showMigration)
			out := buf.String()
			for _, s := range tc.wantContains {
				assert.Contains(t, out, s)
			}
			for _, s := range tc.wantMissing {
				assert.NotContains(t, out, s)
			}
		})
	}
}

func TestCreateProgressCallback(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	cb := createProgressCallback(&buf)
	require.NotNil(t, cb)

	// First update: scanning a new scheme
	cb(discovery.ProgressUpdate{
		Phase:      "scanning",
		SchemeName: "BSV Standard",
	})
	assert.Contains(t, buf.String(), "BSV Standard")

	// Found an address
	cb(discovery.ProgressUpdate{
		Phase:          "found",
		SchemeName:     "BSV Standard",
		CurrentAddress: "1abc123",
		BalanceFound:   50000,
	})
	assert.Contains(t, buf.String(), "1abc123")
	assert.Contains(t, buf.String(), "50000")

	// Switch to new scheme
	buf.Reset()
	cb(discovery.ProgressUpdate{
		Phase:      "scanning",
		SchemeName: "Bitcoin Legacy",
	})
	assert.Contains(t, buf.String(), "Bitcoin Legacy")
}

func TestWalletKeyDeriver_DeriveAddress(t *testing.T) {
	t.Parallel()

	seed, err := wallet.MnemonicToSeed(
		"abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon about",
		"",
	)
	require.NoError(t, err)
	defer wallet.ZeroBytes(seed)

	deriver := &walletKeyDeriver{}

	// BSV Standard coin type = 236
	addr, path, err := deriver.DeriveAddress(seed, 236, 0, 0, 0)
	require.NoError(t, err)
	assert.NotEmpty(t, addr)
	assert.Contains(t, path, "m/44'/236'/0'/0/0")

	// Derive a second address at index 1
	addr2, path2, err := deriver.DeriveAddress(seed, 236, 0, 0, 1)
	require.NoError(t, err)
	assert.NotEmpty(t, addr2)
	assert.NotEqual(t, addr, addr2, "different indices should produce different addresses")
	assert.Contains(t, path2, "m/44'/236'/0'/0/1")
}

func TestWalletKeyDeriver_DeriveLegacyAddress(t *testing.T) {
	t.Parallel()

	seed, err := wallet.MnemonicToSeed(
		"abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon about",
		"",
	)
	require.NoError(t, err)
	defer wallet.ZeroBytes(seed)

	deriver := &walletKeyDeriver{}

	addr, path, err := deriver.DeriveLegacyAddress(seed, 0)
	require.NoError(t, err)
	assert.NotEmpty(t, addr)
	assert.NotEmpty(t, path)

	// Derive at index 1 should be different
	addr2, path2, err := deriver.DeriveLegacyAddress(seed, 1)
	require.NoError(t, err)
	assert.NotEmpty(t, addr2)
	assert.NotEqual(t, addr, addr2, "different indices should produce different addresses")
	assert.NotEqual(t, path, path2)
}
