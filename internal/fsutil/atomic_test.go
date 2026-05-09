package fsutil

import (
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

func TestAtomicWriteFile_Roundtrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "data.txt")
	want := []byte("hello world\n")

	if err := AtomicWriteFile(path, want, 0o644); err != nil {
		t.Fatalf("AtomicWriteFile() error = %v", err)
	}

	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	if string(got) != string(want) {
		t.Errorf("contents = %q, want %q", got, want)
	}
}

func TestAtomicWriteFile_NoLeftoverTempFiles(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "data.txt")

	if err := AtomicWriteFile(path, []byte("payload"), 0o644); err != nil {
		t.Fatalf("AtomicWriteFile() error = %v", err)
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("ReadDir() error = %v", err)
	}
	for _, e := range entries {
		if e.Name() == "data.txt" {
			continue
		}
		if strings.Contains(e.Name(), ".tmp") {
			t.Errorf("leftover temp file %q after successful write", e.Name())
		}
	}
}

func TestAtomicWriteFile_ConcurrentWritersBothSucceed(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "shared.txt")

	var wg sync.WaitGroup
	errs := make(chan error, 2)

	wg.Add(2)
	go func() {
		defer wg.Done()
		errs <- AtomicWriteFile(path, []byte("writer-a-content"), 0o644)
	}()
	go func() {
		defer wg.Done()
		errs <- AtomicWriteFile(path, []byte("writer-b-content"), 0o644)
	}()
	wg.Wait()
	close(errs)

	for err := range errs {
		if err != nil {
			t.Fatalf("concurrent AtomicWriteFile() error = %v", err)
		}
	}

	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	// One writer must have won cleanly — the contents must be one of the two
	// payloads, never an interleaved blend.
	s := string(got)
	if s != "writer-a-content" && s != "writer-b-content" {
		t.Errorf("contents = %q, expected one full payload", s)
	}

	// And no temp files should be left around.
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("ReadDir() error = %v", err)
	}
	for _, e := range entries {
		if e.Name() == "shared.txt" {
			continue
		}
		if strings.Contains(e.Name(), ".tmp") {
			t.Errorf("leftover temp file %q after concurrent writes", e.Name())
		}
	}
}

func TestAtomicWriteFile_CreatesMissingParentDir(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "nested", "deeper", "data.txt")

	if err := AtomicWriteFile(path, []byte("x"), 0o644); err != nil {
		t.Fatalf("AtomicWriteFile() error = %v", err)
	}

	if _, err := os.Stat(path); err != nil {
		t.Errorf("expected file at %s, got stat error: %v", path, err)
	}
}

func TestAtomicWriteFile_HonorsRequestedMode(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "data.txt")

	if err := AtomicWriteFile(path, []byte("x"), 0o600); err != nil {
		t.Fatalf("AtomicWriteFile() error = %v", err)
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("Stat() error = %v", err)
	}
	// Mask out non-permission bits before comparing (umask doesn't apply to
	// chmod, but be explicit about what we're checking).
	gotPerm := info.Mode().Perm()
	if gotPerm != 0o600 {
		t.Errorf("perm = %o, want %o", gotPerm, 0o600)
	}
}
