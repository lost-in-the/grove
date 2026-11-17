package tui

import (
	"fmt"
	"strings"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	lipgloss "charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/lost-in-the/grove/internal/worktree"
)

// Shared string values used across overlays.
const (
	checkboxUnchecked      = "[ ]"
	checkboxChecked        = "[x]"
	boolTrue               = "true"
	boolFalse              = "false"
	reviewApproved         = "APPROVED"
	reviewChangesRequested = "CHANGES_REQUESTED"
)

// filterRow is a list's filter row: the input, then, once there is text,
// how many rows it shows, and, once applied, how to clear it. It is "" when
// no filter is open. Every list with a filter draws it as its first row.
func filterRow(input string, typing bool, filter string, shown, total int) string {
	if !typing && filter == "" {
		return ""
	}
	row := input
	if filter != "" {
		row += "  " + Styles.DetailDim.Render(fmt.Sprintf("%d of %d", shown, total))
		if !typing {
			row += Styles.HelpSep.Render(" · ") + Styles.HelpKey.Render("esc") + " " + Styles.HelpDesc.Render("clear")
		}
	}
	return row
}

// filterInputView is a filter's input as its row shows it: with the text
// cursor while you type, else the prompt and the text alone.
func filterInputView(in textinput.Model, typing bool) string {
	if !typing {
		return in.Prompt + in.Value()
	}
	in.SetWidth(0) // the dashboard list pads it to its width; the count follows the text
	return in.View()
}

// renderFilterBar writes the filter/count bar used by list views (Issues, PRs).
// When filtering is active or a filter is set, it shows the filter row.
// Otherwise it shows "N open" or an empty line. The second, normally blank
// row holds the move notice without shifting the items or hiding the filter.
func (m Model) renderFilterBar(b *strings.Builder, in textinput.Model, filtering bool, filter string, filteredCount, totalCount, width int) {
	if row := filterRow(filterInputView(in, filtering), filtering, filter, filteredCount, totalCount); row != "" {
		b.WriteString(ansi.Truncate(row, width, "…") + "\n")
	} else {
		if totalCount > 0 {
			b.WriteString(Styles.DetailDim.Render(fmt.Sprintf("%d open", totalCount)) + "\n")
		} else {
			b.WriteString("\n")
		}
	}
	if m.cannotMove {
		b.WriteString(ansi.Truncate(moveNotice(), width, "…"))
	}
	b.WriteString("\n")
}

func truncate(s string, max int) string {
	if max <= 0 {
		return ""
	}
	runes := []rune(s)
	if len(runes) <= max {
		return s
	}
	if max <= 1 {
		return string(runes[:max])
	}
	return string(runes[:max-1]) + "…"
}

func filteredBranches(branches []string, filter string) []string {
	if filter == "" {
		return branches
	}
	lower := strings.ToLower(filter)
	var result []string
	for _, br := range branches {
		if strings.Contains(strings.ToLower(br), lower) {
			result = append(result, br)
		}
	}
	return result
}

// ValidateWorktreeName checks if a name is valid for a worktree. Delegates to
// the canonical validator in internal/worktree so the TUI and CLI enforce
// identical rules.
func ValidateWorktreeName(name string) string {
	return worktree.ValidateWorktreeName(name)
}

// scrollWindow computes the visible start/end indices for a cursor-following
// scroll window. Given a total item count, the current cursor position, and the
// max number of items to display, it returns (start, end) such that the cursor
// is always visible within the window.
func scrollWindow(total, cursor, maxVisible int) (start, end int) {
	if total == 0 || maxVisible <= 0 {
		return 0, 0
	}
	if cursor >= maxVisible {
		start = cursor - maxVisible + 1
	}
	end = start + maxVisible
	if end > total {
		end = total
	}
	return start, end
}

// wipCheckMsg is sent after checking a worktree for uncommitted changes.
// Used by the branch form's Copy and Switch modes. The path field correlates
// the result back to the worktree that was checked, so a stale result from a
// canceled form cannot corrupt a later form opened for a different worktree.
type wipCheckMsg struct {
	path   string
	hasWIP bool
	files  []string
	err    error
}

// wipCheckCmd checks the given worktree path for uncommitted changes.
func wipCheckCmd(path string) tea.Cmd {
	return func() tea.Msg {
		wip := worktree.NewWIPHandler(path)
		hasWIP, err := wip.HasWIP()
		if err != nil {
			return wipCheckMsg{path: path, err: err}
		}
		var files []string
		if hasWIP {
			files, err = wip.ListWIPFiles()
			if err != nil {
				return wipCheckMsg{path: path, hasWIP: hasWIP, err: fmt.Errorf("failed to list WIP files: %w", err)}
			}
		}
		return wipCheckMsg{path: path, hasWIP: hasWIP, files: files}
	}
}

func padRight(s string, n int) string {
	w := lipgloss.Width(s)
	if w >= n {
		return s
	}
	return s + strings.Repeat(" ", n-w)
}

// calcDisplaySlots computes how many 3-line item slots fit in the remaining
// height after headerLines have been rendered, and a clamped content width.
// It shows at least minSlots items, even when they overflow maxHeight.
// Used by both renderPRList and renderIssueList.
func calcDisplaySlots(headerLines, maxHeight, width, minSlots int) (slots, contentWidth int) {
	availableLines := maxHeight - headerLines
	if availableLines < 3 {
		availableLines = 3
	}
	slots = max((availableLines+1)/3, minSlots)
	contentWidth = width - 2
	if contentWidth < 40 {
		contentWidth = 40
	}
	return slots, contentWidth
}

// newTextInput is every grove text field. ctrl+k moves in finders, so it
// never deletes to the end of the line; ctrl+u still deletes before the
// cursor.
func newTextInput() textinput.Model {
	ti := textinput.New()
	unbindDeleteAfterCursor(&ti)
	return ti
}

func unbindDeleteAfterCursor(ti *textinput.Model) {
	ti.KeyMap.DeleteAfterCursor.SetEnabled(false)
}

// newFilterInput creates a configured textinput for list filtering.
// All list filter inputs share the same prompt and char limit; only placeholder varies.
func newFilterInput(placeholder string) textinput.Model {
	ti := newTextInput()
	ti.Prompt = "Filter: "
	ti.Placeholder = placeholder
	ti.CharLimit = 100
	return ti
}

// handleListFilterKey handles key routing when a list view's filter input is
// active. Returns the updated textinput, cursor, tea.Cmd, and whether filtering
// should stop: esc clears the filter, enter and tab keep it. No key moves
// the rows while you type.
func handleListFilterKey(msg tea.KeyPressMsg, keys KeyMap, ti textinput.Model, cursor int) (textinput.Model, int, tea.Cmd, bool) {
	switch {
	case key.Matches(msg, keys.Escape):
		ti.Blur()
		ti.SetValue("")
		return ti, 0, nil, true
	case key.Matches(msg, keys.Enter), key.Matches(msg, keys.Tab):
		ti.Blur()
		return ti, cursor, nil, true
	default:
		prevVal := ti.Value()
		var cmd tea.Cmd
		ti, cmd = ti.Update(msg)
		if ti.Value() != prevVal {
			cursor = 0
		}
		return ti, cursor, cmd, false
	}
}

// ExistingWorktreePrompt holds state for the "worktree already exists" popup
// shown in PR and issue views when a user tries to create a duplicate.
type ExistingWorktreePrompt struct {
	WorktreeName string // short name to highlight on dashboard
	Branch       string // branch that matched
	ItemLabel    string // e.g. "PR #42" or "Issue #101"
}

// renderExistingWorktreePrompt renders a popup offering to go to the existing
// worktree or copy it to a new branch.
func renderExistingWorktreePrompt(p *ExistingWorktreePrompt, width int) string {
	d := calcOverlayDims(width)

	var b strings.Builder
	b.WriteString(d.indent + p.ItemLabel + " uses branch " + Styles.DetailValue.Render(p.Branch) + "\n")
	b.WriteString(d.indent + "which already has a worktree:\n\n")
	b.WriteString(d.indent + "  → " + Styles.SuccessText.Render(p.WorktreeName) + "\n")

	content := b.String()
	footer := "\n" + promptKeys(moveFooter(Hint{"esc", "cancel"}, Hint{"enter", "go to worktree"}, Hint{"n", "copy"}), d.inner, d.indent)

	return Styles.OverlayBorder.Width(d.overlay).Render(
		Styles.OverlayTitle.Render("Worktree Exists") + "\n\n" + padToHeight(content, 8) + footer,
	)
}

// handleExistsPromptKey handles key input for the "worktree exists" popup.
// Enter navigates to the existing worktree. N opens Copy. Esc dismisses.
func (m Model) handleExistsPromptKey(msg tea.KeyPressMsg, prompt *ExistingWorktreePrompt, dismiss func()) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(msg, m.keys.Escape):
		dismiss()
		return m, nil
	case key.Matches(msg, m.keys.Enter):
		m.popToRoot()
		m.prState = nil
		m.issueState = nil
		m.pendingSelect = prompt.WorktreeName
		m.toast.Show(NewToast(fmt.Sprintf("Focused worktree %q", prompt.WorktreeName), ToastInfo))
		return m, m.fetchWorktrees
	case msg.String() == "n" || key.Matches(msg, m.keys.New):
		// Copy the existing worktree, over the browser that raised the
		// prompt, so esc from the form returns there.
		item := m.findWorktreeByName(prompt.WorktreeName)
		dismiss()
		if item == nil {
			return m, nil
		}
		return m.openCopyForm(*item)
	}
	return m, nil
}

// findWorktreeByName returns the WorktreeItem with the given short name, or nil.
func (m Model) findWorktreeByName(name string) *WorktreeItem {
	for _, li := range m.list.Items() {
		if item, ok := li.(WorktreeItem); ok && item.ShortName == name {
			return &item
		}
	}
	return nil
}
