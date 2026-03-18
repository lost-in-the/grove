package tui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"

	"github.com/lost-in-the/grove/plugins/tracker"
)

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

func TestRenderPRList_Loading(t *testing.T) {
	s := &PRViewState{Loading: true}
	got := renderPRList(s, 80, "⠋", 20)
	assertContains(t, got, "Loading PRs")
}

func TestRenderPRList_WithItems(t *testing.T) {
	s := &PRViewState{
		PRs: []*tracker.PullRequest{
			{Number: 10, Title: "Add login", Branch: "feat/login", Author: "alice"},
			{Number: 20, Title: "Fix crash", Branch: "fix/crash", Author: "bob"},
		},
		FilterInput: newFilterInput(""),
	}
	got := renderPRList(s, 80, "⠋", 20)
	assertContains(t, got, "#10")
	assertContains(t, got, "Add login")
	assertContains(t, got, "#20")
	assertContains(t, got, "Fix crash")
}

func TestRenderPRList_Filtered(t *testing.T) {
	fi := newFilterInput("")
	fi.SetValue("auth")
	s := &PRViewState{
		PRs: []*tracker.PullRequest{
			{Number: 1, Title: "Add auth", Branch: "feat/auth", Author: "user"},
			{Number: 2, Title: "Fix bug", Branch: "fix/bug", Author: "user"},
			{Number: 3, Title: "Update docs", Branch: "docs", Author: "user"},
		},
		FilterInput: fi,
	}
	got := renderPRList(s, 80, "", 20)
	assertContains(t, got, "of")
}

func TestRenderPRDetailContent_Commits(t *testing.T) {
	pr := &tracker.PullRequest{
		Number: 42,
		Title:  "Test PR",
		Branch: "feat/test",
		Author: "user",
		Commits: []tracker.PRCommit{
			{SHA: "abc1234", Message: "fix bug"},
		},
	}
	got := renderPRDetailContent(pr, 80)
	plain := ansi.Strip(got)
	assertContains(t, plain, "Commits")
	assertContains(t, plain, "abc1234")
	assertContains(t, plain, "fix bug")
}

func TestRenderPRDetailContent_NoCommits(t *testing.T) {
	pr := &tracker.PullRequest{
		Number: 42,
		Title:  "Test PR",
		Branch: "feat/test",
		Author: "user",
	}
	got := renderPRDetailContent(pr, 80)
	plain := ansi.Strip(got)
	if strings.Contains(plain, "Commits") {
		t.Errorf("did not expect 'Commits' section without commits, got:\n%s", plain)
	}
}

func TestRenderPRDetailContent_ReviewDecision(t *testing.T) {
	pr := &tracker.PullRequest{
		Number:         42,
		Title:          "Test PR",
		Branch:         "feat/test",
		Author:         "user",
		ReviewDecision: "APPROVED",
	}
	got := renderPRDetailContent(pr, 80)
	assertContains(t, got, "APPROVED")
}

func TestRenderPRDetailContent_Body(t *testing.T) {
	pr := &tracker.PullRequest{
		Number: 42,
		Title:  "Test PR",
		Branch: "feat/test",
		Author: "user",
		Body:   "This fixes the login flow",
	}
	got := renderPRDetailContent(pr, 80)
	plain := ansi.Strip(got)
	assertContains(t, plain, "Description")
	assertContains(t, plain, "login flow")
}

func TestRenderPRFooter_ListFocused(t *testing.T) {
	m := newTestModel(withItems(3), withSize(120, 30))
	m.activeView = ViewPRs
	m.prState = &PRViewState{
		PRs:         []*tracker.PullRequest{{Number: 1, Title: "T", Branch: "b", Author: "u"}},
		FilterInput: newFilterInput(""),
	}
	got := m.renderPRFooter()
	assertContains(t, got, "tab")
	assertContains(t, got, "detail")
}

func TestRenderPRFooter_DetailFocused(t *testing.T) {
	m := newTestModel(withItems(3), withSize(120, 30))
	m.activeView = ViewPRs
	m.prState = &PRViewState{
		PRs:           []*tracker.PullRequest{{Number: 1, Title: "T", Branch: "b", Author: "u"}},
		FilterInput:   newFilterInput(""),
		DetailFocused: true,
	}
	got := m.renderPRFooter()
	assertContains(t, got, "scroll")
}

func TestRenderContextHeader(t *testing.T) {
	got := renderContextHeader("Pull Requests", "myproject", 120)
	assertContains(t, got, "myproject")
	assertContains(t, got, "Pull Requests")
	assertContains(t, got, "esc to return")
}

// assertContains is a test helper for checking string containment.
func assertContains(t *testing.T, s, substr string) {
	t.Helper()
	if !strings.Contains(s, substr) {
		t.Errorf("expected string to contain %q, got:\n%s", substr, s)
	}
}
