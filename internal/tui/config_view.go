package tui

import (
	"fmt"
	"slices"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
)

// configSectionNames are the tab labels, in section order.
var configSectionNames = [ConfigTabCount]string{"general", "behavior", "plugins", "protection"}

// configValueLabelW is the width of "Value" and "Default" in a text
// field's context.
const configValueLabelW = 9

// Side by side, the list is a column of configColumnMin to configColumnMax
// cells, with configColumnValueW for the value; a longer value ends in an
// ellipsis there, since the context panel shows it whole. The context panel
// is at most configMeasure wide, its one-cell indent included, so its text
// stays readable on a wide terminal.
const (
	configColumnMin    = 44
	configColumnMax    = 56
	configColumnValueW = 24
	configMeasure      = 76
)

// configLayout is the frame's geometry at the terminal size. It depends on
// the size and the fields only, so every section and every focused field
// share it.
type configLayout struct {
	inner   int  // width inside the frame's one-cell padding
	wide    bool // the list and the context panel side by side
	labelW  int  // the list's label column
	listW   int  // the list's width, and its selection bar's
	listH   int  // rows for the list: the tallest section's, or the column's
	ctxW    int  // the context panel's width
	ctxH    int  // rows for the context panel
	footerH int
}

func (m Model) configLayout() configLayout {
	s := m.configState
	l := configLayout{inner: max(20, m.width-2), wide: isWideLayout(m.width)}
	for _, fields := range s.Fields {
		for _, f := range fields {
			l.labelW = max(l.labelW, lipgloss.Width(f.Label))
		}
		l.listH = max(l.listH, len(configListShape(fields)))
	}
	l.labelW = min(l.labelW, max(8, l.inner-2-2-8)) // leave the value 8 cells
	l.listH = max(1, l.listH)
	for _, hints := range configFooterVariants() {
		l.footerH = max(l.footerH, lipgloss.Height(m.helpFooter.RenderCompactWithHints(hints, m.width-4)))
	}
	body := max(0, m.height-1-2-l.footerH) // under the header, the tabs, and a blank
	if l.wide {
		l.listW = min(configColumnMax, max(configColumnMin, 2+l.labelW+2+configColumnValueW))
		l.ctxW = min(configMeasure, l.inner-l.listW-3) // the divider takes 3 cells
		l.listH, l.ctxH = body, body
		return l
	}
	l.listW, l.ctxW = l.inner, l.inner
	l.ctxH = max(0, body-l.listH-1)
	return l
}

// configListRow is one row of a section's list: a field, or a group
// heading, or a blank between groups (field -1 for both).
type configListRow struct {
	field   int
	heading string
}

// configListShape lays out a section's list. Headings appear only when the
// section has more than one group.
func configListShape(fields []ConfigField) []configListRow {
	groups := 0
	for i, f := range fields {
		if i == 0 || f.Group != fields[i-1].Group {
			groups++
		}
	}
	var rows []configListRow
	for i, f := range fields {
		if groups > 1 && (i == 0 || f.Group != fields[i-1].Group) {
			if i > 0 {
				rows = append(rows, configListRow{field: -1})
			}
			rows = append(rows, configListRow{field: -1, heading: f.Group})
		}
		rows = append(rows, configListRow{field: i})
	}
	return rows
}

// configHints are the keys that work in the panel's current state.
func (s *ConfigState) configHints() []Hint {
	f := s.focused()
	switch {
	case f == nil:
		return []Hint{{"esc", "close"}}
	case s.confirming:
		return []Hint{{"enter", "change"}, {"esc", "cancel"}}
	case s.editing:
		return []Hint{{"enter", "save"}, {"esc", "cancel"}}
	}
	return configFieldHints(f.Type, len(s.undo) > 0)
}

func configFieldHints(t ConfigFieldType, canUndo bool) []Hint {
	change := Hint{"←→/hl", "change"}
	if t == ConfigString || t == ConfigList {
		change = Hint{"enter", "edit"}
	}
	hints := []Hint{{"↑↓/jk", "field"}, change, {"", "saves as you go"}}
	if canUndo {
		hints = append(hints, Hint{"u", "undo"})
	}
	return append(hints, Hint{"esc", "close"})
}

// configFooterVariants are every hint line the panel can show, so the
// footer's height is the same whichever is shown.
func configFooterVariants() [][]Hint {
	return [][]Hint{
		configFieldHints(ConfigEnum, true),
		configFieldHints(ConfigString, true),
		{{"enter", "save"}, {"esc", "cancel"}},
		{{"enter", "change"}, {"esc", "cancel"}},
	}
}

// configFooter is the hint line, padded to the layout's footer height.
func (m Model) configFooter() string {
	footer := m.helpFooter.RenderCompactWithHints(m.configState.configHints(), m.width-4)
	return clampLines(footer, m.configLayout().footerH)
}

// renderConfigFrame draws the settings panel: header, section tabs, the
// list, and the focused field's context, then the footer. The context sits
// beside the list at the dashboard's split width, else under a rule.
func (m Model) renderConfigFrame() string {
	s := m.configState
	l := m.configLayout()

	lines := []string{renderConfigTabs(s.Tab, l.inner, !s.editing), ""}
	var list, ctx []string
	switch {
	case s.Err != nil:
		list = []string{" " + Styles.ErrorText.Render("Settings did not load.")}
		ctx = wrapStyled(s.Err.Error(), l.ctxW-1, Styles.ErrorText)
	case s.focused() == nil:
		list = []string{" " + Styles.DetailDim.Render("Loading settings…")}
	default:
		list = s.listLines(l)
		ctx = s.contextLines(l, m.otherWorktreeCount())
	}
	if l.wide {
		left := lipgloss.NewStyle().Width(l.listW).Render(strings.Join(padRows(list, l.listH), "\n"))
		right := strings.Join(padRows(ctx, l.ctxH), "\n")
		lines = append(lines, lipgloss.JoinHorizontal(lipgloss.Top,
			left, renderVerticalDivider(l.listH, Colors.SurfaceDim), right))
	} else {
		lines = append(lines, padRows(list, l.listH)...)
		lines = append(lines, Styles.TextMuted.Render(strings.Repeat("─", l.inner)))
		lines = append(lines, padRows(ctx, l.ctxH)...)
	}

	body := lipgloss.NewStyle().Padding(0, 1).Render(strings.Join(lines, "\n"))
	footer := m.configFooter()
	body = m.clampBodyToHeight(body, footer)
	return lipgloss.JoinVertical(lipgloss.Left, m.configHeader(), body, footer)
}

// padRows is lines cut or padded to exactly n rows.
func padRows(lines []string, n int) []string {
	out := make([]string, n)
	copy(out, lines)
	return out
}

// wrapStyled wraps plain text to width and styles each line, indented one
// cell to sit under the context panel's title.
func wrapStyled(text string, width int, style lipgloss.Style) []string {
	var out []string
	for _, l := range wrapWords(text, width) {
		out = append(out, " "+style.Render(l))
	}
	return out
}

// wrapWords breaks text at spaces only, so a placeholder like
// {project}-{name} stays whole; a word longer than width is cut.
func wrapWords(text string, width int) []string {
	width = max(1, width)
	var lines []string
	line := ""
	for _, word := range strings.Fields(text) {
		for lipgloss.Width(word) > width {
			if line != "" {
				lines, line = append(lines, line), ""
			}
			head := ansi.Truncate(word, width, "")
			lines, word = append(lines, head), word[len(head):]
		}
		switch {
		case line == "":
			line = word
		case lipgloss.Width(line)+1+lipgloss.Width(word) <= width:
			line += " " + word
		default:
			lines, line = append(lines, line), word
		}
	}
	return append(lines, line)
}

// configHeader is "<project> ▸ settings", with the last write's outcome on
// the right.
func (m Model) configHeader() string {
	inner := max(0, m.width-2) // HeaderBar pads one cell each side
	left := Styles.Header.Render(m.projectName) + Styles.TextMuted.Render(" ▸ ") + Styles.TextBright.Render("settings")
	right := m.configState.headerNote()
	left = ansi.Truncate(left, max(0, inner-lipgloss.Width(right)-1), "…")
	gap := max(1, inner-lipgloss.Width(left)-lipgloss.Width(right))
	content := ansi.Truncate(left+strings.Repeat(" ", gap)+right, inner, "")
	return headerBar(content, m.width)
}

// headerNote says how the last write went: an error stays until the next
// write; "saved" goes after the toast timeout.
func (s *ConfigState) headerNote() string {
	undo := ""
	if len(s.undo) > 0 {
		undo = Styles.TextMuted.Render(" · u undo")
	}
	switch {
	case s.Err != nil:
		return Styles.ErrorText.Render("✗ settings did not load")
	case s.saveErr != nil:
		return Styles.ErrorText.Render("✗ not saved")
	case s.flash == "saved", s.flash == "undone":
		return Styles.SuccessText.Render("✓ "+s.flash) + undo
	case s.flash != "":
		return Styles.TextMuted.Render(s.flash)
	}
	return ""
}

// renderConfigTabs draws the section tabs with the active one raised. While
// a field is being typed in, digits type, so the tabs drop their numbers.
func renderConfigTabs(active ConfigTab, width int, numbered bool) string {
	labels := configSectionNames[:]
	if numbered {
		return renderTabRow(labels, int(active), width)
	}
	parts := make([]string, len(labels))
	for i, l := range labels {
		style := Styles.TabInactive
		if i == int(active) {
			style = Styles.TabActive
		}
		parts[i] = style.Render(l)
	}
	row := strings.Join(parts, " ")
	if lipgloss.Width(row) > width {
		row = Styles.TabActive.Render(labels[active])
	}
	return row
}

// renderTabRow draws "[1] label  [2] label ..." with the active tab raised.
// When the full row does not fit, inactive tabs shrink to their number.
func renderTabRow(labels []string, active, width int) string {
	build := func(short bool) string {
		parts := make([]string, 0, len(labels))
		for i, l := range labels {
			label := fmt.Sprintf("[%d] %s", i+1, l)
			style := Styles.TabInactive
			if i == active {
				style = Styles.TabActive
			} else if short {
				label = fmt.Sprintf("[%d]", i+1)
			}
			parts = append(parts, style.Render(label))
		}
		return strings.Join(parts, " ")
	}
	row := build(false)
	if lipgloss.Width(row) > width {
		row = build(true)
	}
	return row
}

// listLines is the active section's list: a row per field with its label
// and value, under group headings. The focused row is drawn as the
// dashboard draws its selected row.
func (s *ConfigState) listLines(l configLayout) []string {
	fields := s.Fields[s.Tab]
	var out []string
	for _, row := range configListShape(fields) {
		switch {
		case row.heading != "":
			out = append(out, " "+Styles.Title.Render(row.heading))
		case row.field < 0:
			out = append(out, "")
		default:
			out = append(out, s.listRow(fields[row.field], row.field == s.cursor[s.Tab], l))
		}
	}
	return out
}

func (s *ConfigState) listRow(f ConfigField, selected bool, l configLayout) string {
	st := func(style lipgloss.Style) lipgloss.Style {
		if selected {
			return style.Background(Colors.SelectionBg)
		}
		return style
	}
	cursor, label := st(lipgloss.NewStyle()).Render("  "), st(Styles.NormalItem)
	if selected {
		cursor = st(Styles.ListCursor).Render("❯") + st(lipgloss.NewStyle()).Render(" ")
		label = st(Styles.SelectedItem)
	}
	value, valueStyle := f.Value, st(Styles.TextNormal)
	if value == "" {
		value, valueStyle = f.Empty, st(Styles.TextMuted)
	}
	valueW := max(1, l.listW-2-l.labelW-2)
	row := cursor + label.Render(padCells(f.Label, l.labelW)) + st(lipgloss.NewStyle()).Render("  ") +
		valueStyle.Render(ansi.Truncate(value, valueW, "…"))
	if gap := l.listW - lipgloss.Width(row); gap > 0 {
		row += st(lipgloss.NewStyle()).Render(strings.Repeat(" ", gap))
	}
	return row
}

// contextLines explain the focused field: its label and key, what it does,
// what changing it does to other worktrees, then its choices or its
// value, cut to fit the panel.
func (s *ConfigState) contextLines(l configLayout, others int) []string {
	f := s.focused()
	w := l.ctxW - 1

	label, keyText := Styles.TextBright.Render(f.Label), Styles.TextMuted.Render(f.Key)
	head := []string{" " + label}
	if lipgloss.Width(f.Label)+2+lipgloss.Width(f.Key) <= w {
		head[0] += "  " + keyText
	} else {
		head = append(head, " "+keyText)
	}

	c := contextParts{head: head, desc: wrapStyled(f.Description, w, Styles.TextNormal)}
	if f.renamesWorktrees && others > 0 {
		c.warn = hangingWrap(renameWarning(others), w, Styles.WarningText)
	}
	switch f.Type {
	case ConfigEnum, ConfigBool:
		c.body = s.optionLines(f, w)
	default:
		c.body, c.help = s.valueLines(f, w)
	}
	switch {
	case s.confirming:
		c.msg = wrapStyled(confirmText(f), w, Styles.WarningText)
	case s.editing && s.editErr != "":
		c.msg = wrapStyled(s.editErr, w, Styles.ErrorText)
	}
	if s.saveErr != nil && s.saveErrKey == f.Key {
		c.msg = append(c.msg, wrapStyled("Not saved: "+s.saveErr.Error(), w, Styles.ErrorText)...)
	}
	return c.fit(l.ctxH)
}

// renameWarning says what changing the project name or folder pattern does
// to the n worktrees besides root.
func renameWarning(n int) string {
	if n == 1 {
		return "⚠ This project has 1 worktree besides root. Changing this does not rename its folder. " +
			"Grove will stop recognizing it by its short name."
	}
	return fmt.Sprintf("⚠ This project has %d worktrees besides root. Changing this does not rename their folders. "+
		"Grove will stop recognizing them by their short names.", n)
}

// confirmText is the status line of the confirm step: the second enter
// writes, esc keeps the value in the file.
func confirmText(f *ConfigField) string {
	keeps := "esc leaves it not set."
	if f.Value != "" {
		keeps = fmt.Sprintf("esc keeps %q.", f.Value)
	}
	return "Press enter again to change. " + keeps
}

// hangingWrap is wrapStyled for text that opens with a glyph and a space:
// the lines after the first sit under the text, not the glyph.
func hangingWrap(text string, width int, style lipgloss.Style) []string {
	glyph, rest, _ := strings.Cut(text, " ")
	indent := lipgloss.Width(glyph) + 1
	out := wrapStyled(rest, width-indent, style)
	for i := range out {
		lead := strings.Repeat(" ", indent)
		if i == 0 {
			lead = style.Render(glyph) + " "
		}
		out[i] = " " + lead + out[i][1:]
	}
	return out
}

// optionLines lists every choice with its meaning: ● marks the current one.
// A meaning too long for the line wraps under itself.
func (s *ConfigState) optionLines(f *ConfigField, w int) []string {
	optW := 0
	for _, o := range f.Options {
		optW = max(optW, lipgloss.Width(o.Value))
	}
	indent := strings.Repeat(" ", 2+2+optW+3)
	var out []string
	for _, o := range f.Options {
		mark, value := Styles.TextMuted.Render("○"), Styles.TextNormal.Render(padCells(o.Value, optW))
		if o.Value == f.Value {
			mark, value = Styles.ListCursor.Render("●"), Styles.TextBright.Render(padCells(o.Value, optW))
		}
		meaning := o.Meaning
		if o.Value == f.Default {
			meaning += " (default)"
		}
		for i, l := range wrapWords(meaning, w+1-len(indent)) {
			l = strings.Replace(Styles.TextNormal.Render(l), "(default)", Styles.TextMuted.Render("(default)"), 1)
			if i == 0 {
				out = append(out, "  "+mark+" "+value+"   "+l)
			} else {
				out = append(out, indent+l)
			}
		}
	}
	return out
}

// valueLines show a text or list field's value (or the field being typed
// in), then as help its default and what to type.
func (s *ConfigState) valueLines(f *ConfigField, w int) ([]string, []string) {
	row := func(name, value string) string {
		return ansi.Truncate("  "+Styles.DetailLabel.Render(padCells(name, configValueLabelW))+value, w+1, "…")
	}
	valueW := max(1, w-1-configValueLabelW)
	var out []string
	switch {
	case s.editing:
		in := s.input
		in.SetWidth(max(1, valueW-2)) // the prompt and the cursor take a cell each
		out = []string{row("Value", in.View())}
	case f.Value == "":
		out = []string{row("Value", Styles.TextMuted.Render(f.Empty))}
	default:
		// A long value, such as a path, wraps rather than hides its end.
		for i, l := range wrapWords(f.Value, valueW) {
			name := ""
			if i == 0 {
				name = "Value"
			}
			out = append(out, row(name, Styles.TextBright.Render(l)))
		}
	}
	help := []string{row("Default", Styles.TextNormal.Render(f.Default))}
	for _, note := range f.Notes {
		for _, l := range wrapStyled(note, w-1, Styles.TextMuted) {
			help = append(help, " "+l)
		}
	}
	return out, help
}

// contextParts are the context panel's pieces, top to bottom: the title,
// the description, the warning, then after a blank the choices or value,
// the help under a value, and the message.
type contextParts struct {
	head, desc, warn, body, help, msg []string
}

// fit joins the parts in h rows. When they do not fit, the blank goes
// first, then help lines from the end, then description lines, then body
// lines from the end, each cut marked with an ellipsis. The title, the
// warning, and the message (a refused value, the confirm step, a write
// error) are kept whole: they are what needs reading now.
func (c contextParts) fit(h int) []string {
	join := func(gap bool) []string {
		out := slices.Concat(c.head, c.desc, c.warn)
		if gap {
			out = append(out, "")
		}
		return slices.Concat(out, c.body, c.help, c.msg)
	}
	if out := join(true); len(out) <= h {
		return out
	}
	if out := join(false); len(out) <= h {
		return out
	}
	room := h - len(c.head) - len(c.warn) - len(c.msg)
	c.help = cutLines(c.help, max(0, room-len(c.desc)-len(c.body)))
	c.desc = cutLines(c.desc, max(0, min(len(c.desc), room-len(c.body)-len(c.help))))
	c.body = cutLines(c.body, max(0, room-len(c.desc)-len(c.help)))
	return padRows(join(false), h)
}

// cutLines keeps the first n lines, marking the last kept one with an
// ellipsis when any were dropped.
func cutLines(lines []string, n int) []string {
	if n >= len(lines) {
		return lines
	}
	lines = append([]string{}, lines[:n]...)
	if n > 0 {
		last := lines[n-1]
		lines[n-1] = ansi.Truncate(last, max(1, lipgloss.Width(last)-1), "") + "…"
	}
	return lines
}
