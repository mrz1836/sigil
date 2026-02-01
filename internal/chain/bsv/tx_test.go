package bsv

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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

	// Calculate fee at 1 sat/byte
	fee := builder.CalculateFee(1)
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

func TestZeroPrivateKey(t *testing.T) {
	key := []byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10}

	ZeroPrivateKey(key)

	for i, b := range key {
		assert.Equal(t, byte(0), b, "byte at position %d should be zero", i)
	}
}
