package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/mrz1836/sigil/internal/config"
	"github.com/mrz1836/sigil/internal/output"
	sigilerr "github.com/mrz1836/sigil/pkg/errors"
)

// configCmd is the parent command for configuration operations.
//
//nolint:gochecknoglobals // Cobra CLI pattern requires package-level command variables
var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Manage configuration",
	Long:  `View and modify Sigil configuration settings.`,
}

// configInitCmd initializes the configuration.
//
//nolint:gochecknoglobals // Cobra CLI pattern requires package-level command variables
var configInitCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize configuration",
	Long: `Create a default configuration file at ~/.sigil/config.yaml.

If a configuration file already exists, this command will not overwrite it
unless --force is specified.

Example:
  sigil config init
  sigil config init --force`,
	RunE: runConfigInit,
}

// configShowCmd shows the current configuration.
//
//nolint:gochecknoglobals // Cobra CLI pattern requires package-level command variables
var configShowCmd = &cobra.Command{
	Use:   "show",
	Short: "Show current configuration",
	Long: `Display the current configuration settings.

Example:
  sigil config show
  sigil config show -o json`,
	RunE: runConfigShow,
}

// configGetCmd gets a specific configuration value.
//
//nolint:gochecknoglobals // Cobra CLI pattern requires package-level command variables
var configGetCmd = &cobra.Command{
	Use:   "get <path>",
	Short: "Get a configuration value",
	Long: `Get a specific configuration value by its path.

The path uses dot notation to navigate the configuration tree.

Examples:
  sigil config get networks.eth.rpc
  sigil config get output.default_format
  sigil config get logging.level`,
	Args: cobra.ExactArgs(1),
	RunE: runConfigGet,
}

// configSetCmd sets a configuration value.
//
//nolint:gochecknoglobals // Cobra CLI pattern requires package-level command variables
var configSetCmd = &cobra.Command{
	Use:   "set <path> <value>",
	Short: "Set a configuration value",
	Long: `Set a specific configuration value by its path.

The path uses dot notation to navigate the configuration tree.
The configuration file will be updated immediately.

Examples:
  sigil config set networks.eth.rpc https://mainnet.infura.io/v3/YOUR_KEY
  sigil config set output.default_format json
  sigil config set logging.level debug`,
	Args: cobra.ExactArgs(2),
	RunE: runConfigSet,
}

//nolint:gochecknoglobals // Cobra CLI pattern requires package-level flag variables
var configForce bool

//nolint:gochecknoinits // Cobra CLI pattern requires init for command registration
func init() {
	rootCmd.AddCommand(configCmd)
	configCmd.AddCommand(configInitCmd)
	configCmd.AddCommand(configShowCmd)
	configCmd.AddCommand(configGetCmd)
	configCmd.AddCommand(configSetCmd)

	configInitCmd.Flags().BoolVar(&configForce, "force", false, "overwrite existing configuration")
}

func runConfigInit(cmd *cobra.Command, _ []string) error {
	configPath := config.Path(cfg.Home)

	// Check if config already exists
	if _, err := os.Stat(configPath); err == nil && !configForce {
		return sigilerr.WithSuggestion(
			sigilerr.ErrGeneral,
			fmt.Sprintf("configuration already exists at %s. Use --force to overwrite.", configPath),
		)
	}

	// Ensure directory exists
	configDir := filepath.Dir(configPath)
	if err := os.MkdirAll(configDir, 0o750); err != nil {
		return fmt.Errorf("creating config directory: %w", err)
	}

	// Create default config
	defaultCfg := config.Defaults()
	defaultCfg.Home = cfg.Home

	// Write config file
	if err := config.Save(defaultCfg, configPath); err != nil {
		return fmt.Errorf("writing config file: %w", err)
	}

	w := cmd.OutOrStdout()
	out(w, "Configuration initialized at %s\n", configPath)
	outln(w)
	outln(w, "Edit this file to configure:")
	outln(w, "  - networks.eth.rpc: Your Ethereum RPC endpoint")
	outln(w, "  - networks.bsv.api_key: Your WhatsOnChain API key (optional)")
	outln(w, "  - output.default_format: Output format (text/json)")
	outln(w, "  - logging.level: Log level (off/error/debug)")

	return nil
}

func runConfigShow(cmd *cobra.Command, _ []string) error {
	w := cmd.OutOrStdout()
	format := formatter.Format()

	if format == output.FormatJSON {
		return displayConfigJSON(w, cfg)
	}

	return displayConfigText(w, cfg)
}

func runConfigGet(cmd *cobra.Command, args []string) error {
	path := args[0]

	value, err := getConfigValue(cfg, path)
	if err != nil {
		return sigilerr.WithSuggestion(
			sigilerr.ErrNotFound,
			fmt.Sprintf("configuration path '%s' not found", path),
		)
	}

	w := cmd.OutOrStdout()
	outln(w, value)

	return nil
}

func runConfigSet(cmd *cobra.Command, args []string) error {
	path := args[0]
	value := args[1]

	// Validate the path exists
	if _, err := getConfigValue(cfg, path); err != nil {
		return sigilerr.WithSuggestion(
			sigilerr.ErrNotFound,
			fmt.Sprintf("configuration path '%s' not found", path),
		)
	}

	// Load current config from file
	configPath := config.Path(cfg.Home)
	currentCfg, err := config.Load(configPath)
	if err != nil {
		// If file doesn't exist, start with defaults
		currentCfg = config.Defaults()
	}

	// Update the value
	if err := setConfigValue(currentCfg, path, value); err != nil {
		return fmt.Errorf("setting config value: %w", err)
	}

	// Save updated config
	if err := config.Save(currentCfg, configPath); err != nil {
		return fmt.Errorf("saving config: %w", err)
	}

	w := cmd.OutOrStdout()
	out(w, "Set %s = %s\n", path, value)

	return nil
}

// getConfigValue retrieves a value from the config using dot notation.
func getConfigValue(c *config.Config, path string) (string, error) {
	parts := strings.Split(path, ".")

	switch len(parts) {
	case 1:
		switch parts[0] {
		case "home":
			return c.Home, nil
		default:
			return "", sigilerr.WithDetails(
				sigilerr.ErrUnknownConfigKey,
				map[string]string{"key": parts[0]},
			)
		}
	case 2:
		switch parts[0] {
		case "output":
			return getOutputValue(c, parts[1])
		case "logging":
			return getLoggingValue(c, parts[1])
		default:
			return "", sigilerr.WithDetails(
				sigilerr.ErrUnknownConfigKey,
				map[string]string{"section": parts[0]},
			)
		}
	case 3:
		switch parts[0] {
		case "networks":
			return getNetworkValue(c, parts[1], parts[2])
		default:
			return "", sigilerr.WithDetails(
				sigilerr.ErrUnknownConfigKey,
				map[string]string{"section": parts[0]},
			)
		}
	default:
		return "", sigilerr.WithDetails(
			sigilerr.ErrUnknownConfigKey,
			map[string]string{"path": path},
		)
	}
}

func getOutputValue(c *config.Config, key string) (string, error) {
	switch key {
	case "default_format":
		return c.Output.DefaultFormat, nil
	case "verbose":
		return fmt.Sprintf("%t", c.Output.Verbose), nil
	case "color":
		return c.Output.Color, nil
	default:
		return "", sigilerr.WithDetails(
			sigilerr.ErrUnknownConfigKey,
			map[string]string{"section": "output", "key": key},
		)
	}
}

func getLoggingValue(c *config.Config, key string) (string, error) {
	switch key {
	case "level":
		return c.Logging.Level, nil
	case "file":
		return c.Logging.File, nil
	default:
		return "", sigilerr.WithDetails(
			sigilerr.ErrUnknownConfigKey,
			map[string]string{"section": "logging", "key": key},
		)
	}
}

func getNetworkValue(c *config.Config, network, key string) (string, error) {
	switch network {
	case "eth":
		switch key {
		case "rpc":
			return c.Networks.ETH.RPC, nil
		default:
			return "", sigilerr.WithDetails(
				sigilerr.ErrUnknownConfigKey,
				map[string]string{"section": "networks.eth", "key": key},
			)
		}
	case "bsv":
		switch key {
		case "api_key":
			return c.Networks.BSV.APIKey, nil
		default:
			return "", sigilerr.WithDetails(
				sigilerr.ErrUnknownConfigKey,
				map[string]string{"section": "networks.bsv", "key": key},
			)
		}
	default:
		return "", sigilerr.WithDetails(
			sigilerr.ErrUnknownConfigKey,
			map[string]string{"network": network},
		)
	}
}

// setConfigValue sets a value in the config using dot notation.
func setConfigValue(c *config.Config, path, value string) error {
	parts := strings.Split(path, ".")

	switch len(parts) {
	case 1:
		switch parts[0] {
		case "home":
			c.Home = value
			return nil
		default:
			return sigilerr.WithDetails(
				sigilerr.ErrUnknownConfigKey,
				map[string]string{"key": parts[0]},
			)
		}
	case 2:
		switch parts[0] {
		case "output":
			return setOutputValue(c, parts[1], value)
		case "logging":
			return setLoggingValue(c, parts[1], value)
		default:
			return sigilerr.WithDetails(
				sigilerr.ErrUnknownConfigKey,
				map[string]string{"section": parts[0]},
			)
		}
	case 3:
		switch parts[0] {
		case "networks":
			return setNetworkValue(c, parts[1], parts[2], value)
		default:
			return sigilerr.WithDetails(
				sigilerr.ErrUnknownConfigKey,
				map[string]string{"section": parts[0]},
			)
		}
	default:
		return sigilerr.WithDetails(
			sigilerr.ErrUnknownConfigKey,
			map[string]string{"path": path},
		)
	}
}

func setOutputValue(c *config.Config, key, value string) error {
	switch key {
	case "default_format":
		if value != "text" && value != "json" && value != "auto" {
			return sigilerr.WithDetails(
				sigilerr.ErrInvalidFormat,
				map[string]string{"value": value, "valid": "text, json, or auto"},
			)
		}
		c.Output.DefaultFormat = value
		return nil
	case "verbose":
		c.Output.Verbose = value == "true"
		return nil
	case "color":
		if value != "auto" && value != "always" && value != "never" {
			return sigilerr.WithDetails(
				sigilerr.ErrInvalidFormat,
				map[string]string{"value": value, "valid": "auto, always, or never"},
			)
		}
		c.Output.Color = value
		return nil
	default:
		return sigilerr.WithDetails(
			sigilerr.ErrUnknownConfigKey,
			map[string]string{"section": "output", "key": key},
		)
	}
}

func setLoggingValue(c *config.Config, key, value string) error {
	switch key {
	case "level":
		validLevels := []string{"off", "error", "debug"}
		for _, l := range validLevels {
			if value == l {
				c.Logging.Level = value
				return nil
			}
		}
		return sigilerr.WithDetails(
			sigilerr.ErrInvalidFormat,
			map[string]string{"value": value, "valid": "off, error, or debug"},
		)
	case "file":
		c.Logging.File = value
		return nil
	default:
		return sigilerr.WithDetails(
			sigilerr.ErrUnknownConfigKey,
			map[string]string{"section": "logging", "key": key},
		)
	}
}

func setNetworkValue(c *config.Config, network, key, value string) error {
	switch network {
	case "eth":
		switch key {
		case "rpc":
			c.Networks.ETH.RPC = value
			return nil
		default:
			return sigilerr.WithDetails(
				sigilerr.ErrUnknownConfigKey,
				map[string]string{"section": "networks.eth", "key": key},
			)
		}
	case "bsv":
		switch key {
		case "api_key":
			c.Networks.BSV.APIKey = value
			return nil
		default:
			return sigilerr.WithDetails(
				sigilerr.ErrUnknownConfigKey,
				map[string]string{"section": "networks.bsv", "key": key},
			)
		}
	default:
		return sigilerr.WithDetails(
			sigilerr.ErrUnknownConfigKey,
			map[string]string{"network": network},
		)
	}
}

// displayConfigText shows the config in text format.
func displayConfigText(w interface {
	Write(p []byte) (n int, err error)
}, c *config.Config,
) error {
	outln(w, "Configuration:")
	outln(w)
	out(w, "  Home: %s\n", c.Home)
	outln(w)
	outln(w, "  Output:")
	out(w, "    default_format: %s\n", c.Output.DefaultFormat)
	out(w, "    verbose: %t\n", c.Output.Verbose)
	out(w, "    color: %s\n", c.Output.Color)
	outln(w)
	outln(w, "  Logging:")
	out(w, "    level: %s\n", c.Logging.Level)
	out(w, "    file: %s\n", c.Logging.File)
	outln(w)
	outln(w, "  Networks:")
	outln(w, "    ETH:")
	rpc := c.Networks.ETH.RPC
	if rpc == "" {
		rpc = "(not configured)"
	}
	out(w, "      rpc: %s\n", rpc)
	outln(w, "    BSV:")
	apiKey := c.Networks.BSV.APIKey
	if apiKey == "" {
		apiKey = "(not configured)"
	} else {
		if len(apiKey) >= 4 {
			apiKey = apiKey[:4] + "..."
		} else {
			apiKey = "***..."
		}
	}
	out(w, "      api_key: %s\n", apiKey)

	return nil
}

// displayConfigJSON shows the config in JSON format.
func displayConfigJSON(w interface {
	Write(p []byte) (n int, err error)
}, c *config.Config,
) error {
	type networkJSON struct {
		RPC    string `json:"rpc,omitempty"`
		APIKey string `json:"api_key,omitempty"`
	}
	type configJSON struct {
		Version int    `json:"version"`
		Home    string `json:"home"`
		Output  struct {
			DefaultFormat string `json:"default_format"`
			Color         string `json:"color"`
			Verbose       bool   `json:"verbose"`
		} `json:"output"`
		Logging struct {
			Level string `json:"level"`
			File  string `json:"file"`
		} `json:"logging"`
		Networks struct {
			ETH networkJSON `json:"eth"`
			BSV networkJSON `json:"bsv"`
		} `json:"networks"`
	}

	maskedKey := "(not configured)"
	if c.Networks.BSV.APIKey != "" {
		if len(c.Networks.BSV.APIKey) >= 4 {
			maskedKey = c.Networks.BSV.APIKey[:4] + "..."
		} else {
			maskedKey = "***..."
		}
	}

	outCfg := configJSON{
		Version: c.Version,
		Home:    c.Home,
	}
	outCfg.Output.DefaultFormat = c.Output.DefaultFormat
	outCfg.Output.Color = c.Output.Color
	outCfg.Output.Verbose = c.Output.Verbose
	outCfg.Logging.Level = c.Logging.Level
	outCfg.Logging.File = c.Logging.File
	outCfg.Networks.ETH = networkJSON{RPC: c.Networks.ETH.RPC}
	outCfg.Networks.BSV = networkJSON{APIKey: maskedKey}

	return writeJSON(w, outCfg)
}
