package updatecheck

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestCache_RoundtripJSON(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "update-check.json")
	c := Cache{
		Version:       1,
		LastCheckedAt: time.Date(2026, 5, 7, 12, 34, 56, 0, time.UTC),
		LatestVersion: "0.6.0",
		LatestURL:     "https://github.com/lost-in-the/grove/releases/tag/v0.6.0",
	}
	if err := WriteCacheToPath(path, c); err != nil {
		t.Fatalf("WriteCacheToPath: %v", err)
	}
	got, err := ReadCacheFromPath(path)
	if err != nil {
		t.Fatalf("ReadCacheFromPath: %v", err)
	}
	if got != c {
		t.Errorf("roundtrip mismatch:\n got  %+v\n want %+v", got, c)
	}
}

func TestCache_MissingFileReturnsZeroValue(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "does-not-exist.json")
	got, err := ReadCacheFromPath(path)
	if err != nil {
		t.Fatalf("ReadCacheFromPath should not error on missing file: %v", err)
	}
	if got != (Cache{}) {
		t.Errorf("missing file should yield zero Cache, got %+v", got)
	}
}

func TestCache_CorruptFileReturnsZeroValue(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "update-check.json")
	if err := os.WriteFile(path, []byte("not json {{{"), 0o644); err != nil {
		t.Fatalf("seed: %v", err)
	}
	got, err := ReadCacheFromPath(path)
	if err != nil {
		t.Fatalf("ReadCacheFromPath should swallow corrupt JSON: %v", err)
	}
	if got != (Cache{}) {
		t.Errorf("corrupt file should yield zero Cache, got %+v", got)
	}
}

func TestCache_AtomicWriteCleansUpTempFiles(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "update-check.json")
	c := Cache{Version: 1, LatestVersion: "0.6.0"}
	if err := WriteCacheToPath(path, c); err != nil {
		t.Fatalf("WriteCacheToPath: %v", err)
	}
	// Final file should exist
	if _, err := os.Stat(path); err != nil {
		t.Errorf("expected final file at %s: %v", path, err)
	}
	// No leftover *.tmp files in the directory
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("ReadDir: %v", err)
	}
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".tmp") {
			t.Errorf("unexpected leftover tmp file: %s", e.Name())
		}
	}
}

func TestCache_DefaultPathUnderHome(t *testing.T) {
	t.Setenv("HOME", "/tmp/fake-home-for-test")
	got := DefaultCachePath()
	want := "/tmp/fake-home-for-test/.grove/update-check.json"
	if got != want {
		t.Errorf("DefaultCachePath() = %q, want %q", got, want)
	}
}
