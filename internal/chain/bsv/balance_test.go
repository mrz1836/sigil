package bsv

import (
	"context"
	"math/big"
	"testing"
	"time"

	whatsonchain "github.com/mrz1836/go-whatsonchain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mrz1836/sigil/internal/chain"
)

func TestGetNativeBalance(t *testing.T) {
	t.Parallel()

	t.Run("returns Balance struct for valid address", func(t *testing.T) {
		t.Parallel()

		mock := &mockWOCClient{
			balanceFunc: func(_ context.Context, _ string) (*whatsonchain.AddressBalance, error) {
				return &whatsonchain.AddressBalance{
					Confirmed:   100000000, // 1 BSV
					Unconfirmed: 0,
				}, nil
			},
		}

		client := NewClient(context.Background(), &ClientOptions{WOCClient: mock})

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		balance, err := client.GetNativeBalance(ctx, "1A1zP1eP5QGefi2DMPTfTL5SLmv7DivfNa")
		require.NoError(t, err)

		assert.Equal(t, "1A1zP1eP5QGefi2DMPTfTL5SLmv7DivfNa", balance.Address)
		assert.Equal(t, big.NewInt(100000000), balance.Amount)
		assert.Nil(t, balance.Unconfirmed)
		assert.Equal(t, "BSV", balance.Symbol)
		assert.Equal(t, 8, balance.Decimals)
	})

	t.Run("returns negative unconfirmed balance", func(t *testing.T) {
		t.Parallel()

		mock := &mockWOCClient{
			balanceFunc: func(_ context.Context, _ string) (*whatsonchain.AddressBalance, error) {
				return &whatsonchain.AddressBalance{
					Confirmed:   70422,  // 0.00070422 BSV
					Unconfirmed: -70422, // spending the full amount
				}, nil
			},
		}

		client := NewClient(context.Background(), &ClientOptions{WOCClient: mock})

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		balance, err := client.GetNativeBalance(ctx, "1A1zP1eP5QGefi2DMPTfTL5SLmv7DivfNa")
		require.NoError(t, err)

		assert.Equal(t, big.NewInt(70422), balance.Amount)
		assert.Equal(t, big.NewInt(-70422), balance.Unconfirmed)
	})

	t.Run("returns positive unconfirmed balance", func(t *testing.T) {
		t.Parallel()

		mock := &mockWOCClient{
			balanceFunc: func(_ context.Context, _ string) (*whatsonchain.AddressBalance, error) {
				return &whatsonchain.AddressBalance{
					Confirmed:   50000000, // 0.5 BSV
					Unconfirmed: 100000,   // receiving 0.001 BSV
				}, nil
			},
		}

		client := NewClient(context.Background(), &ClientOptions{WOCClient: mock})

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		balance, err := client.GetNativeBalance(ctx, "1A1zP1eP5QGefi2DMPTfTL5SLmv7DivfNa")
		require.NoError(t, err)

		assert.Equal(t, big.NewInt(50000000), balance.Amount)
		assert.Equal(t, big.NewInt(100000), balance.Unconfirmed)
	})

	t.Run("propagates error from GetBalance", func(t *testing.T) {
		t.Parallel()

		mock := &mockWOCClient{
			balanceFunc: func(_ context.Context, _ string) (*whatsonchain.AddressBalance, error) {
				return nil, errTestServerError
			},
		}

		client := NewClient(context.Background(), &ClientOptions{WOCClient: mock})

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		_, err := client.GetNativeBalance(ctx, "1A1zP1eP5QGefi2DMPTfTL5SLmv7DivfNa")
		require.Error(t, err)
	})

	t.Run("returns error for invalid address", func(t *testing.T) {
		t.Parallel()

		client := NewClient(context.Background(), nil)
		ctx := context.Background()

		_, err := client.GetNativeBalance(ctx, "invalid")
		require.Error(t, err)
	})

	// BSV-specific: single satoshi precision (no dust limit)
	t.Run("returns 1 satoshi balance", func(t *testing.T) {
		t.Parallel()

		mock := &mockWOCClient{
			balanceFunc: func(_ context.Context, _ string) (*whatsonchain.AddressBalance, error) {
				return &whatsonchain.AddressBalance{
					Confirmed:   1, // 1 satoshi
					Unconfirmed: 0,
				}, nil
			},
		}

		client := NewClient(context.Background(), &ClientOptions{WOCClient: mock})

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		balance, err := client.GetNativeBalance(ctx, "1A1zP1eP5QGefi2DMPTfTL5SLmv7DivfNa")
		require.NoError(t, err)

		assert.Equal(t, big.NewInt(1), balance.Amount)
	})

	t.Run("returns exact satoshi amount - 123 satoshis", func(t *testing.T) {
		t.Parallel()

		mock := &mockWOCClient{
			balanceFunc: func(_ context.Context, _ string) (*whatsonchain.AddressBalance, error) {
				return &whatsonchain.AddressBalance{
					Confirmed:   123,
					Unconfirmed: 0,
				}, nil
			},
		}

		client := NewClient(context.Background(), &ClientOptions{WOCClient: mock})

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		balance, err := client.GetNativeBalance(ctx, "1A1zP1eP5QGefi2DMPTfTL5SLmv7DivfNa")
		require.NoError(t, err)

		assert.Equal(t, big.NewInt(123), balance.Amount)
	})

	t.Run("returns zero balance", func(t *testing.T) {
		t.Parallel()

		mock := &mockWOCClient{
			balanceFunc: func(_ context.Context, _ string) (*whatsonchain.AddressBalance, error) {
				return &whatsonchain.AddressBalance{
					Confirmed:   0,
					Unconfirmed: 0,
				}, nil
			},
		}

		client := NewClient(context.Background(), &ClientOptions{WOCClient: mock})

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		balance, err := client.GetNativeBalance(ctx, "1A1zP1eP5QGefi2DMPTfTL5SLmv7DivfNa")
		require.NoError(t, err)

		assert.Equal(t, big.NewInt(0), balance.Amount)
	})

	t.Run("returns large balance - 21 million BSV", func(t *testing.T) {
		t.Parallel()

		mock := &mockWOCClient{
			balanceFunc: func(_ context.Context, _ string) (*whatsonchain.AddressBalance, error) {
				return &whatsonchain.AddressBalance{
					Confirmed:   2100000000000000, // 21 million BSV in satoshis
					Unconfirmed: 0,
				}, nil
			},
		}

		client := NewClient(context.Background(), &ClientOptions{WOCClient: mock})

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

		mock := &mockWOCClient{
			balanceFunc: func(_ context.Context, _ string) (*whatsonchain.AddressBalance, error) {
				return &whatsonchain.AddressBalance{
					Confirmed:   50000000, // 0.5 BSV
					Unconfirmed: 0,
				}, nil
			},
		}

		client := NewClient(context.Background(), &ClientOptions{WOCClient: mock})

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

		mock := &mockWOCClient{
			balanceFunc: func(_ context.Context, _ string) (*whatsonchain.AddressBalance, error) {
				return nil, errTestServiceUnavailable
			},
		}

		client := NewClient(context.Background(), &ClientOptions{WOCClient: mock})

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		_, err := client.GetAllBalances(ctx, "1A1zP1eP5QGefi2DMPTfTL5SLmv7DivfNa")
		require.Error(t, err)
	})
}

func TestFormatDecimalAmount(t *testing.T) {
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
			result := chain.FormatDecimalAmount(tt.amount, tt.decimals)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestFormatSignedDecimalAmount(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		amount   *big.Int
		decimals int
		expected string
	}{
		{
			name:     "nil returns zero",
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
		{
			name:     "positive delegates to chain.FormatDecimalAmount",
			amount:   big.NewInt(100000000),
			decimals: 8,
			expected: "1.0",
		},
		{
			name:     "negative 1 satoshi",
			amount:   big.NewInt(-1),
			decimals: 8,
			expected: "-0.00000001",
		},
		{
			name:     "negative 70422 satoshis",
			amount:   big.NewInt(-70422),
			decimals: 8,
			expected: "-0.00070422",
		},
		{
			name:     "negative 1 BSV",
			amount:   big.NewInt(-100000000),
			decimals: 8,
			expected: "-1.0",
		},
		{
			name:     "positive small amount",
			amount:   big.NewInt(100000),
			decimals: 8,
			expected: "0.001",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result := chain.FormatSignedDecimalAmount(tt.amount, tt.decimals)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestGetBulkNativeBalance(t *testing.T) {
	t.Parallel()

	t.Run("fetches balances for multiple addresses", func(t *testing.T) {
		t.Parallel()

		mock := &mockWOCClient{
			bulkConfirmedFunc: func(_ context.Context, _ *whatsonchain.AddressList) (whatsonchain.AddressBalances, error) {
				return whatsonchain.AddressBalances{
					{
						Address: "1ABC",
						Balance: &whatsonchain.AddressBalance{Confirmed: 100000000}, // 1 BSV
					},
					{
						Address: "1XYZ",
						Balance: &whatsonchain.AddressBalance{Confirmed: 50000000}, // 0.5 BSV
					},
				}, nil
			},
			bulkUnconfirmedFunc: func(_ context.Context, _ *whatsonchain.AddressList) (whatsonchain.AddressBalances, error) {
				return whatsonchain.AddressBalances{
					{
						Address: "1ABC",
						Balance: &whatsonchain.AddressBalance{Unconfirmed: 10000000}, // 0.1 BSV
					},
					{
						Address: "1XYZ",
						Balance: &whatsonchain.AddressBalance{Unconfirmed: 0},
					},
				}, nil
			},
		}

		client := NewClient(context.Background(), &ClientOptions{WOCClient: mock})

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		addresses := []string{"1ABC", "1XYZ"}
		results, err := client.GetBulkNativeBalance(ctx, addresses)
		require.NoError(t, err)

		// Check that both addresses were returned
		assert.Len(t, results, 2)

		// Check first address
		bal1, ok := results["1ABC"]
		require.True(t, ok, "expected result for 1ABC")
		assert.Equal(t, big.NewInt(100000000), bal1.Amount)
		assert.Equal(t, big.NewInt(10000000), bal1.Unconfirmed)
		assert.Equal(t, "BSV", bal1.Symbol)
		assert.Equal(t, 8, bal1.Decimals)

		// Check second address
		bal2, ok := results["1XYZ"]
		require.True(t, ok, "expected result for 1XYZ")
		assert.Equal(t, big.NewInt(50000000), bal2.Amount)
		assert.Nil(t, bal2.Unconfirmed) // Should be nil since unconfirmed is 0
	})

	t.Run("handles empty address list", func(t *testing.T) {
		t.Parallel()

		mock := &mockWOCClient{}
		client := NewClient(context.Background(), &ClientOptions{WOCClient: mock})

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		results, err := client.GetBulkNativeBalance(ctx, []string{})
		require.NoError(t, err)
		assert.Empty(t, results)
	})

	t.Run("skips addresses with nil balance for fallback", func(t *testing.T) {
		t.Parallel()

		mock := &mockWOCClient{
			bulkConfirmedFunc: func(_ context.Context, _ *whatsonchain.AddressList) (whatsonchain.AddressBalances, error) {
				return whatsonchain.AddressBalances{
					{
						Address: "1ABC",
						Balance: nil, // Nil balance
					},
				}, nil
			},
			bulkUnconfirmedFunc: func(_ context.Context, _ *whatsonchain.AddressList) (whatsonchain.AddressBalances, error) {
				return whatsonchain.AddressBalances{
					{
						Address: "1ABC",
						Balance: nil,
					},
				}, nil
			},
		}

		client := NewClient(context.Background(), &ClientOptions{WOCClient: mock})

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		results, err := client.GetBulkNativeBalance(ctx, []string{"1ABC"})
		require.NoError(t, err)

		// NEW EXPECTED BEHAVIOR: Address should NOT be in results
		// This allows fetcher.go fallback logic to kick in
		_, ok := results["1ABC"]
		require.False(t, ok, "Address with nil Balance should not be in results to trigger fallback")
	})

	t.Run("handles mixed nil and valid balances", func(t *testing.T) {
		t.Parallel()

		mock := &mockWOCClient{
			bulkConfirmedFunc: func(_ context.Context, _ *whatsonchain.AddressList) (whatsonchain.AddressBalances, error) {
				return whatsonchain.AddressBalances{
					{
						Address: "1ABC",
						Balance: &whatsonchain.AddressBalance{Confirmed: 100000000}, // Valid - 1 BSV
					},
					{
						Address: "1XYZ",
						Balance: nil, // Nil - should trigger fallback
					},
				}, nil
			},
			bulkUnconfirmedFunc: func(_ context.Context, _ *whatsonchain.AddressList) (whatsonchain.AddressBalances, error) {
				return whatsonchain.AddressBalances{
					{
						Address: "1ABC",
						Balance: &whatsonchain.AddressBalance{Unconfirmed: 0},
					},
					{
						Address: "1XYZ",
						Balance: nil,
					},
				}, nil
			},
		}

		client := NewClient(context.Background(), &ClientOptions{WOCClient: mock})

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		results, err := client.GetBulkNativeBalance(ctx, []string{"1ABC", "1XYZ"})
		require.NoError(t, err)

		// Valid address should be present
		bal1, ok := results["1ABC"]
		require.True(t, ok, "Address with valid Balance should be in results")
		assert.Equal(t, big.NewInt(100000000), bal1.Amount)
		assert.Equal(t, "BSV", bal1.Symbol)
		assert.Equal(t, 8, bal1.Decimals)

		// Nil balance address should be absent (triggers fallback in fetcher)
		_, ok = results["1XYZ"]
		require.False(t, ok, "Address with nil Balance should not be in bulk results")
	})

	t.Run("handles large batches (more than 20 addresses)", func(t *testing.T) {
		t.Parallel()

		// Create 30 test addresses (will be split into 2 batches of 20 and 10)
		addresses := make([]string, 30)
		for i := 0; i < 30; i++ {
			addresses[i] = string(rune('A' + i))
		}

		mock := &mockWOCClient{
			bulkConfirmedFunc: func(_ context.Context, list *whatsonchain.AddressList) (whatsonchain.AddressBalances, error) {
				results := make(whatsonchain.AddressBalances, len(list.Addresses))
				for i, addr := range list.Addresses {
					results[i] = &whatsonchain.AddressBalanceRecord{
						Address: addr,
						Balance: &whatsonchain.AddressBalance{Confirmed: int64((i + 1) * 1000000)},
					}
				}
				return results, nil
			},
			bulkUnconfirmedFunc: func(_ context.Context, list *whatsonchain.AddressList) (whatsonchain.AddressBalances, error) {
				results := make(whatsonchain.AddressBalances, len(list.Addresses))
				for i, addr := range list.Addresses {
					results[i] = &whatsonchain.AddressBalanceRecord{
						Address: addr,
						Balance: &whatsonchain.AddressBalance{Unconfirmed: 0},
					}
				}
				return results, nil
			},
		}

		client := NewClient(context.Background(), &ClientOptions{WOCClient: mock})

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		results, err := client.GetBulkNativeBalance(ctx, addresses)
		require.NoError(t, err)

		// All 30 addresses should be present
		assert.Len(t, results, 30)

		// Verify all addresses are present
		for _, addr := range addresses {
			_, ok := results[addr]
			assert.True(t, ok, "missing result for address %s", addr)
		}
	})
}
