package eth

import (
	"context"
	"errors"
	"fmt"
	"math/big"
	"sort"
	"sync"

	"github.com/mrz1836/sigil/internal/chain/eth/rpc"
	sigilerrors "github.com/mrz1836/sigil/pkg/errors"
)

// errAllRPCsFailed indicates all RPC endpoints failed to return a gas price.
var errAllRPCsFailed = errors.New("getting gas price: all RPC endpoints failed")

const (
	// minGasPriceWei is the minimum gas price floor in wei (1 Gwei).
	minGasPriceWei = 1_000_000_000
	// mainnetChainID is Ethereum mainnet chain ID.
	mainnetChainID = 1
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
	// gasEstimateBuffer adds a 20% safety margin to eth_estimateGas results.
	gasEstimateBuffer = 1.2
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
// Uses a layered strategy: Etherscan gas oracle → multi-RPC median → minimum floor.
func (c *Client) GetGasPrices(ctx context.Context) (*GasPrices, error) {
	if err := c.connect(ctx); err != nil {
		return nil, err
	}

	// Strategy 1: Try external gas oracle (most reliable, e.g. Etherscan gas tracker)
	if c.gasPriceOracle != nil {
		slow, medium, fast, err := c.gasPriceOracle.GetGasPrices(ctx)
		if err == nil {
			return c.applyGasPriceFloor(&GasPrices{
				Slow: slow, Medium: medium, Fast: fast,
			}), nil
		}
		// Oracle failed — fall through to multi-RPC
	}

	// Strategy 2: Multi-RPC median (query all configured RPCs, take median)
	medianPrice, err := c.getMultiRPCMedianGasPrice(ctx)
	if err != nil {
		return nil, err
	}

	// Apply speed multipliers to the median
	slowPrice := multiplyBigInt(medianPrice, slowMultiplier)
	fastPrice := multiplyBigInt(medianPrice, fastMultiplier)

	// Strategy 3: Apply minimum floor as safety net
	return c.applyGasPriceFloor(&GasPrices{
		Slow:   slowPrice,
		Medium: new(big.Int).Set(medianPrice),
		Fast:   fastPrice,
	}), nil
}

// getMultiRPCMedianGasPrice queries eth_gasPrice from the primary and all fallback RPCs
// in parallel and returns the median price. Returns an error if all RPCs fail.
func (c *Client) getMultiRPCMedianGasPrice(ctx context.Context) (*big.Int, error) {
	var mu sync.Mutex
	prices := make([]*big.Int, 0, 1+len(c.fallbackRPCs))

	// Primary RPC
	if price, err := c.rpcClient.GasPrice(ctx); err == nil {
		prices = append(prices, price)
	}

	// Fallback RPCs in parallel
	var wg sync.WaitGroup
	for _, fallbackURL := range c.fallbackRPCs {
		if fallbackURL == c.rpcURL {
			continue
		}
		wg.Add(1)
		go func(u string) {
			defer wg.Done()
			if price := c.queryFallbackGasPrice(ctx, u); price != nil {
				mu.Lock()
				prices = append(prices, price)
				mu.Unlock()
			}
		}(fallbackURL)
	}
	wg.Wait()

	if len(prices) == 0 {
		return nil, errAllRPCsFailed
	}

	return medianBigInt(prices), nil
}

// queryFallbackGasPrice queries eth_gasPrice from a single fallback RPC endpoint.
// Returns nil if the query fails.
func (c *Client) queryFallbackGasPrice(ctx context.Context, url string) *big.Int {
	var rpcOpts *rpc.ClientOptions
	if c.transport != nil {
		rpcOpts = &rpc.ClientOptions{Transport: c.transport}
	}
	fc := rpc.NewClientWithOptions(url, rpcOpts)
	defer fc.Close()
	price, err := fc.GasPrice(ctx)
	if err != nil {
		return nil
	}
	return price
}

// medianBigInt returns the median of a slice of big.Int values.
// For even-length slices, returns the average of the two middle values.
func medianBigInt(values []*big.Int) *big.Int {
	if len(values) == 0 {
		return big.NewInt(0)
	}
	if len(values) == 1 {
		return new(big.Int).Set(values[0])
	}

	// Sort a copy to avoid mutating the input
	sorted := make([]*big.Int, len(values))
	copy(sorted, values)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].Cmp(sorted[j]) < 0
	})

	mid := len(sorted) / 2
	if len(sorted)%2 != 0 {
		return new(big.Int).Set(sorted[mid])
	}
	// Average of two middle values
	sum := new(big.Int).Add(sorted[mid-1], sorted[mid])
	return sum.Div(sum, big.NewInt(2))
}

// applyGasPriceFloor enforces a minimum gas price on mainnet (chain ID 1).
// L2s and testnets can have sub-gwei gas prices, so the floor is not applied there.
func (c *Client) applyGasPriceFloor(prices *GasPrices) *GasPrices {
	if c.chainID == nil || c.chainID.Int64() != mainnetChainID {
		return prices
	}
	floor := big.NewInt(minGasPriceWei)
	if prices.Slow.Cmp(floor) < 0 {
		prices.Slow = new(big.Int).Set(floor)
	}
	if prices.Medium.Cmp(floor) < 0 {
		prices.Medium = new(big.Int).Set(floor)
	}
	if prices.Fast.Cmp(floor) < 0 {
		prices.Fast = new(big.Int).Set(floor)
	}
	return prices
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

// EstimateGasForETHTransfer estimates gas for a native ETH transfer using eth_estimateGas.
// Falls back to the standard 21000 gas limit if the RPC call fails.
func (c *Client) EstimateGasForETHTransfer(ctx context.Context, from, to string, value *big.Int, speed GasSpeed) (*GasEstimate, error) {
	gasPrice, err := c.GetGasPrice(ctx, speed)
	if err != nil {
		return nil, err
	}

	msg := rpc.CallMsg{
		From:  from,
		To:    to,
		Value: value,
	}

	gasLimit, err := c.estimateGasWithClient(ctx, msg)
	if err != nil {
		gasLimit = GasLimitETHTransfer
	}

	total := new(big.Int).Mul(gasPrice, new(big.Int).SetUint64(gasLimit))

	return &GasEstimate{
		GasPrice: gasPrice,
		GasLimit: gasLimit,
		Total:    total,
	}, nil
}

// EstimateGasForERC20Transfer estimates gas for an ERC-20 token transfer using eth_estimateGas.
// Falls back to the default 65000 gas limit if the RPC call fails.
func (c *Client) EstimateGasForERC20Transfer(ctx context.Context, from, tokenContract string, data []byte, speed GasSpeed) (*GasEstimate, error) {
	gasPrice, err := c.GetGasPrice(ctx, speed)
	if err != nil {
		return nil, err
	}

	msg := rpc.CallMsg{
		From: from,
		To:   tokenContract,
		Data: data,
	}

	gasLimit, err := c.estimateGasWithClient(ctx, msg)
	if err != nil {
		gasLimit = GasLimitERC20Transfer
	}

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

	toAddr, err := NormalizeAddress(to)
	if err != nil {
		return nil, fmt.Errorf("invalid to address: %w", err)
	}

	msg := rpc.CallMsg{
		To:   toAddr,
		Data: data,
	}

	gasLimit, err := c.estimateGasWithClient(ctx, msg)
	if err != nil {
		gasLimit = GasLimitERC20Transfer
	}

	total := new(big.Int).Mul(gasPrice, new(big.Int).SetUint64(gasLimit))

	return &GasEstimate{
		GasPrice: gasPrice,
		GasLimit: gasLimit,
		Total:    total,
	}, nil
}

// estimateGasWithClient calls eth_estimateGas via the RPC client and applies a safety buffer.
func (c *Client) estimateGasWithClient(ctx context.Context, msg rpc.CallMsg) (uint64, error) {
	if err := c.connect(ctx); err != nil {
		return 0, err
	}

	gasLimit, err := c.rpcClient.EstimateGas(ctx, msg)
	if err != nil {
		return 0, fmt.Errorf("eth_estimateGas: %w", err)
	}

	buffered := multiplyBigInt(new(big.Int).SetUint64(gasLimit), gasEstimateBuffer)
	return buffered.Uint64(), nil
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
