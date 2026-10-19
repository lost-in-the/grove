package tui

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/exp/golden"
)

// The frame fills the terminal exactly, and the form block keeps one
// height in every section and after moving between fields (spec 11, 18).
func TestConfig_HeightStableAcrossSections(t *testing.T) {
	for _, size := range [][2]int{{80, 24}, {120, 40}} {
		m := openConfigWith(t, size[0], size[1], nil)
		check := func(label string) {
			t.Helper()
			lines := strings.Split(m.viewString(), "\n")
			if len(lines) != size[1] {
				t.Errorf("%dx%d %s: frame has %d lines, want %d", size[0], size[1], label, len(lines), size[1])
			}
			for i, l := range lines {
				if w := lipgloss.Width(l); w > size[0] {
					t.Errorf("%dx%d %s: line %d is %d cells", size[0], size[1], label, i, w)
				}
			}
			if h := lipgloss.Height(m.configState.form().View()); h != m.configState.formH {
				t.Errorf("%dx%d %s: form block is %d rows, want %d", size[0], size[1], label, h, m.configState.formH)
			}
		}
		for tab := ConfigTab(0); tab < ConfigTabCount; tab++ {
			m.configState.Tab = tab
			check(tabName(tab))
			m = sendConfigKeys(t, m, "tab")
			check(tabName(tab) + " after tab")
		}
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
	if strings.Contains(stripAnsi(m.viewString()), "Configuration") {
		t.Error("the bordered overlay title should be gone")
	}

	m = openConfigWith(t, 44, 24, nil)
	m.configState.Tab = ConfigTabPlugins
	row := strings.Split(stripAnsi(m.viewString()), "\n")[1]
	if !strings.Contains(row, "[3] plugins") || strings.Contains(row, "general") || !strings.Contains(row, "[1]") {
		t.Errorf("narrow tab row should name only the active tab: %q", row)
	}
}

func TestConfig_ResizeRefitsForms(t *testing.T) {
	m := openConfigWith(t, 80, 24, nil)
	m = sendMsg(m, tea.WindowSizeMsg{Width: 120, Height: 40})
	if lines := strings.Split(m.viewString(), "\n"); len(lines) != 40 {
		t.Errorf("after resize the frame has %d lines, want 40", len(lines))
	}
}

func TestGolden_Config(t *testing.T) {
	cases := map[string]struct {
		size  termSize
		tab   ConfigTab
		dirty bool
	}{
		"general_80x24":       {sizeStandard, ConfigTabGeneral, false},
		"behavior_80x24":      {sizeStandard, ConfigTabBehavior, false},
		"plugins_80x24":       {sizeStandard, ConfigTabPlugins, false},
		"protection_80x24":    {sizeStandard, ConfigTabProtection, false},
		"general_120x40":      {sizeWide, ConfigTabGeneral, false},
		"general_44x24":       {termSize{"narrow_44x24", 44, 24}, ConfigTabGeneral, false},
		"general_dirty_80x24": {sizeStandard, ConfigTabGeneral, true},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			m := goldenModel(t, c.size, withItems(3), withConfigOverlay())
			m.configState.Tab = c.tab
			if c.dirty {
				// A different loaded value, not a different shown one: huh
				// keeps an input's text from when it was built.
				m.configState.Fields[ConfigTabGeneral][0].Loaded = "grove"
			}
			golden.RequireEqual(t, []byte(m.viewString()))
		})
	}
}

// The raised tab is color-only, so it needs a themed golden (spec 18).
func TestGolden_Themed_ConfigTabs(t *testing.T) {
	m := goldenModelThemed(t, sizeStandard, withItems(3), withConfigOverlay())
	m.configState.Tab = ConfigTabPlugins
	golden.RequireEqual(t, []byte(m.viewString()))
}
