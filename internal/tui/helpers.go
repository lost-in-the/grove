package tui

import (
	"strings"

	lipgloss "charm.land/lipgloss/v2"
)

func filterItems(items []WorktreeItem, query string) []WorktreeItem {
	query = strings.ToLower(query)
	var result []WorktreeItem
	for _, item := range items {
		if strings.Contains(strings.ToLower(item.ShortName), query) ||
			strings.Contains(strings.ToLower(item.Branch), query) {
			result = append(result, item)
		}
	}
	return result
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

// isPrintableText returns true if s is non-empty and contains only printable
// characters (no control codes). Used to filter tea.KeyPressMsg.Text so that
// control key combinations don't leak into input buffers.
func isPrintableText(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if r < 0x20 || r == 0x7f {
			return false
		}
	}
	return true
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

func padRight(s string, n int) string {
	w := lipgloss.Width(s)
	if w >= n {
		return s
	}
	return s + strings.Repeat(" ", n-w)
}
