//go:build !windows

package docker

import (
	"fmt"
	"os"
	"syscall"
)

// openLocked opens (or creates) the slots file with an exclusive lock.
func (sm *SlotManager) openLocked() (*os.File, error) {
	f, err := os.OpenFile(sm.slotsFile, os.O_RDWR|os.O_CREATE, 0644)
	if err != nil {
		return nil, fmt.Errorf("open slots file: %w", err)
	}
	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX); err != nil {
		_ = f.Close()
		return nil, fmt.Errorf("lock slots file: %w", err)
	}
	return f, nil
}

// closeUnlocked releases the lock and closes the file.
func (sm *SlotManager) closeUnlocked(f *os.File) {
	_ = syscall.Flock(int(f.Fd()), syscall.LOCK_UN)
	_ = f.Close()
}
