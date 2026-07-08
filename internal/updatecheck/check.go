package updatecheck

import (
	"context"
	"fmt"
	"io"
	"time"
)

// upToDateMessage is the format string for the "no update needed" CheckNow output.
// Format takes one argument: the current version string.
const upToDateMessage = "grove is up to date (%s)\n"

// MaybeNotify reads the default cache and writes a notification to w if a
// newer release is available. Always non-blocking. No-op on missing/corrupt
// cache or when the user is already at or ahead of the cached latest.
func MaybeNotify(w io.Writer, currentVersion string) {
	maybeNotifyFromPath(w, currentVersion, DefaultCachePath())
}

func maybeNotifyFromPath(w io.Writer, currentVersion, path string) {
	c, err := ReadCacheFromPath(path)
	if err != nil || c.LatestVersion == "" {
		return
	}
	renderUpdateBox(w, currentVersion, c.LatestVersion, c.LatestURL)
}

// renderUpdateBox computes severity and writes a notification box if newer.
// Returns true if a box was written, false if no update is available.
func renderUpdateBox(w io.Writer, currentVersion, latestVersion, latestURL string) bool {
	severity := CompareSemver(currentVersion, latestVersion)
	if severity == SeverityNone {
		return false
	}
	method := DetectInstall()
	_, _ = io.WriteString(w, RenderBox(currentVersion, latestVersion, latestURL, UpdateLabel(method), UpdateCommand(method), severity))
	return true
}

// CachedUpdateAnnotation returns a parenthetical annotation like
// " (update available: 0.7.0)" when the cache indicates a newer version is
// available. Returns "" otherwise (no cache, equal or older version).
//
// Use case: annotating `grove version` output so users see at a glance
// whether their installed binary is behind.
func CachedUpdateAnnotation(currentVersion string) string {
	return cachedUpdateAnnotationFromPath(currentVersion, DefaultCachePath())
}

func cachedUpdateAnnotationFromPath(currentVersion, path string) string {
	c, err := ReadCacheFromPath(path)
	if err != nil || c.LatestVersion == "" {
		return ""
	}
	if CompareSemver(currentVersion, c.LatestVersion) == SeverityNone {
		return ""
	}
	return " (update available: " + c.LatestVersion + ")"
}

// CachedRelease returns the cached LatestVersion + LatestURL when a newer
// release is available, or zero strings when there's nothing to surface.
// Used by UI surfaces (TUI footer badge, modal, version annotation) that
// need both fields without re-implementing the cache+compare flow.
func CachedRelease(currentVersion string) (latest, url string, available bool) {
	return cachedReleaseFromPath(currentVersion, DefaultCachePath())
}

func cachedReleaseFromPath(currentVersion, path string) (latest, url string, available bool) {
	c, err := ReadCacheFromPath(path)
	if err != nil || c.LatestVersion == "" {
		return "", "", false
	}
	if CompareSemver(currentVersion, c.LatestVersion) == SeverityNone {
		return "", "", false
	}
	return c.LatestVersion, c.LatestURL, true
}

// CheckInterval is the default minimum gap between two refreshes.
const CheckInterval = 24 * time.Hour

// RefreshWaitBudget caps how long a short-lived CLI command should wait on
// RefreshAsync's channel before exiting. When the cache is fresh the refresh
// returns immediately, so the cap only bites on the (at most daily) run that
// actually hits the network — keeping commands within the <500ms budget.
const RefreshWaitBudget = 300 * time.Millisecond

// RefreshAsync starts a goroutine that refreshes the cache if the interval
// has elapsed, and returns a channel that is closed when the refresh
// completes (or was skipped). Never blocks. Failures are silent.
//
// Short-lived CLI processes MUST wait (bounded, e.g. RefreshWaitBudget) on
// the returned channel: goroutines die when main returns, so a fire-and-
// forget refresh would be killed before the HTTPS round trip finishes and
// the cache would never be written. Long-lived callers (the TUI) can ignore
// the channel.
func RefreshAsync() <-chan struct{} {
	done := make(chan struct{})
	go func() {
		defer close(done)
		refreshFromPathWithFetcher(DefaultCachePath(), CheckInterval, func() (Release, error) {
			return FetchLatest(context.Background())
		})
	}()
	return done
}

// refreshFromPathWithFetcher is the testable core: deterministic, no goroutine,
// no global state. Reads cache, checks interval, calls fetcher, writes cache.
func refreshFromPathWithFetcher(path string, interval time.Duration, fetcher func() (Release, error)) {
	c, _ := ReadCacheFromPath(path)
	if !c.LastCheckedAt.IsZero() && time.Since(c.LastCheckedAt) < interval {
		return
	}
	rel, err := fetcher()
	if err != nil {
		// Record the attempt even on failure, preserving any previously
		// cached release info. Otherwise a host where the fetch always
		// fails or times out (offline, firewalled, slow DNS/TLS) never
		// gets a written cache, so every subsequent command re-attempts
		// the network fetch and blocks for the full RefreshWaitBudget
		// instead of the interval applying to failures too.
		c.LastCheckedAt = time.Now()
		_ = WriteCacheToPath(path, c)
		return
	}
	_ = WriteCacheToPath(path, Cache{
		LastCheckedAt: time.Now(),
		LatestVersion: rel.Version(),
		LatestURL:     rel.HTMLURL,
	})
}

// CheckNow synchronously fetches the latest release and prints the result to w.
// Bypasses the cache and the interval for the fetch, but seeds the cache with
// the result so later commands' MaybeNotify works from it. Used by --check-update.
func CheckNow(w io.Writer, currentVersion string) error {
	return checkNowWithDeps(w, currentVersion, DefaultCachePath(), func() (Release, error) {
		return FetchLatest(context.Background())
	})
}

func checkNowWithDeps(w io.Writer, currentVersion, cachePath string, fetcher func() (Release, error)) error {
	rel, err := fetcher()
	if err != nil {
		return err
	}
	// Write-through to the cache: without this, a manual --check-update
	// couldn't seed the cache for later notifications. Failure is non-fatal —
	// the synchronous output below is the primary result.
	if cachePath != "" {
		_ = WriteCacheToPath(cachePath, Cache{
			LastCheckedAt: time.Now(),
			LatestVersion: rel.Version(),
			LatestURL:     rel.HTMLURL,
		})
	}
	if !renderUpdateBox(w, currentVersion, rel.Version(), rel.HTMLURL) {
		_, _ = fmt.Fprintf(w, upToDateMessage, currentVersion)
	}
	return nil
}
