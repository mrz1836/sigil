package discovery

import (
	"context"
	"errors"
	"testing"
)

var errMockBulk = errors.New("mock bulk error")

// MockBulkOperations is a test double for BulkOperations.
type MockBulkOperations struct {
	activityMap map[string]bool
	utxosMap    map[string][]UTXO
	shouldFail  bool
}

func newMockBulkOperations() *MockBulkOperations {
	return &MockBulkOperations{
		activityMap: make(map[string]bool),
		utxosMap:    make(map[string][]UTXO),
	}
}

func (m *MockBulkOperations) SetActivity(address string, hasHistory bool) {
	m.activityMap[address] = hasHistory
}

func (m *MockBulkOperations) SetUTXOs(address string, utxos []UTXO) {
	m.utxosMap[address] = utxos
}

func (m *MockBulkOperations) BulkAddressActivityCheck(_ context.Context, addresses []string) ([]AddressActivity, error) {
	if m.shouldFail {
		return nil, errMockBulk
	}

	results := make([]AddressActivity, len(addresses))
	for i, addr := range addresses {
		results[i] = AddressActivity{
			Address:    addr,
			HasHistory: m.activityMap[addr],
		}
	}
	return results, nil
}

func (m *MockBulkOperations) BulkAddressUTXOFetch(_ context.Context, addresses []string) ([]BulkUTXOResult, error) {
	if m.shouldFail {
		return nil, errMockBulk
	}

	results := make([]BulkUTXOResult, len(addresses))
	for i, addr := range addresses {
		// Divide UTXOs into confirmed and unconfirmed for testing purposes
		// For simplicity, let's say all are confirmed in this mock unless specified
		allUTXOs := m.utxosMap[addr]
		results[i] = BulkUTXOResult{
			Address:        addr,
			ConfirmedUTXOs: allUTXOs,
		}
	}
	return results, nil
}

func TestScanner_Scan_Bulk(t *testing.T) {
	client := newMockChainClient()
	deriver := newMockKeyDeriver()
	bulkOps := newMockBulkOperations()

	// Setup: Address at index 5 has activity and funds
	targetAddr := "addr_236_0_0_5"
	deriver.SetAddress(CoinTypeBSV, 5, targetAddr)

	val := uint64(5000)
	utxos := []UTXO{{TxID: "tx1", Vout: 0, Amount: val, Address: targetAddr}}

	// Set mock data
	bulkOps.SetActivity(targetAddr, true)
	bulkOps.SetUTXOs(targetAddr, utxos)

	// Also set in client for fallback/verification if needed, though bulk should take precedence
	client.SetUTXOs(targetAddr, utxos)

	opts := DefaultOptions()
	opts.GapLimit = 20
	// Ensure batch size (20) covers our target index 5
	opts.PathSchemes = []PathScheme{
		{Name: "BulkTest", CoinType: CoinTypeBSV, Purpose: 44, Accounts: []uint32{0}, ScanChange: false},
	}

	scanner := NewScannerWithBulk(client, deriver, opts, bulkOps)

	seed := []byte("test-seed")
	result, err := scanner.Scan(context.Background(), seed)
	if err != nil {
		t.Fatalf("Scan failed: %v", err)
	}

	if result.TotalBalance != val {
		t.Errorf("TotalBalance = %d, want %d", result.TotalBalance, val)
	}

	if len(result.AllAddresses()) != 1 {
		t.Errorf("Found %d addresses, want 1", len(result.AllAddresses()))
	}

	if result.AllAddresses()[0].Address != targetAddr {
		t.Errorf("Found address %s, want %s", result.AllAddresses()[0].Address, targetAddr)
	}
}

func TestScanner_Scan_Bulk_Fallback(t *testing.T) {
	client := newMockChainClient()
	deriver := newMockKeyDeriver()
	bulkOps := newMockBulkOperations()
	bulkOps.shouldFail = true // Force fallback

	targetAddr := "addr_236_0_0_2"
	deriver.SetAddress(CoinTypeBSV, 2, targetAddr)

	val := uint64(1000)
	utxos := []UTXO{{TxID: "fallback", Vout: 0, Amount: val, Address: targetAddr}}

	// Client must have data for fallback to work
	client.SetUTXOs(targetAddr, utxos)

	opts := DefaultOptions()
	opts.GapLimit = 5
	opts.PathSchemes = []PathScheme{
		{Name: "FallbackTest", CoinType: CoinTypeBSV, Purpose: 44, Accounts: []uint32{0}, ScanChange: false},
	}

	scanner := NewScannerWithBulk(client, deriver, opts, bulkOps)

	seed := []byte("test-seed")
	result, err := scanner.Scan(context.Background(), seed)
	if err != nil {
		t.Fatalf("Scan failed: %v", err)
	}

	// Should still find funds via individual scanning fallback
	if result.TotalBalance != val {
		t.Errorf("TotalBalance = %d, want %d", result.TotalBalance, val)
	}
}
