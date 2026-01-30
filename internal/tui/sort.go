package tui

import (
	"sort"
	"strings"

	"github.com/charmbracelet/bubbles/list"
)

// SortMode defines how worktrees are ordered in the list.
type SortMode int

const (
	SortByName SortMode = iota
	SortByLastAccessed
	SortByDirtyFirst
	sortModeCount // sentinel for cycling
)

// String returns a display label for the sort mode.
func (s SortMode) String() string {
	switch s {
	case SortByName:
		return "name"
	case SortByLastAccessed:
		return "recent"
	case SortByDirtyFirst:
		return "dirty"
	default:
		return "name"
	}
}

// Next cycles to the next sort mode.
func (s SortMode) Next() SortMode {
	return (s + 1) % sortModeCount
}

// sortWorktreeItems sorts a slice of list.Item (which must be WorktreeItem)
// according to the given mode. Returns a new sorted slice.
func sortWorktreeItems(items []list.Item, mode SortMode) []list.Item {
	sorted := make([]list.Item, len(items))
	copy(sorted, items)

	sort.SliceStable(sorted, func(i, j int) bool {
		a := sorted[i].(WorktreeItem)
		b := sorted[j].(WorktreeItem)
		return compareWorktrees(a, b, mode)
	})

	return sorted
}

// compareWorktrees returns true if a should sort before b.
func compareWorktrees(a, b WorktreeItem, mode SortMode) bool {
	// Main worktree always sorts first
	if a.IsMain != b.IsMain {
		return a.IsMain
	}

	switch mode {
	case SortByLastAccessed:
		if !a.LastAccessed.Equal(b.LastAccessed) {
			return a.LastAccessed.After(b.LastAccessed)
		}
		return strings.ToLower(a.ShortName) < strings.ToLower(b.ShortName)

	case SortByDirtyFirst:
		if a.IsDirty != b.IsDirty {
			return a.IsDirty
		}
		return strings.ToLower(a.ShortName) < strings.ToLower(b.ShortName)

	default: // SortByName
		return strings.ToLower(a.ShortName) < strings.ToLower(b.ShortName)
	}
}
