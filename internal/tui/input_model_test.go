package tui

import (
	"strings"
	"testing"

	"charm.land/bubbles/v2/list"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/exp/golden"
)

// The input model (docs/TUI.md, Input model): j/k move; while you type,
// hold ctrl. These tests pin the rule on every screen that has text entry.

// typeText sends text one key at a time.
func typeText(m Model, text string) Model {
	for _, r := range text {
		m = sendKey(m, string(r))
	}
	return m
}

// sendKeyRun sends a key and feeds back what its commands return at once,
// such as the list's filter matches.
func sendKeyRun(m Model, k string) Model {
	next, cmd := m.Update(makeKeyMsg(k))
	m = next.(Model)
	queue := runCmd(cmd)
	for n := 0; len(queue) > 0 && n < 50; n++ {
		next, cmd = m.Update(queue[0])
		m = next.(Model)
		queue = append(queue[1:], runCmd(cmd)...)
	}
	return m
}

// footerText is the view's footer: the last lines, without ANSI, trimmed.
func footerText(view string, lines int) string {
	all := strings.Split(stripAnsi(view), "\n")
	tail := all[max(0, len(all)-lines):]
	for i := range tail {
		tail[i] = strings.TrimSpace(tail[i])
	}
	return strings.Join(tail, "\n")
}

// --- Filter: enter and tab keep it; nothing moves rows while typing ---

func TestFilter_EnterAndTabKeepTheFilter(t *testing.T) {
	for _, k := range []string{"enter", "tab", "ctrl+m", "ctrl+i"} {
		m := newTestModel(withItems(5), withSize(80, 24))
		m = typeText(sendKey(m, "/"), "fea")
		m = sendKey(m, k)
		if m.list.FilterState() != list.FilterApplied || m.list.FilterValue() != "fea" {
			t.Errorf("%s: filter state %v value %q, want applied \"fea\"", k, m.list.FilterState(), m.list.FilterValue())
		}
		if m.activeView() != ViewDashboard || m.detailFocused {
			t.Errorf("%s: left the list (view %v, detail %v)", k, m.activeView(), m.detailFocused)
		}
	}
}

func TestFilter_EnterOnEmptyFilterReturnsToTheList(t *testing.T) {
	m := newTestModel(withItems(5), withSize(80, 24))
	m = sendKey(sendKey(m, "/"), "enter")
	if m.list.FilterState() != list.Unfiltered {
		t.Errorf("enter on an empty filter: state %v, want unfiltered", m.list.FilterState())
	}
}

func TestFilter_MoveKeysDoNothingWhileTyping(t *testing.T) {
	for _, k := range []string{"up", "down", "ctrl+j", "ctrl+k", "ctrl+n", "ctrl+p"} {
		m := newTestModel(withItems(5), withSize(80, 24))
		m = typeText(sendKey(m, "/"), "fea")
		m = sendKey(m, "left")
		m = sendKey(m, k)
		if m.list.FilterState() != list.Filtering || m.list.FilterValue() != "fea" {
			t.Errorf("%s while typing: state %v value %q, want still typing \"fea\"", k, m.list.FilterState(), m.list.FilterValue())
		}
	}
}

func TestFilter_EscWhileTypingClearsAndLeaves(t *testing.T) {
	for _, k := range []string{"esc", "ctrl+["} {
		m := newTestModel(withItems(5), withSize(80, 24))
		m = sendKey(typeText(sendKey(m, "/"), "fea"), k)
		if m.list.FilterState() != list.Unfiltered || m.list.FilterValue() != "" {
			t.Errorf("%s while typing: state %v value %q", k, m.list.FilterState(), m.list.FilterValue())
		}
	}
}

func TestFilter_EscOnTheListClearsAndSlashReopens(t *testing.T) {
	m := newTestModel(withItems(5), withSize(80, 24))
	m = sendKey(typeText(sendKey(m, "/"), "fea"), "enter")
	m = sendKey(m, "/")
	if m.list.FilterState() != list.Filtering || m.list.FilterValue() != "fea" {
		t.Errorf("/ with a filter applied: state %v value %q, want typing \"fea\"", m.list.FilterState(), m.list.FilterValue())
	}
	m = sendKey(m, "tab")
	next, cmd := pressKey(m, "ctrl+[")
	if next.list.FilterState() != list.Unfiltered || isQuit(cmd) {
		t.Errorf("ctrl+[ with a filter applied: state %v quit %v, want the filter cleared", next.list.FilterState(), isQuit(cmd))
	}
}

// browserFilter drives a PR or issue browser's filter.
type browserFilter struct {
	name  string
	open  testOpt
	state func(Model) (input string, filtering bool, open bool)
}

func browserFilters() []browserFilter {
	return []browserFilter{
		{"prs", withPRData(), func(m Model) (string, bool, bool) {
			if m.prState == nil || m.activeView() != ViewPRs {
				return "", false, false
			}
			return m.prState.FilterInput.Value(), m.prState.Filtering, true
		}},
		{"issues", withIssueData(), func(m Model) (string, bool, bool) {
			if m.issueState == nil || m.activeView() != ViewIssues {
				return "", false, false
			}
			return m.issueState.FilterInput.Value(), m.issueState.Filtering, true
		}},
	}
}

func TestBrowserFilter_EnterAndTabKeep_EscClearsThenCloses(t *testing.T) {
	for _, b := range browserFilters() {
		for _, k := range []string{"enter", "tab", "ctrl+m", "ctrl+i"} {
			m := newTestModel(withItems(3), withSize(120, 40), b.open)
			m = sendKey(typeText(sendKey(m, "/"), "a"), k)
			if v, typing, open := b.state(m); v != "a" || typing || !open {
				t.Errorf("%s %s: filter %q typing %v open %v, want \"a\" kept on the list", b.name, k, v, typing, open)
			}
		}
		m := newTestModel(withItems(3), withSize(120, 40), b.open)
		m = sendKey(typeText(sendKey(m, "/"), "a"), "enter")
		m = sendKey(m, "esc")
		if v, _, open := b.state(m); v != "" || !open {
			t.Errorf("%s: esc with a filter applied: filter %q open %v, want cleared and still open", b.name, v, open)
		}
		if _, _, open := b.state(sendKey(m, "esc")); open {
			t.Errorf("%s: a second esc should close the browser", b.name)
		}
	}
}

func TestBrowserFilter_MoveKeysDoNothingWhileTyping(t *testing.T) {
	for _, b := range browserFilters() {
		for _, k := range []string{"up", "down", "ctrl+j", "ctrl+k", "ctrl+n", "ctrl+p"} {
			m := newTestModel(withItems(3), withSize(120, 40), b.open)
			m = sendKey(sendKey(typeText(sendKey(m, "/"), "ab"), "left"), k)
			if v, typing, _ := b.state(m); v != "ab" || !typing {
				t.Errorf("%s %s while typing: filter %q typing %v", b.name, k, v, typing)
			}
		}
	}
}

// --- Leaving and quitting at the root ---

func TestRoot_EscNeverQuits(t *testing.T) {
	for _, k := range []string{"esc", "ctrl+["} {
		m := newTestModel(withItems(3), withSize(80, 24))
		if _, cmd := pressKey(m, k); isQuit(cmd) {
			t.Errorf("%s at the root quit", k)
		}
	}
}

func TestRoot_QAndCtrlCQuit(t *testing.T) {
	for _, k := range []string{"q", "ctrl+c"} {
		m := newTestModel(withItems(3), withSize(80, 24))
		if _, cmd := pressKey(m, k); !isQuit(cmd) {
			t.Errorf("%s at the root did not quit", k)
		}
	}
}

// --- ctrl+k never deletes text ---

func TestCtrlK_NeverDeletesText(t *testing.T) {
	type input struct {
		name  string
		open  func(Model) Model
		value func(Model) string
	}
	inputs := []input{
		{"dashboard filter", func(m Model) Model { return sendKey(m, "/") }, func(m Model) string { return m.list.FilterValue() }},
		{"pr filter", func(m Model) Model { withPRData()(&m); return sendKey(m, "/") }, func(m Model) string { return m.prState.FilterInput.Value() }},
		{"issue filter", func(m Model) Model { withIssueData()(&m); return sendKey(m, "/") }, func(m Model) string { return m.issueState.FilterInput.Value() }},
		{"rename", func(m Model) Model { m.list.Select(1); return sendKey(m, "R") }, func(m Model) string { return m.renameState.Input.Value() }},
		{"find", func(m Model) Model { return sendKey(m, "space") }, func(m Model) string { return m.omni.Input.Value() }},
		{"branch form", func(m Model) Model {
			withForm(func() *BranchFormState { return newBranchForm(testFormOptions()) })(&m)
			return m
		},
			func(m Model) string { return m.branchForm.Branch.Value() }},
		{"settings edit", func(m Model) Model {
			withConfigOverlay()(&m)
			m.configState.Tab = 0
			return sendKey(m, "enter")
		}, func(m Model) string { return m.configState.input.Value() }},
	}
	for _, in := range inputs {
		m := in.open(newTestModel(withItems(3), withSize(120, 40)))
		if in.name == "settings edit" {
			m.configState.input.SetValue("")
		}
		m = sendKey(sendKey(typeText(m, "abc"), "left"), "ctrl+k")
		if got := in.value(m); got != "abc" {
			t.Errorf("%s: ctrl+k left %q, want \"abc\"", in.name, got)
		}
	}
}

// --- ctrl+[, ctrl+m, ctrl+i arrive apart under the kitty protocol ---

func TestAltKeys_CtrlBracketLeavesLikeEsc(t *testing.T) {
	opens := map[string]func(Model) Model{
		"find":        func(m Model) Model { return sendKey(m, "space") },
		"branch form": func(m Model) Model { return sendKey(m, "n") },
		"settings":    func(m Model) Model { withConfigOverlay()(&m); return m },
		"prs":         func(m Model) Model { withPRData()(&m); return m },
		"rename":      func(m Model) Model { m.list.Select(1); return sendKey(m, "R") },
		"detail":      func(m Model) Model { return sendKey(m, "tab") },
	}
	for name, open := range opens {
		m := open(newTestModel(withItems(3), withSize(120, 40)))
		if m.activeView() == ViewDashboard && !m.detailFocused {
			t.Fatalf("%s did not open", name)
		}
		m = sendKey(m, "ctrl+[")
		if m.activeView() != ViewDashboard || m.detailFocused {
			t.Errorf("%s: ctrl+[ left view %v (detail focused %v)", name, m.activeView(), m.detailFocused)
		}
	}
}

func TestAltKeys_CtrlMIsEnterAndCtrlIIsTab(t *testing.T) {
	m := newTestModel(withItems(3), withSize(120, 40))
	m.list.Select(1)
	if next, cmd := pressKey(m, "ctrl+m"); next.switchTo == "" || !isQuit(cmd) {
		t.Errorf("ctrl+m on the list: switchTo %q, want a switch", next.switchTo)
	}
	if next := sendKey(m, "ctrl+i"); !next.detailFocused {
		t.Error("ctrl+i on the list did not focus the detail pane")
	}

	s := newBranchForm(testFormOptions())
	typeKeys(s, "abc-12")
	press(s, "ctrl+i")
	if s.Focus != fieldBase {
		t.Errorf("ctrl+i in the form: focus %v, want Base", s.Focus)
	}
	if res := press(s, "ctrl+m"); res != formSubmit {
		t.Errorf("ctrl+m in the form: %v, want submit", res)
	}
}

// --- Finders: all three move sets, everywhere a finder exists ---

func TestFind_AllMoveSetsAndTabPick(t *testing.T) {
	for _, pair := range [][2]string{{"down", "up"}, {"ctrl+j", "ctrl+k"}, {"ctrl+n", "ctrl+p"}, {"tab", "shift+tab"}, {"ctrl+i", "shift+tab"}} {
		m := sendKey(newTestModel(withItems(5), withSize(120, 40)), "space")
		if len(m.omni.rows()) < 2 {
			t.Fatalf("find needs two rows, has %d", len(m.omni.rows()))
		}
		m = sendKey(m, pair[0])
		if m.omni.Cursor != 1 {
			t.Errorf("%s in find: cursor %d, want 1", pair[0], m.omni.Cursor)
		}
		if m = sendKey(m, pair[1]); m.omni.Cursor != 0 {
			t.Errorf("%s in find: cursor %d, want 0", pair[1], m.omni.Cursor)
		}
		if m.omni.Input.Value() != "" {
			t.Errorf("%s typed %q into find", pair[0], m.omni.Input.Value())
		}
	}
}

func TestBranchForm_AllMoveSetsPick(t *testing.T) {
	for _, pair := range [][2]string{{"down", "up"}, {"ctrl+j", "ctrl+k"}, {"ctrl+n", "ctrl+p"}} {
		s := newBranchForm(testFormOptions())
		typeKeys(s, "f")
		start := s.Pick
		if start < 1 {
			t.Fatalf("the default pick %d leaves no row above it", start)
		}
		press(s, pair[1])
		if s.Pick != start-1 {
			t.Errorf("%s in the form: pick %d, want %d", pair[1], s.Pick, start-1)
		}
		if press(s, pair[0]); s.Pick != start {
			t.Errorf("%s in the form: pick %d, want %d", pair[0], s.Pick, start)
		}
		if s.Branch.Value() != "f" {
			t.Errorf("%s typed into Branch: %q", pair[0], s.Branch.Value())
		}
	}
}

func TestBranchForm_ChangesTakesHAndL(t *testing.T) {
	s := newCopyForm(testFormOptions(), testSource())
	s.setWIP("/r/auth", true, []string{"a.go"}, nil)
	s.setFocus(fieldChanges)
	first := s.Changes
	press(s, "l")
	if s.Changes == first {
		t.Error("l on Changes did not change it")
	}
	press(s, "h")
	if s.Changes != first {
		t.Errorf("h on Changes: %v, want %v", s.Changes, first)
	}
}

// --- Settings: sections on 1-4 and H/L; finders own ctrl+n/p ---

func TestSettings_HAndLSwitchSections(t *testing.T) {
	m := newTestModel(withItems(3), withSize(80, 24), withConfigOverlay())
	m = sendKey(m, "L")
	if m.configState.Tab != 1 {
		t.Errorf("L: section %d, want 1", m.configState.Tab)
	}
	m = sendKey(sendKey(m, "H"), "H")
	if m.configState.Tab != ConfigTabCount-1 {
		t.Errorf("H twice from 1: section %d, want %d", m.configState.Tab, ConfigTabCount-1)
	}
	for _, k := range []string{"ctrl+n", "ctrl+p"} {
		before := m.configState.Tab
		if m = sendKey(m, k); m.configState.Tab != before {
			t.Errorf("%s switched the section", k)
		}
	}
}

// --- Focus: one cursor arrow on screen ---

func TestFocus_OneCursorArrowWithTheFormOpen(t *testing.T) {
	for _, sz := range []termSize{sizeStandard, sizeWide, {"narrow_44x24", 44, 24}} {
		for _, below := range []string{"dashboard", "prs"} {
			m := newTestModel(withItems(5), withSize(sz.width, sz.height))
			if below == "prs" {
				withPRData()(&m)
			}
			withForm(formCaseNamed(t, "new_branch"))(&m)
			if n := strings.Count(stripAnsi(m.viewString()), "❯"); n != 1 {
				t.Errorf("%s over %s: %d cursor arrows, want 1\n%s", sz.name, below, n, stripAnsi(m.viewString()))
			}
		}
	}
}

// --- Filter row: visible while applied, on all three lists ---

func TestFilterRow_StaysVisibleWhileApplied(t *testing.T) {
	m := newTestModel(withItems(5), withSize(80, 24))
	for _, k := range []string{"/", "a", "u", "t", "h", "enter"} {
		m = sendKeyRun(m, k)
	}
	view := stripAnsi(m.viewString())
	for _, want := range []string{"Filter: auth  1 of 5 · esc clear", "1/1"} {
		if !strings.Contains(view, want) {
			t.Errorf("dashboard with a filter applied lacks %q:\n%s", want, view)
		}
	}
	for _, b := range browserFilters() {
		m := newTestModel(withItems(3), withSize(120, 40), b.open)
		m = sendKey(typeText(sendKey(m, "/"), "a"), "enter")
		view := stripAnsi(m.viewString())
		for _, want := range []string{"Filter: a", " of 3", "esc clear"} {
			if !strings.Contains(view, want) {
				t.Errorf("%s with a filter applied lacks %q:\n%s", b.name, want, view)
			}
		}
	}
}

// The filter row is the list's first row on the dashboard, so the list does
// not move when a filter is typed or applied.
func TestFilterRow_KeepsTheListStill(t *testing.T) {
	m := newTestModel(withItems(5), withSize(80, 24))
	rowOf := func(m Model) int { return lineIndex(strings.Split(stripAnsi(m.viewString()), "\n"), "feature-auth") }
	before := rowOf(m)
	typing := typeText(sendKey(m, "/"), "e")
	if got := rowOf(typing); got != before {
		t.Errorf("typing moved the list: feature-auth on row %d, was %d", got, before)
	}
	if got := rowOf(sendKey(typing, "enter")); got != before {
		t.Errorf("applying moved the list: feature-auth on row %d, was %d", got, before)
	}
}

// --- Footers: one format, mode word first ---

func TestFooter_ModeWordFirst(t *testing.T) {
	tests := []struct {
		name string
		m    func() Model
		want string
	}{
		{"dashboard", func() Model { return newTestModel(withItems(5), withSize(160, 40)) },
			"move · ↑↓/jk move · enter switch · space go to · tab detail · / filter · v detailed · t sidebar · g more · ? help · q quit"},
		{"dashboard typing a filter", func() Model {
			return typeText(sendKey(newTestModel(withItems(5), withSize(160, 40)), "/"), "fe")
		}, "type · enter/tab keep · esc clear"},
		{"detail focused", func() Model { return sendKey(newTestModel(withItems(5), withSize(160, 40)), "tab") },
			"move · ↑↓/jk scroll · g/G top/bottom · t sidebar · esc/tab back"},
		{"prs", func() Model { return newTestModel(withItems(3), withSize(160, 40), withPRData()) },
			"move · ↑↓/jk move · enter review · n new worktree · B open in browser · tab detail · / filter · esc close"},
		{"prs filter applied", func() Model {
			m := newTestModel(withItems(3), withSize(160, 40), withPRData())
			return sendKey(typeText(sendKey(m, "/"), "a"), "enter")
		}, "move · ↑↓/jk move · enter review · n new worktree · B open in browser · tab detail · / filter · esc clear"},
		{"issues", func() Model { return newTestModel(withItems(3), withSize(160, 40), withIssueData()) },
			"move · ↑↓/jk move · enter new worktree · B open in browser · tab detail · / filter · esc close"},
		{"settings", func() Model { return newTestModel(withItems(3), withSize(160, 40), withConfigOverlay()) },
			"move · ↑↓/jk move · enter edit · saves as you go · esc close"},
	}
	for _, tt := range tests {
		if got := footerText(tt.m().viewString(), 1); got != tt.want {
			t.Errorf("%s footer:\n got %q\nwant %q", tt.name, got, tt.want)
		}
	}
}

func TestFooter_FormAndFindKeyLines(t *testing.T) {
	s := newBranchForm(testFormOptions())
	typeKeys(s, "abc-12")
	lay := branchFormLayout(160, 50, 94)
	lines := panelLines(s, lay)
	if got := strings.TrimSpace(lines[len(lines)-1]); got != "type · ↑↓/ctrl+jk pick · tab next · enter create · esc cancel" {
		t.Errorf("form key line on Branch: %q", got)
	}
	s.setFocus(fieldAlias)
	lines = panelLines(s, lay)
	if got := strings.TrimSpace(lines[len(lines)-1]); got != "type · tab next · enter create · esc cancel" {
		t.Errorf("form key line on Name: %q", got)
	}

	c := newCopyForm(testFormOptions(), testSource())
	c.setWIP("/r/auth", true, []string{"a.go"}, nil)
	c.setFocus(fieldChanges)
	lines = panelLines(c, lay)
	if got := strings.TrimSpace(lines[len(lines)-1]); got != "move · ←→/hl change · tab next · enter copy · esc cancel" {
		t.Errorf("form key line on Changes: %q", got)
	}

	m := sendKey(newTestModel(withItems(5), withSize(120, 40)), "space")
	if view := stripAnsi(m.viewString()); !strings.Contains(view, "type · ↑↓/ctrl+jk pick · enter choose · esc close") {
		t.Errorf("find key line missing:\n%s", view)
	}
}

// Every footer fits at 44 and 80 columns, keeps its mode word first, and
// keeps its leave key.
func TestFooter_FitsAt44And80(t *testing.T) {
	type screen struct {
		name  string
		open  func(Model) Model
		mode  string
		leave string
	}
	screens := []screen{
		{"dashboard", func(m Model) Model { return m }, "move", "q quit"},
		{"filter", func(m Model) Model { return typeText(sendKey(m, "/"), "f") }, "type", "esc clear"},
		{"prs", func(m Model) Model { withPRData()(&m); return m }, "move", "esc close"},
		{"issues", func(m Model) Model { withIssueData()(&m); return m }, "move", "esc close"},
		{"settings", func(m Model) Model { withConfigOverlay()(&m); return m }, "move", "esc close"},
		{"form", func(m Model) Model { return sendKey(m, "n") }, "type", "esc cancel"},
		{"find", func(m Model) Model { return sendKey(m, "space") }, "type", "esc close"},
		{"rename", func(m Model) Model { m.list.Select(1); return sendKey(m, "R") }, "type", "esc cancel"},
	}
	for _, w := range []int{44, 80} {
		for _, sc := range screens {
			m := sc.open(newTestModel(withItems(5), withSize(w, 24)))
			view := m.viewString()
			lines := strings.Split(view, "\n")
			for _, l := range lines[max(0, len(lines)-2):] {
				if !strings.Contains(stripAnsi(l), " · ") {
					continue // not a key line
				}
				if lw := lipgloss.Width(strings.TrimRight(stripAnsi(l), " ")); lw > w {
					t.Errorf("%s at %d: footer line is %d wide: %q", sc.name, w, lw, stripAnsi(l))
				}
			}
			plain := stripAnsi(view)
			if !strings.Contains(plain, sc.mode+" · ") || !strings.Contains(plain, sc.leave) {
				t.Errorf("%s at %d: footer lacks %q first or %q:\n%s", sc.name, w, sc.mode, sc.leave, plain)
			}
		}
	}
}

// --- A worktree created under a filter is selected ---

func TestCreate_UnderAFilterSelectsTheNewRow(t *testing.T) {
	fetched := func(names ...string) worktreesFetchedMsg {
		items := makeTestItems(5)
		for _, n := range names {
			items = append(items, WorktreeItem{ShortName: n, Branch: n, Path: "/tmp/test-project-" + n})
		}
		return worktreesFetchedMsg{items: items}
	}
	t.Run("hidden by the filter", func(t *testing.T) {
		m := newTestModel(withItems(5), withSize(80, 24))
		m.list.SetFilterText("fea")
		m.pendingSelect = "zeta"
		m = sendMsg(m, fetched("zeta"))
		if sel, ok := m.selectedItem(); !ok || sel.ShortName != "zeta" {
			t.Errorf("selected %q, want zeta", selectedName(m))
		}
		if m.list.FilterState() != list.Unfiltered {
			t.Errorf("filter %v, want cleared so the new row shows", m.list.FilterState())
		}
	})
	t.Run("matching the filter", func(t *testing.T) {
		m := newTestModel(withItems(5), withSize(80, 24))
		m.list.SetFilterText("fea")
		m.pendingSelect = "feature-new"
		m = sendMsg(m, fetched("feature-new"))
		if sel, ok := m.selectedItem(); !ok || sel.ShortName != "feature-new" {
			t.Errorf("selected %q, want feature-new", selectedName(m))
		}
		if m.list.FilterState() != list.FilterApplied || m.list.FilterValue() != "fea" {
			t.Errorf("filter %v %q, want kept", m.list.FilterState(), m.list.FilterValue())
		}
	})
}

// --- Goldens for the input model's states ---

func TestGolden_InputModel(t *testing.T) {
	applied := func(m *Model) {
		m.list.SetFilterText("auth")
		m.updateDetailContent()
	}
	typing := func(m *Model) {
		applied(m)
		m.list.SetFilterState(list.Filtering)
	}
	prApplied := func(m *Model) { m.prState.FilterInput.SetValue("auth") }
	cases := []struct {
		name string
		size termSize
		opts []testOpt
	}{
		{"dashboard_filter_applied_80x24", sizeStandard, []testOpt{applied}},
		{"dashboard_filter_applied_120x40", sizeWide, []testOpt{applied}},
		{"dashboard_filter_typing_80x24", sizeStandard, []testOpt{typing}},
		{"dashboard_filter_typing_120x40", sizeWide, []testOpt{typing}},
		{"prs_filter_applied_80x24", sizeStandard, []testOpt{withPRData(), prApplied}},
		{"prs_filter_applied_120x40", sizeWide, []testOpt{withPRData(), prApplied}},
		{"form_over_filter_80x24", sizeStandard, []testOpt{applied, withForm(formCaseNamed(t, "new_branch"))}},
		{"form_over_filter_120x40", sizeWide, []testOpt{applied, withForm(formCaseNamed(t, "new_branch"))}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			m := goldenModel(t, c.size, append([]testOpt{withItems(5)}, c.opts...)...)
			golden.RequireEqual(t, []byte(m.viewString()))
		})
	}
}

// The status line stays on its row as focus moves and the changes check
// arrives, in the right pane where the key line takes two rows.
func TestBranchForm_StatusLineStaysPut(t *testing.T) {
	lay := branchFormLayout(120, 40, 70)
	statusRow := func(s *BranchFormState) int {
		lines := panelLines(s, lay)
		for i, l := range lines {
			if strings.Contains(l, "✓ ready") || strings.Contains(l, "checking") || strings.Contains(l, "type a branch") {
				return i
			}
		}
		return -1
	}
	n := newBranchForm(testFormOptions())
	typeKeys(n, "abc-12")
	first := statusRow(n)
	n.setFocus(fieldAlias)
	if got := statusRow(n); got != first || first < 0 {
		t.Errorf("new: status on row %d on Branch, %d on Name", first, got)
	}
	c := newCopyForm(testFormOptions(), testSource())
	before := len(panelLines(c, lay)) - 3
	c.setWIP("/r/auth", true, []string{"a.go"}, nil)
	c.setFocus(fieldChanges)
	if got := statusRow(c); got != before {
		t.Errorf("copy: status on row %d after the check, want %d", got, before)
	}
}

// A finder keeps its pick keys on screen at the 44-column floor: the key
// line takes two rows there instead of dropping them.
func TestFinders_KeepPickAt44(t *testing.T) {
	for name, open := range map[string]func(Model) Model{
		"find": func(m Model) Model { return sendKey(m, "space") },
		"form": func(m Model) Model { return sendKey(m, "n") },
	} {
		m := open(newTestModel(withItems(5), withSize(44, 24)))
		if view := stripAnsi(m.viewString()); !strings.Contains(view, "↑↓/ctrl+jk pick") {
			t.Errorf("%s at 44 dropped the pick keys:\n%s", name, view)
		}
	}
}

// The space key is spelled "space" everywhere: the ␣ glyph read as nothing.
func TestUI_NoSpaceGlyph(t *testing.T) {
	const glyph = "␣"
	var screens []string
	for _, w := range []int{44, 80, 160} {
		m := newTestModel(withItems(5), withSize(w, 24))
		screens = append(screens, m.viewString())
		g := m
		g.pendingG = true
		g.updateLayout()
		screens = append(screens, g.viewString())
		screens = append(screens, sendKey(m, "space").viewString(), sendKey(m, "?").viewString())
	}
	for _, s := range screens {
		if strings.Contains(s, glyph) {
			t.Errorf("screen shows %s:\n%s", glyph, stripAnsi(s))
		}
	}
	for _, a := range dashboardActions() {
		if strings.Contains(a.display()+a.Label+a.Short, glyph) {
			t.Errorf("action %s shows %s", a.ID, glyph)
		}
	}
	for _, b := range DefaultKeyMap().ShortHelp() {
		if strings.Contains(b.Help().Key+b.Help().Desc, glyph) {
			t.Errorf("key help %q shows %s", b.Help().Key, glyph)
		}
	}
	if strings.Contains(seeMessages, glyph) {
		t.Errorf("toast pointer %q shows %s", seeMessages, glyph)
	}
}

// The g strip is "more": "go to" names the space screen only.
func TestUI_GStripIsMore(t *testing.T) {
	m := newTestModel(withItems(5), withSize(160, 40))
	if f := footerText(m.viewString(), 1); !strings.Contains(f, "g more") || strings.Contains(f, "goto") {
		t.Errorf("footer %q, want g more", f)
	}
	m.pendingG = true
	m.updateLayout()
	if strip := stripAnsi(strings.Join(m.gStripLines(), "\n")); !strings.HasPrefix(strings.TrimSpace(strip), "g more") {
		t.Errorf("strip %q, want it titled g more", strip)
	}
	found := false
	for _, sec := range helpSectionsDashboard {
		for _, e := range sec.items {
			if strings.Contains(strings.ToLower(e.desc), "goto") {
				t.Errorf("help entry %q still says goto", e.desc)
			}
			found = found || (e.key == "g" && strings.HasPrefix(e.desc, "More:"))
		}
	}
	if !found {
		t.Error("help has no g entry starting More:")
	}
}
