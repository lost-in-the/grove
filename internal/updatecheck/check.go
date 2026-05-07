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
	_, _ = io.WriteString(w, RenderBox(currentVersion, latestVersion, latestURL, UpdateCommand(method), severity))
	return true
}

// CheckInterval is the default minimum gap between two refreshes.
const CheckInterval = 24 * time.Hour

// RefreshAsync starts a detached goroutine that refreshes the cache if the
// interval has elapsed. Never blocks. Failures are silent.
func RefreshAsync() {
	go refreshFromPathWithFetcher(DefaultCachePath(), CheckInterval, func() (Release, error) {
		return FetchLatest(context.Background())
	})
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
		return
	}
	_ = WriteCacheToPath(path, Cache{
		LastCheckedAt: time.Now(),
		LatestVersion: rel.Version(),
		LatestURL:     rel.HTMLURL,
	})
}

// CheckNow synchronously fetches the latest release and prints the result to w.
// Bypasses the cache and the interval. Used by --check-update.
func CheckNow(w io.Writer, currentVersion string) error {
	return checkNowWithDeps(w, currentVersion, func() (Release, error) {
		return FetchLatest(context.Background())
	})
}

func checkNowWithDeps(w io.Writer, currentVersion string, fetcher func() (Release, error)) error {
	rel, err := fetcher()
	if err != nil {
		return err
	}
	if !renderUpdateBox(w, currentVersion, rel.Version(), rel.HTMLURL) {
		_, _ = fmt.Fprintf(w, upToDateMessage, currentVersion)
	}
	return nil
}
