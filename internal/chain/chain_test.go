package chain

import (
	"errors"
	"regexp"
	"testing"

	sigilerr "github.com/mrz1836/sigil/pkg/errors"
)

func TestID_DerivationPath(t *testing.T) {
	tests := []struct {
		name string
		id   ID
		want string
	}{
		{"ETH", ETH, "m/44'/60'/0'"},
		{"BSV", BSV, "m/44'/236'/0'"},
		{"BTC", BTC, "m/44'/0'/0'"},
		{"BCH", BCH, "m/44'/145'/0'"},
		{"unknown", ID("unknown"), ""},
		{"empty", ID(""), ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.id.DerivationPath(); got != tt.want {
				t.Errorf("ID.DerivationPath() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestID_CoinType(t *testing.T) {
	tests := []struct {
		name string
		id   ID
		want uint32
	}{
		{"ETH", ETH, 60},
		{"BSV", BSV, 236},
		{"BTC", BTC, 0},
		{"BCH", BCH, 145},
		{"unknown", ID("unknown"), 0},
		{"empty", ID(""), 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.id.CoinType(); got != tt.want {
				t.Errorf("ID.CoinType() = %d, want %d", got, tt.want)
			}
		})
	}
}

func TestID_String(t *testing.T) {
	tests := []struct {
		name string
		id   ID
		want string
	}{
		{"ETH", ETH, "eth"},
		{"BSV", BSV, "bsv"},
		{"BTC", BTC, "btc"},
		{"BCH", BCH, "bch"},
		{"custom", ID("custom"), "custom"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.id.String(); got != tt.want {
				t.Errorf("ID.String() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestID_IsValid(t *testing.T) {
	tests := []struct {
		name string
		id   ID
		want bool
	}{
		{"ETH", ETH, true},
		{"BSV", BSV, true},
		{"BTC", BTC, true},
		{"BCH", BCH, true},
		{"unknown", ID("foo"), false},
		{"empty", ID(""), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.id.IsValid(); got != tt.want {
				t.Errorf("ID.IsValid() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestID_IsMVP(t *testing.T) {
	tests := []struct {
		name string
		id   ID
		want bool
	}{
		{"ETH", ETH, true},
		{"BSV", BSV, true},
		{"BTC", BTC, false},
		{"BCH", BCH, false},
		{"unknown", ID("unknown"), false},
		{"empty", ID(""), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.id.IsMVP(); got != tt.want {
				t.Errorf("ID.IsMVP() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestParseChainID(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		wantID ID
		wantOK bool
	}{
		{"eth", "eth", ETH, true},
		{"bsv", "bsv", BSV, true},
		{"btc", "btc", BTC, true},
		{"bch", "bch", BCH, true},
		{"invalid", "foo", ID("foo"), false},
		{"empty", "", ID(""), false},
		{"uppercase", "ETH", ID("ETH"), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotID, gotOK := ParseChainID(tt.input)
			if gotID != tt.wantID {
				t.Errorf("ParseChainID() id = %q, want %q", gotID, tt.wantID)
			}
			if gotOK != tt.wantOK {
				t.Errorf("ParseChainID() ok = %v, want %v", gotOK, tt.wantOK)
			}
		})
	}
}

func TestSupportedChains(t *testing.T) {
	chains := SupportedChains()

	if len(chains) != 2 {
		t.Errorf("SupportedChains() returned %d chains, want 2", len(chains))
	}

	expected := map[ID]bool{ETH: true, BSV: true}
	for _, c := range chains {
		if !expected[c] {
			t.Errorf("SupportedChains() contains unexpected chain %q", c)
		}
	}
}

func TestAllChains(t *testing.T) {
	chains := AllChains()

	if len(chains) != 4 {
		t.Errorf("AllChains() returned %d chains, want 4", len(chains))
	}

	expected := map[ID]bool{ETH: true, BSV: true, BTC: true, BCH: true}
	for _, c := range chains {
		if !expected[c] {
			t.Errorf("AllChains() contains unexpected chain %q", c)
		}
	}
}

// assertIsSigilError is a test helper that validates an error is properly structured as a SigilError.
// Use this in tests to ensure user-facing errors follow the documented conventions.
//
// Example usage:
//
//	err := client.ValidateAddress("")
//	assertIsSigilError(t, err, "address validation should return SigilError")
func assertIsSigilError(t *testing.T, err error, context string) {
	t.Helper()
	if err == nil {
		t.Errorf("%s: expected SigilError, got nil", context)
		return
	}

	var sigilErr *sigilerr.SigilError
	if !errors.As(err, &sigilErr) {
		t.Errorf("%s: error is not a SigilError: %v (type: %T)", context, err, err)
	}
}

// TestValidateAddressWithRegex_ErrorTypes verifies that validation errors are SigilErrors.
func TestValidateAddressWithRegex_ErrorTypes(t *testing.T) {
	testErr := &sigilerr.SigilError{
		Code:    "TEST_ERROR",
		Message: "test validation failed",
	}

	testRegex := regexp.MustCompile(`^valid$`)

	// Empty address should return error
	result := ValidateAddressWithRegex("", testRegex, testErr)
	assertIsSigilError(t, result, "empty address validation")

	// Invalid format should return error
	result = ValidateAddressWithRegex("invalid", testRegex, testErr)
	assertIsSigilError(t, result, "invalid format validation")

	// Valid address should return nil
	result = ValidateAddressWithRegex("valid", testRegex, testErr)
	if result != nil {
		t.Errorf("ValidateAddressWithRegex('valid') should return nil, got %v", result)
	}
}
