package cli

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mrz1836/sigil/internal/config"
)

func TestGetConfigValue(t *testing.T) {
	testCfg := config.Defaults()
	testCfg.Home = "/test/home"
	testCfg.Output.DefaultFormat = "json"
	testCfg.Output.Verbose = true
	testCfg.Output.Color = "always"
	testCfg.Logging.Level = "debug"
	testCfg.Logging.File = "/var/log/sigil.log"
	testCfg.Networks.ETH.RPC = "https://eth.example.com"
	testCfg.Networks.BSV.APIKey = "test-api-key"

	tests := []struct {
		name    string
		path    string
		want    string
		wantErr bool
	}{
		// Single-part paths
		{name: "home", path: "home", want: "/test/home"},
		{name: "unknown single key", path: "unknown", wantErr: true},

		// Output section
		{name: "output.default_format", path: "output.default_format", want: "json"},
		{name: "output.verbose true", path: "output.verbose", want: "true"},
		{name: "output.color", path: "output.color", want: "always"},
		{name: "output.unknown", path: "output.unknown", wantErr: true},

		// Logging section
		{name: "logging.level", path: "logging.level", want: "debug"},
		{name: "logging.file", path: "logging.file", want: "/var/log/sigil.log"},
		{name: "logging.unknown", path: "logging.unknown", wantErr: true},

		// Networks section
		{name: "networks.eth.rpc", path: "networks.eth.rpc", want: "https://eth.example.com"},
		{name: "networks.bsv.api_key", path: "networks.bsv.api_key", want: "test-api-key"},
		{name: "networks.eth.unknown", path: "networks.eth.unknown", wantErr: true},
		{name: "networks.bsv.unknown", path: "networks.bsv.unknown", wantErr: true},
		{name: "networks.unknown.key", path: "networks.unknown.key", wantErr: true},

		// Unknown sections
		{name: "unknown.key", path: "unknown.key", wantErr: true},
		{name: "unknown.section.key", path: "unknown.section.key", wantErr: true},

		// Too many parts
		{name: "too many parts", path: "a.b.c.d", wantErr: true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := getConfigValue(testCfg, tc.path)
			if tc.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tc.want, got)
			}
		})
	}
}

func TestGetConfigValue_VerboseFalse(t *testing.T) {
	testCfg := config.Defaults()
	testCfg.Output.Verbose = false

	got, err := getConfigValue(testCfg, "output.verbose")
	require.NoError(t, err)
	assert.Equal(t, "false", got)
}

func TestGetOutputValue(t *testing.T) {
	testCfg := config.Defaults()
	testCfg.Output.DefaultFormat = "text"
	testCfg.Output.Verbose = true
	testCfg.Output.Color = "never"

	tests := []struct {
		key     string
		want    string
		wantErr bool
	}{
		{key: "default_format", want: "text"},
		{key: "verbose", want: "true"},
		{key: "color", want: "never"},
		{key: "unknown", wantErr: true},
	}

	for _, tc := range tests {
		t.Run(tc.key, func(t *testing.T) {
			got, err := getOutputValue(testCfg, tc.key)
			if tc.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tc.want, got)
			}
		})
	}
}

func TestGetLoggingValue(t *testing.T) {
	testCfg := config.Defaults()
	testCfg.Logging.Level = "warn"
	testCfg.Logging.File = "/tmp/test.log"

	tests := []struct {
		key     string
		want    string
		wantErr bool
	}{
		{key: "level", want: "warn"},
		{key: "file", want: "/tmp/test.log"},
		{key: "unknown", wantErr: true},
	}

	for _, tc := range tests {
		t.Run(tc.key, func(t *testing.T) {
			got, err := getLoggingValue(testCfg, tc.key)
			if tc.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tc.want, got)
			}
		})
	}
}

func TestGetNetworkValue(t *testing.T) {
	testCfg := config.Defaults()
	testCfg.Networks.ETH.RPC = "https://mainnet.infura.io"
	testCfg.Networks.BSV.APIKey = "woc-api-key"

	tests := []struct {
		name    string
		network string
		key     string
		want    string
		wantErr bool
	}{
		{name: "eth.rpc", network: "eth", key: "rpc", want: "https://mainnet.infura.io"},
		{name: "eth.unknown", network: "eth", key: "unknown", wantErr: true},
		{name: "bsv.api_key", network: "bsv", key: "api_key", want: "woc-api-key"},
		{name: "bsv.unknown", network: "bsv", key: "unknown", wantErr: true},
		{name: "unknown.key", network: "unknown", key: "key", wantErr: true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := getNetworkValue(testCfg, tc.network, tc.key)
			if tc.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tc.want, got)
			}
		})
	}
}

func TestSetConfigValue(t *testing.T) {
	tests := []struct {
		name    string
		path    string
		value   string
		verify  func(*testing.T, *config.Config)
		wantErr bool
	}{
		// Single-part paths
		{
			name:  "set home",
			path:  "home",
			value: "/new/home",
			verify: func(t *testing.T, c *config.Config) {
				assert.Equal(t, "/new/home", c.Home)
			},
		},
		{name: "set unknown single key", path: "unknown", value: "val", wantErr: true},

		// Output section
		{
			name:  "set output.default_format text",
			path:  "output.default_format",
			value: "text",
			verify: func(t *testing.T, c *config.Config) {
				assert.Equal(t, "text", c.Output.DefaultFormat)
			},
		},
		{
			name:  "set output.default_format json",
			path:  "output.default_format",
			value: "json",
			verify: func(t *testing.T, c *config.Config) {
				assert.Equal(t, "json", c.Output.DefaultFormat)
			},
		},
		{
			name:  "set output.default_format auto",
			path:  "output.default_format",
			value: "auto",
			verify: func(t *testing.T, c *config.Config) {
				assert.Equal(t, "auto", c.Output.DefaultFormat)
			},
		},
		{name: "set output.default_format invalid", path: "output.default_format", value: "invalid", wantErr: true},
		{
			name:  "set output.verbose true",
			path:  "output.verbose",
			value: "true",
			verify: func(t *testing.T, c *config.Config) {
				assert.True(t, c.Output.Verbose)
			},
		},
		{
			name:  "set output.verbose false",
			path:  "output.verbose",
			value: "false",
			verify: func(t *testing.T, c *config.Config) {
				assert.False(t, c.Output.Verbose)
			},
		},
		{
			name:  "set output.color auto",
			path:  "output.color",
			value: "auto",
			verify: func(t *testing.T, c *config.Config) {
				assert.Equal(t, "auto", c.Output.Color)
			},
		},
		{
			name:  "set output.color always",
			path:  "output.color",
			value: "always",
			verify: func(t *testing.T, c *config.Config) {
				assert.Equal(t, "always", c.Output.Color)
			},
		},
		{
			name:  "set output.color never",
			path:  "output.color",
			value: "never",
			verify: func(t *testing.T, c *config.Config) {
				assert.Equal(t, "never", c.Output.Color)
			},
		},
		{name: "set output.color invalid", path: "output.color", value: "invalid", wantErr: true},
		{name: "set output.unknown", path: "output.unknown", value: "val", wantErr: true},

		// Logging section
		{
			name:  "set logging.level debug",
			path:  "logging.level",
			value: "debug",
			verify: func(t *testing.T, c *config.Config) {
				assert.Equal(t, "debug", c.Logging.Level)
			},
		},
		{
			name:  "set logging.level info",
			path:  "logging.level",
			value: "info",
			verify: func(t *testing.T, c *config.Config) {
				assert.Equal(t, "info", c.Logging.Level)
			},
		},
		{
			name:  "set logging.level warn",
			path:  "logging.level",
			value: "warn",
			verify: func(t *testing.T, c *config.Config) {
				assert.Equal(t, "warn", c.Logging.Level)
			},
		},
		{
			name:  "set logging.level error",
			path:  "logging.level",
			value: "error",
			verify: func(t *testing.T, c *config.Config) {
				assert.Equal(t, "error", c.Logging.Level)
			},
		},
		{name: "set logging.level invalid", path: "logging.level", value: "invalid", wantErr: true},
		{
			name:  "set logging.file",
			path:  "logging.file",
			value: "/custom/path.log",
			verify: func(t *testing.T, c *config.Config) {
				assert.Equal(t, "/custom/path.log", c.Logging.File)
			},
		},
		{name: "set logging.unknown", path: "logging.unknown", value: "val", wantErr: true},

		// Networks section
		{
			name:  "set networks.eth.rpc",
			path:  "networks.eth.rpc",
			value: "https://new-rpc.example.com",
			verify: func(t *testing.T, c *config.Config) {
				assert.Equal(t, "https://new-rpc.example.com", c.Networks.ETH.RPC)
			},
		},
		{name: "set networks.eth.unknown", path: "networks.eth.unknown", value: "val", wantErr: true},
		{
			name:  "set networks.bsv.api_key",
			path:  "networks.bsv.api_key",
			value: "new-api-key",
			verify: func(t *testing.T, c *config.Config) {
				assert.Equal(t, "new-api-key", c.Networks.BSV.APIKey)
			},
		},
		{name: "set networks.bsv.unknown", path: "networks.bsv.unknown", value: "val", wantErr: true},
		{name: "set networks.unknown.key", path: "networks.unknown.key", value: "val", wantErr: true},

		// Unknown sections
		{name: "set unknown.key", path: "unknown.key", value: "val", wantErr: true},
		{name: "set unknown.section.key", path: "unknown.section.key", value: "val", wantErr: true},

		// Too many parts
		{name: "set too many parts", path: "a.b.c.d", value: "val", wantErr: true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			c := config.Defaults()
			err := setConfigValue(c, tc.path, tc.value)
			if tc.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				if tc.verify != nil {
					tc.verify(t, c)
				}
			}
		})
	}
}

func TestSetOutputValue(t *testing.T) {
	tests := []struct {
		name    string
		key     string
		value   string
		verify  func(*testing.T, *config.Config)
		wantErr bool
	}{
		{
			name:  "default_format text",
			key:   "default_format",
			value: "text",
			verify: func(t *testing.T, c *config.Config) {
				assert.Equal(t, "text", c.Output.DefaultFormat)
			},
		},
		{name: "default_format invalid", key: "default_format", value: "yaml", wantErr: true},
		{
			name:  "verbose true",
			key:   "verbose",
			value: "true",
			verify: func(t *testing.T, c *config.Config) {
				assert.True(t, c.Output.Verbose)
			},
		},
		{
			name:  "verbose non-true becomes false",
			key:   "verbose",
			value: "anything",
			verify: func(t *testing.T, c *config.Config) {
				assert.False(t, c.Output.Verbose)
			},
		},
		{
			name:  "color always",
			key:   "color",
			value: "always",
			verify: func(t *testing.T, c *config.Config) {
				assert.Equal(t, "always", c.Output.Color)
			},
		},
		{name: "color invalid", key: "color", value: "sometimes", wantErr: true},
		{name: "unknown key", key: "unknown", value: "val", wantErr: true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			c := config.Defaults()
			err := setOutputValue(c, tc.key, tc.value)
			if tc.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				if tc.verify != nil {
					tc.verify(t, c)
				}
			}
		})
	}
}

func TestSetLoggingValue(t *testing.T) {
	tests := []struct {
		name    string
		key     string
		value   string
		verify  func(*testing.T, *config.Config)
		wantErr bool
	}{
		{
			name:  "level debug",
			key:   "level",
			value: "debug",
			verify: func(t *testing.T, c *config.Config) {
				assert.Equal(t, "debug", c.Logging.Level)
			},
		},
		{name: "level invalid", key: "level", value: "trace", wantErr: true},
		{
			name:  "file path",
			key:   "file",
			value: "/tmp/sigil.log",
			verify: func(t *testing.T, c *config.Config) {
				assert.Equal(t, "/tmp/sigil.log", c.Logging.File)
			},
		},
		{name: "unknown key", key: "unknown", value: "val", wantErr: true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			c := config.Defaults()
			err := setLoggingValue(c, tc.key, tc.value)
			if tc.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				if tc.verify != nil {
					tc.verify(t, c)
				}
			}
		})
	}
}

func TestSetNetworkValue(t *testing.T) {
	tests := []struct {
		name    string
		network string
		key     string
		value   string
		verify  func(*testing.T, *config.Config)
		wantErr bool
	}{
		{
			name:    "eth rpc",
			network: "eth",
			key:     "rpc",
			value:   "https://mainnet.infura.io/v3/KEY",
			verify: func(t *testing.T, c *config.Config) {
				assert.Equal(t, "https://mainnet.infura.io/v3/KEY", c.Networks.ETH.RPC)
			},
		},
		{name: "eth unknown", network: "eth", key: "unknown", value: "val", wantErr: true},
		{
			name:    "bsv api_key",
			network: "bsv",
			key:     "api_key",
			value:   "my-api-key",
			verify: func(t *testing.T, c *config.Config) {
				assert.Equal(t, "my-api-key", c.Networks.BSV.APIKey)
			},
		},
		{name: "bsv unknown", network: "bsv", key: "unknown", value: "val", wantErr: true},
		{name: "unknown network", network: "btc", key: "rpc", value: "val", wantErr: true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			c := config.Defaults()
			err := setNetworkValue(c, tc.network, tc.key, tc.value)
			if tc.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				if tc.verify != nil {
					tc.verify(t, c)
				}
			}
		})
	}
}

func TestDisplayConfigText(t *testing.T) {
	testCfg := config.Defaults()
	testCfg.Home = "/test/sigil"
	testCfg.Output.DefaultFormat = "json"
	testCfg.Output.Verbose = true
	testCfg.Output.Color = "always"
	testCfg.Logging.Level = "debug"
	testCfg.Logging.File = "/var/log/sigil.log"
	testCfg.Networks.ETH.RPC = "https://eth.example.com"
	testCfg.Networks.BSV.APIKey = "abcd1234567890"

	buf := new(bytes.Buffer)
	err := displayConfigText(buf, testCfg)
	require.NoError(t, err)

	out := buf.String()

	// Check structure
	assert.Contains(t, out, "Configuration:")
	assert.Contains(t, out, "Home: /test/sigil")
	assert.Contains(t, out, "Output:")
	assert.Contains(t, out, "default_format: json")
	assert.Contains(t, out, "verbose: true")
	assert.Contains(t, out, "color: always")
	assert.Contains(t, out, "Logging:")
	assert.Contains(t, out, "level: debug")
	assert.Contains(t, out, "file: /var/log/sigil.log")
	assert.Contains(t, out, "Networks:")
	assert.Contains(t, out, "ETH:")
	assert.Contains(t, out, "rpc: https://eth.example.com")
	assert.Contains(t, out, "BSV:")
	// API key should be masked
	assert.Contains(t, out, "api_key: abcd...")
	assert.NotContains(t, out, "abcd1234567890")
}

func TestDisplayConfigText_EmptyRPC(t *testing.T) {
	testCfg := config.Defaults()
	testCfg.Networks.ETH.RPC = ""

	buf := new(bytes.Buffer)
	err := displayConfigText(buf, testCfg)
	require.NoError(t, err)

	out := buf.String()
	assert.Contains(t, out, "rpc: (not configured)")
}

func TestDisplayConfigText_EmptyAPIKey(t *testing.T) {
	testCfg := config.Defaults()
	testCfg.Networks.BSV.APIKey = ""

	buf := new(bytes.Buffer)
	err := displayConfigText(buf, testCfg)
	require.NoError(t, err)

	out := buf.String()
	assert.Contains(t, out, "api_key: (not configured)")
}

func TestDisplayConfigText_ShortAPIKey(t *testing.T) {
	testCfg := config.Defaults()
	testCfg.Networks.BSV.APIKey = "ab" // Less than 4 chars

	buf := new(bytes.Buffer)
	err := displayConfigText(buf, testCfg)
	require.NoError(t, err)

	out := buf.String()
	assert.Contains(t, out, "api_key: ***...")
	assert.NotContains(t, out, "api_key: ab")
}

func TestDisplayConfigJSON(t *testing.T) {
	testCfg := config.Defaults()
	testCfg.Home = "/test/sigil"

	buf := new(bytes.Buffer)
	err := displayConfigJSON(buf, testCfg)
	require.NoError(t, err)

	out := buf.String()
	// YAML output should contain the config values
	assert.Contains(t, out, "home: /test/sigil")
	assert.Contains(t, out, "version: 1")
}
