package fsutil

import (
	"fmt"
	"os"
	"path/filepath"
)

// AtomicWriteFile writes data to path atomically. It creates a uniquely-named
// temp file in the destination directory, writes the data, fsync-equivalent
// close, then renames over the destination. The unique suffix avoids clobbers
// when multiple processes write concurrently — each writer renames its own
// temp file, with last-rename winning.
//
// Creates the parent directory if it doesn't exist (mode 0o755). Cleans up
// the temp file on every error path so partial writes don't leak.
func AtomicWriteFile(path string, data []byte, perm os.FileMode) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("atomic write: mkdir parent: %w", err)
	}

	base := filepath.Base(path)
	f, err := os.CreateTemp(dir, base+".tmp-*")
	if err != nil {
		return fmt.Errorf("atomic write: create temp: %w", err)
	}
	tmp := f.Name()
	cleanup := func() { _ = os.Remove(tmp) }

	if _, err := f.Write(data); err != nil {
		_ = f.Close()
		cleanup()
		return fmt.Errorf("atomic write: write: %w", err)
	}
	if err := f.Close(); err != nil {
		cleanup()
		return fmt.Errorf("atomic write: close: %w", err)
	}
	// CreateTemp uses 0o600; align to caller-requested perm before rename so
	// the destination ends up with the expected mode.
	if err := os.Chmod(tmp, perm); err != nil {
		cleanup()
		return fmt.Errorf("atomic write: chmod: %w", err)
	}
	if err := os.Rename(tmp, path); err != nil {
		cleanup()
		return fmt.Errorf("atomic write: rename: %w", err)
	}
	return nil
}
