package worktree

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/lost-in-the/grove/internal/config"
	"github.com/lost-in-the/grove/internal/fsutil"
)

// SetupFiles copies and symlinks files from the main worktree into a new
// worktree based on the external compose config. Returns nil if ext is nil.
func SetupFiles(ext *config.ExternalComposeConfig, newPath, mainPath string) error {
	if ext == nil {
		return nil
	}

	var firstErr error

	for _, relPath := range ext.CopyFiles {
		src := filepath.Join(mainPath, relPath)
		dst := filepath.Join(newPath, relPath)

		if err := copyFile(src, dst); err != nil {
			fmt.Fprintf(os.Stderr, "warning: failed to copy %s: %v\n", relPath, err)
			if firstErr == nil {
				firstErr = err
			}
			continue
		}
		fmt.Fprintf(os.Stderr, "  copied %s\n", relPath)
	}

	for _, relPath := range ext.SymlinkFiles {
		src := filepath.Join(mainPath, relPath)
		dst := filepath.Join(newPath, relPath)

		if err := createSymlink(src, dst); err != nil {
			fmt.Fprintf(os.Stderr, "warning: failed to symlink %s: %v\n", relPath, err)
			if firstErr == nil {
				firstErr = err
			}
			continue
		}
		fmt.Fprintf(os.Stderr, "  symlinked %s\n", relPath)
	}

	for _, relPath := range ext.SymlinkDirs {
		src := filepath.Join(mainPath, relPath)
		dst := filepath.Join(newPath, relPath)

		if err := createSymlink(src, dst); err != nil {
			fmt.Fprintf(os.Stderr, "warning: failed to symlink %s: %v\n", relPath, err)
			if firstErr == nil {
				firstErr = err
			}
			continue
		}
		fmt.Fprintf(os.Stderr, "  symlinked %s\n", relPath)
	}

	return firstErr
}

// copyFile copies a single file from src to dst, creating parent directories as needed.
func copyFile(src, dst string) error {
	return fsutil.CopyFile(src, dst)
}

// createSymlink creates a symbolic link from src to dst, creating parent directories
// as needed. If dst already exists and is a symlink, it is replaced.
func createSymlink(src, dst string) error {
	if _, err := os.Lstat(src); err != nil {
		return fmt.Errorf("source not found: %w", err)
	}

	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return fmt.Errorf("failed to create parent directory: %w", err)
	}

	// Remove existing symlink if present
	if info, err := os.Lstat(dst); err == nil {
		if info.Mode()&os.ModeSymlink != 0 {
			if err := os.Remove(dst); err != nil {
				return fmt.Errorf("failed to remove existing symlink %s: %w", dst, err)
			}
		} else {
			return fmt.Errorf("%s already exists and is not a symlink", dst)
		}
	}

	return os.Symlink(src, dst)
}
