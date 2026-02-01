package bsv

import (
	"context"
	"encoding/json"
	"math/big"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestNewClient tests client creation.
func TestNewClient(t *testing.T) {
	t.Run("creates client with defaults", func(t *testing.T) {
		client := NewClient(nil)
		assert.NotNil(t, client)
	})

	t.Run("creates client with custom options", func(t *testing.T) {
		client := NewClient(&ClientOptions{
			APIKey:  "test-key",
			Network: NetworkMainnet,
		})
		assert.NotNil(t, client)
	})
}

// TestGetBalance tests BSV balance queries.
func TestGetBalance(t *testing.T) {
	t.Run("returns balance for valid address", func(t *testing.T) {
		// Mock WhatsOnChain API
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Contains(t, r.URL.Path, "/address/")
			assert.Contains(t, r.URL.Path, "/balance")

			// Return balance in satoshis
			resp := BalanceResponse{
				Confirmed:   123456789,
				Unconfirmed: 0,
			}
			err := json.NewEncoder(w).Encode(resp)
			assert.NoError(t, err)
		}))
		defer server.Close()

		client := NewClient(&ClientOptions{
			BaseURL: server.URL,
		})

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		balance, err := client.GetBalance(ctx, "1A1zP1eP5QGefi2DMPTfTL5SLmv7DivfNa")
		require.NoError(t, err)

		expected := big.NewInt(123456789)
		assert.Equal(t, expected, balance)
	})

	t.Run("returns error for invalid address", func(t *testing.T) {
		client := NewClient(nil)

		ctx := context.Background()
		_, err := client.GetBalance(ctx, "invalid")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid address")
	})
}

// TestGetTokenBalance tests token balance (not supported for BSV).
func TestGetTokenBalance(t *testing.T) {
	client := NewClient(nil)

	ctx := context.Background()
	_, err := client.GetTokenBalance(ctx, "1A1zP1eP5QGefi2DMPTfTL5SLmv7DivfNa", "token")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not supported")
}

// TestListUTXOs tests UTXO listing.
func TestListUTXOs(t *testing.T) {
	t.Run("returns UTXOs for address", func(t *testing.T) {
		// Mock WhatsOnChain API
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Contains(t, r.URL.Path, "/address/")
			assert.Contains(t, r.URL.Path, "/unspent")

			resp := []UTXOResponse{
				{
					TxID:   "abc123def456",
					Vout:   0,
					Value:  50000000,
					Height: 100,
				},
				{
					TxID:   "def456abc789",
					Vout:   1,
					Value:  25000000,
					Height: 101,
				},
			}
			err := json.NewEncoder(w).Encode(resp)
			assert.NoError(t, err)
		}))
		defer server.Close()

		client := NewClient(&ClientOptions{
			BaseURL: server.URL,
		})

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		utxos, err := client.ListUTXOs(ctx, "1A1zP1eP5QGefi2DMPTfTL5SLmv7DivfNa")
		require.NoError(t, err)
		assert.Len(t, utxos, 2)

		assert.Equal(t, "abc123def456", utxos[0].TxID)
		assert.Equal(t, uint64(50000000), utxos[0].Amount)
		assert.Equal(t, "def456abc789", utxos[1].TxID)
	})
}

// TestValidateAddress tests address validation.
func TestValidateAddress(t *testing.T) {
	tests := []struct {
		name    string
		address string
		wantErr bool
	}{
		{
			name:    "valid P2PKH address starting with 1",
			address: "1A1zP1eP5QGefi2DMPTfTL5SLmv7DivfNa",
			wantErr: false,
		},
		{
			name:    "valid P2SH address starting with 3",
			address: "3J98t1WpEZ73CNmQviecrnyiWrnqRhWNLy",
			wantErr: false,
		},
		{
			name:    "invalid - too short",
			address: "1A1zP1",
			wantErr: true,
		},
		{
			name:    "invalid - contains 0 (not in Base58)",
			address: "1A0zP1eP5QGefi2DMPTfTL5SLmv7DivfNa",
			wantErr: true,
		},
		{
			name:    "invalid - contains O (not in Base58)",
			address: "1AOzP1eP5QGefi2DMPTfTL5SLmv7DivfNa",
			wantErr: true,
		},
		{
			name:    "invalid - contains I (not in Base58)",
			address: "1AIzP1eP5QGefi2DMPTfTL5SLmv7DivfNa",
			wantErr: true,
		},
		{
			name:    "invalid - contains l (not in Base58)",
			address: "1AlzP1eP5QGefi2DMPTfTL5SLmv7DivfNa",
			wantErr: true,
		},
		{
			name:    "empty address",
			address: "",
			wantErr: true,
		},
	}

	client := NewClient(nil)
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := client.ValidateAddress(tt.address)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// TestFormatAmount tests amount formatting.
func TestFormatAmount(t *testing.T) {
	client := NewClient(nil)

	tests := []struct {
		name     string
		amount   *big.Int
		expected string
	}{
		{
			name:     "1 BSV",
			amount:   big.NewInt(100000000),
			expected: "1.00000000",
		},
		{
			name:     "0.5 BSV",
			amount:   big.NewInt(50000000),
			expected: "0.50000000",
		},
		{
			name:     "1 satoshi",
			amount:   big.NewInt(1),
			expected: "0.00000001",
		},
		{
			name:     "zero",
			amount:   big.NewInt(0),
			expected: "0.00000000",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := client.FormatAmount(tt.amount)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// TestParseAmount tests amount parsing.
func TestParseAmount(t *testing.T) {
	client := NewClient(nil)

	tests := []struct {
		name     string
		input    string
		expected *big.Int
		wantErr  bool
	}{
		{
			name:     "1 BSV",
			input:    "1",
			expected: big.NewInt(100000000),
			wantErr:  false,
		},
		{
			name:     "0.5 BSV",
			input:    "0.5",
			expected: big.NewInt(50000000),
			wantErr:  false,
		},
		{
			name:     "0.00000001 BSV (1 satoshi)",
			input:    "0.00000001",
			expected: big.NewInt(1),
			wantErr:  false,
		},
		{
			name:    "invalid - not a number",
			input:   "abc",
			wantErr: true,
		},
		{
			name:    "invalid - negative",
			input:   "-1",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := client.ParseAmount(tt.input)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.expected, result)
			}
		})
	}
}

// TestSelectUTXOs tests UTXO selection.
func TestSelectUTXOs(t *testing.T) {
	client := NewClient(nil)

	utxos := []UTXO{
		{TxID: "tx1", Vout: 0, Amount: 50000000},  // 0.5 BSV
		{TxID: "tx2", Vout: 0, Amount: 30000000},  // 0.3 BSV
		{TxID: "tx3", Vout: 0, Amount: 100000000}, // 1.0 BSV
	}

	t.Run("selects sufficient UTXOs", func(t *testing.T) {
		selected, change, err := client.SelectUTXOs(utxos, 40000000, 1) // 0.4 BSV
		require.NoError(t, err)
		assert.NotEmpty(t, selected)

		// Total selected should cover amount + fee
		var total uint64
		for _, u := range selected {
			total += u.Amount
		}
		assert.Greater(t, total, uint64(40000000))
		_ = change // Change is calculated but we don't assert a specific value
	})

	t.Run("returns error for insufficient funds", func(t *testing.T) {
		_, _, err := client.SelectUTXOs(utxos, 200000000, 1) // 2 BSV - more than available
		require.Error(t, err)
		assert.Contains(t, err.Error(), "insufficient")
	})

	t.Run("handles empty UTXO list", func(t *testing.T) {
		_, _, err := client.SelectUTXOs([]UTXO{}, 10000, 1)
		require.Error(t, err)
	})
}
