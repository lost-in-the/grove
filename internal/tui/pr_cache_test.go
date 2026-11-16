package tui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"charm.land/bubbles/v2/spinner"

	"github.com/lost-in-the/grove/plugins/tracker"
)

// isolateCache points os.UserCacheDir at a temp dir on Linux (XDG) and
// macOS (HOME/Library/Caches).
func isolateCache(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("XDG_CACHE_HOME", dir)
	t.Setenv("HOME", dir)
	return dir
}

var cachedPRs = []*tracker.PullRequest{
	{Number: 7, Title: "Cached seven", Branch: "seven"},
	{Number: 9, Title: "Cached nine", Branch: "nine"},
}

func TestPRCache_RoundTrip(t *testing.T) {
	isolateCache(t)
	root := t.TempDir()
	if err := savePRCache(root, cachedPRs); err != nil {
		t.Fatal(err)
	}
	got := loadPRCache(root)
	if len(got) != 2 || got[1].Number != 9 || got[1].Title != "Cached nine" {
		t.Errorf("loaded %+v", got)
	}
}

func TestPRCache_IgnoresCorruptFile(t *testing.T) {
	isolateCache(t)
	root := t.TempDir()
	path, err := prCachePath(root)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(`{"saved_at":"2026-10-22T00:00:00Z","prs":[{"num`), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := loadPRCache(root); got != nil {
		t.Errorf("truncated file loaded %+v, want nil", got)
	}
}

// A file written by a newer grove, with fields this one does not know, loads.
func TestPRCache_KeepsUnknownFields(t *testing.T) {
	isolateCache(t)
	root := t.TempDir()
	path, _ := prCachePath(root)
	_ = os.MkdirAll(filepath.Dir(path), 0o755)
	if err := os.WriteFile(path, []byte(`{"saved_at":"2026-10-22T00:00:00Z","format":3,"prs":[{"number":5,"title":"Five","future":true}]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := loadPRCache(root); len(got) != 1 || got[0].Number != 5 {
		t.Errorf("loaded %+v", got)
	}
}

// Two projects with the same directory name keep separate caches.
func TestPRCachePath_PerProject(t *testing.T) {
	isolateCache(t)
	a, _ := prCachePath("/one/grove")
	b, _ := prCachePath("/two/grove")
	if a == b || !strings.Contains(filepath.Base(a), "grove-") {
		t.Errorf("paths %q and %q", a, b)
	}
}

func TestPRView_OpensOnCachedList(t *testing.T) {
	m := newTestModel(withItems(3), withSize(120, 30))
	m.prCache = cachedPRs
	next, cmd := m.enterPRView()
	m = next.(Model)
	s := m.prState
	if s.Loading || !s.Stale || len(s.PRs) != 2 {
		t.Errorf("state: loading %v, stale %v, %d PRs", s.Loading, s.Stale, len(s.PRs))
	}
	if cmd == nil || !m.prFetching {
		t.Error("opening on a cached list should still refresh it")
	}
	if view := stripAnsi(m.viewString()); !strings.Contains(view, "cached · refreshing") || !strings.Contains(view, "Cached nine") {
		t.Errorf("view lacks the stale line or the cached rows:\n%s", view)
	}
}

func TestPRView_RefreshReplacesStaleList(t *testing.T) {
	isolateCache(t)
	m := newTestModel(withItems(3), withSize(120, 30))
	m.projectRoot = t.TempDir()
	m.prCache = cachedPRs
	next, _ := m.enterPRView()
	m = next.(Model)
	m.prState.Cursor = 1 // #9
	// #9 moves from index 1 to 2, so keeping the index would select #7.
	fresh := []*tracker.PullRequest{{Number: 11, Title: "New eleven"}, {Number: 7, Title: "Cached seven"}, {Number: 9, Title: "Cached nine"}}
	m = sendMsg(m, prsFetchedMsg{prs: fresh})
	s := m.prState
	if s.Stale || s.Error != "" || len(s.PRs) != 3 || m.prFetching {
		t.Errorf("state: stale %v, error %q, %d PRs, fetching %v", s.Stale, s.Error, len(s.PRs), m.prFetching)
	}
	if s.PRs[s.Cursor].Number != 9 {
		t.Errorf("cursor on #%d, want #9 kept selected", s.PRs[s.Cursor].Number)
	}
}

// fetchPRsCmd saves what it fetched; the save happens in the command, not
// in Update.
func TestPRCache_SavedByFetch(t *testing.T) {
	isolateCache(t)
	root := t.TempDir()
	if err := savePRCacheAfterFetch(root, cachedPRs, nil); err != nil {
		t.Fatal(err)
	}
	if got := loadPRCache(root); len(got) != 2 {
		t.Errorf("after fetch, cache holds %+v", got)
	}
	if err := savePRCacheAfterFetch(root, nil, errTest); err != nil {
		t.Fatal(err)
	}
	if got := loadPRCache(root); len(got) != 2 {
		t.Error("a failed fetch must not overwrite the cache")
	}
}

func TestPRView_FailedRefreshKeepsList(t *testing.T) {
	m := newTestModel(withItems(3), withSize(120, 30))
	m.prCache = cachedPRs
	next, _ := m.enterPRView()
	m = next.(Model)
	m = sendMsg(m, prsFetchedMsg{err: errTest})
	if len(m.prState.PRs) != 2 {
		t.Errorf("failed refresh dropped the list: %+v", m.prState.PRs)
	}
	view := stripAnsi(m.viewString())
	if !strings.Contains(view, "refresh failed: test error") || strings.Contains(view, "refreshing") {
		t.Errorf("view:\n%s", view)
	}
}

// Leaving and reopening the browser while its fetch is in flight starts no
// second fetch; the first one fills the reopened browser.
func TestPRView_NoDoubleFetch(t *testing.T) {
	m := newTestModel(withItems(3), withSize(120, 30))
	next, _ := m.enterPRView()
	m = next.(Model)
	m = sendKey(m, "esc")
	next, cmd := m.enterPRView()
	m = next.(Model)
	if _, ok := cmd().(spinner.TickMsg); !ok {
		t.Error("reopening should only restart the spinner, not start a second fetch")
	}
	m = sendMsg(m, prsFetchedMsg{prs: cachedPRs})
	if m.prState == nil || len(m.prState.PRs) != 2 || m.prState.Loading {
		t.Errorf("the in-flight fetch did not fill the reopened browser: %+v", m.prState)
	}
}

// The stale line's spinner moves while the refresh runs, and stops once it
// has failed.
func TestPRView_StaleListAnimates(t *testing.T) {
	m := newTestModel(withItems(3), withSize(120, 30))
	m.prCache = cachedPRs
	next, _ := m.enterPRView()
	m = next.(Model)
	if !m.isAnimating() {
		t.Error("a stale list being refreshed should animate its spinner")
	}
	m = sendMsg(m, prsFetchedMsg{err: errTest})
	if m.isTrackerAnimating() {
		t.Error("a failed refresh has nothing left to animate")
	}
}

// grove prs starts in the browser with Init's fetch in flight, so reopening
// starts no second fetch.
func TestConfigureForPRs_MarksFetchInFlight(t *testing.T) {
	if m := newTestModel(withItems(3)).ConfigureForPRs(); !m.prFetching {
		t.Error("ConfigureForPRs should record the fetch Init starts")
	}
}
