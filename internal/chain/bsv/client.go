// Package bsv provides Bitcoin SV chain client implementation.
package bsv

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"regexp"
	"sort"
	"time"

	"github.com/mrz1836/sigil/internal/chain"
	"github.com/mrz1836/sigil/internal/metrics"
	sigilerr "github.com/mrz1836/sigil/pkg/errors"
)

const (
	// decimals is the number of decimals for BSV (satoshis).
	decimals = 8

	// defaultTimeout is the default HTTP request timeout.
	defaultTimeout = 30 * time.Second

	// estimatedTxSize is the estimated transaction size in bytes for fee calculation.
	estimatedTxSize = 225 // Typical P2PKH transaction size
)

// Network represents the BSV network.
type Network string

// Network constants.
const (
	// NetworkMainnet is the BSV mainnet.
	NetworkMainnet Network = "main"
	// NetworkTestnet is the BSV testnet.
	NetworkTestnet Network = "test"
)

var (
	// ErrInvalidAddress indicates the address format is invalid.
	ErrInvalidAddress = &sigilerr.SigilError{
		Code:     "BSV_INVALID_ADDRESS",
		Message:  "invalid BSV address format",
		ExitCode: sigilerr.ExitInput,
	}

	// ErrInvalidAmount indicates the amount format is invalid.
	ErrInvalidAmount = &sigilerr.SigilError{
		Code:     "BSV_INVALID_AMOUNT",
		Message:  "invalid amount format",
		ExitCode: sigilerr.ExitInput,
	}

	// ErrInsufficientFunds indicates insufficient funds for transaction.
	ErrInsufficientFunds = &sigilerr.SigilError{
		Code:     "BSV_INSUFFICIENT_FUNDS",
		Message:  "insufficient funds for transaction",
		ExitCode: sigilerr.ExitPermission,
	}

	// Base58 character set (excludes 0, O, I, l).
	base58Regex = regexp.MustCompile("^[13][1-9A-HJ-NP-Za-km-z]{25,34}$")
)

// DebugLogger is the interface for debug logging.
// This allows the client to accept different logger implementations.
type DebugLogger interface {
	Debug(format string, args ...any)
}

// ClientOptions contains optional configuration for the BSV client.
type ClientOptions struct {
	// BaseURL overrides the default WhatsOnChain API URL.
	BaseURL string

	// BroadcastURL overrides the default broadcast URL.
	// When set, a single WhatsOnChain-format broadcaster is used with this URL as base.
	BroadcastURL string

	// APIKey is the optional WhatsOnChain API key for higher rate limits.
	APIKey string

	// Network specifies mainnet or testnet.
	Network Network

	// Logger is an optional debug logger for diagnostic output.
	Logger DebugLogger
}

// Compile-time interface check
var _ chain.Chain = (*Client)(nil)

// Client provides Bitcoin SV blockchain operations.
type Client struct {
	baseURL      string
	apiKey       string
	network      Network
	httpClient   *http.Client
	logger       DebugLogger
	broadcasters []Broadcaster
}

// NewClient creates a new BSV client.
func NewClient(opts *ClientOptions) *Client {
	c := &Client{
		baseURL: "https://api.whatsonchain.com/v1/bsv/main",
		network: NetworkMainnet,
		httpClient: &http.Client{
			Timeout: defaultTimeout,
		},
	}

	if opts != nil {
		c.applyOptions(opts)
	}

	// Default broadcasters: WhatsOnChain (primary) + GorillaPool ARC (fallback).
	if len(c.broadcasters) == 0 {
		c.broadcasters = []Broadcaster{
			&WhatsOnChainBroadcaster{BaseURL: c.baseURL, APIKey: c.apiKey},
			&GorillaPoolARCBroadcaster{BaseURL: GorillaPoolARCURL},
		}
	}

	return c
}

// ID returns the chain identifier.
func (c *Client) ID() chain.ID {
	return chain.BSV
}

// BalanceResponse represents the WhatsOnChain balance response.
type BalanceResponse struct {
	Confirmed   int64 `json:"confirmed"`
	Unconfirmed int64 `json:"unconfirmed"`
}

// UTXOResponse represents a WhatsOnChain UTXO response.
type UTXOResponse struct {
	TxID   string `json:"tx_hash"`
	Vout   uint32 `json:"tx_pos"`
	Value  uint64 `json:"value"`
	Height int64  `json:"height"`
}

// UTXO represents an unspent transaction output.
type UTXO struct {
	TxID          string
	Vout          uint32
	Amount        uint64
	ScriptPubKey  string
	Address       string
	Confirmations uint32
}

// GetBalance retrieves the BSV balance for an address.
func (c *Client) GetBalance(ctx context.Context, address string) (*big.Int, error) {
	start := time.Now()
	result, err := c.doGetBalance(ctx, address)
	metrics.Global.RecordRPCCall("bsv", time.Since(start), err)
	return result, err
}

// doGetBalance performs the actual balance lookup, returning only the confirmed balance.
//
//nolint:funcorder // Helper method grouped with its public caller
func (c *Client) doGetBalance(ctx context.Context, address string) (*big.Int, error) {
	resp, err := c.doGetFullBalance(ctx, address)
	if err != nil {
		return nil, err
	}
	return big.NewInt(resp.Confirmed), nil
}

// doGetFullBalance performs the actual balance lookup, returning the full API response.
//
//nolint:funcorder // Helper method grouped with its public caller
func (c *Client) doGetFullBalance(ctx context.Context, address string) (*BalanceResponse, error) {
	if err := c.ValidateAddress(address); err != nil {
		return nil, err
	}

	url := fmt.Sprintf("%s/address/%s/balance", c.baseURL, address)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}

	if c.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.apiKey)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", sigilerr.ErrNetworkError, err)
	}
	defer func() {
		if closeErr := resp.Body.Close(); closeErr != nil {
			c.debug("failed to close balance response body: %v", closeErr)
		}
	}()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%w: status %d", sigilerr.ErrNetworkError, resp.StatusCode)
	}

	var balance BalanceResponse
	if err := json.NewDecoder(resp.Body).Decode(&balance); err != nil {
		return nil, fmt.Errorf("decoding response: %w", err)
	}

	return &balance, nil
}

// GetTokenBalance is not supported for BSV.
func (c *Client) GetTokenBalance(_ context.Context, _, _ string) (*big.Int, error) {
	return nil, sigilerr.ErrNotSupported
}

// ListUTXOs returns unspent transaction outputs for an address.
func (c *Client) ListUTXOs(ctx context.Context, address string) ([]UTXO, error) {
	start := time.Now()
	result, err := c.doListUTXOs(ctx, address)
	metrics.Global.RecordRPCCall("bsv", time.Since(start), err)
	return result, err
}

// doListUTXOs performs the actual UTXO listing.
//
//nolint:funcorder // Helper method grouped with its public caller
func (c *Client) doListUTXOs(ctx context.Context, address string) ([]UTXO, error) {
	if err := c.ValidateAddress(address); err != nil {
		return nil, err
	}

	url := fmt.Sprintf("%s/address/%s/unspent", c.baseURL, address)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}

	if c.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.apiKey)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", sigilerr.ErrNetworkError, err)
	}
	defer func() {
		if closeErr := resp.Body.Close(); closeErr != nil {
			c.debug("failed to close UTXO response body: %v", closeErr)
		}
	}()

	if resp.StatusCode != http.StatusOK {
		// Drain body to allow connection reuse
		_, _ = io.Copy(io.Discard, resp.Body)
		return nil, fmt.Errorf("%w: status %d", sigilerr.ErrNetworkError, resp.StatusCode)
	}

	var utxoResp []UTXOResponse
	if err := json.NewDecoder(resp.Body).Decode(&utxoResp); err != nil {
		return nil, fmt.Errorf("decoding response: %w", err)
	}

	utxos := make([]UTXO, len(utxoResp))
	for i, u := range utxoResp {
		utxos[i] = UTXO{
			TxID:    u.TxID,
			Vout:    u.Vout,
			Amount:  u.Value,
			Address: address,
		}
	}

	return utxos, nil
}

// SelectUTXOs chooses UTXOs to fund a transaction.
func (c *Client) SelectUTXOs(utxos []UTXO, amount, feeRate uint64) (selected []UTXO, change uint64, err error) {
	if len(utxos) == 0 {
		return nil, 0, ErrInsufficientFunds
	}

	// Sort UTXOs by amount (largest first) for simple selection
	sorted := make([]UTXO, len(utxos))
	copy(sorted, utxos)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].Amount > sorted[j].Amount
	})

	var total uint64
	var estimatedFee uint64
	for _, utxo := range sorted {
		selected = append(selected, utxo)
		total += utxo.Amount

		estimatedFee = EstimateTxSize(len(selected), 2) * feeRate
		target := amount + estimatedFee
		if total >= target {
			change = total - target
			if change < chain.BSV.DustLimit() {
				change = 0
			}
			return selected, change, nil
		}
	}

	target := amount + estimatedFee
	return nil, 0, fmt.Errorf("%w: need %d satoshis, have %d", ErrInsufficientFunds, target, total)
}

// EstimateFee estimates the fee for a transaction.
func (c *Client) EstimateFee(_ context.Context, _, _ string, _ *big.Int) (*big.Int, error) {
	// Default fee rate: 1 sat/byte
	feeRate := int64(1)

	// Estimated transaction size: ~225 bytes for P2PKH
	fee := int64(estimatedTxSize) * feeRate

	return big.NewInt(fee), nil
}

// ValidateAddress checks if an address is valid for BSV.
func (c *Client) ValidateAddress(address string) error {
	if address == "" {
		return ErrInvalidAddress
	}

	if !base58Regex.MatchString(address) {
		return ErrInvalidAddress
	}

	return nil
}

// FormatAmount converts a big.Int (satoshis) to a human-readable BSV string.
func (c *Client) FormatAmount(amount *big.Int) string {
	if amount == nil {
		return "0.00000000"
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

// ParseAmount converts a human-readable BSV string to big.Int (satoshis).
func (c *Client) ParseAmount(amount string) (*big.Int, error) {
	return chain.ParseDecimalAmount(amount, decimals, ErrInvalidAmount)
}

// applyOptions applies optional configuration.
func (c *Client) applyOptions(opts *ClientOptions) {
	if opts.BaseURL != "" {
		c.baseURL = opts.BaseURL
		// When BaseURL is set (e.g., for testing), use a single WoC broadcaster
		// pointing at the same server, unless BroadcastURL is explicitly set.
		if opts.BroadcastURL == "" {
			c.broadcasters = []Broadcaster{
				&WhatsOnChainBroadcaster{BaseURL: opts.BaseURL, APIKey: opts.APIKey},
			}
		}
	}
	if opts.BroadcastURL != "" {
		c.broadcasters = []Broadcaster{
			&WhatsOnChainBroadcaster{BaseURL: opts.BroadcastURL, APIKey: opts.APIKey},
		}
	}
	if opts.APIKey != "" {
		c.apiKey = opts.APIKey
	}
	if opts.Network != "" {
		c.network = opts.Network
		if opts.BaseURL == "" {
			c.baseURL = fmt.Sprintf("https://api.whatsonchain.com/v1/bsv/%s", opts.Network)
		}
	}
	if opts.Logger != nil {
		c.logger = opts.Logger
	}
}

// debug logs a debug message if a logger is configured.
func (c *Client) debug(format string, args ...any) {
	if c.logger != nil {
		c.logger.Debug(format, args...)
	}
}
