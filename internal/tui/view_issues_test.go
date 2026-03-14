package tui

import (
	"strings"
	"testing"
	"time"

	"github.com/lost-in-the/grove/plugins/tracker"
)

func TestFilteredIssues(t *testing.T) {
	now := time.Now()
	issues := []*tracker.Issue{
		{Number: 33, Title: "grove last should be project-scoped", Author: "LeahArmstrong", Labels: []string{"enhancement", "CLI"}, CreatedAt: now},
		{Number: 32, Title: "grove new missing --branch and --from flags", Author: "LeahArmstrong", Labels: []string{"enhancement"}, CreatedAt: now},
		{Number: 31, Title: "grove to should handle dirty worktrees", Author: "JohnDoe", Labels: []string{"bug"}, CreatedAt: now},
	}

	tests := []struct {
		name   string
		filter string
		want   int
	}{
		{"empty filter returns all", "", 3},
		{"filter by title keyword", "branch", 1},
		{"filter by number", "#33", 1},
		{"filter by author", "johndoe", 1},
		{"filter by label", "bug", 1},
		{"no match", "nonexistent", 0},
		{"case insensitive", "GROVE", 3},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := filteredIssues(issues, tt.filter)
			if len(got) != tt.want {
				t.Errorf("filteredIssues(%q) returned %d items, want %d", tt.filter, len(got), tt.want)
			}
		})
	}
}

func TestRenderIssueView(t *testing.T) {
	now := time.Now()
	issues := []*tracker.Issue{
		{Number: 33, Title: "grove last should be project-scoped", Author: "LeahArmstrong", Labels: []string{"enhancement", "CLI"}, CreatedAt: now},
		{Number: 32, Title: "grove new missing flags", Author: "LeahArmstrong", Labels: []string{"enhancement"}, CreatedAt: now},
	}

	tests := []struct {
		name     string
		state    *IssueViewState
		width    int
		contains []string
	}{
		{
			"loading state",
			&IssueViewState{Loading: true},
			80,
			[]string{"Issues", "Loading"},
		},
		{
			"creating state",
			&IssueViewState{Creating: true},
			80,
			[]string{"Issues", "Creating worktree"},
		},
		{
			"renders issue list",
			&IssueViewState{Issues: issues},
			80,
			[]string{"#33", "grove last", "@LeahArmstrong", "enhancement", "2 open"},
		},
		{
			"shows error",
			&IssueViewState{Issues: issues, Error: "something failed"},
			80,
			[]string{"something failed"},
		},
		{
			"filter with count",
			issueStateWithFilter(issues, "grove"),
			80,
			[]string{"Filter:", "grove", "2 of 2"},
		},
		{
			"empty filtered results",
			issueStateWithFilter(issues, "nonexistent"),
			80,
			[]string{"no matching issues"},
		},
		{
			"cursor indicator on selected",
			&IssueViewState{Issues: issues, Cursor: 0},
			80,
			[]string{"❯"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			view := renderIssueView(tt.state, tt.width, "⠋", "test-footer")
			for _, s := range tt.contains {
				if !strings.Contains(view, s) {
					t.Errorf("renderIssueView() missing %q in:\n%s", s, view)
				}
			}
		})
	}
}

func TestRenderIssuePreview(t *testing.T) {
	issue := &tracker.Issue{
		Number: 33,
		Title:  "grove last should be project-scoped",
		Body:   "## Description\n\nThe `grove last` command should filter by project.",
		Author: "LeahArmstrong",
		Labels: []string{"enhancement", "CLI"},
	}

	tests := []struct {
		name     string
		issue    *tracker.Issue
		width    int
		contains []string
	}{
		{
			"renders title and number",
			issue,
			80,
			[]string{"#33", "grove last should be project-scoped"},
		},
		{
			"renders metadata",
			issue,
			80,
			[]string{"@LeahArmstrong", "enhancement", "CLI"},
		},
		{
			"renders body markdown",
			issue,
			80,
			[]string{"Description", "grove last"},
		},
		{
			"empty body message",
			&tracker.Issue{Number: 1, Title: "test", Author: "user"},
			80,
			[]string{"No description provided"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			view := renderIssuePreview(tt.issue, tt.width, "test-footer")
			for _, s := range tt.contains {
				if !strings.Contains(view, s) {
					t.Errorf("renderIssuePreview() missing %q in:\n%s", s, view)
				}
			}
		})
	}
}

func TestIssueViewState(t *testing.T) {
	t.Run("initial state", func(t *testing.T) {
		s := &IssueViewState{Loading: true}
		if !s.Loading {
			t.Error("expected Loading to be true")
		}
		if s.Cursor != 0 {
			t.Error("expected Cursor to be 0")
		}
	})

	t.Run("filter resets cursor in rendering", func(t *testing.T) {
		now := time.Now()
		issues := []*tracker.Issue{
			{Number: 1, Title: "first", CreatedAt: now},
			{Number: 2, Title: "second", CreatedAt: now},
		}
		s := issueStateWithFilter(issues, "first")
		s.Cursor = 1
		filtered := filteredIssues(s.Issues, s.FilterInput.Value())
		if len(filtered) != 1 {
			t.Errorf("expected 1 filtered issue, got %d", len(filtered))
		}
	})
}

// issueStateWithFilter creates an IssueViewState with a pre-set filter value.
func issueStateWithFilter(issues []*tracker.Issue, filter string) *IssueViewState {
	fi := newIssueFilterInput()
	fi.SetValue(filter)
	return &IssueViewState{Issues: issues, FilterInput: fi}
}

func TestRenderIssueList_Loading(t *testing.T) {
	s := &IssueViewState{Loading: true}
	got := renderIssueList(s, 80, "⠋", 20)
	if !strings.Contains(got, "Loading issues") {
		t.Errorf("expected 'Loading issues' in output, got:\n%s", got)
	}
}

func TestRenderIssueList_WithItems(t *testing.T) {
	now := time.Now()
	s := &IssueViewState{
		Issues: []*tracker.Issue{
			{Number: 10, Title: "Add login page", Author: "alice", CreatedAt: now},
			{Number: 20, Title: "Fix crash on startup", Author: "bob", CreatedAt: now},
		},
		FilterInput: newIssueFilterInput(),
	}
	got := renderIssueList(s, 80, "⠋", 20)
	if !strings.Contains(got, "#10") {
		t.Errorf("expected '#10' in output, got:\n%s", got)
	}
	if !strings.Contains(got, "Add login page") {
		t.Errorf("expected 'Add login page' in output, got:\n%s", got)
	}
	if !strings.Contains(got, "#20") {
		t.Errorf("expected '#20' in output, got:\n%s", got)
	}
	if !strings.Contains(got, "Fix crash on startup") {
		t.Errorf("expected 'Fix crash on startup' in output, got:\n%s", got)
	}
}

func TestRenderIssueDetailContent_Body(t *testing.T) {
	issue := &tracker.Issue{
		Number: 33,
		Title:  "Test issue",
		Author: "user",
		Body:   "This is a detailed description of the issue.",
	}
	got := renderIssueDetailContent(issue, 80)
	plain := ansiStripRE.ReplaceAllString(got, "")
	if !strings.Contains(plain, "Description") {
		t.Errorf("expected 'Description' in output, got:\n%s", plain)
	}
	if !strings.Contains(plain, "detailed description") {
		t.Errorf("expected body text in output, got:\n%s", plain)
	}
}

func TestRenderIssueDetailContent_Labels(t *testing.T) {
	issue := &tracker.Issue{
		Number: 33,
		Title:  "Test issue",
		Author: "user",
		Labels: []string{"bug", "urgent"},
	}
	got := renderIssueDetailContent(issue, 80)
	if !strings.Contains(got, "bug") {
		t.Errorf("expected 'bug' label in output, got:\n%s", got)
	}
	if !strings.Contains(got, "urgent") {
		t.Errorf("expected 'urgent' label in output, got:\n%s", got)
	}
}

func TestRenderIssueFooter_ListFocused(t *testing.T) {
	m := newTestModel(withItems(3), withSize(120, 30))
	m.activeView = ViewIssues
	m.issueState = &IssueViewState{
		Issues:        []*tracker.Issue{{Number: 1, Title: "T", Author: "u"}},
		FilterInput:   newIssueFilterInput(),
		DetailFocused: false,
	}
	got := m.renderIssueFooter()
	if !strings.Contains(got, "tab") {
		t.Errorf("expected 'tab' in footer, got:\n%s", got)
	}
	if !strings.Contains(got, "detail") {
		t.Errorf("expected 'detail' in footer, got:\n%s", got)
	}
}

func TestRenderIssueFooter_DetailFocused(t *testing.T) {
	m := newTestModel(withItems(3), withSize(120, 30))
	m.activeView = ViewIssues
	m.issueState = &IssueViewState{
		Issues:        []*tracker.Issue{{Number: 1, Title: "T", Author: "u"}},
		FilterInput:   newIssueFilterInput(),
		DetailFocused: true,
	}
	got := m.renderIssueFooter()
	if !strings.Contains(got, "scroll") {
		t.Errorf("expected 'scroll' in footer, got:\n%s", got)
	}
}

func TestFormatIssueAge(t *testing.T) {
	tests := []struct {
		name string
		age  time.Duration
		want string
	}{
		{"minutes", 30 * time.Minute, "30m ago"},
		{"hours", 5 * time.Hour, "5h ago"},
		{"days", 48 * time.Hour, "2d ago"},
		{"weeks", 14 * 24 * time.Hour, "2w ago"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatIssueAge(time.Now().Add(-tt.age))
			if got != tt.want {
				t.Errorf("formatIssueAge() = %q, want %q", got, tt.want)
			}
		})
	}
}
