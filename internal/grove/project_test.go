package grove

import (
	"os"
	"path/filepath"
	"testing"
)

func TestFindRoot(t *testing.T) {
	t.Run("finds .grove in current directory", func(t *testing.T) {
		tmpDir := t.TempDir()
		groveDir := filepath.Join(tmpDir, ".grove")
		if err := os.MkdirAll(groveDir, 0755); err != nil {
			t.Fatalf("failed to create .grove dir: %v", err)
		}

		found, err := FindRoot(tmpDir)
		if err != nil {
			t.Fatalf("FindRoot() error = %v", err)
		}
		if found != groveDir {
			t.Errorf("FindRoot() = %q, want %q", found, groveDir)
		}
	})

	t.Run("finds .grove in parent directory", func(t *testing.T) {
		tmpDir := t.TempDir()
		groveDir := filepath.Join(tmpDir, ".grove")
		subDir := filepath.Join(tmpDir, "sub", "nested")

		if err := os.MkdirAll(groveDir, 0755); err != nil {
			t.Fatalf("failed to create .grove dir: %v", err)
		}
		if err := os.MkdirAll(subDir, 0755); err != nil {
			t.Fatalf("failed to create sub dir: %v", err)
		}

		found, err := FindRoot(subDir)
		if err != nil {
			t.Fatalf("FindRoot() error = %v", err)
		}
		if found != groveDir {
			t.Errorf("FindRoot() = %q, want %q", found, groveDir)
		}
	})

	t.Run("returns empty when not found", func(t *testing.T) {
		tmpDir := t.TempDir()

		found, err := FindRoot(tmpDir)
		if err != nil {
			t.Fatalf("FindRoot() error = %v", err)
		}
		if found != "" {
			t.Errorf("FindRoot() = %q, want empty", found)
		}
	})
}

func TestIsInsideWorktree(t *testing.T) {
	t.Run("false when .git is directory (main repo)", func(t *testing.T) {
		tmpDir := t.TempDir()
		gitDir := filepath.Join(tmpDir, ".git")
		if err := os.MkdirAll(gitDir, 0755); err != nil {
			t.Fatalf("failed to create .git dir: %v", err)
		}

		// Change to the temp dir
		oldWd, _ := os.Getwd()
		defer os.Chdir(oldWd)
		os.Chdir(tmpDir)

		isWT, err := IsInsideWorktree()
		if err != nil {
			t.Fatalf("IsInsideWorktree() error = %v", err)
		}
		if isWT {
			t.Error("IsInsideWorktree() = true, want false for main repo")
		}
	})

	t.Run("true when .git is file (worktree)", func(t *testing.T) {
		tmpDir := t.TempDir()
		gitFile := filepath.Join(tmpDir, ".git")
		// Worktrees have .git as a file pointing to the main repo
		if err := os.WriteFile(gitFile, []byte("gitdir: /some/path/.git/worktrees/foo"), 0644); err != nil {
			t.Fatalf("failed to create .git file: %v", err)
		}

		oldWd, _ := os.Getwd()
		defer os.Chdir(oldWd)
		os.Chdir(tmpDir)

		isWT, err := IsInsideWorktree()
		if err != nil {
			t.Fatalf("IsInsideWorktree() error = %v", err)
		}
		if !isWT {
			t.Error("IsInsideWorktree() = false, want true for worktree")
		}
	})
}

func TestProjectRoot(t *testing.T) {
	t.Run("returns parent of .grove directory", func(t *testing.T) {
		tmpDir := t.TempDir()
		// Resolve symlinks for consistent comparison (macOS /var -> /private/var)
		tmpDir, _ = filepath.EvalSymlinks(tmpDir)

		groveDir := filepath.Join(tmpDir, ".grove")
		if err := os.MkdirAll(groveDir, 0755); err != nil {
			t.Fatalf("failed to create .grove dir: %v", err)
		}

		oldWd, _ := os.Getwd()
		defer os.Chdir(oldWd)
		os.Chdir(tmpDir)

		root, err := ProjectRoot()
		if err != nil {
			t.Fatalf("ProjectRoot() error = %v", err)
		}
		if root != tmpDir {
			t.Errorf("ProjectRoot() = %q, want %q", root, tmpDir)
		}
	})

	t.Run("returns empty when not in grove project", func(t *testing.T) {
		tmpDir := t.TempDir()

		oldWd, _ := os.Getwd()
		defer os.Chdir(oldWd)
		os.Chdir(tmpDir)

		root, err := ProjectRoot()
		if err != nil {
			t.Fatalf("ProjectRoot() error = %v", err)
		}
		if root != "" {
			t.Errorf("ProjectRoot() = %q, want empty", root)
		}
	})
}
