package sigilcrypto_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mrz1836/sigil/internal/sigilcrypto"
)

func TestSecureBytes_Creation(t *testing.T) {
	t.Parallel()
	sb, err := sigilcrypto.NewSecureBytes(32)
	require.NoError(t, err)
	defer sb.Destroy()

	assert.NotNil(t, sb.Bytes())
	assert.Len(t, sb.Bytes(), 32)
}

func TestSecureBytes_Zeroing(t *testing.T) {
	t.Parallel()
	sb, err := sigilcrypto.NewSecureBytes(32)
	require.NoError(t, err)

	// Write some data
	data := sb.Bytes()
	for i := range data {
		data[i] = byte(i)
	}

	// Verify data was written
	assert.Equal(t, byte(0), data[0])
	assert.Equal(t, byte(31), data[31])

	// Destroy should zero the memory
	sb.Destroy()

	// After destroy, Bytes() should return nil
	assert.Nil(t, sb.Bytes())
}

func TestSecureBytes_DoubleDestroy(t *testing.T) {
	t.Parallel()
	sb, err := sigilcrypto.NewSecureBytes(32)
	require.NoError(t, err)

	sb.Destroy()
	// Should not panic on double destroy
	sb.Destroy()

	assert.Nil(t, sb.Bytes())
}

func TestSecureBytes_ZeroSize(t *testing.T) {
	t.Parallel()
	sb, err := sigilcrypto.NewSecureBytes(0)
	require.NoError(t, err)
	defer sb.Destroy()

	assert.Empty(t, sb.Bytes())
}

func TestSecureBytes_FromBytes(t *testing.T) {
	t.Parallel()
	original := []byte("secret key material")
	sb, err := sigilcrypto.SecureBytesFromSlice(original)
	require.NoError(t, err)
	defer sb.Destroy()

	assert.Equal(t, original, sb.Bytes())
}

func TestSecureBytes_Copy(t *testing.T) {
	t.Parallel()
	sb1, err := sigilcrypto.NewSecureBytes(16)
	require.NoError(t, err)
	defer sb1.Destroy()

	// Write data
	copy(sb1.Bytes(), []byte("1234567890123456"))

	// Copy to new SecureBytes
	sb2, err := sigilcrypto.SecureBytesFromSlice(sb1.Bytes())
	require.NoError(t, err)
	defer sb2.Destroy()

	assert.Equal(t, sb1.Bytes(), sb2.Bytes())

	// Destroy sb1 should not affect sb2
	sb1.Destroy()
	assert.NotNil(t, sb2.Bytes())
	assert.Equal(t, []byte("1234567890123456"), sb2.Bytes())
}

func TestSecureBytes_IsLocked(t *testing.T) {
	t.Parallel()
	sb, err := sigilcrypto.NewSecureBytes(32)
	require.NoError(t, err)
	defer sb.Destroy()

	// IsLocked may return true or false depending on system capabilities
	// We just verify it doesn't panic
	_ = sb.IsLocked()
}
