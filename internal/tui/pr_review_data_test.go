package tui

import (
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/x/exp/golden"

	"github.com/lost-in-the/grove/internal/git"
	"github.com/lost-in-the/grove/plugins/tracker"
)

var reviewUpdated = time.Date(2026, 10, 20, 12, 0, 0, 0, time.UTC)

// openReview opens the review of #42 and resolves its repository, so the
// three fetches are in flight.
func openReview(t *testing.T) Model {
	t.Helper()
	m := newTestModel(withItems(3), withSize(120, 40))
	next, _ := m.openPRReview(&tracker.PullRequest{Number: 42, Title: "Answer", UpdatedAt: reviewUpdated})
	m = next.(Model)
	return sendMsg(m, prReviewRepoMsg{gen: m.prReview.Gen, repo: "o/r"})
}

func reviewText(m Model) string { return stripAnsi(m.viewString()) }

// Reopening the same PR while the first open's fetches are in flight: the
// older generation's results are dropped.
func TestPRReview_StaleGenerationDropped(t *testing.T) {
	m := openReview(t)
	old := m.prReview.Gen
	m = sendKey(m, "esc")
	next, _ := m.openPRReview(&tracker.PullRequest{Number: 42, Title: "Answer", UpdatedAt: reviewUpdated.Add(time.Hour)})
	m = next.(Model)
	m = sendMsg(m, prDiffMsg{gen: old, files: []git.FileDiff{{NewPath: "stale.go"}}})
	if m.prReview.Files != nil || !m.prReview.LoadingDiff {
		t.Errorf("an old generation's diff was taken: %+v", m.prReview.Files)
	}
	m = sendMsg(m, prDiffMsg{gen: m.prReview.Gen, files: []git.FileDiff{{NewPath: "fresh.go"}}})
	if len(m.prReview.Files) != 1 || m.prReview.Files[0].NewPath != "fresh.go" {
		t.Errorf("current diff not taken: %+v", m.prReview.Files)
	}
}

// A PR whose UpdatedAt matches the cached data opens with it and fetches
// nothing.
func TestPRReview_CachedPRSkipsFetch(t *testing.T) {
	m := newTestModel(withItems(3), withSize(120, 40))
	m.prReviewCache = map[int]*prReviewData{42: {
		updatedAt: reviewUpdated,
		detail:    &tracker.PRDetail{Number: 42, Checks: []tracker.PRCheck{{Name: "Lint", State: "SUCCESS"}}},
		files:     []git.FileDiff{{NewPath: "a.go"}},
	}}
	next, cmd := m.openPRReview(&tracker.PullRequest{Number: 42, UpdatedAt: reviewUpdated})
	m = next.(Model)
	if cmd != nil {
		t.Error("a cached, unchanged PR should not be fetched again")
	}
	if len(m.prReview.Files) != 1 || m.prReview.LoadingDiff || m.prReview.Detail == nil {
		t.Errorf("cache not used: %+v", m.prReview)
	}
}

// One failed fetch leaves the others' data in place (spec 13.5).
func TestPRReview_OneFailureKeepsOthers(t *testing.T) {
	m := openReview(t)
	m = sendMsg(m, prDetailMsg{gen: m.prReview.Gen, detail: &tracker.PRDetail{
		Number: 42,
		Checks: []tracker.PRCheck{{Name: "a"}, {Name: "b"}, {Name: "c"}},
	}})
	m = sendMsg(m, prDiffMsg{gen: m.prReview.Gen, err: errTest})
	text := reviewText(m)
	if !strings.Contains(text, "[3] checks 3") {
		t.Errorf("checks count missing:\n%s", text)
	}
	if !strings.Contains(text, "diff unavailable: test error") {
		t.Errorf("files tab should name the diff error:\n%s", text)
	}
}

func TestPRReview_GHMissingPanel(t *testing.T) {
	m := newTestModel(withItems(3), withSize(120, 40))
	next, _ := m.openPRReview(&tracker.PullRequest{Number: 42})
	m = next.(Model)
	m = sendMsg(m, prReviewRepoMsg{gen: m.prReview.Gen, ghMissing: true})
	if text := reviewText(m); !strings.Contains(text, "gh required") {
		t.Errorf("no gh panel:\n%s", text)
	}
}

func TestPRReview_EmptyDiff(t *testing.T) {
	m := openReview(t)
	m = sendMsg(m, prDiffMsg{gen: m.prReview.Gen})
	if text := reviewText(m); !strings.Contains(text, "no files changed") || !strings.Contains(text, "[4] files 0") {
		t.Errorf("empty diff:\n%s", text)
	}
}

// Every result for the open generation is cached once all three succeed,
// keyed on the PR's UpdatedAt.
func TestPRReview_CachesCompleteLoad(t *testing.T) {
	m := openReview(t)
	g := m.prReview.Gen
	m = sendMsg(m, prDetailMsg{gen: g, detail: &tracker.PRDetail{Number: 42}})
	m = sendMsg(m, prDiffMsg{gen: g, files: []git.FileDiff{{NewPath: "a.go"}}})
	m = sendMsg(m, prCommentsMsg{gen: g})
	if c := m.prReviewCache[42]; c == nil || !c.updatedAt.Equal(reviewUpdated) || len(c.files) != 1 {
		t.Errorf("cache after a full load: %+v", c)
	}
}

func TestGolden_PRReview(t *testing.T) {
	t.Run("loading", func(t *testing.T) {
		m := goldenModel(t, sizeStandard, withItems(3))
		next, _ := m.openPRReview(&tracker.PullRequest{Number: 42, Title: "Add user authentication flow"})
		golden.RequireEqual(t, []byte(next.(Model).viewString()))
	})
}

// The PR's own URL names its repository, so opening a review needs no gh
// call to find it (gh repo view uses GraphQL, which can be rate limited).
func TestRepoFromPRURL(t *testing.T) {
	cases := map[string]string{
		"https://github.com/lost-in-the/grove/pull/176": "lost-in-the/grove",
		"https://ghe.example.com/team/app/pull/3":       "ghe.example.com/team/app",
		"https://github.com/o/r/issues/1":               "",
		"":                                              "",
	}
	for url, want := range cases {
		if got := repoFromPRURL(url); got != want {
			t.Errorf("repoFromPRURL(%q) = %q, want %q", url, got, want)
		}
	}
}

// A PR with no UpdatedAt (a list entry gh left without one) is never
// served from the cache, since every such PR would look unchanged.
func TestPRReview_ZeroUpdatedAtNotCached(t *testing.T) {
	m := newTestModel(withItems(3), withSize(120, 40))
	m.prReviewCache = map[int]*prReviewData{42: {detail: &tracker.PRDetail{Number: 42}}}
	if _, cmd := m.openPRReview(&tracker.PullRequest{Number: 42}); cmd == nil {
		t.Error("a PR without UpdatedAt should be fetched")
	}
}
