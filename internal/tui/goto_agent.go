package tui

import (
	"charm.land/bubbles/v2/list"
	tea "charm.land/bubbletea/v2"

	"github.com/lost-in-the/grove/internal/mux"
)

// gotoAgent moves the cursor to the next blocked agent below it, wrapping;
// with none blocked, to the next done one (spec 7.3). A target the filter
// hides clears the filter first.
func (m Model) gotoAgent() (tea.Model, tea.Cmd) {
	items := m.list.Items()
	cur := -1
	if sel, ok := m.selectedItem(); ok {
		cur = indexOfPath(items, sel.Path)
	}
	target := -1
	for _, want := range []mux.AgentStatus{mux.AgentBlocked, mux.AgentDone} {
		if target = nextWithAgent(items, cur, want); target >= 0 {
			break
		}
	}
	if target < 0 {
		m.toast.Show(NewToast("no agents need you", ToastInfo))
		return m, nil
	}

	path := items[target].(WorktreeItem).Path
	if m.list.FilterState() != list.Unfiltered && indexOfPath(m.list.VisibleItems(), path) < 0 {
		m.list.ResetFilter()
	}
	m.moveCursorTo(indexOfPath(m.list.VisibleItems(), path))
	return m, nil
}

// nextWithAgent is the index of the first item after cur, wrapping, whose
// agent is in state want, or -1.
func nextWithAgent(items []list.Item, cur int, want mux.AgentStatus) int {
	n := len(items)
	for step := 1; step <= n; step++ {
		i := (cur + step + n) % n
		if item, ok := items[i].(WorktreeItem); ok && item.AgentStatus == want {
			return i
		}
	}
	return -1
}

func indexOfPath(items []list.Item, path string) int {
	for i, li := range items {
		if item, ok := li.(WorktreeItem); ok && item.Path == path {
			return i
		}
	}
	return -1
}
