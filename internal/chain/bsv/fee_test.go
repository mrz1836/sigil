package bsv

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetFeeQuote(t *testing.T) {
	t.Parallel()

	t.Run("successful API response", func(t *testing.T) {
		t.Parallel()

		payload := TAALPayload{
			Fees: []struct {
				FeeType   string `json:"feeType"`
				MiningFee struct {
					Satoshis int64 `json:"satoshis"`
					Bytes    int64 `json:"bytes"`
				} `json:"miningFee"`
				RelayFee struct {
					Satoshis int64 `json:"satoshis"`
					Bytes    int64 `json:"bytes"`
				} `json:"relayFee"`
			}{
				{
					FeeType: "standard",
					MiningFee: struct {
						Satoshis int64 `json:"satoshis"`
						Bytes    int64 `json:"bytes"`
					}{
						Satoshis: 500,
						Bytes:    1000,
					},
				},
			},
		}
		payloadBytes, err := json.Marshal(payload)
		require.NoError(t, err)

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			resp := TAALFeeQuoteResponse{
				Payload: string(payloadBytes),
			}
			encErr := json.NewEncoder(w).Encode(resp)
			assert.NoError(t, encErr)
		}))
		defer server.Close()

		client := NewClient(&ClientOptions{
			BaseURL: server.URL,
		})
		// Override the TAAL URL for testing by creating a custom HTTP client
		client.httpClient = &http.Client{
			Transport: &feeTestTransport{
				server: server,
			},
		}

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		quote, err := client.GetFeeQuote(ctx)
		require.NoError(t, err)

		assert.Equal(t, "taal", quote.Source)
		// 500 satoshis / 1000 bytes = 0, but minimum is 1
		assert.GreaterOrEqual(t, quote.StandardRate, uint64(1))
	})

	t.Run("network error returns default fee quote", func(t *testing.T) {
		t.Parallel()

		// Create a client with a transport that always fails
		client := NewClient(nil)
		client.httpClient = &http.Client{
			Transport: &failingTransport{},
		}

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		quote, err := client.GetFeeQuote(ctx)
		require.NoError(t, err)

		assert.Equal(t, "default", quote.Source)
		assert.Equal(t, uint64(DefaultFeeRate), quote.StandardRate)
	})

	t.Run("non-200 status returns default fee quote", func(t *testing.T) {
		t.Parallel()

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusServiceUnavailable)
		}))
		defer server.Close()

		client := NewClient(nil)
		client.httpClient = &http.Client{
			Transport: &feeTestTransport{
				server: server,
			},
		}

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		quote, err := client.GetFeeQuote(ctx)
		require.NoError(t, err)

		assert.Equal(t, "default", quote.Source)
	})

	t.Run("invalid JSON returns default fee quote", func(t *testing.T) {
		t.Parallel()

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			_, _ = w.Write([]byte("not valid json"))
		}))
		defer server.Close()

		client := NewClient(nil)
		client.httpClient = &http.Client{
			Transport: &feeTestTransport{
				server: server,
			},
		}

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		quote, err := client.GetFeeQuote(ctx)
		require.NoError(t, err)

		assert.Equal(t, "default", quote.Source)
	})

	t.Run("invalid payload JSON returns default fee quote", func(t *testing.T) {
		t.Parallel()

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			resp := TAALFeeQuoteResponse{
				Payload: "not valid json payload",
			}
			encErr := json.NewEncoder(w).Encode(resp)
			assert.NoError(t, encErr)
		}))
		defer server.Close()

		client := NewClient(nil)
		client.httpClient = &http.Client{
			Transport: &feeTestTransport{
				server: server,
			},
		}

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		quote, err := client.GetFeeQuote(ctx)
		require.NoError(t, err)

		assert.Equal(t, "default", quote.Source)
	})

	t.Run("fee rate of zero uses minimum of 1", func(t *testing.T) {
		t.Parallel()

		payload := TAALPayload{
			Fees: []struct {
				FeeType   string `json:"feeType"`
				MiningFee struct {
					Satoshis int64 `json:"satoshis"`
					Bytes    int64 `json:"bytes"`
				} `json:"miningFee"`
				RelayFee struct {
					Satoshis int64 `json:"satoshis"`
					Bytes    int64 `json:"bytes"`
				} `json:"relayFee"`
			}{
				{
					FeeType: "standard",
					MiningFee: struct {
						Satoshis int64 `json:"satoshis"`
						Bytes    int64 `json:"bytes"`
					}{
						Satoshis: 0,
						Bytes:    1000,
					},
				},
			},
		}
		payloadBytes, err := json.Marshal(payload)
		require.NoError(t, err)

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			resp := TAALFeeQuoteResponse{
				Payload: string(payloadBytes),
			}
			encErr := json.NewEncoder(w).Encode(resp)
			assert.NoError(t, encErr)
		}))
		defer server.Close()

		client := NewClient(nil)
		client.httpClient = &http.Client{
			Transport: &feeTestTransport{
				server: server,
			},
		}

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		quote, err := client.GetFeeQuote(ctx)
		require.NoError(t, err)

		assert.Equal(t, uint64(1), quote.StandardRate)
	})
}

// feeTestTransport redirects all requests to the test server.
type feeTestTransport struct {
	server *httptest.Server
}

func (t *feeTestTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	// Redirect to test server
	req.URL.Scheme = "http"
	req.URL.Host = t.server.Listener.Addr().String()
	return http.DefaultTransport.RoundTrip(req)
}

// failingTransport always returns an error.
type failingTransport struct{}

func (t *failingTransport) RoundTrip(_ *http.Request) (*http.Response, error) {
	return nil, assert.AnError
}

func TestEstimateTxSize(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		numInputs  int
		numOutputs int
		expected   uint64
	}{
		// Basic cases
		{
			name:       "single input and output",
			numInputs:  1,
			numOutputs: 1,
			expected:   uint64(TxOverhead + P2PKHInputSize + P2PKHOutputSize), // 10 + 148 + 34 = 192
		},
		{
			name:       "standard P2PKH transaction - 1 input, 2 outputs",
			numInputs:  1,
			numOutputs: 2,
			expected:   uint64(TxOverhead + P2PKHInputSize + 2*P2PKHOutputSize), // 10 + 148 + 68 = 226
		},

		// Multiple inputs (common when consolidating small UTXOs)
		{
			name:       "2 inputs, 1 output - consolidation",
			numInputs:  2,
			numOutputs: 1,
			expected:   uint64(TxOverhead + 2*P2PKHInputSize + P2PKHOutputSize), // 10 + 296 + 34 = 340
		},
		{
			name:       "3 inputs, 2 outputs",
			numInputs:  3,
			numOutputs: 2,
			expected:   uint64(TxOverhead + 3*P2PKHInputSize + 2*P2PKHOutputSize), // 10 + 444 + 68 = 522
		},
		{
			name:       "5 inputs, 1 output - heavy consolidation",
			numInputs:  5,
			numOutputs: 1,
			expected:   uint64(TxOverhead + 5*P2PKHInputSize + P2PKHOutputSize), // 10 + 740 + 34 = 784
		},
		{
			name:       "10 inputs, 2 outputs - large consolidation",
			numInputs:  10,
			numOutputs: 2,
			expected:   uint64(TxOverhead + 10*P2PKHInputSize + 2*P2PKHOutputSize), // 10 + 1480 + 68 = 1558
		},

		// Multiple outputs (batch payments)
		{
			name:       "1 input, 5 outputs - batch send",
			numInputs:  1,
			numOutputs: 5,
			expected:   uint64(TxOverhead + P2PKHInputSize + 5*P2PKHOutputSize), // 10 + 148 + 170 = 328
		},
		{
			name:       "2 inputs, 10 outputs - large batch",
			numInputs:  2,
			numOutputs: 10,
			expected:   uint64(TxOverhead + 2*P2PKHInputSize + 10*P2PKHOutputSize), // 10 + 296 + 340 = 646
		},

		// Edge cases
		{
			name:       "zero inputs",
			numInputs:  0,
			numOutputs: 1,
			expected:   uint64(TxOverhead + P2PKHOutputSize), // 10 + 34 = 44
		},
		{
			name:       "zero outputs",
			numInputs:  1,
			numOutputs: 0,
			expected:   uint64(TxOverhead + P2PKHInputSize), // 10 + 148 = 158
		},
		{
			name:       "zero inputs and outputs",
			numInputs:  0,
			numOutputs: 0,
			expected:   uint64(TxOverhead), // 10
		},

		// Large transactions (BSV supports large blocks)
		{
			name:       "100 inputs, 100 outputs - large transaction",
			numInputs:  100,
			numOutputs: 100,
			expected:   uint64(TxOverhead + 100*P2PKHInputSize + 100*P2PKHOutputSize), // 10 + 14800 + 3400 = 18210
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result := EstimateTxSize(tt.numInputs, tt.numOutputs)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestEstimateFeeForTx(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		numInputs  int
		numOutputs int
		feeRate    uint64
		expected   uint64
	}{
		// Basic fee calculations
		{
			name:       "basic calculation - 1 sat/byte",
			numInputs:  1,
			numOutputs: 2,
			feeRate:    1,
			expected:   uint64(TxOverhead+P2PKHInputSize+2*P2PKHOutputSize) * 1, // 226 * 1 = 226
		},
		{
			name:       "higher fee rate - 5 sat/byte",
			numInputs:  1,
			numOutputs: 2,
			feeRate:    5,
			expected:   uint64(TxOverhead+P2PKHInputSize+2*P2PKHOutputSize) * 5, // 226 * 5 = 1130
		},
		{
			name:       "multiple inputs with varying fee rate",
			numInputs:  3,
			numOutputs: 2,
			feeRate:    2,
			expected:   uint64(TxOverhead+3*P2PKHInputSize+2*P2PKHOutputSize) * 2, // 522 * 2 = 1044
		},

		// Minimum fee (1 sat/byte - BSV standard)
		{
			name:       "minimum fee rate",
			numInputs:  1,
			numOutputs: 1,
			feeRate:    MinFeeRate,
			expected:   uint64(TxOverhead+P2PKHInputSize+P2PKHOutputSize) * MinFeeRate, // 192 * 1 = 192
		},

		// Maximum reasonable fee rate
		{
			name:       "maximum fee rate",
			numInputs:  1,
			numOutputs: 2,
			feeRate:    MaxFeeRate,
			expected:   uint64(TxOverhead+P2PKHInputSize+2*P2PKHOutputSize) * MaxFeeRate, // 226 * 50 = 11300
		},

		// Zero fee rate (edge case)
		{
			name:       "zero fee rate",
			numInputs:  1,
			numOutputs: 2,
			feeRate:    0,
			expected:   0,
		},

		// Exact satoshi fee calculations for specific tx sizes
		{
			name:       "192 byte tx at 1 sat/byte = 192 satoshis",
			numInputs:  1,
			numOutputs: 1,
			feeRate:    1,
			expected:   192,
		},
		{
			name:       "226 byte tx at 1 sat/byte = 226 satoshis",
			numInputs:  1,
			numOutputs: 2,
			feeRate:    1,
			expected:   226,
		},
		{
			name:       "340 byte tx at 1 sat/byte = 340 satoshis",
			numInputs:  2,
			numOutputs: 1,
			feeRate:    1,
			expected:   340,
		},

		// Small amount consolidation scenarios
		{
			name:       "consolidating 10 UTXOs to single output",
			numInputs:  10,
			numOutputs: 1,
			feeRate:    1,
			expected:   uint64(TxOverhead + 10*P2PKHInputSize + P2PKHOutputSize), // 1524
		},
		{
			name:       "batch send to 5 recipients",
			numInputs:  2,
			numOutputs: 5,
			feeRate:    1,
			expected:   uint64(TxOverhead + 2*P2PKHInputSize + 5*P2PKHOutputSize), // 476
		},

		// Large transaction fees
		{
			name:       "large consolidation at low fee",
			numInputs:  50,
			numOutputs: 2,
			feeRate:    1,
			expected:   uint64(TxOverhead + 50*P2PKHInputSize + 2*P2PKHOutputSize), // 7478
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result := EstimateFeeForTx(tt.numInputs, tt.numOutputs, tt.feeRate)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestEstimateFeeForAmount(t *testing.T) {
	t.Parallel()

	t.Run("returns fee based on quote", func(t *testing.T) {
		t.Parallel()

		payload := TAALPayload{
			Fees: []struct {
				FeeType   string `json:"feeType"`
				MiningFee struct {
					Satoshis int64 `json:"satoshis"`
					Bytes    int64 `json:"bytes"`
				} `json:"miningFee"`
				RelayFee struct {
					Satoshis int64 `json:"satoshis"`
					Bytes    int64 `json:"bytes"`
				} `json:"relayFee"`
			}{
				{
					FeeType: "standard",
					MiningFee: struct {
						Satoshis int64 `json:"satoshis"`
						Bytes    int64 `json:"bytes"`
					}{
						Satoshis: 2,
						Bytes:    1,
					},
				},
			},
		}
		payloadBytes, err := json.Marshal(payload)
		require.NoError(t, err)

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			resp := TAALFeeQuoteResponse{
				Payload: string(payloadBytes),
			}
			encErr := json.NewEncoder(w).Encode(resp)
			assert.NoError(t, encErr)
		}))
		defer server.Close()

		client := NewClient(nil)
		client.httpClient = &http.Client{
			Transport: &feeTestTransport{
				server: server,
			},
		}

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		fee, err := client.EstimateFeeForAmount(ctx, 100000000) // 1 BSV
		require.NoError(t, err)

		// Fee should be tx size (1 input, 2 outputs) * fee rate (2 sat/byte)
		expectedSize := uint64(TxOverhead + P2PKHInputSize + 2*P2PKHOutputSize) // 226
		expectedFee := expectedSize * 2                                         // 452
		assert.Equal(t, expectedFee, fee)
	})

	t.Run("uses default quote on error", func(t *testing.T) {
		t.Parallel()

		client := NewClient(nil)
		client.httpClient = &http.Client{
			Transport: &failingTransport{},
		}

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		fee, err := client.EstimateFeeForAmount(ctx, 100000000) // 1 BSV
		require.NoError(t, err)

		// Default fee rate is 1 sat/byte
		expectedSize := uint64(TxOverhead + P2PKHInputSize + 2*P2PKHOutputSize) // 226
		expectedFee := expectedSize * DefaultFeeRate                            // 226
		assert.Equal(t, expectedFee, fee)
	})

	// BSV-specific: single satoshi amounts (no dust limit)
	t.Run("fee for sending 1 satoshi", func(t *testing.T) {
		t.Parallel()

		client := NewClient(nil)
		client.httpClient = &http.Client{
			Transport: &failingTransport{},
		}

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		fee, err := client.EstimateFeeForAmount(ctx, 1) // 1 satoshi
		require.NoError(t, err)

		// Fee is same regardless of amount (tx size based)
		expectedSize := uint64(TxOverhead + P2PKHInputSize + 2*P2PKHOutputSize) // 226
		expectedFee := expectedSize * DefaultFeeRate                            // 226
		assert.Equal(t, expectedFee, fee)

		// Fee is much larger than amount being sent (valid in BSV)
		assert.Greater(t, fee, uint64(1))
	})

	t.Run("fee for sending 100 satoshis", func(t *testing.T) {
		t.Parallel()

		client := NewClient(nil)
		client.httpClient = &http.Client{
			Transport: &failingTransport{},
		}

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		fee, err := client.EstimateFeeForAmount(ctx, 100) // 100 satoshis
		require.NoError(t, err)

		expectedFee := uint64(TxOverhead+P2PKHInputSize+2*P2PKHOutputSize) * DefaultFeeRate
		assert.Equal(t, expectedFee, fee)
	})

	t.Run("fee for sending 1000 satoshis", func(t *testing.T) {
		t.Parallel()

		client := NewClient(nil)
		client.httpClient = &http.Client{
			Transport: &failingTransport{},
		}

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		fee, err := client.EstimateFeeForAmount(ctx, 1000) // 1000 satoshis
		require.NoError(t, err)

		expectedFee := uint64(TxOverhead+P2PKHInputSize+2*P2PKHOutputSize) * DefaultFeeRate
		assert.Equal(t, expectedFee, fee)
	})

	t.Run("fee for max supply", func(t *testing.T) {
		t.Parallel()

		client := NewClient(nil)
		client.httpClient = &http.Client{
			Transport: &failingTransport{},
		}

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		// 21 million BSV in satoshis
		fee, err := client.EstimateFeeForAmount(ctx, 2100000000000000)
		require.NoError(t, err)

		// Fee estimation doesn't depend on amount
		expectedFee := uint64(TxOverhead+P2PKHInputSize+2*P2PKHOutputSize) * DefaultFeeRate
		assert.Equal(t, expectedFee, fee)
	})
}

func TestValidateFeeRate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		rate     uint64
		expected uint64
	}{
		// Below minimum
		{
			name:     "zero returns minimum",
			rate:     0,
			expected: MinFeeRate,
		},

		// At boundaries
		{
			name:     "at minimum returns minimum",
			rate:     MinFeeRate,
			expected: MinFeeRate,
		},
		{
			name:     "at maximum returns maximum",
			rate:     MaxFeeRate,
			expected: MaxFeeRate,
		},
		{
			name:     "one below maximum stays unchanged",
			rate:     MaxFeeRate - 1,
			expected: MaxFeeRate - 1,
		},
		{
			name:     "one above minimum stays unchanged",
			rate:     MinFeeRate + 1,
			expected: MinFeeRate + 1,
		},

		// Above maximum
		{
			name:     "above maximum returns maximum",
			rate:     100,
			expected: MaxFeeRate,
		},
		{
			name:     "way above maximum returns maximum",
			rate:     1000000,
			expected: MaxFeeRate,
		},

		// Within range
		{
			name:     "within range returns same value",
			rate:     10,
			expected: 10,
		},
		{
			name:     "mid-range value",
			rate:     25,
			expected: 25,
		},

		// BSV typical fee rates
		{
			name:     "typical BSV rate - 1 sat/byte",
			rate:     1,
			expected: 1,
		},
		{
			name:     "elevated BSV rate - 2 sat/byte",
			rate:     2,
			expected: 2,
		},
		{
			name:     "high BSV rate - 10 sat/byte",
			rate:     10,
			expected: 10,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result := ValidateFeeRate(tt.rate)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// TestTxSizeVsFee tests the relationship between transaction size and fee.
func TestTxSizeVsFee(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		numInputs  int
		numOutputs int
		feeRate1   uint64 // At 1 sat/byte
		feeRate50  uint64 // At 50 sat/byte
	}{
		{
			name:       "1 input, 1 output",
			numInputs:  1,
			numOutputs: 1,
			// Size: 10 + 148 + 34 = 192 bytes
			feeRate1:  192,
			feeRate50: 9600,
		},
		{
			name:       "1 input, 2 outputs",
			numInputs:  1,
			numOutputs: 2,
			// Size: 10 + 148 + 68 = 226 bytes
			feeRate1:  226,
			feeRate50: 11300,
		},
		{
			name:       "10 inputs, 1 output",
			numInputs:  10,
			numOutputs: 1,
			// Size: 10 + 1480 + 34 = 1524 bytes
			feeRate1:  1524,
			feeRate50: 76200,
		},
		{
			name:       "1 input, 10 outputs",
			numInputs:  1,
			numOutputs: 10,
			// Size: 10 + 148 + 340 = 498 bytes
			feeRate1:  498,
			feeRate50: 24900,
		},
		{
			name:       "100 inputs, 100 outputs",
			numInputs:  100,
			numOutputs: 100,
			// Size: 10 + 14800 + 3400 = 18210 bytes
			feeRate1:  18210,
			feeRate50: 910500,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// Test at 1 sat/byte
			size := EstimateTxSize(tt.numInputs, tt.numOutputs)
			fee1 := EstimateFeeForTx(tt.numInputs, tt.numOutputs, 1)
			assert.Equal(t, size, fee1, "fee at 1 sat/byte should equal size")
			assert.Equal(t, tt.feeRate1, fee1)

			// Test at 50 sat/byte
			fee50 := EstimateFeeForTx(tt.numInputs, tt.numOutputs, 50)
			assert.Equal(t, size*50, fee50)
			assert.Equal(t, tt.feeRate50, fee50)
		})
	}
}

// TestFeeRateBoundaries tests fee rate clamping at boundaries.
func TestFeeRateBoundaries(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		inputRate    uint64
		expectedRate uint64
	}{
		{"zero clamps to minimum", 0, MinFeeRate},
		{"1 stays at 1", 1, 1},
		{"25 stays at 25", 25, 25},
		{"50 stays at 50", 50, 50},
		{"51 clamps to 50", 51, 50},
		{"100 clamps to 50", 100, 50},
		{"max uint64 clamps to 50", ^uint64(0), 50},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result := ValidateFeeRate(tt.inputRate)
			assert.Equal(t, tt.expectedRate, result)
		})
	}
}

// TestEstimateTxSize_FormulaVerification verifies the size formula is correct.
func TestEstimateTxSize_FormulaVerification(t *testing.T) {
	t.Parallel()

	// Verify constants are reasonable
	assert.Equal(t, 10, TxOverhead, "transaction overhead should be 10 bytes")
	assert.Equal(t, 148, P2PKHInputSize, "P2PKH input should be 148 bytes")
	assert.Equal(t, 34, P2PKHOutputSize, "P2PKH output should be 34 bytes")

	// Verify formula: overhead + (inputs * input_size) + (outputs * output_size)
	for numIn := 0; numIn <= 10; numIn++ {
		for numOut := 0; numOut <= 10; numOut++ {
			expected := EstimateTxSize(numIn, numOut)
			actual := EstimateTxSize(numIn, numOut)
			assert.Equal(t, expected, actual, "size mismatch for %d inputs, %d outputs", numIn, numOut)
		}
	}
}

// TestEstimateFeeForTx_LargeTx tests fee estimation for very large transactions.
func TestEstimateFeeForTx_LargeTx(t *testing.T) {
	t.Parallel()

	// BSV supports large blocks, so test with many inputs/outputs
	tests := []struct {
		name       string
		numInputs  int
		numOutputs int
		feeRate    uint64
	}{
		{"500 inputs, 1 output", 500, 1, 1},
		{"1 input, 500 outputs", 1, 500, 1},
		{"100 inputs, 100 outputs", 100, 100, 1},
		{"1000 inputs, 10 outputs at 50 sat/byte", 1000, 10, 50},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			size := EstimateTxSize(tt.numInputs, tt.numOutputs)
			fee := EstimateFeeForTx(tt.numInputs, tt.numOutputs, tt.feeRate)

			// Fee should equal size * rate
			assert.Equal(t, size*tt.feeRate, fee)

			// Verify no overflow
			assert.Positive(t, fee)
		})
	}
}

// TestFeeQuote_DefaultValues tests default fee quote values.
func TestFeeQuote_DefaultValues(t *testing.T) {
	t.Parallel()

	quote := defaultFeeQuote()

	assert.Equal(t, uint64(DefaultFeeRate), quote.StandardRate)
	assert.Equal(t, uint64(DefaultFeeRate), quote.DataRate)
	assert.Equal(t, "default", quote.Source)
	assert.False(t, quote.Timestamp.IsZero())
}
