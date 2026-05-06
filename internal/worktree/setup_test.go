package worktree

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lost-in-the/grove/internal/config"
)

func TestSetupFiles_NilConfig(t *testing.T) {
	err := SetupFiles(nil, "/tmp/new", "/tmp/main")
	if err != nil {
		t.Errorf("SetupFiles(nil, ...) = %v, want nil", err)
	}
}

func TestSetupFiles_CopyFiles(t *testing.T) {
	mainPath := t.TempDir()
	newPath := t.TempDir()

	// Create source file with parent directory
	_ = os.MkdirAll(filepath.Join(mainPath, "config"), 0755)
	_ = os.WriteFile(filepath.Join(mainPath, "config", "secret.key"), []byte("secret"), 0600)

	ext := &config.ExternalComposeConfig{
		CopyFiles: []string{"config/secret.key"},
	}

	err := SetupFiles(ext, newPath, mainPath)
	if err != nil {
		t.Fatalf("SetupFiles() error = %v", err)
	}

	data, err := os.ReadFile(filepath.Join(newPath, "config", "secret.key"))
	if err != nil {
		t.Fatalf("Failed to read copied file: %v", err)
	}
	if string(data) != "secret" {
		t.Errorf("Expected 'secret', got %q", string(data))
	}

	// Verify it's a regular file (not a symlink)
	info, err := os.Lstat(filepath.Join(newPath, "config", "secret.key"))
	if err != nil {
		t.Fatalf("Lstat failed: %v", err)
	}
	if info.Mode()&os.ModeSymlink != 0 {
		t.Error("Expected a regular file (copy), not a symlink")
	}
}

func TestSetupFiles_SymlinkFilesAndDirs(t *testing.T) {
	mainPath := t.TempDir()
	newPath := t.TempDir()

	// Create source file and directory
	_ = os.MkdirAll(filepath.Join(mainPath, "config"), 0755)
	_ = os.WriteFile(filepath.Join(mainPath, "config", "dev.key"), []byte("devkey"), 0600)
	_ = os.MkdirAll(filepath.Join(mainPath, "vendor", "bundle"), 0755)

	ext := &config.ExternalComposeConfig{
		SymlinkFiles: []string{"config/dev.key"},
		SymlinkDirs:  []string{"vendor/bundle"},
	}

	err := SetupFiles(ext, newPath, mainPath)
	if err != nil {
		t.Fatalf("SetupFiles() error = %v", err)
	}

	// Verify symlinked file
	fileInfo, err := os.Lstat(filepath.Join(newPath, "config", "dev.key"))
	if err != nil {
		t.Fatalf("Failed to stat symlinked file: %v", err)
	}
	if fileInfo.Mode()&os.ModeSymlink == 0 {
		t.Error("Expected config/dev.key to be a symlink")
	}

	// Verify symlinked directory
	dirInfo, err := os.Lstat(filepath.Join(newPath, "vendor", "bundle"))
	if err != nil {
		t.Fatalf("Failed to stat symlinked dir: %v", err)
	}
	if dirInfo.Mode()&os.ModeSymlink == 0 {
		t.Error("Expected vendor/bundle to be a symlink")
	}

	// Verify the symlink target points back to main path
	target, err := os.Readlink(filepath.Join(newPath, "config", "dev.key"))
	if err != nil {
		t.Fatalf("Readlink failed: %v", err)
	}
	if target != filepath.Join(mainPath, "config", "dev.key") {
		t.Errorf("Symlink target = %q, want %q", target, filepath.Join(mainPath, "config", "dev.key"))
	}
}

func TestSetupFiles_MissingSource(t *testing.T) {
	mainPath := t.TempDir()
	newPath := t.TempDir()

	ext := &config.ExternalComposeConfig{
		CopyFiles: []string{"nonexistent.key"},
	}

	err := SetupFiles(ext, newPath, mainPath)
	if err == nil {
		t.Error("Expected error for missing source file, got nil")
	}
}

func TestSetupFiles_MissingSourceContinues(t *testing.T) {
	mainPath := t.TempDir()
	newPath := t.TempDir()

	// Create one valid source file, one missing
	_ = os.MkdirAll(filepath.Join(mainPath, "config"), 0755)
	_ = os.WriteFile(filepath.Join(mainPath, "config", "valid.key"), []byte("valid"), 0600)

	ext := &config.ExternalComposeConfig{
		CopyFiles: []string{"nonexistent.key", "config/valid.key"},
	}

	err := SetupFiles(ext, newPath, mainPath)
	// Should return the first error but still process remaining entries
	if err == nil {
		t.Error("Expected error for missing source file, got nil")
	}

	// The valid file should still have been copied despite the earlier failure
	data, readErr := os.ReadFile(filepath.Join(newPath, "config", "valid.key"))
	if readErr != nil {
		t.Fatalf("Valid file was not copied despite firstErr behavior: %v", readErr)
	}
	if string(data) != "valid" {
		t.Errorf("Expected 'valid', got %q", string(data))
	}
}

func TestSetupFiles_SymlinkMissingSource(t *testing.T) {
	mainPath := t.TempDir()
	newPath := t.TempDir()

	ext := &config.ExternalComposeConfig{
		SymlinkFiles: []string{"nonexistent.key"},
	}

	err := SetupFiles(ext, newPath, mainPath)
	if err == nil {
		t.Error("Expected error for missing symlink source, got nil")
	}
	if !strings.Contains(err.Error(), "source not found") {
		t.Errorf("Expected 'source not found' in error, got %q", err.Error())
	}
}

func TestSetupFiles_PathTraversal(t *testing.T) {
	mainPath := t.TempDir()
	newPath := t.TempDir()

	t.Run("legitimate symlink_files entry succeeds", func(t *testing.T) {
		_ = os.WriteFile(filepath.Join(mainPath, "config.yml"), []byte("ok"), 0644)
		ext := &config.ExternalComposeConfig{
			SymlinkFiles: []string{"config.yml"},
		}
		err := SetupFiles(ext, newPath, mainPath)
		if err != nil {
			t.Errorf("SetupFiles() legitimate entry = %v, want nil", err)
		}
	})

	t.Run("dotdot in copy_files rejected", func(t *testing.T) {
		ext := &config.ExternalComposeConfig{
			CopyFiles: []string{"../../etc/passwd"},
		}
		err := SetupFiles(ext, newPath, mainPath)
		if err == nil {
			t.Error("SetupFiles() with traversal copy_files = nil, want error")
		}
		if !strings.Contains(err.Error(), "escapes base directory") {
			t.Errorf("expected 'escapes base directory' in error, got %q", err.Error())
		}
	})

	t.Run("dotdot in symlink_files rejected", func(t *testing.T) {
		ext := &config.ExternalComposeConfig{
			SymlinkFiles: []string{"../../.ssh/id_rsa"},
		}
		err := SetupFiles(ext, newPath, mainPath)
		if err == nil {
			t.Error("SetupFiles() with traversal symlink_files = nil, want error")
		}
		if !strings.Contains(err.Error(), "escapes base directory") {
			t.Errorf("expected 'escapes base directory' in error, got %q", err.Error())
		}
	})

	t.Run("dotdot in symlink_dirs rejected", func(t *testing.T) {
		ext := &config.ExternalComposeConfig{
			SymlinkDirs: []string{"../../../secret_dir"},
		}
		err := SetupFiles(ext, newPath, mainPath)
		if err == nil {
			t.Error("SetupFiles() with traversal symlink_dirs = nil, want error")
		}
		if !strings.Contains(err.Error(), "escapes base directory") {
			t.Errorf("expected 'escapes base directory' in error, got %q", err.Error())
		}
	})
}
