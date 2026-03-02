package tui

import (
	"strings"
	"testing"

	"github.com/LeahArmstrong/grove-cli/plugins/tracker"
)

func TestRenderPRPreview(t *testing.T) {
	tests := []struct {
		name       string
		pr         *tracker.PullRequest
		width      int
		wantParts  []string
		wantAbsent []string
	}{
		{
			name: "shows title and branch",
			pr: &tracker.PullRequest{
				Number: 42,
				Title:  "Add authentication module",
				Branch: "feat/auth",
				Author: "alice",
				Body:   "This PR adds auth.",
			},
			width:     80,
			wantParts: []string{"#42", "Add authentication module", "feat/auth", "alice"},
		},
		{
			name: "shows draft label",
			pr: &tracker.PullRequest{
				Number:  10,
				Title:   "WIP feature",
				Branch:  "wip",
				Author:  "bob",
				IsDraft: true,
				Body:    "Work in progress.",
			},
			width:     80,
			wantParts: []string{"DRAFT", "WIP feature"},
		},
		{
			name: "shows review decision",
			pr: &tracker.PullRequest{
				Number:         5,
				Title:          "Fix bug",
				Branch:         "fix/bug",
				Author:         "carol",
				ReviewDecision: "APPROVED",
				Body:           "Bug fix.",
			},
			width:     80,
			wantParts: []string{"APPROVED"},
		},
		{
			name: "shows diff stats",
			pr: &tracker.PullRequest{
				Number:    99,
				Title:     "Big refactor",
				Branch:    "refactor",
				Author:    "dave",
				Additions: 500,
				Deletions: 200,
				Body:      "Major refactor.",
			},
			width:     80,
			wantParts: []string{"+500", "-200"},
		},
		{
			name: "shows body content",
			pr: &tracker.PullRequest{
				Number: 1,
				Title:  "Simple PR",
				Branch: "simple",
				Author: "eve",
				Body:   "## Summary\nThis is a simple change.",
			},
			width:     80,
			wantParts: []string{"Summary", "simple change"},
		},
		{
			name: "handles empty body",
			pr: &tracker.PullRequest{
				Number: 2,
				Title:  "No body",
				Branch: "empty",
				Author: "frank",
				Body:   "",
			},
			width:     80,
			wantParts: []string{"No body", "No description"},
		},
		{
			name: "shows commit count",
			pr: &tracker.PullRequest{
				Number:      7,
				Title:       "Multi commit",
				Branch:      "multi",
				Author:      "grace",
				CommitCount: 5,
				Body:        "Five commits.",
			},
			width:     80,
			wantParts: []string{"5 commits"},
		},
		{
			name: "shows footer actions",
			pr: &tracker.PullRequest{
				Number: 3,
				Title:  "Test",
				Branch: "test",
				Author: "hal",
				Body:   "Test.",
			},
			width:     80,
			wantParts: []string{"worktree", "esc", "Back"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := renderPRPreview(tt.pr, tt.width)
			for _, part := range tt.wantParts {
				if !strings.Contains(got, part) {
					t.Errorf("renderPRPreview() missing %q in output:\n%s", part, got)
				}
			}
			for _, absent := range tt.wantAbsent {
				if strings.Contains(got, absent) {
					t.Errorf("renderPRPreview() should not contain %q in output:\n%s", absent, got)
				}
			}
		})
	}
}

func TestPRViewStatePreviewToggle(t *testing.T) {
	prs := []*tracker.PullRequest{
		{Number: 1, Title: "Test PR", Branch: "test", Author: "user", Body: "Body"},
	}
	s := &PRViewState{
		PRs: prs,
	}

	if len(s.PRs) != 1 {
		t.Errorf("PRs length = %d, want 1", len(s.PRs))
	}

	// Initially no preview
	if s.ShowPreview {
		t.Error("ShowPreview should be false initially")
	}

	// Toggle on
	s.ShowPreview = true
	if !s.ShowPreview {
		t.Error("ShowPreview should be true after toggle")
	}

	// Toggle off
	s.ShowPreview = false
	if s.ShowPreview {
		t.Error("ShowPreview should be false after second toggle")
	}
}

func TestRenderPRViewV2WithPreview(t *testing.T) {
	s := &PRViewState{
		PRs: []*tracker.PullRequest{
			{Number: 1, Title: "Test PR", Branch: "test", Author: "user", Body: "PR body text"},
		},
		ShowPreview: true,
	}

	got := renderPRViewV2(s, 120, "⠋")
	// When preview is shown, the PR body content should appear
	if !strings.Contains(got, "PR body text") {
		t.Errorf("renderPRViewV2 with preview should show PR body, got:\n%s", got)
	}
}

func TestRenderPRViewV2WithoutPreview(t *testing.T) {
	s := &PRViewState{
		PRs: []*tracker.PullRequest{
			{Number: 1, Title: "Test PR", Branch: "test", Author: "user", Body: "PR body text"},
		},
		ShowPreview: false,
	}

	got := renderPRViewV2(s, 120, "⠋")
	// When preview is not shown, body should not appear
	if strings.Contains(got, "PR body text") {
		t.Errorf("renderPRViewV2 without preview should NOT show PR body, got:\n%s", got)
	}
}
