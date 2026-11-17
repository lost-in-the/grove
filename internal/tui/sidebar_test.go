package tui

import (
	"fmt"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
)

var sidebarSizes = []struct{ w, h int }{{120, 40}, {160, 50}}

func eachWideListMode(t *testing.T, fn func(t *testing.T, m Model)) {
	t.Helper()
	for _, size := range sidebarSizes {
		for _, mode := range []struct {
			name string
			opt  testOpt
		}{{"compact", withCompact()}, {"detailed", withDetailed()}} {
			t.Run(fmt.Sprintf("%s_%dx%d", mode.name, size.w, size.h), func(t *testing.T) {
				m := newTestModel(withNamedItems(30), withSize(size.w, size.h), mode.opt)
				m.projectRoot = prefsRoot(t)
				fn(t, m)
			})
		}
	}
}

// bodyLines is the screen between the header and the footer.
func bodyLines(t *testing.T, m Model) []string {
	t.Helper()
	lines := screenLines(t, m)
	footerH := m.helpFooter.CompactHeight(ViewDashboard, m.width-4)
	return lines[1 : len(lines)-footerH]
}

// t at 110+ columns hides the pane: the list takes the content width and
// the selection line sits under it, as below 110.
func TestSidebar_THidesThePane(t *testing.T) {
	eachWideListMode(t, func(t *testing.T, m Model) {
		divider := 1 + m.list.Width() + 1
		m = sendKey(m, "t")
		if !m.sidebarHidden {
			t.Fatal("t did not hide the sidebar")
		}
		if got, want := m.list.Width(), m.width-2; got != want {
			t.Errorf("list width %d, want the content width %d", got, want)
		}
		body := bodyLines(t, m)
		for i, l := range body {
			if c := ansi.Cut(l, divider, divider+1); c == "│" {
				t.Errorf("body line %d still has the divider: %q", i, l)
			}
		}
		if sel := body[len(body)-1]; !strings.HasPrefix(strings.TrimSpace(sel), "wt-00") {
			t.Errorf("the selection line should sit above the footer, got %q", sel)
		}
	})
}

func TestSidebar_TAgainRestoresThePane(t *testing.T) {
	eachWideListMode(t, func(t *testing.T, m Model) {
		m.updateDetailContent()
		before := stripAnsi(m.viewString()) // t lights its footer key, so compare text
		m = sendKey(sendKey(m, "t"), "t")
		if m.sidebarHidden {
			t.Fatal("t twice left the sidebar hidden")
		}
		if after := stripAnsi(m.viewString()); after != before {
			t.Errorf("t twice changed the screen:\nbefore:\n%s\nafter:\n%s", before, after)
		}
	})
}

// The choice is saved to sidebar_hidden, and the pins survive the save.
func TestSidebar_ChoicePersistsAndKeepsPins(t *testing.T) {
	root := prefsRoot(t)
	_ = updateUIPrefs(root, func(p *UIPrefs) { p.Pins = map[string]string{"3": "testing"} })
	m := newTestModel(withItems(3), withSize(120, 40))
	m.projectRoot = root

	m = sendKey(m, "t")
	got := loadUIPrefs(root)
	if got == nil || got.SidebarHidden == nil || !*got.SidebarHidden {
		t.Fatalf("after t: prefs %+v, want sidebar_hidden true", got)
	}
	if got.Pins["3"] != "testing" {
		t.Errorf("t dropped the pins: %+v", got)
	}

	sendKey(m, "t")
	if got := loadUIPrefs(root); got == nil || got.SidebarHidden == nil || *got.SidebarHidden {
		t.Errorf("after t twice: prefs %+v, want sidebar_hidden false", got)
	}
}

// tab, ctrl+i, and l while hidden show the pane again and focus it, never
// the full-screen sheet.
func TestSidebar_DetailKeysUnhideAndFocus(t *testing.T) {
	for _, k := range []string{"tab", "ctrl+i", "l"} {
		t.Run(k, func(t *testing.T) {
			root := prefsRoot(t)
			m := newTestModel(withItems(3), withSize(120, 40))
			m.projectRoot = root
			m = sendKey(sendKey(m, "t"), k)
			if m.sidebarHidden || !m.detailFocused {
				t.Errorf("hidden %v focused %v, want the pane shown and focused", m.sidebarHidden, m.detailFocused)
			}
			if m.activeView() != ViewDashboard {
				t.Errorf("view %v, want the dashboard (no sheet at 120 columns)", m.activeView())
			}
			if got := loadUIPrefs(root); got == nil || got.SidebarHidden == nil || *got.SidebarHidden {
				t.Errorf("prefs %+v, want sidebar_hidden false", got)
			}
			screenLines(t, m)
		})
	}
}

// Below 110 columns there is no room for the pane: t is a no-op and saves
// nothing.
func TestSidebar_TBelowSplitWidthIsANoOp(t *testing.T) {
	for _, size := range []struct{ w, h int }{{80, 24}, {44, 24}, {109, 30}} {
		t.Run(fmt.Sprintf("%dx%d", size.w, size.h), func(t *testing.T) {
			root := prefsRoot(t)
			m := newTestModel(withItems(5), withSize(size.w, size.h))
			m.projectRoot = root
			before := m.viewString()
			m = sendKey(m, "t")
			if m.sidebarHidden {
				t.Error("t below 110 columns flipped the saved state")
			}
			if after := m.viewString(); after != before {
				t.Errorf("t changed the screen:\nbefore:\n%s\nafter:\n%s", stripAnsi(before), stripAnsi(after))
			}
			if got := loadUIPrefs(root); got != nil {
				t.Errorf("t saved prefs %+v below 110 columns", got)
			}
		})
	}
}

// A hidden sidebar stays hidden through a narrow terminal and applies when
// it widens again.
func TestSidebar_HiddenStateAppliesWhenWidened(t *testing.T) {
	m := newTestModel(withItems(5), withSize(120, 40))
	m.projectRoot = prefsRoot(t)
	m = sendKey(m, "t")
	m = sendMsg(m, tea.WindowSizeMsg{Width: 80, Height: 24})
	m = sendMsg(m, tea.WindowSizeMsg{Width: 120, Height: 40})
	if !m.sidebarHidden || m.sidebarShown() {
		t.Fatalf("hidden %v shown %v, want hidden after widening", m.sidebarHidden, m.sidebarShown())
	}
	if got, want := m.list.Width(), m.width-2; got != want {
		t.Errorf("list width %d, want %d", got, want)
	}
}

// The pane loses focus when t hides it; the list has it again.
func TestSidebar_TOnFocusedPaneFocusesTheList(t *testing.T) {
	m := newTestModel(withItems(3), withSize(120, 40))
	m.projectRoot = prefsRoot(t)
	m = sendKey(m, "tab")
	if !m.detailFocused {
		t.Fatal("tab did not focus the pane")
	}
	m = sendKey(m, "t")
	if !m.sidebarHidden || m.detailFocused {
		t.Errorf("hidden %v focused %v, want the pane hidden and the list focused", m.sidebarHidden, m.detailFocused)
	}
	screenLines(t, m)
}

// With the sidebar hidden the form docks under the full-width list, as
// below 110 columns, and the screen stays the terminal's size.
func TestSidebar_DockedFormWhileHidden(t *testing.T) {
	m := newTestModel(uniqueItems(8), withSize(120, 40), selectRow(3))
	m.projectRoot = prefsRoot(t)
	m = sendKey(m, "t")
	withForm(formCaseNamed(t, "new_branch"))(&m)
	if lay := m.branchFormLayout(); lay.Dock != dockBottom || lay.Width != m.width-2 {
		t.Fatalf("layout %+v, want docked at the bottom, %d wide", lay, m.width-2)
	}
	lines := screenLines(t, m)
	row, title := cursorRow(lines, "wt-03"), lineIndex(lines, "New worktree")
	if row < 0 || title < 0 || row > title {
		t.Fatalf("want the cursor row above the form (row %d, title %d):\n%s", row, title, strings.Join(lines, "\n"))
	}
	if title != m.height-formPanelRows {
		t.Errorf("form title on row %d, want %d", title, m.height-formPanelRows)
	}
}

// t keeps the selected worktree selected and in view, both ways.
func TestSidebar_ToggleKeepsSelection(t *testing.T) {
	eachWideListMode(t, func(t *testing.T, m Model) {
		m.list.Select(25)
		for _, step := range []string{"hide", "show"} {
			m = sendKey(m, "t")
			if m.list.Index() != 25 || !strings.Contains(cursorLine(m.viewString()), "wt-25") {
				t.Errorf("after t (%s): index %d, cursor row %q", step, m.list.Index(), cursorLine(m.viewString()))
			}
		}
	})
}

// The footer offers t only where it does something (110+ columns) and fits
// without wrapping the footer: a list row is worth more than the hint.
func TestSidebar_FooterHint(t *testing.T) {
	for _, tc := range []struct {
		w, h  int
		hint  bool
		lines int
	}{
		{80, 24, false, 2},
		{120, 40, false, 1},
		{160, 50, true, 1},
	} {
		m := newTestModel(withItems(3), withSize(tc.w, tc.h))
		footer := m.helpFooter.RenderCompact(ViewDashboard, m.width-4)
		if n := strings.Count(footer, "\n") + 1; n != tc.lines {
			t.Errorf("%dx%d: footer is %d lines, want %d:\n%s", tc.w, tc.h, n, tc.lines, stripAnsi(footer))
		}
		if got := strings.Contains(stripAnsi(m.viewString()), "t sidebar"); got != tc.hint {
			t.Errorf("%dx%d: t sidebar shown %v, want %v:\n%s", tc.w, tc.h, got, tc.hint, stripAnsi(footer))
		}
		// t works wherever the pane can show, hint or not.
		if isWideLayout(tc.w) {
			m.projectRoot = prefsRoot(t)
			if m = sendKey(m, "t"); !m.sidebarHidden {
				t.Errorf("%dx%d: t did not hide the sidebar", tc.w, tc.h)
			}
		}
	}
}

func TestSidebarHintFits(t *testing.T) {
	plain := keyLine(moveFooter(Hint{"q", "quit"}, dashboardFooterHints("detailed", false)...), 1000)
	with := keyLine(moveFooter(Hint{"q", "quit"}, dashboardFooterHints("detailed", true)...), 1000)
	need := lipgloss.Width(with)
	if lipgloss.Width(plain) >= need {
		t.Fatalf("the t hint adds no width: %q vs %q", plain, with)
	}
	for _, tc := range []struct {
		width int
		want  bool
	}{{need, true}, {need - 1, false}, {lipgloss.Width(plain), false}, {need + 40, true}} {
		if got := sidebarHintFits("detailed", tc.width); got != tc.want {
			t.Errorf("width %d (one line needs %d): fits %v, want %v", tc.width, need, got, tc.want)
		}
	}
}

// The focused pane's footer offers t when it fits, which it does at 120;
// the full-screen sheet below 110 does not, since t does nothing there.
func TestSidebar_DetailFooterHint(t *testing.T) {
	wide := sendKey(newTestModel(withItems(3), withSize(120, 40)), "tab")
	if f := footerText(wide.viewString(), 1); !strings.Contains(f, "t sidebar") {
		t.Errorf("120 columns, pane focused: footer %q, want t sidebar", f)
	}
	sheet := sendKey(newTestModel(withItems(3), withSize(80, 24)), "tab")
	if f := stripAnsi(sheet.viewString()); strings.Contains(f, "t sidebar") {
		t.Errorf("80 columns, sheet: t sidebar offered where it does nothing")
	}
}

func TestSidebar_HelpEntry(t *testing.T) {
	for _, sec := range helpSectionsDashboard {
		for _, e := range sec.items {
			if e.key == "t" && e.desc == "Show or hide the sidebar" {
				return
			}
		}
	}
	t.Error("help has no t entry")
}

// The list is sized for the dashboard footer, two lines at 110 columns; a
// one-line footer (pane focused, filter typing) must not leave a gap in the
// divider above it.
func TestSidebar_DividerRunsToTheFooter(t *testing.T) {
	for _, tc := range []struct {
		name    string
		step    func(Model) Model
		footerH int
	}{
		{"list", func(m Model) Model { return m }, 2},
		{"pane focused", func(m Model) Model { return sendKey(m, "tab") }, 1},
		{"filter typing", func(m Model) Model { return sendKey(m, "/") }, 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m := tc.step(newTestModel(withItems(5), withSize(110, 40)))
			col := 1 + m.list.Width() + 1
			lines := screenLines(t, m)
			for i := 1; i < len(lines)-tc.footerH; i++ {
				if c := ansi.Cut(lines[i], col, col+1); c != "│" {
					t.Errorf("line %d: %q at column %d, want the divider: %q", i, c, col, lines[i])
				}
			}
			if f := lines[len(lines)-tc.footerH]; !strings.HasPrefix(f, "  ") || strings.Contains(f, "│") {
				t.Errorf("line %d should be the footer: %q", len(lines)-tc.footerH, f)
			}
		})
	}
}
