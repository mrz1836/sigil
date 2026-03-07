package etherscan

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"

	sigilerr "github.com/mrz1836/sigil/pkg/errors"
)

// ErrBroadcastFailed indicates the Etherscan broadcast API returned an error.
var ErrBroadcastFailed = &sigilerr.SigilError{
	Code:     "ETHERSCAN_BROADCAST_FAILED",
	Message:  "Etherscan broadcast returned an error",
	ExitCode: sigilerr.ExitGeneral,
}

// proxyResponse represents the JSON-RPC style response from the Etherscan proxy module.
type proxyResponse struct {
	JSONRPC string         `json:"jsonrpc"`
	ID      int            `json:"id"`
	Result  string         `json:"result,omitempty"`
	Error   *proxyRPCError `json:"error,omitempty"`
}

// proxyRPCError represents a JSON-RPC error in an Etherscan proxy response.
type proxyRPCError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// BroadcastRawTransaction sends a signed raw transaction via the Etherscan proxy API.
// This implements the eth.Broadcaster interface.
func (c *Client) BroadcastRawTransaction(ctx context.Context, rawTx []byte) (string, error) {
	if err := c.rateLimiter.Wait(ctx, "etherscan"); err != nil {
		return "", fmt.Errorf("rate limiter: %w", err)
	}

	hexTx := "0x" + hex.EncodeToString(rawTx)

	params := url.Values{
		"module": {"proxy"},
		"action": {"eth_sendRawTransaction"},
		"hex":    {hexTx},
	}
	params.Set("chainid", c.chainID)

	reqURL := fmt.Sprintf("%s/api?%s", c.baseURL, params.Encode())

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return "", fmt.Errorf("creating request: %w", err)
	}
	httpReq.Header.Set("Authorization", "Bearer "+c.apiKey)

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return "", fmt.Errorf("sending request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBody))
	if err != nil {
		return "", fmt.Errorf("reading response: %w", err)
	}

	if resp.StatusCode == http.StatusTooManyRequests {
		return "", ErrRateLimited
	}

	if resp.StatusCode != http.StatusOK {
		return "", sigilerr.WithDetails(ErrAPIError, map[string]string{
			"status": fmt.Sprintf("%d", resp.StatusCode),
			"body":   truncateBody(string(body), 512),
		})
	}

	var proxyResp proxyResponse
	if err := json.Unmarshal(body, &proxyResp); err != nil {
		return "", fmt.Errorf("parsing proxy response: %w", err)
	}

	if proxyResp.Error != nil {
		return "", sigilerr.WithDetails(ErrBroadcastFailed, map[string]string{
			"code":    fmt.Sprintf("%d", proxyResp.Error.Code),
			"message": proxyResp.Error.Message,
		})
	}

	if proxyResp.Result == "" {
		return "", sigilerr.WithDetails(ErrBroadcastFailed, map[string]string{
			"message": "empty result from proxy",
		})
	}

	return proxyResp.Result, nil
}
