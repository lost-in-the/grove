package tui

import "strings"

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

func padRight(s string, n int) string {
	if len(s) >= n {
		return s
	}
	return s + strings.Repeat(" ", n-len(s))
}
