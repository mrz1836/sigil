// Package rpc provides a minimal JSON-RPC 2.0 client for Ethereum nodes.
package rpc

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"github.com/mrz1836/sigil/internal/chain"
	"github.com/mrz1836/sigil/internal/metrics"
	sigilerr "github.com/mrz1836/sigil/pkg/errors"
)

const maxResponseBody = 10 << 20 // 10 MB

var (
	// ErrRPCRequest indicates an RPC request failed.
	ErrRPCRequest = &sigilerr.SigilError{
		Code:     "RPC_REQUEST_FAILED",
		Message:  "RPC request failed",
		ExitCode: sigilerr.ExitGeneral,
	}

	// ErrRPCRateLimited indicates the RPC endpoint rate limited the request.
	ErrRPCRateLimited = &sigilerr.SigilError{
		Code:     "RATE_LIMITED",
		Message:  "rate limited",
		ExitCode: sigilerr.ExitGeneral,
	}

	// ErrRPCRetryable indicates a retryable RPC error (e.g., 5xx).
	ErrRPCRetryable = &sigilerr.SigilError{
		Code:     "RETRYABLE_ERROR",
		Message:  "retryable error",
		ExitCode: sigilerr.ExitGeneral,
	}

	// ErrRPCTimeout indicates the RPC request timed out.
	ErrRPCTimeout = &sigilerr.SigilError{
		Code:     "TIMEOUT",
		Message:  "operation timed out",
		ExitCode: sigilerr.ExitGeneral,
	}

	// ErrRPCResponse indicates an invalid RPC response.
	ErrRPCResponse = &sigilerr.SigilError{
		Code:     "RPC_INVALID_RESPONSE",
		Message:  "invalid RPC response",
		ExitCode: sigilerr.ExitGeneral,
	}

	// ErrNilResponse indicates a nil result from the RPC.
	ErrNilResponse = &sigilerr.SigilError{
		Code:     "RPC_NIL_RESPONSE",
		Message:  "nil RPC response",
		ExitCode: sigilerr.ExitGeneral,
	}

	// ErrInvalidHexNumber indicates an invalid hex number.
	ErrInvalidHexNumber = &sigilerr.SigilError{
		Code:     "RPC_INVALID_HEX",
		Message:  "invalid hex number",
		ExitCode: sigilerr.ExitInput,
	}
)

// Client is a minimal Ethereum JSON-RPC client.
type Client struct {
	url         string
	httpClient  *http.Client
	idCounter   atomic.Uint64
	rateLimiter *chain.RateLimiter
}

// ClientOptions configures optional behavior for the RPC client.
type ClientOptions struct {
	// Transport overrides the default HTTP transport. Useful for sharing
	// a transport across multiple clients (e.g., primary and fallback RPCs).
	Transport *http.Transport
}

// NewClient creates a new RPC client with connection pooling.
func NewClient(url string) *Client {
	return NewClientWithOptions(url, nil)
}

// NewDefaultTransport creates a new HTTP transport with secure defaults.
// Configures connection pooling and requires TLS 1.2+ for HTTPS connections.
func NewDefaultTransport() *http.Transport {
	return &http.Transport{
		MaxIdleConns:          100,
		MaxIdleConnsPerHost:   10,
		MaxConnsPerHost:       20,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   15 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
		TLSClientConfig:       &tls.Config{MinVersion: tls.VersionTLS12},
	}
}

// NewClientWithOptions creates a new RPC client with the given options.
func NewClientWithOptions(url string, opts *ClientOptions) *Client {
	var transport *http.Transport
	if opts != nil && opts.Transport != nil {
		transport = opts.Transport
	} else {
		transport = NewDefaultTransport()
	}
	return &Client{
		url: url,
		httpClient: &http.Client{
			Transport: transport,
			Timeout:   45 * time.Second,
		},
		rateLimiter: chain.DefaultRateLimiter(),
	}
}

// request represents a JSON-RPC 2.0 request.
type request struct {
	JSONRPC string `json:"jsonrpc"`
	Method  string `json:"method"`
	Params  []any  `json:"params"`
	ID      uint64 `json:"id"`
}

// response represents a JSON-RPC 2.0 response.
type response struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      uint64          `json:"id"`
	Result  json.RawMessage `json:"result"`
	Error   *rpcError       `json:"error,omitempty"`
}

// rpcError represents a JSON-RPC error.
type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

func (e *rpcError) Error() string {
	return fmt.Sprintf("RPC error %d: %s", e.Code, e.Message)
}

// Call performs a JSON-RPC call.
func (c *Client) Call(ctx context.Context, method string, params ...any) (json.RawMessage, error) {
	start := time.Now()
	result, err := c.callInternal(ctx, method, params...)
	metrics.Global.RecordRPCCall("eth", time.Since(start), err)
	return result, err
}

// ChainID returns the chain ID.
func (c *Client) ChainID(ctx context.Context) (*big.Int, error) {
	result, err := c.Call(ctx, "eth_chainId")
	if err != nil {
		return nil, err
	}

	var hexVal string
	if err := json.Unmarshal(result, &hexVal); err != nil {
		return nil, fmt.Errorf("parsing chain ID: %w", err)
	}

	return parseHexBigInt(hexVal)
}

// GetBalance returns the balance of an address in wei.
func (c *Client) GetBalance(ctx context.Context, address, block string) (*big.Int, error) {
	if block == "" {
		block = "latest"
	}

	result, err := c.Call(ctx, "eth_getBalance", address, block)
	if err != nil {
		return nil, err
	}

	var hexVal string
	if err := json.Unmarshal(result, &hexVal); err != nil {
		return nil, fmt.Errorf("parsing balance: %w", err)
	}

	return parseHexBigInt(hexVal)
}

// GetTransactionCount returns the nonce for an address.
func (c *Client) GetTransactionCount(ctx context.Context, address, block string) (uint64, error) {
	if block == "" {
		block = "pending"
	}

	result, err := c.Call(ctx, "eth_getTransactionCount", address, block)
	if err != nil {
		return 0, err
	}

	var hexVal string
	if unmarshalErr := json.Unmarshal(result, &hexVal); unmarshalErr != nil {
		return 0, fmt.Errorf("parsing nonce: %w", unmarshalErr)
	}

	n, err := parseHexBigInt(hexVal)
	if err != nil {
		return 0, err
	}

	return n.Uint64(), nil
}

// GasPrice returns the current gas price in wei.
func (c *Client) GasPrice(ctx context.Context) (*big.Int, error) {
	result, err := c.Call(ctx, "eth_gasPrice")
	if err != nil {
		return nil, err
	}

	var hexVal string
	if err := json.Unmarshal(result, &hexVal); err != nil {
		return nil, fmt.Errorf("parsing gas price: %w", err)
	}

	return parseHexBigInt(hexVal)
}

// CallMsg represents the parameters for eth_call.
type CallMsg struct {
	From  string   `json:"from,omitempty"`
	To    string   `json:"to"`
	Gas   uint64   `json:"gas,omitempty"`
	Value *big.Int `json:"value,omitempty"`
	Data  []byte   `json:"data,omitempty"`
}

// MarshalJSON implements custom JSON marshaling for CallMsg.
func (m CallMsg) MarshalJSON() ([]byte, error) {
	type callMsgJSON struct {
		From  string `json:"from,omitempty"`
		To    string `json:"to"`
		Gas   string `json:"gas,omitempty"`
		Value string `json:"value,omitempty"`
		Data  string `json:"data,omitempty"`
	}

	msg := callMsgJSON{
		From: m.From,
		To:   m.To,
	}

	if m.Gas > 0 {
		msg.Gas = fmt.Sprintf("0x%x", m.Gas)
	}
	if m.Value != nil && m.Value.Sign() > 0 {
		msg.Value = "0x" + m.Value.Text(16)
	}
	if len(m.Data) > 0 {
		msg.Data = "0x" + hex.EncodeToString(m.Data)
	}

	return json.Marshal(msg)
}

// EthCall performs an eth_call.
func (c *Client) EthCall(ctx context.Context, msg CallMsg, block string) ([]byte, error) {
	if block == "" {
		block = "latest"
	}

	result, err := c.Call(ctx, "eth_call", msg, block)
	if err != nil {
		return nil, err
	}

	var hexVal string
	if err := json.Unmarshal(result, &hexVal); err != nil {
		return nil, fmt.Errorf("parsing call result: %w", err)
	}

	return parseHexBytes(hexVal)
}

// EstimateGas estimates the gas needed for a transaction.
func (c *Client) EstimateGas(ctx context.Context, msg CallMsg) (uint64, error) {
	result, err := c.Call(ctx, "eth_estimateGas", msg)
	if err != nil {
		return 0, err
	}

	var hexVal string
	if unmarshalErr := json.Unmarshal(result, &hexVal); unmarshalErr != nil {
		return 0, fmt.Errorf("parsing gas estimate: %w", unmarshalErr)
	}

	n, err := parseHexBigInt(hexVal)
	if err != nil {
		return 0, err
	}

	return n.Uint64(), nil
}

// SendRawTransaction sends a signed transaction.
// Returns the transaction hash.
func (c *Client) SendRawTransaction(ctx context.Context, signedTx []byte) (string, error) {
	hexTx := "0x" + hex.EncodeToString(signedTx)

	result, err := c.Call(ctx, "eth_sendRawTransaction", hexTx)
	if err != nil {
		return "", err
	}

	var txHash string
	if err := json.Unmarshal(result, &txHash); err != nil {
		return "", fmt.Errorf("parsing tx hash: %w", err)
	}

	return txHash, nil
}

// parseHexBigInt parses a hex string (with or without 0x prefix) to big.Int.
func parseHexBigInt(s string) (*big.Int, error) {
	s = strings.TrimPrefix(s, "0x")
	if s == "" {
		return big.NewInt(0), nil
	}

	n := new(big.Int)
	if _, ok := n.SetString(s, 16); !ok {
		return nil, ErrInvalidHexNumber
	}

	return n, nil
}

// parseHexBytes parses a hex string to bytes.
func parseHexBytes(s string) ([]byte, error) {
	s = strings.TrimPrefix(s, "0x")
	if s == "" {
		return []byte{}, nil
	}
	return hex.DecodeString(s)
}

// Close closes the client and releases idle connections.
func (c *Client) Close() {
	if t, ok := c.httpClient.Transport.(*http.Transport); ok {
		t.CloseIdleConnections()
	}
}

// callInternal performs the actual JSON-RPC call.
//
//nolint:gocognit,gocyclo // Rate limiting and error handling add necessary branches
func (c *Client) callInternal(ctx context.Context, method string, params ...any) (json.RawMessage, error) {
	if c.rateLimiter != nil {
		if err := c.rateLimiter.Wait(ctx, c.url); err != nil {
			return nil, fmt.Errorf("rate limiter: %w", err)
		}
	}

	if params == nil {
		params = []any{}
	}

	req := request{
		JSONRPC: "2.0",
		Method:  method,
		Params:  params,
		ID:      c.idCounter.Add(1),
	}

	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshaling request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("creating HTTP request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	httpResp, err := c.httpClient.Do(httpReq) //nolint:gosec // G704: URL is constructed from validated config, not user input
	if err != nil {
		return nil, fmt.Errorf("sending HTTP request: %w", err)
	}
	// Body.Close error is intentionally ignored as it only fails if the
	// connection is already broken, and there's no recovery action.
	defer func() { _ = httpResp.Body.Close() }()

	respBody, err := io.ReadAll(io.LimitReader(httpResp.Body, maxResponseBody))
	if err != nil {
		return nil, fmt.Errorf("reading response body: %w", err)
	}

	if httpResp.StatusCode != http.StatusOK {
		return nil, c.handleHTTPError(httpResp, respBody)
	}

	var resp response
	if err := json.Unmarshal(respBody, &resp); err != nil {
		return nil, fmt.Errorf("unmarshaling response: %w", err)
	}

	if resp.Error != nil {
		return nil, sigilerr.WithDetails(ErrRPCRequest, map[string]string{
			"rpc_code":    strconv.Itoa(resp.Error.Code),
			"rpc_message": resp.Error.Message,
		})
	}

	return resp.Result, nil
}

// handleHTTPError creates an appropriate error based on HTTP status code and response.
func (c *Client) handleHTTPError(httpResp *http.Response, respBody []byte) error {
	details := map[string]string{
		"status": strconv.Itoa(httpResp.StatusCode),
	}
	if retryAfter := httpResp.Header.Get("Retry-After"); retryAfter != "" {
		details["retry_after"] = retryAfter
	}

	body := strings.TrimSpace(string(respBody))
	if body != "" {
		if len(body) > 512 {
			body = body[:512] + "..."
		}
		details["body"] = body
	}

	switch {
	case httpResp.StatusCode == http.StatusTooManyRequests:
		return sigilerr.WithDetails(ErrRPCRateLimited, details)
	case httpResp.StatusCode == http.StatusRequestTimeout || httpResp.StatusCode == http.StatusGatewayTimeout:
		return sigilerr.WithDetails(ErrRPCTimeout, details)
	case httpResp.StatusCode >= http.StatusInternalServerError:
		return sigilerr.WithDetails(ErrRPCRetryable, details)
	default:
		return sigilerr.WithDetails(ErrRPCRequest, details)
	}
}
