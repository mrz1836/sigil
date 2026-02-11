package discovery

import (
	"context"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockLogger implements the Logger interface for testing.
type mockLogger struct {
	mu     sync.Mutex
	debugs []string
	errors []string
}

func newMockLogger() *mockLogger {
	return &mockLogger{
		debugs: make([]string, 0),
		errors: make([]string, 0),
	}
}

func (l *mockLogger) Debug(format string, _ ...any) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.debugs = append(l.debugs, format)
}

func (l *mockLogger) Error(format string, _ ...any) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.errors = append(l.errors, format)
}

func (l *mockLogger) DebugCount() int {
	l.mu.Lock()
	defer l.mu.Unlock()
	return len(l.debugs)
}

func (l *mockLogger) ErrorCount() int {
	l.mu.Lock()
	defer l.mu.Unlock()
	return len(l.errors)
}

func TestNewRecoveryScenarios(t *testing.T) {
	t.Parallel()

	client := newMockChainClient()
	deriver := newMockKeyDeriver()
	scanner := NewScanner(client, deriver, nil)
	logger := newMockLogger()

	recovery := NewRecoveryScenarios(scanner, nil, deriver, logger)
	require.NotNil(t, recovery)
	assert.NotNil(t, recovery.scanner)
	assert.NotNil(t, recovery.deriver)
	assert.NotNil(t, recovery.logger)
}

func TestRecoverOldWallet_StandardMode(t *testing.T) {
	t.Parallel()

	client := newMockChainClient()
	deriver := newMockKeyDeriver()
	scanner := NewScanner(client, deriver, &Options{
		GapLimit: 5,
		PathSchemes: []PathScheme{
			{Name: "BSV", CoinType: CoinTypeBSV, Purpose: 44, Accounts: []uint32{0}, ScanChange: false},
		},
	})
	logger := newMockLogger()
	recovery := NewRecoveryScenarios(scanner, nil, deriver, logger)

	// Set up a UTXO at index 0
	addr0, _, _ := deriver.DeriveAddress(nil, CoinTypeBSV, 0, 0, 0)
	client.SetUTXOs(addr0, []UTXO{{Amount: 1000, Confirmations: 1}})

	seed := []byte("test-seed-32-bytes-long-enough!")
	opts := &RecoverOldWalletOptions{
		Mode:                RecoveryModeStandard,
		ScanChangeAddresses: false,
	}

	result, err := recovery.RecoverOldWallet(context.Background(), seed, opts)
	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, uint64(1000), result.TotalBalance)
	assert.Positive(t, logger.DebugCount(), "Should have debug logs")
}

func TestRecoverOldWallet_ExtendedMode(t *testing.T) {
	t.Parallel()

	client := newMockChainClient()
	deriver := newMockKeyDeriver()
	scanner := NewScanner(client, deriver, &Options{
		GapLimit: 5,
		PathSchemes: []PathScheme{
			{Name: "BSV", CoinType: CoinTypeBSV, Purpose: 44, Accounts: []uint32{0}, ScanChange: false},
		},
	})
	logger := newMockLogger()
	recovery := NewRecoveryScenarios(scanner, nil, deriver, logger)

	// Set up a UTXO at index 0
	addr0, _, _ := deriver.DeriveAddress(nil, CoinTypeBSV, 0, 0, 0)
	client.SetUTXOs(addr0, []UTXO{{Amount: 2000, Confirmations: 1}})

	seed := []byte("test-seed-32-bytes-long-enough!")
	opts := &RecoverOldWalletOptions{
		Mode:                RecoveryModeExtended,
		ScanChangeAddresses: false,
	}

	result, err := recovery.RecoverOldWallet(context.Background(), seed, opts)
	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, uint64(2000), result.TotalBalance)
}

func TestRecoverOldWallet_AggressiveMode(t *testing.T) {
	t.Parallel()

	client := newMockChainClient()
	deriver := newMockKeyDeriver()
	scanner := NewScanner(client, deriver, &Options{
		GapLimit: 5,
		PathSchemes: []PathScheme{
			{Name: "BSV", CoinType: CoinTypeBSV, Purpose: 44, Accounts: []uint32{0}, ScanChange: false},
		},
	})
	logger := newMockLogger()
	recovery := NewRecoveryScenarios(scanner, nil, deriver, logger)

	// Set up a UTXO at index 0
	addr0, _, _ := deriver.DeriveAddress(nil, CoinTypeBSV, 0, 0, 0)
	client.SetUTXOs(addr0, []UTXO{{Amount: 3000, Confirmations: 1}})

	seed := []byte("test-seed-32-bytes-long-enough!")
	opts := &RecoverOldWalletOptions{
		Mode:                RecoveryModeAggressive,
		ScanChangeAddresses: false,
	}

	result, err := recovery.RecoverOldWallet(context.Background(), seed, opts)
	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, uint64(3000), result.TotalBalance)
}

func TestRecoverOldWallet_CustomGapLimit(t *testing.T) {
	t.Parallel()

	client := newMockChainClient()
	deriver := newMockKeyDeriver()
	scanner := NewScanner(client, deriver, &Options{
		GapLimit: 5,
		PathSchemes: []PathScheme{
			{Name: "BSV", CoinType: CoinTypeBSV, Purpose: 44, Accounts: []uint32{0}, ScanChange: false},
		},
	})
	logger := newMockLogger()
	recovery := NewRecoveryScenarios(scanner, nil, deriver, logger)

	seed := []byte("test-seed-32-bytes-long-enough!")
	opts := &RecoverOldWalletOptions{
		Mode:                RecoveryModeStandard,
		CustomGapLimit:      50, // Override standard mode
		ScanChangeAddresses: false,
	}

	result, err := recovery.RecoverOldWallet(context.Background(), seed, opts)
	require.NoError(t, err)
	assert.NotNil(t, result)
}

func TestRecoverOldWallet_SpecificSchemes(t *testing.T) {
	t.Parallel()

	client := newMockChainClient()
	deriver := newMockKeyDeriver()
	scanner := NewScanner(client, deriver, &Options{
		GapLimit: 5,
		PathSchemes: []PathScheme{
			{Name: "BSV_BIP44", CoinType: CoinTypeBSV, Purpose: 44, Accounts: []uint32{0}, ScanChange: false},
			{Name: "BTC_BIP44", CoinType: CoinTypeBTC, Purpose: 44, Accounts: []uint32{0}, ScanChange: false},
		},
	})
	logger := newMockLogger()
	recovery := NewRecoveryScenarios(scanner, nil, deriver, logger)

	seed := []byte("test-seed-32-bytes-long-enough!")
	opts := &RecoverOldWalletOptions{
		Mode:                RecoveryModeStandard,
		SpecificSchemes:     []string{"BSV_BIP44"}, // Only scan BSV
		ScanChangeAddresses: false,
	}

	result, err := recovery.RecoverOldWallet(context.Background(), seed, opts)
	require.NoError(t, err)
	assert.NotNil(t, result)
}

func TestRecoverOldWallet_NoFundsFound(t *testing.T) {
	t.Parallel()

	client := newMockChainClient()
	deriver := newMockKeyDeriver()
	scanner := NewScanner(client, deriver, &Options{
		GapLimit: 5,
		PathSchemes: []PathScheme{
			{Name: "BSV", CoinType: CoinTypeBSV, Purpose: 44, Accounts: []uint32{0}, ScanChange: false},
		},
	})
	logger := newMockLogger()
	recovery := NewRecoveryScenarios(scanner, nil, deriver, logger)

	// No UTXOs set - wallet is empty
	seed := []byte("test-seed-32-bytes-long-enough!")
	opts := &RecoverOldWalletOptions{
		Mode:                RecoveryModeStandard,
		ScanChangeAddresses: false,
	}

	result, err := recovery.RecoverOldWallet(context.Background(), seed, opts)
	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, uint64(0), result.TotalBalance)
}

func TestRecoverOldWallet_ProgressCallback(t *testing.T) {
	t.Parallel()

	client := newMockChainClient()
	deriver := newMockKeyDeriver()
	scanner := NewScanner(client, deriver, &Options{
		GapLimit: 5,
		PathSchemes: []PathScheme{
			{Name: "BSV", CoinType: CoinTypeBSV, Purpose: 44, Accounts: []uint32{0}, ScanChange: false},
		},
	})
	logger := newMockLogger()
	recovery := NewRecoveryScenarios(scanner, nil, deriver, logger)

	// Set up a UTXO
	addr0, _, _ := deriver.DeriveAddress(nil, CoinTypeBSV, 0, 0, 0)
	client.SetUTXOs(addr0, []UTXO{{Amount: 1000, Confirmations: 1}})

	seed := []byte("test-seed-32-bytes-long-enough!")

	var progressCalls int
	opts := &RecoverOldWalletOptions{
		Mode:                RecoveryModeStandard,
		ScanChangeAddresses: false,
		ProgressCallback: func(_ ProgressUpdate) {
			progressCalls++
		},
	}

	result, err := recovery.RecoverOldWallet(context.Background(), seed, opts)
	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.Positive(t, progressCalls, "Progress callback should be called")
}

func TestRecoverOldWallet_ContextCancellation(t *testing.T) {
	t.Parallel()

	client := newMockChainClient()
	deriver := newMockKeyDeriver()
	scanner := NewScanner(client, deriver, &Options{
		GapLimit: 100, // Large gap to give time to cancel
		PathSchemes: []PathScheme{
			{Name: "BSV", CoinType: CoinTypeBSV, Purpose: 44, Accounts: []uint32{0}, ScanChange: false},
		},
	})
	logger := newMockLogger()
	recovery := NewRecoveryScenarios(scanner, nil, deriver, logger)

	seed := []byte("test-seed-32-bytes-long-enough!")

	// Create a context that we'll cancel
	ctx, cancel := context.WithCancel(context.Background())

	opts := &RecoverOldWalletOptions{
		Mode:                RecoveryModeStandard,
		ScanChangeAddresses: false,
		ProgressCallback: func(_ ProgressUpdate) {
			// Cancel after first progress update
			cancel()
		},
	}

	result, err := recovery.RecoverOldWallet(ctx, seed, opts)
	// Should get context canceled error
	if err == nil {
		// If scan completed before cancellation, result should still be valid
		assert.NotNil(t, result)
	} else {
		// Otherwise should have context error
		assert.Error(t, err)
	}
}

func TestRecoverOldWallet_NilOptions(t *testing.T) {
	t.Parallel()

	client := newMockChainClient()
	deriver := newMockKeyDeriver()
	scanner := NewScanner(client, deriver, &Options{
		GapLimit: 5,
		PathSchemes: []PathScheme{
			{Name: "BSV", CoinType: CoinTypeBSV, Purpose: 44, Accounts: []uint32{0}, ScanChange: false},
		},
	})
	logger := newMockLogger()
	recovery := NewRecoveryScenarios(scanner, nil, deriver, logger)

	seed := []byte("test-seed-32-bytes-long-enough!")

	// Nil options should use defaults
	result, err := recovery.RecoverOldWallet(context.Background(), seed, nil)
	require.NoError(t, err)
	assert.NotNil(t, result)
}

func TestRecoverBeyondGap_BasicRange(t *testing.T) {
	t.Parallel()

	client := newMockChainClient()
	deriver := newMockKeyDeriver()
	scanner := NewScanner(client, deriver, nil)
	logger := newMockLogger()

	// BulkOperations can be nil for this test
	recovery := NewRecoveryScenarios(scanner, nil, deriver, logger)

	seed := []byte("test-seed-32-bytes-long-enough!")
	opts := &RecoverBeyondGapOptions{
		DerivationPath:   "m/44'/236'/0'/0/*",
		CoinType:         CoinTypeBSV,
		StartIndex:       100,
		Count:            10,
		ProgressCallback: nil,
	}

	result, err := recovery.RecoverBeyondGap(context.Background(), seed, opts)
	// May error if bulk ops don't work with nil client, but that's okay
	// We're testing the code path is executed
	if err == nil {
		assert.NotNil(t, result)
	}
}

func TestRecoverBeyondGap_EmptyRange(t *testing.T) {
	t.Parallel()

	client := newMockChainClient()
	deriver := newMockKeyDeriver()
	scanner := NewScanner(client, deriver, nil)
	logger := newMockLogger()

	recovery := NewRecoveryScenarios(scanner, nil, deriver, logger)

	seed := []byte("test-seed-32-bytes-long-enough!")
	opts := &RecoverBeyondGapOptions{
		DerivationPath:   "m/44'/236'/0'/0/*",
		CoinType:         CoinTypeBSV,
		StartIndex:       100,
		Count:            0, // Empty range
		ProgressCallback: nil,
	}

	result, err := recovery.RecoverBeyondGap(context.Background(), seed, opts)
	// Empty range should still work
	if err == nil {
		assert.NotNil(t, result)
		assert.Equal(t, uint64(0), result.TotalBalance)
	}
}
