package tui

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/lost-in-the/grove/internal/config"
)

// The panel writes the project config on every change, and the package's
// working directory sits inside this repository, whose .grove/config.toml is
// real. Every write a test makes must land in a temp project, so the writer
// refuses anywhere else.
func init() {
	write := saveProjectConfig
	saveProjectConfig = func(updates map[string]string) error {
		if err := requireTempWorkingDir(); err != nil {
			return err
		}
		return write(updates)
	}
}

func requireTempWorkingDir() error {
	cwd, err := os.Getwd()
	if err != nil {
		return err
	}
	cwd, _ = filepath.EvalSymlinks(cwd)
	tmp, _ := filepath.EvalSymlinks(os.TempDir())
	if tmp == "" || !strings.HasPrefix(cwd, tmp) {
		return errors.New("test tried to write a config outside a temp project")
	}
	return nil
}

// openConfigIn opens the settings panel on a temp project whose
// .grove/config.toml holds content, and returns the model and that file's
// path. The test's working directory moves into the project, so the panel
// loads and writes that file. The dashboard behind it lists root and two
// other worktrees.
func openConfigIn(t *testing.T, w, h int, content string) (Model, string) {
	t.Helper()
	return openConfigOver(t, w, h, content, 3)
}

// openConfigOver is openConfigIn over a dashboard of n worktrees, root
// included.
func openConfigOver(t *testing.T, w, h int, content string, n int) (Model, string) {
	t.Helper()
	project := t.TempDir()
	path := filepath.Join(project, ".grove", "config.toml")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Chdir(project)

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("load the temp project's config: %v", err)
	}
	m := newTestModel(withItems(n), withSize(w, h))
	m = sendKey(m, "c")
	m = sendConfigMsg(t, m, configLoadedMsg{cfg: cfg})
	return m, path
}

// openConfigWith opens the panel on the defaults with edit applied, in an
// empty temp project.
func openConfigWith(t *testing.T, w, h int, edit func(*config.Config)) Model {
	t.Helper()
	m, _ := openConfigIn(t, w, h, "")
	if edit != nil {
		cfg := config.LoadDefaults()
		edit(cfg)
		m = sendConfigMsg(t, m, configLoadedMsg{cfg: cfg})
	}
	return m
}

// readConfigFile returns the file's text.
func readConfigFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

// sendConfigKeys sends keys while the panel is open and feeds the commands
// they return back in. Commands that do not answer at once (the "saved"
// timer, a cursor blink) are skipped.
func sendConfigKeys(t *testing.T, m Model, keys ...string) Model {
	t.Helper()
	for _, k := range keys {
		m = sendConfigMsg(t, m, makeKeyMsg(k))
	}
	return m
}

// typeConfig types text one key at a time.
func typeConfig(t *testing.T, m Model, text string) Model {
	t.Helper()
	for _, r := range text {
		m = sendConfigKeys(t, m, string(r))
	}
	return m
}

// sendConfigMsg is sendConfigKeys for any message.
func sendConfigMsg(t *testing.T, m Model, msg tea.Msg) Model {
	t.Helper()
	next, cmd := m.Update(msg)
	m = next.(Model)
	queue := runCmd(cmd)
	for n := 0; len(queue) > 0 && n < 200 && m.activeView() == ViewConfig; n++ {
		next, cmd = m.Update(queue[0])
		m = next.(Model)
		queue = append(queue[1:], runCmd(cmd)...)
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
	case <-time.After(100 * time.Millisecond):
		return nil
	}
}

// focusedKey is the config key of the focused field.
func focusedKey(m Model) string {
	if f := m.configState.focused(); f != nil {
		return f.Key
	}
	return ""
}

// focusField moves the cursor to key, switching section as needed, with
// the panel's own keys.
func focusField(t *testing.T, m Model, key string) Model {
	t.Helper()
	for tab := ConfigTab(0); tab < ConfigTabCount; tab++ {
		for i, f := range m.configState.Fields[tab] {
			if f.Key != key {
				continue
			}
			m = sendConfigKeys(t, m, string(rune('1'+tab)))
			for range len(m.configState.Fields[tab]) {
				switch c := m.configState.cursor[tab]; {
				case c > i:
					m = sendConfigKeys(t, m, "up")
				case c < i:
					m = sendConfigKeys(t, m, "down")
				}
			}
			if focusedKey(m) != key {
				t.Fatalf("could not focus %s: focus is on %s", key, focusedKey(m))
			}
			return m
		}
	}
	t.Fatalf("no field %s", key)
	return m
}

// configHeaderLine is the frame's first line, without styling.
func configHeaderLine(m Model) string {
	return strings.Split(stripAnsi(m.viewString()), "\n")[0]
}
