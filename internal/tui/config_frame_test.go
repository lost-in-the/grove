package tui

import (
	"fmt"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/exp/golden"
)

// checkConfigFrame checks the frame against the terminal size alone: it
// fills the terminal, the form block gets the rows the header, tabs, blank
// row, status line, and footer leave, at the frame's inner width, and an
// error sits on the status row just above the footer (spec 11, 18).
func checkConfigFrame(t *testing.T, m Model, w, h int, label string) {
	t.Helper()
	const chrome = 4 // header, tabs, blank row, status line
	footerH := lipgloss.Height(m.configFooter())

	lines := strings.Split(m.viewString(), "\n")
	if len(lines) != h {
		t.Errorf("%s: frame has %d lines, want %d", label, len(lines), h)
	}
	for i, l := range lines {
		if lw := lipgloss.Width(l); lw > w {
			t.Errorf("%s: line %d is %d cells, wider than %d", label, i, lw, w)
		}
	}

	form := strings.Split(m.configState.form().View(), "\n")
	if want := h - chrome - footerH; len(form) != want {
		t.Errorf("%s: form block is %d rows, want %d", label, len(form), want)
	}
	if fw := lipgloss.Width(form[0]); fw != w-2 {
		t.Errorf("%s: form block is %d cells wide, want %d", label, fw, w-2)
	}

	m.configState.Err = errTest
	defer func() { m.configState.Err = nil }()
	lines = strings.Split(stripAnsi(m.viewString()), "\n")
	if status := h - 1 - footerH; status >= len(lines) || !strings.Contains(lines[status], "test error") {
		t.Errorf("%s: error not on the status row %d:\n%s", label, h-1-footerH, strings.Join(lines, "\n"))
	}
}

// Every section keeps one frame, before and after moving between fields.
func TestConfig_HeightStableAcrossSections(t *testing.T) {
	for _, size := range [][2]int{{80, 24}, {120, 40}, {44, 24}} {
		w, h := size[0], size[1]
		m := openConfigWith(t, w, h, nil)
		for tab := ConfigTab(0); tab < ConfigTabCount; tab++ {
			m.configState.Tab = tab
			label := fmt.Sprintf("%dx%d %s", w, h, tabName(tab))
			checkConfigFrame(t, m, w, h, label)
			m = sendConfigKeys(t, m, "tab")
			checkConfigFrame(t, m, w, h, label+" after tab")
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

// A resize refits every section's form, growing and shrinking.
func TestConfig_ResizeRefitsForms(t *testing.T) {
	m := openConfigWith(t, 80, 24, nil)
	for _, size := range [][2]int{{120, 40}, {60, 20}} {
		w, h := size[0], size[1]
		m = sendMsg(m, tea.WindowSizeMsg{Width: w, Height: h})
		for tab := ConfigTab(0); tab < ConfigTabCount; tab++ {
			m.configState.Tab = tab
			checkConfigFrame(t, m, w, h, fmt.Sprintf("resized to %dx%d %s", w, h, tabName(tab)))
		}
	}
}

// huh centers a confirm's buttons under its own title and description, so
// fields with different text would stagger them. They line up on the left.
func TestConfig_BoolButtonsAlign(t *testing.T) {
	m := openConfigWith(t, 100, 30, nil)
	m.configState.Tab = ConfigTabPlugins
	var cols []int
	for _, l := range strings.Split(stripAnsi(m.viewString()), "\n") {
		if strings.Contains(l, "true") && strings.Contains(l, "false") {
			cols = append(cols, lipgloss.Width(l[:strings.Index(l, "true")]))
		}
	}
	if len(cols) != 3 {
		t.Fatalf("found %d button rows, want 3", len(cols))
	}
	for _, c := range cols[1:] {
		if c != cols[0] {
			t.Errorf("button columns %v, want all equal", cols)
			break
		}
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
