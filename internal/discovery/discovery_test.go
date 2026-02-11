package discovery

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	sigilerr "github.com/mrz1836/sigil/pkg/errors"
)

// TestOptions_Validate_EdgeCases tests additional validation edge cases beyond scanner_test.go.
func TestOptions_Validate_EdgeCases(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		opts    *Options
		wantErr bool
		errCode string
	}{
		{
			name: "Extremely large gap limit",
			opts: &Options{
				GapLimit:      10000,
				MaxConcurrent: 3,
			},
			wantErr: false,
		},
		{
			name: "Extremely large max concurrent",
			opts: &Options{
				GapLimit:      20,
				MaxConcurrent: 10000,
			},
			wantErr: false,
		},
		{
			name: "Very negative gap limit",
			opts: &Options{
				GapLimit:      -10000,
				MaxConcurrent: 3,
			},
			wantErr: true,
			errCode: ErrInvalidGapLimit.Code,
		},
		{
			name: "Very negative max concurrent",
			opts: &Options{
				GapLimit:      20,
				MaxConcurrent: -10000,
			},
			wantErr: true,
			errCode: ErrInvalidMaxConcurrent.Code,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := tt.opts.Validate()

			if tt.wantErr {
				require.Error(t, err)
				var sigilErr *sigilerr.SigilError
				if assert.ErrorAs(t, err, &sigilErr) {
					assert.Equal(t, tt.errCode, sigilErr.Code)
				}
			} else {
				require.NoError(t, err)
			}
		})
	}
}

// TestDefaultOptions_FieldValues verifies specific default option values.
func TestDefaultOptions_FieldValues(t *testing.T) {
	t.Parallel()

	opts := DefaultOptions()

	// Verify specific numeric values
	assert.Equal(t, DefaultGapLimit, opts.GapLimit)
	assert.Equal(t, ExtendedGapLimit, opts.ExtendedGapLimit)
	assert.Equal(t, DefaultMaxConcurrent, opts.MaxConcurrent)

	// Verify boolean defaults
	assert.True(t, opts.ScanChangeAddresses)

	// Verify slice/map defaults
	assert.NotNil(t, opts.PathSchemes)
	assert.Nil(t, opts.CustomPaths)
	assert.Nil(t, opts.Passphrases)
	assert.Nil(t, opts.ProgressCallback)
}

// TestResult_HasFunds_EdgeCases tests additional edge cases for HasFunds.
func TestResult_HasFunds_EdgeCases(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		result *Result
		want   bool
	}{
		{
			name: "Max uint64 balance",
			result: &Result{
				TotalBalance: ^uint64(0), // Max uint64
			},
			want: true,
		},
		{
			name: "Addresses but zero total balance",
			result: &Result{
				TotalBalance: 0,
				FoundAddresses: map[string][]DiscoveredAddress{
					"bip44": {
						{Address: "1ABC", Balance: 0},
						{Address: "1DEF", Balance: 0},
					},
				},
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := tt.result.HasFunds()
			assert.Equal(t, tt.want, got)
		})
	}
}

// TestResult_AllAddresses_OrderIndependent tests that AllAddresses returns all addresses.
func TestResult_AllAddresses_OrderIndependent(t *testing.T) {
	t.Parallel()

	result := &Result{
		FoundAddresses: map[string][]DiscoveredAddress{
			"scheme1": {
				{Address: "1ABC", Balance: 100},
				{Address: "1DEF", Balance: 200},
			},
			"scheme2": {
				{Address: "1GHI", Balance: 300},
			},
			"scheme3": {
				{Address: "1JKL", Balance: 400},
				{Address: "1MNO", Balance: 500},
				{Address: "1PQR", Balance: 600},
			},
		},
	}

	addresses := result.AllAddresses()

	// Verify count
	assert.Len(t, addresses, 6)

	// Verify all expected addresses are present (order doesn't matter)
	expectedAddresses := []string{"1ABC", "1DEF", "1GHI", "1JKL", "1MNO", "1PQR"}
	foundAddresses := make(map[string]bool)
	for _, addr := range addresses {
		foundAddresses[addr.Address] = true
	}

	for _, expected := range expectedAddresses {
		assert.True(t, foundAddresses[expected], "Expected address %s not found", expected)
	}
}

// TestResult_AddressesByScheme_NilMap tests AddressesByScheme with nil FoundAddresses.
func TestResult_AddressesByScheme_NilMap(t *testing.T) {
	t.Parallel()

	result := &Result{
		FoundAddresses: nil,
	}

	addresses := result.AddressesByScheme("bip44")
	assert.Nil(t, addresses)
}

// TestErrors_AllFields tests that all error constants have required fields.
func TestErrors_AllFields(t *testing.T) {
	t.Parallel()

	errors := []*sigilerr.SigilError{
		ErrNoFundsFound,
		ErrInvalidSeed,
		ErrScanCanceled,
		ErrInvalidGapLimit,
		ErrInvalidMaxConcurrent,
	}

	for _, err := range errors {
		assert.NotEmpty(t, err.Code, "Error should have a code")
		assert.NotEmpty(t, err.Message, "Error should have a message")
		assert.NotEqual(t, 0, err.ExitCode, "Error should have an exit code")
	}
}

// TestProgressUpdate_AllFields tests all ProgressUpdate fields.
func TestProgressUpdate_AllFields(t *testing.T) {
	t.Parallel()

	update := ProgressUpdate{
		Phase:            "test-phase",
		SchemeName:       "test-scheme",
		AddressesScanned: 42,
		UTXOsFound:       10,
		BalanceFound:     123456,
		CurrentAddress:   "test-address",
		Message:          "test-message",
	}

	// Verify all fields are set correctly
	assert.Equal(t, "test-phase", update.Phase)
	assert.Equal(t, "test-scheme", update.SchemeName)
	assert.Equal(t, 42, update.AddressesScanned)
	assert.Equal(t, 10, update.UTXOsFound)
	assert.Equal(t, uint64(123456), update.BalanceFound)
	assert.Equal(t, "test-address", update.CurrentAddress)
	assert.Equal(t, "test-message", update.Message)
}

// TestDiscoveredAddress_AllFields tests all DiscoveredAddress fields.
func TestDiscoveredAddress_AllFields(t *testing.T) {
	t.Parallel()

	addr := DiscoveredAddress{
		Address:    "test-address",
		Path:       "m/44'/236'/0'/0/10",
		SchemeName: "test-scheme",
		Balance:    999999,
		UTXOCount:  5,
		IsChange:   true,
		Index:      10,
		Account:    1,
		CoinType:   236,
	}

	// Verify all fields
	assert.Equal(t, "test-address", addr.Address)
	assert.Equal(t, "m/44'/236'/0'/0/10", addr.Path)
	assert.Equal(t, "test-scheme", addr.SchemeName)
	assert.Equal(t, uint64(999999), addr.Balance)
	assert.Equal(t, 5, addr.UTXOCount)
	assert.True(t, addr.IsChange)
	assert.Equal(t, uint32(10), addr.Index)
	assert.Equal(t, uint32(1), addr.Account)
	assert.Equal(t, uint32(236), addr.CoinType)
}

// TestResult_AllFields tests all Result fields.
func TestResult_AllFields(t *testing.T) {
	t.Parallel()

	result := Result{
		FoundAddresses: map[string][]DiscoveredAddress{
			"scheme1": {{Address: "addr1"}},
		},
		TotalBalance:     123456,
		TotalUTXOs:       10,
		SchemesScanned:   []string{"scheme1", "scheme2"},
		AddressesScanned: 50,
		Duration:         30 * time.Second,
		PassphraseUsed:   true,
		Errors:           []string{"error1", "error2"},
	}

	// Verify all fields
	assert.NotNil(t, result.FoundAddresses)
	assert.Equal(t, uint64(123456), result.TotalBalance)
	assert.Equal(t, 10, result.TotalUTXOs)
	assert.Len(t, result.SchemesScanned, 2)
	assert.Equal(t, 50, result.AddressesScanned)
	assert.Equal(t, 30*time.Second, result.Duration)
	assert.True(t, result.PassphraseUsed)
	assert.Len(t, result.Errors, 2)
}

// TestUTXO_AllFields tests all UTXO fields.
func TestUTXO_AllFields(t *testing.T) {
	t.Parallel()

	utxo := UTXO{
		TxID:          "test-txid",
		Vout:          1,
		Amount:        50000,
		ScriptPubKey:  "test-script",
		Address:       "test-address",
		Confirmations: 10,
	}

	// Verify all fields
	assert.Equal(t, "test-txid", utxo.TxID)
	assert.Equal(t, uint32(1), utxo.Vout)
	assert.Equal(t, uint64(50000), utxo.Amount)
	assert.Equal(t, "test-script", utxo.ScriptPubKey)
	assert.Equal(t, "test-address", utxo.Address)
	assert.Equal(t, uint32(10), utxo.Confirmations)
}

// TestConstants_Values verifies package constant values.
func TestConstants_Values(t *testing.T) {
	t.Parallel()

	assert.Equal(t, 20, DefaultGapLimit)
	assert.Equal(t, 50, ExtendedGapLimit)
	assert.Equal(t, 3, DefaultMaxConcurrent)
	assert.Equal(t, 5*time.Minute, DefaultTimeout)

	// Verify constants are positive
	assert.Positive(t, DefaultGapLimit)
	assert.Positive(t, ExtendedGapLimit)
	assert.Positive(t, DefaultMaxConcurrent)
	assert.Positive(t, DefaultTimeout)

	// Verify ExtendedGapLimit > DefaultGapLimit
	assert.Greater(t, ExtendedGapLimit, DefaultGapLimit)
}

// TestOptions_ZeroValues tests Options with all zero/nil values.
func TestOptions_ZeroValues(t *testing.T) {
	t.Parallel()

	opts := &Options{}

	// Numeric fields should be zero
	assert.Equal(t, 0, opts.GapLimit)
	assert.Equal(t, 0, opts.ExtendedGapLimit)
	assert.Equal(t, 0, opts.MaxConcurrent)

	// Boolean fields should be false
	assert.False(t, opts.ScanChangeAddresses)

	// Pointer/slice fields should be nil
	assert.Nil(t, opts.PathSchemes)
	assert.Nil(t, opts.CustomPaths)
	assert.Nil(t, opts.Passphrases)
	assert.Nil(t, opts.ProgressCallback)

	// Zero-value Options should fail validation
	err := opts.Validate()
	assert.Error(t, err)
}

// TestProgressCallback_FunctionType tests ProgressCallback function type.
func TestProgressCallback_FunctionType(t *testing.T) {
	t.Parallel()

	// Test that we can assign a function to ProgressCallback
	var callback ProgressCallback = func(update ProgressUpdate) {
		// Verify update fields are accessible
		_ = update.Phase
		_ = update.SchemeName
		_ = update.AddressesScanned
	}

	assert.NotNil(t, callback)

	// Test that we can call the callback
	callback(ProgressUpdate{
		Phase:            "test",
		AddressesScanned: 5,
	})
}
