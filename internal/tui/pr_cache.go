package tui

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/lost-in-the/grove/internal/fsutil"
	"github.com/lost-in-the/grove/internal/tuilog"
	"github.com/lost-in-the/grove/plugins/tracker"
)

// prCacheFile is the PR list saved for the next time the browser opens, so
// it shows at once while a refresh runs (spec 13.3).
type prCacheFile struct {
	SavedAt time.Time              `json:"saved_at"`
	PRs     []*tracker.PullRequest `json:"prs"`
}

// prCacheLoadedMsg carries the saved PR list, read off the event loop.
type prCacheLoadedMsg struct {
	prs []*tracker.PullRequest
}

// prCachePath is where a project's PR list is saved: the user cache
// directory, not .grove, whose git excludes are only refreshed by init and
// bootstrap. The hash keeps two projects with the same name apart.
func prCachePath(projectRoot string) (string, error) {
	dir, err := os.UserCacheDir()
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256([]byte(projectRoot))
	name := fmt.Sprintf("%s-%x.json", filepath.Base(projectRoot), sum[:6])
	return filepath.Join(dir, "grove", "prs", name), nil
}

// loadPRCache returns the saved PR list, or nil when there is none or it
// cannot be read.
func loadPRCache(projectRoot string) []*tracker.PullRequest {
	path, err := prCachePath(projectRoot)
	if err != nil {
		return nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	var f prCacheFile
	if err := json.Unmarshal(data, &f); err != nil {
		tuilog.Printf("warning: ignoring PR cache %s: %v", path, err)
		return nil
	}
	return f.PRs
}

// savePRCache saves prs as the project's PR list.
func savePRCache(projectRoot string, prs []*tracker.PullRequest) error {
	path, err := prCachePath(projectRoot)
	if err != nil {
		return err
	}
	data, err := json.Marshal(prCacheFile{SavedAt: time.Now().UTC(), PRs: prs})
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return fsutil.AtomicWriteFile(path, data, 0o644)
}

// savePRCacheAfterFetch saves a successful fetch's list; a failed fetch
// leaves the saved one alone.
func savePRCacheAfterFetch(projectRoot string, prs []*tracker.PullRequest, fetchErr error) error {
	if fetchErr != nil {
		return nil
	}
	return savePRCache(projectRoot, prs)
}

// loadPRCacheCmd reads the saved PR list at startup.
func loadPRCacheCmd(projectRoot string) tea.Cmd {
	return func() tea.Msg { return prCacheLoadedMsg{prs: loadPRCache(projectRoot)} }
}
