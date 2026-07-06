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

func TestRefresh_FetchFailureRecordsAttempt(t *testing.T) {
	// Regression: a failed fetch never wrote LastCheckedAt, so on hosts where
	// the fetch always fails (offline, firewalled, >RefreshWaitBudget latency)
	// the cache was never seeded and EVERY subsequent command re-attempted the
	// network fetch — blocking the full RefreshWaitBudget each time instead of
	// at most once per interval.
	dir := t.TempDir()
	path := filepath.Join(dir, "update-check.json")

	calls := 0
	fetcher := func() (Release, error) {
		calls++
		return Release{}, errFakeNetwork
	}

	// First refresh with no cache at all: fetch fails, attempt must be recorded.
	refreshFromPathWithFetcher(path, 24*time.Hour, fetcher)
	if calls != 1 {
		t.Fatalf("fetcher calls = %d, want 1", calls)
	}
	got, err := ReadCacheFromPath(path)
	if err != nil {
		t.Fatalf("cache not readable after failed fetch: %v", err)
	}
	if got.LastCheckedAt.IsZero() {
		t.Fatal("LastCheckedAt is zero after failed fetch; attempt was not recorded")
	}

	// Second refresh within the interval must not hit the network again.
	refreshFromPathWithFetcher(path, 24*time.Hour, fetcher)
	if calls != 1 {
		t.Errorf("fetcher calls = %d after second refresh, want 1 (attempt timestamp should suppress retry)", calls)
	}
}

func TestRefresh_FetchFailurePreservesReleaseInfoWithAttempt(t *testing.T) {
	// A failed fetch must record the attempt WITHOUT discarding previously
	// cached release info — MaybeNotify still needs it.
	dir := t.TempDir()
	path := filepath.Join(dir, "update-check.json")
	_ = WriteCacheToPath(path, Cache{
		LastCheckedAt: time.Now().Add(-48 * time.Hour),
		LatestVersion: "0.5.0",
		LatestURL:     "https://example/old",
	})

	fetcher := func() (Release, error) { return Release{}, errFakeNetwork }
	refreshFromPathWithFetcher(path, 24*time.Hour, fetcher)

	got, _ := ReadCacheFromPath(path)
	if got.LatestVersion != "0.5.0" || got.LatestURL != "https://example/old" {
		t.Errorf("release info clobbered on failed fetch: got %+v", got)
	}
	if time.Since(got.LastCheckedAt) > time.Minute {
		t.Errorf("LastCheckedAt not refreshed on failed fetch: %v", got.LastCheckedAt)
	}
}

func TestCheckNow_FetchesAndPrintsNotification(t *testing.T) {
	fetcher := func() (Release, error) {
		return Release{TagName: "v0.6.0", HTMLURL: "https://x"}, nil
	}
	var buf bytes.Buffer
	if err := checkNowWithDeps(&buf, "0.5.0", "", fetcher); err != nil {
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
	if err := checkNowWithDeps(&buf, "0.5.0", "", fetcher); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(buf.String(), "up to date") {
		t.Errorf("expected 'up to date' message, got: %q", buf.String())
	}
}

func TestCheckNow_FetchFailureReturnsError(t *testing.T) {
	fetcher := func() (Release, error) { return Release{}, errFakeNetwork }
	var buf bytes.Buffer
	if err := checkNowWithDeps(&buf, "0.5.0", "", fetcher); err == nil {
		t.Error("expected error on fetch failure")
	}
}

func TestCachedUpdateAnnotation_NewerCachedVersionReturnsAnnotation(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "update-check.json")
	_ = WriteCacheToPath(path, Cache{LatestVersion: "0.7.0", LatestURL: "https://x"})

	got := cachedUpdateAnnotationFromPath("0.6.0", path)
	want := " (update available: 0.7.0)"
	if got != want {
		t.Errorf("annotation = %q, want %q", got, want)
	}
}

func TestCachedUpdateAnnotation_EqualVersionReturnsEmpty(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "update-check.json")
	_ = WriteCacheToPath(path, Cache{LatestVersion: "0.6.0"})

	got := cachedUpdateAnnotationFromPath("0.6.0", path)
	if got != "" {
		t.Errorf("expected empty for equal version, got %q", got)
	}
}

func TestCachedUpdateAnnotation_MissingCacheReturnsEmpty(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "missing.json")

	got := cachedUpdateAnnotationFromPath("0.6.0", path)
	if got != "" {
		t.Errorf("expected empty for missing cache, got %q", got)
	}
}

func TestCachedUpdateAnnotation_OlderCachedVersionReturnsEmpty(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "update-check.json")
	_ = WriteCacheToPath(path, Cache{LatestVersion: "0.5.0"})

	got := cachedUpdateAnnotationFromPath("0.6.0", path)
	if got != "" {
		t.Errorf("expected empty for older cached version, got %q", got)
	}
}

func TestCachedRelease_NewerCachedVersionReturnsBoth(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "update-check.json")
	_ = WriteCacheToPath(path, Cache{
		LatestVersion: "0.7.0",
		LatestURL:     "https://github.com/lost-in-the/grove/releases/tag/v0.7.0",
	})

	latest, url, available := cachedReleaseFromPath("0.6.0", path)
	if !available {
		t.Fatal("expected available=true for newer cached version")
	}
	if latest != "0.7.0" {
		t.Errorf("latest = %q, want %q", latest, "0.7.0")
	}
	if url != "https://github.com/lost-in-the/grove/releases/tag/v0.7.0" {
		t.Errorf("url = %q, want release URL", url)
	}
}

func TestCachedRelease_EqualVersionUnavailable(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "update-check.json")
	_ = WriteCacheToPath(path, Cache{LatestVersion: "0.6.0", LatestURL: "https://x"})

	latest, url, available := cachedReleaseFromPath("0.6.0", path)
	if available {
		t.Error("expected available=false for equal version")
	}
	if latest != "" || url != "" {
		t.Errorf("expected zero values, got latest=%q url=%q", latest, url)
	}
}

func TestCachedRelease_MissingCacheUnavailable(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "missing.json")

	_, _, available := cachedReleaseFromPath("0.6.0", path)
	if available {
		t.Error("expected available=false for missing cache")
	}
}

func TestCachedRelease_OlderCachedVersionUnavailable(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "update-check.json")
	_ = WriteCacheToPath(path, Cache{LatestVersion: "0.5.0", LatestURL: "https://x"})

	_, _, available := cachedReleaseFromPath("0.6.0", path)
	if available {
		t.Error("expected available=false when current is newer than cached")
	}
}

// TestCachedRelease_DevVersionUnavailable locks the contract that pre-release
// or otherwise unparseable currentVersion strings (e.g. "0.7.0-dev") never
// surface an update. CompareSemver returns SeverityNone for unparseable inputs,
// which intentionally suppresses the badge/modal for development builds.
func TestCachedRelease_DevVersionUnavailable(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "update-check.json")
	_ = WriteCacheToPath(path, Cache{
		LatestVersion: "0.8.0",
		LatestURL:     "https://github.com/lost-in-the/grove/releases/tag/v0.8.0",
	})

	latest, url, available := cachedReleaseFromPath("0.7.0-dev", path)
	if available {
		t.Errorf("dev version should not show update available, got latest=%q url=%q", latest, url)
	}
	if latest != "" || url != "" {
		t.Errorf("expected zero values for dev version, got latest=%q url=%q", latest, url)
	}
}

func TestCheckNow_SeedsCache(t *testing.T) {
	// Regression: --check-update fetched synchronously but never wrote the
	// cache, so a manual check couldn't seed later MaybeNotify calls.
	dir := t.TempDir()
	path := filepath.Join(dir, "update-check.json")
	fetcher := func() (Release, error) {
		return Release{TagName: "v0.6.0", HTMLURL: "https://x"}, nil
	}

	var buf bytes.Buffer
	if err := checkNowWithDeps(&buf, "0.5.0", path, fetcher); err != nil {
		t.Fatalf("checkNowWithDeps: %v", err)
	}

	c, err := ReadCacheFromPath(path)
	if err != nil {
		t.Fatalf("cache not written: %v", err)
	}
	if c.LatestVersion != "0.6.0" {
		t.Errorf("cached LatestVersion = %q, want %q", c.LatestVersion, "0.6.0")
	}
	if c.LastCheckedAt.IsZero() {
		t.Error("cached LastCheckedAt is zero")
	}
}
