package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// Header renders the dashboard header with project context.
type Header struct {
	ProjectName   string
	WorktreeCount int
	CurrentBranch string
	CurrentName   string
}

// View renders the header at the given width.
func (h Header) View(width int) string {
	// Left side: project · count · branch
	countLabel := "worktrees"
	if h.WorktreeCount == 1 {
		countLabel = "worktree"
	}

	left := Styles.Header.Render(h.ProjectName)
	left += Styles.TextMuted.Render("  ·  ")
	left += Styles.TextMuted.Render(fmt.Sprintf("%d %s", h.WorktreeCount, countLabel))

	if h.CurrentBranch != "" {
		left += Styles.TextMuted.Render("  ·  ")
		left += Styles.TextMuted.Render("on ") + Styles.TextNormal.Render(h.CurrentBranch)
	}

	leftWidth := lipgloss.Width(left)

	// Right side: current worktree indicator
	var right string
	if h.CurrentName != "" {
		right = Styles.StatusSuccess.Render("●") + " " + Styles.TextNormal.Render(h.CurrentName)
	}

	rightWidth := lipgloss.Width(right)

	// If both sides fit, space them apart
	gap := width - leftWidth - rightWidth
	if gap >= 2 && right != "" {
		return left + strings.Repeat(" ", gap) + right
	}

	// If right doesn't fit, truncate or omit it
	if right != "" && width-leftWidth >= 10 {
		available := width - leftWidth - 4 // 2 spaces + dot + space
		if available > 0 && available < len(h.CurrentName) {
			truncated := h.CurrentName[:available] + "…"
			right = Styles.StatusSuccess.Render("●") + " " + Styles.TextNormal.Render(truncated)
			gap = width - leftWidth - lipgloss.Width(right)
			if gap >= 2 {
				return left + strings.Repeat(" ", gap) + right
			}
		}
	}

	// Narrow: just left side, truncated to width
	if leftWidth > width {
		return left[:width]
	}

	return left
}
