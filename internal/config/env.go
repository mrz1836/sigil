package config

import (
	"os"
	"strconv"
	"strings"

	"github.com/mrz1836/go-sanitize"
)

// Environment variable names.
const (
	EnvHome         = "SIGIL_HOME"
	EnvETHRPC       = "SIGIL_ETH_RPC"
	EnvBSVAPIKey    = "SIGIL_BSV_API_KEY" // #nosec G101 -- false positive, this is a const name not a credential
	EnvOutputFormat = "SIGIL_OUTPUT_FORMAT"
	EnvVerbose      = "SIGIL_VERBOSE"
	EnvLogLevel     = "SIGIL_LOG_LEVEL"
	EnvNoColor      = "NO_COLOR"
	EnvSessionTTL   = "SIGIL_SESSION_TTL"
)

// ApplyEnvironment applies environment variable overrides to the configuration.
//
//nolint:gocognit,gocyclo // Environment variable overrides require sequential checks
func ApplyEnvironment(cfg *Config) {
	if v := os.Getenv(EnvHome); v != "" {
		cfg.Home = v
	}

	if v := os.Getenv(EnvETHRPC); v != "" {
		cfg.Networks.ETH.RPC = SanitizeURL(v)
	}

	if v := os.Getenv(EnvBSVAPIKey); v != "" {
		cfg.Networks.BSV.APIKey = v
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
func SanitizeURL(url string) string {
	return sanitize.URL(strings.TrimSpace(url))
}
