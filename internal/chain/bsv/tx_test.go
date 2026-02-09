package bsv

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mrz1836/sigil/internal/chain"
	"github.com/mrz1836/sigil/internal/wallet"
)

func TestTxBuilder_AddInput(t *testing.T) {
	builder := NewTxBuilder()

	utxo := UTXO{
		TxID:    "a1b2c3d4e5f6a7b8c9d0e1f2a3b4c5d6e7f8a9b0c1d2e3f4a5b6c7d8e9f0a1b2",
		Vout:    0,
		Amount:  100000,
		Address: "1BvBMSEYstWetqTFn5Au4m4GFg7xJaNVN2",
	}

	err := builder.AddInput(utxo)
	require.NoError(t, err)
	assert.Len(t, builder.Inputs, 1)
	assert.Equal(t, utxo, builder.Inputs[0])
}

func TestTxBuilder_AddOutput(t *testing.T) {
	builder := NewTxBuilder()

	err := builder.AddOutput("1A1zP1eP5QGefi2DMPTfTL5SLmv7DivfNa", 50000)
	require.NoError(t, err)
	assert.Len(t, builder.Outputs, 1)
}

func TestTxBuilder_AddOutput_InvalidAddress(t *testing.T) {
	builder := NewTxBuilder()

	err := builder.AddOutput("invalid", 50000)
	assert.Error(t, err)
}

func TestTxBuilder_AddOutput_ZeroAmount(t *testing.T) {
	builder := NewTxBuilder()

	err := builder.AddOutput("1A1zP1eP5QGefi2DMPTfTL5SLmv7DivfNa", 0)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "dust")
}

func TestTxBuilder_CalculateFee(t *testing.T) {
	builder := NewTxBuilder()

	// Add 1 input
	_ = builder.AddInput(UTXO{
		TxID:    "a1b2c3d4e5f6a7b8c9d0e1f2a3b4c5d6e7f8a9b0c1d2e3f4a5b6c7d8e9f0a1b2",
		Vout:    0,
		Amount:  100000,
		Address: "1BvBMSEYstWetqTFn5Au4m4GFg7xJaNVN2",
	})

	// Add 2 outputs
	_ = builder.AddOutput("1A1zP1eP5QGefi2DMPTfTL5SLmv7DivfNa", 50000)
	_ = builder.AddOutput("1BvBMSEYstWetqTFn5Au4m4GFg7xJaNVN2", 49000) // Change

	// Calculate fee at 1000 sat/KB (equivalent to 1 sat/byte)
	fee := builder.CalculateFee(1000)
	expectedSize := EstimateTxSize(1, 2)
	assert.Equal(t, expectedSize, fee)
}

func TestTxBuilder_Validate(t *testing.T) {
	tests := []struct {
		name    string
		setup   func(*TxBuilder)
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid transaction",
			setup: func(b *TxBuilder) {
				_ = b.AddInput(UTXO{
					TxID:    "a1b2c3d4e5f6a7b8c9d0e1f2a3b4c5d6e7f8a9b0c1d2e3f4a5b6c7d8e9f0a1b2",
					Vout:    0,
					Amount:  100000,
					Address: "1BvBMSEYstWetqTFn5Au4m4GFg7xJaNVN2",
				})
				_ = b.AddOutput("1A1zP1eP5QGefi2DMPTfTL5SLmv7DivfNa", 99000)
			},
			wantErr: false,
		},
		{
			name:    "no inputs",
			setup:   func(_ *TxBuilder) {},
			wantErr: true,
			errMsg:  "inputs",
		},
		{
			name: "no outputs",
			setup: func(b *TxBuilder) {
				_ = b.AddInput(UTXO{
					TxID:    "a1b2c3d4e5f6a7b8c9d0e1f2a3b4c5d6e7f8a9b0c1d2e3f4a5b6c7d8e9f0a1b2",
					Vout:    0,
					Amount:  100000,
					Address: "1BvBMSEYstWetqTFn5Au4m4GFg7xJaNVN2",
				})
			},
			wantErr: true,
			errMsg:  "outputs",
		},
		{
			name: "insufficient funds",
			setup: func(b *TxBuilder) {
				_ = b.AddInput(UTXO{
					TxID:    "a1b2c3d4e5f6a7b8c9d0e1f2a3b4c5d6e7f8a9b0c1d2e3f4a5b6c7d8e9f0a1b2",
					Vout:    0,
					Amount:  1000, // Only 1000 satoshis
					Address: "1BvBMSEYstWetqTFn5Au4m4GFg7xJaNVN2",
				})
				_ = b.AddOutput("1A1zP1eP5QGefi2DMPTfTL5SLmv7DivfNa", 100000) // Want 100000
			},
			wantErr: true,
			errMsg:  "insufficient",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			builder := NewTxBuilder()
			tc.setup(builder)

			err := builder.Validate()
			if tc.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tc.errMsg)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestTxBuilder_TotalInputAmount(t *testing.T) {
	builder := NewTxBuilder()

	_ = builder.AddInput(UTXO{TxID: "tx1", Vout: 0, Amount: 100000})
	_ = builder.AddInput(UTXO{TxID: "tx2", Vout: 1, Amount: 50000})

	assert.Equal(t, uint64(150000), builder.TotalInputAmount())
}

func TestTxBuilder_TotalOutputAmount(t *testing.T) {
	builder := NewTxBuilder()

	_ = builder.AddOutput("1A1zP1eP5QGefi2DMPTfTL5SLmv7DivfNa", 60000)
	_ = builder.AddOutput("1BvBMSEYstWetqTFn5Au4m4GFg7xJaNVN2", 40000)

	assert.Equal(t, uint64(100000), builder.TotalOutputAmount())
}

func TestZeroBytes(t *testing.T) {
	key := []byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10}

	wallet.ZeroBytes(key)

	for i, b := range key {
		assert.Equal(t, byte(0), b, "byte at position %d should be zero", i)
	}
}

// TestAddOutput_DustLimitEdgeCases tests dust limit boundary conditions.
// BSV removed dust limits in 2018, so 1 satoshi is the minimum valid output.
func TestAddOutput_DustLimitEdgeCases(t *testing.T) {
	t.Parallel()

	dustLimit := chain.BSV.DustLimit() // 1 satoshi for BSV

	tests := []struct {
		name    string
		amount  uint64
		wantErr bool
		errMsg  string
	}{
		{
			name:    "single satoshi (BSV minimum) succeeds",
			amount:  1,
			wantErr: false,
		},
		{
			name:    "zero amount fails",
			amount:  0,
			wantErr: true,
			errMsg:  "dust",
		},
		{
			name:    "exact dust limit succeeds",
			amount:  dustLimit,
			wantErr: false,
		},
		{
			name:    "two satoshis succeeds",
			amount:  2,
			wantErr: false,
		},
		{
			name:    "significantly above dust (10000) succeeds",
			amount:  10000,
			wantErr: false,
		},
		{
			name:    "old BTC dust limit (546) succeeds on BSV",
			amount:  546,
			wantErr: false,
		},
		{
			name:    "1 BSV (100000000 satoshis) succeeds",
			amount:  100000000,
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			builder := NewTxBuilder()

			err := builder.AddOutput(validAddress(), tt.amount)

			if tt.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errMsg)
			} else {
				require.NoError(t, err)
				assert.Len(t, builder.Outputs, 1)
				assert.Equal(t, tt.amount, builder.Outputs[0].Amount)
			}
		})
	}
}

// TestAddOutput_MultipleOutputs tests adding multiple outputs in various configurations.
func TestAddOutput_MultipleOutputs(t *testing.T) {
	t.Parallel()

	dustLimit := chain.BSV.DustLimit() // 1 satoshi for BSV

	tests := []struct {
		name    string
		outputs []struct {
			address string
			amount  uint64
		}
		wantCount int
	}{
		{
			name: "2 outputs same address",
			outputs: []struct {
				address string
				amount  uint64
			}{
				{validAddress(), 10000},
				{validAddress(), 10000},
			},
			wantCount: 2,
		},
		{
			name: "2 outputs different addresses",
			outputs: []struct {
				address string
				amount  uint64
			}{
				{validAddress(), 10000},
				{validAddress2(), 20000},
			},
			wantCount: 2,
		},
		{
			name: "5 outputs batch payment",
			outputs: []struct {
				address string
				amount  uint64
			}{
				{validAddress(), 1000},
				{validAddress2(), 2000},
				{validAddress(), 3000},
				{validAddress2(), 4000},
				{validAddress(), 5000},
			},
			wantCount: 5,
		},
		{
			name: "10 outputs large batch",
			outputs: []struct {
				address string
				amount  uint64
			}{
				{validAddress(), 1000},
				{validAddress2(), 1000},
				{validAddress(), 1000},
				{validAddress2(), 1000},
				{validAddress(), 1000},
				{validAddress2(), 1000},
				{validAddress(), 1000},
				{validAddress2(), 1000},
				{validAddress(), 1000},
				{validAddress2(), 1000},
			},
			wantCount: 10,
		},
		{
			name: "mixed amounts near dust limit (BSV = 1 sat)",
			outputs: []struct {
				address string
				amount  uint64
			}{
				{validAddress(), dustLimit},      // Exactly at dust limit (1 sat)
				{validAddress2(), dustLimit + 1}, // Just above (2 sats)
				{validAddress(), 1000},           // Well above
			},
			wantCount: 3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			builder := NewTxBuilder()

			for _, out := range tt.outputs {
				err := builder.AddOutput(out.address, out.amount)
				require.NoError(t, err)
			}

			assert.Len(t, builder.Outputs, tt.wantCount)

			// Verify total output amount
			var expectedTotal uint64
			for _, out := range tt.outputs {
				expectedTotal += out.amount
			}
			assert.Equal(t, expectedTotal, builder.TotalOutputAmount())
		})
	}
}

// TestAddOutput_100Outputs tests adding 100 outputs (stress test).
func TestAddOutput_100Outputs(t *testing.T) {
	t.Parallel()
	builder := NewTxBuilder()

	dustLimit := chain.BSV.DustLimit() // 1 satoshi for BSV

	for i := 0; i < 100; i++ {
		addr := validAddress()
		if i%2 == 1 {
			addr = validAddress2()
		}
		err := builder.AddOutput(addr, dustLimit)
		require.NoError(t, err, "failed to add output %d", i)
	}

	assert.Len(t, builder.Outputs, 100)
	assert.Equal(t, uint64(100)*dustLimit, builder.TotalOutputAmount())
}

// TestTxBuilder_Validate_EdgeCases tests validation edge cases.
//
//nolint:gocognit // Table-driven test with setup functions is inherently complex
func TestTxBuilder_Validate_EdgeCases(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		setupFunc  func(*TxBuilder)
		wantErr    bool
		errContain string
	}{
		{
			name: "exact balance (input = output + fee)",
			setupFunc: func(b *TxBuilder) {
				// 1 input, 1 output: size = 10 + 148 + 34 = 192 bytes, fee = 10 satoshis (at 50 sat/KB)
				_ = b.AddInput(makeUTXO(testTxID(1), 100000))
				_ = b.AddOutput(validAddress(), 100000-10) // Exactly covers fee
			},
			wantErr: false,
		},
		{
			name: "one satoshi short of covering fee",
			setupFunc: func(b *TxBuilder) {
				// Need 10 for fee (at 50 sat/KB), but only have 9 spare
				_ = b.AddInput(makeUTXO(testTxID(1), 100000))
				_ = b.AddOutput(validAddress(), 100000-9) // One satoshi short
			},
			wantErr:    true,
			errContain: "insufficient",
		},
		{
			name: "large tx with 100 inputs",
			setupFunc: func(b *TxBuilder) {
				// 100 inputs, 1 output: size = 10 + (100*148) + 34 = 14844 bytes
				// Fee at 50 sat/KB = (14844*50+999)/1000 = 743 satoshis
				for i := 0; i < 100; i++ {
					_ = b.AddInput(makeUTXO(testTxID(i), 1000))
				}
				// Total input: 100 * 1000 = 100000
				// Fee: 743
				// Available for output: 100000 - 743 = 99257
				_ = b.AddOutput(validAddress(), 99257)
			},
			wantErr: false,
		},
		{
			name: "large tx with 100 outputs",
			setupFunc: func(b *TxBuilder) {
				// 1 input, 100 outputs: size = 10 + 148 + (100*34) = 3558 bytes
				// Fee at 50 sat/KB = (3558*50+999)/1000 = 178 satoshis
				_ = b.AddInput(makeUTXO(testTxID(1), 5000000)) // 5M satoshis
				dustLimit := chain.BSV.DustLimit()             // 1 satoshi for BSV
				for i := 0; i < 100; i++ {
					// 100 outputs at dust limit = 100 satoshis (BSV)
					_ = b.AddOutput(validAddress(), dustLimit)
				}
				// Total output: 100 * 1 = 100 (BSV dust limit is 1 sat)
				// Fee: 3558
				// Needed: 100 + 3558 = 3658
				// Have: 5000000 - plenty of room
			},
			wantErr: false,
		},
		{
			name: "fee rate affects validation - high fee rate insufficient funds",
			setupFunc: func(b *TxBuilder) {
				b.SetFeeRate(50000) // Max fee rate (50000 sat/KB = 50 sat/byte)
				// 1 input, 1 output: size = 192, fee = (192*50000+999)/1000 = 9600
				_ = b.AddInput(makeUTXO(testTxID(1), 10000))
				_ = b.AddOutput(validAddress(), 1000) // Valid output, but 10000 < 1000 + 9600
			},
			wantErr:    true,
			errContain: "insufficient",
		},
		{
			name: "fee rate 50000 with adequate funds",
			setupFunc: func(b *TxBuilder) {
				b.SetFeeRate(50000) // Max fee rate (50000 sat/KB = 50 sat/byte)
				// 1 input, 1 output: size = 192, fee = (192*50000+999)/1000 = 9600
				_ = b.AddInput(makeUTXO(testTxID(1), 100000))
				_ = b.AddOutput(validAddress(), 100000-9600) // 90400 satoshis
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			builder := NewTxBuilder()
			tt.setupFunc(builder)

			err := builder.Validate()

			if tt.wantErr {
				require.Error(t, err)
				if tt.errContain != "" {
					assert.Contains(t, err.Error(), tt.errContain)
				}
			} else {
				require.NoError(t, err)
			}
		})
	}
}

// TestTxBuilder_SetFeeRate tests fee rate setting and validation.
func TestTxBuilder_SetFeeRate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		inputRate    uint64
		expectedRate uint64
	}{
		{"zero clamps to minimum", 0, MinFeeRate},
		{"minimum stays at minimum", MinFeeRate, MinFeeRate},
		{"mid-range stays unchanged", 25000, 25000},
		{"maximum stays at maximum", MaxFeeRate, MaxFeeRate},
		{"above maximum clamps to maximum", 100000, MaxFeeRate},
		{"way above maximum clamps to maximum", 1000000, MaxFeeRate},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			builder := NewTxBuilder()

			builder.SetFeeRate(tt.inputRate)

			assert.Equal(t, tt.expectedRate, builder.FeeRate)
		})
	}
}

// TestTxBuilder_CalculateFee_Comprehensive tests fee calculation for various tx sizes.
func TestTxBuilder_CalculateFee_Comprehensive(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		numInputs  int
		numOutputs int
		feeRate    uint64
		expected   uint64
	}{
		{
			name:       "1 input, 1 output @ 1000 sat/KB",
			numInputs:  1,
			numOutputs: 1,
			feeRate:    1000,
			expected:   192, // (192*1000+999)/1000 = 192
		},
		{
			name:       "1 input, 2 outputs @ 1000 sat/KB",
			numInputs:  1,
			numOutputs: 2,
			feeRate:    1000,
			expected:   226, // (226*1000+999)/1000 = 226
		},
		{
			name:       "10 inputs, 1 output @ 1000 sat/KB",
			numInputs:  10,
			numOutputs: 1,
			feeRate:    1000,
			expected:   1524, // (1524*1000+999)/1000 = 1524
		},
		{
			name:       "1 input, 10 outputs @ 1000 sat/KB",
			numInputs:  1,
			numOutputs: 10,
			feeRate:    1000,
			expected:   498, // (498*1000+999)/1000 = 498
		},
		{
			name:       "100 inputs, 100 outputs @ 1000 sat/KB",
			numInputs:  100,
			numOutputs: 100,
			feeRate:    1000,
			expected:   18210, // (18210*1000+999)/1000 = 18210
		},
		{
			name:       "1 input, 2 outputs @ 50000 sat/KB",
			numInputs:  1,
			numOutputs: 2,
			feeRate:    50000,
			expected:   11300, // (226*50000+999)/1000 = 11300
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			builder := NewTxBuilder()

			// Add inputs
			for i := 0; i < tt.numInputs; i++ {
				_ = builder.AddInput(makeUTXO(testTxID(i), 1000000))
			}

			// Add outputs
			for i := 0; i < tt.numOutputs; i++ {
				_ = builder.AddOutput(validAddress(), 1000)
			}

			fee := builder.CalculateFee(tt.feeRate)
			assert.Equal(t, tt.expected, fee)
		})
	}
}

// TestTxBuilder_TotalAmounts_Empty tests totals with no inputs/outputs.
func TestTxBuilder_TotalAmounts_Empty(t *testing.T) {
	t.Parallel()
	builder := NewTxBuilder()

	assert.Equal(t, uint64(0), builder.TotalInputAmount())
	assert.Equal(t, uint64(0), builder.TotalOutputAmount())
}

// TestTxBuilder_TotalAmounts_Large tests totals with large amounts.
func TestTxBuilder_TotalAmounts_Large(t *testing.T) {
	t.Parallel()
	builder := NewTxBuilder()

	// Add inputs totaling 21 million BSV (max supply)
	_ = builder.AddInput(makeUTXO(testTxID(1), 2100000000000000))

	// Add output of 1 million BSV
	_ = builder.AddOutput(validAddress(), 100000000000000)

	assert.Equal(t, uint64(2100000000000000), builder.TotalInputAmount())
	assert.Equal(t, uint64(100000000000000), builder.TotalOutputAmount())
}

// TestZeroBytes_Empty tests zeroing an empty key.
func TestZeroBytes_Empty(t *testing.T) {
	t.Parallel()
	key := []byte{}
	wallet.ZeroBytes(key) // Should not panic
	assert.Empty(t, key)
}

// TestZeroBytes_32Bytes tests zeroing a standard 32-byte private key.
func TestZeroBytes_32Bytes(t *testing.T) {
	t.Parallel()
	key := make([]byte, 32)
	for i := range key {
		key[i] = byte(i + 1)
	}

	wallet.ZeroBytes(key)

	for i, b := range key {
		assert.Equal(t, byte(0), b, "byte at position %d should be zero", i)
	}
}
