package tui

import (
	"fmt"
	"io"
	"path/filepath"
	"strings"
	"time"

	"charm.land/bubbles/v2/list"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/lost-in-the/grove/internal/mux"
	"github.com/lost-in-the/grove/internal/plugins"
	"github.com/lost-in-the/grove/internal/worktree"
)

// rowDelegate draws one dashboard row per worktree (spec 5.1): optional
// lead columns, cursor, status, alias, branch chip, counts, age, and the
// session and container tail. TwoLine adds the commit message below.
type rowDelegate struct {
	TwoLine    bool
	Pins       bool // draw the pin column (phase 4b)
	AgentAware bool // draw the agent column
	Secondary  string
	Now        func() time.Time
}

// NewDashboardRowDelegate returns the dashboard's row delegate, for tools
// that render the list outside the TUI (cmd/snapshot).
func NewDashboardRowDelegate(twoLine bool) list.ItemDelegate {
	return rowDelegate{TwoLine: twoLine, Secondary: "branch"}
}

func (d rowDelegate) Height() int {
	if d.TwoLine {
		return 2
	}
	return 1
}
func (d rowDelegate) Spacing() int                            { return 0 }
func (d rowDelegate) Update(_ tea.Msg, _ *list.Model) tea.Cmd { return nil }

func (d rowDelegate) lead() int {
	n := 0
	if d.Pins {
		n++
	}
	if d.AgentAware {
		n++
	}
	return n
}

func (d rowDelegate) now() time.Time {
	if d.Now != nil {
		return d.Now()
	}
	return time.Now()
}

// chip is the secondary text after the alias, or "".
func (d rowDelegate) chip(item WorktreeItem) string {
	switch d.Secondary {
	case "off":
		return ""
	case "dir":
		if item.Path == "" {
			return ""
		}
		return filepath.Base(item.Path)
	}
	if item.Branch == "" || worktree.DeriveWorktreeName(item.Branch, "last_segment") == item.ShortName {
		return ""
	}
	return "⎇ " + item.Branch
}

// layout decides the columns for the whole list, so rows line up.
func (d rowDelegate) layout(m list.Model) rowLayout {
	maxAlias, anyChip := 0, false
	for _, li := range m.Items() {
		item, ok := li.(WorktreeItem)
		if !ok {
			continue
		}
		maxAlias = max(maxAlias, lipgloss.Width(item.ShortName))
		anyChip = anyChip || d.chip(item) != ""
	}
	return layoutColumns(m.Width(), d.lead(), maxAlias, anyChip)
}

func (d rowDelegate) Render(w io.Writer, m list.Model, index int, listItem list.Item) {
	item, ok := listItem.(WorktreeItem)
	if !ok {
		return
	}
	width := m.Width()
	l := d.layout(m)
	selected := index == m.Index()
	st := func(s lipgloss.Style) lipgloss.Style {
		if selected {
			return s.Background(Colors.SelectionBg)
		}
		return s
	}
	plain := st(lipgloss.NewStyle())
	pad := func(s string, n int) string {
		if gap := n - lipgloss.Width(s); gap > 0 {
			return s + plain.Render(strings.Repeat(" ", gap))
		}
		return s
	}
	padLeft := func(s string, n int) string {
		if gap := n - lipgloss.Width(s); gap > 0 {
			return plain.Render(strings.Repeat(" ", gap)) + s
		}
		return s
	}
	sp := plain.Render(" ")

	var left strings.Builder
	if d.Pins {
		left.WriteString(plain.Render("  "))
	}
	if d.AgentAware {
		left.WriteString(pad(agentGlyph(item.AgentStatus, st), rowLeadW))
	}
	if selected {
		left.WriteString(st(Styles.ListCursor).Render("❯") + sp)
	} else {
		left.WriteString(plain.Render("  "))
	}
	left.WriteString(statusGlyph(item, st) + sp)
	left.WriteString(pad(worktreeNameStyle(item, selected, st).Render(middleTruncate(item.ShortName, l.AliasW)), l.AliasW))
	if l.ChipW > 0 {
		chip := d.chip(item)
		left.WriteString(sp + pad(st(Styles.TextMuted).Render(ansi.Truncate(chip, l.ChipW, "…")), l.ChipW))
	}

	var right strings.Builder
	counts := rowCounts(item.AheadCount, item.BehindCount, len(item.DirtyFiles), l.Collapsed)
	if lipgloss.Width(counts) > l.countsW() {
		counts = rowCounts(item.AheadCount, item.BehindCount, len(item.DirtyFiles), true)
	}
	right.WriteString(sp + padLeft(styleCounts(counts, st, sp), l.countsW()))
	if l.ShowAge {
		right.WriteString(sp + padLeft(st(Styles.TextMuted).Render(shortAge(item.CommitTime, d.now())), rowAgeW))
	}
	right.WriteString(sp + tailGlyphs(item, st, sp))

	filler := width - lipgloss.Width(left.String()) - lipgloss.Width(right.String())
	line := left.String() + plain.Render(strings.Repeat(" ", max(0, filler))) + right.String()
	line = fitRow(line, width, plain)

	if d.TwoLine {
		indent := (l.Lead + 2) * rowLeadW
		msg := ansi.Truncate(item.CommitMessage, max(0, width-indent), "…")
		second := plain.Render(strings.Repeat(" ", indent)) + st(Styles.TextMuted).Render(msg)
		line += "\n" + fitRow(second, width, plain)
	}
	_, _ = fmt.Fprint(w, line)
}

// fitRow makes a rendered line exactly width cells, padding with the row's
// background.
func fitRow(line string, width int, plain lipgloss.Style) string {
	if lipgloss.Width(line) > width {
		line = ansi.Truncate(line, width, "")
	}
	if gap := width - lipgloss.Width(line); gap > 0 {
		line += plain.Render(strings.Repeat(" ", gap))
	}
	return line
}

func agentGlyph(s mux.AgentStatus, st func(lipgloss.Style) lipgloss.Style) string {
	switch s {
	case mux.AgentBlocked:
		return st(Styles.StatusDanger).Render("!")
	case mux.AgentWorking:
		return st(Styles.StatusWarning).Render("▸")
	case mux.AgentDone:
		return st(Styles.StatusSuccess).Render("✓")
	}
	return st(lipgloss.NewStyle()).Render(" ")
}

func statusGlyph(item WorktreeItem, st func(lipgloss.Style) lipgloss.Style) string {
	switch {
	case item.IsCurrent:
		return st(Styles.StatusSuccess).Render("●")
	case item.IsDirty:
		return st(Styles.StatusWarning).Render("●")
	case item.IsPrunable:
		return st(Styles.StatusDanger).Render("✗")
	}
	return st(Styles.TextMuted).Render("○")
}

// styleCounts colors each count by kind: ahead, behind, dirty.
func styleCounts(counts string, st func(lipgloss.Style) lipgloss.Style, sp string) string {
	if counts == "" {
		return ""
	}
	parts := strings.Split(counts, " ")
	for i, p := range parts {
		s := Styles.StatusWarning
		switch {
		case strings.HasPrefix(p, "↑"):
			s = Styles.StatusSuccess
		case strings.HasPrefix(p, "↓"):
			s = Styles.StatusDanger
		}
		parts[i] = st(s).Render(p)
	}
	return strings.Join(parts, sp)
}

// tailGlyphs is the 4-cell tail: session ⬢ active / ⬡ open, then container
// ◆ running / ◇ stopped.
func tailGlyphs(item WorktreeItem, st func(lipgloss.Style) lipgloss.Style, sp string) string {
	session := st(lipgloss.NewStyle()).Render(" ")
	switch status := mux.Status(item.TmuxStatus); {
	case status.Foreground():
		session = st(Styles.TmuxBadgeActive).Render("⬢")
	case status.Background():
		session = st(Styles.TmuxBadge).Render("⬡")
	}
	container := st(lipgloss.NewStyle()).Render(" ")
	for _, s := range item.PluginStatuses {
		if s.Short == "" {
			continue
		}
		if s.Level == plugins.StatusActive {
			container = st(Styles.ContainerBadgeActive).Render("◆")
			break
		}
		container = st(Styles.ContainerBadge).Render("◇")
	}
	return session + sp + container + sp
}

// worktreeNameStyle returns the style for the alias column.
func worktreeNameStyle(item WorktreeItem, selected bool, withBg func(lipgloss.Style) lipgloss.Style) lipgloss.Style {
	if item.IsCurrent {
		s := withBg(Styles.CurrentItem)
		if selected {
			s = s.Bold(true)
		}
		return s
	}
	if selected {
		return withBg(Styles.SelectedItem)
	}
	return withBg(Styles.NormalItem)
}
