// Package crypto provides cryptographic utilities for Sigil.
// Internal packages may shadow stdlib names for domain-specific implementations.
//
//nolint:revive // Internal package name is intentional
package crypto

import (
	"runtime"
	"sync"
)

// SecureBytes is a wrapper for sensitive byte slices that provides
// secure memory handling with mlock and explicit zeroing.
type SecureBytes struct {
	data   []byte
	locked bool
	mu     sync.Mutex
}

// NewSecureBytes creates a new SecureBytes with the given size.
// The memory is locked if the system supports it.
func NewSecureBytes(size int) (*SecureBytes, error) {
	data := make([]byte, size)

	sb := &SecureBytes{
		data:   data,
		locked: false,
	}

	// Try to lock memory - don't fail if not possible
	sb.locked = mlock(data)

	// Set finalizer to ensure memory is cleared even if Destroy isn't called
	runtime.SetFinalizer(sb, func(s *SecureBytes) {
		s.Destroy()
	})

	return sb, nil
}

// SecureBytesFromSlice creates a SecureBytes from an existing slice.
// The data is copied into secure memory.
func SecureBytesFromSlice(data []byte) (*SecureBytes, error) {
	sb, err := NewSecureBytes(len(data))
	if err != nil {
		return nil, err
	}
	copy(sb.data, data)
	return sb, nil
}

// Bytes returns the underlying byte slice.
// Returns nil if the SecureBytes has been destroyed.
func (s *SecureBytes) Bytes() []byte {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.data
}

// IsLocked returns whether the memory is locked (mlocked).
func (s *SecureBytes) IsLocked() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.locked
}

// Destroy zeros the memory and unlocks it.
// Safe to call multiple times.
func (s *SecureBytes) Destroy() {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.data == nil {
		return
	}

	// Zero the memory
	for i := range s.data {
		s.data[i] = 0
	}

	// Unlock if locked
	if s.locked {
		munlock(s.data)
		s.locked = false
	}

	// Clear the slice reference
	s.data = nil

	// Remove the finalizer since we've already cleaned up
	runtime.SetFinalizer(s, nil)
}

// Len returns the length of the data.
func (s *SecureBytes) Len() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.data == nil {
		return 0
	}
	return len(s.data)
}
