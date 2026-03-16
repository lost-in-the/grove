package tui

import (
	"fmt"
	"strings"

	lipgloss "charm.land/lipgloss/v2"

	"github.com/lost-in-the/grove/plugins/tracker"
)

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

// renderPRItemList renders a scrollable list of PR items with cursor, scroll window, and overflow indicator.
func renderPRItemList(prs []*tracker.PullRequest, cursor int, scrollSize int, contentWidth int, worktreeBranches map[string]bool) string {
	var b strings.Builder
	if len(prs) == 0 {
		b.WriteString(Styles.DetailDim.Render("  (no matching PRs)") + "\n")
		return b.String()
	}

	start, end := scrollWindow(len(prs), cursor, scrollSize)

	for i := start; i < end; i++ {
		pr := prs[i]

		cur := "  "
		if i == cursor {
			cur = Styles.ListCursor.Render("❯ ")
		}

		number := Styles.DetailDim.Render(fmt.Sprintf("#%-5d", pr.Number))
		titleStr := pr.Title
		if pr.IsDraft {
			titleStr = Styles.WarningText.Render("[DRAFT]") + " " + titleStr
		}
		titleStr = truncate(titleStr, contentWidth-30)
		branch := Styles.DetailDim.Render(truncate(pr.Branch, 20))
		fmt.Fprintf(&b, "%s%s %s  %s\n", cur, number, titleStr, branch)

		indent := "         "
		author := Styles.DetailDim.Render("@" + pr.Author)
		commits := Styles.DetailDim.Render(formatCommitCount(pr.CommitCount))
		diffStats := formatDiffStats(pr.Additions, pr.Deletions)

		badge := ""
		if worktreeBranches[pr.Branch] {
			badge = "  " + Styles.SuccessText.Render("✓ worktree")
		}

		fmt.Fprintf(&b, "%s%s · %s · %s%s\n", indent, author, commits, diffStats, badge)

		if i < end-1 {
			b.WriteString("\n")
		}
	}

	if end < len(prs) {
		b.WriteString(Styles.DetailDim.Render(fmt.Sprintf("\n  … and %d more", len(prs)-end)) + "\n")
	}

	return b.String()
}

// renderPRPanel renders the PR view as a full-screen panel layout
// with a list on one side and a detail preview on the other.
func (m Model) renderPRPanel() string {
	s := m.prState
	if s == nil {
		return ""
	}

	filter := s.FilterInput.Value()
	filtered := filteredPRs(s.PRs, filter)

	separatorName := ""
	if len(filtered) > 0 && s.Cursor < len(filtered) {
		pr := filtered[s.Cursor]
		separatorName = fmt.Sprintf("#%d %s", pr.Number, truncate(pr.Title, m.width-22))
	}

	return m.renderContextPanel(contextPanelConfig{
		contextLabel: "Pull Requests",
		footer:       m.renderPRFooter(),
		renderList: func(width int, spinnerView string, maxHeight int) string {
			return renderPRList(s, width, spinnerView, maxHeight)
		},
		renderDetail: func(width, height int) string {
			if s.Loading {
				return m.spinner.View() + " Loading PRs..."
			}
			if s.Creating {
				return renderCreatingDetail(s.ActivityLog, m.spinner.View(), renderCreatingMsg(s))
			}
			if len(filtered) > 0 && s.Cursor < len(filtered) {
				return m.renderPRDetailViewport(filtered[s.Cursor], width, height)
			}
			return Styles.DetailDim.Render("No PR selected")
		},
		separatorName: separatorName,
	})
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
	renderFilterBar(&b, s.FilterInput.View(), s.Filtering, filter, len(filtered), total)

	displaySlots, contentWidth := calcDisplaySlots(strings.Count(b.String(), "\n"), maxHeight, width)
	b.WriteString(renderPRItemList(filtered, s.Cursor, displaySlots, contentWidth, s.WorktreeBranches))

	return b.String()
}

// renderPRDetailContent renders the inner content of a PR detail panel.
// Used for viewport content.
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
		case reviewApproved:
			style = Styles.SuccessText
		case reviewChangesRequested:
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
func (m Model) renderPRDetailViewport(pr *tracker.PullRequest, width, height int) string {
	s := m.prState
	return renderDetailViewportCard(detailViewportConfig{
		vp:         &s.DetailViewport,
		cursor:     s.Cursor,
		lastCursor: &s.lastCursor,
		focused:    s.DetailFocused,
		itemNumber: pr.Number,
		contentFunc: func(w int) string {
			return renderPRDetailContent(pr, w)
		},
		width:  width,
		height: height,
	})
}

// renderPRFooter returns context-aware footer hints for the PR view.
func (m Model) renderPRFooter() string {
	s := m.prState
	if s != nil && s.DetailFocused {
		return m.helpFooter.RenderCompactWithHints(detailFocusedHints(), m.width-4)
	}
	return m.helpFooter.RenderCompact(ViewPRs, m.width-4)
}

// detailFocusedHints returns hints for when a detail panel is focused (shared by PRs and Issues).
func detailFocusedHints() []Hint {
	return []Hint{
		{"↑↓", "scroll"},
		{"g/G", "top/bottom"},
		{"tab", "list"},
		{"esc", "back"},
	}
}

// contextPanelConfig holds the varying parts of a context panel layout.
type contextPanelConfig struct {
	contextLabel  string
	footer        string
	renderList    func(width int, spinnerView string, maxHeight int) string
	renderDetail  func(width, height int) string
	separatorName string
}

// renderContextPanel renders a full-screen panel layout shared by Issues and PRs.
// Both views use identical layout logic (header, side-by-side vs stacked, toast,
// padding, clamping) — only the list/detail content differs.
func (m Model) renderContextPanel(cfg contextPanelConfig) string {
	statusBar := renderContextHeader(cfg.contextLabel, m.projectName, m.width)

	footerLines := strings.Count(cfg.footer, "\n") + 1
	bodyBudget := m.height - 1 - footerLines

	useSideBySide := m.width > 100
	bodyWidth := m.width - 2

	var body string
	if useSideBySide {
		listWidth := bodyWidth * 40 / 100
		dividerWidth := 3
		detailWidth := bodyWidth - listWidth - dividerWidth

		listContent := cfg.renderList(listWidth, m.spinner.View(), bodyBudget)
		listContent = lipgloss.NewStyle().Width(listWidth).Render(listContent)

		dividerHeight := bodyBudget
		if dividerHeight < 1 {
			dividerHeight = 1
		}
		divider := renderVerticalDivider(dividerHeight, Colors.SurfaceDim)

		detailView := cfg.renderDetail(detailWidth, bodyBudget)

		body = lipgloss.JoinHorizontal(lipgloss.Top, listContent, divider, detailView)
	} else {
		listHeight := bodyBudget * 40 / 100
		if listHeight < 6 {
			listHeight = 6
		}

		listContent := cfg.renderList(bodyWidth, m.spinner.View(), listHeight)
		separator := renderNamedSeparator(cfg.separatorName, bodyWidth)

		detailHeight := bodyBudget - listHeight - 1
		detailView := cfg.renderDetail(bodyWidth, detailHeight)

		body = lipgloss.JoinVertical(lipgloss.Left, listContent, separator, detailView)
	}

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

	return lipgloss.JoinVertical(lipgloss.Left, statusBar, body, cfg.footer)
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
