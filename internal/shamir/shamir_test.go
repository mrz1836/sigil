package shamir

import (
	"bytes"
	"crypto/rand"
	"testing"
)

//nolint:gocognit,gocyclo // Test function with many sub-cases
func TestSplitCombine(t *testing.T) {
	tests := []struct {
		name      string
		secretLen int
		n, k      int
	}{
		{"ShortSecret", 16, 5, 3},
		{"LongSecret", 64, 5, 3},
		{"Threshold2", 32, 5, 2},
		{"ThresholdSameAsN", 32, 5, 5},
		{"MaxShares", 32, 255, 3},
		{"MinShares", 32, 2, 2},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			secret := make([]byte, tt.secretLen)
			if _, err := rand.Read(secret); err != nil {
				t.Fatalf("Failed to generate secret: %v", err)
			}

			shares, err := Split(secret, tt.n, tt.k)
			if err != nil {
				t.Fatalf("Split failed: %v", err)
			}

			if len(shares) != tt.n {
				t.Errorf("Expected %d shares, got %d", tt.n, len(shares))
			}

			// Validate share format
			for _, s := range shares {
				if len(s) == 0 {
					t.Error("Empty share string")
				}
				// Verify prefix
				if len(s) < 6 || s[:6] != "sigil-" {
					t.Error("Invalid share prefix")
				}
			}

			// Recombine with all shares
			recovered, err := Combine(shares)
			if err != nil {
				t.Fatalf("Combine failed with all shares: %v", err)
			}
			if !bytes.Equal(secret, recovered) {
				t.Errorf("Recovered secret mismatch. Got %x, want %x", recovered, secret)
			}

			// Recombine with exactly k shares
			subset := shares[:tt.k]
			recoveredSub, err := Combine(subset)
			if err != nil {
				t.Fatalf("Combine failed with k shares: %v", err)
			}
			if !bytes.Equal(secret, recoveredSub) {
				t.Errorf("Recovered (subset) secret mismatch. Got %x, want %x", recoveredSub, secret)
			}

			// Test with multiple random subsets of size k
			// just try one other subset (last k)
			subset2 := shares[len(shares)-tt.k:]
			recoveredSub2, err := Combine(subset2)
			if err != nil {
				t.Fatalf("Combine failed with last k shares: %v", err)
			}
			if !bytes.Equal(secret, recoveredSub2) {
				t.Errorf("Recovered (subset2) secret mismatch")
			}
		})
	}
}

func TestCombineInsufficientShares(t *testing.T) {
	secret := []byte("test secret")
	n, k := 5, 3
	shares, _ := Split(secret, n, k)

	// Try with k-1 shares
	subset := shares[:k-1]
	_, err := Combine(subset)
	if err == nil {
		t.Error("Combine should verify k threshold, but it succeeded with k-1 shares")
	}
}

func TestCombineDuplicateShares(t *testing.T) {
	secret := []byte("test secret")
	n, k := 5, 3
	shares, _ := Split(secret, n, k)

	// Use k shares but duplicate one to verify we handle it (by failing or ignoring)
	// Our implementation ignores duplicates but checks if unique count >= k.
	// So [s1, s1, s2] is count 2 < k=3 => fail.
	subset := []string{shares[0], shares[0], shares[1]}
	_, err := Combine(subset)
	if err == nil {
		t.Error("Combine should fail if unique shares < k")
	}
}

func TestCombineInvalidFormat(t *testing.T) {
	invalidShares := []string{
		"invalid-prefix-v1-3-1-abcdef",
		"sigil-v2-3-1-abcdef",     // wrong version
		"sigil-v1-x-1-abcdef",     // bad threshold
		"sigil-v1-3-x-abcdef",     // bad index
		"sigil-v1-3-0-abcdef",     // zero index
		"sigil-v1-3-1-invalidhex", // bad hex
	}

	for _, s := range invalidShares {
		_, err := Combine([]string{s, s, s}) // provide k=3 dummy count
		if err == nil {
			t.Errorf("Combine should fail for invalid share: %s", s)
		}
	}
}

func TestDeteministicReconstruction(t *testing.T) {
	// These shares were generated from secret "secret" with n=3, k=2
	shares := []string{
		"sigil-v1-2-1-449abc1b970d",
		"sigil-v1-2-2-1d80c6a09a86",
		"sigil-v1-2-3-2a7f19c968ff",
	}

	expectedSecret := []byte("secret")

	// Test all combinations of 2 shares
	combos := [][]string{
		{shares[0], shares[1]},
		{shares[0], shares[2]},
		{shares[1], shares[2]},
	}

	for i, c := range combos {
		rec, err := Combine(c)
		if err != nil {
			t.Errorf("Combine failed for combo %d: %v", i, err)
			continue
		}
		if !bytes.Equal(rec, expectedSecret) {
			t.Errorf("Combo %d mismatch: got %s, want %s", i, rec, expectedSecret)
		}
	}
}

func TestTamperedShares(t *testing.T) {
	secret := []byte("test secret")
	n, k := 5, 3
	shares, _ := Split(secret, n, k)

	// Mutate one char in the hex part of the last share
	// Original share format: sigil-v1-<k>-<id>-<hex>
	lastShare := shares[k-1]

	// Just change the last character
	runes := []rune(lastShare)
	if runes[len(runes)-1] == 'a' {
		runes[len(runes)-1] = 'b'
	} else {
		runes[len(runes)-1] = 'a'
	}
	badShare := string(runes)

	// Combine using the tampered share
	subset := append(shares[:k-1], badShare)
	rec, err := Combine(subset)

	// It should technically succeed in combining (math works), but the result will be wrong.
	// We want to ensure it DOES NOT panic and produces WRONG output.
	// If err != nil (e.g. invalid hex), that's also acceptable.
	if err == nil {
		if bytes.Equal(rec, secret) {
			t.Error("Reconstructed correct secret despite tampered share!")
		}
	}
}

func TestValidation(t *testing.T) {
	secret := []byte("secret")

	// k < 2
	if _, err := Split(secret, 5, 1); err == nil {
		t.Error("Split should fail for k < 2")
	}

	// n < k
	if _, err := Split(secret, 2, 3); err == nil {
		t.Error("Split should fail for n < k")
	}

	// n > 255
	if _, err := Split(secret, 300, 3); err == nil {
		t.Error("Split should fail for n > 255")
	}

	// empty secret
	if _, err := Split(nil, 5, 3); err == nil {
		t.Error("Split should fail for empty secret")
	}
}

func TestVectors(t *testing.T) {
	// Standard validation of GF arithmetic basic properties

	// g = 3 (0x03) logic check
	// x + 1.
	// In GF(2^8) with 0x11b:
	// 1 + 2 = 3
	if gfAdd(1, 2) != 3 {
		t.Error("gfAdd(1, 2) != 3")
	}

	// Associativity
	if gfAdd(gfAdd(10, 20), 30) != gfAdd(10, gfAdd(20, 30)) {
		t.Error("Add associativity fail")
	}

	// Distributivity
	// a * (b + c) = a*b + a*c
	a, b, c := byte(3), byte(4), byte(5)
	lhs := gfMul(a, gfAdd(b, c))
	rhs := gfAdd(gfMul(a, b), gfMul(a, c))
	if lhs != rhs {
		t.Errorf("Distributivity fail: %d != %d", lhs, rhs)
	}

	// Inverse property
	// a * (1/a) = 1 for a != 0
	for i := 1; i < 256; i++ {
		x := byte(i)
		inv := gfDiv(1, x)
		prod := gfMul(x, inv)
		if prod != 1 {
			t.Errorf("Inverse fail for %d: got %d", x, prod)
		}
	}
}

//nolint:gocognit // Fuzzing loop needs to be self-contained
func TestFuzzSplitCombine(t *testing.T) {
	// Simple fuzz-like test with random parameters
	for i := 0; i < 1000; i++ { // Increased iterations
		secret := make([]byte, 32)
		if _, err := rand.Read(secret); err != nil {
			t.Fatalf("Failed to generate random secret iter %d: %v", i, err)
		}

		// random n between 2 and 50
		// Use crypto/rand for test params too, why not
		b := make([]byte, 2)
		if _, err := rand.Read(b); err != nil {
			t.Fatalf("Failed to generate random params iter %d: %v", i, err)
		}

		n := (int(b[0]) % 49) + 2
		// b[1] is definitely accessed safely now because len(b) is 2
		k := (int(b[1]) % (n - 1)) + 2
		if k > n {
			k = n
		}

		shares, err := Split(secret, n, k)
		if err != nil {
			t.Fatalf("Split failed iter %d: %v", i, err)
		}

		// Combine k
		rec, err := Combine(shares[:k])
		if err != nil {
			t.Fatalf("Combine failed iter %d: %v", i, err)
		}

		if !bytes.Equal(secret, rec) {
			t.Fatalf("Mismatch iter %d", i)
		}
	}
}
