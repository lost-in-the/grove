package state

import (
	"os"

	"github.com/lost-in-the/grove/internal/fsutil"
)

// fileLock acquires an exclusive cross-process lock on the state lock file.
func (m *Manager) fileLock() (*os.File, error) {
	return fsutil.LockFile(m.lockFile)
}

// fileUnlock releases the lock and closes the file.
func (m *Manager) fileUnlock(f *os.File) {
	fsutil.UnlockFile(f)
}
