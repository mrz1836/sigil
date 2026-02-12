// Package shamir implements Shamir's Secret Sharing over GF(2^8).
// It allows splitting a secret into multiple shares and reconstructing it
// from a threshold number of shares.
package shamir

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"strconv"
	"strings"
)

// Share represents a single share of a split secret.
type Share struct {
	Index int    // The x-coordinate (1-based index)
	Value []byte // The y-coordinate (evaluated polynomial value)
}

// Split divides a secret into n shares, requiring k shares to reconstruct.
// The secret can be any byte slice.
// n: total number of shares to generate.
// k: threshold number of shares required to reconstruct.
// Returns a list of shares encoded as strings.
func Split(secret []byte, n, k int) ([]string, error) {
	if k < 2 {
		return nil, ErrThresholdInvalid
	}
	if n < k {
		return nil, ErrSharesInsufficient
	}
	if n > 255 {
		return nil, ErrSharesExceedMax
	}
	if len(secret) == 0 {
		return nil, ErrSecretEmpty
	}

	// We generate a separate random polynomial for each byte of the secret.
	// For a secret of length L, we have L polynomials.
	// Each polynomial is of degree k-1.
	// f_i(x) = secret[i] + a_1*x + a_2*x^2 + ... + a_(k-1)*x^(k-1)

	// Generate random coefficients for all polynomials.
	// We need (k-1) coefficients for each of the len(secret) bytes.
	// Total random bytes = len(secret) * (k - 1)
	coeffs, err := generateCoefficients(len(secret), k)
	if err != nil {
		return nil, err
	}

	// Evaluate polynomials for each share x = 1 to n
	return evaluatePolynomials(secret, coeffs, n, k)
}

func generateCoefficients(secretLen, k int) ([]byte, error) {
	numCoeffs := secretLen * (k - 1)
	coeffs := make([]byte, numCoeffs)
	if _, err := rand.Read(coeffs); err != nil {
		return nil, fmt.Errorf("failed to generate random coefficients: %w", err)
	}
	return coeffs, nil
}

func evaluatePolynomials(secret, coeffs []byte, n, k int) ([]string, error) {
	shares := make([]string, n)

	// For each share (x coordinate from 1 to n)
	for x := 1; x <= n; x++ {
		shareValue := make([]byte, len(secret))
		xByte := byte(x)

		for i, secretByte := range secret {
			// Construct polynomial for this byte:
			// P(x) = secretByte + c_1*x + ... + c_(k-1)*x^(k-1)
			// Coefficients for byte i start at i*(k-1)
			coeffStart := i * (k - 1)

			// Simple iterative evaluation
			// P(x) = secretByte + c1*x + c2*x^2 ...
			val := secretByte
			xPoly := xByte // x^1, then x^2, etc.

			for j := 0; j < k-1; j++ {
				c := coeffs[coeffStart+j]
				term := gfMul(c, xPoly)
				val = gfAdd(val, term)

				// Next power of x
				if j < k-2 {
					xPoly = gfMul(xPoly, xByte)
				}
			}
			shareValue[i] = val
		}

		// Format share: sigil-v1-<threshold>-<index>-<hex_value>
		shares[x-1] = fmt.Sprintf("sigil-v1-%d-%d-%x", k, x, shareValue)
	}

	return shares, nil
}

// Combine reconstructs a secret from a list of shares.
// Requires at least k shares, where k is the threshold embedded in the shares.
func Combine(shareStrings []string) ([]byte, error) {
	if len(shareStrings) == 0 {
		return nil, ErrNoShares
	}

	uniqueShares, _, secretLen, err := parseAndValidateShares(shareStrings)
	if err != nil {
		return nil, err
	}

	// Reconstruct using Lagrange Interpolation at x=0
	return interpolateSecret(uniqueShares, secretLen)
}

type parsedShare struct {
	x byte
	y []byte
}

func parseAndValidateShares(shareStrings []string) ([]parsedShare, int, int, error) {
	uniqueShares, firstThreshold, secretLen, err := processShares(shareStrings)
	if err != nil {
		return nil, 0, 0, err
	}

	if len(uniqueShares) < firstThreshold {
		return nil, 0, 0, fmt.Errorf("%w: have %d, need %d", ErrNotEnoughUniqueShares, len(uniqueShares), firstThreshold)
	}

	return uniqueShares, firstThreshold, secretLen, nil
}

//nolint:gocognit // Complex validation loop
func processShares(shareStrings []string) ([]parsedShare, int, int, error) {
	var firstThreshold int
	var secretLen int
	var uniqueShares []parsedShare
	usedIndices := make(map[byte]bool)

	for _, s := range shareStrings {
		p, k, err := parseShare(s)
		if err != nil {
			return nil, 0, 0, err
		}

		if len(uniqueShares) == 0 {
			firstThreshold = k
			secretLen = len(p.y)
		}

		if err := validateShare(p, k, firstThreshold, secretLen); err != nil {
			return nil, 0, 0, err
		}

		if usedIndices[p.x] {
			continue // skip duplicate index
		}

		usedIndices[p.x] = true
		uniqueShares = append(uniqueShares, p)

		// Optimization: stop once we have enough shares
		if len(uniqueShares) == firstThreshold {
			break
		}
	}
	return uniqueShares, firstThreshold, secretLen, nil
}

func validateShare(p parsedShare, k, firstThreshold, secretLen int) error {
	if k != firstThreshold {
		return ErrThresholdMismatch
	}
	if len(p.y) != secretLen {
		return ErrLengthMismatch
	}
	return nil
}

func parseShare(s string) (parsedShare, int, error) {
	parts := strings.Split(s, "-")
	if len(parts) != 5 {
		return parsedShare{}, 0, fmt.Errorf("%w: %s", ErrInvalidShareFormat, s)
	}

	if parts[0] != "sigil" || parts[1] != "v1" {
		return parsedShare{}, 0, fmt.Errorf("%w: %s", ErrUnsupportedVersion, s)
	}

	k, err := strconv.Atoi(parts[2])
	if err != nil {
		return parsedShare{}, 0, fmt.Errorf("%w: %s", ErrInvalidThreshold, s)
	}

	idx, err := strconv.Atoi(parts[3])
	if err != nil || idx < 1 || idx > 255 {
		return parsedShare{}, 0, fmt.Errorf("%w: %s", ErrInvalidIndex, s)
	}

	val, err := hex.DecodeString(parts[4])
	if err != nil {
		return parsedShare{}, 0, fmt.Errorf("%w: %s", ErrInvalidHex, s)
	}

	return parsedShare{x: byte(idx), y: val}, k, nil
}

func interpolateSecret(uniqueShares []parsedShare, secretLen int) ([]byte, error) {
	// Precompute Lagrange weights for each share since x-coords are same for all bytes
	weights := make([]byte, len(uniqueShares))
	for i, sI := range uniqueShares {
		weight := byte(1) // Multiplicative identity
		for j, sJ := range uniqueShares {
			if i == j {
				continue
			}
			// numerator: x_j
			// denominator: x_j - x_i
			// weight *= x_j / (x_j - x_i)

			top := sJ.x
			bottom := gfSub(sJ.x, sI.x) // Sub is XOR in GF(2^8)

			// If bottom is 0, shares have same index which shouldn't happen due to check above
			factor := gfDiv(top, bottom)
			weight = gfMul(weight, factor)
		}
		weights[i] = weight
	}

	secret := make([]byte, secretLen)
	// Interpolate each byte
	for i := 0; i < secretLen; i++ {
		var val byte // 0
		for j, s := range uniqueShares {
			// term = y_i * weight_i
			term := gfMul(s.y[i], weights[j])
			val = gfAdd(val, term) // Add is XOR
		}
		secret[i] = val
	}

	return secret, nil
}
