package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

func renderDashboard(items []WorktreeItem, cursor int, filterText string, filtering bool, width, height int) string {
	if width < 40 {
		width = 40
	}
	if height < 10 {
		height = 10
	}

	headerHeight := 1
	footerHeight := 1
	separatorHeight := 1
	fixedHeight := headerHeight + footerHeight + separatorHeight

	useSideBySide := width > 120
	var listWidth, detailWidth int
	var listHeight, detailHeight int

	if useSideBySide {
		listWidth = width / 2
		detailWidth = width - listWidth - 1
		listHeight = height - headerHeight - footerHeight
		detailHeight = listHeight
	} else {
		listWidth = width
		detailWidth = width
		contentHeight := height - fixedHeight
		listHeight = contentHeight * 6 / 10
		if listHeight < 3 {
			listHeight = 3
		}
		detailHeight = contentHeight - listHeight
		if detailHeight < 3 {
			detailHeight = 3
		}
	}

	// Header
	header := Theme.Header.Render(" grove")
	if filtering {
		header += "  " + Theme.DetailDim.Render("/") + filterText + Theme.DetailDim.Render("█")
	}

	// Filter
	visible := items
	if filterText != "" {
		visible = filterItems(items, filterText)
	}

	if cursor >= len(visible) {
		cursor = len(visible) - 1
	}
	if cursor < 0 {
		cursor = 0
	}

	listContent := renderList(visible, cursor, listWidth, listHeight)

	var detailContent string
	if cursor >= 0 && cursor < len(visible) {
		detailContent = renderDetailPanel(&visible[cursor], detailWidth)
	}

	footer := renderFooter(filtering, width)

	var body string
	if useSideBySide {
		sep := strings.Repeat("│\n", listHeight)
		sep = strings.TrimRight(sep, "\n")
		body = lipgloss.JoinHorizontal(lipgloss.Top, listContent, sep, detailContent)
	} else {
		separator := Theme.DetailDim.Render(strings.Repeat("─", width))
		body = listContent + "\n" + separator + "\n" + detailContent
	}

	return header + "\n" + body + "\n" + footer
}

func renderList(items []WorktreeItem, cursor, width, height int) string {
	var lines []string

	start := 0
	if cursor >= height {
		start = cursor - height + 1
	}
	end := start + height
	if end > len(items) {
		end = len(items)
	}

	for i := start; i < end; i++ {
		lines = append(lines, renderListItem(&items[i], i == cursor, width))
	}

	for len(lines) < height {
		lines = append(lines, strings.Repeat(" ", width))
	}

	return strings.Join(lines, "\n")
}

func renderListItem(item *WorktreeItem, selected bool, width int) string {
	var cursor string
	if selected {
		cursor = Theme.ListCursor.String()
	} else {
		cursor = Theme.ListCursorDim.String()
	}

	nameStyle := Theme.NormalItem
	if selected {
		nameStyle = Theme.SelectedItem
	}
	if item.IsCurrent {
		nameStyle = Theme.CurrentItem
		if selected {
			nameStyle = nameStyle.Bold(true)
		}
	}
	name := nameStyle.Render(fmt.Sprintf("%-16s", truncate(item.ShortName, 16)))

	branch := Theme.DetailDim.Render(fmt.Sprintf("%-12s", truncate(item.Branch, 12)))

	age := strings.Repeat(" ", 8)
	if item.CommitAge != "" {
		age = Theme.DetailDim.Render(fmt.Sprintf("%-8s", compactAge(item.CommitAge)))
	}

	status := item.StatusText()
	tmuxText := item.TmuxText()

	line := cursor + name + "  " + branch + "  " + age + "  " + status
	if tmuxText != "" {
		line += "  " + tmuxText
	}

	return line
}

func renderFooter(filtering bool, width int) string {
	if filtering {
		return Theme.Footer.Render(" [enter] done  [esc] cancel")
	}

	bindings := [][2]string{
		{"enter", "switch"},
		{"n", "new"},
		{"d", "delete"},
		{"/", "filter"},
		{"?", "help"},
		{"q", "quit"},
	}

	var parts []string
	for _, b := range bindings {
		parts = append(parts, fmt.Sprintf("[%s] %s", b[0], b[1]))
	}
	return Theme.Footer.Render(" " + strings.Join(parts, "  "))
}

func renderDetailPanel(item *WorktreeItem, width int) string {
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

	// Dirty files
	if len(item.DirtyFiles) > 0 {
		maxFiles := 8
		shown := item.DirtyFiles
		if len(shown) > maxFiles {
			shown = shown[:maxFiles]
		}
		for _, f := range shown {
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
		if len(item.DirtyFiles) > maxFiles {
			b.WriteString(Theme.DetailDim.Render(
				fmt.Sprintf("… and %d more", len(item.DirtyFiles)-maxFiles)) + "\n")
		}
	}

	content := strings.TrimRight(b.String(), "\n")
	return Theme.DetailBorder.Width(innerWidth).Render(content)
}

func filterItems(items []WorktreeItem, query string) []WorktreeItem {
	query = strings.ToLower(query)
	var result []WorktreeItem
	for _, item := range items {
		if strings.Contains(strings.ToLower(item.ShortName), query) ||
			strings.Contains(strings.ToLower(item.Branch), query) {
			result = append(result, item)
		}
	}
	return result
}

func truncate(s string, max int) string {
	runes := []rune(s)
	if len(runes) <= max {
		return s
	}
	if max <= 1 {
		return string(runes[:max])
	}
	return string(runes[:max-1]) + "…"
}

func compactAge(age string) string {
	age = strings.TrimSpace(age)
	replacements := [][2]string{
		{" seconds ago", "s ago"},
		{" second ago", "s ago"},
		{" minutes ago", "m ago"},
		{" minute ago", "m ago"},
		{" hours ago", "h ago"},
		{" hour ago", "h ago"},
		{" days ago", "d ago"},
		{" day ago", "d ago"},
		{" weeks ago", "w ago"},
		{" week ago", "w ago"},
		{" months ago", "mo ago"},
		{" month ago", "mo ago"},
		{" years ago", "y ago"},
		{" year ago", "y ago"},
	}
	for _, r := range replacements {
		if after, found := strings.CutSuffix(age, r[0]); found {
			return after + r[1]
		}
	}
	return age
}
