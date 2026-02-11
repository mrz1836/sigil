package chain

import (
	"errors"
	"math/big"
	"testing"
)

var errInvalidAmount = errors.New("invalid amount")

func TestParseDecimalAmount_ValidAmounts(t *testing.T) {
	tests := []struct {
		name     string
		amount   string
		decimals int
		want     string
	}{
		{"1.5 with 18 decimals", "1.5", 18, "1500000000000000000"},
		{"0.1 with 8 decimals", "0.1", 8, "10000000"},
		{"100 no decimal", "100", 18, "100000000000000000000"},
		{".5 no integer", ".5", 18, "500000000000000000"},
		{"0 value", "0", 18, "0"},
		{"0.0 value", "0.0", 8, "0"},
		{"many decimals truncated", "1.123456789012345678901234", 18, "1123456789012345678"},
		{"fewer decimals padded", "1.1", 8, "110000000"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseDecimalAmount(tt.amount, tt.decimals, errInvalidAmount)
			if err != nil {
				t.Fatalf("ParseDecimalAmount() unexpected error = %v", err)
			}
			if got.String() != tt.want {
				t.Errorf("ParseDecimalAmount() = %s, want %s", got.String(), tt.want)
			}
		})
	}
}

func TestParseDecimalAmount_InvalidAmounts(t *testing.T) {
	invalidCases := []struct {
		name     string
		amount   string
		decimals int
	}{
		{"empty string", "", 18},
		{"negative", "-1", 18},
		{"multiple decimals", "1.2.3", 18},
		{"letters", "abc", 18},
		{"letters in decimal", "1.abc", 18},
		{"letters in integer", "abc.1", 18},
		{"spaces", " 1.5", 18},
	}

	for _, tt := range invalidCases {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ParseDecimalAmount(tt.amount, tt.decimals, errInvalidAmount)
			if err == nil {
				t.Error("ParseDecimalAmount() expected error, got nil")
			}
			if !errors.Is(err, errInvalidAmount) {
				t.Errorf("ParseDecimalAmount() error = %v, want %v", err, errInvalidAmount)
			}
		})
	}
}

func TestFormatDecimalAmount(t *testing.T) {
	tests := []struct {
		name     string
		amount   *big.Int
		decimals int
		want     string
	}{
		{"1.5 ETH", big.NewInt(0).SetUint64(1500000000000000000), 18, "1.5"},
		{"0.1 BTC", big.NewInt(10000000), 8, "0.1"},
		{"nil amount", nil, 18, "0"},
		{"zero", big.NewInt(0), 8, "0.0"},
		{"small value", big.NewInt(1), 18, "0.000000000000000001"},
		{"large value", mustBigInt("123456789012345678901234567890"), 18, "123456789012.34567890123456789"},
		{"no decimals", big.NewInt(100), 0, "100."},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FormatDecimalAmount(tt.amount, tt.decimals)
			if got != tt.want {
				t.Errorf("FormatDecimalAmount() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestFormatSignedDecimalAmount(t *testing.T) {
	tests := []struct {
		name     string
		amount   *big.Int
		decimals int
		want     string
	}{
		{"positive amount", big.NewInt(1500000000000000000), 18, "1.5"},
		{"negative amount", big.NewInt(-1500000000000000000), 18, "-1.5"},
		{"zero", big.NewInt(0), 8, "0.0"},
		{"nil", nil, 18, "0"},
		{"positive delegates to FormatDecimalAmount", big.NewInt(10000000), 8, "0.1"},
		{"negative with trailing zeros", big.NewInt(-10000000), 8, "-0.1"},
		{"small negative", big.NewInt(-1), 18, "-0.000000000000000001"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FormatSignedDecimalAmount(tt.amount, tt.decimals)
			if got != tt.want {
				t.Errorf("FormatSignedDecimalAmount() = %q, want %q", got, tt.want)
			}
		})
	}
}

func mustBigInt(s string) *big.Int {
	n, ok := new(big.Int).SetString(s, 10)
	if !ok {
		panic("invalid big int: " + s)
	}
	return n
}

func TestAmountToBigInt(t *testing.T) {
	tests := []struct {
		name   string
		amount uint64
		want   string
	}{
		{"zero", 0, "0"},
		{"small amount", 1, "1"},
		{"medium amount", 100, "100"},
		{"1 BTC in satoshis", 100000000, "100000000"},
		{"max uint64", ^uint64(0), "18446744073709551615"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := AmountToBigInt(tt.amount)
			if got.String() != tt.want {
				t.Errorf("AmountToBigInt(%d) = %s, want %s", tt.amount, got.String(), tt.want)
			}
		})
	}
}
