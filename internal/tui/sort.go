package tui

import (
	"sort"
	"strings"

	"charm.land/bubbles/v2/list"

	"github.com/lost-in-the/grove/internal/mux"
)

// SortMode defines how worktrees are ordered in the list.
type SortMode int

const (
	SortByName SortMode = iota
	SortByAge           // newest last commit first
	SortByDirtyFirst
	SortByAttention // agents that need you first; only under herdr
)

// String returns a display label for the sort mode.
func (s SortMode) String() string {
	switch s {
	case SortByName:
		return "name"
	case SortByAge:
		return "age"
	case SortByDirtyFirst:
		return "dirty"
	case SortByAttention:
		return "attention"
	default:
		return "name"
	}
}

// Next cycles to the next sort mode. Attention is in the cycle only when
// agent state is known (spec 5.7).
func (s SortMode) Next(agentAware bool) SortMode {
	last := SortByDirtyFirst
	if agentAware {
		last = SortByAttention
	}
	if s >= last {
		return SortByName
	}
	return s + 1
}

// sortWorktreeItems sorts a slice of list.Item (which must be WorktreeItem)
// according to the given mode. Returns a new sorted slice.
func sortWorktreeItems(items []list.Item, mode SortMode, reversed bool) []list.Item {
	sorted := make([]list.Item, len(items))
	copy(sorted, items)

	sort.SliceStable(sorted, func(i, j int) bool {
		a, aOk := sorted[i].(WorktreeItem)
		b, bOk := sorted[j].(WorktreeItem)
		if !aOk || !bOk {
			return false
		}
		if a.IsMain != b.IsMain {
			return a.IsMain
		}
		if reversed {
			return compareWorktrees(b, a, mode)
		}
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
	case SortByAge:
		if !a.CommitTime.Equal(b.CommitTime) {
			return a.CommitTime.After(b.CommitTime)
		}
		return strings.ToLower(a.ShortName) < strings.ToLower(b.ShortName)

	case SortByDirtyFirst:
		if a.IsDirty != b.IsDirty {
			return a.IsDirty
		}
		return strings.ToLower(a.ShortName) < strings.ToLower(b.ShortName)

	case SortByAttention:
		if ra, rb := attentionRank(a.AgentStatus), attentionRank(b.AgentStatus); ra != rb {
			return ra < rb
		}
		return strings.ToLower(a.ShortName) < strings.ToLower(b.ShortName)

	default: // SortByName
		return strings.ToLower(a.ShortName) < strings.ToLower(b.ShortName)
	}
}

// attentionRank orders agent states by how much they need you: blocked,
// done, working, then everything else.
func attentionRank(s mux.AgentStatus) int {
	switch s {
	case mux.AgentBlocked:
		return 0
	case mux.AgentDone:
		return 1
	case mux.AgentWorking:
		return 2
	}
	return 3
}
