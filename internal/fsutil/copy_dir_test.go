package fsutil

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCopyFile_RejectsDirectorySource(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "srcdir")
	dst := filepath.Join(dir, "out", "dst")
	if err := os.MkdirAll(src, 0o755); err != nil {
		t.Fatal(err)
	}

	if err := CopyFile(src, dst); err == nil {
		t.Fatal("CopyFile of a directory = nil, want error")
	}
	if _, err := os.Lstat(dst); !os.IsNotExist(err) {
		t.Errorf("destination exists after a rejected copy (stat err = %v), want none", err)
	}
	if _, err := os.Lstat(filepath.Dir(dst)); !os.IsNotExist(err) {
		t.Errorf("destination parent directory was created for a rejected copy")
	}
}
