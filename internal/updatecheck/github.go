package updatecheck

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/lost-in-the/grove/internal/version"
)

// latestReleaseURL is the canonical GitHub Releases API endpoint for grove.
const latestReleaseURL = "https://api.github.com/repos/lost-in-the/grove/releases/latest"

// fetchTimeout is the hard cap on the HTTP request.
const fetchTimeout = 2 * time.Second

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
func FetchLatest(ctx context.Context) (Release, error) {
	return fetchLatestFromURL(ctx, latestReleaseURL, fetchTimeout)
}

func fetchLatestFromURL(ctx context.Context, url string, timeout time.Duration) (Release, error) {
	client := &http.Client{Timeout: timeout}
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return Release{}, fmt.Errorf("updatecheck: github build request: %w", err)
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "grove-update-check/"+version.Version)

	resp, err := client.Do(req)
	if err != nil {
		return Release{}, fmt.Errorf("updatecheck: github request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return Release{}, fmt.Errorf("updatecheck: github HTTP %d", resp.StatusCode)
	}
	var rel Release
	if err := json.NewDecoder(resp.Body).Decode(&rel); err != nil {
		return Release{}, fmt.Errorf("updatecheck: github decode: %w", err)
	}
	return rel, nil
}
