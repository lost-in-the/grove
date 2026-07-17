package docker

import (
	"os"

	"github.com/lost-in-the/grove/internal/fsutil"
)

// openLocked opens (creating if needed) the slots file with an exclusive
// cross-process lock. Call closeUnlocked to release it.
func (sm *SlotManager) openLocked() (*os.File, error) {
	return fsutil.LockFile(sm.slotsFile)
}

// closeUnlocked releases the lock and closes the file.
func (sm *SlotManager) closeUnlocked(f *os.File) {
	fsutil.UnlockFile(f)
}
