package eth

import (
	"context"
	"math/big"
)

// Balance represents a balance result with metadata.
type Balance struct {
	Address     string
	Amount      *big.Int
	Unconfirmed *big.Int // Pending balance delta in wei (pending - latest; can be negative)
	Symbol      string
	Decimals    int
	Token       string // Empty for native ETH
}

// GetNativeBalance retrieves the native ETH balance including pending (unconfirmed) data.
func (c *Client) GetNativeBalance(ctx context.Context, address string) (*Balance, error) {
	amount, err := c.GetBalance(ctx, address)
	if err != nil {
		return nil, err
	}

	bal := &Balance{
		Address:  address,
		Amount:   amount,
		Symbol:   "ETH",
		Decimals: decimals,
	}

	// Attempt to fetch pending balance for unconfirmed delta.
	// Failure is non-fatal — pending data is optional.
	if c.rpcClient != nil {
		pendingAmount, pendingErr := c.rpcClient.GetBalance(ctx, address, "pending")
		if pendingErr == nil && pendingAmount.Cmp(amount) != 0 {
			bal.Unconfirmed = new(big.Int).Sub(pendingAmount, amount)
		}
	}

	return bal, nil
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
