package config

import (
	"os"
	"strconv"
	"strings"
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
)

// ApplyEnvironment applies environment variable overrides to the configuration.
func ApplyEnvironment(cfg *Config) {
	if v := os.Getenv(EnvHome); v != "" {
		cfg.Home = v
	}

	if v := os.Getenv(EnvETHRPC); v != "" {
		cfg.Networks.ETH.RPC = v
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
