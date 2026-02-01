// Package chain provides blockchain interface definitions and common utilities.
package chain

import (
	"math/big"
	"strings"
)

// ParseDecimalAmount parses a decimal amount string to big.Int with the given decimal places.
// For example, "1.5" with 18 decimals returns 1500000000000000000.
//
//nolint:gocognit,gocyclo // Decimal parsing requires sequential validation steps
func ParseDecimalAmount(amount string, decimalPlaces int, invalidAmountErr error) (*big.Int, error) {
	if amount == "" {
		return nil, invalidAmountErr
	}

	// Check for negative amounts
	if strings.HasPrefix(amount, "-") {
		return nil, invalidAmountErr
	}

	// Split by decimal point
	parts := strings.Split(amount, ".")
	if len(parts) > 2 {
		return nil, invalidAmountErr
	}

	intPart := parts[0]
	decPart := ""
	if len(parts) == 2 {
		decPart = parts[1]
	}

	// Validate integer part
	if intPart == "" {
		intPart = "0"
	}
	intVal, ok := new(big.Int).SetString(intPart, 10)
	if !ok {
		return nil, invalidAmountErr
	}

	// Scale integer part
	multiplier := new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(decimalPlaces)), nil)
	result := new(big.Int).Mul(intVal, multiplier)

	// Handle decimal part
	if decPart != "" {
		// Validate decimal characters
		for _, c := range decPart {
			if c < '0' || c > '9' {
				return nil, invalidAmountErr
			}
		}

		// Pad or truncate decimal part
		for len(decPart) < decimalPlaces {
			decPart += "0"
		}
		decPart = decPart[:decimalPlaces]

		decVal, ok := new(big.Int).SetString(decPart, 10)
		if !ok {
			return nil, invalidAmountErr
		}

		result = result.Add(result, decVal)
	}

	return result, nil
}

// FormatDecimalAmount converts a big.Int to a human-readable string with the given decimal places.
// For example, 1500000000000000000 with 18 decimals returns "1.500000000000000000".
func FormatDecimalAmount(amount *big.Int, decimalPlaces int) string {
	if amount == nil {
		// Build zero string with proper decimal places
		zeros := strings.Repeat("0", decimalPlaces)
		return "0." + zeros
	}

	// Convert to string with all digits
	str := amount.String()

	// Pad with leading zeros if necessary
	for len(str) <= decimalPlaces {
		str = "0" + str
	}

	// Insert decimal point
	decimalPos := len(str) - decimalPlaces
	return str[:decimalPos] + "." + str[decimalPos:]
}
