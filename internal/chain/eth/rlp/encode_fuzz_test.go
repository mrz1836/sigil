package rlp

import (
	"bytes"
	"math/big"
	"testing"
)

// FuzzEncodeTransaction ensures the legacy-transaction RLP encoder never panics
// on arbitrary field values and always emits a well-formed, deterministic list
// (RLP lists carry a prefix >= 0xc0). This encoder produces the exact bytes that
// get signed and broadcast, so a panic or malformed prefix here is a money bug.
func FuzzEncodeTransaction(f *testing.F) {
	f.Add(uint64(0), []byte(nil), uint64(21000), []byte(nil), []byte(nil), []byte(nil))
	f.Add(uint64(42), []byte{0x3b, 0x9a, 0xca, 0x00}, uint64(21000), make([]byte, 20), []byte{0x01}, []byte("data"))

	f.Fuzz(func(t *testing.T, nonce uint64, gasPriceB []byte, gasLimit uint64, to, valueB, data []byte) {
		gasPrice := new(big.Int).SetBytes(gasPriceB)
		value := new(big.Int).SetBytes(valueB)

		out := EncodeTransaction(nonce, gasPrice, gasLimit, to, value, data, nil, nil, nil)
		if len(out) == 0 {
			t.Fatal("EncodeTransaction returned empty output")
		}
		if out[0] < 0xc0 {
			t.Errorf("transaction RLP must start with a list prefix (>=0xc0), got 0x%02x", out[0])
		}

		// Encoding must be deterministic for identical inputs.
		if out2 := EncodeTransaction(nonce, gasPrice, gasLimit, to, value, data, nil, nil, nil); !bytes.Equal(out, out2) {
			t.Error("EncodeTransaction is not deterministic")
		}

		// The EIP-155 signing variant must also be a well-formed list.
		signOut := EncodeTransactionForSigning(nonce, gasPrice, gasLimit, to, value, data, big.NewInt(1))
		if len(signOut) == 0 || signOut[0] < 0xc0 {
			t.Errorf("EncodeTransactionForSigning produced an invalid list encoding: %x", signOut)
		}
	})
}

// FuzzEncodeBytes ensures the byte-string encoder preserves its structural
// invariants and never panics: a single byte < 0x80 is its own encoding;
// everything else carries a string prefix in [0x80, 0xc0).
func FuzzEncodeBytes(f *testing.F) {
	f.Add([]byte(nil))
	f.Add([]byte{0x00})
	f.Add([]byte{0x7f})
	f.Add([]byte{0x80})
	f.Add(bytes.Repeat([]byte{0xab}, 100))

	f.Fuzz(func(t *testing.T, b []byte) {
		out := Encode(b)
		if len(out) == 0 {
			t.Fatal("Encode([]byte) returned empty output")
		}

		if isSingleLowByte(b) {
			if len(out) != 1 || out[0] != b[0] {
				t.Errorf("single byte < 0x80 must be its own encoding; got %x for %x", out, b)
			}
			return
		}

		// All other byte strings carry a string prefix, never a list prefix.
		if !isRLPStringPrefix(out[0]) {
			t.Errorf("byte-string encoding must carry a prefix in [0x80, 0xc0), got 0x%02x", out[0])
		}
	})
}

// isSingleLowByte reports whether b is a single byte < 0x80 (its own RLP encoding).
func isSingleLowByte(b []byte) bool {
	return len(b) == 1 && b[0] < 0x80
}

// isRLPStringPrefix reports whether p is a valid RLP string prefix (0x80..0xbf).
func isRLPStringPrefix(p byte) bool {
	return p >= 0x80 && p < 0xc0
}
