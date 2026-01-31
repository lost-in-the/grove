package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// renderDetailV2 builds an enhanced detail panel with bordered card,
// metadata grid, sync status, changed files list, and tmux indicator.
func renderDetailV2(item *WorktreeItem, width int) string {
	if item == nil || width < 20 {
		return ""
	}

	// Reserve space for border padding
	innerWidth := max(width-6, 16)

	var sections []string

	// Metadata grid: label-value pairs
	sections = append(sections, renderMetadataGrid(item, innerWidth))

	// Changed files section (only if dirty)
	if len(item.DirtyFiles) > 0 {
		sections = append(sections, renderChangesSection(item.DirtyFiles, innerWidth))
	}

	body := strings.Join(sections, "\n\n")

	// Wrap in bordered card with title
	card := Styles.DetailBorder.
		Width(width - 2).
		Render(body)

	// Insert title into top border
	title := " " + Styles.DetailTitle.Render(item.ShortName) + " "
	card = injectBorderTitle(card, title)

	return card
}

// renderMetadataGrid renders label: value rows for the detail panel.
func renderMetadataGrid(item *WorktreeItem, width int) string {
	const labelWidth = 10

	label := func(s string) string {
		return Styles.DetailLabel.Render(padRight(s, labelWidth))
	}

	var rows []string

	// Branch
	branchVal := Styles.DetailValue.Render(truncate(item.Branch, width-labelWidth-2))
	rows = append(rows, label("Branch")+branchVal)

	// Commit
	if item.Commit != "" {
		commitVal := Styles.DetailValue.Render(item.Commit)
		if item.CommitAge != "" {
			commitVal += Styles.DetailDim.Render(" · " + item.CommitAge)
		}
		rows = append(rows, label("Commit")+commitVal)
	}

	// Status
	statusVal := renderStatusValue(item)
	rows = append(rows, label("Status")+statusVal)

	// Sync (ahead/behind/synced) — only show when remote is tracked
	if item.HasRemote {
		syncVal := renderSyncValue(item)
		rows = append(rows, label("Sync")+syncVal)
	}

	// Tmux (only if session exists)
	if item.TmuxStatus != "none" && item.TmuxStatus != "" {
		tmuxVal := renderTmuxValue(item)
		rows = append(rows, label("Tmux")+tmuxVal)
	}

	return strings.Join(rows, "\n")
}

// renderStatusValue returns styled status text for the detail panel.
func renderStatusValue(item *WorktreeItem) string {
	switch {
	case item.IsPrunable:
		return Styles.StatusDanger.Render("✗ stale")
	case item.IsDirty:
		count := len(item.DirtyFiles)
		return Styles.StatusWarning.Render(fmt.Sprintf("● dirty (%d files)", count))
	default:
		return Styles.StatusSuccess.Render("✓ clean")
	}
}

// renderSyncValue returns styled sync status (ahead/behind/synced/no remote).
func renderSyncValue(item *WorktreeItem) string {
	if !item.HasRemote {
		return Styles.StatusWarning.Render("⚠ no remote")
	}
	if item.AheadCount == 0 && item.BehindCount == 0 {
		return Styles.StatusSuccess.Render("✓ synced")
	}
	var parts []string
	if item.AheadCount > 0 {
		parts = append(parts, Styles.StatusSuccess.Render(fmt.Sprintf("↑%d", item.AheadCount)))
	}
	if item.BehindCount > 0 {
		parts = append(parts, Styles.StatusWarning.Render(fmt.Sprintf("↓%d", item.BehindCount)))
	}
	return strings.Join(parts, " ")
}

// renderTmuxValue returns styled tmux session indicator.
func renderTmuxValue(item *WorktreeItem) string {
	switch item.TmuxStatus {
	case "attached":
		return Styles.StatusSuccess.Render("● active session")
	case "detached":
		return Styles.StatusInfo.Render("○ detached session")
	default:
		return ""
	}
}

// renderChangesSection renders the changed files list with type indicators.
func renderChangesSection(files []string, width int) string {
	header := Styles.DetailLabel.Render("── Changes ") +
		Styles.DetailDim.Render(strings.Repeat("─", max(0, width-12)))

	var lines []string
	lines = append(lines, header)

	for _, f := range files {
		f = strings.TrimSpace(f)
		if len(f) < 3 {
			continue
		}
		prefix := f[:2]
		file := strings.TrimSpace(f[2:])
		file = truncate(file, width-4)

		var styled string
		switch {
		case strings.Contains(prefix, "?"):
			styled = Styles.DetailFileAdd.Render("+ " + file)
		case strings.Contains(prefix, "D"):
			styled = Styles.DetailFileDel.Render("- " + file)
		default:
			styled = Styles.DetailFileMod.Render("M " + file)
		}
		lines = append(lines, " "+styled)
	}

	return strings.Join(lines, "\n")
}

// injectBorderTitle replaces the top border of a rounded-border box
// with the title inset after the corner character.
func injectBorderTitle(rendered, title string) string {
	lines := strings.Split(rendered, "\n")
	if len(lines) == 0 {
		return rendered
	}

	topLine := lines[0]
	topRunes := []rune(topLine)

	// Find where to inject: after "╭─ "
	titleWidth := lipgloss.Width(title)

	// Build new top: corner + "─ " + title + remaining border chars
	if len(topRunes) > titleWidth+3 {
		newTop := string(topRunes[:2]) + title + string(topRunes[2+titleWidth:])
		lines[0] = newTop
	}

	return strings.Join(lines, "\n")
}

