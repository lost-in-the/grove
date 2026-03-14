package tui

import (
	"fmt"
	"strings"

	lipgloss "charm.land/lipgloss/v2"

	"github.com/lost-in-the/grove/plugins/tracker"
)

// prOverlayWidth returns a fixed overlay width based on terminal width.
func prOverlayWidth(termWidth int) int {
	w := termWidth - 8
	if w > 100 {
		w = 100
	}
	if w < 50 {
		w = 50
	}
	return w
}

// renderPRViewV2 renders the PR browser with two-line items, draft labels,
// diff stats, and worktree badges.
func renderPRViewV2(s *PRViewState, width int, spinnerView string, footer string) string {
	overlayWidth := prOverlayWidth(width)

	if s.Loading {
		return Styles.OverlayBorderInfo.Width(overlayWidth).Render(
			Styles.OverlayTitle.Render("Pull Requests") + "\n\n" +
				spinnerView + " Loading PRs...",
		)
	}

	if s.Creating {
		var content strings.Builder
		if s.CreatingPR != nil {
			content.WriteString(Styles.DetailDim.Render(fmt.Sprintf("PR #%d: %s",
				s.CreatingPR.Number, truncate(s.CreatingPR.Title, 40))) + "\n\n")
		}
		if s.ActivityLog != nil {
			content.WriteString(s.ActivityLog.View(spinnerView))
		} else {
			content.WriteString(spinnerView + " Creating worktree from PR...\n")
		}
		content.WriteString("\n" + Styles.Footer.Render("Please wait..."))
		return Styles.OverlayBorderInfo.Width(overlayWidth).Render(
			Styles.OverlayTitle.Render("Pull Requests") + "\n\n" + content.String(),
		)
	}

	var b strings.Builder

	if s.Error != "" {
		b.WriteString(Styles.ErrorText.Render(s.Error) + "\n\n")
	}

	filter := s.FilterInput.Value()
	filtered := filteredPRs(s.PRs, filter)

	// Detail preview is always visible in the panel layout (renderPRPanel).
	// The overlay view shows the list only.
	total := len(s.PRs)

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
	const prDisplaySlots = 8
	const prSlotLines = prDisplaySlots*3 - 1 // 23 lines for 8 items

	var itemContent strings.Builder

	if len(filtered) == 0 {
		itemContent.WriteString(Styles.DetailDim.Render("  (no matching PRs)") + "\n")
	} else {
		start, end := scrollWindow(len(filtered), s.Cursor, 10)

		contentWidth := width - 8 // padding from overlay border
		if contentWidth < 40 {
			contentWidth = 40
		}

		for i := start; i < end; i++ {
			pr := filtered[i]

			cursor := "  "
			if i == s.Cursor {
				cursor = Styles.ListCursor.Render("❯ ")
			}

			// Line 1: cursor + #number + title + branch
			number := Styles.DetailDim.Render(fmt.Sprintf("#%-5d", pr.Number))
			titleStr := pr.Title
			if pr.IsDraft {
				titleStr = Styles.WarningText.Render("[DRAFT]") + " " + titleStr
			}
			titleStr = truncate(titleStr, contentWidth-30)
			branch := Styles.DetailDim.Render(truncate(pr.Branch, 20))
			fmt.Fprintf(&itemContent, "%s%s %s  %s\n", cursor, number, titleStr, branch)

			// Line 2: metadata indent + author + commits + diff stats + worktree badge
			indent := "         " // align with title after cursor+number
			author := Styles.DetailDim.Render("@" + pr.Author)
			commits := Styles.DetailDim.Render(formatCommitCount(pr.CommitCount))
			diffStats := formatDiffStats(pr.Additions, pr.Deletions)

			badge := ""
			if s.WorktreeBranches[pr.Branch] {
				badge = "  " + Styles.SuccessText.Render("✓ worktree")
			}

			fmt.Fprintf(&itemContent, "%s%s · %s · %s%s\n", indent, author, commits, diffStats, badge)

			// Blank line between items (except last)
			if i < end-1 {
				itemContent.WriteString("\n")
			}
		}

		if end < len(filtered) {
			itemContent.WriteString(Styles.DetailDim.Render(fmt.Sprintf("\n  … and %d more", len(filtered)-end)) + "\n")
		}
	}

	b.WriteString(padToHeight(itemContent.String(), prSlotLines))

	b.WriteString("\n" + footer)

	return Styles.OverlayBorderInfo.Width(overlayWidth).Render(
		Styles.OverlayTitle.Render("Pull Requests") + "\n\n" + b.String(),
	)
}

// formatDiffStats formats additions/deletions with comma separators.
func formatDiffStats(additions, deletions int) string {
	return Styles.DetailFileAdd.Render("+"+formatNumber(additions)) + " " +
		Styles.DetailFileDel.Render("-"+formatNumber(deletions))
}

// formatCommitCount returns "N commit(s)".
func formatCommitCount(count int) string {
	if count == 1 {
		return "1 commit"
	}
	return fmt.Sprintf("%d commits", count)
}

// formatNumber adds comma separators to integers (e.g. 1203 -> "1,203").
func formatNumber(n int) string {
	s := fmt.Sprintf("%d", n)
	if len(s) <= 3 {
		return s
	}
	var result []byte
	for i, c := range s {
		if i > 0 && (len(s)-i)%3 == 0 {
			result = append(result, ',')
		}
		result = append(result, byte(c))
	}
	return string(result)
}

// renderPRPanel renders the PR view as a full-screen panel layout
// with a list on one side and a detail preview on the other.
func (m Model) renderPRPanel() string {
	s := m.prState
	if s == nil {
		return ""
	}

	// Header: context-aware bar showing we're in PR view
	statusBar := renderContextHeader("Pull Requests", m.projectName, m.width)

	// Footer: context-aware based on focus state
	footer := m.renderPRFooter()

	footerLines := strings.Count(footer, "\n") + 1
	bodyBudget := m.height - 1 - footerLines // 1 for header

	useSideBySide := m.width > 100
	bodyWidth := m.width - 2 // 1-char padding each side

	filter := s.FilterInput.Value()
	filtered := filteredPRs(s.PRs, filter)

	var body string
	if useSideBySide {
		listWidth := bodyWidth * 40 / 100
		dividerWidth := 3 // " │ "
		detailWidth := bodyWidth - listWidth - dividerWidth

		listContent := renderPRList(s, listWidth, m.spinner.View(), bodyBudget)
		listContent = lipgloss.NewStyle().Width(listWidth).Render(listContent)

		dividerHeight := bodyBudget
		if dividerHeight < 1 {
			dividerHeight = 1
		}
		divider := renderVerticalDivider(dividerHeight, Colors.SurfaceDim)

		var detailView string
		if s.Loading {
			detailView = m.spinner.View() + " Loading PRs..."
		} else if s.Creating {
			detailView = renderCreatingDetail(s.ActivityLog, m.spinner.View(), renderCreatingMsg(s))
		} else if len(filtered) > 0 && s.Cursor < len(filtered) {
			detailView = m.renderPRDetailViewport(filtered[s.Cursor], detailWidth, bodyBudget)
		} else {
			detailView = Styles.DetailDim.Render("No PR selected")
		}

		body = lipgloss.JoinHorizontal(lipgloss.Top, listContent, divider, detailView)
	} else {
		// Stacked layout
		listHeight := bodyBudget * 40 / 100
		if listHeight < 6 {
			listHeight = 6
		}

		listContent := renderPRList(s, bodyWidth, m.spinner.View(), listHeight)

		// Named separator showing selected PR
		separatorName := ""
		if len(filtered) > 0 && s.Cursor < len(filtered) {
			pr := filtered[s.Cursor]
			separatorName = fmt.Sprintf("#%d %s", pr.Number, truncate(pr.Title, bodyWidth-20))
		}
		separator := renderNamedSeparator(separatorName, bodyWidth)

		detailHeight := bodyBudget - listHeight - 1 // 1 for separator
		var detailView string
		if s.Loading {
			detailView = m.spinner.View() + " Loading PRs..."
		} else if s.Creating {
			detailView = renderCreatingDetail(s.ActivityLog, m.spinner.View(), renderCreatingMsg(s))
		} else if len(filtered) > 0 && s.Cursor < len(filtered) {
			detailView = m.renderPRDetailViewport(filtered[s.Cursor], bodyWidth, detailHeight)
		} else {
			detailView = Styles.DetailDim.Render("No PR selected")
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

// renderCreatingMsg returns the "creating worktree" message for PR view.
func renderCreatingMsg(s *PRViewState) string {
	if s.CreatingPR != nil {
		return fmt.Sprintf("Creating worktree for PR #%d: %s...",
			s.CreatingPR.Number, truncate(s.CreatingPR.Title, 40))
	}
	return "Creating worktree from PR..."
}

// renderPRList renders the PR list content for the panel layout.
// It handles loading/creating states, the filter bar, and the scrollable item list.
func renderPRList(s *PRViewState, width int, spinnerView string, maxHeight int) string {
	var b strings.Builder

	if s.Loading {
		b.WriteString(spinnerView + " Loading PRs...")
		return b.String()
	}

	if s.Error != "" {
		b.WriteString(Styles.ErrorText.Render(s.Error) + "\n")
	}

	filter := s.FilterInput.Value()
	filtered := filteredPRs(s.PRs, filter)
	total := len(s.PRs)

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

	// Compute how many item slots fit in the remaining height
	// Each item takes 3 lines (title + metadata + blank), except last which takes 2
	headerLines := strings.Count(b.String(), "\n")
	availableLines := maxHeight - headerLines
	if availableLines < 3 {
		availableLines = 3
	}
	displaySlots := (availableLines + 1) / 3 // inverse of slots*3-1
	if displaySlots < 3 {
		displaySlots = 3
	}

	contentWidth := width - 2
	if contentWidth < 40 {
		contentWidth = 40
	}

	if len(filtered) == 0 {
		b.WriteString(Styles.DetailDim.Render("  (no matching PRs)") + "\n")
	} else {
		start, end := scrollWindow(len(filtered), s.Cursor, displaySlots)

		for i := start; i < end; i++ {
			pr := filtered[i]

			cursor := "  "
			if i == s.Cursor {
				cursor = Styles.ListCursor.Render("❯ ")
			}

			// Line 1: cursor + #number + title + branch
			number := Styles.DetailDim.Render(fmt.Sprintf("#%-5d", pr.Number))
			titleStr := pr.Title
			if pr.IsDraft {
				titleStr = Styles.WarningText.Render("[DRAFT]") + " " + titleStr
			}
			titleStr = truncate(titleStr, contentWidth-30)
			branch := Styles.DetailDim.Render(truncate(pr.Branch, 20))
			fmt.Fprintf(&b, "%s%s %s  %s\n", cursor, number, titleStr, branch)

			// Line 2: metadata
			indent := "         "
			author := Styles.DetailDim.Render("@" + pr.Author)
			commits := Styles.DetailDim.Render(formatCommitCount(pr.CommitCount))
			diffStats := formatDiffStats(pr.Additions, pr.Deletions)

			badge := ""
			if s.WorktreeBranches[pr.Branch] {
				badge = "  " + Styles.SuccessText.Render("✓ worktree")
			}

			fmt.Fprintf(&b, "%s%s · %s · %s%s\n", indent, author, commits, diffStats, badge)

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

// renderPRDetailPanel renders a PR detail card for the panel layout,
// wrapped in a DetailBorder with a title (matching the worktree detail panel style).
func renderPRDetailPanel(pr *tracker.PullRequest, width int) string {
	innerWidth := max(width-6, 16)

	var sections []string

	// Title
	title := pr.Title
	if pr.IsDraft {
		title = Styles.WarningText.Render("[DRAFT]") + " " + title
	}
	sections = append(sections, Styles.DetailTitle.Render(truncate(title, innerWidth)))

	// Metadata grid
	const labelWidth = 10
	label := func(s string) string {
		return Styles.DetailLabel.Render(padRight(s, labelWidth))
	}

	var metaRows []string
	metaRows = append(metaRows, renderSectionHeader("PR Info", innerWidth))
	metaRows = append(metaRows, label("Branch")+Styles.DetailValue.Render(truncate(pr.Branch, innerWidth-labelWidth-2)))
	metaRows = append(metaRows, label("Author")+Styles.DetailValue.Render("@"+pr.Author))

	if pr.CommitCount > 0 {
		metaRows = append(metaRows, label("Commits")+Styles.DetailValue.Render(formatCommitCount(pr.CommitCount)))
	}
	if pr.Additions > 0 || pr.Deletions > 0 {
		metaRows = append(metaRows, label("Changes")+formatDiffStats(pr.Additions, pr.Deletions))
	}
	if pr.ReviewDecision != "" {
		style := Styles.DetailDim
		switch pr.ReviewDecision {
		case "APPROVED":
			style = Styles.SuccessText
		case "CHANGES_REQUESTED":
			style = Styles.ErrorText
		}
		metaRows = append(metaRows, label("Review")+style.Render(pr.ReviewDecision))
	}
	sections = append(sections, strings.Join(metaRows, "\n"))

	// Commits
	if len(pr.Commits) > 0 {
		var commitRows []string
		commitRows = append(commitRows, renderSectionHeader("Commits", innerWidth))
		maxCommits := 10
		shown := pr.Commits
		if len(shown) > maxCommits {
			shown = shown[:maxCommits]
		}
		for _, c := range shown {
			msg := truncate(c.Message, innerWidth-12)
			commitRows = append(commitRows, "  "+Styles.DetailDim.Render(c.SHA)+" "+msg)
		}
		if len(pr.Commits) > maxCommits {
			commitRows = append(commitRows, "  "+Styles.DetailDim.Render(fmt.Sprintf("... and %d more", len(pr.Commits)-maxCommits)))
		}
		sections = append(sections, strings.Join(commitRows, "\n"))
	}

	// Body
	if pr.Body != "" {
		bodyHeader := renderSectionHeader("Description", innerWidth)
		rendered := renderMarkdown(pr.Body, innerWidth)
		sections = append(sections, bodyHeader+"\n"+rendered)
	}

	body := strings.Join(sections, "\n\n")

	card := Styles.DetailBorder.
		Width(width - 2).
		Render(body)

	// Insert PR number as border title
	titleLabel := " " + Styles.DetailTitle.Render(fmt.Sprintf("#%d", pr.Number)) + " "
	card = injectBorderTitle(card, titleLabel)

	return card
}

// renderPRDetailContent renders the inner content of a PR detail panel without
// the border wrapper. Used for viewport content.
func renderPRDetailContent(pr *tracker.PullRequest, width int) string {
	innerWidth := max(width-6, 16)

	var sections []string

	// Title
	title := pr.Title
	if pr.IsDraft {
		title = Styles.WarningText.Render("[DRAFT]") + " " + title
	}
	sections = append(sections, Styles.DetailTitle.Render(truncate(title, innerWidth)))

	// Metadata grid
	const labelWidth = 10
	label := func(s string) string {
		return Styles.DetailLabel.Render(padRight(s, labelWidth))
	}

	var metaRows []string
	metaRows = append(metaRows, renderSectionHeader("PR Info", innerWidth))
	metaRows = append(metaRows, label("Branch")+Styles.DetailValue.Render(truncate(pr.Branch, innerWidth-labelWidth-2)))
	metaRows = append(metaRows, label("Author")+Styles.DetailValue.Render("@"+pr.Author))

	if pr.CommitCount > 0 {
		metaRows = append(metaRows, label("Commits")+Styles.DetailValue.Render(formatCommitCount(pr.CommitCount)))
	}
	if pr.Additions > 0 || pr.Deletions > 0 {
		metaRows = append(metaRows, label("Changes")+formatDiffStats(pr.Additions, pr.Deletions))
	}
	if pr.ReviewDecision != "" {
		style := Styles.DetailDim
		switch pr.ReviewDecision {
		case "APPROVED":
			style = Styles.SuccessText
		case "CHANGES_REQUESTED":
			style = Styles.ErrorText
		}
		metaRows = append(metaRows, label("Review")+style.Render(pr.ReviewDecision))
	}
	sections = append(sections, strings.Join(metaRows, "\n"))

	// Commits
	if len(pr.Commits) > 0 {
		var commitRows []string
		commitRows = append(commitRows, renderSectionHeader("Commits", innerWidth))
		maxCommits := 10
		shown := pr.Commits
		if len(shown) > maxCommits {
			shown = shown[:maxCommits]
		}
		for _, c := range shown {
			msg := truncate(c.Message, innerWidth-12)
			commitRows = append(commitRows, "  "+Styles.DetailDim.Render(c.SHA)+" "+msg)
		}
		if len(pr.Commits) > maxCommits {
			commitRows = append(commitRows, "  "+Styles.DetailDim.Render(fmt.Sprintf("... and %d more", len(pr.Commits)-maxCommits)))
		}
		sections = append(sections, strings.Join(commitRows, "\n"))
	}

	// Body
	if pr.Body != "" {
		bodyHeader := renderSectionHeader("Description", innerWidth)
		rendered := renderMarkdown(pr.Body, innerWidth)
		sections = append(sections, bodyHeader+"\n"+rendered)
	}

	return strings.Join(sections, "\n\n")
}

// renderPRDetailViewport renders the PR detail using the viewport for scrolling.
// It updates the viewport content when the cursor changes and applies a
// highlighted border when the detail panel is focused.
func (m Model) renderPRDetailViewport(pr *tracker.PullRequest, width, height int) string {
	s := m.prState

	// Update viewport dimensions
	// Reserve 2 for border on each side
	vpWidth := width - 4
	if vpWidth < 16 {
		vpWidth = 16
	}
	vpHeight := height - 2 // border top + bottom
	if vpHeight < 1 {
		vpHeight = 1
	}
	s.DetailViewport.SetWidth(vpWidth)
	s.DetailViewport.SetHeight(vpHeight)

	// Update viewport content when cursor changes
	if s.Cursor != s.lastCursor || s.DetailViewport.GetContent() == "" {
		content := renderPRDetailContent(pr, width)
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

	// Insert PR number as border title
	titleLabel := " " + Styles.DetailTitle.Render(fmt.Sprintf("#%d", pr.Number)) + " "
	if s.DetailFocused {
		card = injectBorderTitleWithColor(card, titleLabel, Colors.Primary)
	} else {
		card = injectBorderTitle(card, titleLabel)
	}

	return card
}

// renderPRFooter returns context-aware footer hints for the PR view.
func (m Model) renderPRFooter() string {
	s := m.prState
	if s != nil && s.DetailFocused {
		return m.helpFooter.RenderCompactWithHints(prDetailFocusedHints(), m.width-4)
	}
	return m.helpFooter.RenderCompact(ViewPRs, m.width-4)
}

// prDetailFocusedHints returns hints for when the PR detail panel is focused.
func prDetailFocusedHints() []Hint {
	return []Hint{
		{"↑↓", "scroll"},
		{"g/G", "top/bottom"},
		{"tab", "list"},
		{"esc", "back"},
	}
}

// renderContextHeader renders a header bar for context views (PRs, Issues).
// Format: "  project  ←  Context Label  "
func renderContextHeader(contextLabel, projectName string, width int) string {
	left := Styles.Header.Render(projectName)
	left += Styles.TextMuted.Render("  ←  ")
	left += Styles.TextBright.Render(contextLabel)

	right := Styles.TextMuted.Render("esc to return")
	rightWidth := lipgloss.Width(right)
	leftWidth := lipgloss.Width(left)

	var content string
	gap := width - leftWidth - rightWidth - 2
	if gap >= 2 {
		content = left + strings.Repeat(" ", gap) + right
	} else {
		content = left
	}

	return Styles.HeaderBar.Width(width).Render(content)
}
