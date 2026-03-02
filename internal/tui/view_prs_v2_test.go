package tui

import (
	"strings"
	"testing"
	"time"

	"github.com/LeahArmstrong/grove-cli/plugins/tracker"
)

func TestRenderPRViewV2_Loading(t *testing.T) {
	s := &PRViewState{Loading: true}
	view := renderPRViewV2(s, 80, "⠋")
	assertContains(t, view, "Pull Requests")
	assertContains(t, view, "Loading")
}

func TestRenderPRViewV2_Creating(t *testing.T) {
	s := &PRViewState{Creating: true}
	view := renderPRViewV2(s, 80, "⠋")
	assertContains(t, view, "Creating worktree")
}

func TestRenderPRViewV2_Error(t *testing.T) {
	s := &PRViewState{Error: "something broke"}
	view := renderPRViewV2(s, 80, "")
	assertContains(t, view, "something broke")
}

func TestRenderPRViewV2_EmptyPRs(t *testing.T) {
	s := &PRViewState{PRs: nil}
	view := renderPRViewV2(s, 80, "")
	assertContains(t, view, "no matching PRs")
}

func TestRenderPRViewV2_FilterCount(t *testing.T) {
	fi := newPRFilterInput()
	fi.SetValue("alpha")
	s := &PRViewState{
		PRs: []*tracker.PullRequest{
			{Number: 1, Title: "Alpha", Branch: "alpha", Author: "user"},
			{Number: 2, Title: "Beta", Branch: "beta", Author: "user"},
		},
		FilterInput: fi,
	}
	view := renderPRViewV2(s, 100, "")
	assertContains(t, view, "alpha")
	assertContains(t, view, "1 of 2")
}

func TestRenderPRViewV2_TwoLineItems(t *testing.T) {
	s := &PRViewState{
		PRs: []*tracker.PullRequest{
			{
				Number:      116,
				Title:       "Fix diff review cleanup",
				Branch:      "fix/diff-review",
				Author:      "LeahArmstrong",
				Additions:   234,
				Deletions:   89,
				CommitCount: 5,
				CreatedAt:   time.Now().Add(-2 * time.Hour),
			},
		},
	}
	view := renderPRViewV2(s, 100, "")
	assertContains(t, view, "#116")
	assertContains(t, view, "Fix diff review cleanup")
	assertContains(t, view, "@LeahArmstrong")
	assertContains(t, view, "5 commits")
	assertContains(t, view, "+234")
	assertContains(t, view, "-89")
}

func TestRenderPRViewV2_DraftLabel(t *testing.T) {
	s := &PRViewState{
		PRs: []*tracker.PullRequest{
			{Number: 106, Title: "Staging", Branch: "staging", Author: "user", IsDraft: true},
		},
	}
	view := renderPRViewV2(s, 100, "")
	assertContains(t, view, "[DRAFT]")
}

func TestRenderPRViewV2_WorktreeBadge(t *testing.T) {
	s := &PRViewState{
		PRs: []*tracker.PullRequest{
			{Number: 116, Title: "Fix", Branch: "fix/diff-review", Author: "user"},
		},
		WorktreeBranches: map[string]bool{"fix/diff-review": true},
	}
	view := renderPRViewV2(s, 100, "")
	assertContains(t, view, "✓ worktree")
}

func TestRenderPRViewV2_SelectedCursor(t *testing.T) {
	s := &PRViewState{
		PRs: []*tracker.PullRequest{
			{Number: 1, Title: "First", Branch: "a", Author: "u"},
			{Number: 2, Title: "Second", Branch: "b", Author: "u"},
		},
		Cursor: 1,
	}
	view := renderPRViewV2(s, 100, "")
	lines := strings.Split(view, "\n")
	foundCursorOnSecond := false
	for _, line := range lines {
		if strings.Contains(line, "Second") && strings.Contains(line, "❯") {
			foundCursorOnSecond = true
		}
	}
	if !foundCursorOnSecond {
		t.Error("cursor should be on second PR")
	}
}

func TestRenderPRViewV2_Footer(t *testing.T) {
	s := &PRViewState{
		PRs: []*tracker.PullRequest{
			{Number: 1, Title: "Test", Branch: "test", Author: "u"},
		},
	}
	view := renderPRViewV2(s, 100, "")
	assertContains(t, view, "enter")
	assertContains(t, view, "esc")
}

func TestRenderPRViewV2_BranchColumn(t *testing.T) {
	s := &PRViewState{
		PRs: []*tracker.PullRequest{
			{Number: 1, Title: "Test PR", Branch: "feature/my-branch", Author: "user"},
		},
	}
	view := renderPRViewV2(s, 100, "")
	assertContains(t, view, "feature/my-branch")
}

func TestFormatDiffStats(t *testing.T) {
	tests := []struct {
		name      string
		additions int
		deletions int
		wantPlus  string
		wantMinus string
	}{
		{"both", 234, 89, "+234", "-89"},
		{"large", 1203, 445, "+1,203", "-445"},
		{"zero", 0, 0, "+0", "-0"},
		{"only additions", 10, 0, "+10", "-0"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := formatDiffStats(tt.additions, tt.deletions)
			assertContains(t, result, tt.wantPlus)
			assertContains(t, result, tt.wantMinus)
		})
	}
}

func TestFormatCommitCount(t *testing.T) {
	tests := []struct {
		name  string
		count int
		want  string
	}{
		{"singular", 1, "1 commit"},
		{"plural", 5, "5 commits"},
		{"zero", 0, "0 commits"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatCommitCount(tt.count)
			if got != tt.want {
				t.Errorf("formatCommitCount(%d) = %q, want %q", tt.count, got, tt.want)
			}
		})
	}
}

// assertContains is a test helper for checking string containment.
func assertContains(t *testing.T, s, substr string) {
	t.Helper()
	if !strings.Contains(s, substr) {
		t.Errorf("expected string to contain %q, got:\n%s", substr, s)
	}
}
