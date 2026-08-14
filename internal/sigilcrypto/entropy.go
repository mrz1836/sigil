package sigilcrypto

import (
	"crypto/rand"
	"io"
)

// Reader is the cryptographically secure random number generator.
// It wraps crypto/rand.Reader for consistency and testability.
//
//nolint:gochecknoglobals // Package-level RNG is required for testability
var Reader io.Reader = rand.Reader

// RandomBytes generates cryptographically secure random bytes.
func RandomBytes(n int) ([]byte, error) {
	b := make([]byte, n)
	if _, err := io.ReadFull(Reader, b); err != nil {
		return nil, err
	}
	return b, nil
}
