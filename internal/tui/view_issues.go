package tui

import (
	"fmt"
	"strings"
	"time"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/textinput"
	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	lipgloss "charm.land/lipgloss/v2"

	"github.com/lost-in-the/grove/internal/state"
	"github.com/lost-in-the/grove/internal/worktree"
	"github.com/lost-in-the/grove/plugins/tracker"
)

// IssueViewState holds the state for the issue browser view.
type IssueViewState struct {
	Issues         []*tracker.Issue
	Cursor         int
	Loading        bool
	Error          string
	Creating       bool
	ActivityLog    *ActivityLog // streaming creation progress
	FilterInput    textinput.Model
	Filtering      bool // true when filter input is active (activated by /)
	DetailFocused  bool // true when detail panel has focus (Tab to toggle)
	DetailViewport viewport.Model
	lastCursor     int // tracks cursor changes to update viewport content
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

	// When filtering is active, route keys to the textinput
	if s.Filtering {
		switch {
		case key.Matches(msg, m.keys.Escape):
			s.Filtering = false
			s.FilterInput.Blur()
			s.FilterInput.SetValue("")
			s.Cursor = 0
			return m, nil
		case key.Matches(msg, m.keys.Enter):
			s.Filtering = false
			s.FilterInput.Blur()
			return m, nil
		default:
			prevVal := s.FilterInput.Value()
			var cmd tea.Cmd
			s.FilterInput, cmd = s.FilterInput.Update(msg)
			if s.FilterInput.Value() != prevVal {
				s.Cursor = 0
			}
			return m, cmd
		}
	}

	// When detail panel is focused, route keys for scrolling
	if s.DetailFocused {
		switch {
		case key.Matches(msg, m.keys.Escape):
			s.DetailFocused = false
			return m, nil
		case key.Matches(msg, m.keys.Tab):
			s.DetailFocused = false
			return m, nil
		case key.Matches(msg, m.keys.Up):
			s.DetailViewport.ScrollUp(1)
			return m, nil
		case key.Matches(msg, m.keys.Down):
			s.DetailViewport.ScrollDown(1)
			return m, nil
		case msg.String() == "g":
			s.DetailViewport.GotoTop()
			return m, nil
		case msg.String() == "G":
			s.DetailViewport.GotoBottom()
			return m, nil
		case msg.String() == "ctrl+u":
			s.DetailViewport.HalfPageUp()
			return m, nil
		case msg.String() == "ctrl+d":
			s.DetailViewport.HalfPageDown()
			return m, nil
		default:
			// Swallow all other keys while detail is focused
			return m, nil
		}
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

	case key.Matches(msg, m.keys.Filter):
		s.Filtering = true
		return m, s.FilterInput.Focus()

	case key.Matches(msg, m.keys.Enter):
		if len(filtered) > 0 && s.Cursor < len(filtered) {
			issue := filtered[s.Cursor]
			s.Creating = true
			s.Error = ""
			s.ActivityLog = NewActivityLog(60, 10)
			name := tracker.GenerateWorktreeName("issue", issue.Number, issue.Title)
			return m, tea.Batch(m.spinner.Tick, createIssueWorktreeCmd(m.worktreeMgr, m.stateMgr, m.projectRoot, name))
		}
		return m, nil
	}

	return m, nil
}

func createIssueWorktreeCmd(mgr *worktree.Manager, stateMgr *state.Manager, projectRoot, name string) tea.Cmd {
	ch := make(chan creationEvent, 10)

	go func() {
		defer close(ch)

		ch <- creationEvent{line: fmt.Sprintf("Creating worktree '%s'...", name)}
		ch <- creationEvent{line: fmt.Sprintf("Creating branch '%s'...", name)}

		err := mgr.Create(name, name)
		if err != nil {
			ch <- creationEvent{done: true, name: name, err: err}
			return
		}

		wt, err := mgr.Find(name)
		if err != nil || wt == nil {
			ch <- creationEvent{done: true, name: name, err: errWorktreeNotFound}
			return
		}

		result := runPostCreateStreaming(ch, mgr, stateMgr, projectRoot, name, wt)
		ch <- creationEvent{
			done:       true,
			name:       name,
			path:       wt.Path,
			hookOutput: result.hookOutput,
			hookErr:    result.hookErr,
		}
	}()

	return readCreationLog(ch, "issue")
}

// renderIssueView renders the issue browser overlay.
func renderIssueView(s *IssueViewState, width int, spinnerView string, footer string) string {
	if s.Loading {
		return Styles.OverlayBorderInfo.Render(
			Styles.OverlayTitle.Render("Issues") + "\n\n" +
				spinnerView + " Loading issues...",
		)
	}

	if s.Creating {
		var content strings.Builder
		if s.ActivityLog != nil {
			content.WriteString(s.ActivityLog.View(spinnerView))
		} else {
			content.WriteString(spinnerView + " Creating worktree from issue...\n")
		}
		content.WriteString("\n" + Styles.Footer.Render("Please wait..."))
		return Styles.OverlayBorderInfo.Render(
			Styles.OverlayTitle.Render("Issues") + "\n\n" + content.String(),
		)
	}

	var b strings.Builder

	if s.Error != "" {
		b.WriteString(Styles.ErrorText.Render(s.Error) + "\n\n")
	}

	filter := s.FilterInput.Value()
	filtered := filteredIssues(s.Issues, filter)

	// Detail preview is always visible in the panel layout (renderIssuePanel).
	// The overlay view shows the list only.
	total := len(s.Issues)

	// Always render the filter/count bar for consistent height
	if s.Filtering || filter != "" {
		b.WriteString(s.FilterInput.View())
		if filter != "" {
			fmt.Fprintf(&b, "  %s", Styles.DetailDim.Render(fmt.Sprintf("%d of %d", len(filtered), total)))
		}
		b.WriteString("\n\n")
	} else {
		if total > 0 {
			b.WriteString(Styles.DetailDim.Render(fmt.Sprintf("%d open", total)) + "\n\n")
		} else {
			b.WriteString("\n\n")
		}
	}

	// Fixed number of item display slots to prevent height jitter during filtering.
	// Each item takes 3 lines (title + metadata + blank separator), except the last
	// which takes 2 lines (no trailing blank).
	const issueDisplaySlots = 8
	const issueSlotLines = issueDisplaySlots*3 - 1 // 23 lines for 8 items

	var itemContent strings.Builder

	if len(filtered) == 0 {
		itemContent.WriteString(Styles.DetailDim.Render("  (no matching issues)") + "\n")
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
			fmt.Fprintf(&itemContent, "%s%s %s\n", cursor, number, titleStr)

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

			fmt.Fprintf(&itemContent, "%s%s · %s%s\n", indent, author, age, labels)

			// Blank line between items (except last)
			if i < end-1 {
				itemContent.WriteString("\n")
			}
		}

		if end < len(filtered) {
			itemContent.WriteString(Styles.DetailDim.Render(fmt.Sprintf("\n  … and %d more", len(filtered)-end)) + "\n")
		}
	}

	b.WriteString(padToHeight(itemContent.String(), issueSlotLines))

	b.WriteString("\n" + footer)

	return Styles.OverlayBorderInfo.Render(
		Styles.OverlayTitle.Render("Issues") + "\n\n" + b.String(),
	)
}

// renderIssuePreview renders a detailed preview panel for a single issue.
func renderIssuePreview(issue *tracker.Issue, width int, footer string) string {
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
	b.WriteString(footer)

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

// renderIssuePanel renders the issue view as a full-screen panel layout
// with a list on one side and a detail preview on the other.
func (m Model) renderIssuePanel() string {
	s := m.issueState
	if s == nil {
		return ""
	}

	// Header
	statusBar := renderContextHeader("Issues", m.projectName, m.width)

	// Footer: context-aware based on focus state
	footer := m.renderIssueFooter()

	footerLines := strings.Count(footer, "\n") + 1
	bodyBudget := m.height - 1 - footerLines

	useSideBySide := m.width > 100
	bodyWidth := m.width - 2

	filter := s.FilterInput.Value()
	filtered := filteredIssues(s.Issues, filter)

	var body string
	if useSideBySide {
		listWidth := bodyWidth * 40 / 100
		dividerWidth := 3
		detailWidth := bodyWidth - listWidth - dividerWidth

		listContent := renderIssueList(s, listWidth, m.spinner.View(), bodyBudget)
		listContent = lipgloss.NewStyle().Width(listWidth).Render(listContent)

		dividerHeight := bodyBudget
		if dividerHeight < 1 {
			dividerHeight = 1
		}
		divider := renderVerticalDivider(dividerHeight, Colors.SurfaceDim)

		var detailView string
		if s.Loading {
			detailView = m.spinner.View() + " Loading issues..."
		} else if s.Creating {
			detailView = renderCreatingDetail(s.ActivityLog, m.spinner.View(), "Creating worktree from issue...")
		} else if len(filtered) > 0 && s.Cursor < len(filtered) {
			detailView = m.renderIssueDetailViewport(filtered[s.Cursor], detailWidth, bodyBudget)
		} else {
			detailView = Styles.DetailDim.Render("No issue selected")
		}

		body = lipgloss.JoinHorizontal(lipgloss.Top, listContent, divider, detailView)
	} else {
		listHeight := bodyBudget * 40 / 100
		if listHeight < 6 {
			listHeight = 6
		}

		listContent := renderIssueList(s, bodyWidth, m.spinner.View(), listHeight)

		separatorName := ""
		if len(filtered) > 0 && s.Cursor < len(filtered) {
			issue := filtered[s.Cursor]
			separatorName = fmt.Sprintf("#%d %s", issue.Number, truncate(issue.Title, bodyWidth-20))
		}
		separator := renderNamedSeparator(separatorName, bodyWidth)

		detailHeight := bodyBudget - listHeight - 1 // 1 for separator
		var detailView string
		if s.Loading {
			detailView = m.spinner.View() + " Loading issues..."
		} else if s.Creating {
			detailView = renderCreatingDetail(s.ActivityLog, m.spinner.View(), "Creating worktree from issue...")
		} else if len(filtered) > 0 && s.Cursor < len(filtered) {
			detailView = m.renderIssueDetailViewport(filtered[s.Cursor], bodyWidth, detailHeight)
		} else {
			detailView = Styles.DetailDim.Render("No issue selected")
		}

		body = lipgloss.JoinVertical(lipgloss.Left, listContent, separator, detailView)
	}

	// Toast on header
	if m.toast != nil && m.toast.Current != nil {
		toastView := m.toast.View(m.width)
		if toastView != "" {
			statusBar = compositeToastOnHeader(statusBar, toastView, m.width)
		}
	}

	body = lipgloss.NewStyle().Padding(0, 1).Render(body)

	if m.height > 0 && bodyBudget > 0 {
		body = clampLines(body, bodyBudget)
	}

	return lipgloss.JoinVertical(lipgloss.Left, statusBar, body, footer)
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
	if s.Filtering || filter != "" {
		b.WriteString(s.FilterInput.View())
		if filter != "" {
			fmt.Fprintf(&b, "  %s", Styles.DetailDim.Render(fmt.Sprintf("%d of %d", len(filtered), total)))
		}
		b.WriteString("\n\n")
	} else {
		if total > 0 {
			b.WriteString(Styles.DetailDim.Render(fmt.Sprintf("%d open", total)) + "\n\n")
		} else {
			b.WriteString("\n\n")
		}
	}

	headerLines := strings.Count(b.String(), "\n")
	availableLines := maxHeight - headerLines
	if availableLines < 3 {
		availableLines = 3
	}
	displaySlots := (availableLines + 1) / 3
	if displaySlots < 3 {
		displaySlots = 3
	}

	contentWidth := width - 2
	if contentWidth < 40 {
		contentWidth = 40
	}

	if len(filtered) == 0 {
		b.WriteString(Styles.DetailDim.Render("  (no matching issues)") + "\n")
	} else {
		start, end := scrollWindow(len(filtered), s.Cursor, displaySlots)

		for i := start; i < end; i++ {
			issue := filtered[i]

			cursor := "  "
			if i == s.Cursor {
				cursor = Styles.ListCursor.Render("❯ ")
			}

			number := Styles.DetailDim.Render(fmt.Sprintf("#%-5d", issue.Number))
			titleStr := truncate(issue.Title, contentWidth-20)
			fmt.Fprintf(&b, "%s%s %s\n", cursor, number, titleStr)

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

		if end < len(filtered) {
			b.WriteString(Styles.DetailDim.Render(fmt.Sprintf("\n  … and %d more", len(filtered)-end)) + "\n")
		}
	}

	return b.String()
}

// renderIssueDetailPanel renders an issue detail card for the panel layout,
// wrapped in a DetailBorder with a title.
func renderIssueDetailPanel(issue *tracker.Issue, width int) string {
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

	body := strings.Join(sections, "\n\n")

	card := Styles.DetailBorder.
		Width(width - 2).
		Render(body)

	titleLabel := " " + Styles.DetailTitle.Render(fmt.Sprintf("#%d", issue.Number)) + " "
	card = injectBorderTitle(card, titleLabel)

	return card
}

// renderIssueDetailContent renders the inner content of an issue detail panel
// without the border wrapper. Used for viewport content.
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

	vpWidth := width - 4
	if vpWidth < 16 {
		vpWidth = 16
	}
	vpHeight := height - 2
	if vpHeight < 1 {
		vpHeight = 1
	}
	s.DetailViewport.SetWidth(vpWidth)
	s.DetailViewport.SetHeight(vpHeight)

	// Update viewport content when cursor changes
	if s.Cursor != s.lastCursor || s.DetailViewport.GetContent() == "" {
		content := renderIssueDetailContent(issue, width)
		s.DetailViewport.SetContent(content)
		s.DetailViewport.GotoTop()
		s.lastCursor = s.Cursor
	}

	// Choose border style based on focus
	borderStyle := Styles.DetailBorder
	if s.DetailFocused {
		borderStyle = Styles.DetailBorder.BorderForeground(Colors.Primary)
	}

	card := borderStyle.
		Width(width - 2).
		Render(s.DetailViewport.View())

	titleLabel := " " + Styles.DetailTitle.Render(fmt.Sprintf("#%d", issue.Number)) + " "
	if s.DetailFocused {
		card = injectBorderTitleWithColor(card, titleLabel, Colors.Primary)
	} else {
		card = injectBorderTitle(card, titleLabel)
	}

	return card
}

// renderIssueFooter returns context-aware footer hints for the issue view.
func (m Model) renderIssueFooter() string {
	s := m.issueState
	if s != nil && s.DetailFocused {
		return m.helpFooter.RenderCompactWithHints(issueDetailFocusedHints(), m.width-4)
	}
	return m.helpFooter.RenderCompact(ViewIssues, m.width-4)
}

// issueDetailFocusedHints returns hints for when the issue detail panel is focused.
func issueDetailFocusedHints() []Hint {
	return []Hint{
		{"↑↓", "scroll"},
		{"g/G", "top/bottom"},
		{"tab", "list"},
		{"esc", "back"},
	}
}
