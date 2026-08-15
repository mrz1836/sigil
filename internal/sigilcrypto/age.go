package sigilcrypto

import (
	"bytes"
	"fmt"
	"io"
	"sync/atomic"

	"filippo.io/age"
)

// scryptWorkFactor controls the scrypt work factor.
// Default is 18 (age's secure default). Lower values for testing.
//
//nolint:gochecknoglobals // Package-level atomic for thread-safe work factor configuration
var scryptWorkFactor atomic.Int32

//nolint:gochecknoinits // Required to set secure default work factor
func init() {
	scryptWorkFactor.Store(18) // Secure default
}

// SetScryptWorkFactor sets the work factor for scrypt operations.
// Lower values are faster but less secure. Use only for testing.
// Range: 10 (fast/insecure) to 22 (very secure). Default: 18.
func SetScryptWorkFactor(factor int) {
	// Clamp to valid range to prevent overflow and invalid values
	if factor < 10 {
		factor = 10
	} else if factor > 22 {
		factor = 22
	}
	scryptWorkFactor.Store(int32(factor))
}

// EncryptBytes encrypts plaintext under a []byte password. It is the
// no-caller-side-string path: callers keep their password as a []byte they can
// zero, and the unavoidable string conversion required by age's scrypt API
// happens once here (age copies it into an unexported []byte it never zeroes —
// a residual documented in SECURITY.md). Byte-compatible with Encrypt: same age
// code path, so ciphertext from either decrypts with either.
//
// EncryptBytes does not mutate or retain the password slice.
func EncryptBytes(plaintext, password []byte) ([]byte, error) {
	return Encrypt(plaintext, string(password))
}

// DecryptBytes decrypts ciphertext under a []byte password. See EncryptBytes for
// the string-conversion note. The caller MUST zero the returned slice.
func DecryptBytes(ciphertext, password []byte) ([]byte, error) {
	return Decrypt(ciphertext, string(password))
}

// Encrypt encrypts plaintext using age with a password-based recipient.
func Encrypt(plaintext []byte, password string) ([]byte, error) {
	recipient, err := age.NewScryptRecipient(password)
	if err != nil {
		return nil, fmt.Errorf("creating scrypt recipient: %w", err)
	}
	recipient.SetWorkFactor(int(scryptWorkFactor.Load()))

	buf := &bytes.Buffer{}
	w, err := age.Encrypt(buf, recipient)
	if err != nil {
		return nil, fmt.Errorf("initializing encryption: %w", err)
	}

	if _, err := w.Write(plaintext); err != nil {
		return nil, fmt.Errorf("writing encrypted data: %w", err)
	}

	if err := w.Close(); err != nil {
		return nil, fmt.Errorf("finalizing encryption: %w", err)
	}

	return buf.Bytes(), nil
}

// Decrypt decrypts ciphertext using age with a password-based identity.
//
// SECURITY: The caller MUST zero the returned byte slice when done to prevent
// sensitive data (e.g., seed material) from lingering in memory. Prefer
// DecryptSecure when possible, which handles zeroing automatically.
func Decrypt(ciphertext []byte, password string) ([]byte, error) {
	identity, err := age.NewScryptIdentity(password)
	if err != nil {
		return nil, fmt.Errorf("creating scrypt identity: %w", err)
	}
	identity.SetMaxWorkFactor(int(scryptWorkFactor.Load()))

	r, err := age.Decrypt(bytes.NewReader(ciphertext), identity)
	if err != nil {
		return nil, fmt.Errorf("initializing decryption: %w", err)
	}

	plaintext, err := io.ReadAll(r)
	if err != nil {
		return nil, fmt.Errorf("reading decrypted data: %w", err)
	}

	return plaintext, nil
}
