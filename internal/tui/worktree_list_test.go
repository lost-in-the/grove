package tui

import "testing"

func TestNewWorktreeList_DisablesListQuit(t *testing.T) {
	l := newWorktreeList(NewWorktreeDelegateV2())
	if l.KeyMap.Quit.Enabled() {
		t.Fatal("list Quit binding is enabled; grove handles quitting, so the list's own binding must be off")
	}
}

func TestNewWorktreeList_KeepsGroveBindings(t *testing.T) {
	l := newWorktreeList(NewWorktreeDelegateV2())
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
