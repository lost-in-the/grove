package updatecheck

import (
	"bytes"
	"fmt"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestMaybeNotify_NewerCachedVersionPrintsBox(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "update-check.json")
	_ = WriteCacheToPath(path, Cache{
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

func TestRefresh_WithinIntervalSkips(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "update-check.json")
	original := Cache{
		LastCheckedAt: time.Now(), // brand new
		LatestVersion: "0.5.0",
		LatestURL:     "https://example/old",
	}
	_ = WriteCacheToPath(path, original)

	called := false
	fetcher := func() (Release, error) {
		called = true
		return Release{TagName: "v9.9.9", HTMLURL: "https://example/new"}, nil
	}
	refreshFromPathWithFetcher(path, time.Hour, fetcher)
	if called {
		t.Error("fetcher should not be called within the interval")
	}
	got, _ := ReadCacheFromPath(path)
	if got.LatestVersion != "0.5.0" {
		t.Errorf("cache should be unchanged, got LatestVersion=%q", got.LatestVersion)
	}
}

func TestRefresh_BeyondIntervalUpdatesCache(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "update-check.json")
	_ = WriteCacheToPath(path, Cache{
		LastCheckedAt: time.Now().Add(-48 * time.Hour),
		LatestVersion: "0.5.0",
	})

	fetcher := func() (Release, error) {
		return Release{TagName: "v0.6.0", HTMLURL: "https://example/new"}, nil
	}
	refreshFromPathWithFetcher(path, 24*time.Hour, fetcher)

	got, err := ReadCacheFromPath(path)
	if err != nil {
		t.Fatal(err)
	}
	if got.LatestVersion != "0.6.0" {
		t.Errorf("LatestVersion = %q, want %q", got.LatestVersion, "0.6.0")
	}
	if got.LatestURL != "https://example/new" {
		t.Errorf("LatestURL not updated: %q", got.LatestURL)
	}
}

func TestRefresh_FetchFailureLeavesCacheAlone(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "update-check.json")
	original := Cache{
		LastCheckedAt: time.Now().Add(-48 * time.Hour),
		LatestVersion: "0.5.0",
		LatestURL:     "https://example/old",
	}
	_ = WriteCacheToPath(path, original)

	fetcher := func() (Release, error) {
		return Release{}, errFakeNetwork
	}
	refreshFromPathWithFetcher(path, 24*time.Hour, fetcher)

	got, _ := ReadCacheFromPath(path)
	// Compare ignoring LastCheckedAt (atomic write may have nanosecond drift).
	// On fetch failure, LatestVersion + LatestURL must be preserved.
	if got.LatestVersion != original.LatestVersion {
		t.Errorf("LatestVersion changed: got %q, want %q", got.LatestVersion, original.LatestVersion)
	}
	if got.LatestURL != original.LatestURL {
		t.Errorf("LatestURL changed: got %q, want %q", got.LatestURL, original.LatestURL)
	}
}

var errFakeNetwork = fmt.Errorf("simulated network failure")

func TestCheckNow_FetchesAndPrintsNotification(t *testing.T) {
	fetcher := func() (Release, error) {
		return Release{TagName: "v0.6.0", HTMLURL: "https://x"}, nil
	}
	var buf bytes.Buffer
	if err := checkNowWithDeps(&buf, "0.5.0", fetcher); err != nil {
		t.Fatalf("checkNowWithDeps: %v", err)
	}
	if !strings.Contains(buf.String(), "0.6.0") {
		t.Errorf("expected '0.6.0' in output, got:\n%s", buf.String())
	}
}

func TestCheckNow_UpToDateMessage(t *testing.T) {
	fetcher := func() (Release, error) {
		return Release{TagName: "v0.5.0", HTMLURL: "https://x"}, nil
	}
	var buf bytes.Buffer
	if err := checkNowWithDeps(&buf, "0.5.0", fetcher); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(buf.String(), "up to date") {
		t.Errorf("expected 'up to date' message, got: %q", buf.String())
	}
}

func TestCheckNow_FetchFailureReturnsError(t *testing.T) {
	fetcher := func() (Release, error) { return Release{}, errFakeNetwork }
	var buf bytes.Buffer
	if err := checkNowWithDeps(&buf, "0.5.0", fetcher); err == nil {
		t.Error("expected error on fetch failure")
	}
}
