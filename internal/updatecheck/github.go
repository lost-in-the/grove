package updatecheck

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

// LatestReleaseURL is the canonical GitHub Releases API endpoint for grove.
const LatestReleaseURL = "https://api.github.com/repos/lost-in-the/grove/releases/latest"

// FetchTimeout is the hard cap on the HTTP request.
const FetchTimeout = 2 * time.Second

// Release is the subset of the GitHub release JSON we care about.
type Release struct {
	TagName string `json:"tag_name"`
	HTMLURL string `json:"html_url"`
}

// Version returns the bare semver (no leading v).
func (r Release) Version() string {
	return strings.TrimPrefix(r.TagName, "v")
}

// FetchLatest queries the GitHub Releases API for the latest release.
func FetchLatest() (Release, error) {
	return fetchLatestFromURL(LatestReleaseURL, FetchTimeout)
}

func fetchLatestFromURL(url string, timeout time.Duration) (Release, error) {
	client := &http.Client{Timeout: timeout}
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return Release{}, fmt.Errorf("update check: build request: %w", err)
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "grove-update-check")

	resp, err := client.Do(req)
	if err != nil {
		return Release{}, fmt.Errorf("update check: github request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return Release{}, fmt.Errorf("update check: github HTTP %d", resp.StatusCode)
	}
	var rel Release
	if err := json.NewDecoder(resp.Body).Decode(&rel); err != nil {
		return Release{}, fmt.Errorf("update check: github decode: %w", err)
	}
	return rel, nil
}
