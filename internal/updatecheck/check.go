package updatecheck

import (
	"context"
	"io"
	"time"
)

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
	if CompareSemver(currentVersion, c.LatestVersion) == SeverityNone {
		return
	}
	method := DetectInstall()
	box := RenderBox(currentVersion, c.LatestVersion, c.LatestURL, UpdateCommand(method))
	_, _ = io.WriteString(w, box)
}

// CheckInterval is the default minimum gap between two refreshes.
const CheckInterval = 24 * time.Hour

// RefreshAsync starts a detached goroutine that refreshes the cache if the
// interval has elapsed. Never blocks. Failures are silent.
func RefreshAsync(currentVersion string) {
	go refreshFromPathWithFetcher(DefaultCachePath(), currentVersion, CheckInterval, func() (Release, error) {
		return FetchLatest(context.Background())
	})
}

// refreshFromPathWithFetcher is the testable core: deterministic, no goroutine,
// no global state. Reads cache, checks interval, calls fetcher, writes cache.
func refreshFromPathWithFetcher(path, currentVersion string, interval time.Duration, fetcher func() (Release, error)) {
	c, _ := ReadCacheFromPath(path)
	if !c.LastCheckedAt.IsZero() && time.Since(c.LastCheckedAt) < interval {
		return
	}
	rel, err := fetcher()
	if err != nil {
		return
	}
	_ = WriteCacheToPath(path, Cache{
		Version:       1,
		LastCheckedAt: time.Now(),
		LatestVersion: rel.Version(),
		LatestURL:     rel.HTMLURL,
	})
}
