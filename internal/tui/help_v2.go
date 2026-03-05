package tui

import (
	"strings"

	"charm.land/lipgloss/v2"
)

// Hint represents a single key-description pair for the help footer.
type Hint struct {
	Key         string
	Description string
}

// HelpFooter manages a two-level help system: compact hints and expanded panel.
type HelpFooter struct {
	Expanded    bool
	CompactMode bool // mirrors Model.compactMode for dynamic hint labels
}

// NewHelpFooter creates a collapsed HelpFooter.
func NewHelpFooter() *HelpFooter {
	return &HelpFooter{}
}

// Toggle switches between compact and expanded modes.
func (h *HelpFooter) Toggle() {
	h.Expanded = !h.Expanded
}

// viewModeLabel returns "compact" or "detailed" based on current mode.
// When compact mode is active, the toggle will switch to detailed, and vice versa.
func (h *HelpFooter) viewModeLabel() string {
	if h.CompactMode {
		return "detailed"
	}
	return "compact"
}

// CompactHints returns context-aware key hints for the given view.
func (h *HelpFooter) CompactHints(view ActiveView) []Hint {
	switch view {
	case ViewDashboard:
		return []Hint{
			{"↑↓", "navigate"},
			{"enter", "switch"},
			{"n", "new"},
			{"d", "delete"},
			{"f", "fork"},
			{"s", "sync"},
			{"c", "config"},
			{"o", "sort"},
			{"v", h.viewModeLabel()},
			{"/", "filter"},
			{"p", "PRs"},
			{"i", "issues"},
			{"?", "more"},
		}
	case ViewCreate:
		return []Hint{
			{"enter", "continue"},
			{"esc", "cancel"},
		}
	case ViewDelete:
		return []Hint{
			{"y", "confirm"},
			{"n", "cancel"},
			{"space", "toggle branch"},
		}
	case ViewBulk:
		return []Hint{
			{"↑↓", "navigate"},
			{"space", "toggle"},
			{"enter", "delete selected"},
			{"esc", "cancel"},
		}
	case ViewPRs:
		return []Hint{
			{"↑↓", "navigate"},
			{"enter", "create worktree"},
			{"esc", "close"},
		}
	case ViewFork:
		return []Hint{
			{"enter", "continue"},
			{"esc", "cancel"},
		}
	case ViewSync:
		return []Hint{
			{"↑↓", "navigate"},
			{"enter", "select"},
			{"esc", "cancel"},
		}
	case ViewConfig:
		return []Hint{
			{"tab", "next tab"},
			{"↑↓", "navigate"},
			{"enter", "edit"},
			{"esc", "close"},
		}
	default:
		return []Hint{
			{"?", "help"},
			{"q", "quit"},
		}
	}
}

// RenderCompact renders a one- or two-line footer with key hints.
// At narrow widths, hints wrap to a second line instead of being dropped.
func (h *HelpFooter) RenderCompact(view ActiveView, width int) string {
	hints := h.CompactHints(view)

	var parts []string
	for _, hint := range hints {
		part := Styles.HelpKey.Render(hint.Key) + " " + Styles.HelpDesc.Render(hint.Description)
		parts = append(parts, part)
	}

	sep := Styles.HelpSep.Render(" · ")
	line := strings.Join(parts, sep)

	// Fits on one line — done
	if lipgloss.Width(line) <= width || width <= 0 {
		return "  " + line
	}

	// Try two lines: split roughly in half
	for split := len(parts) / 2; split >= 1; split-- {
		line1 := strings.Join(parts[:split], sep)
		line2 := strings.Join(parts[split:], sep)
		if lipgloss.Width(line1) <= width && lipgloss.Width(line2) <= width {
			return "  " + line1 + "\n  " + line2
		}
	}

	// Even two lines overflow — fall back to dropping from right
	for i := len(parts) - 1; i >= 1; i-- {
		line = strings.Join(parts[:i], sep)
		if lipgloss.Width(line) <= width {
			break
		}
	}
	return "  " + line
}

// CompactHeight returns the number of lines the compact footer will occupy.
func (h *HelpFooter) CompactHeight(view ActiveView, width int) int {
	rendered := h.RenderCompact(view, width)
	return strings.Count(rendered, "\n") + 1
}

// RenderExpanded renders a three-column help panel.
func (h *HelpFooter) RenderExpanded(width int) string {
	cols := []struct {
		header string
		items  []Hint
	}{
		{
			header: "Navigation",
			items: []Hint{
				{"↑/k", "move up"},
				{"↓/j", "move down"},
				{"enter", "switch"},
				{"esc", "back/close"},
			},
		},
		{
			header: "Actions",
			items: []Hint{
				{"n", "new worktree"},
				{"d", "delete"},
				{"f", "fork worktree"},
				{"s", "sync changes"},
				{"c", "configure"},
				{"p", "browse PRs"},
				{"i", "browse issues"},
				{"a", "bulk delete"},
				{"o", "cycle sort"},
				{"r", "refresh"},
			},
		},
		{
			header: "Views",
			items: []Hint{
				{"1-9", "quick-switch"},
				{"/", "filter"},
				{"v", h.viewModeLabel()},
				{"?", "toggle help"},
				{"q", "quit"},
			},
		},
	}

	colWidth := (width - 8) / 3
	colWidth = max(colWidth, 15)

	var sections []string
	for _, col := range cols {
		var lines []string
		lines = append(lines, Styles.DetailTitle.Render(col.header))
		lines = append(lines, "")
		for _, item := range col.items {
			k := Styles.HelpKey.Render(padRight(item.Key, 10))
			d := Styles.HelpDesc.Render(item.Description)
			lines = append(lines, "  "+k+d)
		}
		section := strings.Join(lines, "\n")
		sections = append(sections, lipgloss.NewStyle().Width(colWidth).Render(section))
	}

	body := lipgloss.JoinHorizontal(lipgloss.Top, sections...)
	footer := Styles.TextMuted.Render("Press ? again to close")

	content := Styles.OverlayTitle.Render("Quick Reference") + "\n\n" + body + "\n\n" + footer

	return Styles.RoundedBorder.
		Width(width-4).
		Padding(1, 2).
		BorderForeground(Colors.SurfaceBorder).
		Render(content)
}
