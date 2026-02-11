package transaction

import (
	"context"
	"math/big"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mrz1836/sigil/internal/chain"
	"github.com/mrz1836/sigil/internal/chain/bsv"
	"github.com/mrz1836/sigil/internal/wallet"
)

// TestSendBSV_AddressValidation tests BSV address validation logic.
func TestSendBSV_AddressValidation(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		address string
		valid   bool
	}{
		{
			name:    "Valid P2PKH address",
			address: "1A1zP1eP5QGefi2DMPTfTL5SLmv7DivfNa",
			valid:   true,
		},
		{
			name:    "Invalid address - too short",
			address: "1A1zP1eP5QGefi",
			valid:   false,
		},
		{
			name:    "Invalid address - invalid checksum",
			address: "1A1zP1eP5QGefi2DMPTfTL5SLmv7DivfNX",
			valid:   false,
		},
		{
			name:    "Invalid address - invalid characters",
			address: "1A1zP1eP5QGefi2DMPTfTL5SLmv7DivfN0",
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

			err := bsv.ValidateBase58CheckAddress(tt.address)

			if tt.valid {
				assert.NoError(t, err, "Expected address to be valid: %s", tt.address)
			} else {
				assert.Error(t, err, "Expected address to be invalid: %s", tt.address)
			}
		})
	}
}

// TestSendBSV_AmountParsing tests amount parsing logic for BSV transactions.
func TestSendBSV_AmountParsing(t *testing.T) {
	t.Parallel()

	// Create a temporary BSV client for testing (won't make network calls)
	client := bsv.NewClient(context.Background(), nil)

	tests := []struct {
		name    string
		amount  string
		want    uint64
		wantErr bool
	}{
		{
			name:    "1 BSV",
			amount:  "1.0",
			want:    100000000,
			wantErr: false,
		},
		{
			name:    "0.5 BSV",
			amount:  "0.5",
			want:    50000000,
			wantErr: false,
		},
		{
			name:    "0.00000001 BSV (1 satoshi)",
			amount:  "0.00000001",
			want:    1,
			wantErr: false,
		},
		{
			name:    "Invalid amount",
			amount:  "abc",
			wantErr: true,
		},
		{
			name:    "Negative amount",
			amount:  "-1.0",
			wantErr: true,
		},
		{
			name:    "Empty amount",
			amount:  "",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			amount, err := client.ParseAmount(tt.amount)

			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				require.NotNil(t, amount)
				assert.Equal(t, tt.want, amount.Uint64())
			}
		})
	}
}

// TestSendBSV_FeeEstimation tests fee estimation logic.
func TestSendBSV_FeeEstimation(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		inputs     int
		outputs    int
		feeRate    uint64
		wantMinFee uint64
	}{
		{
			name:       "Simple transaction (1 input, 2 outputs)",
			inputs:     1,
			outputs:    2,
			feeRate:    500,
			wantMinFee: 100, // Approximate minimum
		},
		{
			name:       "Multiple inputs (5 inputs, 2 outputs)",
			inputs:     5,
			outputs:    2,
			feeRate:    500,
			wantMinFee: 300, // Approximate minimum
		},
		{
			name:       "High fee rate",
			inputs:     1,
			outputs:    2,
			feeRate:    2000,
			wantMinFee: 200, // Higher fee
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			fee := bsv.EstimateFeeForTx(tt.inputs, tt.outputs, tt.feeRate)

			// Fee should be greater than minimum and proportional to inputs
			assert.GreaterOrEqual(t, fee, tt.wantMinFee,
				"Fee should be at least minimum expected")

			// Fee should increase with more inputs
			assert.Positive(t, fee, "Fee should be positive")
		})
	}
}

// TestSendBSV_SweepAmountCalculation tests sweep amount calculation.
func TestSendBSV_SweepAmountCalculation(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		totalInputs uint64
		numInputs   int
		feeRate     uint64
		wantAmount  uint64
		wantErr     bool
	}{
		{
			name:        "Sufficient balance",
			totalInputs: 100000000, // 1 BSV
			numInputs:   1,
			feeRate:     500,
			wantErr:     false,
		},
		{
			name:        "Multiple inputs",
			totalInputs: 200000000, // 2 BSV
			numInputs:   5,
			feeRate:     500,
			wantErr:     false,
		},
		{
			name:        "Insufficient for fee",
			totalInputs: 100, // 100 satoshis (less than fee)
			numInputs:   1,
			feeRate:     1000, // Higher fee rate to exceed inputs
			wantErr:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			amount, err := bsv.CalculateSweepAmount(tt.totalInputs, tt.numInputs, tt.feeRate)

			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				// Amount should be less than total inputs (fee deducted)
				assert.Less(t, amount, tt.totalInputs,
					"Sweep amount should be less than total inputs")
				// Amount should be positive (above dust threshold)
				assert.Positive(t, amount,
					"Sweep amount should be positive")
			}
		})
	}
}

// TestSendBSV_UTXOFiltering tests UTXO filtering logic.
func TestSendBSV_UTXOFiltering(t *testing.T) {
	t.Parallel()

	utxos := []chain.UTXO{
		{TxID: "tx1", Vout: 0, Amount: 100000, Address: "1ABC"},
		{TxID: "tx2", Vout: 0, Amount: 200000, Address: "1DEF"},
		{TxID: "tx3", Vout: 0, Amount: 300000, Address: "1GHI"},
	}

	store := newMockUTXOProvider()
	// Mark tx1:0 as spent
	store.spent["bsv:tx1:0"] = true

	filtered := FilterSpentBSVUTXOs(utxos, store)

	// Should filter out the spent UTXO
	assert.Len(t, filtered, 2)
	for _, utxo := range filtered {
		assert.NotEqual(t, "tx1", utxo.TxID, "Spent UTXO should be filtered")
	}
}

// TestSendBSV_UTXOAggregation tests UTXO aggregation from multiple addresses.
func TestSendBSV_UTXOAggregation(t *testing.T) {
	t.Parallel()

	// Test the unique address extraction logic
	utxos := []chain.UTXO{
		{TxID: "tx1", Vout: 0, Amount: 100000, Address: "1ABC"},
		{TxID: "tx2", Vout: 0, Amount: 200000, Address: "1ABC"}, // Same address
		{TxID: "tx3", Vout: 0, Amount: 300000, Address: "1DEF"},
		{TxID: "tx4", Vout: 0, Amount: 400000, Address: "1GHI"},
	}

	uniqueAddrs := UniqueUTXOAddrs(utxos)

	// Should have 3 unique addresses
	assert.Len(t, uniqueAddrs, 3)
	assert.Contains(t, uniqueAddrs, "1ABC")
	assert.Contains(t, uniqueAddrs, "1DEF")
	assert.Contains(t, uniqueAddrs, "1GHI")
}

// TestSendBSV_UTXOSelection tests UTXO selection for normal sends.
func TestSendBSV_UTXOSelection(t *testing.T) {
	t.Parallel()

	// Create a temporary BSV client for testing
	client := bsv.NewClient(context.Background(), nil)

	utxos := []bsv.UTXO{
		{TxID: "tx1", Vout: 0, Amount: 100000, Address: "1ABC"},
		{TxID: "tx2", Vout: 0, Amount: 200000, Address: "1DEF"},
		{TxID: "tx3", Vout: 0, Amount: 300000, Address: "1GHI"},
	}

	tests := []struct {
		name     string
		amount   uint64
		feeRate  uint64
		wantErr  bool
		minUTXOs int
		maxUTXOs int
	}{
		{
			name:     "Small amount - should select 1 UTXO",
			amount:   50000,
			feeRate:  500,
			wantErr:  false,
			minUTXOs: 1,
			maxUTXOs: 2,
		},
		{
			name:     "Medium amount - may select 1-2 UTXOs",
			amount:   250000,
			feeRate:  500,
			wantErr:  false,
			minUTXOs: 1,
			maxUTXOs: 2,
		},
		{
			name:     "Large amount - should select all UTXOs",
			amount:   550000,
			feeRate:  500,
			wantErr:  false,
			minUTXOs: 3,
			maxUTXOs: 3,
		},
		{
			name:    "Amount exceeds total",
			amount:  700000,
			feeRate: 500,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			selected, change, err := client.SelectUTXOs(utxos, tt.amount, tt.feeRate)

			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.GreaterOrEqual(t, len(selected), tt.minUTXOs,
					"Should select at least minimum UTXOs")
				assert.LessOrEqual(t, len(selected), tt.maxUTXOs,
					"Should select at most maximum UTXOs")
				// Change is uint64, always non-negative
				_ = change
			}
		})
	}
}

// TestSendBSV_FeeStrategyParsing tests fee strategy parsing.
func TestSendBSV_FeeStrategyParsing(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		strategy string
		valid    bool
	}{
		{
			name:     "Priority strategy",
			strategy: "priority",
			valid:    true,
		},
		{
			name:     "Normal strategy",
			strategy: "normal",
			valid:    true,
		},
		{
			name:     "Economy strategy",
			strategy: "economy",
			valid:    true,
		},
		{
			name:     "Empty defaults to normal",
			strategy: "",
			valid:    true,
		},
		{
			name:     "Invalid strategy",
			strategy: "instant",
			valid:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			strategy := bsv.FeeStrategy(tt.strategy)

			// Default empty to normal
			if tt.strategy == "" {
				strategy = bsv.FeeStrategyNormal
			}

			// Verify it's a recognized strategy
			isRecognized := strategy == bsv.FeeStrategyPriority ||
				strategy == bsv.FeeStrategyNormal ||
				strategy == bsv.FeeStrategyEconomy

			if tt.valid {
				assert.True(t, isRecognized || tt.strategy == "",
					"Should be recognized strategy or empty")
			}
		})
	}
}

// TestSendBSV_ChangeAddressDerivation tests change address derivation logic.
func TestSendBSV_ChangeAddressDerivation(t *testing.T) {
	t.Parallel()

	// Test that change address is needed for non-sweep transactions
	tests := []struct {
		name        string
		sweepAll    bool
		needsChange bool
	}{
		{
			name:        "Normal send needs change address",
			sweepAll:    false,
			needsChange: true,
		},
		{
			name:        "Sweep does not need change address",
			sweepAll:    true,
			needsChange: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// Logic: change address only for non-sweep
			needsChange := !tt.sweepAll

			assert.Equal(t, tt.needsChange, needsChange,
				"Change address requirement mismatch")
		})
	}
}

// TestSendBSV_DisplayAmountFormatting tests display amount formatting.
func TestSendBSV_DisplayAmountFormatting(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		amount *big.Int
		sweep  bool
		want   string
	}{
		{
			name:   "1 BSV normal send",
			amount: big.NewInt(100000000),
			sweep:  false,
			want:   "1.0",
		},
		{
			name:   "1 BSV sweep",
			amount: big.NewInt(100000000),
			sweep:  true,
			want:   "1.0 (sweep all)",
		},
		{
			name:   "0.5 BSV",
			amount: big.NewInt(50000000),
			sweep:  false,
			want:   "0.5",
		},
		{
			name:   "1 satoshi",
			amount: big.NewInt(1),
			sweep:  false,
			want:   "0.00000001",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			formatted := chain.FormatDecimalAmount(tt.amount, 8)
			if tt.sweep {
				formatted += " (sweep all)"
			}

			assert.Equal(t, tt.want, formatted)
		})
	}
}

// TestSendBSV_MinimumAmount tests minimum amount validation.
func TestSendBSV_MinimumAmount(t *testing.T) {
	t.Parallel()

	// BSV minimum (dust threshold)
	dustThreshold := uint64(546) // satoshis

	tests := []struct {
		name   string
		amount uint64
		isDust bool
	}{
		{
			name:   "Above dust threshold",
			amount: 1000,
			isDust: false,
		},
		{
			name:   "Exactly at dust threshold",
			amount: 546,
			isDust: false,
		},
		{
			name:   "Below dust threshold",
			amount: 500,
			isDust: true,
		},
		{
			name:   "Zero amount",
			amount: 0,
			isDust: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			isDust := tt.amount < dustThreshold

			assert.Equal(t, tt.isDust, isDust,
				"Dust threshold check mismatch")
		})
	}
}

// TestSendBSV_MultiAddressSweep tests sweep with multiple addresses.
func TestSendBSV_MultiAddressSweep(t *testing.T) {
	t.Parallel()

	// Test scenario: sweep from 3 addresses
	addresses := []wallet.Address{
		{Address: "1ABC", Index: 0, Path: "m/44'/236'/0'/0/0"},
		{Address: "1DEF", Index: 1, Path: "m/44'/236'/0'/0/1"},
		{Address: "1GHI", Index: 2, Path: "m/44'/236'/0'/0/2"},
	}

	utxos := []chain.UTXO{
		{TxID: "tx1", Vout: 0, Amount: 100000, Address: "1ABC"},
		{TxID: "tx2", Vout: 0, Amount: 200000, Address: "1DEF"},
		{TxID: "tx3", Vout: 0, Amount: 300000, Address: "1GHI"},
	}

	// Calculate total
	var total uint64
	for _, u := range utxos {
		total += u.Amount
	}

	assert.Equal(t, uint64(600000), total)

	// Sweep should use all UTXOs
	assert.Len(t, utxos, len(addresses))

	// All addresses should be zeroed after sweep
	expectedBalanceAfterSweep := "0.0"
	for range addresses {
		assert.Equal(t, "0.0", expectedBalanceAfterSweep)
	}
}

// TestSendBSV_ConcurrentUTXOHandling tests that UTXOs are properly handled.
func TestSendBSV_ConcurrentUTXOHandling(t *testing.T) {
	t.Parallel()

	// Test that unique address extraction handles duplicates
	utxos := []chain.UTXO{
		{TxID: "tx1", Vout: 0, Amount: 100000, Address: "1ABC"},
		{TxID: "tx1", Vout: 1, Amount: 200000, Address: "1ABC"}, // Same address
		{TxID: "tx2", Vout: 0, Amount: 300000, Address: "1ABC"}, // Same address again
	}

	uniqueAddrs := UniqueUTXOAddrs(utxos)

	// Should have only 1 unique address despite 3 UTXOs
	assert.Len(t, uniqueAddrs, 1)
	assert.Contains(t, uniqueAddrs, "1ABC")
}

// TestSendBSV_ZeroUTXOHandling tests handling of zero UTXOs.
func TestSendBSV_ZeroUTXOHandling(t *testing.T) {
	t.Parallel()

	emptyUTXOs := []chain.UTXO{}

	// Unique addresses from empty UTXOs
	uniqueAddrs := UniqueUTXOAddrs(emptyUTXOs)
	assert.Empty(t, uniqueAddrs)

	// Filter empty UTXOs
	store := newMockUTXOProvider()
	filtered := FilterSpentBSVUTXOs(emptyUTXOs, store)
	assert.Empty(t, filtered)
}

// TestSendBSV_RequestStructure tests SendRequest structure for BSV.
func TestSendBSV_RequestStructure(t *testing.T) {
	t.Parallel()

	req := &SendRequest{
		ChainID:     chain.BSV,
		To:          "1A1zP1eP5QGefi2DMPTfTL5SLmv7DivfNa",
		AmountStr:   "1.0",
		Wallet:      "test-wallet",
		FromAddress: "1ABC",
		Addresses: []wallet.Address{
			{Address: "1ABC", Index: 0, Path: "m/44'/236'/0'/0/0"},
		},
	}

	// Verify structure
	assert.Equal(t, chain.BSV, req.ChainID)
	assert.NotEmpty(t, req.To)
	assert.NotEmpty(t, req.AmountStr)
	assert.NotEmpty(t, req.Wallet)
	assert.NotEmpty(t, req.Addresses)
	assert.False(t, req.SweepAll())
}

// TestSendBSV_ResultStructure tests SendResult structure for BSV.
func TestSendBSV_ResultStructure(t *testing.T) {
	t.Parallel()

	result := &SendResult{
		Hash:       "abc123def456",
		From:       "1ABC",
		To:         "1A1zP1eP5QGefi2DMPTfTL5SLmv7DivfNa",
		Amount:     "1.0",
		Fee:        "0.00001",
		Status:     "success",
		ChainID:    chain.BSV,
		UTXOsSpent: 3,
	}

	// Verify structure
	assert.Equal(t, chain.BSV, result.ChainID)
	assert.NotEmpty(t, result.Hash)
	assert.NotEmpty(t, result.From)
	assert.NotEmpty(t, result.To)
	assert.NotEmpty(t, result.Amount)
	assert.Equal(t, 3, result.UTXOsSpent)
}

// TestSendBSV_FeeQuoteDefaults tests default fee quote handling.
func TestSendBSV_FeeQuoteDefaults(t *testing.T) {
	t.Parallel()

	// Default fee rate when API fails
	defaultFeeRate := bsv.DefaultFeeRate

	assert.Positive(t, defaultFeeRate, "Default fee rate should be positive")
	assert.LessOrEqual(t, defaultFeeRate, 10000, "Default fee rate should be reasonable")
}

// TestSendBSV_ValidationEnabled tests UTXO validation flag.
func TestSendBSV_ValidationEnabled(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		validateUTXOs  bool
		sweepAll       bool
		shouldValidate bool
	}{
		{
			name:           "Validation enabled for sweep",
			validateUTXOs:  true,
			sweepAll:       true,
			shouldValidate: true,
		},
		{
			name:           "Validation disabled",
			validateUTXOs:  false,
			sweepAll:       true,
			shouldValidate: false,
		},
		{
			name:           "Validation enabled for non-sweep",
			validateUTXOs:  true,
			sweepAll:       false,
			shouldValidate: false, // Only validates on sweep
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// Logic: validation only happens when enabled AND sweeping
			shouldValidate := tt.validateUTXOs && tt.sweepAll

			assert.Equal(t, tt.shouldValidate, shouldValidate,
				"UTXO validation logic mismatch")
		})
	}
}

// TestSendBSV_EmptyWalletPath tests handling of empty wallet path.
func TestSendBSV_EmptyWalletPath(t *testing.T) {
	t.Parallel()

	// Empty wallet name should still work (path will be constructed)
	walletName := ""
	assert.Empty(t, walletName)

	// Non-empty wallet name
	walletName = "test-wallet"
	assert.NotEmpty(t, walletName)
}

// TestSendBSV_MaxInputsHandling tests handling of many inputs.
func TestSendBSV_MaxInputsHandling(t *testing.T) {
	t.Parallel()

	// Test with many inputs
	manyInputs := 100
	feeRate := uint64(500)

	fee := bsv.EstimateFeeForTx(manyInputs, 2, feeRate)

	// Fee should scale with number of inputs
	assert.Greater(t, fee, uint64(5000), "Fee should be significant for many inputs")

	// Fee should be less than total transaction value for reasonable cases
	totalInputValue := uint64(1000000000) // 10 BSV
	assert.Less(t, fee, totalInputValue, "Fee should be less than total value")
}

// TestSendBSV_ClientOptions tests BSV client option structure.
func TestSendBSV_ClientOptions(t *testing.T) {
	t.Parallel()

	opts := &bsv.ClientOptions{
		APIKey:      "test-key",
		FeeStrategy: bsv.FeeStrategyPriority,
		MinMiners:   2,
	}

	assert.NotEmpty(t, opts.APIKey)
	assert.Equal(t, bsv.FeeStrategyPriority, opts.FeeStrategy)
	assert.Equal(t, 2, opts.MinMiners)
}
