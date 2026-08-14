package sigilcrypto_test

import (
	"bytes"
	"encoding/base64"
	"os"
	"testing"

	"github.com/mrz1836/sigil/internal/sigilcrypto"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The golden was generated once at the secure default work factor (logN=18) with
// this exact plaintext/password; see testdata/age-logn18.golden.b64. It guards
// that the current code still decrypts age scrypt ciphertext produced earlier.
const (
	goldenPlaintext = "golden-plaintext-seed-material"
	goldenPassword  = "golden-password-correct"
)

func loadGolden(t *testing.T) []byte {
	t.Helper()
	raw, err := os.ReadFile("testdata/age-logn18.golden.b64")
	require.NoError(t, err)
	ct, err := base64.StdEncoding.DecodeString(string(bytes.TrimSpace(raw)))
	require.NoError(t, err)
	return ct
}

// TestGolden_ByteAndStringPathsAgree proves DecryptBytes([]byte) and
// Decrypt(string) are the same age code path: both recover the checked-in
// logN=18 golden ciphertext.
func TestGolden_ByteAndStringPathsAgree(t *testing.T) {
	t.Parallel()
	ct := loadGolden(t)

	viaBytes, err := sigilcrypto.DecryptBytes(ct, []byte(goldenPassword))
	require.NoError(t, err)
	assert.Equal(t, goldenPlaintext, string(viaBytes))

	viaString, err := sigilcrypto.Decrypt(ct, goldenPassword)
	require.NoError(t, err)
	assert.Equal(t, goldenPlaintext, string(viaString))

	assert.Equal(t, viaString, viaBytes, "byte-path and string-path must agree")
}

// TestEncryptBytes_CrossCompatWithStringPath confirms ciphertext from either API
// decrypts under the other (age is non-deterministic, so we assert decrypt
// compatibility, not byte-identity).
func TestEncryptBytes_CrossCompatWithStringPath(t *testing.T) {
	t.Parallel()
	pt := []byte("cross-compat-plaintext")
	pw := "cross-compat-pass" //nolint:gosec // G101: test password literal, not a real credential

	fromBytes, err := sigilcrypto.EncryptBytes(pt, []byte(pw))
	require.NoError(t, err)
	fromString, err := sigilcrypto.Encrypt(pt, pw)
	require.NoError(t, err)

	got1, err := sigilcrypto.Decrypt(fromBytes, pw)
	require.NoError(t, err)
	assert.Equal(t, pt, got1)

	got2, err := sigilcrypto.DecryptBytes(fromString, []byte(pw))
	require.NoError(t, err)
	assert.Equal(t, pt, got2)
}

// TestEncryptBytes_DoesNotMutatePassword is the memory-hygiene guarantee: the
// caller's password slice is neither modified nor zeroed, so the caller remains
// responsible for (and able to) zero it afterward.
func TestEncryptBytes_DoesNotMutatePassword(t *testing.T) {
	t.Parallel()
	password := []byte("do-not-touch-me")
	snapshot := append([]byte(nil), password...)

	_, err := sigilcrypto.EncryptBytes([]byte("payload"), password)
	require.NoError(t, err)
	assert.Equal(t, snapshot, password, "EncryptBytes must not mutate the password slice")

	_, err = sigilcrypto.DecryptBytes(loadGolden(t), []byte(goldenPassword))
	require.NoError(t, err)
}

// TestDecryptBytes_WrongPassword fails closed.
func TestDecryptBytes_WrongPassword(t *testing.T) {
	t.Parallel()
	_, err := sigilcrypto.DecryptBytes(loadGolden(t), []byte("wrong-password"))
	require.Error(t, err)
}
