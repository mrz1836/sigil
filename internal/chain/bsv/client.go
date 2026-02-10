// Package bsv provides Bitcoin SV chain client implementation.
package bsv

import (
	"context"
	"fmt"
	"math/big"
	"net/http"
	"regexp"
	"sort"
	"time"

	whatsonchain "github.com/mrz1836/go-whatsonchain"

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

// WOCClient is the narrow interface for WhatsOnChain SDK methods used by this package.
type WOCClient interface {
	AddressBalance(ctx context.Context, address string) (*whatsonchain.AddressBalance, error)
	AddressUnspentTransactions(ctx context.Context, address string) (whatsonchain.AddressHistory, error)
	GetMinerFeesStats(ctx context.Context, from, to int64) ([]*whatsonchain.MinerFeeStats, error)
	BroadcastTx(ctx context.Context, txHex string) (string, error)

	// Bulk operations (max 20 addresses per call)
	BulkAddressConfirmedBalance(ctx context.Context, list *whatsonchain.AddressList) (whatsonchain.AddressBalances, error)
	BulkAddressUnconfirmedBalance(ctx context.Context, list *whatsonchain.AddressList) (whatsonchain.AddressBalances, error)
}

// Compile-time check that the real SDK client satisfies WOCClient.
var _ WOCClient = (whatsonchain.ClientInterface)(nil)

// Logger is the interface for client logging.
// Supports both debug (verbose) and error (always captured) levels.
type Logger interface {
	Debug(format string, args ...any)
	Error(format string, args ...any)
}

// DebugLogger is kept as an alias for backward compatibility.
type DebugLogger = Logger

// ClientOptions contains optional configuration for the BSV client.
type ClientOptions struct {
	// WOCClient allows injecting a custom WhatsOnChain client (e.g., for testing).
	WOCClient WOCClient

	// Broadcasters overrides the default broadcast providers (e.g., for testing).
	// When set, no default broadcasters are created.
	Broadcasters []Broadcaster

	// APIKey is the optional WhatsOnChain API key for higher rate limits.
	APIKey string

	// Network specifies mainnet or testnet.
	Network Network

	// Logger is an optional debug logger for diagnostic output.
	Logger DebugLogger

	// FeeStrategy selects the fee rate selection strategy (economy, normal, priority).
	FeeStrategy FeeStrategy

	// MinMiners is the minimum number of miners that must accept the fee (used by normal strategy).
	MinMiners int
}

// Compile-time interface check
var _ chain.Chain = (*Client)(nil)

// Client provides Bitcoin SV blockchain operations.
type Client struct {
	woc          WOCClient
	network      Network
	logger       DebugLogger
	broadcasters []Broadcaster
	feeStrategy  FeeStrategy
	minMiners    int
}

// NewClient creates a new BSV client.
func NewClient(ctx context.Context, opts *ClientOptions) *Client {
	c := &Client{
		network:     NetworkMainnet,
		feeStrategy: FeeStrategyNormal,
		minMiners:   3,
	}

	if opts != nil {
		c.applyOptions(opts)
	}

	c.initializeWOCClient(ctx, opts)
	c.initializeBroadcasters(opts)

	return c
}

// initializeWOCClient creates the WhatsOnChain SDK client if not already injected.
//
//nolint:funcorder // Helper method grouped with NewClient
func (c *Client) initializeWOCClient(ctx context.Context, opts *ClientOptions) {
	if c.woc != nil {
		return
	}

	var wocOpts []whatsonchain.ClientOption
	wocOpts = append(wocOpts, whatsonchain.WithNetwork(mapNetwork(c.network)))
	if opts != nil && opts.APIKey != "" {
		wocOpts = append(wocOpts, whatsonchain.WithAPIKey(opts.APIKey))
	}

	wocClient, err := whatsonchain.NewClient(ctx, wocOpts...)
	if err != nil {
		// SDK NewClient only returns an error for truly invalid configuration.
		// Fall back to a default client.
		wocClient, _ = whatsonchain.NewClient(ctx)
	}
	c.woc = wocClient
}

// initializeBroadcasters sets up broadcast providers if not already configured.
//
//nolint:funcorder // Helper method grouped with NewClient
func (c *Client) initializeBroadcasters(opts *ClientOptions) {
	if len(c.broadcasters) > 0 {
		return
	}

	if opts != nil && opts.WOCClient != nil {
		// WOC client was injected (e.g., testing) — only use the SDK broadcaster.
		// No real network fallback to avoid accidental live HTTP calls in tests.
		c.broadcasters = []Broadcaster{
			&WOCSDKBroadcaster{woc: c.woc},
		}
		return
	}

	// Production: WhatsOnChain SDK (primary) + GorillaPool ARC (fallback).
	c.broadcasters = []Broadcaster{
		&WOCSDKBroadcaster{woc: c.woc},
		&GorillaPoolARCBroadcaster{
			BaseURL:    GorillaPoolARCURL,
			httpClient: &http.Client{Timeout: defaultTimeout},
		},
	}
}

// mapNetwork converts the sigil Network type to the SDK's NetworkType.
func mapNetwork(n Network) whatsonchain.NetworkType {
	switch n {
	case NetworkTestnet:
		return whatsonchain.NetworkTest
	case NetworkMainnet:
		return whatsonchain.NetworkMain
	default:
		return whatsonchain.NetworkMain
	}
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

	bal, err := c.woc.AddressBalance(ctx, address)
	if err != nil {
		c.logError("balance fetch failed for %s: %v", address, err)
		return nil, fmt.Errorf("%w: %w", sigilerr.ErrNetworkError, err)
	}

	return &BalanceResponse{
		Confirmed:   bal.Confirmed,
		Unconfirmed: bal.Unconfirmed,
	}, nil
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

	history, err := c.woc.AddressUnspentTransactions(ctx, address)
	if err != nil {
		c.logError("utxo fetch failed for %s: %v", address, err)
		return nil, fmt.Errorf("%w: %w", sigilerr.ErrNetworkError, err)
	}

	utxos := make([]UTXO, len(history))
	for i, h := range history {
		utxos[i] = UTXO{
			TxID:    h.TxHash,
			Vout:    uint32(h.TxPos), //nolint:gosec // TxPos is always non-negative for UTXOs
			Amount:  uint64(h.Value), //nolint:gosec // Value is always non-negative for UTXOs
			Address: address,
		}
	}

	return utxos, nil
}

// SelectUTXOs chooses UTXOs to fund a transaction.
//
//nolint:gocognit // Overflow checks add necessary complexity for fund safety
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

		sum, addErr := checkedAdd(total, utxo.Amount)
		if addErr != nil {
			return nil, 0, fmt.Errorf("UTXO sum: %w", addErr)
		}
		total = sum

		estimatedFee = (EstimateTxSize(len(selected), 2)*feeRate + 999) / 1000
		target, targetErr := checkedAdd(amount, estimatedFee)
		if targetErr != nil {
			return nil, 0, fmt.Errorf("target amount: %w", targetErr)
		}
		if total >= target {
			change = total - target
			if change < chain.BSV.DustLimit() {
				change = 0
			}
			return selected, change, nil
		}
	}

	target, _ := checkedAdd(amount, estimatedFee)
	return nil, 0, fmt.Errorf("%w: need %d satoshis, have %d", ErrInsufficientFunds, target, total)
}

// EstimateFee estimates the fee for a transaction.
func (c *Client) EstimateFee(_ context.Context, _, _ string, _ *big.Int) (*big.Int, error) {
	// Default fee rate: 250 sat/KB (0.25 sat/byte)
	feeRate := int64(DefaultFeeRate)

	// Estimated transaction size: ~225 bytes for P2PKH
	fee := (int64(estimatedTxSize)*feeRate + 999) / 1000

	return big.NewInt(fee), nil
}

// ValidateAddress checks if an address is valid for BSV.
func (c *Client) ValidateAddress(address string) error {
	return chain.ValidateAddressWithRegex(address, base58Regex, ErrInvalidAddress)
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
	if opts.WOCClient != nil {
		c.woc = opts.WOCClient
	}
	if len(opts.Broadcasters) > 0 {
		c.broadcasters = opts.Broadcasters
	}
	if opts.Network != "" {
		c.network = opts.Network
	}
	if opts.Logger != nil {
		c.logger = opts.Logger
	}
	if opts.FeeStrategy != "" {
		c.feeStrategy = opts.FeeStrategy
	}
	if opts.MinMiners > 0 {
		c.minMiners = opts.MinMiners
	}
}

// debug logs a debug message if a logger is configured.
func (c *Client) debug(format string, args ...any) {
	if c.logger != nil {
		c.logger.Debug(format, args...)
	}
}

// logError logs an error message if a logger is configured.
func (c *Client) logError(format string, args ...any) {
	if c.logger != nil {
		c.logger.Error(format, args...)
	}
}
