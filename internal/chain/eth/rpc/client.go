// Package rpc provides a minimal JSON-RPC 2.0 client for Ethereum nodes.
package rpc

import (
	"bytes"
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"strings"
	"sync/atomic"

	sigilerr "github.com/mrz1836/sigil/pkg/errors"
)

var (
	// ErrRPCRequest indicates an RPC request failed.
	ErrRPCRequest = &sigilerr.SigilError{
		Code:     "RPC_REQUEST_FAILED",
		Message:  "RPC request failed",
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
	url        string
	httpClient *http.Client
	idCounter  atomic.Uint64
}

// NewClient creates a new RPC client.
func NewClient(url string) *Client {
	return &Client{
		url:        url,
		httpClient: &http.Client{},
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

	httpResp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("sending HTTP request: %w", err)
	}
	// Body.Close error is intentionally ignored as it only fails if the
	// connection is already broken, and there's no recovery action.
	defer func() { _ = httpResp.Body.Close() }()

	respBody, err := io.ReadAll(httpResp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading response body: %w", err)
	}

	var resp response
	if err := json.Unmarshal(respBody, &resp); err != nil {
		return nil, fmt.Errorf("unmarshaling response: %w", err)
	}

	if resp.Error != nil {
		return nil, resp.Error
	}

	return resp.Result, nil
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

// Close closes the client.
func (c *Client) Close() {
	// HTTP client doesn't need explicit closing, but we include this
	// for interface compatibility
}
