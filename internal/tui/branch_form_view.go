package tui

import (
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/lost-in-the/grove/internal/worktree"
)

const (
	formTop         = 2
	formMinWidth    = 56
	formMaxWidth    = 76
	formHeight      = 14
	formSuggestRows = 4
	formLabelWidth  = 9
)

// branchFormSize is the frame's width and height for a terminal size. It
// depends only on the terminal, never on the form's state, so the frame
// cannot change size while the user works in it.
func branchFormSize(termW, termH int) (int, int) {
	w := min(max(termW*60/100, formMinWidth), formMaxWidth)
	w = min(w, termW-2)
	h := min(formHeight, termH-formTop-1)
	return max(w, 24), max(h, 6)
}

func formLogWidth(termW int) int {
	w, _ := branchFormSize(termW, formHeight+formTop+1)
	return w - 4
}

// renderBranchForm draws the form at exactly branchFormSize. The title,
// status line, and key line are pinned; the body scrolls between them to
// keep the focused field visible when the terminal is short.
func renderBranchForm(s *BranchFormState, termW, termH int, spinner string) string {
	w, h := branchFormSize(termW, termH)
	inner := w - 4
	bodyRows := h - 5

	body, focusLine := s.bodyLines(inner, spinner)
	body = windowLines(body, bodyRows, focusLine)

	msg, kind := s.status()
	lines := []string{Styles.OverlayTitle.Render(s.title())}
	lines = append(lines, body...)
	lines = append(lines, formStatusStyle(kind).Render(msg), Styles.Footer.Render(s.keyLine()))
	for i, l := range lines {
		lines[i] = " " + padCells(l, inner) + " "
	}
	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(Colors.Success).
		Render(strings.Join(lines, "\n"))
}

func formStatusStyle(kind formStatusKind) lipgloss.Style {
	switch kind {
	case formStatusError:
		return Styles.ErrorText
	case formStatusReady:
		return Styles.SuccessText
	case formStatusInfo:
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

func (s *BranchFormState) label(name string, f formField) string {
	marker := "  "
	if s.Focus == f {
		marker = Styles.ListCursor.Render("❯ ")
	}
	return marker + Styles.DetailLabel.Render(fmt.Sprintf("%-7s", name))
}

// bodyLines returns the body and the index of the focused field's line.
func (s *BranchFormState) bodyLines(inner int, spinner string) ([]string, int) {
	if s.Busy && s.Log != nil {
		return strings.Split(s.Log.View(spinner), "\n"), 0
	}
	indent := strings.Repeat(" ", formLabelWidth)
	lines := []string{s.label("Branch", fieldBranch) + s.Branch.View()}
	lines = append(lines, s.suggestionLines(indent, inner)...)
	if s.isNewBranch() {
		lines = append(lines, s.label("Base", fieldBase)+s.Base.View())
	} else {
		lines = append(lines, s.label("Base", fieldBase)+Styles.DetailDim.Render("n/a · existing branch"))
	}
	lines = append(lines, s.label("Alias", fieldAlias)+s.Alias.View())
	preview := ""
	if a := s.aliasValue(); a != "" {
		preview = fmt.Sprintf("→ %s · session %s",
			worktree.InterpolateNamePattern(s.opts.NamePattern, s.opts.ProjectName, a),
			worktree.TmuxSessionName(s.opts.ProjectName, a))
	}
	lines = append(lines, indent+Styles.DetailDim.Render(preview), "")

	focus := 0
	switch s.Focus {
	case fieldBase:
		focus = 1 + formSuggestRows
	case fieldAlias:
		focus = 2 + formSuggestRows
	}
	return lines, focus
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
			cursor = Styles.ListCursor.Render("❯ ")
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
