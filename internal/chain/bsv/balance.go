package bsv

import (
	"context"
	"math/big"
	"time"

	"github.com/mrz1836/sigil/internal/metrics"
)

// Balance represents a balance result with metadata.
type Balance struct {
	Address     string
	Amount      *big.Int
	Unconfirmed *big.Int // Unconfirmed balance delta in satoshis (can be negative)
	Symbol      string
	Decimals    int
}

// GetNativeBalance retrieves the native BSV balance including unconfirmed data.
func (c *Client) GetNativeBalance(ctx context.Context, address string) (*Balance, error) {
	start := time.Now()
	resp, err := c.doGetFullBalance(ctx, address)
	metrics.Global.RecordRPCCall("bsv", time.Since(start), err)
	if err != nil {
		return nil, err
	}

	bal := &Balance{
		Address:  address,
		Amount:   big.NewInt(resp.Confirmed),
		Symbol:   "BSV",
		Decimals: decimals,
	}
	if resp.Unconfirmed != 0 {
		bal.Unconfirmed = big.NewInt(resp.Unconfirmed)
	}

	return bal, nil
}

// GetAllBalances retrieves all BSV balances (just native for BSV).
func (c *Client) GetAllBalances(ctx context.Context, address string) ([]*Balance, error) {
	balance, err := c.GetNativeBalance(ctx, address)
	if err != nil {
		return nil, err
	}

	return []*Balance{balance}, nil
}
