package tui

import (
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/lost-in-the/grove/internal/config"
)

// sendConfigKeys sends keys while the config frame is open and feeds the
// commands they return back in: huh moves focus through commands, so
// sendKey alone cannot tab. It stops running commands once config closes,
// so a save command never runs in a test.
func sendConfigKeys(t *testing.T, m Model, keys ...string) Model {
	t.Helper()
	for _, k := range keys {
		next, cmd := m.Update(makeKeyMsg(k))
		m = next.(Model)
		var queue []tea.Msg
		if m.activeView() == ViewConfig {
			queue = runCmd(cmd)
		}
		for n := 0; len(queue) > 0 && n < 200 && m.activeView() == ViewConfig; n++ {
			next, cmd = m.Update(queue[0])
			m = next.(Model)
			queue = queue[1:]
			if m.activeView() == ViewConfig {
				queue = append(queue, runCmd(cmd)...)
			}
		}
	}
	return m
}

// runCmd runs cmd and returns its messages, expanding batches. Commands that
// do not return promptly (cursor blinks, ticks) are skipped.
func runCmd(cmd tea.Cmd) []tea.Msg {
	if cmd == nil {
		return nil
	}
	ch := make(chan tea.Msg, 1)
	go func() { ch <- cmd() }()
	select {
	case msg := <-ch:
		switch msg := msg.(type) {
		case nil:
			return nil
		case tea.BatchMsg:
			var out []tea.Msg
			for _, c := range msg {
				out = append(out, runCmd(c)...)
			}
			return out
		}
		return []tea.Msg{msg}
	case <-time.After(20 * time.Millisecond):
		return nil
	}
}

// openConfig opens the config frame on the defaults with the given changes
// applied to the loaded config.
func openConfigWith(t *testing.T, w, h int, edit func(*config.Config)) Model {
	t.Helper()
	m := newTestModel(withItems(3), withSize(w, h))
	m = sendKey(m, "c")
	cfg := config.LoadDefaults()
	if edit != nil {
		edit(cfg)
	}
	next, cmd := m.Update(configLoadedMsg{cfg: cfg})
	m = next.(Model)
	for _, msg := range runCmd(cmd) {
		next, _ = m.Update(msg)
		m = next.(Model)
	}
	return m
}

func focusedKey(m Model) string {
	if f := m.configState.Forms[m.configState.Tab].GetFocusedField(); f != nil {
		return f.GetKey()
	}
	return ""
}
