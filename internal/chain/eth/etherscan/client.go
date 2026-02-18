// Package etherscan provides an Etherscan API client for balance queries.
package etherscan

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	"github.com/mrz1836/sigil/internal/chain"
	sigilerr "github.com/mrz1836/sigil/pkg/errors"
)

const (
	// DefaultBaseURL is the Etherscan API v2 base URL.
	DefaultBaseURL = "https://api.etherscan.io/v2"

	// DefaultChainID is the Ethereum mainnet chain ID for the Etherscan v2 API.
	DefaultChainID = "1"

	// httpTimeout is the default HTTP request timeout.
	httpTimeout = 30 * time.Second

	// maxResponseBody is the maximum response body size to read (1 MB).
	maxResponseBody = 1 << 20
)

// Sentinel errors for Etherscan API.
var (
	// ErrAPIKeyRequired indicates the Etherscan API key was not provided.
	ErrAPIKeyRequired = &sigilerr.SigilError{
		Code:     "ETHERSCAN_API_KEY_REQUIRED",
		Message:  "Etherscan API key is required",
		ExitCode: sigilerr.ExitInput,
	}

	// ErrAPIError indicates the Etherscan API returned an error response.
	ErrAPIError = &sigilerr.SigilError{
		Code:     "ETHERSCAN_API_ERROR",
		Message:  "Etherscan API returned an error",
		ExitCode: sigilerr.ExitGeneral,
	}

	// ErrRateLimited indicates the Etherscan API rate limit was exceeded.
	ErrRateLimited = &sigilerr.SigilError{
		Code:     "ETHERSCAN_RATE_LIMITED",
		Message:  "Etherscan API rate limit exceeded",
		ExitCode: sigilerr.ExitGeneral,
	}
)

// apiResponse represents the standard Etherscan API response.
type apiResponse struct {
	Status  string `json:"status"`  // "1" for success, "0" for error
	Message string `json:"message"` // "OK" or error message
	Result  string `json:"result"`  // Balance value as decimal string
}

// Client is an Etherscan API client for balance queries.
type Client struct {
	apiKey      string
	baseURL     string
	chainID     string
	httpClient  *http.Client
	rateLimiter *chain.RateLimiter
}

// ClientOptions configures the Etherscan client.
type ClientOptions struct {
	// BaseURL overrides the default Etherscan API URL (useful for testing).
	BaseURL string
	// HTTPClient overrides the default HTTP client.
	HTTPClient *http.Client
	// ChainID overrides the default chain ID (default "1" for Ethereum mainnet).
	ChainID string
}

// NewClient creates a new Etherscan API client.
func NewClient(apiKey string, opts *ClientOptions) (*Client, error) {
	if apiKey == "" {
		return nil, ErrAPIKeyRequired
	}

	c := &Client{
		apiKey:  apiKey,
		baseURL: DefaultBaseURL,
		chainID: DefaultChainID,
		httpClient: &http.Client{
			Timeout: httpTimeout,
			Transport: &http.Transport{
				TLSClientConfig: &tls.Config{MinVersion: tls.VersionTLS12},
			},
		},
		rateLimiter: chain.NewRateLimiter(5, 5), // 5 req/s, burst of 5 (Etherscan free tier)
	}

	if opts != nil {
		if opts.BaseURL != "" {
			c.baseURL = opts.BaseURL
		}
		if opts.HTTPClient != nil {
			c.httpClient = opts.HTTPClient
		}
		if opts.ChainID != "" {
			c.chainID = opts.ChainID
		}
	}

	return c, nil
}

// doRequest performs an HTTP GET request to the Etherscan API and returns the result string.
func (c *Client) doRequest(ctx context.Context, params url.Values) (string, error) {
	// Rate limit
	if err := c.rateLimiter.Wait(ctx, "etherscan"); err != nil {
		return "", fmt.Errorf("rate limiter: %w", err)
	}

	// Etherscan v2 API requires chainid on every request
	params.Set("chainid", c.chainID)

	reqURL := fmt.Sprintf("%s/api?%s", c.baseURL, params.Encode())

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return "", fmt.Errorf("creating request: %w", err)
	}

	// Send API key in header rather than URL query parameters to avoid
	// leaking it in server logs, proxy logs, and URL history.
	httpReq.Header.Set("Authorization", "Bearer "+c.apiKey)

	resp, err := c.httpClient.Do(httpReq) //nolint:gosec // G704: URL is constructed from validated config, not user input
	if err != nil {
		return "", fmt.Errorf("sending request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBody))
	if err != nil {
		return "", fmt.Errorf("reading response: %w", err)
	}

	// Handle HTTP-level rate limiting
	if resp.StatusCode == http.StatusTooManyRequests {
		return "", sigilerr.WithDetails(ErrRateLimited, map[string]string{
			"status": fmt.Sprintf("%d", resp.StatusCode),
		})
	}

	if resp.StatusCode != http.StatusOK {
		return "", sigilerr.WithDetails(ErrAPIError, map[string]string{
			"status": fmt.Sprintf("%d", resp.StatusCode),
			"body":   truncateBody(string(body), 512),
		})
	}

	var apiResp apiResponse
	if err := json.Unmarshal(body, &apiResp); err != nil {
		return "", fmt.Errorf("parsing response: %w", err)
	}

	if apiResp.Status != "1" {
		// Etherscan returns status "0" for errors.
		if apiResp.Result == "Max rate limit reached" {
			return "", ErrRateLimited
		}
		return "", sigilerr.WithDetails(ErrAPIError, map[string]string{
			"message": apiResp.Message,
			"result":  truncateBody(apiResp.Result, 256),
		})
	}

	return apiResp.Result, nil
}

// truncateBody truncates a string to maxLen characters.
func truncateBody(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
