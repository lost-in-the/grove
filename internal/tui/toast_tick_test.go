package tui

import (
	"testing"
	"time"

	"charm.land/bubbles/v2/spinner"
	tea "charm.land/bubbletea/v2"
)

// A toast expires on the spinner tick, which runs only while something
// animates. On an idle dashboard, a message that shows a toast must start
// the tick, or the toast never goes away.
func TestToast_ShowingOneStartsTheTick(t *testing.T) {
	cases := []struct {
		name string
		msg  tea.Msg
	}{
		{"empty pin slot", makeKeyMsg("3")},
		{"no previous worktree", makeKeyMsg("-")},
		{"config save failed after close", configSavedMsg{err: errTest}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			m := newTestModel(withItems(3), withSize(100, 24))
			next, cmd := m.Update(c.msg)
			if next.(Model).toast.Current == nil {
				t.Fatal("no toast shown")
			}
			if !startsSpinner(cmd) {
				t.Error("showing a toast should start the spinner tick that expires it")
			}
		})
	}
}

// startsSpinner reports whether cmd, run through its batches, yields a
// spinner tick. spinner.Tick returns at once; the timeout only bounds a
// command that blocks.
func startsSpinner(cmd tea.Cmd) bool {
	if cmd == nil {
		return false
	}
	ch := make(chan tea.Msg, 1)
	go func() { ch <- cmd() }()
	select {
	case msg := <-ch:
		switch msg := msg.(type) {
		case spinner.TickMsg:
			return true
		case tea.BatchMsg:
			for _, c := range msg {
				if startsSpinner(c) {
					return true
				}
			}
		}
	case <-time.After(time.Second):
	}
	return false
}
