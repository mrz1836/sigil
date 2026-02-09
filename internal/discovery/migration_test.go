package discovery

import (
	"context"
	"errors"
	"testing"
)

func TestCreateMigrationPlan_Success(t *testing.T) {
	result := &Result{
		TotalBalance: 100000,
		FoundAddresses: map[string][]DiscoveredAddress{
			"BSV Standard": {
				{Address: "addr1", Balance: 50000, UTXOCount: 2},
				{Address: "addr2", Balance: 50000, UTXOCount: 1},
			},
		},
	}

	plan, err := CreateMigrationPlan(result, "dest_address", DefaultFeeRate)
	if err != nil {
		t.Fatalf("CreateMigrationPlan failed: %v", err)
	}

	if plan.Destination != "dest_address" {
		t.Errorf("Destination = %s, want %s", plan.Destination, "dest_address")
	}

	if plan.TotalInput != 100000 {
		t.Errorf("TotalInput = %d, want %d", plan.TotalInput, 100000)
	}

	if plan.FeeRate != DefaultFeeRate {
		t.Errorf("FeeRate = %d, want %d", plan.FeeRate, DefaultFeeRate)
	}

	if plan.TotalUTXOs != 3 {
		t.Errorf("TotalUTXOs = %d, want %d", plan.TotalUTXOs, 3)
	}

	// Fee should be calculated based on tx size
	// Size = overhead(10) + inputs(3 * 148) + output(34) = 10 + 444 + 34 = 488
	expectedSize := OverheadSize + (3 * InputSize) + OutputSize
	if plan.EstimatedSize != expectedSize {
		t.Errorf("EstimatedSize = %d, want %d", plan.EstimatedSize, expectedSize)
	}

	expectedFee := (expectedSize*DefaultFeeRate + 999) / 1000
	if plan.EstimatedFee != expectedFee {
		t.Errorf("EstimatedFee = %d, want %d", plan.EstimatedFee, expectedFee)
	}

	expectedNet := plan.TotalInput - plan.EstimatedFee
	if plan.NetAmount != expectedNet {
		t.Errorf("NetAmount = %d, want %d", plan.NetAmount, expectedNet)
	}
}

func TestCreateMigrationPlan_CustomFeeRate(t *testing.T) {
	result := &Result{
		TotalBalance: 100000,
		FoundAddresses: map[string][]DiscoveredAddress{
			"Test": {{Address: "addr1", Balance: 100000, UTXOCount: 1}},
		},
	}

	customFeeRate := uint64(5000)
	plan, err := CreateMigrationPlan(result, "dest", customFeeRate)
	if err != nil {
		t.Fatalf("CreateMigrationPlan failed: %v", err)
	}

	if plan.FeeRate != customFeeRate {
		t.Errorf("FeeRate = %d, want %d", plan.FeeRate, customFeeRate)
	}

	expectedSize := OverheadSize + InputSize + OutputSize
	expectedFee := (expectedSize*customFeeRate + 999) / 1000
	if plan.EstimatedFee != expectedFee {
		t.Errorf("EstimatedFee = %d, want %d", plan.EstimatedFee, expectedFee)
	}
}

func TestCreateMigrationPlan_DefaultFeeRate(t *testing.T) {
	result := &Result{
		TotalBalance: 100000,
		FoundAddresses: map[string][]DiscoveredAddress{
			"Test": {{Address: "addr1", Balance: 100000, UTXOCount: 1}},
		},
	}

	// Pass 0 fee rate to use default
	plan, err := CreateMigrationPlan(result, "dest", 0)
	if err != nil {
		t.Fatalf("CreateMigrationPlan failed: %v", err)
	}

	if plan.FeeRate != DefaultFeeRate {
		t.Errorf("FeeRate = %d, want default %d", plan.FeeRate, DefaultFeeRate)
	}
}

func TestCreateMigrationPlan_NoFunds(t *testing.T) {
	// Nil result
	_, err := CreateMigrationPlan(nil, "dest", DefaultFeeRate)
	if err == nil {
		t.Error("expected error for nil result")
	}
	if !errors.Is(err, ErrNoAddressesToMigrate) {
		t.Errorf("error = %v, want %v", err, ErrNoAddressesToMigrate)
	}

	// Result with no funds
	result := &Result{
		TotalBalance:   0,
		FoundAddresses: map[string][]DiscoveredAddress{},
	}
	_, err = CreateMigrationPlan(result, "dest", DefaultFeeRate)
	if err == nil {
		t.Error("expected error for empty result")
	}
}

func TestCreateMigrationPlan_NoDestination(t *testing.T) {
	result := &Result{
		TotalBalance: 100000,
		FoundAddresses: map[string][]DiscoveredAddress{
			"Test": {{Address: "addr1", Balance: 100000, UTXOCount: 1}},
		},
	}

	_, err := CreateMigrationPlan(result, "", DefaultFeeRate)
	if err == nil {
		t.Error("expected error for empty destination")
	}
}

func TestCreateMigrationPlan_DustAmount(t *testing.T) {
	// Very small balance that would be consumed by fees
	// At 50 sat/KB, fee for 1 UTXO = (192*50+999)/1000 = 10 sats
	result := &Result{
		TotalBalance: 5, // Only 5 satoshis, fee is 10
		FoundAddresses: map[string][]DiscoveredAddress{
			"Test": {{Address: "addr1", Balance: 5, UTXOCount: 1}},
		},
	}

	_, err := CreateMigrationPlan(result, "dest", DefaultFeeRate)
	if err == nil {
		t.Error("expected error for dust amount")
	}
	if !errors.Is(err, ErrDustAmount) {
		t.Errorf("error = %v, want %v", err, ErrDustAmount)
	}
}

func TestCreateMigrationPlan_DustWarning(t *testing.T) {
	// Balance where fee is more than 10% of total
	// Fee for 1 UTXO ~192 bytes at 50 sat/KB = 10 sats
	// 10% warning threshold means warning if total < ~100 sats
	result := &Result{
		TotalBalance: 50, // 50 satoshis, fee ~10 = 20% > 10%
		FoundAddresses: map[string][]DiscoveredAddress{
			"Test": {{Address: "addr1", Balance: 50, UTXOCount: 1}},
		},
	}

	plan, err := CreateMigrationPlan(result, "dest", DefaultFeeRate)
	if err != nil {
		t.Fatalf("CreateMigrationPlan failed: %v", err)
	}

	if plan.Warning == "" {
		t.Error("expected warning for high fee ratio")
	}
}

func TestCreateMigrationPlan_NoWarningForLargeAmounts(t *testing.T) {
	// Large balance where fee is negligible
	result := &Result{
		TotalBalance: 100000000, // 1 BSV
		FoundAddresses: map[string][]DiscoveredAddress{
			"Test": {{Address: "addr1", Balance: 100000000, UTXOCount: 1}},
		},
	}

	plan, err := CreateMigrationPlan(result, "dest", DefaultFeeRate)
	if err != nil {
		t.Fatalf("CreateMigrationPlan failed: %v", err)
	}

	if plan.Warning != "" {
		t.Errorf("unexpected warning: %s", plan.Warning)
	}
}

func TestCreateMigrationPlan_MultipleSchemes(t *testing.T) {
	result := &Result{
		TotalBalance: 300000,
		FoundAddresses: map[string][]DiscoveredAddress{
			"BSV Standard": {
				{Address: "addr1", Balance: 100000, UTXOCount: 1},
			},
			"Bitcoin Legacy": {
				{Address: "addr2", Balance: 200000, UTXOCount: 2},
			},
		},
	}

	plan, err := CreateMigrationPlan(result, "dest", DefaultFeeRate)
	if err != nil {
		t.Fatalf("CreateMigrationPlan failed: %v", err)
	}

	if len(plan.Sources) != 2 {
		t.Errorf("Sources = %d, want 2", len(plan.Sources))
	}

	if plan.TotalInput != 300000 {
		t.Errorf("TotalInput = %d, want 300000", plan.TotalInput)
	}

	if plan.TotalUTXOs != 3 {
		t.Errorf("TotalUTXOs = %d, want 3", plan.TotalUTXOs)
	}
}

func TestMigrationSource(t *testing.T) {
	source := MigrationSource{
		Address:   "test_addr",
		Path:      "m/44'/236'/0'/0/0",
		Balance:   50000,
		UTXOCount: 2,
	}

	if source.Address != "test_addr" {
		t.Errorf("Address = %s", source.Address)
	}
	if source.Path != "m/44'/236'/0'/0/0" {
		t.Errorf("Path = %s", source.Path)
	}
	if source.Balance != 50000 {
		t.Errorf("Balance = %d", source.Balance)
	}
	if source.UTXOCount != 2 {
		t.Errorf("UTXOCount = %d", source.UTXOCount)
	}
}

// mockTransactionBuilder implements TransactionBuilder for testing.
type mockTransactionBuilder struct {
	buildErr     error
	signErr      error
	broadcastErr error
	txID         string
}

func (m *mockTransactionBuilder) BuildConsolidationTx(_ context.Context, _ []TxInput, _ string, _ uint64) ([]byte, error) {
	if m.buildErr != nil {
		return nil, m.buildErr
	}
	return []byte("mock_tx"), nil
}

func (m *mockTransactionBuilder) SignInput(tx []byte, _ int, _ []byte, _ uint32) ([]byte, error) {
	if m.signErr != nil {
		return nil, m.signErr
	}
	return tx, nil
}

func (m *mockTransactionBuilder) BroadcastTx(_ context.Context, _ []byte) (string, error) {
	if m.broadcastErr != nil {
		return "", m.broadcastErr
	}
	if m.txID == "" {
		return "mock_txid", nil
	}
	return m.txID, nil
}

func TestNewMigrator(t *testing.T) {
	client := newMockChainClient()
	builder := &mockTransactionBuilder{}
	deriver := newMockKeyDeriver()

	migrator := NewMigrator(client, builder, deriver)
	if migrator == nil {
		t.Fatal("NewMigrator returned nil")
	}
}

func TestMigrator_Execute_Success(t *testing.T) {
	client := newMockChainClient()
	client.SetUTXOs("addr1", []UTXO{
		{TxID: "tx1", Vout: 0, Amount: 50000, Address: "addr1"},
	})

	builder := &mockTransactionBuilder{txID: "success_txid"}
	deriver := newMockKeyDeriver()

	migrator := NewMigrator(client, builder, deriver)

	plan := &MigrationPlan{
		Sources: []MigrationSource{
			{Address: "addr1", Balance: 50000, UTXOCount: 1},
		},
		Destination:  "dest_addr",
		TotalInput:   50000,
		EstimatedFee: 200,
		NetAmount:    49800,
	}

	seed := []byte("test-seed-32-bytes-long-enough!")
	result, err := migrator.Execute(context.Background(), seed, plan)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	if result.TxID != "success_txid" {
		t.Errorf("TxID = %s, want success_txid", result.TxID)
	}

	if result.TotalMigrated != plan.NetAmount {
		t.Errorf("TotalMigrated = %d, want %d", result.TotalMigrated, plan.NetAmount)
	}

	if result.Destination != "dest_addr" {
		t.Errorf("Destination = %s", result.Destination)
	}
}

func TestMigrator_Execute_NilPlan(t *testing.T) {
	client := newMockChainClient()
	builder := &mockTransactionBuilder{}
	deriver := newMockKeyDeriver()

	migrator := NewMigrator(client, builder, deriver)

	seed := []byte("test-seed-32-bytes-long-enough!")
	_, err := migrator.Execute(context.Background(), seed, nil)

	if err == nil {
		t.Error("expected error for nil plan")
	}
}

func TestMigrator_Execute_EmptySeed(t *testing.T) {
	client := newMockChainClient()
	builder := &mockTransactionBuilder{}
	deriver := newMockKeyDeriver()

	migrator := NewMigrator(client, builder, deriver)

	plan := &MigrationPlan{
		Sources: []MigrationSource{{Address: "addr1"}},
	}

	_, err := migrator.Execute(context.Background(), nil, plan)
	if err == nil {
		t.Error("expected error for nil seed")
	}

	_, err = migrator.Execute(context.Background(), []byte{}, plan)
	if err == nil {
		t.Error("expected error for empty seed")
	}
}

func TestMigrator_Execute_NoUTXOs(t *testing.T) {
	client := newMockChainClient()
	// Don't set any UTXOs

	builder := &mockTransactionBuilder{}
	deriver := newMockKeyDeriver()

	migrator := NewMigrator(client, builder, deriver)

	plan := &MigrationPlan{
		Sources: []MigrationSource{
			{Address: "addr1", Balance: 50000, UTXOCount: 1},
		},
		Destination: "dest",
		NetAmount:   49800,
	}

	seed := []byte("test-seed-32-bytes-long-enough!")
	_, err := migrator.Execute(context.Background(), seed, plan)

	if err == nil {
		t.Error("expected error when no UTXOs found")
	}
}

func TestMigrator_Execute_BuildError(t *testing.T) {
	client := newMockChainClient()
	client.SetUTXOs("addr1", []UTXO{
		{TxID: "tx1", Vout: 0, Amount: 50000, Address: "addr1"},
	})

	builder := &mockTransactionBuilder{buildErr: errors.New("build failed")} //nolint:err113 // Test mock error
	deriver := newMockKeyDeriver()

	migrator := NewMigrator(client, builder, deriver)

	plan := &MigrationPlan{
		Sources:     []MigrationSource{{Address: "addr1"}},
		Destination: "dest",
		NetAmount:   49800,
	}

	seed := []byte("test-seed-32-bytes-long-enough!")
	_, err := migrator.Execute(context.Background(), seed, plan)

	if err == nil {
		t.Error("expected error when build fails")
	}
}

func TestMigrator_Execute_BroadcastError(t *testing.T) {
	client := newMockChainClient()
	client.SetUTXOs("addr1", []UTXO{
		{TxID: "tx1", Vout: 0, Amount: 50000, Address: "addr1"},
	})

	builder := &mockTransactionBuilder{broadcastErr: errors.New("broadcast failed")} //nolint:err113 // Test mock error
	deriver := newMockKeyDeriver()

	migrator := NewMigrator(client, builder, deriver)

	plan := &MigrationPlan{
		Sources:     []MigrationSource{{Address: "addr1"}},
		Destination: "dest",
		NetAmount:   49800,
	}

	seed := []byte("test-seed-32-bytes-long-enough!")
	_, err := migrator.Execute(context.Background(), seed, plan)

	if err == nil {
		t.Error("expected error when broadcast fails")
	}
}

func TestMigrator_ValidatePlan_Success(t *testing.T) {
	client := newMockChainClient()
	client.SetUTXOs("addr1", []UTXO{
		{TxID: "tx1", Vout: 0, Amount: 50000, Address: "addr1"},
	})

	builder := &mockTransactionBuilder{}
	deriver := newMockKeyDeriver()

	migrator := NewMigrator(client, builder, deriver)

	plan := &MigrationPlan{
		Sources: []MigrationSource{
			{Address: "addr1", Balance: 50000},
		},
		Destination: "dest",
		TotalInput:  50000,
	}

	err := migrator.ValidatePlan(context.Background(), plan)
	if err != nil {
		t.Errorf("ValidatePlan failed: %v", err)
	}
}

func TestMigrator_ValidatePlan_NilPlan(t *testing.T) {
	client := newMockChainClient()
	builder := &mockTransactionBuilder{}
	deriver := newMockKeyDeriver()

	migrator := NewMigrator(client, builder, deriver)

	err := migrator.ValidatePlan(context.Background(), nil)
	if err == nil {
		t.Error("expected error for nil plan")
	}
}

func TestMigrator_ValidatePlan_InvalidDestination(t *testing.T) {
	client := newMockChainClient()
	builder := &mockTransactionBuilder{}
	deriver := newMockKeyDeriver()

	migrator := NewMigrator(client, builder, deriver)

	plan := &MigrationPlan{
		Sources:     []MigrationSource{{Address: "addr1", Balance: 50000}},
		Destination: "", // Invalid - empty
		TotalInput:  50000,
	}

	err := migrator.ValidatePlan(context.Background(), plan)
	if err == nil {
		t.Error("expected error for invalid destination")
	}
}

func TestMigrator_ValidatePlan_BalanceChanged(t *testing.T) {
	client := newMockChainClient()
	// Set different balance than expected
	client.SetUTXOs("addr1", []UTXO{
		{TxID: "tx1", Vout: 0, Amount: 30000, Address: "addr1"}, // 30000 instead of 50000
	})

	builder := &mockTransactionBuilder{}
	deriver := newMockKeyDeriver()

	migrator := NewMigrator(client, builder, deriver)

	plan := &MigrationPlan{
		Sources: []MigrationSource{
			{Address: "addr1", Balance: 50000}, // Expected 50000
		},
		Destination: "dest",
		TotalInput:  50000,
	}

	err := migrator.ValidatePlan(context.Background(), plan)
	if err == nil {
		t.Error("expected error when balance changed")
	}
}

func TestFeeCalculationConstants(t *testing.T) {
	// Verify constants are reasonable
	if InputSize < 100 || InputSize > 200 {
		t.Errorf("InputSize = %d, expected ~148 bytes for P2PKH", InputSize)
	}

	if OutputSize < 30 || OutputSize > 50 {
		t.Errorf("OutputSize = %d, expected ~34 bytes for P2PKH", OutputSize)
	}

	if OverheadSize < 5 || OverheadSize > 20 {
		t.Errorf("OverheadSize = %d, expected ~10 bytes", OverheadSize)
	}

	if DefaultFeeRate < 50 {
		t.Errorf("DefaultFeeRate = %d, expected >= 50 sat/KB", DefaultFeeRate)
	}
}

func TestTxInput(t *testing.T) {
	input := TxInput{
		TxID:         "txid123",
		Vout:         1,
		Amount:       50000,
		ScriptPubKey: "76a914...",
		Address:      "1ABC...",
		PrivateKey:   []byte{1, 2, 3},
	}

	if input.TxID != "txid123" {
		t.Errorf("TxID = %s", input.TxID)
	}
	if input.Vout != 1 {
		t.Errorf("Vout = %d", input.Vout)
	}
	if input.Amount != 50000 {
		t.Errorf("Amount = %d", input.Amount)
	}
}

func TestMigrationResult(t *testing.T) {
	result := MigrationResult{
		TxID:            "txid",
		TotalMigrated:   49800,
		Fee:             200,
		InputCount:      3,
		SourceAddresses: []string{"addr1", "addr2"},
		Destination:     "dest",
	}

	if result.TxID != "txid" {
		t.Errorf("TxID = %s", result.TxID)
	}
	if result.TotalMigrated != 49800 {
		t.Errorf("TotalMigrated = %d", result.TotalMigrated)
	}
	if result.Fee != 200 {
		t.Errorf("Fee = %d", result.Fee)
	}
	if result.InputCount != 3 {
		t.Errorf("InputCount = %d", result.InputCount)
	}
	if len(result.SourceAddresses) != 2 {
		t.Errorf("SourceAddresses = %v", result.SourceAddresses)
	}
}

// BenchmarkCreateMigrationPlan benchmarks plan creation.
func BenchmarkCreateMigrationPlan(b *testing.B) {
	result := &Result{
		TotalBalance: 1000000,
		FoundAddresses: map[string][]DiscoveredAddress{
			"Test": {
				{Address: "addr1", Balance: 500000, UTXOCount: 5},
				{Address: "addr2", Balance: 500000, UTXOCount: 5},
			},
		},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = CreateMigrationPlan(result, "dest", DefaultFeeRate)
	}
}
