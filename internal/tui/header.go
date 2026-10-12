package tui

import (
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
)

// Header renders the dashboard header (spec 5.5): project, worktree count,
// agents that need you, and sort on the left; the cursor position in the
// visible rows on the right.
type Header struct {
	ProjectName   string
	WorktreeCount int
	Blocked       int    // agents blocked on you; counted only under herdr
	Done          int    // agents done and waiting for review
	SortLabel     string // "name", "age", "dirty"
	SortReversed  bool
	Cursor        int // 1-based position of the selected row among visible rows
	Total         int // visible rows (after any filter)
}

// View renders the header at the given width. As space runs out the count
// and the agent summary drop their words, then the project name truncates;
// the cursor position stays.
func (h Header) View(width int) string {
	inner := max(0, width-2) // HeaderBar pads one cell each side
	sep := Styles.TextMuted.Render(" · ")
	right := ""
	if h.Total > 0 {
		right = Styles.TextMuted.Render(fmt.Sprintf("%d/%d", h.Cursor, h.Total))
	}
	sort := ""
	if h.SortLabel != "" {
		sort = Styles.TextMuted.Render("⬍ ") + Styles.TextNormal.Render(h.SortLabel)
		if h.SortReversed {
			sort += Styles.TextMuted.Render(" ↑")
		}
	}
	count := fmt.Sprintf("%d worktrees", h.WorktreeCount)
	if h.WorktreeCount == 1 {
		count = "1 worktree"
	}

	build := func(name, count string, words bool) string {
		parts := []string{Styles.Header.Render(name), Styles.TextMuted.Render(count)}
		parts = append(parts, h.triage(words)...)
		if sort != "" {
			parts = append(parts, sort)
		}
		return strings.Join(parts, sep)
	}
	fits := func(left string) bool { return lipgloss.Width(left)+1+lipgloss.Width(right) <= inner }

	words := true
	left := build(h.ProjectName, count, words)
	if !fits(left) {
		count, words = fmt.Sprintf("%d", h.WorktreeCount), false
		left = build(h.ProjectName, count, words)
	}
	if !fits(left) {
		over := lipgloss.Width(left) + 1 + lipgloss.Width(right) - inner
		name := ansi.Truncate(h.ProjectName, max(1, lipgloss.Width(h.ProjectName)-over), "…")
		left = build(name, count, words)
	}

	gap := inner - lipgloss.Width(left) - lipgloss.Width(right)
	content := left + strings.Repeat(" ", max(1, gap)) + right
	content = ansi.Truncate(content, inner, "")
	return Styles.HeaderBar.Width(width).MaxHeight(1).Render(content)
}

// triage is the agent summary, "! 2 blocked" and "✓ 1 done", without the
// words when space is short. Zero counts show nothing.
func (h Header) triage(words bool) []string {
	var out []string
	add := func(n int, glyph string, style lipgloss.Style, word string) {
		if n == 0 {
			return
		}
		s := style.Render(glyph) + Styles.TextNormal.Render(fmt.Sprintf(" %d", n))
		if words {
			s += Styles.TextMuted.Render(" " + word)
		}
		out = append(out, s)
	}
	add(h.Blocked, "!", Styles.StatusDanger, "blocked")
	add(h.Done, "✓", Styles.StatusSuccess, "done")
	return out
}
