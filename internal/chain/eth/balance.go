package eth

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
	Token    string // Empty for native ETH
}

// GetNativeBalance retrieves the native ETH balance.
func (c *Client) GetNativeBalance(ctx context.Context, address string) (*Balance, error) {
	amount, err := c.GetBalance(ctx, address)
	if err != nil {
		return nil, err
	}

	return &Balance{
		Address:  address,
		Amount:   amount,
		Symbol:   "ETH",
		Decimals: decimals,
	}, nil
}

// GetUSDCBalance retrieves the USDC balance.
func (c *Client) GetUSDCBalance(ctx context.Context, address string) (*Balance, error) {
	amount, err := c.GetTokenBalance(ctx, address, USDCMainnet)
	if err != nil {
		return nil, err
	}

	return &Balance{
		Address:  address,
		Amount:   amount,
		Symbol:   "USDC",
		Decimals: USDCDecimals,
		Token:    USDCMainnet,
	}, nil
}

// GetAllBalances retrieves both ETH and USDC balances.
func (c *Client) GetAllBalances(ctx context.Context, address string) ([]*Balance, error) {
	balances := make([]*Balance, 0, 2)

	// Get ETH balance
	ethBalance, err := c.GetNativeBalance(ctx, address)
	if err != nil {
		return nil, err
	}
	balances = append(balances, ethBalance)

	// Get USDC balance
	usdcBalance, usdcErr := c.GetUSDCBalance(ctx, address)
	if usdcErr != nil {
		// Don't fail if USDC query fails, just skip it
		// (could be network issue or contract not deployed on testnet)
		return balances, nil //nolint:nilerr // intentionally ignoring USDC error
	}
	balances = append(balances, usdcBalance)

	return balances, nil
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
