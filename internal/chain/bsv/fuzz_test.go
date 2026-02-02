package bsv

import (
	"testing"
)

// FuzzIsValidAddress tests that BSV address validation never panics.
func FuzzIsValidAddress(f *testing.F) {
	// Valid P2PKH addresses (start with 1)
	f.Add("1A1zP1eP5QGefi2DMPTfTL5SLmv7DivfNa") // Genesis block address
	f.Add("1BvBMSEYstWetqTFn5Au4m4GFg7xJaNVN2")

	// Valid P2SH addresses (start with 3)
	f.Add("3J98t1WpEZ73CNmQviecrnyiWrnqRhWNLy")

	// Invalid addresses
	f.Add("")
	f.Add("0x742d35Cc6634C0532925a3b844Bc9e7595f8b2E0") // ETH address
	f.Add("1")                                          // Too short
	f.Add("1AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA")        // Invalid checksum
	f.Add("1111111111111111111111111111111111")         // All 1s
	f.Add("OIl0")                                       // Invalid base58 chars (O, I, l, 0)
	f.Add("\x00\x01\x02")

	f.Fuzz(func(t *testing.T, input string) {
		// Should not panic
		result := IsValidAddress(input)

		// If valid, verify it starts with 1 or 3
		if result && len(input) > 0 {
			first := input[0]
			if first != '1' && first != '3' {
				t.Errorf("IsValidAddress returned true for address not starting with 1 or 3: %q", input)
			}
		}
	})
}

// FuzzValidateBase58CheckAddress tests full address validation.
func FuzzValidateBase58CheckAddress(f *testing.F) {
	f.Add("1A1zP1eP5QGefi2DMPTfTL5SLmv7DivfNa")
	f.Add("3J98t1WpEZ73CNmQviecrnyiWrnqRhWNLy")
	f.Add("")
	f.Add("invalid")
	f.Add("\x00\x01\x02")

	f.Fuzz(func(_ *testing.T, input string) {
		// Should not panic - that's the main test
		_ = ValidateBase58CheckAddress(input)
	})
}

// FuzzDecodeBase58Check tests base58check decoding.
func FuzzDecodeBase58Check(f *testing.F) {
	f.Add("1A1zP1eP5QGefi2DMPTfTL5SLmv7DivfNa")
	f.Add("3J98t1WpEZ73CNmQviecrnyiWrnqRhWNLy")
	f.Add("")
	f.Add("1")
	f.Add("111111111111111111111111111111111")
	f.Add("\x00\x01\x02")

	f.Fuzz(func(_ *testing.T, input string) {
		// Should not panic - that's the main test
		_, _, _ = DecodeBase58Check(input)
	})
}

// FuzzBase58Decode tests raw base58 decoding.
func FuzzBase58Decode(f *testing.F) {
	f.Add("1A1zP1eP5QGefi2DMPTfTL5SLmv7DivfNa")
	f.Add("")
	f.Add("1")
	f.Add("1111")
	f.Add("abc")
	f.Add("OIl0") // Invalid chars

	f.Fuzz(func(_ *testing.T, input string) {
		// Should not panic - that's the main test
		_, _ = base58Decode(input)
	})
}

// FuzzBase58Encode tests base58 encoding.
func FuzzBase58Encode(f *testing.F) {
	f.Add([]byte{0x00, 0x01, 0x02, 0x03})
	f.Add([]byte{0x00, 0x00, 0x00, 0x01})
	f.Add([]byte{0x01}) // Non-empty minimal input
	f.Add([]byte{0xff, 0xff, 0xff, 0xff})

	f.Fuzz(func(t *testing.T, input []byte) {
		// Skip empty input - base58Encode returns empty string which base58Decode rejects
		if len(input) == 0 {
			return
		}

		// Should not panic
		encoded := base58Encode(input)

		// Skip if encoded is empty (happens for all-zero input with no content)
		if encoded == "" {
			return
		}

		// All characters should be valid base58
		validateBase58Chars(t, encoded, input)

		// Decode should succeed for non-empty encoded values
		_, err := base58Decode(encoded)
		if err != nil {
			t.Errorf("base58Decode failed for encoded value %q (input %v): %v", encoded, input, err)
		}
	})
}

func validateBase58Chars(t *testing.T, encoded string, input []byte) {
	t.Helper()
	for _, c := range encoded {
		if _, ok := base58AlphabetMap[c]; !ok {
			t.Errorf("base58Encode returned invalid character '%c' for input %v", c, input)
		}
	}
}

// FuzzTxBuilder_AddOutput tests that AddOutput never panics on any input.
func FuzzTxBuilder_AddOutput(f *testing.F) {
	// Valid addresses
	f.Add("1A1zP1eP5QGefi2DMPTfTL5SLmv7DivfNa", uint64(10000))
	f.Add("1BvBMSEYstWetqTFn5Au4m4GFg7xJaNVN2", uint64(546))
	f.Add("3J98t1WpEZ73CNmQviecrnyiWrnqRhWNLy", uint64(100000000))

	// Invalid addresses
	f.Add("", uint64(10000))
	f.Add("invalid", uint64(10000))
	f.Add("0x742d35Cc6634C0532925a3b844Bc9e7595f8b2E0", uint64(10000))

	// Edge case amounts
	f.Add("1A1zP1eP5QGefi2DMPTfTL5SLmv7DivfNa", uint64(0))
	f.Add("1A1zP1eP5QGefi2DMPTfTL5SLmv7DivfNa", uint64(1))
	f.Add("1A1zP1eP5QGefi2DMPTfTL5SLmv7DivfNa", uint64(545))
	f.Add("1A1zP1eP5QGefi2DMPTfTL5SLmv7DivfNa", uint64(546))
	f.Add("1A1zP1eP5QGefi2DMPTfTL5SLmv7DivfNa", ^uint64(0)) // Max uint64

	f.Fuzz(func(t *testing.T, address string, amount uint64) {
		builder := NewTxBuilder()

		// Should never panic
		err := builder.AddOutput(address, amount)

		// Verify invariants
		if err == nil {
			// If successful, output should be added
			if len(builder.Outputs) != 1 {
				t.Error("successful AddOutput should add exactly one output")
			}
			// Amount should be at least dust limit
			if amount < DustLimit {
				t.Errorf("AddOutput succeeded with amount %d below dust limit %d", amount, DustLimit)
			}
		}
	})
}

// FuzzTxBuilder_Validate tests that Validate never panics on any configuration.
//
//nolint:gocognit // Fuzz test setup is inherently complex
func FuzzTxBuilder_Validate(f *testing.F) {
	// Seed with various input/output counts
	f.Add(uint8(0), uint8(0), uint64(1000), uint64(500), uint64(1))
	f.Add(uint8(1), uint8(1), uint64(10000), uint64(5000), uint64(1))
	f.Add(uint8(5), uint8(2), uint64(100000), uint64(50000), uint64(10))
	f.Add(uint8(10), uint8(10), uint64(1000000), uint64(500000), uint64(50))

	f.Fuzz(func(_ *testing.T, numInputs, numOutputs uint8, inputAmount, outputAmount, feeRate uint64) {
		builder := NewTxBuilder()

		// Limit to reasonable values to avoid timeout
		if numInputs > 50 {
			numInputs = 50
		}
		if numOutputs > 50 {
			numOutputs = 50
		}

		// Add inputs
		for i := uint8(0); i < numInputs; i++ {
			_ = builder.AddInput(UTXO{
				TxID:   testTxID(int(i)),
				Vout:   0,
				Amount: inputAmount,
			})
		}

		// Add outputs (only if amount is above dust)
		if outputAmount >= DustLimit {
			for i := uint8(0); i < numOutputs; i++ {
				_ = builder.AddOutput(testAddress, outputAmount)
			}
		}

		// Set fee rate
		builder.SetFeeRate(feeRate)

		// Should never panic
		_ = builder.Validate()
	})
}

// FuzzSelectUTXOs tests that SelectUTXOs never panics on any input.
func FuzzSelectUTXOs(f *testing.F) {
	// Seed with various scenarios
	f.Add(uint8(1), uint64(10000), uint64(5000), uint64(1))
	f.Add(uint8(5), uint64(1000), uint64(3000), uint64(10))
	f.Add(uint8(10), uint64(500), uint64(2000), uint64(50))
	f.Add(uint8(0), uint64(10000), uint64(5000), uint64(1)) // Empty UTXO list

	f.Fuzz(func(t *testing.T, numUTXOs uint8, utxoAmount, targetAmount, feeRate uint64) {
		client := NewClient(nil)

		// Limit to reasonable values
		if numUTXOs > 50 {
			numUTXOs = 50
		}

		// Create UTXOs
		utxos := make([]UTXO, numUTXOs)
		for i := uint8(0); i < numUTXOs; i++ {
			utxos[i] = UTXO{
				TxID:   testTxID(int(i)),
				Vout:   0,
				Amount: utxoAmount,
			}
		}

		// Should never panic
		selected, change, err := client.SelectUTXOs(utxos, targetAmount, feeRate)

		// Verify invariants when successful
		verifySelectUTXOsInvariants(t, selected, change, err, targetAmount, feeRate)
	})
}

func verifySelectUTXOsInvariants(t *testing.T, selected []UTXO, change uint64, err error, targetAmount, feeRate uint64) {
	t.Helper()
	if err != nil || len(selected) == 0 {
		return
	}

	// Total selected should be >= target + estimated fee
	var total uint64
	for _, u := range selected {
		total += u.Amount
	}
	fee := estimatedTxSize * feeRate
	if total < targetAmount+fee {
		t.Errorf("selected total %d < target %d + fee %d", total, targetAmount, fee)
	}

	// Change should be consistent
	expectedChange := total - targetAmount - fee
	if change != expectedChange {
		t.Errorf("change %d != expected %d", change, expectedChange)
	}
}

// FuzzCalculateSweepAmount tests that CalculateSweepAmount never panics.
func FuzzCalculateSweepAmount(f *testing.F) {
	// Seed with various scenarios
	f.Add(uint64(10000), uint8(1), uint64(1))
	f.Add(uint64(100000), uint8(10), uint64(50))
	f.Add(uint64(0), uint8(0), uint64(1))
	f.Add(uint64(192), uint8(1), uint64(1))   // Exactly fee
	f.Add(uint64(738), uint8(1), uint64(1))   // Exactly dust limit after fee
	f.Add(^uint64(0), uint8(100), uint64(50)) // Max values

	f.Fuzz(func(t *testing.T, totalInputs uint64, numInputs uint8, feeRate uint64) {
		// Limit to reasonable values
		if numInputs > 100 {
			numInputs = 100
		}

		// Should never panic
		amount, err := CalculateSweepAmount(totalInputs, int(numInputs), feeRate)

		// Verify invariants
		if err == nil {
			// Amount should be above dust limit
			if amount < DustLimit {
				t.Errorf("sweep amount %d below dust limit %d", amount, DustLimit)
			}

			// Amount + fee should equal total (approximately, accounting for fee rate clamping)
			validFeeRate := ValidateFeeRate(feeRate)
			expectedFee := EstimateFeeForTx(int(numInputs), 1, validFeeRate)
			expectedAmount := totalInputs - expectedFee

			if amount != expectedAmount {
				t.Errorf("amount %d != expected %d (total %d - fee %d)", amount, expectedAmount, totalInputs, expectedFee)
			}
		}
	})
}

// FuzzEstimateTxSize tests that EstimateTxSize never panics or overflows.
func FuzzEstimateTxSize(f *testing.F) {
	f.Add(0, 0)
	f.Add(1, 1)
	f.Add(100, 100)
	f.Add(1000, 1000)

	f.Fuzz(func(t *testing.T, numInputs, numOutputs int) {
		// Limit to prevent overflow in test
		if numInputs > 10000 {
			numInputs = 10000
		}
		if numOutputs > 10000 {
			numOutputs = 10000
		}
		if numInputs < 0 {
			numInputs = 0
		}
		if numOutputs < 0 {
			numOutputs = 0
		}

		// Should never panic
		size := EstimateTxSize(numInputs, numOutputs)

		// Size should always be at least overhead
		if size < uint64(TxOverhead) {
			t.Errorf("size %d < overhead %d", size, TxOverhead)
		}
	})
}

// FuzzValidateFeeRate tests that ValidateFeeRate never panics.
func FuzzValidateFeeRate(f *testing.F) {
	f.Add(uint64(0))
	f.Add(uint64(1))
	f.Add(uint64(50))
	f.Add(uint64(51))
	f.Add(^uint64(0))

	f.Fuzz(func(t *testing.T, rate uint64) {
		// Should never panic
		result := ValidateFeeRate(rate)

		// Result should always be in valid range
		if result < MinFeeRate {
			t.Errorf("result %d < min %d", result, MinFeeRate)
		}
		if result > MaxFeeRate {
			t.Errorf("result %d > max %d", result, MaxFeeRate)
		}
	})
}
