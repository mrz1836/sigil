//go:build windows

package crypto

import (
	"unsafe"

	"golang.org/x/sys/windows"
)

// mlock attempts to lock the memory region containing the data.
// Returns true if successful, false otherwise.
func mlock(data []byte) bool {
	if len(data) == 0 {
		return false
	}
	return windows.VirtualLock(uintptr(unsafe.Pointer(&data[0])), uintptr(len(data))) == nil
}

// munlock unlocks the memory region.
func munlock(data []byte) {
	if len(data) == 0 {
		return
	}
	_ = windows.VirtualUnlock(uintptr(unsafe.Pointer(&data[0])), uintptr(len(data)))
}
