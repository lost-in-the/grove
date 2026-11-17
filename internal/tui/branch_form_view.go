package tui

import (
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/lost-in-the/grove/internal/worktree"
)

const (
	formSuggestRows = 4
	formLabelWidth  = 10
	// formBodyRows fits New mode: Branch, Base or its blank, Name, the
	// folder and session lines, and the suggestion rows under the focused
	// field. Copy and Switch fit in the same rows.
	formBodyRows = 5 + formSuggestRows
	// formInnerRows adds a blank row, the status line, and the key line.
	// Where the key line needs two rows, it takes the blank row's place.
	formInnerRows = formBodyRows + 3
	// formPanelRows adds the title rule on top.
	formPanelRows = 1 + formInnerRows
	// formMinListRows is the fewest list rows worth keeping above the
	// bottom dock. With fewer, the form takes the whole screen.
	formMinListRows = 3
)

type formDock int

const (
	dockRight  formDock = iota // split width: in the detail pane's place
	dockBottom                 // under the list, full width
	dockFull                   // the whole screen: too short to keep a list
)

// formLayout is where the form sits and its size, chrome included. It
// depends only on the terminal and the list beside it, never on the form's
// state, so the form cannot change size while the user works in it.
type formLayout struct {
	Dock  formDock
	Width int
	Rows  int
}

// branchFormLayout places the form. listW is the list column kept beside
// the form at split widths; 0 means the view beneath has no list there.
func branchFormLayout(termW, termH, listW int) formLayout {
	if isWideLayout(termW) && listW > 0 {
		return formLayout{Dock: dockRight, Width: termW - 2 - listW - 3, Rows: max(2, min(formPanelRows, termH-1))}
	}
	if termH-1-formPanelRows < formMinListRows {
		return formLayout{Dock: dockFull, Width: termW - 2, Rows: max(2, termH)}
	}
	return formLayout{Dock: dockBottom, Width: termW - 2, Rows: formPanelRows}
}

// innerWidth is the width of a form line.
func (l formLayout) innerWidth() int { return max(8, l.Width) }

// renderBranchForm draws the form at exactly lay's size: a title rule, then
// the fields, in every dock. The right pane has no side border so the whole
// key line fits at 120 columns. The status line and key line are pinned to
// the bottom; the body scrolls above them to keep the focused field visible
// when the terminal is short.
func renderBranchForm(s *BranchFormState, lay formLayout, spinner string) string {
	inner, rows := lay.innerWidth(), lay.Rows-1
	bodyRows := max(1, rows-3)

	body, focus := s.bodyLines(inner, spinner, bodyRows >= formBodyRows)
	body = windowLines(body, bodyRows, focus)

	msg, kind := s.status()
	keyRows := s.keyRows(inner)
	keys := renderKeyLine(s.keyFooter(), inner, keyRows)
	keys = append(keys, make([]string, max(0, keyRows-len(keys)))...)
	if keyRows == 1 {
		body = append(body, "")
	}
	lines := append(body, formStatusLine(msg, kind))
	lines = append(lines, keys...)
	if len(lines) > rows {
		lines = lines[len(lines)-rows:]
	}
	for i, l := range lines {
		lines[i] = padCells(l, inner)
	}
	return formRule(s.title(), lay.Width) + "\n" + strings.Join(lines, "\n")
}

// formRule is the form's top edge: a rule with the title in it, in the
// focus color, as a focused pane's border is.
func formRule(title string, width int) string {
	rule := lipgloss.NewStyle().Foreground(Colors.Primary)
	label := " " + title + " "
	right := max(0, width-2-lipgloss.Width(label))
	return rule.Render("──") + Styles.DetailTitle.Render(label) + rule.Render(strings.Repeat("─", right))
}

// formStatusLine renders the status line. A blocker and a failure carry
// the ✗ glyph, so they read as errors without color.
func formStatusLine(msg string, kind formStatusKind) string {
	if kind == formStatusBlocker || kind == formStatusError {
		msg = "✗ " + msg
	}
	return formStatusStyle(kind).Render(msg)
}

func formStatusStyle(kind formStatusKind) lipgloss.Style {
	switch kind {
	case formStatusBlocker, formStatusError:
		return Styles.ErrorText
	case formStatusReady:
		return Styles.SuccessText
	case formStatusNotice, formStatusBusy:
		return Styles.WarningText
	}
	return Styles.DetailDim
}

// padCells truncates or pads s to exactly n terminal cells.
func padCells(s string, n int) string {
	s = ansi.Truncate(s, n, "…")
	if pad := n - lipgloss.Width(s); pad > 0 {
		s += strings.Repeat(" ", pad)
	}
	return s
}

// windowLines returns exactly rows lines of lines, scrolled so that line
// focus stays visible.
func windowLines(lines []string, rows, focus int) []string {
	if len(lines) <= rows {
		for len(lines) < rows {
			lines = append(lines, "")
		}
		return lines
	}
	start := max(0, min(focus-rows/2, len(lines)-rows))
	return lines[start : start+rows]
}

// suggestionMark marks the highlighted suggestion: lighter than the ❯ on
// the focused field, so the form shows one cursor arrow.
const suggestionMark = "› "

// label is a field's label column. The field a blocker blames reads
// "Name ✗" in the error token, whether or not it has focus. Changes says
// "check failed" in its value instead.
func (s *BranchFormState) label(name string, f formField) string {
	marker := "  "
	if s.Focus == f {
		marker = Styles.ListCursor.Render("❯ ")
	}
	if fault, ok := s.faultField(); ok && fault == f && f != fieldChanges {
		return marker + Styles.ErrorText.Render(fmt.Sprintf("%-8s", name+" ✗"))
	}
	return marker + Styles.DetailLabel.Render(fmt.Sprintf("%-8s", name))
}

// noField labels a read-only row; it never takes focus.
const noField formField = -1

// hasSuggestions reports whether field f opens suggestions under itself.
func (s *BranchFormState) hasSuggestions(f formField) bool {
	switch s.Mode {
	case formModeCopy:
		return false
	case formModeSwitch:
		return f == fieldBranch
	}
	return f == fieldBranch || (f == fieldBase && s.isNewBranch())
}

// bodyLines returns the body and the index of the line to keep in view.
// Each field is one row; the focused field's suggestion rows open under it.
// When the focused field has none, those rows are blank at the end, so the
// body is the same height whichever field has focus. roomy puts the folder
// and the session on lines of their own.
func (s *BranchFormState) bodyLines(inner int, spinner string, roomy bool) ([]string, int) {
	if s.Log != nil && (s.Busy || s.Log.IsDone()) {
		// Keep the newest line, which carries the spinner or the result,
		// in view when the terminal is too short for the whole log.
		lines := strings.Split(strings.TrimRight(s.Log.View(spinner), "\n"), "\n")
		return lines, len(lines) - 1
	}
	indent := strings.Repeat(" ", formLabelWidth)
	var lines []string
	focus, placed := 0, false
	row := func(f formField, line string) {
		lines = append(lines, line)
		if f != s.Focus {
			return
		}
		focus = len(lines) - 1
		if s.hasSuggestions(f) {
			lines = append(lines, s.suggestionLines(indent, inner)...)
			focus += formSuggestRows / 2
			placed = true
		}
	}
	switch s.Mode {
	case formModeCopy:
		row(fieldBranch, s.label("Branch", fieldBranch)+s.Branch.View())
		row(noField, indent+Styles.DetailDim.Render(fmt.Sprintf("new branch from %s's HEAD", s.Source.ShortName)))
		row(noField, s.label("From", noField)+Styles.DetailDim.Render(s.Source.ShortName+" · "+sourceBranch(s.Source)))
		row(fieldAlias, s.label("Name", fieldAlias)+s.Alias.View())
		lines = append(lines, s.previewLines(indent, roomy)...)
		row(fieldChanges, s.changesRow(fieldChanges))
		return lines, focus
	case formModeSwitch:
		row(fieldBranch, s.label("Branch", fieldBranch)+s.Branch.View())
		row(noField, s.label("In", noField)+Styles.DetailDim.Render(s.Source.ShortName))
		row(noField, s.label("Now", noField)+Styles.DetailDim.Render(sourceBranch(s.Source)))
		row(noField, s.changesRow(noField))
	default:
		row(fieldBranch, s.label("Branch", fieldBranch)+s.Branch.View())
		if s.isNewBranch() {
			row(fieldBase, s.label("Base", fieldBase)+s.Base.View())
		} else {
			row(noField, "")
		}
		row(fieldAlias, s.label("Name", fieldAlias)+s.Alias.View())
		lines = append(lines, s.previewLines(indent, roomy)...)
	}
	if !placed {
		lines = append(lines, make([]string, formSuggestRows)...)
	}
	return lines, focus
}

// sourceBranch names the source's branch, or says it has none.
func sourceBranch(w *WorktreeItem) string {
	if w.Branch == "" {
		return "detached HEAD"
	}
	return w.Branch
}

// previewLines name the folder and the session the name produces: one line
// each when roomy, else both on one line.
func (s *BranchFormState) previewLines(indent string, roomy bool) []string {
	a := s.aliasValue()
	if a == "" {
		if roomy {
			return []string{"", ""}
		}
		return []string{""}
	}
	fact := func(label, value string) string {
		return Styles.DetailDim.Render(label+" ") + Styles.TextNormal.Render(value)
	}
	folder := fact("folder", worktree.InterpolateNamePattern(s.opts.NamePattern, s.opts.ProjectName, a))
	session := fact("session", worktree.TmuxSessionName(s.opts.ProjectName, a))
	if !roomy {
		return []string{indent + folder + Styles.DetailDim.Render(" · ") + session}
	}
	return []string{indent + folder, indent + session}
}

// changesRow is the Changes field, or a blank row when there is nothing to
// say about changes.
func (s *BranchFormState) changesRow(f formField) string {
	line := s.changesLine()
	if line == "" {
		return ""
	}
	return s.label("Changes", f) + line
}

// changesLine describes the source's uncommitted changes and, in Copy
// mode, what will happen to them. It is blank while the check runs and when
// there are none; the status line covers both.
func (s *BranchFormState) changesLine() string {
	switch {
	case s.wipErr != nil:
		return Styles.ErrorText.Render("check failed")
	case !s.HasWIP:
		return ""
	case s.Mode == formModeSwitch:
		return Styles.WarningText.Render("stash " + fileCount(len(s.WIPFiles)) + " before switching")
	}
	parts := make([]string, len(changesOrder))
	for i, w := range changesOrder {
		mark := "○ "
		if w == s.Changes {
			mark = "● "
		}
		parts[i] = mark + changesLabel(w)
	}
	return strings.Join(parts, "  ") + Styles.DetailDim.Render(" · "+fileCount(len(s.WIPFiles)))
}

// suggestionLines always returns formSuggestRows lines, so the body never
// changes height.
func (s *BranchFormState) suggestionLines(indent string, inner int) []string {
	out := make([]string, formSuggestRows)
	sg := s.suggestions()
	start := max(0, min(s.Pick-formSuggestRows+1, len(sg)-formSuggestRows))
	for i := 0; i < formSuggestRows && start+i < len(sg); i++ {
		x := sg[start+i]
		cursor := "  "
		if start+i == s.Pick {
			cursor = Styles.ListCursor.Render(suggestionMark)
		}
		name := x.Branch
		if x.New {
			name = fmt.Sprintf("+ new %q", x.Branch)
		}
		nameW := max(10, inner-formLabelWidth-2-lipgloss.Width(x.Note)-2)
		out[i] = indent + cursor + padCells(name, nameW) + "  " + Styles.DetailDim.Render(x.Note)
	}
	for i := range out {
		if out[i] == "" {
			out[i] = indent
		}
	}
	return out
}

// formBelow is the view the form is docked into: the frame under the form
// on the stack, or, before the form opens, the view it opens over.
func (m Model) formBelow() ActiveView {
	for i := len(m.views) - 1; i >= 0; i-- {
		if m.views[i].view != ViewBranchForm {
			continue
		}
		if i == 0 {
			return ViewDashboard
		}
		return m.views[i-1].view
	}
	return m.activeView()
}

// formListWidth is the list column of view v at split widths, which the
// form docks beside. 0 means the form docks under v instead: v has no list
// to keep in view, or the dashboard's sidebar is hidden.
func (m Model) formListWidth(v ActiveView) int {
	switch v {
	case ViewDashboard:
		if m.sidebarHidden {
			return 0
		}
		return m.list.Width()
	case ViewPRs, ViewIssues:
		return contextListWidth(m.width)
	}
	return 0
}

func (m Model) branchFormLayout() formLayout {
	return branchFormLayout(m.width, m.height, m.formListWidth(m.formBelow()))
}

// formListAt renders view v's header and its list at w x rows with the
// cursor row in view, or ok false when v has no list to keep. The form has
// focus, so the list draws no cursor arrow.
func (m Model) formListAt(v ActiveView, w, rows int) (header, body string, ok bool) {
	switch v {
	case ViewDashboard:
		l := m.list
		idx := l.Index()
		d := m.rowDelegate()
		d.Unfocused = true
		l.SetDelegate(d)
		l.SetSize(w, max(1, rows-1-d.headRows())) // under the filter row and the heading
		l.Select(idx)
		return m.buildDashboardHeader().View(m.width), m.dashboardListView(l, d), true
	case ViewPRs:
		if m.prState != nil {
			return renderContextHeader("Pull Requests", m.projectName, m.width), m.renderPRList(m.prState, w, m.spinner.View(), rows, 1, false), true
		}
	case ViewIssues:
		if m.issueState != nil {
			return renderContextHeader("Issues", m.projectName, m.width), m.renderIssueList(m.issueState, w, m.spinner.View(), rows, 1, false), true
		}
	}
	return "", "", false
}

// renderDockedForm draws the form docked into frame i, the frame beneath
// it: in the detail pane's place beside the list at split widths, else
// under the list. The form's key line replaces the frame's footer.
func (m Model) renderDockedForm(i int) string {
	below := ViewDashboard
	if i >= 0 {
		below = m.views[i].view
	}
	lay := branchFormLayout(m.width, m.height, m.formListWidth(below))
	panel := renderBranchForm(m.branchForm, lay, m.spinner.View())
	margin := lipgloss.NewStyle().Padding(0, 1)
	if lay.Dock == dockFull {
		return clampLines(margin.Render(panel), m.height)
	}

	listW, listRows := m.width-2, m.height-1-lay.Rows
	if lay.Dock == dockRight {
		listW, listRows = m.formListWidth(below), m.height-1
	}
	header, list, ok := m.formListAt(below, listW, listRows)
	if !ok {
		// No list to keep: the form docks over the frame's bottom rows.
		return composeLayerAt(m.renderFrame(i), margin.Render(panel), m.width, m.height, 0, m.height-lay.Rows)
	}
	list = clampLines(lipgloss.NewStyle().Width(listW).Render(list), listRows)

	var body string
	if lay.Dock == dockRight {
		divider := renderVerticalDivider(listRows, Colors.SurfaceDim)
		body = lipgloss.JoinHorizontal(lipgloss.Top, list, divider, clampLines(panel, listRows))
	} else {
		body = list + "\n" + panel
	}
	return lipgloss.JoinVertical(lipgloss.Left, header, margin.Render(body))
}
