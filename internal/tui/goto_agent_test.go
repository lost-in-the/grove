package tui

import (
	"strings"
	"testing"

	"charm.land/bubbles/v2/list"
	"charm.land/bubbles/v2/spinner"

	"github.com/lost-in-the/grove/internal/mux"
)

// withAgents seeds herdr items with the given agent states, in
// makeTestItems order: root, feature-auth, fix-bug, testing, refactor.
func withAgents(statuses ...mux.AgentStatus) testOpt {
	return func(m *Model) {
		items := makeTestItems(len(statuses))
		li := make([]list.Item, len(items))
		for i := range items {
			items[i].SessionBackend = "herdr"
			items[i].TmuxStatus = string(mux.StatusOpen)
			items[i].AgentStatus = statuses[i]
			li[i] = items[i]
		}
		m.list.SetItems(li)
		m.agentAware = true
		m.refreshDelegate()
	}
}

func selectedName(m Model) string {
	item, _ := m.selectedItem()
	return item.ShortName
}

func pressGA(m Model) Model {
	return sendKey(sendKey(m, "g"), "a")
}

// g a moves to the next blocked row below the cursor, wrapping (spec 7.3).
func TestGA_JumpsToNextBlockedWrapping(t *testing.T) {
	none, blocked := mux.AgentUnreported, mux.AgentBlocked
	m := newTestModel(withAgents(none, blocked, none, blocked, none), withSize(100, 24))
	for _, want := range []string{"feature-auth", "testing", "feature-auth"} {
		m = pressGA(m)
		if got := selectedName(m); got != want {
			t.Fatalf("g a selected %q, want %q", got, want)
		}
	}
}

func TestGA_FallsBackToDone(t *testing.T) {
	none, done := mux.AgentUnreported, mux.AgentDone
	m := newTestModel(withAgents(none, mux.AgentWorking, done, none), withSize(100, 24))
	if m = pressGA(m); selectedName(m) != "fix-bug" {
		t.Errorf("with no blocked rows g a selected %q, want the done row fix-bug", selectedName(m))
	}

	// Blocked wins over a nearer done row.
	m = newTestModel(withAgents(none, done, none, mux.AgentBlocked), withSize(100, 24))
	if m = pressGA(m); selectedName(m) != "testing" {
		t.Errorf("g a selected %q, want the blocked row testing", selectedName(m))
	}
}

func TestGA_NoneFlashes(t *testing.T) {
	m := newTestModel(withAgents(mux.AgentUnreported, mux.AgentWorking, mux.AgentIdle), withSize(100, 24))
	m.list.Select(1)
	m = pressGA(m)
	if selectedName(m) != "feature-auth" {
		t.Errorf("cursor moved to %q with nothing needing you", selectedName(m))
	}
	if m.toast.Current == nil || m.toast.Message() != "no agents need you" || m.toast.Current.Level != ToastInfo {
		t.Errorf("want an info flash \"no agents need you\", got %+v", m.toast.Current)
	}
}

// A target hidden by the filter clears the filter; a visible one keeps it.
func TestGA_ClearsFilterWhenTargetHidden(t *testing.T) {
	none := mux.AgentUnreported
	m := newTestModel(withAgents(none, mux.AgentBlocked, none, none, none), withSize(100, 24))
	m.list.SetFilterText("feat")
	m = pressGA(m)
	if m.list.FilterState() != list.FilterApplied || selectedName(m) != "feature-auth" {
		t.Errorf("visible target: filter %v, selected %q; want the filter kept and feature-auth", m.list.FilterState(), selectedName(m))
	}

	m.list.SetFilterText("refactor")
	m = pressGA(m)
	if m.list.FilterState() != list.Unfiltered {
		t.Errorf("hidden target: filter still %v", m.list.FilterState())
	}
	if selectedName(m) != "feature-auth" {
		t.Errorf("hidden target: selected %q, want feature-auth", selectedName(m))
	}
}

func TestGA_AbsentUnderTmux(t *testing.T) {
	m := newTestModel(withAgents(mux.AgentUnreported, mux.AgentBlocked, mux.AgentUnreported), withSize(100, 24))
	if strip := stripAnsi(strings.Join(sendKey(m, "g").gStripLines(), "\n")); !strings.Contains(strip, "a agent needing you") {
		t.Errorf("herdr g strip should list a: %q", strip)
	}

	m.agentAware = false
	if strip := stripAnsi(strings.Join(sendKey(m, "g").gStripLines(), "\n")); strings.Contains(strip, "agent") {
		t.Errorf("tmux g strip should not list a: %q", strip)
	}
	m = pressGA(m)
	if selectedName(m) != "root" || m.toast.Current != nil || m.activeView() != ViewDashboard || m.pendingG {
		t.Errorf("g a under tmux: selected %q, toast %+v, view %v, pending g %v; want nothing to happen",
			selectedName(m), m.toast.Current, m.activeView(), m.pendingG)
	}
}

// The flash must start the spinner loop: toasts expire on its ticks, and an
// idle dashboard has no loop running, so without one the flash never clears.
func TestGA_NoneFlashStartsTheTicksThatClearIt(t *testing.T) {
	m := newTestModel(withAgents(mux.AgentUnreported, mux.AgentWorking), withSize(100, 24))
	m = sendKey(m, "g")
	_, cmd := m.Update(makeKeyMsg("a"))
	if cmd == nil {
		t.Fatal("g a with nothing needing you returned no command; the toast would never expire")
	}
	if _, ok := cmd().(spinner.TickMsg); !ok {
		t.Error("g a should start the spinner tick that expires toasts")
	}
}
