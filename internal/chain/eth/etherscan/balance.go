package etherscan

import (
	"context"
	"math/big"
	"net/url"
	"time"

	"github.com/mrz1836/sigil/internal/chain/eth"
	"github.com/mrz1836/sigil/internal/metrics"
	sigilerr "github.com/mrz1836/sigil/pkg/errors"
)

// ErrInvalidBalance indicates a balance result could not be parsed.
var ErrInvalidBalance = &sigilerr.SigilError{
	Code:     "ETHERSCAN_INVALID_BALANCE",
	Message:  "invalid balance value in Etherscan response",
	ExitCode: sigilerr.ExitGeneral,
}

// GetNativeBalance retrieves the native ETH balance for an address via Etherscan API.
func (c *Client) GetNativeBalance(ctx context.Context, address string) (*eth.Balance, error) {
	start := time.Now()

	params := url.Values{
		"module":  {"account"},
		"action":  {"balance"},
		"address": {address},
		"tag":     {"latest"},
		"apikey":  {c.apiKey},
	}

	result, err := c.doRequest(ctx, params)
	metrics.Global.RecordRPCCall("eth", time.Since(start), err)
	if err != nil {
		return nil, err
	}

	amount := new(big.Int)
	if _, ok := amount.SetString(result, 10); !ok {
		return nil, sigilerr.WithDetails(ErrInvalidBalance, map[string]string{
			"result": result,
		})
	}

	return &eth.Balance{
		Address:  address,
		Amount:   amount,
		Symbol:   "ETH",
		Decimals: 18,
	}, nil
}

// GetTokenBalance retrieves an ERC-20 token balance for an address via Etherscan API.
func (c *Client) GetTokenBalance(ctx context.Context, address, tokenAddress string) (*eth.Balance, error) {
	start := time.Now()

	params := url.Values{
		"module":          {"account"},
		"action":          {"tokenbalance"},
		"contractaddress": {tokenAddress},
		"address":         {address},
		"tag":             {"latest"},
		"apikey":          {c.apiKey},
	}

	result, err := c.doRequest(ctx, params)
	metrics.Global.RecordRPCCall("eth", time.Since(start), err)
	if err != nil {
		return nil, err
	}

	amount := new(big.Int)
	if _, ok := amount.SetString(result, 10); !ok {
		return nil, sigilerr.WithDetails(ErrInvalidBalance, map[string]string{
			"result": result,
		})
	}

	return &eth.Balance{
		Address: address,
		Amount:  amount,
	}, nil
}

// GetUSDCBalance retrieves the USDC balance for an address.
func (c *Client) GetUSDCBalance(ctx context.Context, address string) (*eth.Balance, error) {
	balance, err := c.GetTokenBalance(ctx, address, eth.USDCMainnet)
	if err != nil {
		return nil, err
	}

	balance.Symbol = "USDC"
	balance.Decimals = eth.USDCDecimals
	balance.Token = eth.USDCMainnet

	return balance, nil
}
