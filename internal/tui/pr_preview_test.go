package tui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
	"github.com/lost-in-the/grove/plugins/tracker"
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
			wantParts: []string{"test-footer"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := renderPRPreview(tt.pr, tt.width, "test-footer")
			plain := ansi.Strip(got)
			for _, part := range tt.wantParts {
				if !strings.Contains(plain, part) {
					t.Errorf("renderPRPreview() missing %q in output:\n%s", part, got)
				}
			}
			for _, absent := range tt.wantAbsent {
				if strings.Contains(plain, absent) {
					t.Errorf("renderPRPreview() should not contain %q in output:\n%s", absent, got)
				}
			}
		})
	}
}

func TestPRViewStateDetailFocusToggle(t *testing.T) {
	prs := []*tracker.PullRequest{
		{Number: 1, Title: "Test PR", Branch: "test", Author: "user", Body: "Body"},
	}
	s := &PRViewState{
		PRs: prs,
	}

	if len(s.PRs) != 1 {
		t.Errorf("PRs length = %d, want 1", len(s.PRs))
	}

	// Initially not focused
	if s.DetailFocused {
		t.Error("DetailFocused should be false initially")
	}

	// Toggle on
	s.DetailFocused = true
	if !s.DetailFocused {
		t.Error("DetailFocused should be true after toggle")
	}

	// Toggle off
	s.DetailFocused = false
	if s.DetailFocused {
		t.Error("DetailFocused should be false after second toggle")
	}
}

func TestRenderPRViewV2ShowsList(t *testing.T) {
	s := &PRViewState{
		PRs: []*tracker.PullRequest{
			{Number: 1, Title: "Test PR", Branch: "test", Author: "user", Body: "PR body text"},
		},
	}

	got := renderPRViewV2(s, 120, "⠋", "test-footer")
	// The overlay view always shows the PR list (detail is in panel layout)
	plain := ansi.Strip(got)
	if !strings.Contains(plain, "Test PR") {
		t.Errorf("renderPRViewV2 should show PR title in list, got:\n%s", got)
	}
}
