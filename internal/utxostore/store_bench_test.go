package utxostore

import (
	"testing"

	"github.com/mrz1836/sigil/internal/chain"
)

// BenchmarkGetBalance measures the full-scan balance aggregation on a large
// store (it walks every tracked UTXO for the chain).
func BenchmarkGetBalance(b *testing.B) {
	store, _ := createLargeScaleStore(b, chain.BSV, 200, 10, 50_000)

	b.ReportAllocs()
	for b.Loop() {
		_ = store.GetBalance(chain.BSV)
	}
}

// BenchmarkGetUTXOs measures the per-address UTXO lookup on a large store.
func BenchmarkGetUTXOs(b *testing.B) {
	store, _ := createLargeScaleStore(b, chain.BSV, 200, 10, 50_000)
	addr := testAddressN(0)

	b.ReportAllocs()
	for b.Loop() {
		_ = store.GetUTXOs(chain.BSV, addr)
	}
}

// BenchmarkSaveLoad measures a persistence round-trip (JSON serialize + reload)
// on a large store — the store scales with address and UTXO count.
func BenchmarkSaveLoad(b *testing.B) {
	store, _ := createLargeScaleStore(b, chain.BSV, 200, 10, 50_000)

	b.ReportAllocs()
	for b.Loop() {
		if err := store.Save(); err != nil {
			b.Fatalf("save: %v", err)
		}
		if err := store.Load(); err != nil {
			b.Fatalf("load: %v", err)
		}
	}
}
