package tui

import "testing"

func TestExit_UpLeavesTheMoveToTheCaller(t *testing.T) {
	m := newTestModel(withItems(3), withSize(80, 24))
	m = sendKey(m, "j")
	item := m.list.Items()[1].(WorktreeItem)

	m = sendKey(m, "U")

	want := Exit{Path: item.Path, Name: item.displayName(), Up: true}
	if got := m.Exit(); got != want {
		t.Errorf("Exit() = %+v, want %+v", got, want)
	}
	if m.movesOnExit() {
		t.Error("the dashboard moved you itself after U; the caller must move you so the stack comes up first")
	}
}

func TestExit_UpOnTheWorktreeYouAreInStillBringsTheStack(t *testing.T) {
	m := newTestModel(withItems(3), withSize(80, 24))
	item := m.list.Items()[0].(WorktreeItem)

	m = sendKey(m, "U")

	want := Exit{Path: item.Path, Name: item.displayName(), Up: true}
	if got := m.Exit(); got != want {
		t.Errorf("Exit() = %+v, want %+v", got, want)
	}
}

func TestExit_EnterMovesYouWithoutTheStack(t *testing.T) {
	m := newTestModel(withItems(3), withSize(80, 24))
	m = sendKey(m, "j")
	item := m.list.Items()[1].(WorktreeItem)

	m = sendKey(m, "enter")

	want := Exit{Path: item.Path, Name: item.displayName()}
	if got := m.Exit(); got != want {
		t.Errorf("Exit() = %+v, want %+v", got, want)
	}
	if !m.movesOnExit() {
		t.Error("enter must move you from the dashboard's own exit path")
	}
}

func TestExit_QuitAsksForNothing(t *testing.T) {
	m := newTestModel(withItems(3), withSize(80, 24))

	m = sendKey(m, "q")

	if got := m.Exit(); got != (Exit{}) {
		t.Errorf("Exit() = %+v, want the zero Exit", got)
	}
	if m.movesOnExit() {
		t.Error("quit must not move you")
	}
}
