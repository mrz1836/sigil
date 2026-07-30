package btc

import (
	"cmp"
	"context"
	"fmt"
	"math/big"
	"net/http"
	"slices"
	"time"

	"github.com/mrz1836/sigil/internal/chain"
	"github.com/mrz1836/sigil/internal/metrics"
	sigilerr "github.com/mrz1836/sigil/pkg/errors"
)

const (
	// decimals is the number of decimals for BTC (satoshis).
	decimals = 8

	// defaultTimeout is the default HTTP request timeout.
	defaultTimeout = 30 * time.Second
)

var (
	// ErrInvalidAmount indicates the amount format is invalid.
	ErrInvalidAmount = &sigilerr.SigilError{
		Code:     "BTC_INVALID_AMOUNT",
		Message:  "invalid amount format",
		ExitCode: sigilerr.ExitInput,
	}

	// ErrInsufficientFunds indicates insufficient funds for a transaction.
	ErrInsufficientFunds = &sigilerr.SigilError{
		Code:     "BTC_INSUFFICIENT_FUNDS",
		Message:  "insufficient funds for transaction",
		ExitCode: sigilerr.ExitPermission,
	}
)

// UTXO is an unspent transaction output. It is an alias for chain.UTXO so the
// BTC client satisfies chain.UTXOChain directly (no conversion layer): Sigil
// only spends its own legacy P2PKH outputs, whose fields map 1:1.
type UTXO = chain.UTXO

// Logger is the interface for client logging (debug + always-captured error).
type Logger interface {
	Debug(format string, args ...any)
	Error(format string, args ...any)
}

// ClientOptions contains optional configuration for the BTC client.
type ClientOptions struct {
	// Provider allows injecting a custom Esplora data provider (e.g., for testing).
	Provider EsploraProvider

	// Broadcasters overrides the default broadcast providers (e.g., for testing).
	// When set, no default broadcasters are created.
	Broadcasters []Broadcaster

	// Network specifies mainnet or testnet (testnet4).
	Network Network

	// Logger is an optional debug logger for diagnostic output.
	Logger Logger

	// FeeStrategy selects the fee tier (economy, normal, priority).
	FeeStrategy FeeStrategy

	// BaseURL overrides the Esplora base URL (useful for testing / self-hosting).
	BaseURL string

	// HTTPClient overrides the default HTTP client.
	HTTPClient *http.Client

	// APIKey is an optional Esplora provider API key (sent as a bearer token).
	// The public mempool.space API is keyless; this is for paid/self-hosted tiers.
	APIKey string
}

// Compile-time interface checks.
var (
	_ chain.Chain     = (*Client)(nil)
	_ chain.UTXOChain = (*Client)(nil)
)

// Client provides Bitcoin (BTC) blockchain operations.
type Client struct {
	provider     EsploraProvider
	network      Network
	logger       Logger
	broadcasters []Broadcaster
	feeStrategy  FeeStrategy
}

// NewClient creates a new BTC client. When opts.Provider is nil, a concrete
// mempool.space (Esplora) provider is created for the selected network.
func NewClient(_ context.Context, opts *ClientOptions) *Client {
	c := &Client{
		network:     NetworkMainnet,
		feeStrategy: FeeStrategyNormal,
	}

	if opts != nil {
		c.applyOptions(opts)
	}

	c.initializeProvider(opts)
	c.initializeBroadcasters(opts)

	return c
}

// applyOptions applies optional configuration.
//
//nolint:funcorder // Helper method grouped with NewClient
func (c *Client) applyOptions(opts *ClientOptions) {
	if opts.Provider != nil {
		c.provider = opts.Provider
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
}

// initializeProvider creates the concrete Esplora provider if not injected.
//
//nolint:funcorder // Helper method grouped with NewClient
func (c *Client) initializeProvider(opts *ClientOptions) {
	if c.provider != nil {
		return
	}

	baseURL := baseURLForNetwork(c.network)
	var apiKey string
	var httpClient *http.Client
	if opts != nil {
		if opts.BaseURL != "" {
			baseURL = opts.BaseURL
		}
		apiKey = opts.APIKey
		httpClient = opts.HTTPClient
	}
	if httpClient == nil {
		httpClient = &http.Client{Timeout: defaultTimeout}
	}

	c.provider = newEsploraHTTP(baseURL, apiKey, httpClient, c.logger)
}

// initializeBroadcasters sets up broadcast providers if not already configured.
//
//nolint:funcorder // Helper method grouped with NewClient
func (c *Client) initializeBroadcasters(opts *ClientOptions) {
	if len(c.broadcasters) > 0 {
		return
	}

	baseURL := baseURLForNetwork(c.network)
	var httpClient *http.Client
	if opts != nil {
		if opts.BaseURL != "" {
			baseURL = opts.BaseURL
		}
		httpClient = opts.HTTPClient
	}
	if httpClient == nil {
		httpClient = &http.Client{Timeout: defaultTimeout}
	}

	// Primary: mempool.space (matches the data provider + supports testnet4).
	c.broadcasters = []Broadcaster{
		&MempoolBroadcaster{BaseURL: baseURL, httpClient: httpClient},
	}

	// Blockstream is a mainnet-only fallback: its testnet is testnet3, a
	// different chain than mempool's testnet4, so never use it off mainnet.
	if c.network == NetworkMainnet {
		c.broadcasters = append(c.broadcasters, &BlockstreamBroadcaster{
			BaseURL:    BlockstreamMainnetURL,
			httpClient: httpClient,
		})
	}
}

// ID returns the chain identifier.
func (c *Client) ID() chain.ID {
	return chain.BTC
}

// GetBalance retrieves the confirmed BTC balance (in satoshis) for an address.
func (c *Client) GetBalance(ctx context.Context, address string) (*big.Int, error) {
	start := time.Now()
	bal, err := c.GetNativeBalance(ctx, address)
	metrics.Global.RecordRPCCall("btc", time.Since(start), err)
	if err != nil {
		return nil, err
	}
	return bal.Amount, nil
}

// GetTokenBalance is not supported for BTC.
func (c *Client) GetTokenBalance(_ context.Context, _, _ string) (*big.Int, error) {
	return nil, sigilerr.ErrNotSupported
}

// ListUTXOs returns unspent transaction outputs for an address. The Esplora UTXO
// endpoint omits scriptPubKey; since Sigil only tracks its own legacy P2PKH
// addresses, the locking script is rebuilt from the address at signing time.
func (c *Client) ListUTXOs(ctx context.Context, address string) ([]UTXO, error) {
	start := time.Now()
	result, err := c.doListUTXOs(ctx, address)
	metrics.Global.RecordRPCCall("btc", time.Since(start), err)
	return result, err
}

// doListUTXOs performs the actual UTXO listing.
//
//nolint:funcorder // Helper method grouped with its public caller
func (c *Client) doListUTXOs(ctx context.Context, address string) ([]UTXO, error) {
	if err := c.ValidateAddress(address); err != nil {
		return nil, err
	}

	raw, err := c.provider.AddressUTXOs(ctx, address)
	if err != nil {
		c.logError("utxo fetch failed for %s: %v", address, err)
		return nil, err
	}

	utxos := make([]UTXO, len(raw))
	for i, u := range raw {
		var confirmations uint32
		if u.Status.Confirmed {
			confirmations = 1
		}
		utxos[i] = UTXO{
			TxID:          u.TxID,
			Vout:          u.Vout,
			Amount:        u.Value,
			Address:       address,
			Confirmations: confirmations,
		}
	}

	return utxos, nil
}

// SelectUTXOs chooses UTXOs (largest-first) to fund a transaction. The fee is
// sat/vByte × estimated vsize; since only legacy P2PKH inputs are spent, vsize
// equals the serialized size. Change below the 546-satoshi dust limit is dropped.
//
//nolint:gocognit // Overflow checks add necessary complexity for fund safety
func (c *Client) SelectUTXOs(utxos []UTXO, amount, feeRate uint64) (selected []UTXO, change uint64, err error) {
	if len(utxos) == 0 {
		return nil, 0, ErrInsufficientFunds
	}

	// Sort UTXOs by amount (largest first) for simple selection. A stable sort
	// keeps equal-amount UTXOs in input order, giving deterministic, reproducible
	// coin selection.
	sorted := slices.Clone(utxos)
	slices.SortStableFunc(sorted, func(a, b UTXO) int {
		return cmp.Compare(b.Amount, a.Amount) // descending
	})

	dustLimit := chain.BTC.DustLimit()

	var total uint64
	var estimatedFee uint64
	for _, utxo := range sorted {
		selected = append(selected, utxo)

		sum, addErr := checkedAdd(total, utxo.Amount)
		if addErr != nil {
			return nil, 0, fmt.Errorf("UTXO sum: %w", addErr)
		}
		total = sum

		// Assume 2 outputs (recipient + change) while selecting.
		estimatedFee = EstimateFeeForTx(len(selected), 2, feeRate)
		target, targetErr := checkedAdd(amount, estimatedFee)
		if targetErr != nil {
			return nil, 0, fmt.Errorf("target amount: %w", targetErr)
		}
		if total >= target {
			change = total - target
			if change < dustLimit {
				change = 0
			}
			return selected, change, nil
		}
	}

	target, _ := checkedAdd(amount, estimatedFee)
	return nil, 0, fmt.Errorf("%w: need %d satoshis, have %d", ErrInsufficientFunds, target, total)
}

// EstimateFee estimates the fee for a typical 1-input, 2-output transaction using
// the client's fee strategy. Falls back to the default rate on API error.
func (c *Client) EstimateFee(ctx context.Context, _, _ string, _ *big.Int) (*big.Int, error) {
	rate := c.FeeRate(ctx)
	fee := EstimateFeeForTx(1, 2, rate)
	//nolint:gosec // fee is a small, non-negative satoshi amount
	return big.NewInt(int64(fee)), nil
}

// ValidateAddress checks if an address is valid for BTC on this client's network.
// Full validation (checksum + network-scoped version byte / HRP) is applied so a
// mainnet address is rejected on a testnet client and vice versa.
func (c *Client) ValidateAddress(address string) error {
	return ValidateBTCAddressForNetwork(address, c.network)
}

// FormatAmount converts a big.Int (satoshis) to a human-readable BTC string.
func (c *Client) FormatAmount(amount *big.Int) string {
	if amount == nil {
		return "0.00000000"
	}

	str := amount.String()
	for len(str) <= decimals {
		str = "0" + str
	}

	decimalPos := len(str) - decimals
	return str[:decimalPos] + "." + str[decimalPos:]
}

// ParseAmount converts a human-readable BTC string to big.Int (satoshis).
func (c *Client) ParseAmount(amount string) (*big.Int, error) {
	return chain.ParseDecimalAmount(amount, decimals, ErrInvalidAmount)
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
