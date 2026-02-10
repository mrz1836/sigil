package config

import (
	"errors"
	"fmt"
	"net/url"
	"os"
	"strconv"
	"strings"

	"github.com/mrz1836/go-sanitize"
)

// ErrInsecureRPCURL indicates an RPC URL is using plaintext HTTP.
var ErrInsecureRPCURL = errors.New("RPC URL must use HTTPS")

// Environment variable names.
const (
	EnvHome            = "SIGIL_HOME"
	EnvETHRPC          = "SIGIL_ETH_RPC"
	EnvETHProvider     = "SIGIL_ETH_PROVIDER"
	EnvEtherscanAPIKey = "ETHERSCAN_API_KEY"      // #nosec G101 -- false positive, this is a const name not a credential
	EnvBSVAPIKey       = "SIGIL_BSV_API_KEY"      // #nosec G101 -- false positive, this is a const name not a credential
	EnvWOCAPIKey       = "WHATS_ON_CHAIN_API_KEY" // #nosec G101 -- false positive, this is a const name not a credential
	EnvOutputFormat    = "SIGIL_OUTPUT_FORMAT"
	EnvVerbose         = "SIGIL_VERBOSE"
	EnvLogLevel        = "SIGIL_LOG_LEVEL"
	EnvNoColor         = "NO_COLOR"
	EnvSessionTTL      = "SIGIL_SESSION_TTL"
	EnvBSVFeeStrategy  = "SIGIL_BSV_FEE_STRATEGY"
	EnvBSVMinMiners    = "SIGIL_BSV_MIN_MINERS"
	EnvAgentToken      = "SIGIL_AGENT_TOKEN" //nolint:gosec // G101 -- false positive, this is a const name not a credential
	EnvAgentXpub       = "SIGIL_AGENT_XPUB"
)

// ApplyEnvironment applies environment variable overrides to the configuration.
//
//nolint:gocognit,gocyclo // Environment variable overrides require sequential checks
func ApplyEnvironment(cfg *Config) {
	if v := os.Getenv(EnvHome); v != "" {
		cfg.Home = v
	}

	if v := os.Getenv(EnvETHRPC); v != "" {
		sanitized := SanitizeURL(v)
		if err := ValidateRPCURL(sanitized); err != nil {
			// Log warning but still set the URL — validation errors are
			// surfaced at connection time via the ETH client.
			cfg.Warnings = append(cfg.Warnings, fmt.Sprintf("SIGIL_ETH_RPC: %v", err))
		}
		cfg.Networks.ETH.RPC = sanitized
	}

	if v := os.Getenv(EnvETHProvider); v != "" {
		v = strings.ToLower(strings.TrimSpace(v))
		if v == "rpc" || v == "etherscan" {
			cfg.Networks.ETH.Provider = v
		}
	}

	if v := os.Getenv(EnvEtherscanAPIKey); v != "" {
		cfg.Networks.ETH.EtherscanAPIKey = strings.TrimSpace(v)
	}

	if v := os.Getenv(EnvBSVAPIKey); v != "" {
		cfg.Networks.BSV.APIKey = v
	}

	// SIGIL_BSV_FEE_STRATEGY overrides fee strategy (silently ignore invalid values)
	if v := os.Getenv(EnvBSVFeeStrategy); v != "" {
		v = strings.ToLower(strings.TrimSpace(v))
		if v == "economy" || v == "normal" || v == "priority" {
			cfg.Fees.BSVFeeStrategy = v
		}
	}

	// SIGIL_BSV_MIN_MINERS overrides minimum miners for normal strategy
	if v := os.Getenv(EnvBSVMinMiners); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			cfg.Fees.BSVMinMiners = n
		}
	}

	// Fallback: accept the standard WhatsOnChain env var if sigil-specific one is not set
	if cfg.Networks.BSV.APIKey == "" {
		if v := os.Getenv(EnvWOCAPIKey); v != "" {
			cfg.Networks.BSV.APIKey = strings.TrimSpace(v)
		}
	}

	if v := os.Getenv(EnvOutputFormat); v != "" {
		cfg.Output.DefaultFormat = strings.ToLower(v)
	}

	if v := os.Getenv(EnvVerbose); v != "" {
		cfg.Output.Verbose = parseBool(v)
	}

	if v := os.Getenv(EnvLogLevel); v != "" {
		cfg.Logging.Level = strings.ToLower(v)
	}

	// NO_COLOR disables colored output
	if _, ok := os.LookupEnv(EnvNoColor); ok {
		cfg.Output.Color = "never"
	}

	// SIGIL_SESSION_TTL sets session timeout in minutes
	if v := os.Getenv(EnvSessionTTL); v != "" {
		if ttl, err := strconv.Atoi(v); err == nil && ttl > 0 {
			cfg.Security.SessionTTLMinutes = ttl
		}
	}
}

// parseBool parses a boolean string value.
func parseBool(s string) bool {
	s = strings.ToLower(strings.TrimSpace(s))
	if s == "1" || s == "true" || s == "yes" || s == "on" {
		return true
	}
	b, _ := strconv.ParseBool(s)
	return b
}

// SanitizeURL cleans a URL string by removing invalid characters and trimming whitespace.
// This is useful for cleaning user-provided RPC URLs that may contain copy-paste artifacts.
func SanitizeURL(rawURL string) string {
	return sanitize.URL(strings.TrimSpace(rawURL))
}

// ValidateRPCURL validates that an RPC URL uses HTTPS (or localhost for development).
// Returns an error if the URL scheme is not https and the host is not localhost.
func ValidateRPCURL(rawURL string) error {
	if rawURL == "" {
		return nil
	}

	u, err := url.Parse(rawURL)
	if err != nil {
		return fmt.Errorf("invalid RPC URL: %w", err)
	}

	if u.Scheme == "https" || u.Scheme == "wss" {
		return nil
	}

	// Allow plaintext for localhost/loopback development
	host := u.Hostname()
	if host == "localhost" || host == "127.0.0.1" || host == "::1" {
		return nil
	}

	return fmt.Errorf("%w (got %s://%s): plaintext HTTP exposes signed transactions to network attackers", ErrInsecureRPCURL, u.Scheme, u.Host)
}
