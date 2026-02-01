package crypto_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"sigil/internal/crypto"
)

func TestAge_EncryptDecrypt_RoundTrip(t *testing.T) {
	plaintext := []byte("this is secret wallet data")
	password := "strong-passphrase-123" // gitleaks:allow

	// Encrypt
	ciphertext, err := crypto.Encrypt(plaintext, password)
	require.NoError(t, err)
	assert.NotEqual(t, plaintext, ciphertext)
	assert.NotEmpty(t, ciphertext)

	// Decrypt
	decrypted, err := crypto.Decrypt(ciphertext, password)
	require.NoError(t, err)
	assert.Equal(t, plaintext, decrypted)
}

func TestAge_DecryptWrongPassword(t *testing.T) {
	plaintext := []byte("secret data")
	password := "correct-password" // gitleaks:allow
	wrongPassword := "wrong-password"

	ciphertext, err := crypto.Encrypt(plaintext, password)
	require.NoError(t, err)

	_, err = crypto.Decrypt(ciphertext, wrongPassword)
	assert.Error(t, err)
}

func TestAge_EmptyPlaintext(t *testing.T) {
	plaintext := []byte{}
	password := "password" // gitleaks:allow

	ciphertext, err := crypto.Encrypt(plaintext, password)
	require.NoError(t, err)

	decrypted, err := crypto.Decrypt(ciphertext, password)
	require.NoError(t, err)
	assert.Empty(t, decrypted)
}

func TestAge_EmptyPassword(t *testing.T) {
	plaintext := []byte("data")
	password := ""

	// Empty password is rejected by age
	_, err := crypto.Encrypt(plaintext, password)
	assert.Error(t, err)
}

func TestAge_LargePlaintext(t *testing.T) {
	// 1MB of data
	plaintext := make([]byte, 1024*1024)
	for i := range plaintext {
		plaintext[i] = byte(i % 256)
	}
	password := "password" // gitleaks:allow

	ciphertext, err := crypto.Encrypt(plaintext, password)
	require.NoError(t, err)

	decrypted, err := crypto.Decrypt(ciphertext, password)
	require.NoError(t, err)
	assert.Equal(t, plaintext, decrypted)
}

func TestAge_InvalidCiphertext(t *testing.T) {
	_, err := crypto.Decrypt([]byte("not valid ciphertext"), "password") // gitleaks:allow
	assert.Error(t, err)
}

func TestAge_EncryptWithSecureBytes(t *testing.T) {
	plaintext := []byte("secret wallet data")
	password := "password123" // gitleaks:allow

	sb, err := crypto.SecureBytesFromSlice(plaintext)
	require.NoError(t, err)
	defer sb.Destroy()

	ciphertext, err := crypto.EncryptSecure(sb, password)
	require.NoError(t, err)

	decrypted, err := crypto.Decrypt(ciphertext, password)
	require.NoError(t, err)
	assert.Equal(t, plaintext, decrypted)
}

func TestAge_DecryptToSecureBytes(t *testing.T) {
	plaintext := []byte("secret wallet data")
	password := "password123" // gitleaks:allow

	ciphertext, err := crypto.Encrypt(plaintext, password)
	require.NoError(t, err)

	sb, err := crypto.DecryptSecure(ciphertext, password)
	require.NoError(t, err)
	defer sb.Destroy()

	assert.Equal(t, plaintext, sb.Bytes())
}
