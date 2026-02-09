package bsv

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	sigilerr "github.com/mrz1836/sigil/pkg/errors"
)

const (
	// DefaultMaxResponseBody is the maximum response body size for broadcast endpoints.
	// Set to 10 MB to accommodate large BSV transactions.
	DefaultMaxResponseBody int64 = 10 << 20

	// GorillaPoolARCURL is the default GorillaPool ARC broadcast endpoint.
	GorillaPoolARCURL = "https://arc.gorillapool.io"

	// ARC custom HTTP status codes.
	arcStatusNotExtendedFormat             = 460
	arcStatusFeeTooLow                     = 465
	arcStatusCumulativeFeeValidationFailed = 473
)

// Broadcaster defines the interface for broadcasting raw transactions.
type Broadcaster interface {
	// Broadcast sends a raw transaction hex to the network and returns the txid.
	Broadcast(ctx context.Context, rawTxHex string) (string, error)
	// Name returns the broadcaster name for logging.
	Name() string
}

// WOCSDKBroadcaster broadcasts via the WhatsOnChain SDK.
type WOCSDKBroadcaster struct {
	woc WOCClient
}

// Name returns the broadcaster name.
func (w *WOCSDKBroadcaster) Name() string { return "whatsonchain" }

// Broadcast sends a raw transaction via the WhatsOnChain SDK.
func (w *WOCSDKBroadcaster) Broadcast(ctx context.Context, rawTxHex string) (string, error) {
	txid, err := w.woc.BroadcastTx(ctx, rawTxHex)
	if err != nil {
		// Check for "already in mempool" in the error message.
		if isAlreadyBroadcasted(err.Error()) {
			return err.Error(), nil
		}
		return "", fmt.Errorf("%w: %w", ErrBroadcastFailed, err)
	}

	if txid == "" {
		return "", fmt.Errorf("%w: empty txid in response", ErrBroadcastFailed)
	}

	return txid, nil
}

// isAlreadyBroadcasted checks if the error response indicates the transaction
// is already known to the network. Uses case-insensitive matching per
// go-wallet-toolbox/pkg/services/internal/whatsonchain/broadcast.go.
func isAlreadyBroadcasted(responseText string) bool {
	lower := strings.ToLower(responseText)
	return strings.Contains(lower, "already in mempool") ||
		strings.Contains(lower, "already in the mempool") ||
		strings.Contains(lower, "txn-already-known")
}

// GorillaPoolARCBroadcaster broadcasts via the GorillaPool ARC API.
//
// API: POST {BaseURL}/v1/tx
// Request: {"rawTx": "<hex>"}
// Response: JSON with txid, txStatus, etc. on success; APIError on failure.
type GorillaPoolARCBroadcaster struct {
	// BaseURL is the ARC API base URL (e.g. "https://arc.gorillapool.io").
	BaseURL string
	// httpClient is the HTTP client used for ARC requests.
	httpClient *http.Client
}

// Name returns the broadcaster name.
func (g *GorillaPoolARCBroadcaster) Name() string { return "gorillapool" }

// arcTXInfo represents the ARC transaction response.
// Matches go-wallet-toolbox/pkg/services/internal/arc/tx_info.go.
type arcTXInfo struct {
	TxID      string `json:"txid"`
	TXStatus  string `json:"txStatus"`
	ExtraInfo string `json:"extraInfo"`
}

// arcAPIError represents an ARC error response.
// Matches go-wallet-toolbox/pkg/services/internal/arc/arc_error.go.
type arcAPIError struct {
	Type      string `json:"type"`
	Title     string `json:"title"`
	Status    int    `json:"status"`
	Detail    string `json:"detail"`
	ExtraInfo string `json:"extraInfo"`
}

// Error implements the error interface.
func (e *arcAPIError) Error() string {
	if e.Detail != "" {
		return fmt.Sprintf("arc: %s (status %d: %s)", e.Detail, e.Status, e.Title)
	}
	return fmt.Sprintf("arc: %s (status %d)", e.Title, e.Status)
}

// Broadcast sends a raw transaction via GorillaPool ARC.
func (g *GorillaPoolARCBroadcaster) Broadcast(ctx context.Context, rawTxHex string) (string, error) {
	url := g.BaseURL + "/v1/tx"

	payload := struct {
		RawTx string `json:"rawTx"`
	}{RawTx: rawTxHex}

	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("marshaling payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(payloadBytes))
	if err != nil {
		return "", fmt.Errorf("creating request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := g.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("%w: %w", sigilerr.ErrNetworkError, err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return "", g.handleErrorResponse(resp)
	}

	var result arcTXInfo
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("decoding response: %w", err)
	}

	if result.TxID == "" {
		return "", fmt.Errorf("%w: empty txid in response", ErrBroadcastFailed)
	}

	return result.TxID, nil
}

// handleErrorResponse parses an ARC error response and returns an appropriate error.
func (g *GorillaPoolARCBroadcaster) handleErrorResponse(resp *http.Response) error {
	body, _ := io.ReadAll(io.LimitReader(resp.Body, DefaultMaxResponseBody))

	var apiErr arcAPIError
	if err := json.Unmarshal(body, &apiErr); err != nil {
		// Could not parse JSON error — return raw body.
		return fmt.Errorf("%w: status %d, body: %s", ErrBroadcastFailed, resp.StatusCode, string(body))
	}

	switch resp.StatusCode {
	case http.StatusUnauthorized, http.StatusForbidden, http.StatusNotFound:
		return fmt.Errorf("%w: unauthorized: %w", ErrBroadcastFailed, &apiErr)
	case arcStatusNotExtendedFormat:
		return fmt.Errorf("%w: extended format required: %w", ErrBroadcastFailed, &apiErr)
	case arcStatusFeeTooLow, arcStatusCumulativeFeeValidationFailed:
		return fmt.Errorf("%w: fee too low: %w", ErrBroadcastFailed, &apiErr)
	default:
		return fmt.Errorf("%w: %w", ErrBroadcastFailed, &apiErr)
	}
}
