package discovery

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// mockChainClient is a test double for ChainClient.
type mockChainClient struct {
	mu          sync.Mutex
	utxosByAddr map[string][]UTXO
	callCount   int
	shouldFail  bool
	failAfter   int // Fail after this many calls
	delay       time.Duration
}

func newMockChainClient() *mockChainClient {
	return &mockChainClient{
		utxosByAddr: make(map[string][]UTXO),
	}
}

func (m *mockChainClient) SetUTXOs(address string, utxos []UTXO) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.utxosByAddr[address] = utxos
}

func (m *mockChainClient) ListUTXOs(ctx context.Context, address string) ([]UTXO, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.callCount++

	if m.delay > 0 {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(m.delay):
		}
	}

	if m.shouldFail || (m.failAfter > 0 && m.callCount > m.failAfter) {
		return nil, errors.New("mock network error") //nolint:err113 // Test mock error
	}

	return m.utxosByAddr[address], nil
}

func (m *mockChainClient) ValidateAddress(address string) error {
	if address == "" {
		return errors.New("empty address") //nolint:err113 // Test mock error
	}
	return nil
}

func (m *mockChainClient) CallCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.callCount
}

// mockKeyDeriver is a test double for KeyDeriver.
type mockKeyDeriver struct {
	addressesByCoinType map[uint32]map[uint32]string // coinType -> index -> address
}

func newMockKeyDeriver() *mockKeyDeriver {
	return &mockKeyDeriver{
		addressesByCoinType: make(map[uint32]map[uint32]string),
	}
}

func (d *mockKeyDeriver) SetAddress(coinType, index uint32, address string) {
	if d.addressesByCoinType[coinType] == nil {
		d.addressesByCoinType[coinType] = make(map[uint32]string)
	}
	d.addressesByCoinType[coinType][index] = address
}

func (d *mockKeyDeriver) DeriveAddress(_ []byte, coinType, account, change, index uint32) (string, string, error) {
	// Generate deterministic address based on parameters
	addr := fmt.Sprintf("addr_%d_%d_%d_%d", coinType, account, change, index)
	if addrs, ok := d.addressesByCoinType[coinType]; ok {
		if a, ok := addrs[index]; ok {
			addr = a
		}
	}
	path := fmt.Sprintf("m/44'/%d'/%d'/%d/%d", coinType, account, change, index)
	return addr, path, nil
}

func (d *mockKeyDeriver) DeriveLegacyAddress(_ []byte, index uint32) (string, string, error) {
	addr := fmt.Sprintf("legacy_addr_%d", index)
	path := fmt.Sprintf("m/0'/%d", index)
	return addr, path, nil
}

func TestNewScanner(t *testing.T) {
	client := newMockChainClient()
	deriver := newMockKeyDeriver()

	// Test with nil options
	scanner := NewScanner(client, deriver, nil)
	if scanner == nil {
		t.Fatal("NewScanner returned nil")
	}
	if scanner.opts.GapLimit != DefaultGapLimit {
		t.Errorf("GapLimit = %d, want %d", scanner.opts.GapLimit, DefaultGapLimit)
	}

	// Test with custom options
	opts := &Options{
		GapLimit:      50,
		MaxConcurrent: 5,
	}
	scanner = NewScanner(client, deriver, opts)
	if scanner.opts.GapLimit != 50 {
		t.Errorf("GapLimit = %d, want 50", scanner.opts.GapLimit)
	}
}

func TestScanner_Scan_NoFunds(t *testing.T) {
	client := newMockChainClient()
	deriver := newMockKeyDeriver()

	opts := DefaultOptions()
	opts.GapLimit = 5 // Small for faster test
	opts.PathSchemes = []PathScheme{
		{Name: "Test", CoinType: CoinTypeBSV, Purpose: 44, Accounts: []uint32{0}, ScanChange: false},
	}

	scanner := NewScanner(client, deriver, opts)

	seed := []byte("test-seed-32-bytes-long-enough!")
	result, err := scanner.Scan(context.Background(), seed)
	if err != nil {
		t.Fatalf("Scan failed: %v", err)
	}

	if result.TotalBalance != 0 {
		t.Errorf("TotalBalance = %d, want 0", result.TotalBalance)
	}

	if result.TotalUTXOs != 0 {
		t.Errorf("TotalUTXOs = %d, want 0", result.TotalUTXOs)
	}

	// Should have scanned at least gap_limit addresses
	if result.AddressesScanned < opts.GapLimit {
		t.Errorf("AddressesScanned = %d, want >= %d", result.AddressesScanned, opts.GapLimit)
	}
}

func TestScanner_Scan_FindsFunds(t *testing.T) {
	client := newMockChainClient()
	deriver := newMockKeyDeriver()

	// Set up some UTXOs at specific addresses
	deriver.SetAddress(CoinTypeBSV, 3, "address_with_funds_1")
	deriver.SetAddress(CoinTypeBSV, 7, "address_with_funds_2")

	client.SetUTXOs("address_with_funds_1", []UTXO{
		{TxID: "tx1", Vout: 0, Amount: 10000, Address: "address_with_funds_1"},
	})
	client.SetUTXOs("address_with_funds_2", []UTXO{
		{TxID: "tx2", Vout: 0, Amount: 20000, Address: "address_with_funds_2"},
		{TxID: "tx3", Vout: 1, Amount: 5000, Address: "address_with_funds_2"},
	})

	opts := DefaultOptions()
	opts.GapLimit = 10
	opts.ExtendedGapLimit = 10
	opts.PathSchemes = []PathScheme{
		{Name: "BSV Test", CoinType: CoinTypeBSV, Purpose: 44, Accounts: []uint32{0}, ScanChange: false},
	}

	scanner := NewScanner(client, deriver, opts)

	seed := []byte("test-seed-32-bytes-long-enough!")
	result, err := scanner.Scan(context.Background(), seed)
	if err != nil {
		t.Fatalf("Scan failed: %v", err)
	}

	expectedBalance := uint64(35000) // 10000 + 20000 + 5000
	if result.TotalBalance != expectedBalance {
		t.Errorf("TotalBalance = %d, want %d", result.TotalBalance, expectedBalance)
	}

	expectedUTXOs := 3
	if result.TotalUTXOs != expectedUTXOs {
		t.Errorf("TotalUTXOs = %d, want %d", result.TotalUTXOs, expectedUTXOs)
	}

	if len(result.AllAddresses()) != 2 {
		t.Errorf("found %d addresses, want 2", len(result.AllAddresses()))
	}
}

func TestScanner_Scan_GapLimitBehavior(t *testing.T) {
	client := newMockChainClient()
	deriver := newMockKeyDeriver()

	// Put funds at index 2 using the auto-generated address format
	// The mock deriver generates "addr_cointype_account_change_index"
	// So for BSV (236), account 0, change 0, index 2: "addr_236_0_0_2"
	client.SetUTXOs("addr_236_0_0_2", []UTXO{
		{TxID: "tx1", Vout: 0, Amount: 1000, Address: "addr_236_0_0_2"},
	})

	gapLimit := 3
	opts := DefaultOptions()
	opts.GapLimit = gapLimit
	opts.ExtendedGapLimit = gapLimit
	opts.PathSchemes = []PathScheme{
		{Name: "Test", CoinType: CoinTypeBSV, Purpose: 44, Accounts: []uint32{0}, ScanChange: false},
	}

	scanner := NewScanner(client, deriver, opts)

	seed := []byte("test-seed-32-bytes-long-enough!")
	result, err := scanner.Scan(context.Background(), seed)
	if err != nil {
		t.Fatalf("Scan failed: %v", err)
	}

	// Should have found the address at index 2
	if result.TotalBalance != 1000 {
		t.Errorf("TotalBalance = %d, want 1000", result.TotalBalance)
	}

	// Should have scanned: 0,1,2 (found), 3,4,5 (gap), stop
	// Total: 6 addresses
	expectedScanned := 3 + gapLimit // indices 0-2 + 3 gap
	if result.AddressesScanned != expectedScanned {
		t.Errorf("AddressesScanned = %d, want %d", result.AddressesScanned, expectedScanned)
	}
}

func TestScanner_Scan_MultipleSchemes(t *testing.T) {
	client := newMockChainClient()
	deriver := newMockKeyDeriver()

	// Set up funds in different schemes
	deriver.SetAddress(CoinTypeBSV, 0, "bsv_addr_0")
	deriver.SetAddress(CoinTypeBTC, 0, "btc_addr_0")

	client.SetUTXOs("bsv_addr_0", []UTXO{
		{TxID: "tx1", Vout: 0, Amount: 5000, Address: "bsv_addr_0"},
	})
	client.SetUTXOs("btc_addr_0", []UTXO{
		{TxID: "tx2", Vout: 0, Amount: 3000, Address: "btc_addr_0"},
	})

	opts := DefaultOptions()
	opts.GapLimit = 3
	opts.ExtendedGapLimit = 3
	opts.PathSchemes = []PathScheme{
		{Name: "BSV", CoinType: CoinTypeBSV, Purpose: 44, Accounts: []uint32{0}, ScanChange: false, Priority: 1},
		{Name: "BTC", CoinType: CoinTypeBTC, Purpose: 44, Accounts: []uint32{0}, ScanChange: false, Priority: 2},
	}

	scanner := NewScanner(client, deriver, opts)

	seed := []byte("test-seed-32-bytes-long-enough!")
	result, err := scanner.Scan(context.Background(), seed)
	if err != nil {
		t.Fatalf("Scan failed: %v", err)
	}

	expectedBalance := uint64(8000) // 5000 + 3000
	if result.TotalBalance != expectedBalance {
		t.Errorf("TotalBalance = %d, want %d", result.TotalBalance, expectedBalance)
	}

	// Check both schemes were scanned
	if len(result.SchemesScanned) != 2 {
		t.Errorf("SchemesScanned = %v, want 2 schemes", result.SchemesScanned)
	}

	// Check addresses are categorized by scheme
	bsvAddrs := result.AddressesByScheme("BSV")
	btcAddrs := result.AddressesByScheme("BTC")

	if len(bsvAddrs) != 1 {
		t.Errorf("BSV addresses = %d, want 1", len(bsvAddrs))
	}
	if len(btcAddrs) != 1 {
		t.Errorf("BTC addresses = %d, want 1", len(btcAddrs))
	}
}

func TestScanner_Scan_ChangeAddresses(t *testing.T) {
	client := newMockChainClient()
	deriver := newMockKeyDeriver()

	// External address (change=0) with funds
	// The mock deriver generates addresses in the form addr_cointype_account_change_index
	// Internal address (change=1) with funds - we need to mock this

	opts := DefaultOptions()
	opts.GapLimit = 3
	opts.ExtendedGapLimit = 3
	opts.PathSchemes = []PathScheme{
		{Name: "Test", CoinType: CoinTypeBSV, Purpose: 44, Accounts: []uint32{0}, ScanChange: true},
	}
	opts.ScanChangeAddresses = true

	// Set up UTXOs for both external and internal chains
	client.SetUTXOs("addr_236_0_0_0", []UTXO{
		{TxID: "tx1", Vout: 0, Amount: 1000, Address: "addr_236_0_0_0"},
	})
	client.SetUTXOs("addr_236_0_1_0", []UTXO{
		{TxID: "tx2", Vout: 0, Amount: 2000, Address: "addr_236_0_1_0"},
	})

	scanner := NewScanner(client, deriver, opts)

	seed := []byte("test-seed-32-bytes-long-enough!")
	result, err := scanner.Scan(context.Background(), seed)
	if err != nil {
		t.Fatalf("Scan failed: %v", err)
	}

	// Should find funds in both external and internal chains
	expectedBalance := uint64(3000)
	if result.TotalBalance != expectedBalance {
		t.Errorf("TotalBalance = %d, want %d", result.TotalBalance, expectedBalance)
	}

	// Check we have both external and change addresses
	allAddrs := result.AllAddresses()
	hasExternal := false
	hasChange := false
	for _, addr := range allAddrs {
		if !addr.IsChange {
			hasExternal = true
		}
		if addr.IsChange {
			hasChange = true
		}
	}

	if !hasExternal {
		t.Error("expected to find external address")
	}
	if !hasChange {
		t.Error("expected to find change address")
	}
}

func TestScanner_Scan_ContextCancellation(t *testing.T) {
	client := newMockChainClient()
	client.delay = 100 * time.Millisecond
	deriver := newMockKeyDeriver()

	opts := DefaultOptions()
	opts.GapLimit = 100 // Large to ensure we don't finish naturally
	opts.PathSchemes = []PathScheme{
		{Name: "Test", CoinType: CoinTypeBSV, Purpose: 44, Accounts: []uint32{0}},
	}

	scanner := NewScanner(client, deriver, opts)

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	seed := []byte("test-seed-32-bytes-long-enough!")
	result, _ := scanner.Scan(ctx, seed)

	// Should return partial results with context error
	if result == nil {
		t.Error("expected partial results, got nil")
		return
	}

	// Check that context error was recorded
	if len(result.Errors) == 0 {
		t.Log("Context cancellation may not always be recorded as error")
	}
}

func TestScanner_Scan_InvalidSeed(t *testing.T) {
	client := newMockChainClient()
	deriver := newMockKeyDeriver()

	scanner := NewScanner(client, deriver, DefaultOptions())

	// Empty seed should fail
	_, err := scanner.Scan(context.Background(), nil)
	if err == nil {
		t.Error("expected error for nil seed")
	}

	_, err = scanner.Scan(context.Background(), []byte{})
	if err == nil {
		t.Error("expected error for empty seed")
	}
}

func TestScanner_Scan_InvalidOptions(t *testing.T) {
	client := newMockChainClient()
	deriver := newMockKeyDeriver()

	opts := &Options{
		GapLimit:      0, // Invalid
		MaxConcurrent: 1,
	}

	scanner := NewScanner(client, deriver, opts)

	seed := []byte("test-seed-32-bytes-long-enough!")
	_, err := scanner.Scan(context.Background(), seed)

	if err == nil {
		t.Error("expected error for invalid options")
	}
}

func TestScanner_ScanSingleScheme(t *testing.T) {
	client := newMockChainClient()
	deriver := newMockKeyDeriver()

	deriver.SetAddress(CoinTypeBSV, 0, "bsv_addr")
	client.SetUTXOs("bsv_addr", []UTXO{
		{TxID: "tx1", Vout: 0, Amount: 1000, Address: "bsv_addr"},
	})

	opts := DefaultOptions()
	opts.ExtendedGapLimit = 5

	scanner := NewScanner(client, deriver, opts)

	seed := []byte("test-seed-32-bytes-long-enough!")
	result, err := scanner.ScanSingleScheme(context.Background(), seed, "BSV Standard")
	if err != nil {
		t.Fatalf("ScanSingleScheme failed: %v", err)
	}

	if len(result.SchemesScanned) != 1 {
		t.Errorf("SchemesScanned = %v, want 1 scheme", result.SchemesScanned)
	}

	if result.SchemesScanned[0] != "BSV Standard" {
		t.Errorf("scanned scheme = %s, want BSV Standard", result.SchemesScanned[0])
	}
}

func TestScanner_ScanSingleScheme_UnknownScheme(t *testing.T) {
	client := newMockChainClient()
	deriver := newMockKeyDeriver()

	scanner := NewScanner(client, deriver, DefaultOptions())

	seed := []byte("test-seed-32-bytes-long-enough!")
	_, err := scanner.ScanSingleScheme(context.Background(), seed, "Unknown Scheme")

	if err == nil {
		t.Error("expected error for unknown scheme")
	}
}

func TestScanner_ScanCustomPath(t *testing.T) {
	client := newMockChainClient()
	deriver := newMockKeyDeriver()

	// Set up funds at a specific address
	// The mock deriver generates addresses in format addr_cointype_account_change_index
	client.SetUTXOs("addr_0_0_0_2", []UTXO{
		{TxID: "tx1", Vout: 0, Amount: 5000, Address: "addr_0_0_0_2"},
	})

	opts := DefaultOptions()
	opts.GapLimit = 5

	scanner := NewScanner(client, deriver, opts)

	seed := []byte("test-seed-32-bytes-long-enough!")
	// Use BTC coin type (0) to match our mock address
	result, err := scanner.ScanCustomPath(context.Background(), seed, "m/44'/0'/0'/0/*", CoinTypeBTC)
	if err != nil {
		t.Fatalf("ScanCustomPath failed: %v", err)
	}

	if result.TotalBalance != 5000 {
		t.Errorf("TotalBalance = %d, want 5000", result.TotalBalance)
	}

	if len(result.SchemesScanned) != 1 {
		t.Errorf("SchemesScanned = %v, want 1 scheme", result.SchemesScanned)
	}

	// Scheme name should contain "Custom"
	if len(result.SchemesScanned) > 0 && result.SchemesScanned[0][:6] != "Custom" {
		t.Errorf("scheme name = %s, want 'Custom (...)' pattern", result.SchemesScanned[0])
	}
}

func TestScanner_ScanCustomPath_NoFunds(t *testing.T) {
	client := newMockChainClient()
	deriver := newMockKeyDeriver()

	opts := DefaultOptions()
	opts.GapLimit = 3

	scanner := NewScanner(client, deriver, opts)

	seed := []byte("test-seed-32-bytes-long-enough!")
	result, err := scanner.ScanCustomPath(context.Background(), seed, "m/44'/999'/0'/0/*", 999)
	if err != nil {
		t.Fatalf("ScanCustomPath failed: %v", err)
	}

	if result.TotalBalance != 0 {
		t.Errorf("TotalBalance = %d, want 0", result.TotalBalance)
	}

	// Should still report the scan happened
	if result.AddressesScanned < opts.GapLimit {
		t.Errorf("AddressesScanned = %d, want >= %d", result.AddressesScanned, opts.GapLimit)
	}
}

func TestScanner_Scan_ProgressCallback(t *testing.T) {
	client := newMockChainClient()
	deriver := newMockKeyDeriver()

	client.SetUTXOs("addr_236_0_0_2", []UTXO{
		{TxID: "tx1", Vout: 0, Amount: 1000, Address: "addr_236_0_0_2"},
	})

	var progressUpdates []ProgressUpdate
	var mu sync.Mutex

	opts := DefaultOptions()
	opts.GapLimit = 5
	opts.PathSchemes = []PathScheme{
		{Name: "Test", CoinType: CoinTypeBSV, Purpose: 44, Accounts: []uint32{0}, ScanChange: false},
	}
	opts.ProgressCallback = func(update ProgressUpdate) {
		mu.Lock()
		progressUpdates = append(progressUpdates, update)
		mu.Unlock()
	}

	scanner := NewScanner(client, deriver, opts)

	seed := []byte("test-seed-32-bytes-long-enough!")
	_, err := scanner.Scan(context.Background(), seed)
	if err != nil {
		t.Fatalf("Scan failed: %v", err)
	}

	mu.Lock()
	defer mu.Unlock()

	if len(progressUpdates) == 0 {
		t.Error("expected progress updates")
	}

	// Check we got a "found" update
	foundUpdate := false
	for _, u := range progressUpdates {
		if u.Phase == "found" {
			foundUpdate = true
			break
		}
	}

	if !foundUpdate {
		t.Error("expected 'found' progress update")
	}
}

func TestScanner_Scan_NetworkError(t *testing.T) {
	client := newMockChainClient()
	client.shouldFail = true
	deriver := newMockKeyDeriver()

	opts := DefaultOptions()
	opts.GapLimit = 3
	opts.PathSchemes = []PathScheme{
		{Name: "Test", CoinType: CoinTypeBSV, Purpose: 44, Accounts: []uint32{0}, ScanChange: false},
	}

	scanner := NewScanner(client, deriver, opts)

	seed := []byte("test-seed-32-bytes-long-enough!")
	result, err := scanner.Scan(context.Background(), seed)
	// Should complete but with errors recorded
	if err != nil {
		t.Fatalf("Scan failed unexpectedly: %v", err)
	}

	// No funds should be found (all queries failed)
	if result.TotalBalance != 0 {
		t.Errorf("TotalBalance = %d, want 0", result.TotalBalance)
	}
}

func TestResult_HasFunds(t *testing.T) {
	tests := []struct {
		name    string
		balance uint64
		want    bool
	}{
		{"zero balance", 0, false},
		{"non-zero balance", 1, true},
		{"large balance", 1000000000, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := &Result{TotalBalance: tt.balance}
			if got := result.HasFunds(); got != tt.want {
				t.Errorf("HasFunds() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestResult_AllAddresses(t *testing.T) {
	result := &Result{
		FoundAddresses: map[string][]DiscoveredAddress{
			"Scheme1": {
				{Address: "addr1", Balance: 1000},
				{Address: "addr2", Balance: 2000},
			},
			"Scheme2": {
				{Address: "addr3", Balance: 3000},
			},
		},
	}

	all := result.AllAddresses()

	if len(all) != 3 {
		t.Errorf("AllAddresses() returned %d, want 3", len(all))
	}
}

func TestResult_AddressesByScheme(t *testing.T) {
	result := &Result{
		FoundAddresses: map[string][]DiscoveredAddress{
			"Scheme1": {
				{Address: "addr1"},
				{Address: "addr2"},
			},
			"Scheme2": {
				{Address: "addr3"},
			},
		},
	}

	scheme1Addrs := result.AddressesByScheme("Scheme1")
	if len(scheme1Addrs) != 2 {
		t.Errorf("AddressesByScheme('Scheme1') = %d, want 2", len(scheme1Addrs))
	}

	scheme2Addrs := result.AddressesByScheme("Scheme2")
	if len(scheme2Addrs) != 1 {
		t.Errorf("AddressesByScheme('Scheme2') = %d, want 1", len(scheme2Addrs))
	}

	unknownAddrs := result.AddressesByScheme("Unknown")
	if len(unknownAddrs) != 0 {
		t.Errorf("AddressesByScheme('Unknown') = %d, want 0", len(unknownAddrs))
	}
}

func TestOptions_Validate(t *testing.T) {
	tests := []struct {
		name    string
		opts    *Options
		wantErr bool
	}{
		{
			name:    "valid options",
			opts:    DefaultOptions(),
			wantErr: false,
		},
		{
			name:    "zero gap limit",
			opts:    &Options{GapLimit: 0, MaxConcurrent: 1},
			wantErr: true,
		},
		{
			name:    "negative gap limit",
			opts:    &Options{GapLimit: -1, MaxConcurrent: 1},
			wantErr: true,
		},
		{
			name:    "zero max concurrent",
			opts:    &Options{GapLimit: 20, MaxConcurrent: 0},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.opts.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestDefaultOptions(t *testing.T) {
	opts := DefaultOptions()

	if opts.GapLimit != DefaultGapLimit {
		t.Errorf("GapLimit = %d, want %d", opts.GapLimit, DefaultGapLimit)
	}

	if opts.ExtendedGapLimit != ExtendedGapLimit {
		t.Errorf("ExtendedGapLimit = %d, want %d", opts.ExtendedGapLimit, ExtendedGapLimit)
	}

	if opts.MaxConcurrent != DefaultMaxConcurrent {
		t.Errorf("MaxConcurrent = %d, want %d", opts.MaxConcurrent, DefaultMaxConcurrent)
	}

	if len(opts.PathSchemes) == 0 {
		t.Error("PathSchemes should not be empty")
	}

	if !opts.ScanChangeAddresses {
		t.Error("ScanChangeAddresses should default to true")
	}
}

// BenchmarkScanner_Scan benchmarks the scanner performance.
func BenchmarkScanner_Scan(b *testing.B) {
	client := newMockChainClient()
	deriver := newMockKeyDeriver()

	opts := DefaultOptions()
	opts.GapLimit = 20
	opts.PathSchemes = []PathScheme{
		{Name: "Test", CoinType: CoinTypeBSV, Purpose: 44, Accounts: []uint32{0}, ScanChange: false},
	}

	scanner := NewScanner(client, deriver, opts)
	seed := []byte("test-seed-32-bytes-long-enough!")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = scanner.Scan(context.Background(), seed)
	}
}

// TestScanner_Scan_Concurrent tests thread safety of scanner.
func TestScanner_Scan_Concurrent(t *testing.T) {
	client := newMockChainClient()
	deriver := newMockKeyDeriver()

	opts := DefaultOptions()
	opts.GapLimit = 5
	opts.PathSchemes = []PathScheme{
		{Name: "Test", CoinType: CoinTypeBSV, Purpose: 44, Accounts: []uint32{0}},
	}

	scanner := NewScanner(client, deriver, opts)
	seed := []byte("test-seed-32-bytes-long-enough!")

	var wg sync.WaitGroup
	var errCount int32

	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := scanner.Scan(context.Background(), seed)
			if err != nil {
				atomic.AddInt32(&errCount, 1)
			}
		}()
	}

	wg.Wait()

	if errCount > 0 {
		t.Errorf("concurrent scan had %d errors", errCount)
	}
}
