package tui

import (
	"fmt"
	"strings"
	"time"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"

	"github.com/lost-in-the/grove/internal/state"
	"github.com/lost-in-the/grove/internal/worktree"
	"github.com/lost-in-the/grove/plugins/tracker"
)

// IssueViewState holds the state for the issue browser view.
type IssueViewState struct {
	Issues      []*tracker.Issue
	Cursor      int
	Loading     bool
	Error       string
	Creating    bool
	FilterInput textinput.Model
	ShowPreview bool
}

// newIssueFilterInput creates a configured textinput for issue filtering.
func newIssueFilterInput() textinput.Model {
	ti := textinput.New()
	ti.Prompt = "Filter: "
	ti.Placeholder = ""
	ti.CharLimit = 100
	return ti
}

func (m Model) fetchIssuesCmd() tea.Msg {
	if !tracker.IsGHInstalled() {
		return issuesFetchedMsg{err: fmt.Errorf("gh CLI not installed or not authenticated")}
	}

	repo, err := tracker.DetectRepo()
	if err != nil {
		return issuesFetchedMsg{err: fmt.Errorf("failed to detect repository: %w", err)}
	}

	gh := tracker.NewGitHubAdapter(repo)
	issues, err := gh.ListIssues(tracker.ListOptions{State: "open", Limit: 30})
	return issuesFetchedMsg{issues: issues, err: err}
}

func (m Model) handleIssueKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	if m.issueState == nil {
		m.activeView = ViewDashboard
		return m, nil
	}

	s := m.issueState

	if s.Loading || s.Creating {
		if key.Matches(msg, m.keys.Escape) {
			m.activeView = ViewDashboard
			m.issueState = nil
			return m, nil
		}
		return m, nil
	}

	filtered := filteredIssues(s.Issues, s.FilterInput.Value())

	switch {
	case key.Matches(msg, m.keys.Escape):
		m.activeView = ViewDashboard
		m.issueState = nil
		return m, nil

	case key.Matches(msg, m.keys.Up):
		if s.Cursor > 0 {
			s.Cursor--
		}
		return m, nil

	case key.Matches(msg, m.keys.Down):
		if s.Cursor < len(filtered)-1 {
			s.Cursor++
		}
		return m, nil

	case key.Matches(msg, m.keys.Tab):
		s.ShowPreview = !s.ShowPreview
		return m, nil

	case key.Matches(msg, m.keys.Enter):
		if len(filtered) > 0 && s.Cursor < len(filtered) {
			issue := filtered[s.Cursor]
			s.Creating = true
			s.Error = ""
			name := tracker.GenerateWorktreeName("issue", issue.Number, issue.Title)
			return m, tea.Batch(m.spinner.Tick, createIssueWorktreeCmd(m.worktreeMgr, m.stateMgr, m.projectRoot, name))
		}
		return m, nil

	default:
		// Route remaining keys through the filter textinput
		prevVal := s.FilterInput.Value()
		var cmd tea.Cmd
		s.FilterInput, cmd = s.FilterInput.Update(msg)
		if s.FilterInput.Value() != prevVal {
			s.Cursor = 0
		}
		return m, cmd
	}
}

func createIssueWorktreeCmd(mgr *worktree.Manager, stateMgr *state.Manager, projectRoot, name string) tea.Cmd {
	return func() tea.Msg {
		err := mgr.Create(name, name)
		if err != nil {
			return issueWorktreeCreatedMsg{name: name, err: err}
		}
		wt, err := mgr.Find(name)
		if err != nil || wt == nil {
			return issueWorktreeCreatedMsg{name: name, err: errWorktreeNotFound}
		}

		result := runPostCreate(mgr, stateMgr, projectRoot, name, wt)
		return issueWorktreeCreatedMsg{name: name, path: wt.Path, hookOutput: result.hookOutput, hookErr: result.hookErr}
	}
}

// renderIssueView renders the issue browser overlay.
func renderIssueView(s *IssueViewState, width int, spinnerView string) string {
	if s.Loading {
		return Styles.OverlayBorderInfo.Render(
			Styles.OverlayTitle.Render("Issues") + "\n\n" +
				spinnerView + " Loading issues...",
		)
	}

	if s.Creating {
		return Styles.OverlayBorderInfo.Render(
			Styles.OverlayTitle.Render("Issues") + "\n\n" +
				spinnerView + " Creating worktree from issue...",
		)
	}

	var b strings.Builder

	if s.Error != "" {
		b.WriteString(Styles.ErrorText.Render(s.Error) + "\n\n")
	}

	filter := s.FilterInput.Value()
	filtered := filteredIssues(s.Issues, filter)

	// If preview mode and we have a selected issue, render preview
	if s.ShowPreview && len(filtered) > 0 && s.Cursor < len(filtered) {
		return renderIssuePreview(filtered[s.Cursor], width)
	}

	total := len(s.Issues)

	// Filter bar with count
	if filter != "" {
		b.WriteString(s.FilterInput.View())
		fmt.Fprintf(&b, "  %s", Styles.DetailDim.Render(fmt.Sprintf("%d of %d", len(filtered), total)))
		b.WriteString("\n\n")
	} else if total > 0 {
		b.WriteString(Styles.DetailDim.Render(fmt.Sprintf("%d open", total)) + "\n\n")
	}

	if len(filtered) == 0 {
		b.WriteString(Styles.DetailDim.Render("  (no matching issues)") + "\n")
	} else {
		start, end := scrollWindow(len(filtered), s.Cursor, 10)

		contentWidth := width - 8
		if contentWidth < 40 {
			contentWidth = 40
		}

		for i := start; i < end; i++ {
			issue := filtered[i]

			cursor := "  "
			if i == s.Cursor {
				cursor = Styles.ListCursor.Render("❯ ")
			}

			// Line 1: cursor + #number + title
			number := Styles.DetailDim.Render(fmt.Sprintf("#%-5d", issue.Number))
			titleStr := truncate(issue.Title, contentWidth-20)
			fmt.Fprintf(&b, "%s%s %s\n", cursor, number, titleStr)

			// Line 2: metadata indent + author + age + labels
			indent := "         "
			author := Styles.DetailDim.Render("@" + issue.Author)
			age := Styles.DetailDim.Render(formatIssueAge(issue.CreatedAt))

			var labelParts []string
			for _, l := range issue.Labels {
				labelParts = append(labelParts, Styles.WarningText.Render(l))
			}
			labels := ""
			if len(labelParts) > 0 {
				labels = "  " + strings.Join(labelParts, ", ")
			}

			fmt.Fprintf(&b, "%s%s · %s%s\n", indent, author, age, labels)

			// Blank line between items (except last)
			if i < end-1 {
				b.WriteString("\n")
			}
		}

		if end < len(filtered) {
			b.WriteString(Styles.DetailDim.Render(fmt.Sprintf("\n  … and %d more", len(filtered)-end)) + "\n")
		}
	}

	b.WriteString("\n" + Styles.Footer.Render("[enter] create worktree  [tab] preview  [esc] close  type to filter"))

	return Styles.OverlayBorderInfo.Render(
		Styles.OverlayTitle.Render("Issues") + "\n\n" + b.String(),
	)
}

// renderIssuePreview renders a detailed preview panel for a single issue.
func renderIssuePreview(issue *tracker.Issue, width int) string {
	contentWidth := max(width-6, 30)

	var b strings.Builder

	// Title
	b.WriteString(Styles.OverlayTitle.Render(fmt.Sprintf("#%d  %s", issue.Number, issue.Title)))
	b.WriteString("\n\n")

	// Metadata row
	meta := []string{
		Styles.DetailDim.Render("Author: ") + "@" + issue.Author,
	}
	if len(issue.Labels) > 0 {
		labelStrs := make([]string, len(issue.Labels))
		for i, l := range issue.Labels {
			labelStrs[i] = Styles.WarningText.Render(l)
		}
		meta = append(meta, Styles.DetailDim.Render("Labels: ")+strings.Join(labelStrs, ", "))
	}
	if !issue.CreatedAt.IsZero() {
		meta = append(meta, Styles.DetailDim.Render("Opened: ")+formatIssueAge(issue.CreatedAt))
	}
	b.WriteString(strings.Join(meta, "  ·  "))
	b.WriteString("\n")
	b.WriteString(Styles.DetailDim.Render(strings.Repeat("─", contentWidth)))
	b.WriteString("\n\n")

	// Body
	if issue.Body == "" {
		b.WriteString(Styles.DetailDim.Render("No description provided."))
	} else {
		rendered := renderMarkdown(issue.Body, contentWidth)
		b.WriteString(rendered)
	}

	b.WriteString("\n\n")
	b.WriteString(Styles.Footer.Render("[enter] Create worktree  [tab] Back  [esc] Close"))

	return Styles.OverlayBorderInfo.Render(b.String())
}

func filteredIssues(issues []*tracker.Issue, filter string) []*tracker.Issue {
	if filter == "" {
		return issues
	}
	lower := strings.ToLower(filter)
	var result []*tracker.Issue
	for _, issue := range issues {
		if strings.Contains(strings.ToLower(issue.Title), lower) ||
			strings.Contains(strings.ToLower(issue.Author), lower) ||
			strings.Contains(fmt.Sprintf("#%d", issue.Number), filter) ||
			matchesLabel(issue.Labels, lower) {
			result = append(result, issue)
		}
	}
	return result
}

func matchesLabel(labels []string, lower string) bool {
	for _, l := range labels {
		if strings.Contains(strings.ToLower(l), lower) {
			return true
		}
	}
	return false
}

func formatIssueAge(t time.Time) string {
	d := time.Since(t)
	switch {
	case d < time.Hour:
		return fmt.Sprintf("%dm ago", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%dh ago", int(d.Hours()))
	case d < 7*24*time.Hour:
		return fmt.Sprintf("%dd ago", int(d.Hours()/24))
	default:
		return fmt.Sprintf("%dw ago", int(d.Hours()/(24*7)))
	}
}
