// Package httpx centralizes the small HTTP plumbing shared by Sigil's JSON API
// clients (eth/rpc, btc/esplora, eth/etherscan): building a context request,
// sending it, and reading a size-capped response body. It deliberately does NOT
// classify status codes or map errors — each client keeps its own taxonomy and
// retry semantics; Do simply returns the status and body (or a transport error).
package httpx

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
)

// Request describes a single HTTP request for Do.
type Request struct {
	// Method is the HTTP method (e.g. http.MethodGet, http.MethodPost).
	Method string
	// URL is the fully-formed request URL.
	URL string
	// Body is the request body, or nil for bodyless requests (e.g. GET).
	Body []byte
	// Header holds optional request headers (e.g. Content-Type, Authorization).
	Header map[string]string
	// MaxBodyBytes caps how many response-body bytes are read (defense against
	// unbounded responses). Must be positive.
	MaxBodyBytes int64
}

// Response is the outcome of a successful round trip: the HTTP status code,
// response headers, and the response body, read and capped at Request.MaxBodyBytes.
// A non-2xx status is not an error here — the caller inspects StatusCode and
// applies its own error mapping.
type Response struct {
	StatusCode int
	Header     http.Header
	Body       []byte
}

// Do performs a single HTTP request and reads the (capped) response body,
// centralizing the NewRequestWithContext + client.Do + io.LimitReader boilerplate.
// It returns an error only for transport-level failures (request build, send, or
// read); the caller wraps that error in its own taxonomy and classifies the
// returned status code itself.
func Do(ctx context.Context, client *http.Client, req *Request) (*Response, error) {
	var body io.Reader
	if req.Body != nil {
		body = bytes.NewReader(req.Body)
	}

	httpReq, err := http.NewRequestWithContext(ctx, req.Method, req.URL, body)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}
	for key, value := range req.Header {
		httpReq.Header.Set(key, value)
	}

	resp, err := client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("sending request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	respBody, err := io.ReadAll(io.LimitReader(resp.Body, req.MaxBodyBytes))
	if err != nil {
		return nil, fmt.Errorf("reading response: %w", err)
	}

	return &Response{StatusCode: resp.StatusCode, Header: resp.Header, Body: respBody}, nil
}

// TruncateBody truncates s to maxLen characters, appending an ellipsis when it
// was truncated. It bounds response-body text captured in wrapped errors.
func TruncateBody(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
