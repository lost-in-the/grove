package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// renderDetailContent builds the detail panel string for a given worktree item.
func renderDetailContent(item *WorktreeItem, width int) string {
	if item == nil || width < 20 {
		return ""
	}

	innerWidth := width - 4
	if innerWidth < 16 {
		innerWidth = 16
	}

	var b strings.Builder

	// Title
	title := Theme.DetailTitle.Render(item.ShortName)
	if item.Commit != "" {
		title += Theme.DetailDim.Render(" · " + item.Commit)
	}
	if item.CommitMessage != "" {
		msg := item.CommitMessage
		maxMsg := innerWidth - lipgloss.Width(title) - 5
		if maxMsg > 0 && len(msg) > maxMsg {
			msg = msg[:maxMsg-1] + "…"
		}
		title += Theme.DetailDim.Render(fmt.Sprintf(" · %q", msg))
	}
	b.WriteString(title + "\n")

	// Branch
	branchLine := Theme.DetailLabel.Render("branch: ") + Theme.DetailValue.Render(item.Branch)
	if item.AheadCount > 0 || item.BehindCount > 0 {
		branchLine += "  " + Theme.DetailValue.Render(
			fmt.Sprintf("↑%d ↓%d", item.AheadCount, item.BehindCount))
	}
	if item.CommitAge != "" {
		branchLine += "  " + Theme.DetailDim.Render(item.CommitAge)
	}
	if item.IsEnvironment {
		branchLine += "  " + Theme.EnvBadge.Render("[env]")
	}
	b.WriteString(branchLine + "\n")

	// Dirty files — no limit, viewport scrolls
	if len(item.DirtyFiles) > 0 {
		for _, f := range item.DirtyFiles {
			f = strings.TrimSpace(f)
			if len(f) < 3 {
				continue
			}
			prefix := f[:2]
			file := strings.TrimSpace(f[2:])

			var styled string
			switch {
			case strings.Contains(prefix, "?"):
				styled = Theme.DetailFileAdd.Render("+ " + file)
			case strings.Contains(prefix, "D"):
				styled = Theme.DetailFileDel.Render("- " + file)
			default:
				styled = Theme.DetailFileMod.Render("M " + file)
			}
			b.WriteString(styled + "\n")
		}
	}

	return strings.TrimRight(b.String(), "\n")
}
