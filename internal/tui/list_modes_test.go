package tui

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"charm.land/bubbles/v2/list"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
)

// withNamedItems seeds n worktrees with distinct names, wt-00 to wt-NN.
func withNamedItems(n int) testOpt {
	return func(m *Model) {
		li := make([]list.Item, n)
		for i := range n {
			name := fmt.Sprintf("wt-%02d", i)
			li[i] = WorktreeItem{
				ShortName: name, Path: "/tmp/test-project-" + name, Branch: "feat/" + name,
				IsMain: i == 0, CommitTime: testNow.Add(-time.Duration(i) * time.Hour), TmuxStatus: "none",
			}
		}
		m.list.SetItems(li)
		m.updateLayout()
	}
}

var listModeSizes = []struct{ w, h int }{{44, 24}, {80, 24}, {80, 15}, {120, 40}, {160, 50}}

func eachListMode(t *testing.T, fn func(t *testing.T, m Model)) {
	t.Helper()
	for _, size := range listModeSizes {
		for _, mode := range []struct {
			name string
			opt  testOpt
		}{{"compact", withCompact()}, {"detailed", withDetailed()}} {
			t.Run(fmt.Sprintf("%s_%dx%d", mode.name, size.w, size.h), func(t *testing.T) {
				fn(t, newTestModel(withNamedItems(30), withSize(size.w, size.h), mode.opt))
			})
		}
	}
}

// cursorLine is the list line with ❯, or "".
func cursorLine(view string) string {
	for _, l := range strings.Split(stripAnsi(view), "\n") {
		if strings.Contains(l, "❯ ") {
			return l
		}
	}
	return ""
}

// The heading row and the gap between detailed rows are counted: the screen
// is exactly the terminal's height and width, with the heading under the
// filter row.
func TestListMode_FitsTheScreen(t *testing.T) {
	eachListMode(t, func(t *testing.T, m Model) {
		lines := strings.Split(stripAnsi(m.viewString()), "\n")
		if len(lines) != m.height {
			t.Fatalf("%d lines, want %d:\n%s", len(lines), m.height, strings.Join(lines, "\n"))
		}
		for i, l := range lines {
			if w := lipgloss.Width(l); w > m.width {
				t.Errorf("line %d is %d cells: %q", i, w, l)
			}
		}
		footerH := m.helpFooter.CompactHeight(ViewDashboard, m.width-4)
		body := lines[1 : len(lines)-footerH]
		if isWideLayout(m.width) {
			// The divider with the pane runs the body's full height in one
			// column.
			col := 1 + m.list.Width() + 1 // body margin, list, divider margin
			for i, l := range body {
				if c := ansi.Cut(l, col, col+1); c != "│" {
					t.Errorf("body line %d: %q at column %d, want the divider: %q", i, c, col, l)
				}
			}
		} else if sel := body[len(body)-1]; !strings.HasPrefix(strings.TrimSpace(sel), "wt-00") {
			t.Errorf("the selection line should sit above the footer, got %q", sel)
		}
		heading := strings.Contains(lines[2], "NAME")
		if detailed := m.rowDelegate().Detailed; heading == detailed {
			t.Errorf("detailed %v, heading under the filter row %v: %q", detailed, heading, lines[2])
		}
		perRow := 1
		if m.rowDelegate().Detailed {
			perRow = 3
		}
		// A page of rows, with a detailed row's gap between rows, fits the
		// list's height.
		if p := m.list.Paginator.PerPage; p < 1 || p*perRow-(perRow-1) > m.list.Height() {
			t.Errorf("%d rows per page in a list %d lines high", p, m.list.Height())
		}
	})
}

// G, gg, ctrl+d, and ctrl+u keep the cursor row on screen and the n/total
// counter right.
func TestListMode_CursorStaysInView(t *testing.T) {
	eachListMode(t, func(t *testing.T, m Model) {
		check := func(step string, wantIdx int) {
			t.Helper()
			if got := m.list.Index(); got != wantIdx {
				t.Fatalf("%s: index %d, want %d", step, got, wantIdx)
			}
			view := m.viewString()
			name := fmt.Sprintf("wt-%02d", wantIdx)
			if !strings.Contains(cursorLine(view), name) {
				t.Errorf("%s: the cursor row %q does not show %s", step, cursorLine(view), name)
			}
			if !strings.Contains(strings.Split(stripAnsi(view), "\n")[0], fmt.Sprintf("%d/30", wantIdx+1)) {
				t.Errorf("%s: header %q, want %d/30", step, strings.Split(stripAnsi(view), "\n")[0], wantIdx+1)
			}
		}
		half := m.halfPage()
		perPage := m.list.Paginator.PerPage
		if half != max(1, perPage/2) {
			t.Errorf("half page %d, rows per page %d", half, perPage)
		}
		m = sendKey(m, "G")
		check("G", 29)
		m = sendKey(sendKey(m, "g"), "g")
		check("gg", 0)
		m = sendKey(m, "ctrl+d")
		check("ctrl+d", half)
		m = sendKey(m, "ctrl+d")
		check("ctrl+d twice", min(2*half, 29))
		m = sendKey(m, "ctrl+u")
		check("ctrl+u", min(2*half, 29)-half)
		at := m.list.Index()
		for range perPage {
			m = sendKey(m, "j")
		}
		check("a page of j", min(at+perPage, 29))
	})
}

// v at any size keeps the selected worktree selected and in view.
func TestListMode_ToggleKeepsSelection(t *testing.T) {
	eachListMode(t, func(t *testing.T, m Model) {
		m.projectRoot = t.TempDir()
		m.list.Select(25)
		m = sendKey(m, "v")
		if m.list.Index() != 25 || !strings.Contains(cursorLine(m.viewString()), "wt-25") {
			t.Errorf("after v: index %d, cursor row %q", m.list.Index(), cursorLine(m.viewString()))
		}
	})
}
