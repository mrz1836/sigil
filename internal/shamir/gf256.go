package shamir

import "sync"

// gf256.go implements the Galois Field (2^8) arithmetic for Shamir's Secret Sharing.
// This implementation uses the Rijndael finite field (GF(2^8) with polynomial x^8 + x^4 + x^3 + x + 1).

const (
	// The polynomial used for GF(2^8) in AES (Rijndael) is x^8 + x^4 + x^3 + x + 1 (0x11b)
	// However, for SSS, we just need *a* field.
	// Many SSS implementations use the same field as AES (0x11b).
	// Let's use 0x11b (283 decimal).
	primitivePolynomial = 0x11b

	// fieldSize is the number of elements in the field (2^8).
	fieldSize = 256
)

var (
	// expTable stores the exponentiation table (generator^i).
	//nolint:gochecknoglobals // precomputed table
	expTable [fieldSize]byte

	// logTable stores the logarithm table (log_generator(x)).
	//nolint:gochecknoglobals // precomputed table
	logTable [fieldSize]byte

	// tablesInit ensures tables are computed only once.
	//nolint:gochecknoglobals // sync.Once is inherently global state management here
	tablesInit sync.Once
)

// initTables computes the exponentiation and logarithm tables.
// This is done to speed up multiplication and division in the field.
func initTables() {
	tablesInit.Do(func() {
		// The generator for GF(2^8) with 0x11b is typically 3.
		// However, many SSS implementations actually use a generator of 3, or simply compute powers.
		// Let's stick to a standard construction.
		//
		// We'll construct the tables using generator g=3.
		//
		// 0 is not in the multiplicative group, so log[0] is undefined (we'll leave it as 0 but handle it).
		// exp[255] = exp[0] = 1 (cyclic).

		var x uint16 = 1
		for i := 0; i < fieldSize-1; i++ {
			expTable[i] = byte(x)
			logTable[x] = byte(i)

			// Multiply by 3 (polynomial x + 1)
			// x * 3 = x * (x + 1) = x^2 + x
			// In binary: (x << 1) ^ x
			x = (x << 1) ^ x

			// If we overflowed 8 bits (x >= 256), subtract the primitive polynomial
			if x >= fieldSize {
				x ^= primitivePolynomial
			}
		}

		// exp[255] is effectively exp[0] = 1
		// But let's verify exp[255] would map correctly if we wrapped.
		// expTable[255] = expTable[0] // logic check
	})
}

// gfAdd adds two elements in GF(2^8).
// In GF(2^n), addition is XOR.
func gfAdd(a, b byte) byte {
	return a ^ b
}

// gfSub subtracts two elements in GF(2^8).
// In GF(2^n), subtraction is identical to addition (XOR).
func gfSub(a, b byte) byte {
	return a ^ b
}

// gfMul multiplies two elements in GF(2^8).
// Uses the log/exp tables for efficient lookup.
// a * b = g^(log(a)) * g^(log(b)) = g^(log(a) + log(b))
func gfMul(a, b byte) byte {
	initTables()
	if a == 0 || b == 0 {
		return 0
	}
	logA := int(logTable[a])
	logB := int(logTable[b])
	return expTable[(logA+logB)%(fieldSize-1)]
}

// gfDiv divides two elements in GF(2^8).
// a / b = a * (1/b) = g^(log(a)) * g^(-log(b)) = g^(log(a) - log(b))
func gfDiv(a, b byte) byte {
	initTables()
	if b == 0 {
		// Division by zero is undefined.
		// In this context, it should not happen if the algorithm is correct.
		// We could panic or return 0. Returning 0 is safer but incorrect mathematically.
		panic("division by zero in GF(2^8)")
	}
	if a == 0 {
		return 0
	}
	logA := int(logTable[a])
	logB := int(logTable[b])
	// Ensure result is positive before mod
	diff := (logA - logB) % (fieldSize - 1)
	if diff < 0 {
		diff += (fieldSize - 1)
	}
	return expTable[diff]
}
