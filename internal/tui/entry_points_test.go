package tui

import (
	"testing"
)

func TestNewModelForPRs(t *testing.T) {
	m := newTestModel(withItems(3))
	m = m.ConfigureForPRs()

	if m.activeView != ViewPRs {
		t.Errorf("expected activeView=ViewPRs, got %d", m.activeView)
	}
	if m.prState == nil {
		t.Fatal("expected prState to be initialized")
	}
	if !m.prState.Loading {
		t.Error("expected prState.Loading to be true")
	}
}

func TestNewModelForIssues(t *testing.T) {
	m := newTestModel(withItems(3))
	m = m.ConfigureForIssues()

	if m.activeView != ViewIssues {
		t.Errorf("expected activeView=ViewIssues, got %d", m.activeView)
	}
	if m.issueState == nil {
		t.Fatal("expected issueState to be initialized")
	}
	if !m.issueState.Loading {
		t.Error("expected issueState.Loading to be true")
	}
}

func TestConfigureForPRs_WorktreeBranches(t *testing.T) {
	m := newTestModel(withItems(3))
	m = m.ConfigureForPRs()

	if m.prState.WorktreeBranches == nil {
		t.Error("expected WorktreeBranches to be initialized")
	}
}

// TestHandleWorktreesFetched_RefreshesPRWorktreeBranches reproduces the
// `grove prs` entry point: prState is built before worktrees load, so its
// WorktreeBranches map starts empty and must be refreshed once worktrees
// arrive (otherwise the "worktree exists" badge/prompt never fire).
func TestHandleWorktreesFetched_RefreshesPRWorktreeBranches(t *testing.T) {
	m := newTestModel(withSize(80, 30))
	m.prState = &PRViewState{Loading: true, WorktreeBranches: map[string]string{}}

	items := makeTestItems(2) // root + feature-auth (branch "feature-auth")
	res, _ := m.handleWorktreesFetched(worktreesFetchedMsg{items: items})
	mm := res.(Model)

	if got := mm.prState.WorktreeBranches["feature-auth"]; got != "feature-auth" {
		t.Errorf("expected prState.WorktreeBranches to include feature-auth after fetch, got %q (map=%v)",
			got, mm.prState.WorktreeBranches)
	}
}
