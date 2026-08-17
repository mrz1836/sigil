package config

import (
	"net/url"
	"strings"
	"testing"
)

// FuzzValidateRPCURL ensures the RPC-URL validator never panics and that any URL
// it accepts genuinely satisfies the security contract it enforces: an HTTPS/WSS
// scheme, or a loopback host for local development. A fuzz input that slips past
// this invariant would let a plaintext endpoint through — a real security hole.
func FuzzValidateRPCURL(f *testing.F) {
	seeds := []string{
		"", "https://rpc.example.com", "http://rpc.example.com",
		"http://localhost:8545", "http://127.0.0.1:8545", "wss://node.example/ws",
		"ftp://x", "://", "https://", "http://[::1]:8545", "not a url",
		"HTTPS://UP.CASE", "https://user:pass@host/path?q=1#frag", "ws://x",
	}
	for _, s := range seeds {
		f.Add(s)
	}

	f.Fuzz(func(t *testing.T, raw string) {
		err := ValidateRPCURL(raw)
		if raw == "" {
			if err != nil {
				t.Errorf("empty URL must be accepted, got %v", err)
			}
			return
		}
		if err != nil {
			return // rejected — acceptable for any input
		}
		// Accepted: re-derive the contract and confirm it actually holds.
		if !rpcURLMeetsSecurityContract(t, raw) {
			t.Errorf("ValidateRPCURL accepted insecure URL %q", raw)
		}
	})
}

// rpcURLMeetsSecurityContract re-derives the invariant ValidateRPCURL claims to
// enforce: an HTTPS/WSS scheme, or a loopback host. It fails the test if the URL
// ValidateRPCURL accepted cannot even be parsed.
func rpcURLMeetsSecurityContract(t *testing.T, raw string) bool {
	t.Helper()
	u, err := url.Parse(raw)
	if err != nil {
		t.Fatalf("ValidateRPCURL accepted an unparseable URL %q: %v", raw, err)
	}
	if u.Scheme == "https" || u.Scheme == "wss" {
		return true
	}
	host := u.Hostname()
	return host == "localhost" || host == "127.0.0.1" || host == "::1"
}

// FuzzSanitizeURL ensures URL sanitization never panics and never leaves
// surrounding whitespace on its output.
func FuzzSanitizeURL(f *testing.F) {
	seeds := []string{
		"", "  https://x.io  ", "https://x.io\n", "javascript:alert(1)",
		"  ", "\t\thttp://h\t", "https://xn--r8jz45g.example/path",
	}
	for _, s := range seeds {
		f.Add(s)
	}

	f.Fuzz(func(t *testing.T, raw string) {
		out := SanitizeURL(raw)
		if out != strings.TrimSpace(out) {
			t.Errorf("SanitizeURL(%q) = %q has surrounding whitespace", raw, out)
		}
	})
}
