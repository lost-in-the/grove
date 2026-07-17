//go:build windows

package fsutil

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

// LockFile opens (creating if needed) the file at path and acquires an
// exclusive lock on it, returning the open, locked file. Call UnlockFile to
// release the lock and close the file.
func LockFile(path string) (*os.File, error) {
	f, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE, 0644)
	if err != nil {
		return nil, fmt.Errorf("open lock file %s: %w", path, err)
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
		return nil, fmt.Errorf("acquire lock on %s: %w", path, err)
	}
	return f, nil
}

// UnlockFile releases the lock acquired by LockFile and closes the file.
func UnlockFile(f *os.File) {
	var ol syscall.Overlapped
	//nolint:errcheck
	procUnlockFileEx.Call(f.Fd(), 0, 1, 0, uintptr(unsafe.Pointer(&ol)))
	_ = f.Close()
}
