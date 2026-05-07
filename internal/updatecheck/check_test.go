package updatecheck

import (
	"bytes"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestMaybeNotify_NewerCachedVersionPrintsBox(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "update-check.json")
	_ = WriteCacheToPath(path, Cache{
		Version:       1,
		LastCheckedAt: time.Now().Add(-1 * time.Hour),
		LatestVersion: "0.6.0",
		LatestURL:     "https://github.com/lost-in-the/grove/releases/tag/v0.6.0",
	})

	var buf bytes.Buffer
	maybeNotifyFromPath(&buf, "0.5.0", path)

	if !strings.Contains(buf.String(), "Update available") {
		t.Errorf("expected 'Update available' in output, got:\n%s", buf.String())
	}
	if !strings.Contains(buf.String(), "0.6.0") {
		t.Errorf("expected '0.6.0' in output, got:\n%s", buf.String())
	}
}

func TestMaybeNotify_SameVersionNoOutput(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "update-check.json")
	_ = WriteCacheToPath(path, Cache{LatestVersion: "0.6.0"})

	var buf bytes.Buffer
	maybeNotifyFromPath(&buf, "0.6.0", path)

	if buf.Len() != 0 {
		t.Errorf("expected no output when versions match, got: %q", buf.String())
	}
}

func TestMaybeNotify_MissingCacheNoOutput(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "missing.json")

	var buf bytes.Buffer
	maybeNotifyFromPath(&buf, "0.5.0", path)

	if buf.Len() != 0 {
		t.Errorf("expected no output with missing cache, got: %q", buf.String())
	}
}

func TestMaybeNotify_OlderCachedVersionNoOutput(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "update-check.json")
	_ = WriteCacheToPath(path, Cache{LatestVersion: "0.4.0"})

	var buf bytes.Buffer
	maybeNotifyFromPath(&buf, "0.5.0", path)

	if buf.Len() != 0 {
		t.Errorf("expected no output when current is newer than cached latest, got: %q", buf.String())
	}
}
