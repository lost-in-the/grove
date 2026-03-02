//go:build windows

package docker

import (
	"fmt"
	"os"
	"syscall"
	"unsafe"
)

var (
	modkernel32      = syscall.NewLazyDLL("kernel32.dll")
	procLockFileEx   = modkernel32.NewProc("LockFileEx")
	procUnlockFileEx = modkernel32.NewProc("UnlockFileEx")
)

const lockfileExclusiveLock = 0x00000002

// openLocked opens (or creates) the slots file with an exclusive lock.
func (sm *SlotManager) openLocked() (*os.File, error) {
	f, err := os.OpenFile(sm.slotsFile, os.O_RDWR|os.O_CREATE, 0644)
	if err != nil {
		return nil, fmt.Errorf("open slots file: %w", err)
	}
	var ol syscall.Overlapped
	r1, _, err := procLockFileEx.Call(
		f.Fd(),
		lockfileExclusiveLock,
		0,
		1, 0,
		uintptr(unsafe.Pointer(&ol)),
	)
	if r1 == 0 {
		_ = f.Close()
		return nil, fmt.Errorf("lock slots file: %w", err)
	}
	return f, nil
}

// closeUnlocked releases the lock and closes the file.
func (sm *SlotManager) closeUnlocked(f *os.File) {
	var ol syscall.Overlapped
	//nolint:errcheck
	procUnlockFileEx.Call(f.Fd(), 0, 1, 0, uintptr(unsafe.Pointer(&ol)))
	_ = f.Close()
}
