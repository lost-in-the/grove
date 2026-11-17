package tui

import (
	"fmt"
	"io"
	"strings"
	"time"

	"charm.land/bubbles/v2/list"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/lost-in-the/grove/internal/mux"
	"github.com/lost-in-the/grove/internal/plugins"
)

// rowDelegate draws the dashboard's worktree rows in one of two modes
// (docs/TUI.md, Worktree List).
//
// Compact: one line in columns under a heading row (heading): agent,
// cursor, status, name and ≠, branch, a dotted leader, state, age, tail.
//
// Detailed: two lines and a blank line after. Line one: pin, agent, cursor,
// status, name, │, folder, a dotted leader, age, tail. Line two, under the
// name: ⎇ branch, then the state.
type rowDelegate struct {
	Detailed   bool
	Pins       bool              // detailed: draw the pin column
	PinOf      map[string]string // alias -> slot digit
	LastActive string            // display name of the last-active worktree; its row shows ↩
	AgentAware bool              // draw the agent column
	Secondary  string            // compact second column: "branch", "dir", or "off"
	Now        func() time.Time
	// Unfocused draws the selected row without the cursor and the
	// selection bar, for a list beside a focused panel. Its name stays
	// bold, so the row can still be found.
	Unfocused bool
}

// NewDashboardRowDelegate returns the dashboard's row delegate, for tools
// that render the list outside the TUI (cmd/snapshot).
func NewDashboardRowDelegate(detailed bool) list.ItemDelegate {
	return rowDelegate{Detailed: detailed, Secondary: "branch"}
}

func (d rowDelegate) Height() int {
	if d.Detailed {
		return 2
	}
	return 1
}

func (d rowDelegate) Spacing() int {
	if d.Detailed {
		return 1
	}
	return 0
}

func (d rowDelegate) Update(_ tea.Msg, _ *list.Model) tea.Cmd { return nil }

// headRows is the rows the mode draws above the list: compact's heading.
func (d rowDelegate) headRows() int {
	if d.Detailed {
		return 0
	}
	return 1
}

func (d rowDelegate) agentLead() int {
	if d.AgentAware {
		return rowLeadW
	}
	return 0
}

func (d rowDelegate) now() time.Time {
	if d.Now != nil {
		return d.Now()
	}
	return time.Now()
}

// secondText is compact mode's second column for item.
func (d rowDelegate) secondText(item WorktreeItem) string {
	switch d.Secondary {
	case "off":
		return ""
	case "dir":
		return item.folder()
	}
	return item.Branch
}

func (d rowDelegate) secondStyle() lipgloss.Style {
	if d.Secondary == "dir" {
		return Styles.TextMuted
	}
	return Styles.RowBranch
}

// compactLayout decides the compact columns for the whole list, so rows
// line up.
func (d rowDelegate) compactLayout(m list.Model) compactLayout {
	longest, stateW := 0, 0
	for _, li := range m.Items() {
		item, ok := li.(WorktreeItem)
		if !ok {
			continue
		}
		w := lipgloss.Width(item.ShortName)
		if item.FolderOffPattern {
			w += 2 // " ≠"
		}
		longest = max(longest, w)
		stateW = max(stateW, countsWidth(item.AheadCount, item.BehindCount, len(item.DirtyFiles)))
	}
	return layoutCompact(m.Width(), d.agentLead(), longest, stateW, d.Secondary != "off")
}

// heading is compact mode's column heading row, exactly the list's width,
// or "" in detailed mode.
func (d rowDelegate) heading(m list.Model) string {
	if d.Detailed {
		return ""
	}
	l := d.compactLayout(m)
	word := func(s string, w int, right bool) string {
		s = ansi.Truncate(s, w, "")
		gap := strings.Repeat(" ", max(0, w-lipgloss.Width(s)))
		if right {
			return gap + Styles.DimmedItem.Render(s)
		}
		return Styles.DimmedItem.Render(s) + gap
	}
	var b strings.Builder
	b.WriteString(strings.Repeat(" ", l.Lead+2*rowLeadW))
	b.WriteString(word("NAME", l.NameW, false))
	if l.Second {
		second := "BRANCH"
		if d.Secondary == "dir" {
			second = "FOLDER"
		}
		b.WriteString(" " + word(second, l.SecondW, false))
	}
	b.WriteString(" " + word("STATE", l.StateW, true))
	if l.ShowAge {
		b.WriteString(" " + word("AGE", rowAgeW, true))
	}
	return fitRow(b.String(), m.Width(), lipgloss.NewStyle())
}

// rowPaint styles one row's segments: with the selection background when
// the row is lit.
type rowPaint struct{ lit bool }

func (p rowPaint) st(s lipgloss.Style) lipgloss.Style {
	if p.lit {
		return s.Background(Colors.SelectionBg)
	}
	return s
}

func (p rowPaint) blank(n int) string {
	switch {
	case n <= 0:
		return ""
	case !p.lit:
		return strings.Repeat(" ", n)
	}
	return p.st(lipgloss.NewStyle()).Render(strings.Repeat(" ", n))
}

// pad fills s to n cells on the right; padLeft on the left.
func (p rowPaint) pad(s string, n int) string { return s + p.blank(n-lipgloss.Width(s)) }

func (p rowPaint) padLeft(s string, n int) string { return p.blank(n-lipgloss.Width(s)) + s }

// leader is text, a gap, dots, a gap, and state, in exactly span cells. With
// no state the dots run to the end of the span.
func (p rowPaint) leader(text string, span int, state string) string {
	tw, sw := lipgloss.Width(text), lipgloss.Width(state)
	dots := span - tw - sw
	var b strings.Builder
	b.WriteString(text)
	if tw > 0 {
		b.WriteString(p.blank(1))
		dots--
	}
	if sw > 0 {
		dots--
	}
	if dots > 0 {
		b.WriteString(p.st(Styles.RowRule).Render(strings.Repeat("·", dots)))
	}
	if sw > 0 {
		b.WriteString(p.blank(1) + state)
	}
	return p.pad(b.String(), span)
}

func (d rowDelegate) Render(w io.Writer, m list.Model, index int, listItem list.Item) {
	item, ok := listItem.(WorktreeItem)
	if !ok {
		return
	}
	selected := index == m.Index()
	p := rowPaint{lit: selected && !d.Unfocused}
	var out string
	if d.Detailed {
		out = d.renderDetailed(item, m.Width(), selected, p)
	} else {
		out = d.renderCompact(item, d.compactLayout(m), m.Width(), selected, p)
	}
	_, _ = fmt.Fprint(w, out)
}

// lead is the columns before the name: pin (detailed), agent, cursor or ↩,
// and status.
func (d rowDelegate) lead(item WorktreeItem, p rowPaint) string {
	var b strings.Builder
	if d.Detailed && d.Pins {
		b.WriteString(p.pad(p.st(Styles.TextMuted).Render(d.PinOf[item.ShortName]), rowLeadW))
	}
	if d.AgentAware {
		b.WriteString(p.pad(agentGlyph(item.AgentStatus, p.st), rowLeadW))
	}
	switch {
	case p.lit:
		b.WriteString(p.st(Styles.ListCursor).Render("❯") + p.blank(1))
	case d.LastActive != "" && !item.IsCurrent && item.displayName() == d.LastActive:
		b.WriteString(p.st(Styles.TextMuted).Render("↩") + p.blank(1))
	default:
		b.WriteString(p.blank(rowLeadW))
	}
	b.WriteString(statusGlyph(item, p.st) + p.blank(1))
	return b.String()
}

// tail is the session and container glyphs, then the blank cell kept for a
// pull request mark, after a one-cell gap.
func tail(item WorktreeItem, p rowPaint) string {
	sp := p.blank(1)
	return sp + tailGlyphs(item, p.st, sp) + p.blank(1)
}

func (d rowDelegate) renderCompact(item WorktreeItem, l compactLayout, width int, selected bool, p rowPaint) string {
	sp := p.blank(1)
	nameW, mark := l.NameW, ""
	if item.FolderOffPattern {
		nameW -= 2
		mark = sp + p.st(Styles.StatusWarning).Render("≠")
	}
	name := worktreeNameStyle(item, selected, p.st).Render(middleTruncate(item.ShortName, max(1, nameW))) + mark

	counts := rowCounts(item.AheadCount, item.BehindCount, len(item.DirtyFiles), l.Collapsed)
	if lipgloss.Width(counts) > l.StateW {
		counts = rowCounts(item.AheadCount, item.BehindCount, len(item.DirtyFiles), true)
	}
	state := styleCounts(counts, p.st, sp)

	var b strings.Builder
	b.WriteString(d.lead(item, p))
	if l.Second {
		b.WriteString(p.pad(name, l.NameW) + sp)
		text := p.st(d.secondStyle()).Render(middleTruncate(d.secondText(item), max(0, l.SecondW-2)))
		b.WriteString(p.leader(text, l.SecondW+1+l.StateW, state))
	} else {
		b.WriteString(p.leader(name, l.NameW+1+l.StateW, state))
	}
	if l.ShowAge {
		b.WriteString(sp + p.padLeft(p.st(Styles.TextMuted).Render(shortAge(item.CommitTime, d.now())), rowAgeW))
	}
	b.WriteString(tail(item, p))
	return fitRow(b.String(), width, p.st(lipgloss.NewStyle()))
}

// rowFolderMin is the fewest folder cells worth drawing; with less room the
// divider and the folder go.
const rowFolderMin = 8

func (d rowDelegate) renderDetailed(item WorktreeItem, width int, selected bool, p rowPaint) string {
	sp := p.blank(1)
	plain := p.st(lipgloss.NewStyle())
	lead := d.lead(item, p)
	leadW := lipgloss.Width(lead)

	nameStyle := p.st(Styles.TextBright)
	if item.IsCurrent {
		nameStyle = p.st(Styles.CurrentItem).Bold(true)
	}
	// The name is never cut while anything else can give way: first the
	// age, then the tail moves to line two.
	nameText, nameW := item.ShortName, lipgloss.Width(item.ShortName)
	tailOne, tailTwo := tail(item, p), ""
	right := sp + p.padLeft(p.st(Styles.TextMuted).Render(shortAge(item.CommitTime, d.now())), rowAgeW) + tailOne
	if nameW > width-leadW-lipgloss.Width(right) {
		right = tailOne
	}
	if nameW > width-leadW-lipgloss.Width(right) {
		right, tailTwo = "", tailOne
	}
	avail := width - leadW - lipgloss.Width(right)
	if nameW > avail {
		nameText = cutMiddle(nameText, max(1, avail))
		nameW = lipgloss.Width(nameText)
	}
	rest := avail - nameW

	var one strings.Builder
	one.WriteString(lead + nameStyle.Render(nameText))
	// After the name: " │ ", the folder and "/", then " ·" at least.
	switch folder, budget := item.folder(), rest-3-1-2; {
	case folder != "" && budget >= rowFolderMin:
		folderStyle := Styles.TextMuted
		if item.FolderOffPattern {
			folderStyle = Styles.StatusWarning
		}
		f := p.st(folderStyle).Render(middleTruncate(folder, budget) + "/")
		one.WriteString(sp + p.st(Styles.RowRule).Render("│") + sp)
		one.WriteString(p.leader(f, rest-3, ""))
	case item.FolderOffPattern && rest >= 2:
		one.WriteString(sp + p.leader(p.st(Styles.StatusWarning).Render("≠"), rest-1, ""))
	case rest > 0:
		one.WriteString(sp + p.leader("", rest-1, ""))
	}
	one.WriteString(right)

	counts := rowCounts(item.AheadCount, item.BehindCount, len(item.DirtyFiles), false)
	stateW := func() int {
		if counts == "" {
			return 0
		}
		return 3 + lipgloss.Width(counts)
	}
	avail2 := width - leadW - lipgloss.Width(tailTwo)
	if 2+lipgloss.Width(item.Branch)+stateW() > avail2 && counts != "" {
		counts = rowCounts(item.AheadCount, item.BehindCount, len(item.DirtyFiles), true)
	}
	var two strings.Builder
	two.WriteString(p.blank(leadW))
	if item.Branch != "" {
		two.WriteString(p.st(Styles.RowBranch).Render("⎇ " + middleTruncate(item.Branch, max(1, avail2-2-stateW()))))
	}
	if counts != "" {
		two.WriteString(p.blank(3) + styleCounts(counts, p.st, sp))
	}
	line2 := fitRow(two.String(), width-lipgloss.Width(tailTwo), plain) + tailTwo
	return fitRow(one.String(), width, plain) + "\n" + line2
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

// tailGlyphs is the session ⬢ active / ⬡ open, a gap, then the container
// ◆ running / ◇ stopped, a gap.
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

// worktreeNameStyle returns the style for the compact name column.
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
