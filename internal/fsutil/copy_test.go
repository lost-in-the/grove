package fsutil

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCopyFile(t *testing.T) {
	t.Run("copies content and permissions", func(t *testing.T) {
		dir := t.TempDir()
		src := filepath.Join(dir, "src.txt")
		dst := filepath.Join(dir, "dst.txt")

		if err := os.WriteFile(src, []byte("hello"), 0644); err != nil {
			t.Fatal(err)
		}

		if err := CopyFile(src, dst); err != nil {
			t.Fatalf("CopyFile: %v", err)
		}

		got, err := os.ReadFile(dst)
		if err != nil {
			t.Fatalf("read dst: %v", err)
		}
		if string(got) != "hello" {
			t.Errorf("content = %q, want %q", got, "hello")
		}

		srcInfo, _ := os.Stat(src)
		dstInfo, _ := os.Stat(dst)
		if srcInfo.Mode() != dstInfo.Mode() {
			t.Errorf("mode = %v, want %v", dstInfo.Mode(), srcInfo.Mode())
		}
	})

	t.Run("creates parent directories", func(t *testing.T) {
		dir := t.TempDir()
		src := filepath.Join(dir, "src.txt")
		dst := filepath.Join(dir, "a", "b", "dst.txt")

		if err := os.WriteFile(src, []byte("nested"), 0644); err != nil {
			t.Fatal(err)
		}

		if err := CopyFile(src, dst); err != nil {
			t.Fatalf("CopyFile: %v", err)
		}

		got, err := os.ReadFile(dst)
		if err != nil {
			t.Fatalf("read dst: %v", err)
		}
		if string(got) != "nested" {
			t.Errorf("content = %q, want %q", got, "nested")
		}
	})

	t.Run("nonexistent source returns error", func(t *testing.T) {
		dir := t.TempDir()
		err := CopyFile(filepath.Join(dir, "nope"), filepath.Join(dir, "dst"))
		if err == nil {
			t.Fatal("expected error for nonexistent source")
		}
	})

	t.Run("overwrites existing destination", func(t *testing.T) {
		dir := t.TempDir()
		src := filepath.Join(dir, "src.txt")
		dst := filepath.Join(dir, "dst.txt")

		if err := os.WriteFile(src, []byte("new"), 0644); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(dst, []byte("old-content-that-is-longer"), 0644); err != nil {
			t.Fatal(err)
		}

		if err := CopyFile(src, dst); err != nil {
			t.Fatalf("CopyFile: %v", err)
		}

		got, err := os.ReadFile(dst)
		if err != nil {
			t.Fatalf("read dst: %v", err)
		}
		if string(got) != "new" {
			t.Errorf("content = %q, want %q", got, "new")
		}
	})
}
