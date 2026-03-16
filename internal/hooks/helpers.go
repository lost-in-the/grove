package hooks

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/lost-in-the/grove/internal/fsutil"
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

// copyDirEntry copies a single directory entry (file or symlink) from srcPath to dstPath.
func copyDirEntry(srcPath, dstPath string, info os.FileInfo) error {
	if info.Mode()&os.ModeSymlink != 0 {
		link, err := os.Readlink(srcPath)
		if err != nil {
			return err
		}
		return os.Symlink(link, dstPath)
	}
	return copyFile(srcPath, dstPath)
}

// copyDir recursively copies a directory from src to dst
func copyDir(src, dst string) error {
	srcInfo, err := os.Stat(src)
	if err != nil {
		return err
	}

	if err := os.MkdirAll(dst, srcInfo.Mode()); err != nil {
		return err
	}

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
			continue
		}

		info, err := entry.Info()
		if err != nil {
			return err
		}
		if err := copyDirEntry(srcPath, dstPath, info); err != nil {
			return err
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
