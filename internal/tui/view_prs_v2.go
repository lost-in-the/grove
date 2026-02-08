package tui

import (
	"fmt"
	"strings"
)

// renderPRViewV2 renders the PR browser with two-line items, draft labels,
// diff stats, and worktree badges.
func renderPRViewV2(s *PRViewState, width int, spinnerView string) string {
	if s.Loading {
		return Styles.OverlayBorderInfo.Render(
			Styles.OverlayTitle.Render("Pull Requests") + "\n\n" +
				spinnerView + " Loading PRs...",
		)
	}

	if s.Creating {
		creatingMsg := "Creating worktree from PR..."
		if s.CreatingPR != nil {
			creatingMsg = fmt.Sprintf("Creating worktree for PR #%d: %s...",
				s.CreatingPR.Number, truncate(s.CreatingPR.Title, 40))
		}
		return Styles.OverlayBorderInfo.Render(
			Styles.OverlayTitle.Render("Pull Requests") + "\n\n" +
				spinnerView + " " + creatingMsg,
		)
	}

	var b strings.Builder

	if s.Error != "" {
		b.WriteString(Styles.ErrorText.Render(s.Error) + "\n\n")
	}

	filtered := filteredPRs(s.PRs, s.Filter)

	// If preview mode and we have a selected PR, render the preview instead
	if s.ShowPreview && len(filtered) > 0 && s.Cursor < len(filtered) {
		return renderPRPreview(filtered[s.Cursor], width)
	}
	total := len(s.PRs)

	// Filter bar with count
	if s.Filter != "" {
		fmt.Fprintf(&b, "Filter: %s█", s.Filter)
		fmt.Fprintf(&b, "  %s", Styles.DetailDim.Render(fmt.Sprintf("%d of %d", len(filtered), total)))
		b.WriteString("\n\n")
	} else if total > 0 {
		b.WriteString(Styles.DetailDim.Render(fmt.Sprintf("%d open", total)) + "\n\n")
	}

	if len(filtered) == 0 {
		b.WriteString(Styles.DetailDim.Render("  (no matching PRs)") + "\n")
	} else {
		maxShow := 10
		start := 0
		if s.Cursor >= maxShow {
			start = s.Cursor - maxShow + 1
		}
		end := start + maxShow
		if end > len(filtered) {
			end = len(filtered)
		}

		contentWidth := width - 8 // padding from overlay border
		if contentWidth < 40 {
			contentWidth = 40
		}

		for i := start; i < end; i++ {
			pr := filtered[i]

			cursor := "  "
			if i == s.Cursor {
				cursor = Styles.ListCursor.String()
			}

			// Line 1: cursor + #number + title + branch
			number := Styles.DetailDim.Render(fmt.Sprintf("#%-5d", pr.Number))
			titleStr := pr.Title
			if pr.IsDraft {
				titleStr = Styles.WarningText.Render("[DRAFT]") + " " + titleStr
			}
			titleStr = truncate(titleStr, contentWidth-30)
			branch := Styles.DetailDim.Render(truncate(pr.Branch, 20))
			b.WriteString(fmt.Sprintf("%s%s %s  %s\n", cursor, number, titleStr, branch))

			// Line 2: metadata indent + author + commits + diff stats + worktree badge
			indent := "         " // align with title after cursor+number
			author := Styles.DetailDim.Render("@" + pr.Author)
			commits := Styles.DetailDim.Render(formatCommitCount(pr.CommitCount))
			diffStats := formatDiffStats(pr.Additions, pr.Deletions)

			badge := ""
			if s.WorktreeBranches[pr.Branch] {
				badge = "  " + Styles.SuccessText.Render("✓ worktree")
			}

			b.WriteString(fmt.Sprintf("%s%s · %s · %s%s\n", indent, author, commits, diffStats, badge))

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
