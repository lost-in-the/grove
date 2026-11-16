package tui

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/exp/golden"

	"github.com/lost-in-the/grove/internal/config"
)

// ruleRow is the row of the rule between the list and the context panel.
func ruleRow(t *testing.T, view string) int {
	t.Helper()
	for i, l := range strings.Split(stripAnsi(view), "\n") {
		if strings.HasPrefix(strings.TrimSpace(l), "────") {
			return i
		}
	}
	t.Fatalf("no rule in:\n%s", stripAnsi(view))
	return -1
}

// checkConfigFrame checks the frame fills the terminal exactly and no line
// is wider than it.
func checkConfigFrame(t *testing.T, m Model, w, h int, label string) {
	t.Helper()
	lines := strings.Split(m.viewString(), "\n")
	if len(lines) != h {
		t.Errorf("%s: frame has %d lines, want %d", label, len(lines), h)
	}
	for i, l := range lines {
		if lw := lipgloss.Width(l); lw > w {
			t.Errorf("%s: line %d is %d cells, wider than %d", label, i, lw, w)
		}
	}
}

// One layout at a given size: every section, every focused field, and an
// edit in progress put the rule on the same row and fill the terminal.
func TestConfig_LayoutStableAcrossSectionsAndFields(t *testing.T) {
	for _, size := range [][2]int{{80, 24}, {120, 40}, {44, 24}} {
		w, h := size[0], size[1]
		m := openConfigWith(t, w, h, nil)
		rule := ruleRow(t, m.viewString())
		for tab := ConfigTab(0); tab < ConfigTabCount; tab++ {
			m = sendConfigKeys(t, m, string(rune('1'+tab)))
			for i, f := range m.configState.Fields[tab] {
				label := fmt.Sprintf("%dx%d %s", w, h, f.Key)
				checkConfigFrame(t, m, w, h, label)
				if got := ruleRow(t, m.viewString()); got != rule {
					t.Errorf("%s: rule on row %d, want %d", label, got, rule)
				}
				if f.Type == ConfigString || f.Type == ConfigList {
					edit := sendConfigKeys(t, m, "enter")
					checkConfigFrame(t, edit, w, h, label+" editing")
					if got := ruleRow(t, edit.viewString()); got != rule {
						t.Errorf("%s editing: rule on row %d, want %d", label, got, rule)
					}
				}
				if i < len(m.configState.Fields[tab])-1 {
					m = sendConfigKeys(t, m, "down")
				}
			}
		}
	}
}

// At 80x24 nothing is cut: every field's whole context fits, with every
// option and its meaning.
func TestConfig_NothingClippedAt80x24(t *testing.T) {
	m := openConfigWith(t, 80, 24, func(c *config.Config) { c.ProjectsDir = "~/projects" })
	for tab := ConfigTab(0); tab < ConfigTabCount; tab++ {
		m = sendConfigKeys(t, m, string(rune('1'+tab)))
		for i, f := range m.configState.Fields[tab] {
			view := stripAnsi(m.viewString())
			if strings.Contains(view, "…") {
				t.Errorf("%s: something is truncated:\n%s", f.Key, view)
			}
			if !strings.Contains(view, f.Description[:min(len(f.Description), 30)]) {
				t.Errorf("%s: description missing:\n%s", f.Key, view)
			}
			for _, o := range f.Options {
				if !strings.Contains(view, o.Meaning) {
					t.Errorf("%s: option %s's meaning missing:\n%s", f.Key, o.Value, view)
				}
			}
			for _, other := range m.configState.Fields[tab] {
				if !strings.Contains(view, other.Label) {
					t.Errorf("%s: row %s missing:\n%s", f.Key, other.Label, view)
				}
			}
			if i < len(m.configState.Fields[tab])-1 {
				m = sendConfigKeys(t, m, "down")
			}
		}
	}
}

// The footer names the keys that work on the focused field.
func TestConfig_FooterMatchesTheField(t *testing.T) {
	m := openConfigWith(t, 80, 24, nil)
	footer := func(m Model) string { return stripAnsi(m.configFooter()) }

	m = focusField(t, m, "switch.dirty_handling")
	if f := footer(m); !strings.Contains(f, "←→ change") || strings.Contains(f, "enter edit") || strings.Contains(f, "u undo") {
		t.Errorf("on a choice, nothing changed yet: %q", f)
	}
	m = sendConfigKeys(t, m, "l")
	if f := footer(m); !strings.Contains(f, "u undo") {
		t.Errorf("after a change: %q, want u undo", f)
	}
	m = focusField(t, m, "naming.pattern")
	if f := footer(m); !strings.Contains(f, "enter edit") || strings.Contains(f, "←→") {
		t.Errorf("on a text field: %q", f)
	}
	m = sendConfigKeys(t, m, "enter")
	if f := footer(m); !strings.Contains(f, "enter save") || !strings.Contains(f, "esc cancel") || strings.Contains(f, "↑↓") {
		t.Errorf("while editing: %q", f)
	}
}

// The "saved" note goes when its timer fires, unless a newer write
// restarted it.
func TestConfig_SavedNoteExpires(t *testing.T) {
	m := openConfigWith(t, 80, 24, nil)
	m = focusField(t, m, "switch.dirty_handling")
	m = sendConfigKeys(t, m, "l")
	old := m.configState.flashGen
	m = sendConfigKeys(t, m, "l")
	m = sendMsg(m, configFlashDoneMsg{gen: old})
	if h := configHeaderLine(m); !strings.Contains(h, "saved") {
		t.Errorf("an old timer cleared the note: %q", h)
	}
	m = sendMsg(m, configFlashDoneMsg{gen: m.configState.flashGen})
	if h := configHeaderLine(m); strings.Contains(h, "saved") {
		t.Errorf("the note should be gone: %q", h)
	}
}

func TestConfig_HeaderAndTabs(t *testing.T) {
	m := openConfigWith(t, 100, 30, nil)
	lines := strings.Split(stripAnsi(m.viewString()), "\n")
	if !strings.Contains(lines[0], "test-project ▸ settings") {
		t.Errorf("header: %q", lines[0])
	}
	for _, tab := range []string{"[1] general", "[2] behavior", "[3] plugins", "[4] protection"} {
		if !strings.Contains(lines[1], tab) {
			t.Errorf("tab row lacks %q: %q", tab, lines[1])
		}
	}

	m = openConfigWith(t, 44, 24, nil)
	m = sendConfigKeys(t, m, "3")
	row := strings.Split(stripAnsi(m.viewString()), "\n")[1]
	if !strings.Contains(row, "[3] plugins") || strings.Contains(row, "general") || !strings.Contains(row, "[1]") {
		t.Errorf("narrow tab row should name only the active tab: %q", row)
	}
}

// A resize keeps the frame filling the terminal.
func TestConfig_Resize(t *testing.T) {
	m := openConfigWith(t, 80, 24, nil)
	for _, size := range [][2]int{{120, 40}, {60, 20}, {44, 24}} {
		m = sendMsg(m, tea.WindowSizeMsg{Width: size[0], Height: size[1]})
		for tab := ConfigTab(0); tab < ConfigTabCount; tab++ {
			m = sendConfigKeys(t, m, string(rune('1'+tab)))
			checkConfigFrame(t, m, size[0], size[1], fmt.Sprintf("%dx%d %s", size[0], size[1], tabName(tab)))
		}
	}
}

func TestRenderConfig_Loading(t *testing.T) {
	m := newTestModel(withItems(3), withSize(80, 24))
	m = sendKey(m, "c")
	if v := stripAnsi(m.viewString()); !strings.Contains(v, "Loading settings") {
		t.Errorf("expected a loading message:\n%s", v)
	}
	checkConfigFrame(t, m, 80, 24, "loading")
}

// goldenConfig opens the panel on the golden fixture at size, focused on
// key.
func goldenConfig(t *testing.T, size termSize, key string) Model {
	t.Helper()
	m := goldenModel(t, size, withItems(3), withConfigOverlay())
	return focusField(t, m, key)
}

func TestGolden_Config(t *testing.T) {
	wide := termSize{"wide_120x40", 120, 40}
	narrow := termSize{"narrow_44x24", 44, 24}
	cases := []struct {
		name string
		size termSize
		key  string
	}{
		{"general_80x24", sizeStandard, "project_name"},
		{"behavior_80x24", sizeStandard, "switch.dirty_handling"},
		{"plugins_80x24", sizeStandard, "plugins.docker.auto_start"},
		{"protection_80x24", sizeStandard, "protection.protected"},
		{"behavior_44x24", narrow, "switch.dirty_handling"},
		{"behavior_120x40", wide, "switch.dirty_handling"},
		{"text_field_80x24", sizeStandard, "naming.pattern"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			m := goldenConfig(t, c.size, c.key)
			golden.RequireEqual(t, []byte(m.viewString()))
		})
	}
}

// States reached through keys: an edit, a write, a failed write, and a
// rejected value.
func TestGolden_Config_States(t *testing.T) {
	t.Run("text_edit_80x24", func(t *testing.T) {
		m := goldenConfig(t, sizeStandard, "naming.pattern")
		m = sendConfigKeys(t, m, "enter")
		m = typeConfig(t, m, "-wt")
		golden.RequireEqual(t, []byte(m.viewString()))
	})
	t.Run("text_invalid_80x24", func(t *testing.T) {
		m := goldenConfig(t, sizeStandard, "naming.pattern")
		m = sendConfigKeys(t, m, "enter", "ctrl+u")
		m = typeConfig(t, m, "{name}")
		m = sendConfigKeys(t, m, "enter")
		golden.RequireEqual(t, []byte(m.viewString()))
	})
	t.Run("saved_80x24", func(t *testing.T) {
		m := goldenConfig(t, sizeStandard, "switch.dirty_handling")
		m = writeIgnoringDisk(t, m, "l")
		golden.RequireEqual(t, []byte(m.viewString()))
	})
	t.Run("error_80x24", func(t *testing.T) {
		m := goldenConfig(t, sizeStandard, "switch.dirty_handling")
		failWrites(t, errors.New("open .grove/config.toml: permission denied"))
		m = sendConfigKeys(t, m, "l")
		golden.RequireEqual(t, []byte(m.viewString()))
	})
	t.Run("error_44x24", func(t *testing.T) {
		m := goldenConfig(t, termSize{"narrow_44x24", 44, 24}, "switch.dirty_handling")
		failWrites(t, errors.New("open .grove/config.toml: permission denied"))
		m = sendConfigKeys(t, m, "l")
		golden.RequireEqual(t, []byte(m.viewString()))
	})
}

// writeIgnoringDisk sends keys with writes that succeed without touching a
// file, for goldens of the state after a write. The reload after a write
// reads an empty temp project, not this repository's config.
func writeIgnoringDisk(t *testing.T, m Model, keys ...string) Model {
	t.Helper()
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(t.TempDir()); err != nil {
		t.Fatal(err)
	}
	write := saveProjectConfig
	saveProjectConfig = func(map[string]string) error { return nil }
	defer func() {
		saveProjectConfig = write
		_ = os.Chdir(cwd) // back to the package, where the goldens live
	}()
	return sendConfigKeys(t, m, keys...)
}

// This one opens the panel the way the dashboard does, on the default
// config, so the Behavior section shows every field it really has.
func TestGolden_Config_BehaviorDefaults(t *testing.T) {
	m := goldenModel(t, sizeStandard, withItems(3))
	m = sendKey(m, "c")
	m = sendConfigMsg(t, m, configLoadedMsg{cfg: config.LoadDefaults()})
	m = sendConfigKeys(t, m, "2")
	golden.RequireEqual(t, []byte(m.viewString()))
}

// The raised tab and the selected row are color only, so they need a
// themed golden (spec 18).
func TestGolden_Themed_ConfigTabs(t *testing.T) {
	m := goldenModelThemed(t, sizeStandard, withItems(3), withConfigOverlay())
	m = focusField(t, m, "plugins.docker.auto_start")
	golden.RequireEqual(t, []byte(m.viewString()))
}
