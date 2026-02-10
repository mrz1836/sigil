// Package eth provides Ethereum chain client implementation.
package eth

import (
	"context"
	"fmt"
	"math/big"
	"net/http"
	"regexp"
	"sync"

	"github.com/mrz1836/sigil/internal/chain"
	ethcrypto "github.com/mrz1836/sigil/internal/chain/eth/crypto"
	"github.com/mrz1836/sigil/internal/chain/eth/rpc"
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
	ErrInvalidAddress = &sigilerr.SigilError{
		Code:     "ETH_INVALID_ADDRESS",
		Message:  "invalid Ethereum address format",
		ExitCode: sigilerr.ExitInput,
	}

	// ErrInvalidTokenAddress indicates the token address format is invalid.
	ErrInvalidTokenAddress = &sigilerr.SigilError{
		Code:     "ETH_INVALID_TOKEN_ADDRESS",
		Message:  "invalid token address format",
		ExitCode: sigilerr.ExitInput,
	}

	// ErrInvalidAmount indicates the amount format is invalid.
	ErrInvalidAmount = &sigilerr.SigilError{
		Code:     "ETH_INVALID_AMOUNT",
		Message:  "invalid amount format",
		ExitCode: sigilerr.ExitInput,
	}

	// ErrRPCURLRequired indicates the RPC URL was not provided.
	ErrRPCURLRequired = &sigilerr.SigilError{
		Code:     "ETH_RPC_URL_REQUIRED",
		Message:  "RPC URL is required",
		ExitCode: sigilerr.ExitInput,
	}

	// addressRegex validates Ethereum addresses.
	addressRegex = regexp.MustCompile("^0x[0-9a-fA-F]{40}$")
)

// ClientOptions contains optional configuration for the ETH client.
type ClientOptions struct {
	// ChainID overrides the default chain ID detection.
	ChainID *big.Int
	// Transport overrides the default HTTP transport for the underlying RPC client.
	// Useful for sharing a transport across primary and fallback clients.
	Transport *http.Transport
}

// Compile-time interface checks
var (
	_ chain.Chain      = (*Client)(nil)
	_ chain.TokenChain = (*Client)(nil)
)

// Client provides Ethereum blockchain operations.
type Client struct {
	rpcURL    string
	rpcClient *rpc.Client
	chainID   *big.Int
	transport *http.Transport
	mu        sync.Mutex
	initErr   error
}

// NewClient creates a new ETH client.
func NewClient(rpcURL string, opts *ClientOptions) (*Client, error) {
	if rpcURL == "" {
		return nil, ErrRPCURLRequired
	}

	c := &Client{
		rpcURL: rpcURL,
	}

	if opts != nil {
		if opts.ChainID != nil {
			c.chainID = opts.ChainID
		}
		if opts.Transport != nil {
			c.transport = opts.Transport
		}
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

	balance, err := c.rpcClient.GetBalance(ctx, address, "latest")
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
	addr, _ := ethcrypto.HexToAddress(address)
	paddedAddr := ethcrypto.LeftPadBytes(addr.Bytes(), 32)

	// Build call data
	data := append(selector, paddedAddr...)

	// Create call message
	msg := rpc.CallMsg{
		To:   tokenAddress,
		Data: data,
	}

	// Execute call
	result, err := c.rpcClient.EthCall(ctx, msg, "latest")
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
	gasPrice, err := c.rpcClient.GasPrice(ctx)
	if err != nil {
		return nil, fmt.Errorf("getting gas price: %w", err)
	}

	// Estimate gas for a simple ETH transfer (21000 gas)
	msg := rpc.CallMsg{
		From:  from,
		To:    to,
		Value: amount,
	}

	gasLimit, err := c.rpcClient.EstimateGas(ctx, msg)
	if err != nil {
		// Default to 21000 for simple transfers
		gasLimit = defaultGasLimit
	}

	// Fee = gasPrice * gasLimit
	fee := new(big.Int).Mul(gasPrice, new(big.Int).SetUint64(gasLimit))
	return fee, nil
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
	return chain.ParseDecimalAmount(amount, decimals, ErrInvalidAmount)
}

// Close closes the client connection.
func (c *Client) Close() {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.rpcClient != nil {
		c.rpcClient.Close()
		c.rpcClient = nil
	}
}

// connect establishes the RPC connection if not already connected.
// This method is thread-safe and allows retries after transient failures.
func (c *Client) connect(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.rpcClient != nil && c.initErr == nil {
		return nil
	}

	var rpcOpts *rpc.ClientOptions
	if c.transport != nil {
		rpcOpts = &rpc.ClientOptions{Transport: c.transport}
	}
	c.rpcClient = rpc.NewClientWithOptions(c.rpcURL, rpcOpts)

	// Get chain ID if not set
	if c.chainID == nil {
		chainID, err := c.rpcClient.ChainID(ctx)
		if err != nil {
			c.rpcClient.Close()
			c.rpcClient = nil
			c.initErr = fmt.Errorf("getting chain ID: %w", err)
			return c.initErr
		}
		c.chainID = chainID
	}

	c.initErr = nil
	return nil
}
