package tui

import (
	"errors"
	"reflect"
	"strings"
	"testing"

	"charm.land/bubbles/v2/list"

	"github.com/lost-in-the/grove/internal/mux"
)

// sessionsFor reports a herdr session for every item as it stands, matched
// by path, so a poll built from it changes nothing until a test edits it.
func sessionsFor(m Model) []mux.Session {
	var out []mux.Session
	for _, li := range m.list.Items() {
		item := li.(WorktreeItem)
		out = append(out, mux.Session{
			Name:   "test-project-" + item.ShortName,
			Path:   item.Path,
			Status: mux.Status(item.TmuxStatus),
			Agent:  item.AgentStatus,
		})
	}
	return out
}

func withAgent(sessions []mux.Session, alias string, a mux.AgentStatus) []mux.Session {
	for i := range sessions {
		if sessions[i].Name == "test-project-"+alias {
			sessions[i].Agent = a
		}
	}
	return sessions
}

// inFlight puts the model mid-poll, as a tick would.
func inFlight(m Model) Model {
	m.polling, m.pollInFlight = true, true
	m.pollGen++
	return m
}

func itemNamed(m Model, alias string) WorktreeItem {
	for _, li := range m.list.Items() {
		if item := li.(WorktreeItem); item.ShortName == alias {
			return item
		}
	}
	return WorktreeItem{}
}

func TestPoll_StartsOnlyWhenAgentAware(t *testing.T) {
	items := makeTestItems(3)
	m := newTestModel(withSize(100, 24))
	if m = sendMsg(m, worktreesFetchedMsg{items: items, backend: "herdr"}); !m.polling {
		t.Error("the first herdr fetch should start the poll")
	}
	m = newTestModel(withSize(100, 24))
	if m = sendMsg(m, worktreesFetchedMsg{items: items, backend: "tmux"}); m.polling {
		t.Error("a tmux fetch must not start the poll")
	}
	// A tick under tmux (none should arrive) does not poll or re-arm.
	result, cmd := m.Update(agentPollTickMsg{})
	if m = result.(Model); cmd != nil || m.pollInFlight {
		t.Error("a tick under tmux should do nothing")
	}
}

func TestPoll_TickStartsOnePoll(t *testing.T) {
	m := newTestModel(withAgents(mux.AgentUnreported, mux.AgentWorking), withSize(100, 24))
	m.polling = true
	result, cmd := m.Update(agentPollTickMsg{})
	m = result.(Model)
	if cmd == nil || !m.pollInFlight || m.pollGen != 1 {
		t.Fatalf("first tick: cmd %v, in flight %v, gen %d; want a poll at gen 1", cmd != nil, m.pollInFlight, m.pollGen)
	}
	result, cmd = m.Update(agentPollTickMsg{})
	m = result.(Model)
	if cmd == nil || m.pollGen != 1 {
		t.Errorf("tick during a poll: cmd %v, gen %d; want only the re-arm at gen 1", cmd != nil, m.pollGen)
	}
}

func TestPoll_UpdatesAgentStatus(t *testing.T) {
	none := mux.AgentUnreported
	m := newTestModel(withAgents(none, mux.AgentWorking, none, none), withSize(100, 24))
	before := itemNamed(m, "fix-bug")
	m = inFlight(m)
	m = sendMsg(m, agentPollMsg{gen: m.pollGen, sessions: withAgent(sessionsFor(m), "fix-bug", mux.AgentBlocked), backend: "herdr"})

	after := itemNamed(m, "fix-bug")
	if after.AgentStatus != mux.AgentBlocked {
		t.Fatalf("fix-bug agent %q, want blocked", after.AgentStatus)
	}
	after.AgentStatus = before.AgentStatus
	if !reflect.DeepEqual(after, before) {
		t.Errorf("the poll changed more than the session fields:\nbefore %+v\nafter  %+v", before, after)
	}
	if m.pollInFlight {
		t.Error("the result should clear in-flight")
	}
	if h := m.buildDashboardHeader(); h.Blocked != 1 {
		t.Errorf("header blocked %d, want 1", h.Blocked)
	}
}

// A result from an older poll, or one started before a fetch, is dropped.
func TestPoll_DropsStaleGeneration(t *testing.T) {
	none := mux.AgentUnreported
	m := newTestModel(withAgents(none, none, none), withSize(100, 24))
	m = inFlight(m)
	stale := m.pollGen
	m = sendMsg(m, worktreesFetchedMsg{items: itemsOf(m), backend: "herdr"})
	m = sendMsg(m, agentPollMsg{gen: stale, sessions: withAgent(sessionsFor(m), "fix-bug", mux.AgentBlocked), backend: "herdr"})
	if itemNamed(m, "fix-bug").AgentStatus == mux.AgentBlocked {
		t.Error("a poll started before the fetch overwrote it")
	}
	if m.pollInFlight {
		t.Error("the dropped result should still clear in-flight")
	}
}

func TestPoll_FailureKeepsState(t *testing.T) {
	m := newTestModel(withAgents(mux.AgentUnreported, mux.AgentBlocked), withSize(100, 24))
	m = inFlight(m)
	m = sendMsg(m, agentPollMsg{gen: m.pollGen, err: errors.New("herdr gone")})
	if itemNamed(m, "feature-auth").AgentStatus != mux.AgentBlocked || itemNamed(m, "feature-auth").TmuxStatus != string(mux.StatusOpen) {
		t.Errorf("a failed poll changed state: %+v", itemNamed(m, "feature-auth"))
	}
	if m.toast.Current != nil || m.pollInFlight {
		t.Errorf("a failed poll: toast %v, in flight %v; want neither", m.toast.Current, m.pollInFlight)
	}
}

// Under attention sort a poll re-sorts the rows and the cursor follows the
// worktree, with or without a filter.
func TestPoll_AttentionSortKeepsCursorOnPath(t *testing.T) {
	none := mux.AgentUnreported
	m := newTestModel(withAgents(none, none, none, none, none), withSize(100, 24))
	m.sortMode = SortByAttention
	m.applySortToList()
	m.moveCursorTo(indexOfPath(m.list.VisibleItems(), itemNamed(m, "fix-bug").Path))

	m = inFlight(m)
	m = sendMsg(m, agentPollMsg{gen: m.pollGen, sessions: withAgent(sessionsFor(m), "refactor", mux.AgentBlocked), backend: "herdr"})
	if got := strings.Join(names(m.list.Items()), " "); got != "root refactor feature-auth fix-bug testing" {
		t.Errorf("rows after the poll: %s", got)
	}
	if selectedName(m) != "fix-bug" {
		t.Errorf("cursor on %q, want fix-bug", selectedName(m))
	}

	// With a filter applied the rows still re-sort and nothing blanks.
	m.list.SetFilterText("f")
	m.moveCursorTo(indexOfPath(m.list.VisibleItems(), itemNamed(m, "fix-bug").Path))
	m = inFlight(m)
	m = sendMsg(m, agentPollMsg{gen: m.pollGen, sessions: withAgent(sessionsFor(m), "fix-bug", mux.AgentBlocked), backend: "herdr"})
	if m.list.FilterState() != list.FilterApplied || len(m.list.VisibleItems()) == 0 {
		t.Fatalf("filter %v with %d rows after the poll", m.list.FilterState(), len(m.list.VisibleItems()))
	}
	if first := m.list.VisibleItems()[0].(WorktreeItem); first.ShortName != "fix-bug" || selectedName(m) != "fix-bug" {
		t.Errorf("filtered: first row %q, cursor %q; want fix-bug for both", first.ShortName, selectedName(m))
	}
}

// A poll that changes nothing leaves the detail pane where the user
// scrolled it.
func TestPoll_UnchangedResultLeavesPaneAlone(t *testing.T) {
	m := newTestModel(withAgents(mux.AgentUnreported, mux.AgentBlocked, mux.AgentDone), withSize(140, 30))
	m.detail.SetContent(strings.Repeat("line\n", 100))
	m.detail.SetYOffset(7)
	m = inFlight(m)
	m = sendMsg(m, agentPollMsg{gen: m.pollGen, sessions: sessionsFor(m), backend: "herdr"})
	if got := m.detail.YOffset(); got != 7 {
		t.Errorf("detail scrolled to %d after an unchanged poll, want 7", got)
	}
}

func itemsOf(m Model) []WorktreeItem {
	var out []WorktreeItem
	for _, li := range m.list.Items() {
		out = append(out, li.(WorktreeItem))
	}
	return out
}
