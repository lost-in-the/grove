package tui

import (
	"fmt"
	"strings"
	"time"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/textinput"
	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"

	"github.com/lost-in-the/grove/internal/git"
	"github.com/lost-in-the/grove/plugins/tracker"
)

// IssueViewState holds the state for the issue browser view.
type IssueViewState struct {
	Issues           []*tracker.Issue
	Cursor           int
	Loading          bool
	Error            string
	Creating         bool
	ActivityLog      *ActivityLog // streaming creation progress
	FilterInput      textinput.Model
	Filtering        bool // true when filter input is active (activated by /)
	DetailFocused    bool // true when detail panel has focus (Tab to toggle)
	DetailViewport   viewport.Model
	lastCursor       int               // tracks cursor changes to update viewport content
	WorktreeBranches map[string]string // branch → worktree short name
	ExistsPrompt     *ExistingWorktreePrompt
}

func (s *IssueViewState) getActivityLog() *ActivityLog { return s.ActivityLog }
func (s *IssueViewState) setCreatingDone(errMsg string) {
	s.Creating = false
	s.Error = errMsg
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

	if s.ExistsPrompt != nil {
		return m.handleExistsPromptKey(msg, s.ExistsPrompt,
			func() { s.ExistsPrompt = nil },
		)
	}

	if s.Loading || s.Creating {
		if key.Matches(msg, m.keys.Escape) {
			m.activeView = ViewDashboard
			m.issueState = nil
			return m, nil
		}
		return m, nil
	}

	filtered := filteredIssues(s.Issues, s.FilterInput.Value())

	// When filtering is active, route keys to the textinput
	if s.Filtering {
		var cmd tea.Cmd
		var stop bool
		s.FilterInput, s.Cursor, cmd, stop = handleListFilterKey(msg, m.keys, s.FilterInput, s.Cursor)
		if stop {
			s.Filtering = false
		}
		return m, cmd
	}

	// When detail panel is focused, route keys for scrolling
	if s.DetailFocused {
		if key.Matches(msg, m.keys.Escape) || key.Matches(msg, m.keys.Tab) {
			s.DetailFocused = false
			return m, nil
		}
		if msg.String() == "B" {
			m.openSelectedIssueURL(filtered)
			return m, nil
		}
		handleDetailFocusedKey(msg, m.keys, &s.DetailViewport)
		return m, nil
	}

	// Normal mode (not filtering, list focused)
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
		s.DetailFocused = true
		return m, nil

	case msg.String() == "B":
		m.openSelectedIssueURL(filtered)
		return m, nil

	case key.Matches(msg, m.keys.Filter):
		s.Filtering = true
		return m, s.FilterInput.Focus()

	case key.Matches(msg, m.keys.Enter):
		if len(filtered) > 0 && s.Cursor < len(filtered) {
			issue := filtered[s.Cursor]
			name := fmt.Sprintf("issue-%d", issue.Number)

			// Check if a worktree already exists with this name
			if existing := checkDuplicateWorktree(name, m.existingWorktreeItems()); existing != nil {
				s.ExistsPrompt = &ExistingWorktreePrompt{
					WorktreeName: existing.ShortName,
					Branch:       existing.Branch,
					ItemLabel:    fmt.Sprintf("Issue #%d", issue.Number),
				}
				return m, nil
			}

			return m.openCreateWizardForIssue(issue)
		}
		return m, nil
	}

	return m, nil
}

func (m Model) openCreateWizardForIssue(issue *tracker.Issue) (tea.Model, tea.Cmd) {
	branches, _ := git.ListAllBranches(m.projectRoot)
	m.createState = prefillCreateStateForIssue(issue, m.projectName, branches)
	m.createState.ReturnView = ViewIssues
	m.createState.WorktreeBranches = m.worktreeBranchMap()
	m.activeView = ViewCreate
	return m, nil
}

// renderIssueItemList renders a scrollable list of issue items with cursor, scroll window, and overflow indicator.
func renderIssueItemList(issues []*tracker.Issue, cursor int, scrollSize int, contentWidth int) string {
	var b strings.Builder
	if len(issues) == 0 {
		b.WriteString(Styles.DetailDim.Render("  (no matching issues)") + "\n")
		return b.String()
	}

	start, end := scrollWindow(len(issues), cursor, scrollSize)

	for i := start; i < end; i++ {
		issue := issues[i]

		cur := "  "
		if i == cursor {
			cur = Styles.ListCursor.Render("❯ ")
		}

		number := Styles.DetailDim.Render(fmt.Sprintf("#%-5d", issue.Number))
		titleStr := truncate(issue.Title, contentWidth-20)
		fmt.Fprintf(&b, "%s%s %s\n", cur, number, titleStr)

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

		if i < end-1 {
			b.WriteString("\n")
		}
	}

	if end < len(issues) {
		b.WriteString(Styles.DetailDim.Render(fmt.Sprintf("\n  … and %d more", len(issues)-end)) + "\n")
	}

	return b.String()
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

// renderIssuePanel renders the issue view as a full-screen panel layout
// with a list on one side and a detail preview on the other.
func (m Model) renderIssuePanel() string {
	s := m.issueState
	if s == nil {
		return ""
	}

	filter := s.FilterInput.Value()
	filtered := filteredIssues(s.Issues, filter)

	separatorName := ""
	if len(filtered) > 0 && s.Cursor < len(filtered) {
		issue := filtered[s.Cursor]
		separatorName = fmt.Sprintf("#%d %s", issue.Number, truncate(issue.Title, m.width-22))
	}

	panel := m.renderContextPanel(contextPanelConfig{
		contextLabel: "Issues",
		footer:       m.renderIssueFooter(),
		renderList: func(width int, spinnerView string, maxHeight int) string {
			return renderIssueList(s, width, spinnerView, maxHeight)
		},
		renderDetail: func(width, height int) string {
			if s.Loading {
				return ""
			}
			if s.Creating {
				return renderCreatingDetail(s.ActivityLog, m.spinner.View(), "Creating worktree from issue...")
			}
			if len(filtered) > 0 && s.Cursor < len(filtered) {
				return m.renderIssueDetailViewport(filtered[s.Cursor], width, height)
			}
			return Styles.DetailDim.Render("No issue selected")
		},
		separatorName: separatorName,
	})

	if s.ExistsPrompt != nil {
		return centerOverlay(panel, renderExistingWorktreePrompt(s.ExistsPrompt, m.width), m.width, m.height)
	}
	return panel
}

// renderIssueList renders the issue list content for the panel layout.
func renderIssueList(s *IssueViewState, width int, spinnerView string, maxHeight int) string {
	var b strings.Builder

	if s.Loading {
		b.WriteString(spinnerView + " Loading issues...")
		return b.String()
	}

	if s.Error != "" {
		b.WriteString(Styles.ErrorText.Render(s.Error) + "\n")
	}

	filter := s.FilterInput.Value()
	filtered := filteredIssues(s.Issues, filter)
	total := len(s.Issues)

	// Filter/count bar
	renderFilterBar(&b, s.FilterInput.View(), s.Filtering, filter, len(filtered), total)

	displaySlots, contentWidth := calcDisplaySlots(strings.Count(b.String(), "\n"), maxHeight, width)
	b.WriteString(renderIssueItemList(filtered, s.Cursor, displaySlots, contentWidth))

	return b.String()
}

// renderIssueDetailContent renders the inner content of an issue detail panel.
// Used for viewport content.
func renderIssueDetailContent(issue *tracker.Issue, width int) string {
	innerWidth := max(width-6, 16)

	var sections []string

	// Title
	sections = append(sections, Styles.DetailTitle.Render(truncate(issue.Title, innerWidth)))

	// Metadata grid
	const labelWidth = 10
	label := func(s string) string {
		return Styles.DetailLabel.Render(padRight(s, labelWidth))
	}

	var metaRows []string
	metaRows = append(metaRows, renderSectionHeader("Issue Info", innerWidth))
	metaRows = append(metaRows, label("Author")+Styles.DetailValue.Render("@"+issue.Author))

	if len(issue.Labels) > 0 {
		labelStrs := make([]string, len(issue.Labels))
		for i, l := range issue.Labels {
			labelStrs[i] = Styles.WarningText.Render(l)
		}
		metaRows = append(metaRows, label("Labels")+strings.Join(labelStrs, ", "))
	}
	if !issue.CreatedAt.IsZero() {
		metaRows = append(metaRows, label("Opened")+Styles.DetailValue.Render(formatIssueAge(issue.CreatedAt)))
	}
	sections = append(sections, strings.Join(metaRows, "\n"))

	// Body
	if issue.Body != "" {
		bodyHeader := renderSectionHeader("Description", innerWidth)
		rendered := renderMarkdown(issue.Body, innerWidth)
		sections = append(sections, bodyHeader+"\n"+rendered)
	}

	return strings.Join(sections, "\n\n")
}

// renderIssueDetailViewport renders the issue detail using the viewport for scrolling.
func (m Model) renderIssueDetailViewport(issue *tracker.Issue, width, height int) string {
	s := m.issueState
	return renderDetailViewportCard(detailViewportConfig{
		vp:         &s.DetailViewport,
		cursor:     s.Cursor,
		lastCursor: &s.lastCursor,
		focused:    s.DetailFocused,
		itemNumber: issue.Number,
		contentFunc: func(w int) string {
			return renderIssueDetailContent(issue, w)
		},
		width:  width,
		height: height,
	})
}

// openSelectedIssueURL opens the URL of the currently selected issue in the browser.
func (m *Model) openSelectedIssueURL(filtered []*tracker.Issue) {
	s := m.issueState
	if s != nil && len(filtered) > 0 && s.Cursor < len(filtered) {
		if url := filtered[s.Cursor].URL; url != "" {
			openURL(url)
		}
	}
}

// renderIssueFooter returns context-aware footer hints for the issue view.
func (m Model) renderIssueFooter() string {
	s := m.issueState
	if s != nil && s.DetailFocused {
		return m.helpFooter.RenderCompactWithHints(detailFocusedHints(), m.width-4)
	}
	return m.helpFooter.RenderCompact(ViewIssues, m.width-4)
}
