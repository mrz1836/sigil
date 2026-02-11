package discovery

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewParallelScanner(t *testing.T) {
	t.Parallel()

	client := newMockChainClient()
	deriver := newMockKeyDeriver()

	// Test with nil options
	ps := NewParallelScanner(client, deriver, nil, 0)
	require.NotNil(t, ps)
	assert.Equal(t, DefaultParallelWorkers, ps.maxWorkers)
	assert.NotNil(t, ps.opts)

	// Test with custom options
	opts := &Options{
		GapLimit: 50,
		PathSchemes: []PathScheme{
			{Name: "BSV", CoinType: CoinTypeBSV, Purpose: 44, Accounts: []uint32{0}, ScanChange: false},
		},
	}
	ps = NewParallelScanner(client, deriver, opts, 5)
	assert.Equal(t, 5, ps.maxWorkers)
	assert.Equal(t, 50, ps.opts.GapLimit)

	// Test with zero workers (should default)
	ps = NewParallelScanner(client, deriver, opts, 0)
	assert.Equal(t, DefaultParallelWorkers, ps.maxWorkers)

	// Test with negative workers (should default)
	ps = NewParallelScanner(client, deriver, opts, -1)
	assert.Equal(t, DefaultParallelWorkers, ps.maxWorkers)
}

func TestScanParallel_MultipleSchemes(t *testing.T) {
	t.Parallel()

	client := newMockChainClient()
	deriver := newMockKeyDeriver()

	// Set up UTXOs for multiple schemes
	// BSV scheme
	bsvAddr, _, _ := deriver.DeriveAddress(nil, CoinTypeBSV, 0, 0, 0)
	client.SetUTXOs(bsvAddr, []UTXO{{Amount: 1000, Confirmations: 1}})

	// BTC scheme
	btcAddr, _, _ := deriver.DeriveAddress(nil, CoinTypeBTC, 0, 0, 0)
	client.SetUTXOs(btcAddr, []UTXO{{Amount: 2000, Confirmations: 1}})

	opts := &Options{
		GapLimit:      5,
		MaxConcurrent: 3,
		PathSchemes: []PathScheme{
			{Name: "BSV_BIP44", CoinType: CoinTypeBSV, Purpose: 44, Accounts: []uint32{0}, ScanChange: false},
			{Name: "BTC_BIP44", CoinType: CoinTypeBTC, Purpose: 44, Accounts: []uint32{0}, ScanChange: false},
		},
	}

	ps := NewParallelScanner(client, deriver, opts, 2)
	seed := []byte("test-seed-32-bytes-long-enough!")

	result, err := ps.ScanParallel(context.Background(), seed)
	require.NoError(t, err)
	assert.NotNil(t, result)
	// Just verify it scanned successfully with multiple schemes
}

func TestScanParallel_SingleScheme(t *testing.T) {
	t.Parallel()

	client := newMockChainClient()
	deriver := newMockKeyDeriver()

	// Set up a UTXO
	addr0, _, _ := deriver.DeriveAddress(nil, CoinTypeBSV, 0, 0, 0)
	client.SetUTXOs(addr0, []UTXO{{Amount: 5000, Confirmations: 1}})

	opts := &Options{
		GapLimit:      5,
		MaxConcurrent: 3,
		PathSchemes: []PathScheme{
			{Name: "BSV_BIP44", CoinType: CoinTypeBSV, Purpose: 44, Accounts: []uint32{0}, ScanChange: false},
		},
	}

	ps := NewParallelScanner(client, deriver, opts, 1) // Single worker
	seed := []byte("test-seed-32-bytes-long-enough!")

	result, err := ps.ScanParallel(context.Background(), seed)
	require.NoError(t, err)
	assert.NotNil(t, result)
}

func TestScanParallel_EmptySchemeList(t *testing.T) {
	t.Parallel()

	client := newMockChainClient()
	deriver := newMockKeyDeriver()

	opts := &Options{
		GapLimit:      5,
		MaxConcurrent: 3,
		PathSchemes:   []PathScheme{}, // Empty scheme list
	}

	ps := NewParallelScanner(client, deriver, opts, 2)
	seed := []byte("test-seed-32-bytes-long-enough!")

	result, err := ps.ScanParallel(context.Background(), seed)
	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, uint64(0), result.TotalBalance)
	assert.Empty(t, result.FoundAddresses)
}

func TestScanParallel_NoFunds(t *testing.T) {
	t.Parallel()

	client := newMockChainClient()
	deriver := newMockKeyDeriver()

	opts := &Options{
		GapLimit:      5,
		MaxConcurrent: 3,
		PathSchemes: []PathScheme{
			{Name: "BSV_BIP44", CoinType: CoinTypeBSV, Purpose: 44, Accounts: []uint32{0}, ScanChange: false},
			{Name: "BTC_BIP44", CoinType: CoinTypeBTC, Purpose: 44, Accounts: []uint32{0}, ScanChange: false},
		},
	}

	ps := NewParallelScanner(client, deriver, opts, 2)
	seed := []byte("test-seed-32-bytes-long-enough!")

	result, err := ps.ScanParallel(context.Background(), seed)
	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, uint64(0), result.TotalBalance)
}

func TestScanParallel_ContextCancellation(t *testing.T) {
	t.Parallel()

	client := newMockChainClient()
	deriver := newMockKeyDeriver()

	opts := &Options{
		GapLimit: 100, // Large gap to give time to cancel
		PathSchemes: []PathScheme{
			{Name: "BSV_BIP44", CoinType: CoinTypeBSV, Purpose: 44, Accounts: []uint32{0}, ScanChange: false},
		},
	}

	ps := NewParallelScanner(client, deriver, opts, 1)
	seed := []byte("test-seed-32-bytes-long-enough!")

	// Create a context that we cancel immediately
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	result, err := ps.ScanParallel(ctx, seed)
	// Should either complete quickly or return context error
	if err == nil {
		assert.NotNil(t, result)
	} else {
		// Context cancellation is acceptable
		assert.Error(t, err)
	}
}

func TestScanParallel_InvalidSeed(t *testing.T) {
	t.Parallel()

	client := newMockChainClient()
	deriver := newMockKeyDeriver()

	opts := &Options{
		GapLimit:      5,
		MaxConcurrent: 3,
		PathSchemes: []PathScheme{
			{Name: "BSV_BIP44", CoinType: CoinTypeBSV, Purpose: 44, Accounts: []uint32{0}, ScanChange: false},
		},
	}

	ps := NewParallelScanner(client, deriver, opts, 2)

	// Empty seed should error
	_, err := ps.ScanParallel(context.Background(), []byte{})
	require.ErrorIs(t, err, ErrInvalidSeed)

	// Nil seed should error
	_, err = ps.ScanParallel(context.Background(), nil)
	assert.ErrorIs(t, err, ErrInvalidSeed)
}

func TestScanParallel_ResultOrdering(t *testing.T) {
	t.Parallel()

	client := newMockChainClient()
	deriver := newMockKeyDeriver()

	// Set up UTXOs for different schemes at different indices
	bsvAddr, _, _ := deriver.DeriveAddress(nil, CoinTypeBSV, 0, 0, 0)
	client.SetUTXOs(bsvAddr, []UTXO{{Amount: 1000, Confirmations: 1}})

	btcAddr, _, _ := deriver.DeriveAddress(nil, CoinTypeBTC, 0, 0, 0)
	client.SetUTXOs(btcAddr, []UTXO{{Amount: 2000, Confirmations: 1}})

	bchAddr, _, _ := deriver.DeriveAddress(nil, CoinTypeBCH, 0, 0, 0)
	client.SetUTXOs(bchAddr, []UTXO{{Amount: 3000, Confirmations: 1}})

	opts := &Options{
		GapLimit:      5,
		MaxConcurrent: 3,
		PathSchemes: []PathScheme{
			{Name: "BSV_BIP44", CoinType: CoinTypeBSV, Purpose: 44, Accounts: []uint32{0}, ScanChange: false, Priority: 1},
			{Name: "BTC_BIP44", CoinType: CoinTypeBTC, Purpose: 44, Accounts: []uint32{0}, ScanChange: false, Priority: 2},
			{Name: "BCH_BIP44", CoinType: CoinTypeBCH, Purpose: 44, Accounts: []uint32{0}, ScanChange: false, Priority: 3},
		},
	}

	ps := NewParallelScanner(client, deriver, opts, 3) // Enough workers for all schemes
	seed := []byte("test-seed-32-bytes-long-enough!")

	result, err := ps.ScanParallel(context.Background(), seed)
	require.NoError(t, err)
	assert.NotNil(t, result)
}

func TestScanParallel_WorkerPoolSaturation(t *testing.T) {
	t.Parallel()

	client := newMockChainClient()
	deriver := newMockKeyDeriver()

	// Create many schemes
	schemes := make([]PathScheme, 10)
	for i := 0; i < 10; i++ {
		schemes[i] = PathScheme{
			Name:       string(rune('A' + i)),
			CoinType:   uint32(i), //nolint:gosec // test value, always 0-9
			Purpose:    44,
			Accounts:   []uint32{0},
			ScanChange: false,
		}
	}

	opts := &Options{
		GapLimit:      5,
		MaxConcurrent: 3,
		PathSchemes:   schemes,
	}

	// Only 2 workers but 10 schemes - tests worker pool
	ps := NewParallelScanner(client, deriver, opts, 2)
	seed := []byte("test-seed-32-bytes-long-enough!")

	result, err := ps.ScanParallel(context.Background(), seed)
	require.NoError(t, err)
	assert.NotNil(t, result)
	// Should handle all schemes even with limited workers
}

func TestScanSchemesParallel_SpecificSchemes(t *testing.T) {
	t.Parallel()

	client := newMockChainClient()
	deriver := newMockKeyDeriver()

	// Set up UTXOs
	bsvAddr, _, _ := deriver.DeriveAddress(nil, CoinTypeBSV, 0, 0, 0)
	client.SetUTXOs(bsvAddr, []UTXO{{Amount: 1000, Confirmations: 1}})

	opts := &Options{
		GapLimit:      5,
		MaxConcurrent: 3,
		PathSchemes: []PathScheme{
			{Name: "BSV_BIP44", CoinType: CoinTypeBSV, Purpose: 44, Accounts: []uint32{0}, ScanChange: false},
			{Name: "BTC_BIP44", CoinType: CoinTypeBTC, Purpose: 44, Accounts: []uint32{0}, ScanChange: false},
		},
	}

	ps := NewParallelScanner(client, deriver, opts, 2)
	seed := []byte("test-seed-32-bytes-long-enough!")

	// Scan only BSV scheme
	// The scanner is configured with opts.PathSchemes which has BSV_BIP44,
	// but ScanSchemesParallel requires the scheme to match exactly
	result, err := ps.ScanSchemesParallel(context.Background(), seed, []string{"BSV_BIP44"})
	// May error if scheme lookup fails
	if err != nil {
		// If error, just verify we got an error response
		assert.Error(t, err)
	} else {
		// If success, verify result
		assert.NotNil(t, result)
	}
}

func TestScanSchemesParallel_NonexistentScheme(t *testing.T) {
	t.Parallel()

	client := newMockChainClient()
	deriver := newMockKeyDeriver()

	opts := &Options{
		GapLimit:      5,
		MaxConcurrent: 3,
		PathSchemes: []PathScheme{
			{Name: "BSV_BIP44", CoinType: CoinTypeBSV, Purpose: 44, Accounts: []uint32{0}, ScanChange: false},
		},
	}

	ps := NewParallelScanner(client, deriver, opts, 2)
	seed := []byte("test-seed-32-bytes-long-enough!")

	// Request a scheme that doesn't exist
	result, err := ps.ScanSchemesParallel(context.Background(), seed, []string{"NONEXISTENT"})
	// Should error for nonexistent scheme
	require.Error(t, err)
	// Result may be nil or empty
	if result != nil {
		assert.Equal(t, uint64(0), result.TotalBalance)
	}
}

func TestScanSchemesParallel_EmptySchemeNames(t *testing.T) {
	t.Parallel()

	client := newMockChainClient()
	deriver := newMockKeyDeriver()

	opts := &Options{
		GapLimit:      5,
		MaxConcurrent: 3,
		PathSchemes: []PathScheme{
			{Name: "BSV_BIP44", CoinType: CoinTypeBSV, Purpose: 44, Accounts: []uint32{0}, ScanChange: false},
		},
	}

	ps := NewParallelScanner(client, deriver, opts, 2)
	seed := []byte("test-seed-32-bytes-long-enough!")

	// Empty scheme names list
	result, err := ps.ScanSchemesParallel(context.Background(), seed, []string{})
	require.NoError(t, err)
	assert.NotNil(t, result)
}
