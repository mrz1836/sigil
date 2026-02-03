package discovery

import (
	"testing"
)

func TestDefaultSchemes(t *testing.T) {
	schemes := DefaultSchemes()

	if len(schemes) == 0 {
		t.Fatal("expected at least one default scheme")
	}

	// Verify expected schemes exist
	expectedNames := []string{
		"BSV Standard",
		"Bitcoin Legacy",
		"Bitcoin Cash",
		"HandCash Legacy",
		"Multi-Account BSV",
	}

	schemeNames := make(map[string]bool)
	for _, s := range schemes {
		schemeNames[s.Name] = true
	}

	for _, name := range expectedNames {
		if !schemeNames[name] {
			t.Errorf("expected scheme %q not found", name)
		}
	}
}

//nolint:gocognit // Table-driven test with multiple verifications per case
func TestPathSchemeProperties(t *testing.T) {
	tests := []struct {
		name           string
		schemeName     string
		wantCoinType   uint32
		wantPurpose    uint32
		wantScanChange bool
		wantIsLegacy   bool
		wantWallets    []string
	}{
		{
			name:           "BSV Standard",
			schemeName:     "BSV Standard",
			wantCoinType:   CoinTypeBSV,
			wantPurpose:    PurposeBIP44,
			wantScanChange: true,
			wantIsLegacy:   false,
			wantWallets:    []string{"RelayX", "RockWallet", "Twetch"},
		},
		{
			name:           "Bitcoin Legacy",
			schemeName:     "Bitcoin Legacy",
			wantCoinType:   CoinTypeBTC,
			wantPurpose:    PurposeBIP44,
			wantScanChange: true,
			wantIsLegacy:   false,
			wantWallets:    []string{"MoneyButton", "ElectrumSV"},
		},
		{
			name:           "Bitcoin Cash",
			schemeName:     "Bitcoin Cash",
			wantCoinType:   CoinTypeBCH,
			wantPurpose:    PurposeBIP44,
			wantScanChange: true,
			wantIsLegacy:   false,
			wantWallets:    []string{"Exodus", "Simply.Cash"},
		},
		{
			name:           "HandCash Legacy",
			schemeName:     "HandCash Legacy",
			wantCoinType:   CoinTypeBTC,
			wantPurpose:    PurposeLegacy,
			wantScanChange: false,
			wantIsLegacy:   true,
			wantWallets:    []string{"HandCash 1.x"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			scheme := SchemeByName(tt.schemeName)
			if scheme == nil {
				t.Fatalf("scheme %q not found", tt.schemeName)
			}

			if scheme.CoinType != tt.wantCoinType {
				t.Errorf("CoinType = %d, want %d", scheme.CoinType, tt.wantCoinType)
			}

			if scheme.Purpose != tt.wantPurpose {
				t.Errorf("Purpose = %d, want %d", scheme.Purpose, tt.wantPurpose)
			}

			if scheme.ScanChange != tt.wantScanChange {
				t.Errorf("ScanChange = %v, want %v", scheme.ScanChange, tt.wantScanChange)
			}

			if scheme.IsLegacy != tt.wantIsLegacy {
				t.Errorf("IsLegacy = %v, want %v", scheme.IsLegacy, tt.wantIsLegacy)
			}

			// Check some expected wallets are present
			walletSet := make(map[string]bool)
			for _, w := range scheme.Wallets {
				walletSet[w] = true
			}
			for _, expectedWallet := range tt.wantWallets {
				if !walletSet[expectedWallet] {
					t.Errorf("expected wallet %q not found in scheme", expectedWallet)
				}
			}
		})
	}
}

func TestSchemeByName(t *testing.T) {
	tests := []struct {
		name       string
		schemeName string
		wantNil    bool
	}{
		{
			name:       "existing scheme",
			schemeName: "BSV Standard",
			wantNil:    false,
		},
		{
			name:       "another existing scheme",
			schemeName: "Bitcoin Legacy",
			wantNil:    false,
		},
		{
			name:       "non-existent scheme",
			schemeName: "Nonexistent",
			wantNil:    true,
		},
		{
			name:       "empty name",
			schemeName: "",
			wantNil:    true,
		},
		{
			name:       "case sensitive",
			schemeName: "bsv standard",
			wantNil:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := SchemeByName(tt.schemeName)
			if (got == nil) != tt.wantNil {
				t.Errorf("SchemeByName(%q) = %v, wantNil = %v", tt.schemeName, got, tt.wantNil)
			}
			if got != nil && got.Name != tt.schemeName {
				t.Errorf("SchemeByName(%q).Name = %q", tt.schemeName, got.Name)
			}
		})
	}
}

func TestSchemesForWallet(t *testing.T) {
	tests := []struct {
		name       string
		walletName string
		wantCount  int
		wantScheme string
	}{
		{
			name:       "RelayX uses BSV Standard",
			walletName: "RelayX",
			wantCount:  1,
			wantScheme: "BSV Standard",
		},
		{
			name:       "MoneyButton uses Bitcoin Legacy",
			walletName: "MoneyButton",
			wantCount:  1,
			wantScheme: "Bitcoin Legacy",
		},
		{
			name:       "Exodus uses Bitcoin Cash",
			walletName: "Exodus",
			wantCount:  1,
			wantScheme: "Bitcoin Cash",
		},
		{
			name:       "HandCash 1.x uses HandCash Legacy",
			walletName: "HandCash 1.x",
			wantCount:  1,
			wantScheme: "HandCash Legacy",
		},
		{
			name:       "unknown wallet returns empty",
			walletName: "Unknown Wallet",
			wantCount:  0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := SchemesForWallet(tt.walletName)
			if len(got) != tt.wantCount {
				t.Errorf("SchemesForWallet(%q) returned %d schemes, want %d", tt.walletName, len(got), tt.wantCount)
			}
			if tt.wantCount > 0 && got[0].Name != tt.wantScheme {
				t.Errorf("SchemesForWallet(%q)[0].Name = %q, want %q", tt.walletName, got[0].Name, tt.wantScheme)
			}
		})
	}
}

func TestSortByPriority(t *testing.T) {
	// Create test schemes with various priorities
	schemes := []PathScheme{
		{Name: "Third", Priority: 3},
		{Name: "First", Priority: 1},
		{Name: "Fifth", Priority: 5},
		{Name: "Second", Priority: 2},
		{Name: "Fourth", Priority: 4},
	}

	sorted := SortByPriority(schemes)

	// Check that result is sorted by priority
	for i := 0; i < len(sorted)-1; i++ {
		if sorted[i].Priority > sorted[i+1].Priority {
			t.Errorf("not sorted: schemes[%d].Priority (%d) > schemes[%d].Priority (%d)",
				i, sorted[i].Priority, i+1, sorted[i+1].Priority)
		}
	}

	// Check expected order
	expectedOrder := []string{"First", "Second", "Third", "Fourth", "Fifth"}
	for i, name := range expectedOrder {
		if sorted[i].Name != name {
			t.Errorf("sorted[%d].Name = %q, want %q", i, sorted[i].Name, name)
		}
	}
}

func TestSortByPriority_DoesNotMutateInput(t *testing.T) {
	original := []PathScheme{
		{Name: "Third", Priority: 3},
		{Name: "First", Priority: 1},
		{Name: "Second", Priority: 2},
	}

	// Remember original order
	originalOrder := make([]string, len(original))
	for i, s := range original {
		originalOrder[i] = s.Name
	}

	_ = SortByPriority(original)

	// Verify original was not mutated
	for i, s := range original {
		if s.Name != originalOrder[i] {
			t.Errorf("original[%d].Name = %q, want %q (input was mutated)", i, s.Name, originalOrder[i])
		}
	}
}

func TestSortByPriority_EmptySlice(t *testing.T) {
	sorted := SortByPriority(nil)
	if sorted == nil {
		t.Error("SortByPriority(nil) should return empty slice, not nil")
	}
	if len(sorted) != 0 {
		t.Errorf("SortByPriority(nil) returned %d elements, want 0", len(sorted))
	}
}

func TestSortByPriority_SingleElement(t *testing.T) {
	schemes := []PathScheme{{Name: "Single", Priority: 1}}
	sorted := SortByPriority(schemes)

	if len(sorted) != 1 {
		t.Errorf("len(sorted) = %d, want 1", len(sorted))
	}
	if sorted[0].Name != "Single" {
		t.Errorf("sorted[0].Name = %q, want %q", sorted[0].Name, "Single")
	}
}

func TestCoinTypeConstants(t *testing.T) {
	// Verify coin type constants match SLIP-0044 specification
	tests := []struct {
		name     string
		coinType uint32
		want     uint32
	}{
		{"BTC", CoinTypeBTC, 0},
		{"BCH", CoinTypeBCH, 145},
		{"BSV", CoinTypeBSV, 236},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.coinType != tt.want {
				t.Errorf("CoinType%s = %d, want %d (per SLIP-0044)", tt.name, tt.coinType, tt.want)
			}
		})
	}
}

func TestPurposeConstants(t *testing.T) {
	if PurposeBIP44 != 44 {
		t.Errorf("PurposeBIP44 = %d, want 44", PurposeBIP44)
	}
	if PurposeLegacy != 0 {
		t.Errorf("PurposeLegacy = %d, want 0", PurposeLegacy)
	}
}

func TestDefaultSchemes_Priorities(t *testing.T) {
	schemes := DefaultSchemes()

	// BSV Standard should have highest priority (lowest number)
	bsvStandard := SchemeByName("BSV Standard")
	if bsvStandard == nil {
		t.Fatal("BSV Standard scheme not found")
	}

	for _, scheme := range schemes {
		if scheme.Name != "BSV Standard" && scheme.Priority <= bsvStandard.Priority {
			t.Errorf("scheme %q has priority %d, but BSV Standard should be highest with %d",
				scheme.Name, scheme.Priority, bsvStandard.Priority)
		}
	}
}

func TestDefaultSchemes_AllHaveAccounts(t *testing.T) {
	schemes := DefaultSchemes()

	for _, scheme := range schemes {
		if len(scheme.Accounts) == 0 {
			t.Errorf("scheme %q has no accounts configured", scheme.Name)
		}
	}
}

func TestDefaultSchemes_AllHaveWallets(t *testing.T) {
	schemes := DefaultSchemes()

	for _, scheme := range schemes {
		if len(scheme.Wallets) == 0 {
			t.Errorf("scheme %q has no wallets configured", scheme.Name)
		}
	}
}
