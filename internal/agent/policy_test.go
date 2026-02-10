package agent

import (
	"math/big"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/mrz1836/sigil/internal/chain"
)

func TestValidateTransaction_ChainAuth(t *testing.T) {
	t.Parallel()

	cred := &Credential{
		Chains: []chain.ID{chain.BSV},
		Policy: Policy{},
	}

	// Allowed chain
	err := ValidateTransaction(cred, chain.BSV, "1ABC", big.NewInt(1000))
	if err != nil {
		t.Errorf("ValidateTransaction() error for allowed chain: %v", err)
	}

	// Denied chain
	err = ValidateTransaction(cred, chain.ETH, "0x123", big.NewInt(1000))
	if err == nil {
		t.Error("ValidateTransaction() expected error for unauthorized chain")
	}
}

func TestValidateTransaction_AddressAllowlist(t *testing.T) {
	t.Parallel()

	cred := &Credential{
		Chains: []chain.ID{chain.BSV},
		Policy: Policy{
			AllowedAddrs: []string{"1ALLOWED", "1ALSO_ALLOWED"},
		},
	}

	// Allowed address
	err := ValidateTransaction(cred, chain.BSV, "1ALLOWED", big.NewInt(1000))
	if err != nil {
		t.Errorf("ValidateTransaction() error for allowed address: %v", err)
	}

	// Denied address
	err = ValidateTransaction(cred, chain.BSV, "1DENIED", big.NewInt(1000))
	if err == nil {
		t.Error("ValidateTransaction() expected error for denied address")
	}
}

func TestValidateTransaction_EmptyAllowlist(t *testing.T) {
	t.Parallel()

	cred := &Credential{
		Chains: []chain.ID{chain.BSV},
		Policy: Policy{
			AllowedAddrs: nil,
		},
	}

	// Any address should be allowed when allowlist is empty
	err := ValidateTransaction(cred, chain.BSV, "1ANYTHING", big.NewInt(1000))
	if err != nil {
		t.Errorf("ValidateTransaction() error for empty allowlist: %v", err)
	}
}

func TestValidateTransaction_BSV_PerTxLimit(t *testing.T) {
	t.Parallel()

	cred := &Credential{
		Chains: []chain.ID{chain.BSV},
		Policy: Policy{
			MaxPerTxSat: 50000,
		},
	}

	// Under limit
	err := ValidateTransaction(cred, chain.BSV, "1ABC", big.NewInt(49999))
	if err != nil {
		t.Errorf("ValidateTransaction() error for amount under limit: %v", err)
	}

	// At limit
	err = ValidateTransaction(cred, chain.BSV, "1ABC", big.NewInt(50000))
	if err != nil {
		t.Errorf("ValidateTransaction() error for amount at limit: %v", err)
	}

	// Over limit
	err = ValidateTransaction(cred, chain.BSV, "1ABC", big.NewInt(50001))
	if err == nil {
		t.Error("ValidateTransaction() expected error for amount over limit")
	}
}

func TestValidateTransaction_BSV_NoLimit(t *testing.T) {
	t.Parallel()

	cred := &Credential{
		Chains: []chain.ID{chain.BSV},
		Policy: Policy{
			MaxPerTxSat: 0, // unlimited
		},
	}

	// Large amount should pass
	err := ValidateTransaction(cred, chain.BSV, "1ABC", big.NewInt(999999999))
	if err != nil {
		t.Errorf("ValidateTransaction() error for unlimited: %v", err)
	}
}

func TestValidateTransaction_ETH_PerTxLimit(t *testing.T) {
	t.Parallel()

	cred := &Credential{
		Chains: []chain.ID{chain.ETH},
		Policy: Policy{
			MaxPerTxWei: "1000000000000000", // 0.001 ETH
		},
	}

	// Under limit
	err := ValidateTransaction(cred, chain.ETH, "0xABC", big.NewInt(999999999999999))
	if err != nil {
		t.Errorf("ValidateTransaction() error for amount under ETH limit: %v", err)
	}

	// Over limit
	overLimit, _ := new(big.Int).SetString("1000000000000001", 10)
	err = ValidateTransaction(cred, chain.ETH, "0xABC", overLimit)
	if err == nil {
		t.Error("ValidateTransaction() expected error for ETH amount over limit")
	}
}

func TestCheckDailyLimit_BSV(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	counterPath := filepath.Join(dir, "test.counter")
	token := "test-token"

	cred := &Credential{
		Chains: []chain.ID{chain.BSV},
		Policy: Policy{
			MaxDailySat: 100000,
		},
	}

	// First transaction should pass
	err := CheckDailyLimit(counterPath, token, cred, chain.BSV, big.NewInt(50000))
	if err != nil {
		t.Fatalf("CheckDailyLimit() error for first tx: %v", err)
	}

	// Record the spend
	if recordErr := RecordSpend(counterPath, token, chain.BSV, big.NewInt(50000)); recordErr != nil {
		t.Fatalf("RecordSpend() error: %v", recordErr)
	}

	// Second transaction within limit should pass
	err = CheckDailyLimit(counterPath, token, cred, chain.BSV, big.NewInt(40000))
	if err != nil {
		t.Fatalf("CheckDailyLimit() error for second tx: %v", err)
	}

	// Transaction that exceeds daily limit should fail
	err = CheckDailyLimit(counterPath, token, cred, chain.BSV, big.NewInt(60000))
	if err == nil {
		t.Error("CheckDailyLimit() expected error when exceeding daily limit")
	}
}

func TestCheckDailyLimit_ETH(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	counterPath := filepath.Join(dir, "eth.counter")
	token := "eth-test-token" //nolint:gosec // G101: Test token

	cred := &Credential{
		Chains: []chain.ID{chain.ETH},
		Policy: Policy{
			MaxDailyWei: "10000000000000000", // 0.01 ETH
		},
	}

	amount, _ := new(big.Int).SetString("5000000000000000", 10) // 0.005 ETH

	// First transaction should pass
	err := CheckDailyLimit(counterPath, token, cred, chain.ETH, amount)
	if err != nil {
		t.Fatalf("CheckDailyLimit() ETH error for first tx: %v", err)
	}

	// Record spend
	if recordErr := RecordSpend(counterPath, token, chain.ETH, amount); recordErr != nil {
		t.Fatalf("RecordSpend() ETH error: %v", recordErr)
	}

	// Exceeding limit should fail
	overLimit, _ := new(big.Int).SetString("6000000000000000", 10)
	err = CheckDailyLimit(counterPath, token, cred, chain.ETH, overLimit)
	if err == nil {
		t.Error("CheckDailyLimit() ETH expected error when exceeding daily limit")
	}
}

func TestCheckDailyLimit_Unlimited(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	counterPath := filepath.Join(dir, "unlimited.counter")
	token := "unlimited-token"

	cred := &Credential{
		Chains: []chain.ID{chain.BSV},
		Policy: Policy{
			MaxDailySat: 0, // unlimited
		},
	}

	err := CheckDailyLimit(counterPath, token, cred, chain.BSV, big.NewInt(999999999))
	if err != nil {
		t.Errorf("CheckDailyLimit() error for unlimited: %v", err)
	}
}

func TestCheckDailyLimit_EmptyCounterPath(t *testing.T) {
	t.Parallel()

	cred := &Credential{
		Chains: []chain.ID{chain.BSV},
		Policy: Policy{
			MaxDailySat: 100000,
		},
	}

	// Empty counter path should use fresh counter
	err := CheckDailyLimit("", "token", cred, chain.BSV, big.NewInt(50000))
	if err != nil {
		t.Errorf("CheckDailyLimit() error with empty counter path: %v", err)
	}
}

func TestRecordSpend_BSV(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	counterPath := filepath.Join(dir, "spend.counter")
	token := "spend-token"

	// Record first spend
	if err := RecordSpend(counterPath, token, chain.BSV, big.NewInt(10000)); err != nil {
		t.Fatalf("RecordSpend() error: %v", err)
	}

	// Verify counter was written
	if _, err := os.Stat(counterPath); os.IsNotExist(err) {
		t.Error("counter file was not created")
	}

	// Record second spend
	if err := RecordSpend(counterPath, token, chain.BSV, big.NewInt(20000)); err != nil {
		t.Fatalf("RecordSpend() second error: %v", err)
	}

	// Verify accumulated spend
	satSpent, _ := GetDailySpent(counterPath, token)
	if satSpent != 30000 {
		t.Errorf("GetDailySpent() sat = %d, want 30000", satSpent)
	}
}

func TestRecordSpend_ETH(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	counterPath := filepath.Join(dir, "eth-spend.counter")
	token := "eth-spend-token" //nolint:gosec // G101: Test token

	amount1, _ := new(big.Int).SetString("1000000000000000", 10)
	amount2, _ := new(big.Int).SetString("2000000000000000", 10)

	if err := RecordSpend(counterPath, token, chain.ETH, amount1); err != nil {
		t.Fatalf("RecordSpend() ETH error: %v", err)
	}
	if err := RecordSpend(counterPath, token, chain.ETH, amount2); err != nil {
		t.Fatalf("RecordSpend() ETH second error: %v", err)
	}

	_, weiSpent := GetDailySpent(counterPath, token)
	expected := "3000000000000000"
	if weiSpent != expected {
		t.Errorf("GetDailySpent() wei = %q, want %q", weiSpent, expected)
	}
}

func TestRecordSpend_EmptyPath(t *testing.T) {
	t.Parallel()

	// Should not error
	err := RecordSpend("", "token", chain.BSV, big.NewInt(1000))
	if err != nil {
		t.Errorf("RecordSpend() error with empty path: %v", err)
	}
}

func TestGetDailySpent_NoCounter(t *testing.T) {
	t.Parallel()

	sat, wei := GetDailySpent("/nonexistent/path", "token")
	if sat != 0 {
		t.Errorf("GetDailySpent() sat = %d, want 0", sat)
	}
	if wei != "" {
		t.Errorf("GetDailySpent() wei = %q, want empty", wei)
	}
}

func TestLoadCounter_DailyReset(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	counterPath := filepath.Join(dir, "reset.counter")
	token := "reset-token"

	// Record a spend
	if err := RecordSpend(counterPath, token, chain.BSV, big.NewInt(50000)); err != nil {
		t.Fatalf("RecordSpend() error: %v", err)
	}

	// Verify spend was recorded
	sat, _ := GetDailySpent(counterPath, token)
	if sat != 50000 {
		t.Fatalf("GetDailySpent() sat = %d, want 50000", sat)
	}

	// The counter uses todayDate(), so we can't easily test date rollover
	// without mocking time. Instead, verify the counter loads with today's date.
	counter := loadCounter(counterPath, token)
	if counter.Date != todayDate() {
		t.Errorf("counter.Date = %q, want %q", counter.Date, todayDate())
	}
}

func TestCounterHMAC_TamperDetection(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	counterPath := filepath.Join(dir, "tamper.counter")
	token := "tamper-token"

	// Record a spend
	if err := RecordSpend(counterPath, token, chain.BSV, big.NewInt(10000)); err != nil {
		t.Fatalf("RecordSpend() error: %v", err)
	}

	// Tamper with the counter file - change spent amount
	tampered := []byte(`{"date":"` + todayDate() + `","spent_sat":999999,"spent_wei":"","hmac":"fake"}`)
	_ = os.WriteFile(counterPath, tampered, 0o600)

	// Loading with wrong HMAC should max out the counter to block further spending
	counter := loadCounter(counterPath, token)
	if counter.SpentSat != ^uint64(0) {
		t.Errorf("tampered counter should be maxed out (deny spending), got %d", counter.SpentSat)
	}
}

func TestTodayDate(t *testing.T) {
	t.Parallel()

	date := todayDate()
	if len(date) != 10 {
		t.Errorf("todayDate() = %q, expected YYYY-MM-DD format", date)
	}

	// Should match Go's UTC date
	expected := time.Now().UTC().Format("2006-01-02")
	if date != expected {
		t.Errorf("todayDate() = %q, want %q", date, expected)
	}
}

func TestDailyCounter_SpentWeiBig(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		spentWei string
		want     int64
	}{
		{"empty", "", 0},
		{"zero", "0", 0},
		{"valid", "1000000000000000", 1000000000000000},
		{"invalid", "not-a-number", 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dc := &DailyCounter{SpentWei: tt.spentWei}
			got := dc.spentWeiBig()
			if got.Int64() != tt.want {
				t.Errorf("spentWeiBig() = %v, want %v", got, tt.want)
			}
		})
	}
}
