package btc

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/mrz1836/sigil/internal/chain"
	sigilerr "github.com/mrz1836/sigil/pkg/errors"
)

const (
	// BlockstreamMainnetURL is the Blockstream Esplora API base URL (mainnet only;
	// Blockstream's testnet is testnet3, a different chain than mempool testnet4).
	BlockstreamMainnetURL = "https://blockstream.info/api"

	// maxBroadcastResponseBody caps broadcast response reads.
	maxBroadcastResponseBody int64 = 1 << 20
)

// errAlreadyKnown indicates the network already has the transaction. The caller
// treats this as success and returns the locally-computed txid.
var errAlreadyKnown = errors.New("transaction already known to network")

// isValidTxID reports whether s is a 64-character hex transaction ID.
func isValidTxID(s string) bool {
	return chain.IsValidTxID(s)
}

// extractTxID returns the first 64-hex substring of s, or "" if none.
func extractTxID(s string) string {
	return chain.ExtractTxID(s)
}

// isAlreadyBroadcasted reports whether an error/response body indicates the
// transaction is already known to the network (Bitcoin Core / Esplora variants).
func isAlreadyBroadcasted(responseText string) bool {
	lower := strings.ToLower(responseText)
	return strings.Contains(lower, "txn-already-known") ||
		strings.Contains(lower, "already in block chain") ||
		strings.Contains(lower, "already in the mempool") ||
		strings.Contains(lower, "already in mempool") ||
		strings.Contains(lower, "already known") ||
		strings.Contains(lower, "transaction already exists")
}

// Broadcaster broadcasts a raw transaction hex to the network.
type Broadcaster interface {
	// Broadcast sends a raw transaction hex and returns the txid.
	Broadcast(ctx context.Context, rawTxHex string) (string, error)
	// Name returns the broadcaster name for logging.
	Name() string
}

// esploraBroadcast performs the shared Esplora POST /tx contract: the request
// body is the raw tx hex (text/plain) and a 200 response body is the txid.
func esploraBroadcast(ctx context.Context, httpClient *http.Client, baseURL, rawTxHex string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, baseURL+"/tx", strings.NewReader(rawTxHex))
	if err != nil {
		return "", fmt.Errorf("creating request: %w", err)
	}
	req.Header.Set("Content-Type", "text/plain")

	resp, err := httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("%w: %w", sigilerr.ErrNetworkError, err)
	}
	defer func() { _ = resp.Body.Close() }()

	body, _ := io.ReadAll(io.LimitReader(resp.Body, maxBroadcastResponseBody))
	return interpretBroadcastResponse(resp.StatusCode, strings.TrimSpace(string(body)))
}

// interpretBroadcastResponse maps an Esplora POST /tx response to a txid or error.
// A 200 body is the txid; a non-200 body is checked for already-known variants
// (idempotent success) before being surfaced as a broadcast failure.
func interpretBroadcastResponse(statusCode int, text string) (string, error) {
	if statusCode != http.StatusOK {
		if isAlreadyBroadcasted(text) {
			if extracted := extractTxID(text); extracted != "" {
				return extracted, nil
			}
			return "", errAlreadyKnown
		}
		return "", fmt.Errorf("%w: status %d: %s", ErrBroadcastFailed, statusCode, truncateBody(text))
	}

	if isValidTxID(text) {
		return text, nil
	}
	// Some deployments echo extra whitespace/quotes — try to salvage a txid.
	if extracted := extractTxID(text); extracted != "" {
		return extracted, nil
	}
	return "", fmt.Errorf("%w: unexpected response: %s", ErrBroadcastFailed, truncateBody(text))
}

// MempoolBroadcaster broadcasts via the mempool.space Esplora POST /tx endpoint.
// This is the primary broadcaster and works on both mainnet and testnet4.
type MempoolBroadcaster struct {
	// BaseURL is the Esplora API base (e.g. "https://mempool.space/api").
	BaseURL string
	// httpClient is the HTTP client used for broadcast requests.
	httpClient *http.Client
}

// Name returns the broadcaster name.
func (m *MempoolBroadcaster) Name() string { return "mempool.space" }

// Broadcast sends a raw transaction via mempool.space.
func (m *MempoolBroadcaster) Broadcast(ctx context.Context, rawTxHex string) (string, error) {
	return esploraBroadcast(ctx, m.httpClient, m.BaseURL, rawTxHex)
}

// BlockstreamBroadcaster broadcasts via the Blockstream Esplora POST /tx endpoint.
// Mainnet-only fallback (same API contract as mempool.space).
type BlockstreamBroadcaster struct {
	// BaseURL is the Blockstream Esplora API base.
	BaseURL string
	// httpClient is the HTTP client used for broadcast requests.
	httpClient *http.Client
}

// Name returns the broadcaster name.
func (b *BlockstreamBroadcaster) Name() string { return "blockstream.info" }

// Broadcast sends a raw transaction via Blockstream.
func (b *BlockstreamBroadcaster) Broadcast(ctx context.Context, rawTxHex string) (string, error) {
	return esploraBroadcast(ctx, b.httpClient, b.BaseURL, rawTxHex)
}
