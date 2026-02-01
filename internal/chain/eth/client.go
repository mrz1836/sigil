// Package eth provides Ethereum chain client implementation.
package eth

import (
	"context"
	"errors"
	"fmt"
	"math/big"
	"regexp"
	"strings"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"

	"github.com/mrz1836/sigil/internal/chain"
	sigilerr "github.com/mrz1836/sigil/pkg/errors"
)

const (
	// decimals is the number of decimals for ETH.
	decimals = 18

	// USDCMainnet is the USDC contract address on Ethereum mainnet.
	USDCMainnet = "0xA0b86991c6218b36c1d19D4a2e9Eb0cE3606eB48"

	// USDCDecimals is the number of decimals for USDC.
	USDCDecimals = 6

	// defaultGasLimit is the default gas limit for simple ETH transfers.
	defaultGasLimit = 21000
)

var (
	// ErrInvalidAddress indicates the address format is invalid.
	ErrInvalidAddress = errors.New("invalid address format")

	// ErrInvalidTokenAddress indicates the token address format is invalid.
	ErrInvalidTokenAddress = errors.New("invalid token address format")

	// ErrInvalidAmount indicates the amount format is invalid.
	ErrInvalidAmount = errors.New("invalid amount format")

	// ErrRPCURLRequired indicates the RPC URL was not provided.
	ErrRPCURLRequired = errors.New("RPC URL is required")

	// addressRegex validates Ethereum addresses.
	addressRegex = regexp.MustCompile("^0x[0-9a-fA-F]{40}$")
)

// ClientOptions contains optional configuration for the ETH client.
type ClientOptions struct {
	// ChainID overrides the default chain ID detection.
	ChainID *big.Int
}

// Client provides Ethereum blockchain operations.
type Client struct {
	rpcURL    string
	ethClient *ethclient.Client
	chainID   *big.Int
}

// NewClient creates a new ETH client.
func NewClient(rpcURL string, opts *ClientOptions) (*Client, error) {
	if rpcURL == "" {
		return nil, ErrRPCURLRequired
	}

	c := &Client{
		rpcURL: rpcURL,
	}

	if opts != nil && opts.ChainID != nil {
		c.chainID = opts.ChainID
	}

	return c, nil
}

// ID returns the chain identifier.
func (c *Client) ID() chain.ID {
	return chain.ETH
}

// GetBalance retrieves the ETH balance for an address.
func (c *Client) GetBalance(ctx context.Context, address string) (*big.Int, error) {
	if err := c.ValidateAddress(address); err != nil {
		return nil, err
	}

	if err := c.connect(ctx); err != nil {
		return nil, err
	}

	addr := common.HexToAddress(address)
	balance, err := c.ethClient.BalanceAt(ctx, addr, nil)
	if err != nil {
		return nil, fmt.Errorf("getting balance: %w", err)
	}

	return balance, nil
}

// GetTokenBalance retrieves the ERC-20 token balance for an address.
func (c *Client) GetTokenBalance(ctx context.Context, address, tokenAddress string) (*big.Int, error) {
	if err := c.ValidateAddress(address); err != nil {
		return nil, err
	}

	if !addressRegex.MatchString(tokenAddress) {
		return nil, ErrInvalidTokenAddress
	}

	if err := c.connect(ctx); err != nil {
		return nil, err
	}

	// ERC-20 balanceOf selector: keccak256("balanceOf(address)")[0:4]
	// = 0x70a08231
	selector := []byte{0x70, 0xa0, 0x82, 0x31}

	// Pad address to 32 bytes
	addr := common.HexToAddress(address)
	paddedAddr := common.LeftPadBytes(addr.Bytes(), 32)

	// Build call data
	data := append(selector, paddedAddr...)

	// Create call message
	tokenAddr := common.HexToAddress(tokenAddress)
	msg := ethereum.CallMsg{
		To:   &tokenAddr,
		Data: data,
	}

	// Execute call
	result, err := c.ethClient.CallContract(ctx, msg, nil)
	if err != nil {
		return nil, fmt.Errorf("calling balanceOf: %w", err)
	}

	// Parse result (32-byte big-endian uint256)
	if len(result) < 32 {
		return big.NewInt(0), nil
	}

	balance := new(big.Int).SetBytes(result)
	return balance, nil
}

// EstimateFee estimates the fee for a transaction.
func (c *Client) EstimateFee(ctx context.Context, from, to string, amount *big.Int) (*big.Int, error) {
	if err := c.ValidateAddress(from); err != nil {
		return nil, fmt.Errorf("from address: %w", err)
	}
	if err := c.ValidateAddress(to); err != nil {
		return nil, fmt.Errorf("to address: %w", err)
	}

	if err := c.connect(ctx); err != nil {
		return nil, err
	}

	// Get gas price
	gasPrice, err := c.ethClient.SuggestGasPrice(ctx)
	if err != nil {
		return nil, fmt.Errorf("getting gas price: %w", err)
	}

	// Estimate gas for a simple ETH transfer (21000 gas)
	fromAddr := common.HexToAddress(from)
	toAddr := common.HexToAddress(to)

	msg := ethereum.CallMsg{
		From:  fromAddr,
		To:    &toAddr,
		Value: amount,
	}

	gasLimit, err := c.ethClient.EstimateGas(ctx, msg)
	if err != nil {
		// Default to 21000 for simple transfers
		gasLimit = defaultGasLimit
	}

	// Fee = gasPrice * gasLimit
	fee := new(big.Int).Mul(gasPrice, new(big.Int).SetUint64(gasLimit))
	return fee, nil
}

// Send builds, signs, and broadcasts a transaction.
func (c *Client) Send(_ context.Context, _ chain.SendRequest) (*chain.TransactionResult, error) {
	// TODO: Implement in Phase 6 (T071-T074)
	return nil, sigilerr.ErrNotImplemented
}

// ValidateAddress checks if an address is valid for Ethereum.
func (c *Client) ValidateAddress(address string) error {
	if address == "" {
		return ErrInvalidAddress
	}

	if !addressRegex.MatchString(address) {
		return ErrInvalidAddress
	}

	return nil
}

// FormatAmount converts a big.Int (wei) to a human-readable ETH string.
func (c *Client) FormatAmount(amount *big.Int) string {
	if amount == nil {
		return "0.000000000000000000"
	}

	// Convert to string with all digits
	str := amount.String()

	// Pad with leading zeros if necessary
	for len(str) <= decimals {
		str = "0" + str
	}

	// Insert decimal point
	decimalPos := len(str) - decimals
	return str[:decimalPos] + "." + str[decimalPos:]
}

// ParseAmount converts a human-readable ETH string to big.Int (wei).
func (c *Client) ParseAmount(amount string) (*big.Int, error) {
	return parseAmount(amount, decimals)
}

// Close closes the client connection.
func (c *Client) Close() {
	if c.ethClient != nil {
		c.ethClient.Close()
		c.ethClient = nil
	}
}

// connect establishes the RPC connection if not already connected.
func (c *Client) connect(ctx context.Context) error {
	if c.ethClient != nil {
		return nil
	}

	client, err := ethclient.DialContext(ctx, c.rpcURL)
	if err != nil {
		return fmt.Errorf("connecting to ETH RPC: %w", err)
	}

	c.ethClient = client

	// Get chain ID if not set
	if c.chainID == nil {
		chainID, err := client.ChainID(ctx)
		if err != nil {
			return fmt.Errorf("getting chain ID: %w", err)
		}
		c.chainID = chainID
	}

	return nil
}

// parseAmount is a shared helper for parsing decimal amounts.
//
//nolint:gocognit,gocyclo // Decimal parsing requires sequential validation steps
func parseAmount(amount string, decimalPlaces int) (*big.Int, error) {
	if amount == "" {
		return nil, ErrInvalidAmount
	}

	// Check for negative amounts
	if strings.HasPrefix(amount, "-") {
		return nil, ErrInvalidAmount
	}

	// Split by decimal point
	parts := strings.Split(amount, ".")
	if len(parts) > 2 {
		return nil, ErrInvalidAmount
	}

	intPart := parts[0]
	decPart := ""
	if len(parts) == 2 {
		decPart = parts[1]
	}

	// Validate integer part
	if intPart == "" {
		intPart = "0"
	}
	intVal, ok := new(big.Int).SetString(intPart, 10)
	if !ok {
		return nil, ErrInvalidAmount
	}

	// Scale integer part
	multiplier := new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(decimalPlaces)), nil)
	result := new(big.Int).Mul(intVal, multiplier)

	// Handle decimal part
	if decPart != "" {
		// Validate decimal characters
		for _, c := range decPart {
			if c < '0' || c > '9' {
				return nil, ErrInvalidAmount
			}
		}

		// Pad or truncate decimal part
		for len(decPart) < decimalPlaces {
			decPart += "0"
		}
		decPart = decPart[:decimalPlaces]

		decVal, ok := new(big.Int).SetString(decPart, 10)
		if !ok {
			return nil, ErrInvalidAmount
		}

		result = result.Add(result, decVal)
	}

	return result, nil
}
