package btc

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"github.com/mrz1836/sigil/internal/chain"
	"github.com/mrz1836/sigil/internal/chain/httpx"
	sigilerr "github.com/mrz1836/sigil/pkg/errors"
)

const (
	// MempoolMainnetAPI is the mempool.space Esplora REST base URL for mainnet.
	MempoolMainnetAPI = "https://mempool.space/api"

	// MempoolTestnet4API is the mempool.space Esplora REST base URL for testnet4.
	MempoolTestnet4API = "https://mempool.space/testnet4/api"

	// maxResponseBody is the maximum response body size to read (4 MB). An
	// address UTXO response caps at 500 entries, comfortably within this bound.
	maxResponseBody = 4 << 20
)

// ErrTooManyUTXOs indicates the address exceeds the Esplora 500-UTXO/address cap.
// It is non-retryable; the fix is to consolidate UTXOs or spend from a wallet
// with funds spread across multiple addresses.
var ErrTooManyUTXOs = &sigilerr.SigilError{
	Code:     "BTC_TOO_MANY_UTXOS",
	Message:  "address has too many UTXOs (provider caps at 500 per address)",
	ExitCode: sigilerr.ExitGeneral,
}

// ChainStats holds Esplora funded/spent output sums for one context (confirmed
// chain or mempool). Balance in that context is FundedTxoSum - SpentTxoSum.
type ChainStats struct {
	FundedTxoSum int64 `json:"funded_txo_sum"`
	SpentTxoSum  int64 `json:"spent_txo_sum"`
	TxCount      int64 `json:"tx_count"`
}

// AddressStats is the Esplora GET /address/{address} response.
type AddressStats struct {
	Address      string     `json:"address"`
	ChainStats   ChainStats `json:"chain_stats"`
	MempoolStats ChainStats `json:"mempool_stats"`
}

// EsploraUTXOStatus is the confirmation status of an Esplora UTXO.
type EsploraUTXOStatus struct {
	Confirmed   bool  `json:"confirmed"`
	BlockHeight int64 `json:"block_height"`
}

// EsploraUTXO is one entry of the Esplora GET /address/{address}/utxo response.
// The endpoint omits scriptPubKey; Sigil rebuilds the P2PKH script from its own
// address at signing time.
type EsploraUTXO struct {
	TxID   string            `json:"txid"`
	Vout   uint32            `json:"vout"`
	Value  uint64            `json:"value"`
	Status EsploraUTXOStatus `json:"status"`
}

// FeeEstimates is the Esplora GET /v1/fees/recommended response (sat/vByte).
type FeeEstimates struct {
	FastestFee  float64 `json:"fastestFee"`
	HalfHourFee float64 `json:"halfHourFee"`
	HourFee     float64 `json:"hourFee"`
	EconomyFee  float64 `json:"economyFee"`
	MinimumFee  float64 `json:"minimumFee"`
}

// EsploraProvider is the narrow interface for the Esplora endpoints Sigil uses.
// It is the test seam (Pattern A): tests inject a func-field fake.
type EsploraProvider interface {
	AddressStats(ctx context.Context, address string) (*AddressStats, error)
	AddressUTXOs(ctx context.Context, address string) ([]EsploraUTXO, error)
	FeeEstimates(ctx context.Context) (*FeeEstimates, error)
}

// baseURLForNetwork returns the mempool.space Esplora base URL for the network.
func baseURLForNetwork(network Network) string {
	if network == NetworkTestnet {
		return MempoolTestnet4API
	}
	return MempoolMainnetAPI
}

// httpStatusError carries a non-2xx HTTP status so endpoint methods can classify
// it (e.g., 400 on the UTXO endpoint → ErrTooManyUTXOs). It is non-retryable.
type httpStatusError struct {
	Code int
	Body string
}

func (e *httpStatusError) Error() string {
	return fmt.Sprintf("esplora: status %d: %s", e.Code, e.Body)
}

// esploraHTTP is the concrete net/http Esplora provider (mempool.space). It
// mirrors the hand-rolled internal/chain/eth/etherscan client: no new deps,
// per-endpoint rate limiting, and exponential-backoff retry on transient errors.
type esploraHTTP struct {
	baseURL     string
	apiKey      string
	httpClient  *http.Client
	rateLimiter *chain.RateLimiter
	logger      Logger
}

// Compile-time interface check.
var _ EsploraProvider = (*esploraHTTP)(nil)

// newEsploraHTTP creates a concrete Esplora provider.
func newEsploraHTTP(baseURL, apiKey string, httpClient *http.Client, logger Logger) *esploraHTTP {
	return &esploraHTTP{
		baseURL:     baseURL,
		apiKey:      apiKey,
		httpClient:  httpClient,
		rateLimiter: chain.NewRateLimiter(5, 10),
		logger:      logger,
	}
}

// AddressStats fetches confirmed + mempool output sums for an address.
func (e *esploraHTTP) AddressStats(ctx context.Context, address string) (*AddressStats, error) {
	body, err := e.doGet(ctx, "/address/"+address)
	if err != nil {
		return nil, err
	}
	var stats AddressStats
	if uErr := json.Unmarshal(body, &stats); uErr != nil {
		return nil, fmt.Errorf("parsing address stats: %w", uErr)
	}
	return &stats, nil
}

// AddressUTXOs fetches the unspent outputs for an address. A 400 response means
// the address exceeds the provider's 500-UTXO cap → typed ErrTooManyUTXOs.
func (e *esploraHTTP) AddressUTXOs(ctx context.Context, address string) ([]EsploraUTXO, error) {
	body, err := e.doGet(ctx, "/address/"+address+"/utxo")
	if err != nil {
		var se *httpStatusError
		if errors.As(err, &se) && se.Code == http.StatusBadRequest {
			return nil, ErrTooManyUTXOs
		}
		return nil, err
	}
	var utxos []EsploraUTXO
	if uErr := json.Unmarshal(body, &utxos); uErr != nil {
		return nil, fmt.Errorf("parsing utxo response: %w", uErr)
	}
	return utxos, nil
}

// FeeEstimates fetches the recommended sat/vByte fee tiers.
func (e *esploraHTTP) FeeEstimates(ctx context.Context) (*FeeEstimates, error) {
	body, err := e.doGet(ctx, "/v1/fees/recommended")
	if err != nil {
		return nil, err
	}
	var fees FeeEstimates
	if uErr := json.Unmarshal(body, &fees); uErr != nil {
		return nil, fmt.Errorf("parsing fee estimates: %w", uErr)
	}
	return &fees, nil
}

// doGet performs a rate-limited, retryable HTTP GET against the Esplora base URL.
func (e *esploraHTTP) doGet(ctx context.Context, path string) ([]byte, error) {
	return chain.Retry(ctx, func() ([]byte, error) {
		return e.get(ctx, path)
	})
}

// get performs a single HTTP GET attempt, mapping status codes to the shared
// retryable/non-retryable error sentinels.
func (e *esploraHTTP) get(ctx context.Context, path string) ([]byte, error) {
	if err := e.rateLimiter.Wait(ctx, "esplora"); err != nil {
		return nil, fmt.Errorf("rate limiter: %w", err)
	}

	header := map[string]string{}
	if e.apiKey != "" {
		// Send the key in a header (not the URL) to avoid leaking it in logs.
		header["Authorization"] = "Bearer " + e.apiKey
	}

	resp, err := httpx.Do(ctx, e.httpClient, &httpx.Request{
		Method:       http.MethodGet,
		URL:          e.baseURL + path,
		Header:       header,
		MaxBodyBytes: maxResponseBody,
	})
	if err != nil {
		// Transport-level failures (build/send/read) are transient — retry.
		return nil, chain.WrapRetryable(fmt.Errorf("%w: %w", sigilerr.ErrNetworkError, err))
	}
	body := resp.Body

	switch {
	case resp.StatusCode == http.StatusOK:
		return body, nil
	case resp.StatusCode == http.StatusTooManyRequests:
		return nil, chain.ErrRateLimited
	case resp.StatusCode == http.StatusBadRequest:
		// Non-retryable; endpoint methods classify (e.g. too-many-UTXOs).
		return nil, &httpStatusError{Code: resp.StatusCode, Body: httpx.TruncateBody(string(body), maxTruncateLen)}
	case resp.StatusCode >= http.StatusInternalServerError:
		return nil, chain.WrapRetryable(fmt.Errorf("%w: status %d", sigilerr.ErrNetworkError, resp.StatusCode))
	default:
		return nil, fmt.Errorf("%w: status %d: %s", sigilerr.ErrNetworkError, resp.StatusCode, httpx.TruncateBody(string(body), maxTruncateLen))
	}
}

// maxTruncateLen bounds error-body text captured in wrapped errors.
const maxTruncateLen = 256
