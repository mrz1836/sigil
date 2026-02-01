//go:build !windows

package sigilcrypto

import (
	"golang.org/x/sys/unix"
)

// mlock attempts to lock the memory region containing the data.
// Returns true if successful, false otherwise.
func mlock(data []byte) bool {
	if len(data) == 0 {
		return false
	}
	return unix.Mlock(data) == nil
}

// munlock unlocks the memory region.
func munlock(data []byte) {
	if len(data) == 0 {
		return
	}
	_ = unix.Munlock(data)
}
