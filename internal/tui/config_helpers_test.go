package tui

import (
	"testing"
	"time"

	"charm.land/bubbles/v2/cursor"
	tea "charm.land/bubbletea/v2"

	"github.com/lost-in-the/grove/internal/config"
)

// sendConfigKeys sends keys while the config frame is open and feeds the
// commands they return back in: huh moves focus through commands, so
// sendKey alone cannot tab. It stops running commands once config closes,
// so a save command never runs in a test.
//
// Batches are expanded; sequences are not (bubbletea's sequenceMsg is
// unexported), so a sequence's commands never run. huh returns one only
// from Form.Init, which focuses the first field synchronously, so the
// tests do not depend on what the sequence carries.
func sendConfigKeys(t *testing.T, m Model, keys ...string) Model {
	t.Helper()
	for _, k := range keys {
		m, _ = sendConfigKey(t, m, k)
	}
	return m
}

// sendConfigKey is sendConfigKeys for one key. It also returns the commands
// that did not return promptly, such as a cursor blink, so a test can check
// what the key started.
func sendConfigKey(t *testing.T, m Model, k string) (Model, []tea.Cmd) {
	t.Helper()
	return sendConfigMsg(t, m, makeKeyMsg(k))
}

// sendConfigMsg is sendConfigKey for any message.
func sendConfigMsg(t *testing.T, m Model, msg tea.Msg) (Model, []tea.Cmd) {
	t.Helper()
	next, cmd := m.Update(msg)
	m = next.(Model)
	var queue []tea.Msg
	var slow []tea.Cmd
	if m.activeView() == ViewConfig {
		queue, slow = runCmdKeepSlow(cmd)
	}
	for n := 0; len(queue) > 0 && n < 200 && m.activeView() == ViewConfig; n++ {
		next, cmd = m.Update(queue[0])
		m = next.(Model)
		queue = queue[1:]
		if m.activeView() == ViewConfig {
			msgs, s := runCmdKeepSlow(cmd)
			queue = append(queue, msgs...)
			slow = append(slow, s...)
		}
	}
	return m, slow
}

// runCmd runs cmd and returns its messages, expanding batches. Commands that
// do not return promptly (cursor blinks, ticks) are skipped. The timeout is
// generous so a slow runner under -race does not drop a focus move.
func runCmd(cmd tea.Cmd) []tea.Msg {
	msgs, _ := runCmdKeepSlow(cmd)
	return msgs
}

// runCmdKeepSlow is runCmd, but also returns the commands it skipped.
func runCmdKeepSlow(cmd tea.Cmd) ([]tea.Msg, []tea.Cmd) {
	if cmd == nil {
		return nil, nil
	}
	ch := make(chan tea.Msg, 1)
	go func() { ch <- cmd() }()
	select {
	case msg := <-ch:
		switch msg := msg.(type) {
		case nil:
			return nil, nil
		case tea.BatchMsg:
			var out []tea.Msg
			var slow []tea.Cmd
			for _, c := range msg {
				m, s := runCmdKeepSlow(c)
				out = append(out, m...)
				slow = append(slow, s...)
			}
			return out, slow
		}
		return []tea.Msg{msg}, nil
	case <-time.After(100 * time.Millisecond):
		return nil, []tea.Cmd{cmd}
	}
}

// startsBlink reports whether any of cmds is a cursor blink: it runs them
// all at once, waiting up to a second, longer than the blink interval.
func startsBlink(cmds []tea.Cmd) bool {
	found := make(chan bool, len(cmds))
	for _, c := range cmds {
		go func() {
			_, ok := c().(cursor.BlinkMsg)
			found <- ok
		}()
	}
	timeout := time.After(time.Second)
	for range cmds {
		select {
		case ok := <-found:
			if ok {
				return true
			}
		case <-timeout:
			return false
		}
	}
	return false
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
