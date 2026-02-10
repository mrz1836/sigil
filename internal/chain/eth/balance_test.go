package eth

import (
	"math/big"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/mrz1836/sigil/internal/chain"
)

func TestFormatDecimalAmount(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		amount   *big.Int
		decimals int
		expected string
	}{
		{
			name:     "nil amount returns 0",
			amount:   nil,
			decimals: 18,
			expected: "0",
		},
		{
			name:     "zero ETH (18 decimals)",
			amount:   big.NewInt(0),
			decimals: 18,
			expected: "0.0",
		},
		{
			name:     "1 ETH (18 decimals)",
			amount:   big.NewInt(1_000_000_000_000_000_000),
			decimals: 18,
			expected: "1.0",
		},
		{
			name:     "0.5 ETH (18 decimals)",
			amount:   big.NewInt(500_000_000_000_000_000),
			decimals: 18,
			expected: "0.5",
		},
		{
			name:     "1.234 ETH (18 decimals)",
			amount:   big.NewInt(1_234_000_000_000_000_000),
			decimals: 18,
			expected: "1.234",
		},
		{
			name:     "tiny ETH amount (1 wei)",
			amount:   big.NewInt(1),
			decimals: 18,
			expected: "0.000000000000000001",
		},
		{
			name:     "zero USDC (6 decimals)",
			amount:   big.NewInt(0),
			decimals: 6,
			expected: "0.0",
		},
		{
			name:     "1 USDC (6 decimals)",
			amount:   big.NewInt(1_000_000),
			decimals: 6,
			expected: "1.0",
		},
		{
			name:     "500 USDC (6 decimals)",
			amount:   big.NewInt(500_000_000),
			decimals: 6,
			expected: "500.0",
		},
		{
			name:     "0.01 USDC (6 decimals)",
			amount:   big.NewInt(10_000),
			decimals: 6,
			expected: "0.01",
		},
		{
			name:     "tiny USDC amount (1 base unit)",
			amount:   big.NewInt(1),
			decimals: 6,
			expected: "0.000001",
		},
		{
			name:     "large ETH amount",
			amount:   new(big.Int).Mul(big.NewInt(1000), big.NewInt(1_000_000_000_000_000_000)),
			decimals: 18,
			expected: "1000.0",
		},
		{
			name:     "8 decimals token",
			amount:   big.NewInt(100_000_000),
			decimals: 8,
			expected: "1.0",
		},
		{
			name:     "0 decimals token",
			amount:   big.NewInt(42),
			decimals: 0,
			expected: "42.",
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
			decimals: 18,
			expected: "0",
		},
		{
			name:     "zero value",
			amount:   big.NewInt(0),
			decimals: 18,
			expected: "0.0",
		},
		{
			name:     "positive delegates to chain.FormatDecimalAmount",
			amount:   big.NewInt(1_000_000_000_000_000_000),
			decimals: 18,
			expected: "1.0",
		},
		{
			name:     "negative 1 wei",
			amount:   big.NewInt(-1),
			decimals: 18,
			expected: "-0.000000000000000001",
		},
		{
			name:     "negative 1 ETH",
			amount:   big.NewInt(-1_000_000_000_000_000_000),
			decimals: 18,
			expected: "-1.0",
		},
		{
			name:     "negative 0.5 ETH",
			amount:   big.NewInt(-500_000_000_000_000_000),
			decimals: 18,
			expected: "-0.5",
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

func TestBalanceStruct(t *testing.T) {
	t.Parallel()

	t.Run("ETH balance without unconfirmed", func(t *testing.T) {
		t.Parallel()
		balance := &Balance{
			Address:  "0x742d35Cc6634C0532925a3b844Bc454e4438f44e",
			Amount:   big.NewInt(1_000_000_000_000_000_000),
			Symbol:   "ETH",
			Decimals: 18,
			Token:    "",
		}
		assert.Equal(t, "ETH", balance.Symbol)
		assert.Equal(t, 18, balance.Decimals)
		assert.Empty(t, balance.Token)
		assert.Nil(t, balance.Unconfirmed)
	})

	t.Run("ETH balance with unconfirmed delta", func(t *testing.T) {
		t.Parallel()
		balance := &Balance{
			Address:     "0x742d35Cc6634C0532925a3b844Bc454e4438f44e",
			Amount:      big.NewInt(1_000_000_000_000_000_000),
			Unconfirmed: big.NewInt(-500_000_000_000_000_000),
			Symbol:      "ETH",
			Decimals:    18,
		}
		assert.Equal(t, big.NewInt(-500_000_000_000_000_000), balance.Unconfirmed)
	})

	t.Run("USDC balance", func(t *testing.T) {
		t.Parallel()
		balance := &Balance{
			Address:  "0x742d35Cc6634C0532925a3b844Bc454e4438f44e",
			Amount:   big.NewInt(1_000_000),
			Symbol:   "USDC",
			Decimals: 6,
			Token:    USDCMainnet,
		}
		assert.Equal(t, "USDC", balance.Symbol)
		assert.Equal(t, 6, balance.Decimals)
		assert.Equal(t, USDCMainnet, balance.Token)
	})
}
