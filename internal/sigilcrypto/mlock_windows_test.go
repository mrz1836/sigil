//go:build windows

package sigilcrypto

import (
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMlock_Windows(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		size int
	}{
		{
			name: "empty buffer",
			size: 0,
		},
		{
			name: "small buffer (32 bytes)",
			size: 32,
		},
		{
			name: "page-sized buffer (4KB)",
			size: 4096,
		},
		{
			name: "large buffer (1MB)",
			size: 1024 * 1024,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			data := make([]byte, tc.size)

			// Empty buffer should always return false
			if tc.size == 0 {
				result := mlock(data)
				assert.False(t, result, "mlock on empty buffer should return false")
				return
			}

			// For non-empty buffers, mlock may succeed or fail depending on system limits
			// What matters is that it doesn't panic
			result := mlock(data)
			t.Logf("mlock(%d bytes) = %v", tc.size, result)

			// Clean up if locked
			if result {
				munlock(data)
			}
		})
	}
}

func TestMlock_NilSlice(t *testing.T) {
	t.Parallel()

	// nil slice should not panic
	var data []byte
	result := mlock(data)
	assert.False(t, result, "mlock on nil slice should return false")

	// munlock on nil slice should not panic
	munlock(data)
}

func TestMunlock_Windows(t *testing.T) {
	t.Parallel()

	t.Run("munlock after mlock", func(t *testing.T) {
		t.Parallel()

		data := make([]byte, 32)
		locked := mlock(data)
		t.Logf("mlock result: %v", locked)

		// munlock should not panic regardless of whether mlock succeeded
		munlock(data)
	})

	t.Run("munlock without mlock", func(t *testing.T) {
		t.Parallel()

		data := make([]byte, 32)
		// munlock should be idempotent - safe to call even if not locked
		munlock(data)
	})

	t.Run("double munlock", func(t *testing.T) {
		t.Parallel()

		data := make([]byte, 32)
		locked := mlock(data)
		t.Logf("mlock result: %v", locked)

		// First munlock
		munlock(data)

		// Second munlock should not panic
		munlock(data)
	})

	t.Run("munlock on empty buffer", func(t *testing.T) {
		t.Parallel()

		data := make([]byte, 0)
		// Should not panic
		munlock(data)
	})
}

func TestMlock_Concurrent(t *testing.T) {
	t.Parallel()

	// Test that multiple concurrent mlock/munlock calls don't cause issues
	const numGoroutines = 10
	var wg sync.WaitGroup

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()

			data := make([]byte, 1024)
			locked := mlock(data)
			t.Logf("goroutine %d: mlock = %v", id, locked)

			if locked {
				munlock(data)
			}
		}(i)
	}

	wg.Wait()
}
