package config

// Defaults returns the default configuration.
func Defaults() *Config {
	return &Config{
		Version: 1,
		Home:    "~/.sigil",
		Encryption: EncryptionConfig{
			Method:        "age",
			IdentityFile:  "~/.sigil/identity.age",
			KeyDerivation: "argon2id",
		},
		Networks: NetworksConfig{
			ETH: ETHNetworkConfig{
				Enabled: true,
				RPC:     "", // User must configure
				ChainID: 1,
				Tokens: []TokenConfig{
					{
						Symbol:   "USDC",
						Address:  "0xA0b86991c6218b36c1d19D4a2e9Eb0cE3606eB48",
						Decimals: 6,
					},
				},
			},
			BSV: BSVNetworkConfig{
				Enabled:   true,
				API:       "whatsonchain",
				Broadcast: "taal",
				APIKey:    "",
			},
			BTC: BTCNetworkConfig{
				Enabled: false, // Phase 2
				API:     "mempool",
			},
			BCH: BCHNetworkConfig{
				Enabled: false, // Phase 2
				API:     "fullstack",
			},
		},
		Fees: FeesConfig{
			Provider:            "taal",
			FallbackSatsPerByte: 1,
			MaxSatsPerByte:      100,
			ETHGasStrategy:      "medium",
		},
		Derivation: DerivationConfig{
			DefaultAccount: 0,
			AddressGap:     20,
			Paths:          map[string]string{},
		},
		Security: SecurityConfig{
			AutoLockSeconds:     0, // Disabled for MVP
			RequireConfirmAbove: 0, // Disabled for MVP
			MemoryLock:          true,
		},
		Output: OutputConfig{
			DefaultFormat: "auto",
			Color:         "auto",
			Verbose:       false,
		},
		Logging: LoggingConfig{
			Level: "error",
			File:  "~/.sigil/sigil.log",
		},
	}
}
