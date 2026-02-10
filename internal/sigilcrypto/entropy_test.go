package sigilcrypto

import (
	"bytes"
	"errors"
	"io"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var errMockReaderNotConfigured = errors.New("mock reader not configured")

// mockReader implements io.Reader for testing.
type mockReader struct {
	readFunc func(p []byte) (int, error)
}

func (m *mockReader) Read(p []byte) (int, error) {
	if m.readFunc != nil {
		return m.readFunc(p)
	}
	return 0, errMockReaderNotConfigured
}

func TestRandomBytes(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		n       int
		wantLen int
	}{
		{
			name:    "zero bytes",
			n:       0,
			wantLen: 0,
		},
		{
			name:    "32 bytes",
			n:       32,
			wantLen: 32,
		},
		{
			name:    "1024 bytes",
			n:       1024,
			wantLen: 1024,
		},
		{
			name:    "1MB",
			n:       1024 * 1024,
			wantLen: 1024 * 1024,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			data, err := RandomBytes(tc.n)
			require.NoError(t, err)
			assert.Len(t, data, tc.wantLen)
		})
	}
}

func TestRandomBytes_Randomness(t *testing.T) {
	t.Parallel()

	t.Run("consecutive calls produce different output", func(t *testing.T) {
		t.Parallel()

		data1, err := RandomBytes(32)
		require.NoError(t, err)

		data2, err := RandomBytes(32)
		require.NoError(t, err)

		assert.NotEqual(t, data1, data2, "consecutive calls should produce different random bytes")
	})

	t.Run("output not all zeros", func(t *testing.T) {
		t.Parallel()

		data, err := RandomBytes(32)
		require.NoError(t, err)

		allZeros := bytes.Equal(data, make([]byte, 32))
		assert.False(t, allZeros, "random bytes should not be all zeros")
	})
}

func TestRandomBytes_Errors(t *testing.T) {
	// Cannot run in parallel because we modify package-level Reader variable

	t.Run("reader error", func(t *testing.T) {
		// Save original reader
		originalReader := Reader
		defer func() { Reader = originalReader }()

		// Mock reader that returns error
		expectedErr := io.ErrUnexpectedEOF
		Reader = &mockReader{
			readFunc: func(_ []byte) (int, error) {
				return 0, expectedErr
			},
		}

		data, err := RandomBytes(32)
		require.Error(t, err)
		assert.Nil(t, data)
	})

	t.Run("partial read", func(t *testing.T) {
		// Save original reader
		originalReader := Reader
		defer func() { Reader = originalReader }()

		// Mock reader that returns partial read
		Reader = &mockReader{
			readFunc: func(p []byte) (int, error) {
				return len(p) / 2, io.ErrUnexpectedEOF
			},
		}

		data, err := RandomBytes(32)
		require.Error(t, err)
		assert.Nil(t, data)
	})
}

func TestSecureRandomBytes(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		n       int
		wantLen int
	}{
		{
			name:    "zero bytes",
			n:       0,
			wantLen: 0,
		},
		{
			name:    "32 bytes",
			n:       32,
			wantLen: 32,
		},
		{
			name:    "1024 bytes",
			n:       1024,
			wantLen: 1024,
		},
		{
			name:    "1MB",
			n:       1024 * 1024,
			wantLen: 1024 * 1024,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			sb, err := SecureRandomBytes(tc.n)
			require.NoError(t, err)
			require.NotNil(t, sb)
			defer sb.Destroy()

			assert.Equal(t, tc.wantLen, sb.Len())
		})
	}
}

func TestSecureRandomBytes_Randomness(t *testing.T) {
	t.Parallel()

	t.Run("consecutive calls produce different output", func(t *testing.T) {
		t.Parallel()

		sb1, err := SecureRandomBytes(32)
		require.NoError(t, err)
		require.NotNil(t, sb1)
		defer sb1.Destroy()

		sb2, err := SecureRandomBytes(32)
		require.NoError(t, err)
		require.NotNil(t, sb2)
		defer sb2.Destroy()

		assert.NotEqual(t, sb1.Bytes(), sb2.Bytes(), "consecutive calls should produce different random bytes")
	})

	t.Run("output not all zeros", func(t *testing.T) {
		t.Parallel()

		sb, err := SecureRandomBytes(32)
		require.NoError(t, err)
		require.NotNil(t, sb)
		defer sb.Destroy()

		allZeros := bytes.Equal(sb.Bytes(), make([]byte, 32))
		assert.False(t, allZeros, "random bytes should not be all zeros")
	})
}

func TestSecureRandomBytes_Errors(t *testing.T) {
	// Cannot run in parallel because we modify package-level Reader variable

	t.Run("reader error and cleanup", func(t *testing.T) {
		// Save original reader
		originalReader := Reader
		defer func() { Reader = originalReader }()

		// Mock reader that returns error
		expectedErr := io.ErrUnexpectedEOF
		Reader = &mockReader{
			readFunc: func(_ []byte) (int, error) {
				return 0, expectedErr
			},
		}

		sb, err := SecureRandomBytes(32)
		require.Error(t, err)
		assert.Nil(t, sb, "SecureRandomBytes should return nil on error")
	})

	t.Run("partial read and cleanup", func(t *testing.T) {
		// Save original reader
		originalReader := Reader
		defer func() { Reader = originalReader }()

		// Mock reader that returns partial read
		Reader = &mockReader{
			readFunc: func(p []byte) (int, error) {
				return len(p) / 2, io.ErrUnexpectedEOF
			},
		}

		sb, err := SecureRandomBytes(32)
		require.Error(t, err)
		assert.Nil(t, sb, "SecureRandomBytes should return nil on partial read")
	})
}
