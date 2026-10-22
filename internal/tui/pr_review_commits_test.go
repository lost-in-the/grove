package tui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/charmbracelet/x/exp/golden"

	"github.com/lost-in-the/grove/internal/git"
	"github.com/lost-in-the/grove/plugins/tracker"
)

var commitDetail = &tracker.PRDetail{Number: 42, Commits: []tracker.PRCommit{
	{SHA: "aaaaaaa", OID: "aaaaaaa111", Message: "feat: first"},
	{SHA: "bbbbbbb", OID: "bbbbbbb222", Message: "fix: second"},
}}

// commitsReview opens #42 with its detail and PR diff loaded, on tab 2,
// with the cursor on the second commit.
func commitsReview(t *testing.T) Model {
	t.Helper()
	m := reviewWithFiles(t, 120, 40, railFiles)
	m = sendMsg(m, prDetailMsg{gen: m.prReview.Gen, detail: commitDetail})
	return sendKey(sendKey(m, "2"), "j")
}

func TestPRReview_CommitsList(t *testing.T) {
	m := commitsReview(t)
	text := stripAnsi(m.viewString())
	if !strings.Contains(text, "aaaaaaa feat: first") || !strings.Contains(text, "bbbbbbb fix: second") || !strings.Contains(text, "[2] commits 2") {
		t.Errorf("commits tab:\n%s", text)
	}
}

// enter scopes the files tab to the commit; a goes back to the whole PR.
func TestPRReview_EnterScopesToCommit(t *testing.T) {
	m := sendKey(commitsReview(t), "enter")
	s := m.prReview
	if s.Tab != 4 || s.ScopedSHA != "bbbbbbb222" || !s.LoadingScope {
		t.Fatalf("after enter: tab %d scoped %q loading %v", s.Tab, s.ScopedSHA, s.LoadingScope)
	}
	if text := stripAnsi(m.viewString()); !strings.Contains(text, "loading bbbbbbb") {
		t.Errorf("no loading line:\n%s", text)
	}
	m = sendMsg(m, prCommitDiffMsg{gen: s.Gen, sha: "bbbbbbb222", files: []git.FileDiff{diffFile("only.go", 1, 0)}})
	text := stripAnsi(m.viewString())
	if !strings.Contains(text, "files · bbbbbbb") || !strings.Contains(text, "only.go") {
		t.Errorf("scoped view:\n%s", text)
	}
	m = sendKey(m, "a")
	if m.prReview.ScopedSHA != "" || len(m.prReview.Files) != len(railFiles) {
		t.Errorf("a did not restore the PR's files: %d files", len(m.prReview.Files))
	}
}

// A commit's diff is fetched once per session.
func TestPRReview_CommitDiffCachedBySha(t *testing.T) {
	m := sendKey(commitsReview(t), "enter")
	m = sendMsg(m, prCommitDiffMsg{gen: m.prReview.Gen, sha: "bbbbbbb222", files: []git.FileDiff{diffFile("only.go", 1, 0)}})
	m = sendKey(sendKey(sendKey(m, "a"), "2"), "enter")
	if m.prReview.LoadingScope || len(m.prReview.Files) != 1 {
		t.Errorf("second scope refetched: loading %v, %d files", m.prReview.LoadingScope, len(m.prReview.Files))
	}
}

func TestPRReview_CommitScopeError(t *testing.T) {
	m := sendKey(commitsReview(t), "enter")
	m = sendMsg(m, prCommitDiffMsg{gen: m.prReview.Gen, sha: "bbbbbbb222", err: errTest})
	if text := stripAnsi(m.viewString()); !strings.Contains(text, "diff unavailable: test error") {
		t.Errorf("scope error:\n%s", text)
	}
}

func TestPRReview_StaleCommitDiffDropped(t *testing.T) {
	m := sendKey(commitsReview(t), "enter")
	old := m.prReview.Gen
	m = sendKey(m, "a") // back to the PR's files
	m = sendMsg(m, prCommitDiffMsg{gen: old, sha: "bbbbbbb222", files: []git.FileDiff{diffFile("late.go", 1, 0)}})
	if m.prReview.ScopedSHA != "" || currentPath(m) == "late.go" {
		t.Errorf("a late commit diff took over: scoped %q, file %q", m.prReview.ScopedSHA, currentPath(m))
	}
}

// Viewed marks belong to the whole PR, so v is refused while scoped and
// ui_prefs.json stays untouched.
func TestPRReview_ViewedRefusedWhileScoped(t *testing.T) {
	m := sendKey(commitsReview(t), "enter")
	m = sendMsg(m, prCommitDiffMsg{gen: m.prReview.Gen, sha: "bbbbbbb222", files: []git.FileDiff{diffFile("only.go", 1, 0)}})
	m = sendKey(m, "v")
	if _, err := os.Stat(filepath.Join(m.projectRoot, ".grove", "ui_prefs.json")); err == nil {
		t.Error("v while scoped wrote ui_prefs.json")
	}
	if m.toast.Current == nil || !strings.Contains(m.toast.Message(), "whole PR") {
		t.Errorf("no toast explaining the refusal: %q", m.toast.Message())
	}
}

func TestGolden_PRReviewCommits(t *testing.T) {
	m := goldenModel(t, sizeStandard, withItems(3))
	next, _ := m.openPRReview(&tracker.PullRequest{Number: 42, Title: "Add user authentication flow"})
	m = next.(Model)
	m = sendMsg(m, prDetailMsg{gen: m.prReview.Gen, detail: commitDetail})
	golden.RequireEqual(t, []byte(sendKey(m, "2").viewString()))
}
