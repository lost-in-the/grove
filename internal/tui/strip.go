package tui

import (
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
)

// stripEntry is one "key label" pair in a strip.
type stripEntry struct {
	Key, Label string
}

// stripLines draws a strip (spec 6.3, 8.2): a title line, then the entries
// wrapped to width. Long labels and the title shorten to fit, so a strip
// never widens the screen.
func stripLines(title string, entries []stripEntry, width int) []string {
	width = max(20, width)
	lines := []string{ansi.Truncate(" "+title, width, "…")}
	line := " "
	for _, e := range entries {
		item := Styles.HelpKey.Render(e.Key) + " " + Styles.HelpDesc.Render(middleTruncate(e.Label, 20))
		next := line + " " + item
		if line != " " {
			next = line + "   " + item
		}
		if lipgloss.Width(next) > width && line != " " {
			lines = append(lines, line)
			next = "  " + item
		}
		line = next
	}
	return append(lines, line)
}

// stripTitle styles a strip title: its prefix key, then the description.
func stripTitle(prefix, desc string) string {
	return Styles.HelpKey.Render(prefix) + Styles.HelpDesc.Render(" "+desc)
}

// gStripLines is the goto strip: the table's g-layer actions.
func (m Model) gStripLines() []string {
	var entries []stripEntry
	for _, a := range dashboardActions() {
		if a.Layer == "g" {
			entries = append(entries, stripEntry{a.display(), a.Label})
		}
	}
	entries = append(entries, stripEntry{"esc", "close"})
	return stripLines(stripTitle("g", "goto"), entries, m.width-2)
}

// openStripLines is whichever strip is open, or nil.
func (m Model) openStripLines() []string {
	switch {
	case m.pendingG:
		return m.gStripLines()
	case m.pendingM:
		return m.pinStripLines()
	}
	return nil
}

// stripHeight is the open strip's height, or 0.
func (m Model) stripHeight() int {
	return len(m.openStripLines())
}

func (m Model) renderOpenStrip() string {
	return strings.Join(m.openStripLines(), "\n")
}
