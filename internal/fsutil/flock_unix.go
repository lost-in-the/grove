//go:build !windows

package fsutil

import (
	"fmt"
	"os"
	"syscall"
)

// LockFile opens (creating if needed) the file at path and acquires an
// exclusive advisory lock on it, returning the open, locked file. Call
// UnlockFile to release the lock and close the file. Shared by the state and
// docker-slot managers so the cross-process locking lives in one place.
func LockFile(path string) (*os.File, error) {
	f, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE, 0644)
	if err != nil {
		return nil, fmt.Errorf("open lock file %s: %w", path, err)
	}
	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX); err != nil {
		_ = f.Close()
		return nil, fmt.Errorf("acquire lock on %s: %w", path, err)
	}
	return f, nil
}

// UnlockFile releases the lock acquired by LockFile and closes the file.
func UnlockFile(f *os.File) {
	_ = syscall.Flock(int(f.Fd()), syscall.LOCK_UN)
	_ = f.Close()
}
