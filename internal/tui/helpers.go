package tui

import (
	"fmt"
	"strings"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	lipgloss "charm.land/lipgloss/v2"

	"github.com/lost-in-the/grove/internal/worktree"
)

// renderFilterBar writes the filter/count bar used by list views (Issues, PRs).
// When filtering is active or a filter is set, it shows the filter input view
// with a "N of M" count. Otherwise it shows "N open" or an empty line.
func renderFilterBar(b *strings.Builder, filterView string, filtering bool, filter string, filteredCount, totalCount int) {
	if filtering || filter != "" {
		b.WriteString(filterView)
		if filter != "" {
			fmt.Fprintf(b, "  %s", Styles.DetailDim.Render(fmt.Sprintf("%d of %d", filteredCount, totalCount)))
		}
		b.WriteString("\n\n")
	} else {
		if totalCount > 0 {
			b.WriteString(Styles.DetailDim.Render(fmt.Sprintf("%d open", totalCount)) + "\n\n")
		} else {
			b.WriteString("\n\n")
		}
	}
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

// ValidateWorktreeName checks if a name is valid for a worktree.
func ValidateWorktreeName(name string) string {
	if name == "" {
		return ""
	}
	if strings.ContainsAny(name, " /\\:*?\"<>|") {
		return "name contains invalid characters"
	}
	if strings.HasPrefix(name, "-") || strings.HasPrefix(name, ".") {
		return "name cannot start with - or ."
	}
	return ""
}

// exactBranchMatch returns true if any branch exactly matches the given name.
func exactBranchMatch(branches []string, name string) bool {
	for _, b := range branches {
		if b == name {
			return true
		}
	}
	return false
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
// Used by both the checkout and fork overlays.
type wipCheckMsg struct {
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
			return wipCheckMsg{err: err}
		}
		var files []string
		if hasWIP {
			files, err = wip.ListWIPFiles()
			if err != nil {
				return wipCheckMsg{hasWIP: hasWIP, err: fmt.Errorf("failed to list WIP files: %w", err)}
			}
		}
		return wipCheckMsg{hasWIP: hasWIP, files: files}
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
// Used by both renderPRList and renderIssueList.
func calcDisplaySlots(headerLines, maxHeight, width int) (slots, contentWidth int) {
	availableLines := maxHeight - headerLines
	if availableLines < 3 {
		availableLines = 3
	}
	slots = (availableLines + 1) / 3
	if slots < 3 {
		slots = 3
	}
	contentWidth = width - 2
	if contentWidth < 40 {
		contentWidth = 40
	}
	return slots, contentWidth
}

// newFilterInput creates a configured textinput for list filtering.
// All list filter inputs share the same prompt and char limit; only placeholder varies.
func newFilterInput(placeholder string) textinput.Model {
	ti := textinput.New()
	ti.Prompt = "Filter: "
	ti.Placeholder = placeholder
	ti.CharLimit = 100
	return ti
}

// handleListFilterKey handles key routing when a list view's filter input is
// active. Returns the updated textinput, cursor, tea.Cmd, and whether filtering
// should stop (true when Escape or Enter is pressed).
func handleListFilterKey(msg tea.KeyPressMsg, keys KeyMap, ti textinput.Model, cursor int) (textinput.Model, int, tea.Cmd, bool) {
	switch {
	case key.Matches(msg, keys.Escape):
		ti.Blur()
		ti.SetValue("")
		return ti, 0, nil, true
	case key.Matches(msg, keys.Enter):
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
