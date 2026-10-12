package tui

import (
	"testing"

	"charm.land/bubbles/v2/list"
	tea "charm.land/bubbletea/v2"
)

func TestNewWorktreeList_DisablesListQuit(t *testing.T) {
	l := newWorktreeList(rowDelegate{})
	if l.KeyMap.Quit.Enabled() {
		t.Fatal("list Quit binding is enabled; grove handles quitting, so the list's own binding must be off")
	}
}

// The list re-derives its bindings on SetItems, SetSize, and filter changes,
// so the guard must survive those, not just construction.
func TestNewWorktreeList_QuitStaysDisabledAfterListUpdates(t *testing.T) {
	l := newWorktreeList(rowDelegate{})
	l.SetItems([]list.Item{makeTestItems(1)[0]})
	l.SetSize(80, 24)
	l.SetFilteringEnabled(true)
	if l.KeyMap.Quit.Enabled() {
		t.Error("list Quit binding re-enabled after list updates")
	}
	if !l.KeyMap.ForceQuit.Enabled() {
		t.Error("list ForceQuit disabled; ctrl+c quits the dashboard through it")
	}
}

func TestNewWorktreeList_KeepsGroveBindings(t *testing.T) {
	l := newWorktreeList(rowDelegate{})
	if got := l.KeyMap.Filter.Keys(); len(got) != 1 || got[0] != "/" {
		t.Errorf("Filter keys = %v, want [/]", got)
	}
	if got := l.KeyMap.CursorUp.Keys(); len(got) != 2 || got[0] != "up" || got[1] != "k" {
		t.Errorf("CursorUp keys = %v, want [up k]", got)
	}
	if got := l.KeyMap.CursorDown.Keys(); len(got) != 2 || got[0] != "down" || got[1] != "j" {
		t.Errorf("CursorDown keys = %v, want [down j]", got)
	}
}

func TestDashboard_CtrlCQuits(t *testing.T) {
	m := newTestModel(withItems(1), withSize(80, 24))
	_, cmd := m.Update(tea.KeyPressMsg{Code: 'c', Mod: tea.ModCtrl})
	if cmd == nil {
		t.Fatal("ctrl+c returned no command; want quit")
	}
	if _, ok := cmd().(tea.QuitMsg); !ok {
		t.Errorf("ctrl+c command produced %T, want tea.QuitMsg", cmd())
	}
}
