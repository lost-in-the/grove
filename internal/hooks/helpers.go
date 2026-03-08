package hooks

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/LeahArmstrong/grove-cli/internal/fsutil"
)

// resolvePath resolves a path that may be relative or absolute
// If the path is relative, it's resolved against basePath
func resolvePath(path, basePath string) string {
	if path == "" {
		return basePath
	}
	if filepath.IsAbs(path) {
		return path
	}
	return filepath.Join(basePath, path)
}

// copyFile copies a single file from src to dst
func copyFile(src, dst string) error {
	return fsutil.CopyFile(src, dst)
}

// copyDir recursively copies a directory from src to dst
func copyDir(src, dst string) error {
	// Get source directory info
	srcInfo, err := os.Stat(src)
	if err != nil {
		return err
	}

	// Create destination directory
	if err := os.MkdirAll(dst, srcInfo.Mode()); err != nil {
		return err
	}

	// Read source directory
	entries, err := os.ReadDir(src)
	if err != nil {
		return err
	}

	for _, entry := range entries {
		srcPath := filepath.Join(src, entry.Name())
		dstPath := filepath.Join(dst, entry.Name())

		if entry.IsDir() {
			if err := copyDir(srcPath, dstPath); err != nil {
				return err
			}
		} else {
			// Handle symlinks
			info, err := entry.Info()
			if err != nil {
				return err
			}
			if info.Mode()&os.ModeSymlink != 0 {
				// Copy symlink as symlink
				link, err := os.Readlink(srcPath)
				if err != nil {
					return err
				}
				if err := os.Symlink(link, dstPath); err != nil {
					return err
				}
			} else {
				if err := copyFile(srcPath, dstPath); err != nil {
					return err
				}
			}
		}
	}

	return nil
}

// runCommand executes a shell command with a timeout
// Output is streamed to the provided stdout/stderr writers in real-time
func runCommand(command, workDir string, timeout time.Duration, stdout, stderr io.Writer) error {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	// Use shell to execute the command
	cmd := exec.CommandContext(ctx, "sh", "-c", command)
	cmd.Dir = workDir
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	cmd.Stdin = nil

	// Set up environment
	cmd.Env = os.Environ()

	err := cmd.Run()
	if ctx.Err() == context.DeadlineExceeded {
		return fmt.Errorf("command timed out after %v", timeout)
	}
	if err != nil {
		return fmt.Errorf("command failed: %w", err)
	}

	return nil
}
