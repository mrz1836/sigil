package shamir

import (
	"fmt"
	"testing"
)

// benchSecret is a 32-byte (seed-sized) secret for the Shamir benchmarks.
func benchSecret() []byte {
	secret := make([]byte, 32)
	for i := range secret {
		secret[i] = byte(i * 7)
	}
	return secret
}

// shamirConfigs are representative (n, k) splits for the benchmarks.
var shamirConfigs = []struct{ n, k int }{ //nolint:gochecknoglobals // benchmark fixture table
	{3, 2},
	{5, 3},
	{10, 6},
}

// BenchmarkSplit measures secret splitting across representative (n, k) configs.
func BenchmarkSplit(b *testing.B) {
	secret := benchSecret()
	for _, c := range shamirConfigs {
		b.Run(fmt.Sprintf("n=%d_k=%d", c.n, c.k), func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				if _, err := Split(secret, c.n, c.k); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

// BenchmarkCombine measures secret reconstruction from exactly k shares.
func BenchmarkCombine(b *testing.B) {
	secret := benchSecret()
	for _, c := range shamirConfigs {
		shares, err := Split(secret, c.n, c.k)
		if err != nil {
			b.Fatalf("split for benchmark setup: %v", err)
		}
		threshold := shares[:c.k] // exactly k shares reconstructs the secret

		b.Run(fmt.Sprintf("k=%d", c.k), func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				if _, err := Combine(threshold); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}
