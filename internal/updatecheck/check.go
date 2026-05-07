package updatecheck

import (
	"io"
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
