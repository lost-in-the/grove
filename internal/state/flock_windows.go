//go:build windows

package state

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

// fileLock acquires an exclusive file lock for cross-process safety.
func (m *Manager) fileLock() (*os.File, error) {
	f, err := os.OpenFile(m.lockFile, os.O_RDWR|os.O_CREATE, 0644)
	if err != nil {
		return nil, fmt.Errorf("open state lock: %w", err)
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
		return nil, fmt.Errorf("acquire state lock: %w", err)
	}
	return f, nil
}

// fileUnlock releases the file lock.
func (m *Manager) fileUnlock(f *os.File) {
	var ol syscall.Overlapped
	//nolint:errcheck
	procUnlockFileEx.Call(f.Fd(), 0, 1, 0, uintptr(unsafe.Pointer(&ol)))
	_ = f.Close()
}
