package tui

import "strings"

// checkDuplicateWorktree checks if a worktree name already exists in the list.
// Returns the matching WorktreeItem if found, nil otherwise.
func checkDuplicateWorktree(name string, items []WorktreeItem) *WorktreeItem {
	if name == "" {
		return nil
	}
	lower := strings.ToLower(name)
	for i := range items {
		if strings.ToLower(items[i].ShortName) == lower {
			return &items[i]
		}
	}
	return nil
}
