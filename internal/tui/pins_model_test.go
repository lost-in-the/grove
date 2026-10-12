package tui

import (
	"errors"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
)

// pinModel is a 120x40 dashboard with real prefs on disk and the given pins.
func pinModel(t *testing.T, pins pinSet) Model {
	t.Helper()
	m := newTestModel(withItems(4), withSize(120, 40))
	m.projectRoot = prefsRoot(t)
	m.pins = pins
	m.refreshDelegate()
	return m
}

func pressKey(m Model, k string) (Model, tea.Cmd) {
	next, cmd := m.Update(makeKeyMsg(k))
	return next.(Model), cmd
}

func isQuit(cmd tea.Cmd) bool {
	if cmd == nil {
		return false
	}
	_, ok := cmd().(tea.QuitMsg)
	return ok
}

func TestPins_MThenDigitPins(t *testing.T) {
	m := pinModel(t, pinSet{})
	m.list.Select(1)
	m = sendKey(m, "m")
	m = sendKey(m, "3")
	if m.pins["3"] != "feature-auth" {
		t.Errorf("pins = %v, want slot 3 = feature-auth", m.pins)
	}
	if saved := loadUIPrefs(m.projectRoot); saved == nil || saved.Pins["3"] != "feature-auth" {
		t.Errorf("saved prefs = %+v", saved)
	}
}

func TestPins_MShowsStrip(t *testing.T) {
	m := pinModel(t, pinSet{"1": "root"})
	m.list.Select(1)
	m = sendKey(m, "m")
	out := ansi.Strip(m.viewString())
	for _, want := range []string{"m pin to slot · feature-auth is selected", "1 root", "2 ·", "0 unpin", "esc close"} {
		if !strings.Contains(out, want) {
			t.Errorf("strip missing %q:\n%s", want, out)
		}
	}
	m = sendKey(m, "esc")
	if strings.Contains(ansi.Strip(m.viewString()), "pin to slot") || len(m.pins) != 1 {
		t.Errorf("esc should close the strip and change nothing (pins %v)", m.pins)
	}
}

func TestPins_StripRestoresListHeight(t *testing.T) {
	m := pinModel(t, pinSet{})
	h := m.list.Height()
	m = sendKey(m, "m")
	if m.list.Height() >= h {
		t.Errorf("list height %d with the strip open, want less than %d", m.list.Height(), h)
	}
	m = sendKey(m, "esc")
	if m.list.Height() != h {
		t.Errorf("list height %d after closing, want %d", m.list.Height(), h)
	}
}

func TestPins_MOtherKeyCloses(t *testing.T) {
	m := pinModel(t, pinSet{})
	m = sendKey(m, "m")
	m = sendKey(m, "x")
	if m.pendingM || len(m.pins) != 0 {
		t.Errorf("pendingM %v pins %v, want closed and unchanged", m.pendingM, m.pins)
	}
}

func TestPins_DigitSwitches(t *testing.T) {
	m := pinModel(t, pinSet{"2": "fix-bug", "1": "root"})
	next, cmd := pressKey(m, "2")
	if next.switchTo != "/tmp/test-project-fix-bug" || !isQuit(cmd) {
		t.Errorf("2: switchTo %q quit %v", next.switchTo, isQuit(cmd))
	}
	next, cmd = pressKey(m, "1")
	if next.switchTo != "" || !isQuit(cmd) {
		t.Errorf("1 on the current worktree: switchTo %q quit %v, want quit without a switch", next.switchTo, isQuit(cmd))
	}
}

func TestPins_EmptySlotFlashes(t *testing.T) {
	m := pinModel(t, pinSet{})
	next, cmd := pressKey(m, "4")
	if isQuit(cmd) || next.switchTo != "" {
		t.Error("an empty slot must not switch")
	}
	if got := next.toast.Message(); got != "slot 4 is empty · m 4 to pin" {
		t.Errorf("toast = %q", got)
	}
}

func TestPins_NoPositionalQuickSwitch(t *testing.T) {
	m := pinModel(t, pinSet{})
	if next, cmd := pressKey(m, "2"); next.switchTo != "" || isQuit(cmd) {
		t.Error("2 with no pins switched by row position")
	}
}

func TestPins_DigitWithFilterApplied(t *testing.T) {
	m := pinModel(t, pinSet{"2": "fix-bug"})
	m.list.SetFilterText("test")
	next, cmd := pressKey(m, "2")
	if next.switchTo != "/tmp/test-project-fix-bug" || !isQuit(cmd) {
		t.Errorf("with a filter applied: switchTo %q", next.switchTo)
	}
}

func TestPins_DigitWhileTypingFilterTypes(t *testing.T) {
	m := pinModel(t, pinSet{"2": "fix-bug"})
	m = sendKey(m, "/")
	next, _ := pressKey(m, "2")
	if next.switchTo != "" || next.list.FilterValue() != "2" {
		t.Errorf("typing 2 in the filter: switchTo %q filter %q", next.switchTo, next.list.FilterValue())
	}
}

func TestPins_RenameMovesPin(t *testing.T) {
	m := pinModel(t, pinSet{"2": "feature-auth"})
	m = sendMsg(m, renameCompleteMsg{oldName: "feature-auth", newName: "auth"})
	if m.pins["2"] != "auth" {
		t.Errorf("pins = %v", m.pins)
	}
	if saved := loadUIPrefs(m.projectRoot); saved == nil || saved.Pins["2"] != "auth" {
		t.Errorf("saved = %+v", saved)
	}
}

func TestPins_DeleteDropsPin(t *testing.T) {
	m := pinModel(t, pinSet{"2": "feature-auth", "1": "root"})
	m = sendMsg(m, worktreeDeletedMsg{name: "feature-auth"})
	if _, ok := m.pins["2"]; ok || m.pins["1"] != "root" {
		t.Errorf("pins = %v", m.pins)
	}
}

func TestPins_PrunedOnlyAfterSuccessfulFetch(t *testing.T) {
	m := pinModel(t, pinSet{"5": "gone", "1": "root"})
	failed := sendMsg(m, worktreesFetchedMsg{err: errors.New("boom")})
	if failed.pins["5"] != "gone" {
		t.Error("a failed fetch pruned pins")
	}
	ok := sendMsg(m, worktreesFetchedMsg{items: makeTestItems(2)})
	if _, kept := ok.pins["5"]; kept || ok.pins["1"] != "root" {
		t.Errorf("successful fetch: pins %v, want only root", ok.pins)
	}
}
