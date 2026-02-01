// Package bsv provides Bitcoin SV chain client implementation.
package bsv

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/mrz1836/sigil/internal/chain"
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
	ErrInvalidAddress = errors.New("invalid address format")

	// ErrInvalidAmount indicates the amount format is invalid.
	ErrInvalidAmount = errors.New("invalid amount format")

	// ErrInsufficientFunds indicates insufficient funds for transaction.
	ErrInsufficientFunds = errors.New("insufficient funds")

	// Base58 character set (excludes 0, O, I, l).
	base58Regex = regexp.MustCompile("^[13][1-9A-HJ-NP-Za-km-z]{25,34}$")
)

// ClientOptions contains optional configuration for the BSV client.
type ClientOptions struct {
	// BaseURL overrides the default WhatsOnChain API URL.
	BaseURL string

	// APIKey is the optional WhatsOnChain API key for higher rate limits.
	APIKey string

	// Network specifies mainnet or testnet.
	Network Network
}

// Client provides Bitcoin SV blockchain operations.
type Client struct {
	baseURL    string
	apiKey     string
	network    Network
	httpClient *http.Client
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
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%w: status %d", sigilerr.ErrNetworkError, resp.StatusCode)
	}

	var balance BalanceResponse
	if err := json.NewDecoder(resp.Body).Decode(&balance); err != nil {
		return nil, fmt.Errorf("decoding response: %w", err)
	}

	// Return confirmed balance in satoshis
	return big.NewInt(balance.Confirmed), nil
}

// GetTokenBalance is not supported for BSV.
func (c *Client) GetTokenBalance(_ context.Context, _, _ string) (*big.Int, error) {
	return nil, sigilerr.ErrNotSupported
}

// ListUTXOs returns unspent transaction outputs for an address.
func (c *Client) ListUTXOs(ctx context.Context, address string) ([]UTXO, error) {
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
	defer func() { _ = resp.Body.Close() }()

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

	// Calculate estimated fee
	estimatedFee := uint64(estimatedTxSize) * feeRate
	target := amount + estimatedFee

	var total uint64
	for _, utxo := range sorted {
		selected = append(selected, utxo)
		total += utxo.Amount

		if total >= target {
			break
		}
	}

	if total < target {
		return nil, 0, fmt.Errorf("%w: need %d satoshis, have %d", ErrInsufficientFunds, target, total)
	}

	change = total - target
	return selected, change, nil
}

// EstimateFee estimates the fee for a transaction.
func (c *Client) EstimateFee(_ context.Context, _, _ string, _ *big.Int) (*big.Int, error) {
	// Default fee rate: 1 sat/byte
	feeRate := int64(1)

	// Estimated transaction size: ~225 bytes for P2PKH
	fee := int64(estimatedTxSize) * feeRate

	return big.NewInt(fee), nil
}

// Send builds, signs, and broadcasts a transaction.
func (c *Client) Send(_ context.Context, _ chain.SendRequest) (*chain.TransactionResult, error) {
	// TODO: Implement in Phase 8 (T088-T090)
	return nil, sigilerr.ErrNotImplemented
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
	return parseAmount(amount, decimals)
}

// applyOptions applies optional configuration.
func (c *Client) applyOptions(opts *ClientOptions) {
	if opts.BaseURL != "" {
		c.baseURL = opts.BaseURL
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

	// Scale integer part to satoshis
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
