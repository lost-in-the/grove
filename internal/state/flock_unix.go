//go:build !windows

package state

import (
	"fmt"
	"os"
	"syscall"
)

// fileLock acquires an exclusive file lock for cross-process safety.
func (m *Manager) fileLock() (*os.File, error) {
	f, err := os.OpenFile(m.lockFile, os.O_RDWR|os.O_CREATE, 0644)
	if err != nil {
		return nil, fmt.Errorf("open state lock: %w", err)
	}
	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX); err != nil {
		_ = f.Close()
		return nil, fmt.Errorf("acquire state lock: %w", err)
	}
	return f, nil
}

// fileUnlock releases the file lock.
func (m *Manager) fileUnlock(f *os.File) {
	_ = syscall.Flock(int(f.Fd()), syscall.LOCK_UN)
	_ = f.Close()
}
