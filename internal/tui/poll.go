package tui

import (
	"slices"
	"time"

	"charm.land/bubbles/v2/list"
	tea "charm.land/bubbletea/v2"

	"github.com/lost-in-the/grove/internal/mux"
	"github.com/lost-in-the/grove/internal/tuilog"
)

// agentPollInterval is how often the dashboard asks herdr for agent state
// (spec 7.4).
const agentPollInterval = 3 * time.Second

// agentPollTickMsg fires every agentPollInterval while the poll runs.
type agentPollTickMsg struct{}

// agentPollMsg carries one poll's sessions. gen is the poll generation it
// was started under; a fetch or a newer poll makes it stale.
type agentPollMsg struct {
	gen      int
	sessions []mux.Session
	backend  string
	err      error
}

func agentPollTick() tea.Cmd {
	return tea.Tick(agentPollInterval, func(time.Time) tea.Msg { return agentPollTickMsg{} })
}

// pollAgentsCmd lists the multiplexer's sessions. The caller resolves mx on
// the update goroutine: the command runs on another one and must not read the
// model or its config.
func pollAgentsCmd(mx mux.Multiplexer, gen int) tea.Cmd {
	return func() tea.Msg {
		sessions, err := mx.List()
		return agentPollMsg{gen: gen, sessions: sessions, backend: string(mx.Backend()), err: err}
	}
}

// handleAgentPollTick re-arms the tick and starts a poll unless one is
// still running.
func (m Model) handleAgentPollTick() (tea.Model, tea.Cmd) {
	if !m.agentAware || !m.polling {
		return m, nil
	}
	if m.pollInFlight {
		return m, agentPollTick()
	}
	m.pollGen++
	m.pollInFlight = true
	return m, tea.Batch(agentPollTick(), pollAgentsCmd(muxFor(m.cfg), m.pollGen))
}

// handleAgentPoll applies a poll's sessions to the items. Only the session
// fields change. A poll that changes nothing touches nothing, so the
// detail pane keeps its scroll.
func (m Model) handleAgentPoll(msg agentPollMsg) (tea.Model, tea.Cmd) {
	m.pollInFlight = false
	if msg.gen != m.pollGen {
		return m, nil
	}
	if msg.err != nil {
		tuilog.Printf("warning: agent poll failed: %v", msg.err)
		return m, nil
	}

	ix := mux.NewIndex(msg.sessions)
	sel, hasSel := m.selectedItem()
	items := slices.Clone(m.list.Items())
	var changed []int
	selChanged := false
	for i, li := range items {
		item, ok := li.(WorktreeItem)
		if !ok {
			continue
		}
		before := item
		applySession(&item, ix, m.projectName, msg.backend)
		if item.TmuxStatus == before.TmuxStatus && item.SessionBackend == before.SessionBackend && item.AgentStatus == before.AgentStatus {
			continue
		}
		items[i] = item
		changed = append(changed, i)
		if hasSel && item.Path == sel.Path {
			selChanged = true
		}
	}

	var cmd tea.Cmd
	if sorted, ok := m.attentionResort(items); ok {
		// Re-sort, refilter in place so the rows never blank, and keep the
		// cursor on the same worktree.
		m.list.SetItems(sorted)
		if m.list.FilterState() == list.FilterApplied {
			m.list.SetFilterText(m.list.FilterValue())
		}
		if hasSel {
			if i := indexOfPath(m.list.VisibleItems(), sel.Path); i >= 0 {
				m.list.Select(i)
			}
		}
	} else {
		if len(changed) == 0 {
			return m, nil
		}
		for _, i := range changed {
			cmd = m.list.SetItem(i, items[i])
		}
	}
	if selChanged {
		m.refreshDetailKeepScroll()
	}
	return m, cmd
}

// attentionResort returns items in attention order when that sort is on and
// the order is wrong. Not while the filter is being typed: rows do not move
// under the cursor mid-filter, and the next poll catches up.
func (m Model) attentionResort(items []list.Item) ([]list.Item, bool) {
	if m.sortMode != SortByAttention || m.list.FilterState() == list.Filtering {
		return nil, false
	}
	sorted := sortWorktreeItems(items, m.sortMode, m.sortReversed)
	for i := range items {
		a, _ := items[i].(WorktreeItem)
		b, _ := sorted[i].(WorktreeItem)
		if a.Path != b.Path {
			return sorted, true
		}
	}
	return nil, false
}

// refreshDetailKeepScroll re-renders the selected worktree's detail without
// moving the pane's scroll.
func (m *Model) refreshDetailKeepScroll() {
	off := m.detail.YOffset()
	m.updateDetailContent()
	m.detail.SetYOffset(off)
}
