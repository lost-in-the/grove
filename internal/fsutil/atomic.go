package fsutil

import (
	"fmt"
	"os"
	"path/filepath"
)

// AtomicWriteFile writes data to path atomically. It creates a uniquely-named
// temp file in the destination directory, writes the data, fsyncs it to disk,
// closes it, then renames over the destination and fsyncs the parent directory.
// The unique suffix avoids clobbers when multiple processes write concurrently —
// each writer renames its own temp file, with last-rename winning.
//
// perm is applied at creation (O_CREATE) so the process umask filters it,
// exactly like os.WriteFile — an explicit post-hoc Chmod would ignore the
// umask and silently widen files like state.json to world-readable on hosts
// running umask 077.
//
// Creates the parent directory if it doesn't exist (mode 0o755). Cleans up
// the temp file on every error path so partial writes don't leak.
func AtomicWriteFile(path string, data []byte, perm os.FileMode) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("atomic write: mkdir parent: %w", err)
	}

	f, tmp, err := createUniqueTemp(path, perm)
	if err != nil {
		return fmt.Errorf("atomic write: create temp: %w", err)
	}
	cleanup := func() { _ = os.Remove(tmp) }

	if _, err := f.Write(data); err != nil {
		_ = f.Close()
		cleanup()
		return fmt.Errorf("atomic write: write: %w", err)
	}
	// fsync the data to disk BEFORE the rename. Without this, a crash shortly
	// after the rename can leave the destination pointing at not-yet-flushed
	// (zero-length) content on journaled-metadata filesystems — which then
	// fails to parse on the next load (B35).
	if err := f.Sync(); err != nil {
		_ = f.Close()
		cleanup()
		return fmt.Errorf("atomic write: sync: %w", err)
	}
	if err := f.Close(); err != nil {
		cleanup()
		return fmt.Errorf("atomic write: close: %w", err)
	}
	if err := os.Rename(tmp, path); err != nil {
		cleanup()
		return fmt.Errorf("atomic write: rename: %w", err)
	}
	// Best-effort fsync of the directory so the rename itself is durable.
	// Not all filesystems support directory fsync; ignore failures.
	if d, err := os.Open(dir); err == nil {
		_ = d.Sync()
		_ = d.Close()
	}
	return nil
}

// createUniqueTemp opens a fresh temp file next to path with perm applied at
// creation (umask-filtered, like os.WriteFile). The pid distinguishes
// concurrent processes and the counter concurrent goroutines / stale
// leftovers; O_EXCL turns any residual collision into a retry.
func createUniqueTemp(path string, perm os.FileMode) (*os.File, string, error) {
	pid := os.Getpid()
	for i := 0; i < 10000; i++ {
		tmp := fmt.Sprintf("%s.tmp-%d-%d", path, pid, i)
		f, err := os.OpenFile(tmp, os.O_RDWR|os.O_CREATE|os.O_EXCL, perm)
		if err == nil {
			return f, tmp, nil
		}
		if !os.IsExist(err) {
			return nil, "", err
		}
	}
	return nil, "", fmt.Errorf("could not create a unique temp file for %s", path)
}
