package tui

import (
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"
)

// Header renders the dashboard header with project context.
type Header struct {
	ProjectName     string
	WorktreeCount   int
	CurrentBranch   string
	CurrentName     string
	ContainerTarget string // worktree name that containers are pointed to
	SortLabel       string // current sort mode label (e.g. "recent", "dirty")
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

	if h.ContainerTarget != "" {
		left += Styles.TextMuted.Render("  ·  ")
		left += Styles.ContainerBadgeActive.Render("◆") + Styles.TextMuted.Render(" → ") + Styles.TextNormal.Render(h.ContainerTarget)
	}

	if h.SortLabel != "" {
		left += Styles.TextMuted.Render("  ·  ")
		left += Styles.TextMuted.Render("↕ ") + Styles.TextNormal.Render(h.SortLabel)
	}

	leftWidth := lipgloss.Width(left)

	// Right side: current worktree indicator
	var right string
	if h.CurrentName != "" {
		right = Styles.StatusSuccess.Render("●") + " " + Styles.TextNormal.Render(h.CurrentName)
	}

	rightWidth := lipgloss.Width(right)

	var content string

	// If both sides fit, space them apart
	gap := width - leftWidth - rightWidth - 2 // -2 for HeaderBar padding
	if gap >= 2 && right != "" {
		content = left + strings.Repeat(" ", gap) + right
	} else if right != "" && width-leftWidth >= 10 {
		// If right doesn't fit, truncate or omit it
		available := width - leftWidth - 4 // 2 spaces + dot + space
		if available > 0 && available < len(h.CurrentName) {
			truncated := h.CurrentName[:available] + "…"
			right = Styles.StatusSuccess.Render("●") + " " + Styles.TextNormal.Render(truncated)
			gap = width - leftWidth - lipgloss.Width(right) - 2
			if gap >= 2 {
				content = left + strings.Repeat(" ", gap) + right
			} else {
				content = left
			}
		} else {
			content = left
		}
	} else {
		content = left
	}

	// Wrap in full-width header bar with background
	return Styles.HeaderBar.Width(width).Render(content)
}
