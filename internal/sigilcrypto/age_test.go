package sigilcrypto_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mrz1836/sigil/internal/sigilcrypto"
)

func TestAge_EncryptDecrypt_RoundTrip(t *testing.T) {
	t.Parallel()
	plaintext := []byte("this is secret wallet data")
	password := "strong-passphrase-123" // gitleaks:allow

	// Encrypt
	ciphertext, err := sigilcrypto.Encrypt(plaintext, password)
	require.NoError(t, err)
	assert.NotEqual(t, plaintext, ciphertext)
	assert.NotEmpty(t, ciphertext)

	// Decrypt
	decrypted, err := sigilcrypto.Decrypt(ciphertext, password)
	require.NoError(t, err)
	assert.Equal(t, plaintext, decrypted)
}

func TestAge_DecryptWrongPassword(t *testing.T) {
	t.Parallel()
	plaintext := []byte("secret data")
	password := "correct-password" // gitleaks:allow
	wrongPassword := "wrong-password"

	ciphertext, err := sigilcrypto.Encrypt(plaintext, password)
	require.NoError(t, err)

	_, err = sigilcrypto.Decrypt(ciphertext, wrongPassword)
	assert.Error(t, err)
}

func TestAge_EmptyPlaintext(t *testing.T) {
	t.Parallel()
	plaintext := []byte{}
	password := "password" // gitleaks:allow

	ciphertext, err := sigilcrypto.Encrypt(plaintext, password)
	require.NoError(t, err)

	decrypted, err := sigilcrypto.Decrypt(ciphertext, password)
	require.NoError(t, err)
	assert.Empty(t, decrypted)
}

func TestAge_EmptyPassword(t *testing.T) {
	t.Parallel()
	plaintext := []byte("data")
	password := ""

	// Empty password is rejected by age
	_, err := sigilcrypto.Encrypt(plaintext, password)
	assert.Error(t, err)
}

func TestAge_LargePlaintext(t *testing.T) {
	t.Parallel()
	// 1MB of data
	plaintext := make([]byte, 1024*1024)
	for i := range plaintext {
		plaintext[i] = byte(i % 256)
	}
	password := "password" // gitleaks:allow

	ciphertext, err := sigilcrypto.Encrypt(plaintext, password)
	require.NoError(t, err)

	decrypted, err := sigilcrypto.Decrypt(ciphertext, password)
	require.NoError(t, err)
	assert.Equal(t, plaintext, decrypted)
}

func TestAge_InvalidCiphertext(t *testing.T) {
	t.Parallel()
	_, err := sigilcrypto.Decrypt([]byte("not valid ciphertext"), "password") // gitleaks:allow
	assert.Error(t, err)
}

func TestAge_EncryptWithSecureBytes(t *testing.T) {
	t.Parallel()
	plaintext := []byte("secret wallet data")
	password := "password123" // gitleaks:allow

	sb, err := sigilcrypto.SecureBytesFromSlice(plaintext)
	require.NoError(t, err)
	defer sb.Destroy()

	ciphertext, err := sigilcrypto.EncryptSecure(sb, password)
	require.NoError(t, err)

	decrypted, err := sigilcrypto.Decrypt(ciphertext, password)
	require.NoError(t, err)
	assert.Equal(t, plaintext, decrypted)
}

func TestAge_DecryptToSecureBytes(t *testing.T) {
	t.Parallel()
	plaintext := []byte("secret wallet data")
	password := "password123" // gitleaks:allow

	ciphertext, err := sigilcrypto.Encrypt(plaintext, password)
	require.NoError(t, err)

	sb, err := sigilcrypto.DecryptSecure(ciphertext, password)
	require.NoError(t, err)
	defer sb.Destroy()

	assert.Equal(t, plaintext, sb.Bytes())
}
