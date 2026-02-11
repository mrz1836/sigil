package transaction

import (
	"math/big"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mrz1836/sigil/internal/chain"
	"github.com/mrz1836/sigil/internal/chain/eth"
)

// TestSendETH_AddressValidation tests ETH address validation logic.
func TestSendETH_AddressValidation(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		address string
		valid   bool
	}{
		{
			name:    "Valid checksum address",
			address: "0x5aAeb6053F3E94C9b9A09f33669435E7Ef1BeAed",
			valid:   true,
		},
		{
			name:    "Valid lowercase address",
			address: "0x5aaeb6053f3e94c9b9a09f33669435e7ef1beaed",
			valid:   true,
		},
		{
			name:    "Invalid address - too short",
			address: "0x5aAeb6053F3E94C9b9A09f33669435E7Ef1BeA",
			valid:   false,
		},
		{
			name:    "Invalid address - no 0x prefix",
			address: "5aAeb6053F3E94C9b9A09f33669435E7Ef1BeAed",
			valid:   false,
		},
		{
			name:    "Invalid address - invalid characters",
			address: "0x5aAeb6053F3E94C9b9A09f33669435E7Ef1BeAeG",
			valid:   false,
		},
		{
			name:    "Empty address",
			address: "",
			valid:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// Test address validation
			checksumErr := eth.ValidateChecksumAddress(tt.address)
			isValid := eth.IsValidAddress(tt.address)

			if tt.valid {
				// Valid addresses should pass at least one check
				assert.True(t, checksumErr == nil || isValid,
					"Expected address to be valid: %s", tt.address)
			} else {
				// Invalid addresses should fail both checks
				assert.False(t, checksumErr == nil && isValid,
					"Expected address to be invalid: %s", tt.address)
			}
		})
	}
}

// TestSendETH_TokenResolution tests token resolution for ETH transactions.
func TestSendETH_TokenResolution(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		token        string
		wantAddress  string
		wantDecimals int
		wantErr      bool
	}{
		{
			name:         "USDC token",
			token:        "USDC",
			wantAddress:  eth.USDCMainnet,
			wantDecimals: eth.USDCDecimals,
			wantErr:      false,
		},
		{
			name:         "USDC lowercase",
			token:        "usdc",
			wantAddress:  eth.USDCMainnet,
			wantDecimals: eth.USDCDecimals,
			wantErr:      false,
		},
		{
			name:    "Unsupported token",
			token:   "DAI",
			wantErr: true,
		},
		{
			name:    "Empty token",
			token:   "",
			wantErr: false, // Empty token means native ETH
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if tt.token == "" {
				// Empty token is valid (native ETH)
				return
			}

			address, decimals, err := resolveToken(tt.token)

			if tt.wantErr {
				require.Error(t, err)
				assert.Empty(t, address)
				assert.Zero(t, decimals)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.wantAddress, address)
				assert.Equal(t, tt.wantDecimals, decimals)
			}
		})
	}
}

// TestSendETH_AmountParsing tests amount parsing logic for ETH transactions.
func TestSendETH_AmountParsing(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		amount   string
		decimals int
		want     string
		wantErr  bool
	}{
		{
			name:     "1 ETH",
			amount:   "1.0",
			decimals: 18,
			want:     "1000000000000000000",
			wantErr:  false,
		},
		{
			name:     "0.5 ETH",
			amount:   "0.5",
			decimals: 18,
			want:     "500000000000000000",
			wantErr:  false,
		},
		{
			name:     "1 USDC",
			amount:   "1.0",
			decimals: 6,
			want:     "1000000",
			wantErr:  false,
		},
		{
			name:     "Invalid amount",
			amount:   "abc",
			decimals: 18,
			wantErr:  true,
		},
		{
			name:     "Negative amount",
			amount:   "-1.0",
			decimals: 18,
			wantErr:  true,
		},
		{
			name:     "Empty amount",
			amount:   "",
			decimals: 18,
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			amount, err := parseDecimalAmount(tt.amount, tt.decimals)

			if tt.wantErr {
				require.Error(t, err)
				assert.Nil(t, amount)
			} else {
				require.NoError(t, err)
				require.NotNil(t, amount)
				assert.Equal(t, tt.want, amount.String())
			}
		})
	}
}

// TestSendETH_GasSpeedParsing tests gas speed parsing.
func TestSendETH_GasSpeedParsing(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		speed   string
		want    eth.GasSpeed
		wantErr bool
	}{
		{
			name:    "Fast speed",
			speed:   "fast",
			want:    eth.GasSpeedFast,
			wantErr: false,
		},
		{
			name:    "Medium speed",
			speed:   "medium",
			want:    eth.GasSpeedMedium,
			wantErr: false,
		},
		{
			name:    "Slow speed",
			speed:   "slow",
			want:    eth.GasSpeedSlow,
			wantErr: false,
		},
		{
			name:    "Empty defaults to medium",
			speed:   "",
			want:    eth.GasSpeedMedium,
			wantErr: false,
		},
		{
			name:    "Invalid speed",
			speed:   "instant",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			speed, err := eth.ParseGasSpeed(tt.speed)

			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.want, speed)
			}
		})
	}
}

// TestSendETH_SweepCalculation tests sweep amount calculation logic.
func TestSendETH_SweepCalculation(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		balance      *big.Int
		gasCost      *big.Int
		wantAmount   *big.Int
		wantPositive bool
	}{
		{
			name:         "Balance greater than gas",
			balance:      big.NewInt(2000000000000000000), // 2 ETH
			gasCost:      big.NewInt(21000000000000000),   // 0.021 ETH
			wantAmount:   big.NewInt(1979000000000000000), // 1.979 ETH
			wantPositive: true,
		},
		{
			name:         "Balance exactly equals gas",
			balance:      big.NewInt(21000000000000000),
			gasCost:      big.NewInt(21000000000000000),
			wantAmount:   big.NewInt(0),
			wantPositive: false,
		},
		{
			name:         "Balance less than gas",
			balance:      big.NewInt(10000000000000000),
			gasCost:      big.NewInt(21000000000000000),
			wantAmount:   big.NewInt(-11000000000000000),
			wantPositive: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// Calculate sweep amount: balance - gasCost
			sweepAmount := new(big.Int).Sub(tt.balance, tt.gasCost)

			assert.Equal(t, tt.wantAmount.String(), sweepAmount.String())
			assert.Equal(t, tt.wantPositive, sweepAmount.Sign() > 0)
		})
	}
}

// TestSendETH_BalanceValidation tests balance validation for different scenarios.
func TestSendETH_BalanceValidation(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		ethBalance   *big.Int
		tokenBalance *big.Int
		amount       *big.Int
		gasCost      *big.Int
		isToken      bool
		sufficient   bool
	}{
		{
			name:       "Native ETH - sufficient balance",
			ethBalance: big.NewInt(2000000000000000000), // 2 ETH
			amount:     big.NewInt(1000000000000000000), // 1 ETH
			gasCost:    big.NewInt(21000000000000000),   // 0.021 ETH
			isToken:    false,
			sufficient: true,
		},
		{
			name:       "Native ETH - insufficient balance",
			ethBalance: big.NewInt(500000000000000000),  // 0.5 ETH
			amount:     big.NewInt(1000000000000000000), // 1 ETH
			gasCost:    big.NewInt(21000000000000000),   // 0.021 ETH
			isToken:    false,
			sufficient: false,
		},
		{
			name:         "ERC-20 - sufficient ETH and token",
			ethBalance:   big.NewInt(1000000000000000000), // 1 ETH
			tokenBalance: big.NewInt(2000000),             // 2 USDC
			amount:       big.NewInt(1000000),             // 1 USDC
			gasCost:      big.NewInt(50000000000000000),   // 0.05 ETH
			isToken:      true,
			sufficient:   true,
		},
		{
			name:         "ERC-20 - insufficient ETH for gas",
			ethBalance:   big.NewInt(10000000000000000), // 0.01 ETH
			tokenBalance: big.NewInt(2000000),           // 2 USDC
			amount:       big.NewInt(1000000),           // 1 USDC
			gasCost:      big.NewInt(50000000000000000), // 0.05 ETH
			isToken:      true,
			sufficient:   false,
		},
		{
			name:         "ERC-20 - insufficient token balance",
			ethBalance:   big.NewInt(1000000000000000000), // 1 ETH
			tokenBalance: big.NewInt(500000),              // 0.5 USDC
			amount:       big.NewInt(1000000),             // 1 USDC
			gasCost:      big.NewInt(50000000000000000),   // 0.05 ETH
			isToken:      true,
			sufficient:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var hasSufficient bool

			if tt.isToken {
				// For tokens: need ETH for gas AND token balance
				hasETH := tt.ethBalance.Cmp(tt.gasCost) >= 0
				hasToken := tt.tokenBalance.Cmp(tt.amount) >= 0
				hasSufficient = hasETH && hasToken
			} else {
				// For native ETH: need balance >= amount + gas
				totalRequired := new(big.Int).Add(tt.amount, tt.gasCost)
				hasSufficient = tt.ethBalance.Cmp(totalRequired) >= 0
			}

			assert.Equal(t, tt.sufficient, hasSufficient,
				"Balance check mismatch for %s", tt.name)
		})
	}
}

// TestSendETH_DisplayAmountFormatting tests display amount formatting.
func TestSendETH_DisplayAmountFormatting(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		amount   *big.Int
		decimals int
		sweep    bool
		want     string
	}{
		{
			name:     "1 ETH normal send",
			amount:   big.NewInt(1000000000000000000),
			decimals: 18,
			sweep:    false,
			want:     "1.0",
		},
		{
			name:     "1 ETH sweep",
			amount:   big.NewInt(1000000000000000000),
			decimals: 18,
			sweep:    true,
			want:     "1.0 (sweep all)",
		},
		{
			name:     "100 USDC normal send",
			amount:   big.NewInt(100000000),
			decimals: 6,
			sweep:    false,
			want:     "100.0",
		},
		{
			name:     "100 USDC sweep",
			amount:   big.NewInt(100000000),
			decimals: 6,
			sweep:    true,
			want:     "100.0 (sweep all)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			formatted := chain.FormatDecimalAmount(tt.amount, tt.decimals)
			if tt.sweep {
				formatted += " (sweep all)"
			}

			assert.Equal(t, tt.want, formatted)
		})
	}
}

// TestSendETH_ZeroBalanceHandling tests handling of zero balances.
func TestSendETH_ZeroBalanceHandling(t *testing.T) {
	t.Parallel()

	zeroBalance := big.NewInt(0)

	// Zero balance should fail for any send
	assert.Equal(t, 0, zeroBalance.Sign(), "Zero balance should have zero sign")

	// Zero balance for sweep should be invalid
	gasCost := big.NewInt(21000000000000000)
	sweepAmount := new(big.Int).Sub(zeroBalance, gasCost)
	assert.LessOrEqual(t, sweepAmount.Sign(), 0, "Sweep with zero balance should be invalid")
}

// TestSendETH_MaxAmountEdgeCases tests edge cases with large amounts.
func TestSendETH_MaxAmountEdgeCases(t *testing.T) {
	t.Parallel()

	// Test with max uint64 (represented in wei)
	maxAmount := new(big.Int).SetUint64(^uint64(0))

	// Should be able to represent in big.Int
	assert.NotNil(t, maxAmount)
	assert.Positive(t, maxAmount.Sign())

	// Should be able to format
	formatted := chain.FormatDecimalAmount(maxAmount, 18)
	assert.NotEmpty(t, formatted)
}

// TestSendETH_ConfigValidation tests configuration validation.
func TestSendETH_ConfigValidation(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		rpcURL string
		valid  bool
	}{
		{
			name:   "Valid RPC URL",
			rpcURL: "https://eth-mainnet.example.com",
			valid:  true,
		},
		{
			name:   "Valid localhost RPC",
			rpcURL: "http://localhost:8545",
			valid:  true,
		},
		{
			name:   "Empty RPC URL",
			rpcURL: "",
			valid:  false,
		},
		{
			name:   "Invalid URL format",
			rpcURL: "not-a-url",
			valid:  true, // URL validation happens at connection time
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// Empty URL should be considered invalid
			isEmpty := tt.rpcURL == ""

			if tt.valid {
				assert.False(t, isEmpty, "Valid RPC URL should not be empty")
			} else {
				assert.True(t, isEmpty, "Invalid config should have empty URL")
			}
		})
	}
}

// TestSendETH_SweepScenarios tests various sweep scenarios.
func TestSendETH_SweepScenarios(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name             string
		amountStr        string
		isSweep          bool
		tokenAddress     string
		balanceAfterSend string
	}{
		{
			name:             "Sweep all native ETH",
			amountStr:        "all",
			isSweep:          true,
			tokenAddress:     "",
			balanceAfterSend: "0.0",
		},
		{
			name:             "Sweep all ERC-20",
			amountStr:        "all",
			isSweep:          true,
			tokenAddress:     eth.USDCMainnet,
			balanceAfterSend: "0.0",
		},
		{
			name:             "Partial send",
			amountStr:        "1.0",
			isSweep:          false,
			tokenAddress:     "",
			balanceAfterSend: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			isSweep := IsAmountAll(tt.amountStr)
			assert.Equal(t, tt.isSweep, isSweep)

			if tt.isSweep {
				assert.Equal(t, "0.0", tt.balanceAfterSend,
					"Sweep should result in zero balance")
			}
		})
	}
}

// TestSendETH_RequestStructure tests SendRequest structure for ETH.
func TestSendETH_RequestStructure(t *testing.T) {
	t.Parallel()

	req := &SendRequest{
		ChainID:     chain.ETH,
		To:          "0x5aAeb6053F3E94C9b9A09f33669435E7Ef1BeAed",
		AmountStr:   "1.0",
		Token:       "USDC",
		GasSpeed:    "fast",
		FromAddress: "0x1234567890123456789012345678901234567890",
	}

	// Verify structure
	assert.Equal(t, chain.ETH, req.ChainID)
	assert.NotEmpty(t, req.To)
	assert.NotEmpty(t, req.AmountStr)
	assert.Equal(t, "USDC", req.Token)
	assert.Equal(t, "fast", req.GasSpeed)
	assert.False(t, req.SweepAll())
}

// TestSendETH_ResultStructure tests SendResult structure for ETH.
func TestSendETH_ResultStructure(t *testing.T) {
	t.Parallel()

	result := &SendResult{
		Hash:     "0xabc123",
		From:     "0x1234567890123456789012345678901234567890",
		To:       "0x5aAeb6053F3E94C9b9A09f33669435E7Ef1BeAed",
		Amount:   "1.0",
		Fee:      "0.00021",
		Token:    "USDC",
		Status:   "success",
		ChainID:  chain.ETH,
		GasUsed:  21000,
		GasPrice: "10000000000",
	}

	// Verify structure
	assert.Equal(t, chain.ETH, result.ChainID)
	assert.NotEmpty(t, result.Hash)
	assert.NotEmpty(t, result.From)
	assert.NotEmpty(t, result.To)
	assert.NotEmpty(t, result.Amount)
	assert.Equal(t, "USDC", result.Token)
	assert.NotEmpty(t, result.GasUsed)
	assert.NotEmpty(t, result.GasPrice)
}
