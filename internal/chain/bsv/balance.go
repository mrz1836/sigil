package bsv

import (
	"context"
	"math/big"
)

// Balance represents a balance result with metadata.
type Balance struct {
	Address  string
	Amount   *big.Int
	Symbol   string
	Decimals int
}

// GetNativeBalance retrieves the native BSV balance.
func (c *Client) GetNativeBalance(ctx context.Context, address string) (*Balance, error) {
	amount, err := c.GetBalance(ctx, address)
	if err != nil {
		return nil, err
	}

	return &Balance{
		Address:  address,
		Amount:   amount,
		Symbol:   "BSV",
		Decimals: decimals,
	}, nil
}

// GetAllBalances retrieves all BSV balances (just native for BSV).
func (c *Client) GetAllBalances(ctx context.Context, address string) ([]*Balance, error) {
	balance, err := c.GetNativeBalance(ctx, address)
	if err != nil {
		return nil, err
	}

	return []*Balance{balance}, nil
}

// FormatBalanceAmount formats a balance amount with the correct decimals.
func FormatBalanceAmount(amount *big.Int, decimals int) string {
	if amount == nil {
		return "0"
	}

	str := amount.String()

	// Pad with leading zeros if necessary
	for len(str) <= decimals {
		str = "0" + str
	}

	// Insert decimal point
	decimalPos := len(str) - decimals

	// Trim trailing zeros after decimal point
	result := str[:decimalPos] + "." + str[decimalPos:]

	// Remove unnecessary trailing zeros
	for len(result) > 1 && result[len(result)-1] == '0' && result[len(result)-2] != '.' {
		result = result[:len(result)-1]
	}

	return result
}
