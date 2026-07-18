//go:build !windows

package fsutil

import (
	"os"
	"path/filepath"
	"syscall"
	"testing"
)

// TestAtomicWriteFile_RespectsUmask: the requested permission must be
// filtered through the process umask, exactly like os.WriteFile — the
// pre-consolidation writers created their temp files with O_CREATE and never
// chmod'ed, so state.json landed 0600 under umask 077. An explicit
// Chmod(0644) after the fact silently made grove's state world-readable on
// hardened hosts.
func TestAtomicWriteFile_RespectsUmask(t *testing.T) {
	old := syscall.Umask(0o077)
	defer syscall.Umask(old)

	dir := t.TempDir()
	path := filepath.Join(dir, "state.json")
	if err := AtomicWriteFile(path, []byte("{}"), 0o644); err != nil {
		t.Fatalf("AtomicWriteFile() error = %v", err)
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := info.Mode().Perm(); got != 0o600 {
		t.Errorf("perm = %o, want %o (0644 masked by umask 077)", got, 0o600)
	}
}
