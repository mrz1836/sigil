package httpx_test

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mrz1836/sigil/internal/chain/httpx"
)

func TestTruncateBody(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		input  string
		maxLen int
		want   string
	}{
		{name: "short string unchanged", input: "hello", maxLen: 10, want: "hello"},
		{name: "exact length unchanged", input: "hello", maxLen: 5, want: "hello"},
		{name: "long string truncated", input: "hello world", maxLen: 5, want: "hello..."},
		{name: "empty string", input: "", maxLen: 5, want: ""},
		{name: "zero max length", input: "x", maxLen: 0, want: "..."},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.want, httpx.TruncateBody(tt.input, tt.maxLen))
		})
	}
}

func TestDoGET(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method)
		assert.Equal(t, "Bearer tok", r.Header.Get("Authorization"))
		w.Header().Set("X-Test", "yes")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("hello"))
	}))
	defer srv.Close()

	resp, err := httpx.Do(context.Background(), srv.Client(), &httpx.Request{
		Method:       http.MethodGet,
		URL:          srv.URL,
		Header:       map[string]string{"Authorization": "Bearer tok"},
		MaxBodyBytes: 1 << 20,
	})
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, "yes", resp.Header.Get("X-Test"))
	assert.Equal(t, "hello", string(resp.Body))
}

func TestDoPOSTSendsBody(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		assert.Equal(t, "ping", string(b))
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))
		w.WriteHeader(http.StatusTeapot)
		_, _ = w.Write([]byte("pong"))
	}))
	defer srv.Close()

	resp, err := httpx.Do(context.Background(), srv.Client(), &httpx.Request{
		Method:       http.MethodPost,
		URL:          srv.URL,
		Body:         []byte("ping"),
		Header:       map[string]string{"Content-Type": "application/json"},
		MaxBodyBytes: 1 << 20,
	})
	require.NoError(t, err)
	assert.Equal(t, http.StatusTeapot, resp.StatusCode)
	assert.Equal(t, "pong", string(resp.Body))
}

func TestDoCapsBody(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("0123456789"))
	}))
	defer srv.Close()

	resp, err := httpx.Do(context.Background(), srv.Client(), &httpx.Request{
		Method:       http.MethodGet,
		URL:          srv.URL,
		MaxBodyBytes: 4,
	})
	require.NoError(t, err)
	assert.Equal(t, "0123", string(resp.Body))
}

func TestDoInvalidMethodErrors(t *testing.T) {
	t.Parallel()

	resp, err := httpx.Do(context.Background(), http.DefaultClient, &httpx.Request{
		Method:       "bad method",
		URL:          "http://example.com",
		MaxBodyBytes: 1 << 20,
	})
	require.Error(t, err)
	assert.Nil(t, resp)
}
