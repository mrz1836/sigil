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

func TestGetNativeBalance(t *testing.T) {
	t.Parallel()

	t.Run("returns Balance struct for valid address", func(t *testing.T) {
		t.Parallel()

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			resp := BalanceResponse{
				Confirmed:   100000000, // 1 BSV
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

		balance, err := client.GetNativeBalance(ctx, "1A1zP1eP5QGefi2DMPTfTL5SLmv7DivfNa")
		require.NoError(t, err)

		assert.Equal(t, "1A1zP1eP5QGefi2DMPTfTL5SLmv7DivfNa", balance.Address)
		assert.Equal(t, big.NewInt(100000000), balance.Amount)
		assert.Equal(t, "BSV", balance.Symbol)
		assert.Equal(t, 8, balance.Decimals)
	})

	t.Run("propagates error from GetBalance", func(t *testing.T) {
		t.Parallel()

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
		}))
		defer server.Close()

		client := NewClient(&ClientOptions{
			BaseURL: server.URL,
		})

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		_, err := client.GetNativeBalance(ctx, "1A1zP1eP5QGefi2DMPTfTL5SLmv7DivfNa")
		require.Error(t, err)
	})

	t.Run("returns error for invalid address", func(t *testing.T) {
		t.Parallel()

		client := NewClient(nil)
		ctx := context.Background()

		_, err := client.GetNativeBalance(ctx, "invalid")
		require.Error(t, err)
	})

	// BSV-specific: single satoshi precision (no dust limit)
	t.Run("returns 1 satoshi balance", func(t *testing.T) {
		t.Parallel()

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			resp := BalanceResponse{
				Confirmed:   1, // 1 satoshi
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

		balance, err := client.GetNativeBalance(ctx, "1A1zP1eP5QGefi2DMPTfTL5SLmv7DivfNa")
		require.NoError(t, err)

		assert.Equal(t, big.NewInt(1), balance.Amount)
	})

	t.Run("returns exact satoshi amount - 123 satoshis", func(t *testing.T) {
		t.Parallel()

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			resp := BalanceResponse{
				Confirmed:   123,
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

		balance, err := client.GetNativeBalance(ctx, "1A1zP1eP5QGefi2DMPTfTL5SLmv7DivfNa")
		require.NoError(t, err)

		assert.Equal(t, big.NewInt(123), balance.Amount)
	})

	t.Run("returns zero balance", func(t *testing.T) {
		t.Parallel()

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			resp := BalanceResponse{
				Confirmed:   0,
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

		balance, err := client.GetNativeBalance(ctx, "1A1zP1eP5QGefi2DMPTfTL5SLmv7DivfNa")
		require.NoError(t, err)

		assert.Equal(t, big.NewInt(0), balance.Amount)
	})

	t.Run("returns large balance - 21 million BSV", func(t *testing.T) {
		t.Parallel()

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			resp := BalanceResponse{
				Confirmed:   2100000000000000, // 21 million BSV in satoshis
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

		balance, err := client.GetNativeBalance(ctx, "1A1zP1eP5QGefi2DMPTfTL5SLmv7DivfNa")
		require.NoError(t, err)

		assert.Equal(t, big.NewInt(2100000000000000), balance.Amount)
	})
}

func TestGetAllBalances(t *testing.T) {
	t.Parallel()

	t.Run("returns slice with single balance", func(t *testing.T) {
		t.Parallel()

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			resp := BalanceResponse{
				Confirmed:   50000000, // 0.5 BSV
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

		balances, err := client.GetAllBalances(ctx, "1A1zP1eP5QGefi2DMPTfTL5SLmv7DivfNa")
		require.NoError(t, err)

		require.Len(t, balances, 1)
		assert.Equal(t, "BSV", balances[0].Symbol)
		assert.Equal(t, big.NewInt(50000000), balances[0].Amount)
	})

	t.Run("propagates error from GetNativeBalance", func(t *testing.T) {
		t.Parallel()

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusServiceUnavailable)
		}))
		defer server.Close()

		client := NewClient(&ClientOptions{
			BaseURL: server.URL,
		})

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		_, err := client.GetAllBalances(ctx, "1A1zP1eP5QGefi2DMPTfTL5SLmv7DivfNa")
		require.Error(t, err)
	})
}

func TestFormatBalanceAmount(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		amount   *big.Int
		decimals int
		expected string
	}{
		// Nil and zero cases
		{
			name:     "nil input returns zero",
			amount:   nil,
			decimals: 8,
			expected: "0",
		},
		{
			name:     "zero value",
			amount:   big.NewInt(0),
			decimals: 8,
			expected: "0.0",
		},

		// Single satoshi precision (no dust limit in BSV)
		{
			name:     "1 satoshi - minimum non-zero",
			amount:   big.NewInt(1),
			decimals: 8,
			expected: "0.00000001",
		},
		{
			name:     "2 satoshis",
			amount:   big.NewInt(2),
			decimals: 8,
			expected: "0.00000002",
		},
		{
			name:     "9 satoshis",
			amount:   big.NewInt(9),
			decimals: 8,
			expected: "0.00000009",
		},
		{
			name:     "10 satoshis",
			amount:   big.NewInt(10),
			decimals: 8,
			expected: "0.0000001",
		},
		{
			name:     "11 satoshis - mixed trailing digits",
			amount:   big.NewInt(11),
			decimals: 8,
			expected: "0.00000011",
		},
		{
			name:     "99 satoshis",
			amount:   big.NewInt(99),
			decimals: 8,
			expected: "0.00000099",
		},
		{
			name:     "100 satoshis",
			amount:   big.NewInt(100),
			decimals: 8,
			expected: "0.000001",
		},
		{
			name:     "101 satoshis",
			amount:   big.NewInt(101),
			decimals: 8,
			expected: "0.00000101",
		},
		{
			name:     "999 satoshis",
			amount:   big.NewInt(999),
			decimals: 8,
			expected: "0.00000999",
		},
		{
			name:     "1000 satoshis",
			amount:   big.NewInt(1000),
			decimals: 8,
			expected: "0.00001",
		},

		// Sub-satoshi decimal positions
		{
			name:     "10000 satoshis (0.0001 BSV)",
			amount:   big.NewInt(10000),
			decimals: 8,
			expected: "0.0001",
		},
		{
			name:     "100000 satoshis (0.001 BSV)",
			amount:   big.NewInt(100000),
			decimals: 8,
			expected: "0.001",
		},
		{
			name:     "1000000 satoshis (0.01 BSV)",
			amount:   big.NewInt(1000000),
			decimals: 8,
			expected: "0.01",
		},
		{
			name:     "10000000 satoshis (0.1 BSV)",
			amount:   big.NewInt(10000000),
			decimals: 8,
			expected: "0.1",
		},

		// Whole BSV values
		{
			name:     "one BSV",
			amount:   big.NewInt(100000000),
			decimals: 8,
			expected: "1.0",
		},
		{
			name:     "10 BSV",
			amount:   big.NewInt(1000000000),
			decimals: 8,
			expected: "10.0",
		},
		{
			name:     "100 BSV",
			amount:   big.NewInt(10000000000),
			decimals: 8,
			expected: "100.0",
		},
		{
			name:     "1000 BSV",
			amount:   big.NewInt(100000000000),
			decimals: 8,
			expected: "1000.0",
		},

		// Mixed whole and fractional
		{
			name:     "1.5 BSV",
			amount:   big.NewInt(150000000),
			decimals: 8,
			expected: "1.5",
		},
		{
			name:     "1.00000001 BSV - whole plus 1 satoshi",
			amount:   big.NewInt(100000001),
			decimals: 8,
			expected: "1.00000001",
		},
		{
			name:     "1.10000001 BSV - complex fractional",
			amount:   big.NewInt(110000001),
			decimals: 8,
			expected: "1.10000001",
		},
		{
			name:     "fractional with all digits used",
			amount:   big.NewInt(123456789),
			decimals: 8,
			expected: "1.23456789",
		},
		{
			name:     "12.34567891 BSV",
			amount:   big.NewInt(1234567891),
			decimals: 8,
			expected: "12.34567891",
		},

		// Large values
		{
			name:     "21 million BSV (max supply)",
			amount:   big.NewInt(2100000000000000),
			decimals: 8,
			expected: "21000000.0",
		},
		{
			name:     "max supply minus 1 satoshi",
			amount:   big.NewInt(2099999999999999),
			decimals: 8,
			expected: "20999999.99999999",
		},

		// Different decimal configurations
		{
			name:     "6 decimals - 1.5 units",
			amount:   big.NewInt(1500000),
			decimals: 6,
			expected: "1.5",
		},
		{
			name:     "6 decimals - smallest unit",
			amount:   big.NewInt(1),
			decimals: 6,
			expected: "0.000001",
		},
		{
			name:     "18 decimals - wei-like precision",
			amount:   big.NewInt(1),
			decimals: 18,
			expected: "0.000000000000000001",
		},
		{
			name:     "0 decimals - whole numbers only",
			amount:   big.NewInt(12345),
			decimals: 0,
			expected: "12345.",
		},

		// Edge cases with specific bit patterns
		{
			name:     "power of 2 - 256 satoshis",
			amount:   big.NewInt(256),
			decimals: 8,
			expected: "0.00000256",
		},
		{
			name:     "power of 2 - 65536 satoshis",
			amount:   big.NewInt(65536),
			decimals: 8,
			expected: "0.00065536",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result := FormatBalanceAmount(tt.amount, tt.decimals)
			assert.Equal(t, tt.expected, result)
		})
	}
}
