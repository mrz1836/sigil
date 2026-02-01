package bitcoin

import (
	"encoding/hex"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestHash160(t *testing.T) {
	tests := []struct {
		name     string
		input    string // hex encoded input
		expected string // hex encoded output
	}{
		{
			name:     "empty input",
			input:    "",
			expected: "b472a266d0bd89c13706a4132ccfb16f7c3b9fcb",
		},
		{
			name:     "Bitcoin public key example",
			input:    "0279be667ef9dcbbac55a06295ce870b07029bfcdb2dce28d959f2815b16f81798",
			expected: "751e76e8199196d454941c45d1b3a323f1433bd6",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			input, err := hex.DecodeString(tc.input)
			if err != nil {
				t.Fatal(err)
			}

			result := Hash160(input)
			resultHex := hex.EncodeToString(result)

			assert.Equal(t, tc.expected, resultHex)
			assert.Len(t, result, 20) // RIPEMD160 produces 20 bytes
		})
	}
}

func TestHash160_Consistency(t *testing.T) {
	// Verify that Hash160 always produces the same output for the same input
	input := []byte("test data")

	result1 := Hash160(input)
	result2 := Hash160(input)

	assert.Equal(t, result1, result2)
}
