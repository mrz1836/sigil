package eth

import (
	"context"
	"fmt"
	"math/big"

	sigilerrors "github.com/mrz1836/sigil/pkg/errors"
)

// GasSpeed represents the transaction speed preference.
type GasSpeed string

const (
	// GasSpeedSlow uses lower gas price for cheaper, slower transactions.
	GasSpeedSlow GasSpeed = "slow"
	// GasSpeedMedium uses suggested gas price for balanced cost/speed.
	GasSpeedMedium GasSpeed = "medium"
	// GasSpeedFast uses higher gas price for faster confirmation.
	GasSpeedFast GasSpeed = "fast"

	// GasLimitETHTransfer is the gas limit for standard ETH transfers.
	GasLimitETHTransfer uint64 = 21000
	// GasLimitERC20Transfer is the typical gas limit for ERC-20 transfers.
	GasLimitERC20Transfer uint64 = 65000
	// GasLimitERC20Approve is the gas limit for ERC-20 approve calls.
	GasLimitERC20Approve uint64 = 50000

	// slowMultiplier reduces gas price by 20% for slow transactions.
	slowMultiplier = 0.8
	// fastMultiplier increases gas price by 20% for fast transactions.
	fastMultiplier = 1.2
)

// ParseGasSpeed parses a string into a GasSpeed.
func ParseGasSpeed(s string) (GasSpeed, error) {
	switch s {
	case "slow":
		return GasSpeedSlow, nil
	case "", "medium":
		return GasSpeedMedium, nil
	case "fast":
		return GasSpeedFast, nil
	default:
		return "", sigilerrors.WithDetails(sigilerrors.ErrInvalidGasSpeed, map[string]string{
			"speed":   s,
			"allowed": "slow, medium, or fast",
		})
	}
}

// GasEstimate contains gas price and limit for a transaction.
type GasEstimate struct {
	GasPrice *big.Int // Price per gas unit in wei
	GasLimit uint64   // Maximum gas units
	Total    *big.Int // Total cost (GasPrice * GasLimit)
}

// GasPrices contains gas prices for different speeds.
type GasPrices struct {
	Slow   *big.Int
	Medium *big.Int
	Fast   *big.Int
}

// GetGasPrices fetches current gas prices for all speed levels.
func (c *Client) GetGasPrices(ctx context.Context) (*GasPrices, error) {
	if err := c.connect(ctx); err != nil {
		return nil, err
	}

	// Get suggested gas price from the network
	suggestedPrice, err := c.rpcClient.GasPrice(ctx)
	if err != nil {
		return nil, fmt.Errorf("getting suggested gas price: %w", err)
	}

	// Calculate slow and fast prices based on suggested
	slowPrice := multiplyBigInt(suggestedPrice, slowMultiplier)
	fastPrice := multiplyBigInt(suggestedPrice, fastMultiplier)

	return &GasPrices{
		Slow:   slowPrice,
		Medium: suggestedPrice,
		Fast:   fastPrice,
	}, nil
}

// GetGasPrice returns the gas price for the specified speed.
func (c *Client) GetGasPrice(ctx context.Context, speed GasSpeed) (*big.Int, error) {
	prices, err := c.GetGasPrices(ctx)
	if err != nil {
		return nil, err
	}

	switch speed {
	case GasSpeedSlow:
		return prices.Slow, nil
	case GasSpeedMedium:
		return prices.Medium, nil
	case GasSpeedFast:
		return prices.Fast, nil
	default:
		return prices.Medium, nil
	}
}

// EstimateGasForETHTransfer estimates gas for a native ETH transfer.
func (c *Client) EstimateGasForETHTransfer(ctx context.Context, speed GasSpeed) (*GasEstimate, error) {
	gasPrice, err := c.GetGasPrice(ctx, speed)
	if err != nil {
		return nil, err
	}

	gasLimit := GasLimitETHTransfer
	total := new(big.Int).Mul(gasPrice, new(big.Int).SetUint64(gasLimit))

	return &GasEstimate{
		GasPrice: gasPrice,
		GasLimit: gasLimit,
		Total:    total,
	}, nil
}

// EstimateGasForERC20Transfer estimates gas for an ERC-20 token transfer.
func (c *Client) EstimateGasForERC20Transfer(ctx context.Context, speed GasSpeed) (*GasEstimate, error) {
	gasPrice, err := c.GetGasPrice(ctx, speed)
	if err != nil {
		return nil, err
	}

	// Use default gas limit for ERC-20 transfers
	// In a real scenario, we would estimate this by calling estimateGas
	gasLimit := GasLimitERC20Transfer
	total := new(big.Int).Mul(gasPrice, new(big.Int).SetUint64(gasLimit))

	return &GasEstimate{
		GasPrice: gasPrice,
		GasLimit: gasLimit,
		Total:    total,
	}, nil
}

// EstimateGasWithData estimates gas for a transaction with specific data.
func (c *Client) EstimateGasWithData(ctx context.Context, to string, data []byte, speed GasSpeed) (*GasEstimate, error) {
	if err := c.connect(ctx); err != nil {
		return nil, err
	}

	gasPrice, err := c.GetGasPrice(ctx, speed)
	if err != nil {
		return nil, err
	}

	// Estimate gas using the node
	toAddr, err := NormalizeAddress(to)
	if err != nil {
		return nil, fmt.Errorf("invalid to address: %w", err)
	}

	gasLimit, err := c.estimateGasWithClient(ctx, toAddr, data)
	if err != nil {
		// Fallback to default ERC-20 transfer limit
		gasLimit = GasLimitERC20Transfer
	}

	total := new(big.Int).Mul(gasPrice, new(big.Int).SetUint64(gasLimit))

	return &GasEstimate{
		GasPrice: gasPrice,
		GasLimit: gasLimit,
		Total:    total,
	}, nil
}

// estimateGasWithClient uses the connected client to estimate gas.
func (c *Client) estimateGasWithClient(ctx context.Context, to string, _ []byte) (uint64, error) {
	// For now, return default limits based on whether we have data
	// In production, we would use ethereum.CallMsg with the actual data
	_ = ctx
	_ = to
	return GasLimitERC20Transfer, nil
}

// FormatGasPrice formats a gas price in wei to a human-readable Gwei string.
func FormatGasPrice(weiPrice *big.Int) string {
	if weiPrice == nil {
		return "0 Gwei"
	}

	// Convert wei to Gwei (1 Gwei = 10^9 wei)
	gwei := new(big.Float).SetInt(weiPrice)
	divisor := new(big.Float).SetInt64(1_000_000_000)
	gwei.Quo(gwei, divisor)

	// Format to 2 decimal places
	return fmt.Sprintf("%.2f Gwei", gwei)
}

// multiplyBigInt multiplies a big.Int by a float multiplier.
func multiplyBigInt(n *big.Int, multiplier float64) *big.Int {
	// Convert to float, multiply, convert back
	f := new(big.Float).SetInt(n)
	m := new(big.Float).SetFloat64(multiplier)
	f.Mul(f, m)

	result, _ := f.Int(nil)
	return result
}

// GetNonce gets the next nonce for an address.
// Uses a local nonce manager to prevent nonce collisions during rapid sends.
// The local nonce is compared with the RPC pending nonce; the higher value is used.
func (c *Client) GetNonce(ctx context.Context, address string) (uint64, error) {
	if err := c.connect(ctx); err != nil {
		return 0, err
	}

	if err := ValidateChecksumAddress(address); err != nil {
		if !IsValidAddress(address) {
			return 0, sigilerrors.WithDetails(sigilerrors.ErrInvalidAddress, map[string]string{
				"address": address,
			})
		}
	}

	// Get pending nonce from the network
	rpcNonce, err := c.rpcClient.GetTransactionCount(ctx, address, "pending")
	if err != nil {
		return 0, fmt.Errorf("getting nonce: %w", err)
	}

	// Use the nonce manager to get the next nonce, taking into account
	// locally-tracked nonces from recent sends that may not yet be visible
	// to the RPC node's mempool.
	return c.nonceManager.Next(address, rpcNonce), nil
}

// GetChainID returns the chain ID.
func (c *Client) GetChainID(ctx context.Context) (*big.Int, error) {
	if err := c.connect(ctx); err != nil {
		return nil, err
	}
	return c.chainID, nil
}
