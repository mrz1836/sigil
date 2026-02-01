package sigilcrypto

import (
	"bytes"
	"fmt"
	"io"

	"filippo.io/age"
)

// Encrypt encrypts plaintext using age with a password-based recipient.
func Encrypt(plaintext []byte, password string) ([]byte, error) {
	recipient, err := age.NewScryptRecipient(password)
	if err != nil {
		return nil, fmt.Errorf("creating scrypt recipient: %w", err)
	}

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
func Decrypt(ciphertext []byte, password string) ([]byte, error) {
	identity, err := age.NewScryptIdentity(password)
	if err != nil {
		return nil, fmt.Errorf("creating scrypt identity: %w", err)
	}

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

// EncryptSecure encrypts SecureBytes using age with a password-based recipient.
func EncryptSecure(sb *SecureBytes, password string) ([]byte, error) {
	data := sb.Bytes()
	if data == nil {
		return nil, nil
	}
	return Encrypt(data, password)
}

// DecryptSecure decrypts ciphertext into SecureBytes.
func DecryptSecure(ciphertext []byte, password string) (*SecureBytes, error) {
	plaintext, err := Decrypt(ciphertext, password)
	if err != nil {
		return nil, err
	}

	// Ensure plaintext is zeroed on all paths including errors
	defer func() {
		for i := range plaintext {
			plaintext[i] = 0
		}
	}()

	sb, err := SecureBytesFromSlice(plaintext)
	if err != nil {
		return nil, err
	}

	return sb, nil
}
