package worktree

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/lost-in-the/grove/internal/config"
	"github.com/lost-in-the/grove/internal/fsutil"
)

// FileField names the config list a file setup entry came from.
type FileField string

const (
	FieldCopyFiles    FileField = "copy_files"
	FieldSymlinkFiles FileField = "symlink_files"
	FieldSymlinkDirs  FileField = "symlink_dirs"
)

// FileStatus is the outcome of one file setup entry.
type FileStatus string

const (
	FileCopied FileStatus = "copied"
	FileLinked FileStatus = "linked"
	FileFailed FileStatus = "failed"
)

// FileResult is the outcome of one configured copy_files, symlink_files, or
// symlink_dirs entry.
type FileResult struct {
	Field  FileField
	Path   string // entry as written in the config, relative to the worktree
	Status FileStatus
	Err    error // set only when Status is FileFailed
}

// ErrSourceMissing means the entry does not exist in the main worktree.
var ErrSourceMissing = errors.New("source missing in the main worktree")

// ErrFileSetupIncomplete is matched (errors.Is) by the error BootstrapWorktree
// returns when at least one configured entry was not copied or linked.
var ErrFileSetupIncomplete = errors.New("file setup incomplete")

// FileSetupError carries the entries that were not copied or linked.
type FileSetupError struct {
	Failed []FileResult
}

func (e *FileSetupError) Error() string {
	parts := make([]string, 0, len(e.Failed))
	for _, r := range e.Failed {
		parts = append(parts, r.line())
	}
	return ErrFileSetupIncomplete.Error() + ": " + strings.Join(parts, "; ")
}

// Is lets errors.Is(err, ErrFileSetupIncomplete) match.
func (e *FileSetupError) Is(target error) bool { return target == ErrFileSetupIncomplete }

// NewFileSetupError returns a *FileSetupError for the failed results, or nil
// when every entry was copied or linked.
func NewFileSetupError(results []FileResult) error {
	var failed []FileResult
	for _, r := range results {
		if r.Status == FileFailed {
			failed = append(failed, r)
		}
	}
	if len(failed) == 0 {
		return nil
	}
	return &FileSetupError{Failed: failed}
}

// Lines returns the user-facing report: one line per failed entry, then one
// line with the remedy for the worktree called name. lead is the opening of
// that line, such as "The worktree was created".
func (e *FileSetupError) Lines(name, lead string) []string {
	lines := make([]string, 0, len(e.Failed)+1)
	for _, r := range e.Failed {
		lines = append(lines, r.line())
	}
	n := len(e.Failed)
	noun, verb := "file", "is"
	if n != 1 {
		noun, verb = "files", "are"
	}
	lines = append(lines, fmt.Sprintf("%s, but %d %s %s missing. Fix the source, then run `grove doctor %s --fix`.", lead, n, noun, verb, name))
	return lines
}

func (r FileResult) line() string {
	prefix := "Not copied"
	if r.Field != FieldCopyFiles {
		prefix = "Not linked"
	}
	return fmt.Sprintf("%s: %s (%s)", prefix, r.Path, r.reason())
}

func (r FileResult) reason() string {
	switch {
	case errors.Is(r.Err, ErrSourceMissing):
		return "missing in the main worktree"
	case errors.Is(r.Err, fsutil.ErrNotRegularFile):
		return "is a directory or special file, not a file"
	case r.Err == nil:
		return "unknown error"
	}
	return strings.ReplaceAll(r.Err.Error(), "\n", " ")
}

// SetupFiles copies and symlinks files from the main worktree into a new
// worktree based on the external compose config. It returns one result per
// configured entry, in config order, and prints nothing: callers decide how
// to report. Returns nil if ext is nil.
func SetupFiles(ext *config.ExternalComposeConfig, newPath, mainPath string) []FileResult {
	if ext == nil {
		return nil
	}

	results := make([]FileResult, 0, len(ext.CopyFiles)+len(ext.SymlinkFiles)+len(ext.SymlinkDirs))
	run := func(field FileField, entries []string, done FileStatus, apply func(src, dst string) error) {
		for _, rel := range entries {
			res := FileResult{Field: field, Path: rel, Status: done}
			if err := setupEntry(rel, newPath, mainPath, apply); err != nil {
				res.Status, res.Err = FileFailed, err
			}
			results = append(results, res)
		}
	}
	run(FieldCopyFiles, ext.CopyFiles, FileCopied, copyFile)
	run(FieldSymlinkFiles, ext.SymlinkFiles, FileLinked, createSymlink)
	run(FieldSymlinkDirs, ext.SymlinkDirs, FileLinked, createSymlink)
	return results
}

func setupEntry(rel, newPath, mainPath string, apply func(src, dst string) error) error {
	src, err := fsutil.SafeJoin(mainPath, rel)
	if err != nil {
		return err
	}
	dst, err := fsutil.SafeJoin(newPath, rel)
	if err != nil {
		return err
	}
	if _, err := os.Lstat(src); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return ErrSourceMissing
		}
		return err
	}
	return apply(src, dst)
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

	if err := os.Symlink(src, dst); err != nil {
		if runtime.GOOS == "windows" {
			return fmt.Errorf("%w\n  hint: symlinks on Windows require Developer Mode or administrator privileges; for files, use copy_files instead", err)
		}
		return err
	}
	return nil
}
