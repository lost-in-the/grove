package tui

import (
	"fmt"
	"strings"
	"testing"

	"charm.land/bubbles/v2/list"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/charmbracelet/x/exp/golden"
)

type formCase struct {
	name  string
	build func() *BranchFormState
}

func formCases() []formCase {
	return []formCase{
		{"empty", func() *BranchFormState { return newBranchForm(testFormOptions()) }},
		{"new_branch", func() *BranchFormState {
			s := newBranchForm(testFormOptions())
			s.Branch.SetValue("abc-12")
			s.syncAlias()
			return s
		}},
		{"base_focused", func() *BranchFormState {
			s := newBranchForm(testFormOptions())
			typeKeys(s, "abc-12")
			press(s, "tab")
			for range "main" {
				press(s, "backspace")
			}
			typeKeys(s, "f")
			return s
		}},
		{"existing_branch", func() *BranchFormState {
			s := newBranchForm(testFormOptions())
			s.Branch.SetValue("fix/login")
			s.syncAlias()
			return s
		}},
		{"in_use", func() *BranchFormState {
			s := newBranchForm(testFormOptions())
			s.Branch.SetValue("feature/auth")
			s.syncAlias()
			return s
		}},
		{"name_taken", func() *BranchFormState {
			s := newBranchForm(testFormOptions())
			typeKeys(s, "fix/auth")
			press(s, "tab")
			press(s, "tab")
			press(s, "enter")
			return s
		}},
		{"base_missing", func() *BranchFormState {
			s := newBranchForm(testFormOptions())
			typeKeys(s, "abc-12")
			press(s, "tab")
			for range "main" {
				press(s, "backspace")
			}
			typeKeys(s, "nope")
			press(s, "tab")
			return s
		}},
		{"error", func() *BranchFormState {
			s := newBranchForm(testFormOptions())
			s.Branch.SetValue("abc-12")
			s.syncAlias()
			s.Status = "failed to create worktree: something went wrong in git"
			return s
		}},
		{"creating", func() *BranchFormState {
			s := newBranchForm(testFormOptions())
			s.Branch.SetValue("abc-12")
			s.syncAlias()
			s.Busy = true
			s.Log = NewActivityLog(40, 6)
			s.Log.AddLine("Creating worktree 'abc-12'...")
			return s
		}},
		{"copy_checking", func() *BranchFormState { return newCopyForm(testFormOptions(), testSource()) }},
		{"copy_dirty", func() *BranchFormState {
			s := newCopyForm(testFormOptions(), testSource())
			s.setWIP("/r/auth", true, []string{"a.go", "b.go"}, nil)
			return s
		}},
		{"switch_clean", func() *BranchFormState {
			s := newSwitchForm(testFormOptions(), testSource())
			s.setWIP("/r/auth", false, nil, nil)
			return s
		}},
		{"switch_dirty", func() *BranchFormState {
			s := newSwitchForm(testFormOptions(), testSource())
			s.setWIP("/r/auth", true, []string{"a.go"}, nil)
			return s
		}},
	}
}

func formCaseNamed(t *testing.T, name string) func() *BranchFormState {
	t.Helper()
	for _, c := range formCases() {
		if c.name == name {
			return c.build
		}
	}
	t.Fatalf("no form case %q", name)
	return nil
}

// withForm opens s over the current top view, sized the way the Model
// sizes it on open.
func withForm(build func() *BranchFormState) testOpt {
	return func(m *Model) {
		s := build()
		s.setWidth(m.branchFormLayout().innerWidth())
		m.branchForm = s
		m.pushView(ViewBranchForm)
	}
}

// panelLines renders s for lay and returns its lines without ANSI.
func panelLines(s *BranchFormState, lay formLayout) []string {
	s.setWidth(lay.innerWidth())
	return strings.Split(ansi.Strip(renderBranchForm(s, lay, "⠋")), "\n")
}

func lineIndex(lines []string, sub string) int {
	for i, l := range lines {
		if strings.Contains(l, sub) {
			return i
		}
	}
	return -1
}

func TestBranchFormLayout_DockByTerminalSize(t *testing.T) {
	tests := []struct {
		w, h, listW int
		dock        formDock
		rows        int
	}{
		{120, 40, 70, dockRight, formPanelRows},
		{160, 50, 94, dockRight, formPanelRows},
		{110, 30, 64, dockRight, formPanelRows},
		{120, 10, 70, dockRight, 9}, // the pane never runs past the screen
		{109, 30, 107, dockBottom, formPanelRows},
		{80, 24, 78, dockBottom, formPanelRows},
		{44, 24, 42, dockBottom, formPanelRows},
		{80, 20, 78, dockBottom, formPanelRows},
		{80, 1 + formPanelRows + formMinListRows, 78, dockBottom, formPanelRows},
		{80, formPanelRows + formMinListRows, 78, dockFull, formPanelRows + formMinListRows},
		{80, 12, 78, dockFull, 12},
		{120, 40, 0, dockBottom, formPanelRows}, // no list beside it
	}
	for _, tt := range tests {
		lay := branchFormLayout(tt.w, tt.h, tt.listW)
		if lay.Dock != tt.dock || lay.Rows != tt.rows {
			t.Errorf("%dx%d list %d: dock %v rows %d, want dock %v rows %d", tt.w, tt.h, tt.listW, lay.Dock, lay.Rows, tt.dock, tt.rows)
		}
		wantW := tt.w - 2
		if tt.dock == dockRight {
			wantW = tt.w - 2 - tt.listW - 3
		}
		if lay.Width != wantW {
			t.Errorf("%dx%d list %d: width %d, want %d", tt.w, tt.h, tt.listW, lay.Width, wantW)
		}
	}
}

// The panel is exactly the layout's size in every state: typing, the
// suggestions coming and going, Base appearing, a long error, and progress.
func TestBranchForm_PanelSizeNeverChanges(t *testing.T) {
	layouts := []formLayout{
		branchFormLayout(120, 40, 70),
		branchFormLayout(160, 50, 94),
		branchFormLayout(80, 24, 78),
		branchFormLayout(44, 24, 42),
		branchFormLayout(80, 20, 78),
		branchFormLayout(80, 12, 78),
	}
	for _, lay := range layouts {
		for _, c := range formCases() {
			out := renderBranchForm(withInner(c.build(), lay), lay, "⠋")
			if w, h := lipgloss.Width(out), lipgloss.Height(out); w != lay.Width || h != lay.Rows {
				t.Errorf("%+v/%s: panel %dx%d, want %dx%d", lay, c.name, w, h, lay.Width, lay.Rows)
			}
		}
		s := withInner(newBranchForm(testFormOptions()), lay)
		for i, r := range "fix/login-x" {
			typeInto(t, s, string(r))
			if h := lipgloss.Height(renderBranchForm(s, lay, "⠋")); h != lay.Rows {
				t.Errorf("%+v: after %d keys the panel is %d rows, want %d", lay, i+1, h, lay.Rows)
			}
		}
	}
}

func withInner(s *BranchFormState, lay formLayout) *BranchFormState {
	s.setWidth(lay.innerWidth())
	return s
}

// Fields keep their rows as Base comes and goes: the Name row does not move
// when the typed branch turns out to exist.
func TestBranchForm_NameRowStaysPutWhenBaseHides(t *testing.T) {
	lay := branchFormLayout(120, 40, 70)
	s := newBranchForm(testFormOptions())
	typeInto(t, s, "fix/logi")
	before := lineIndex(panelLines(s, lay), "Name")
	typeInto(t, s, "n")
	after := panelLines(s, lay)
	if got := lineIndex(after, "Name"); got != before || got < 0 {
		t.Errorf("Name row moved from %d to %d when Base hid", before, got)
	}
}

func TestBranchForm_SuggestionsSitUnderTheFocusedField(t *testing.T) {
	lay := branchFormLayout(120, 40, 70)
	s := newBranchForm(testFormOptions())
	typeInto(t, s, "abc")

	lines := panelLines(s, lay)
	branch, base := lineIndex(lines, "Branch"), lineIndex(lines, "Base")
	if newRow := lineIndex(lines, `+ new "abc"`); newRow <= branch || newRow >= base {
		t.Errorf("Branch focused: + new row at %d, want between Branch (%d) and Base (%d):\n%s", newRow, branch, base, strings.Join(lines, "\n"))
	}
	if base != branch+1+formSuggestRows {
		t.Errorf("Base at %d, want right after Branch's %d suggestion rows (Branch %d)", base, formSuggestRows, branch)
	}

	press(s, "tab") // to Base, which suggests every branch for "main"
	lines = panelLines(s, lay)
	branch, base = lineIndex(lines, "Branch"), lineIndex(lines, "Base")
	if base != branch+1 {
		t.Errorf("Base focused: Base at %d, want directly under Branch (%d):\n%s", base, branch, strings.Join(lines, "\n"))
	}
	if !strings.Contains(lines[base+1], "main") {
		t.Errorf("Base focused: row under Base = %q, want its suggestion main", lines[base+1])
	}
	if name := lineIndex(lines, "Name"); name != base+1+formSuggestRows {
		t.Errorf("Name at %d, want after Base's %d suggestion rows (Base %d)", name, formSuggestRows, base)
	}
}

func TestBranchForm_ExistingBranchHasNoBaseRow(t *testing.T) {
	lay := branchFormLayout(80, 24, 78)
	s := newBranchForm(testFormOptions())
	typeInto(t, s, "fix/login")
	out := strings.Join(panelLines(s, lay), "\n")
	if strings.Contains(out, "Base") || strings.Contains(out, "n/a") {
		t.Errorf("existing branch still shows a Base row:\n%s", out)
	}
	press(s, "tab")
	if s.Focus != fieldAlias {
		t.Errorf("tab from Branch went to %v, want Name", s.Focus)
	}
	press(s, "tab")
	if s.Focus != fieldBranch {
		t.Errorf("tab from Name went to %v, want Branch (no Base to stop on)", s.Focus)
	}
}

func TestBranchForm_LabelSaysName(t *testing.T) {
	for _, build := range []func() *BranchFormState{
		func() *BranchFormState { return newBranchForm(testFormOptions()) },
		func() *BranchFormState { return newCopyForm(testFormOptions(), testSource()) },
	} {
		out := strings.Join(panelLines(build(), branchFormLayout(120, 40, 70)), "\n")
		if !strings.Contains(out, "Name") || strings.Contains(out, "Alias") {
			t.Errorf("want a Name label and no Alias:\n%s", out)
		}
	}
}

func TestBranchForm_PreviewShowsFolderAndSession(t *testing.T) {
	s := newBranchForm(testFormOptions())
	s.Branch.SetValue("fix/login")
	s.syncAlias()
	lines := panelLines(s, branchFormLayout(120, 40, 70))
	folder, session := lineIndex(lines, "folder grove-login"), lineIndex(lines, "session grove-login")
	if folder < 0 || session != folder+1 {
		t.Errorf("want folder and session on two lines:\n%s", strings.Join(lines, "\n"))
	}
	// Too short for the whole body: the two facts share one line.
	s.setFocus(fieldAlias)
	short := strings.Join(panelLines(s, branchFormLayout(80, 11, 78)), "\n")
	if !strings.Contains(short, "folder grove-login · session grove-login") {
		t.Errorf("short terminal: want one preview line:\n%s", short)
	}
}

func TestBranchForm_KeyLineIsNotOnTheEdge(t *testing.T) {
	out := strings.Join(panelLines(newBranchForm(testFormOptions()), branchFormLayout(80, 8, 78)), "\n")
	if !strings.Contains(out, "esc cancel") {
		t.Errorf("key line missing at 80x8:\n%s", out)
	}
}

func TestBranchForm_PlaceholderAndLongInputRender(t *testing.T) {
	lay := branchFormLayout(80, 24, 78)
	s := newBranchForm(testFormOptions())
	if out := strings.Join(panelLines(s, lay), "\n"); !strings.Contains(out, "type to search or create") {
		t.Errorf("placeholder missing:\n%s", out)
	}
	typeInto(t, s, strings.Repeat("x", 70)+"-tail")
	if out := strings.Join(panelLines(s, lay), "\n"); !strings.Contains(out, "x-tail") {
		t.Errorf("the end of a long branch name is not visible:\n%s", out)
	}
}

func TestCopyForm_RendersChangesChoice(t *testing.T) {
	s := newCopyForm(testFormOptions(), testSource())
	s.setWIP("/r/auth", true, []string{"a.go", "b.go"}, nil)
	out := strings.Join(panelLines(s, branchFormLayout(120, 40, 70)), "\n")
	for _, want := range []string{"Copy auth", "● copy", "○ move", "○ leave", "2 files", "From"} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q:\n%s", want, out)
		}
	}
}

func TestSwitchForm_RendersStashNote(t *testing.T) {
	s := newSwitchForm(testFormOptions(), testSource())
	s.setWIP("/r/auth", true, []string{"a.go"}, nil)
	out := strings.Join(panelLines(s, branchFormLayout(120, 40, 70)), "\n")
	for _, want := range []string{"Switch branch of auth", "stash 1 file before switching", "feature/auth"} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q:\n%s", want, out)
		}
	}
}

func TestCopyForm_ChangesRowFocusVisibleWhenShort(t *testing.T) {
	s := newCopyForm(testFormOptions(), testSource())
	s.setWIP("/r/auth", true, []string{"a.go"}, nil)
	s.setFocus(fieldChanges)
	out := strings.Join(panelLines(s, branchFormLayout(80, 9, 78)), "\n")
	if !strings.Contains(out, "● copy") {
		t.Errorf("focused Changes row scrolled out of view:\n%s", out)
	}
}

// uniqueItems makes n worktrees with distinct names, so a test can find
// any one row on screen.
func uniqueItems(n int) testOpt {
	return func(m *Model) {
		items := make([]list.Item, n)
		for i, it := range makeTestItems(n) {
			it.ShortName = fmt.Sprintf("wt-%02d", i)
			it.Branch = it.ShortName
			it.Path = "/tmp/test-project-" + it.ShortName
			items[i] = it
		}
		m.list.SetItems(items)
	}
}

func selectRow(i int) testOpt { return func(m *Model) { m.list.Select(i) } }

func screenLines(t *testing.T, m Model) []string {
	t.Helper()
	lines := strings.Split(ansi.Strip(m.viewString()), "\n")
	if len(lines) != m.height {
		t.Errorf("screen has %d lines, want %d", len(lines), m.height)
	}
	for i, l := range lines {
		if w := lipgloss.Width(l); w > m.width {
			t.Errorf("line %d is %d cells, wider than %d: %q", i, w, m.width, l)
		}
	}
	return lines
}

// cursorRow is the row of the list's selected worktree. With the form open
// the list draws no cursor arrow, so the row is found by its unique name.
func cursorRow(lines []string, name string) int { return lineIndex(lines, name) }

func TestDockedForm_WideKeepsListBesideIt(t *testing.T) {
	m := newTestModel(uniqueItems(8), withSize(120, 40), selectRow(3), withForm(formCaseNamed(t, "new_branch")))
	lines := screenLines(t, m)
	row := cursorRow(lines, "wt-03")
	if row < 0 {
		t.Fatalf("cursor row wt-03 not on screen:\n%s", strings.Join(lines, "\n"))
	}
	listW := m.list.Width()
	for _, name := range []string{"wt-00", "wt-07"} {
		i := lineIndex(lines, name)
		if i < 0 || strings.Index(lines[i], name) > listW {
			t.Errorf("%s not in the list column:\n%s", name, strings.Join(lines, "\n"))
		}
	}
	title := lineIndex(lines, "New worktree")
	if title < 0 || strings.Index(lines[title], "New worktree") < listW {
		t.Errorf("form title not in the right pane:\n%s", strings.Join(lines, "\n"))
	}
	if !strings.Contains(lines[title], "│") {
		t.Errorf("no divider between the list and the form on %q", lines[title])
	}
	if lineIndex(lines, "q quit") >= 0 {
		t.Errorf("the dashboard footer is still drawn under the form:\n%s", strings.Join(lines, "\n"))
	}
}

func TestDockedForm_NarrowKeepsListAboveIt(t *testing.T) {
	m := newTestModel(uniqueItems(5), withSize(80, 24), selectRow(2), withForm(formCaseNamed(t, "new_branch")))
	lines := screenLines(t, m)
	row, title := cursorRow(lines, "wt-02"), lineIndex(lines, "New worktree")
	if row < 0 || title < 0 || row > title {
		t.Fatalf("want the cursor row above the form (row %d, title %d):\n%s", row, title, strings.Join(lines, "\n"))
	}
	if title != m.height-formPanelRows {
		t.Errorf("form title on row %d, want %d (docked at the bottom)", title, m.height-formPanelRows)
	}
	if keys := strings.Join(lines[len(lines)-2:], "\n"); !strings.Contains(keys, "esc cancel") {
		t.Errorf("last lines = %q, want the form's key line", keys)
	}
	if n := strings.Count(strings.Join(lines, "\n"), "esc cancel"); n != 1 {
		t.Errorf("%d key lines on screen, want 1", n)
	}
}

// The list above the dock is shorter, so it scrolls to keep the cursor.
func TestDockedForm_NarrowKeepsCursorRowInView(t *testing.T) {
	for _, h := range []int{24, 20, 1 + formPanelRows + formMinListRows} {
		m := newTestModel(uniqueItems(30), withSize(80, h), selectRow(17), withForm(formCaseNamed(t, "new_branch")))
		lines := screenLines(t, m)
		if row := cursorRow(lines, "wt-17"); row < 0 || row >= m.height-formPanelRows {
			t.Errorf("80x%d: cursor row wt-17 not in the list above the form:\n%s", h, strings.Join(lines, "\n"))
		}
	}
}

func TestDockedForm_TooShortTakesTheScreen(t *testing.T) {
	h := formPanelRows + formMinListRows
	m := newTestModel(uniqueItems(5), withSize(80, h), withForm(formCaseNamed(t, "new_branch")))
	lines := screenLines(t, m)
	if !strings.Contains(lines[0], "New worktree") {
		t.Errorf("80x%d: want the form from the first row:\n%s", h, strings.Join(lines, "\n"))
	}
	if lineIndex(lines, "wt-00") >= 0 {
		t.Errorf("80x%d: no list rows expected:\n%s", h, strings.Join(lines, "\n"))
	}
}

func TestDockedForm_OverPRBrowser(t *testing.T) {
	for _, sz := range []termSize{{"wide", 120, 40}, {"narrow", 80, 24}} {
		m := newTestModel(withItems(3), withSize(sz.width, sz.height), withPRData(), withForm(formCaseNamed(t, "new_branch")))
		lines := screenLines(t, m)
		pr, title := lineIndex(lines, "#42"), lineIndex(lines, "New worktree")
		if pr < 0 || title < 0 {
			t.Fatalf("%s: want the PR list and the form:\n%s", sz.name, strings.Join(lines, "\n"))
		}
		if !strings.Contains(lines[0], "Pull Requests") {
			t.Errorf("%s: header = %q, want the PR browser's", sz.name, lines[0])
		}
		if sz.name == "wide" && strings.Index(lines[title], "New worktree") < contextListWidth(sz.width) {
			t.Errorf("wide: form not beside the PR list:\n%s", strings.Join(lines, "\n"))
		}
		if sz.name == "narrow" && pr > title {
			t.Errorf("narrow: PR list not above the form:\n%s", strings.Join(lines, "\n"))
		}
	}
}

func TestDockedForm_OverIssueBrowser(t *testing.T) {
	m := newTestModel(withItems(3), withSize(80, 24), withIssueData(), withForm(formCaseNamed(t, "new_branch")))
	lines := screenLines(t, m)
	if issue, title := lineIndex(lines, "#101"), lineIndex(lines, "New worktree"); issue < 0 || title < 0 || issue > title {
		t.Errorf("want the issue list above the form:\n%s", strings.Join(lines, "\n"))
	}
}

var (
	sizeTiny      = termSize{"narrow_44x24", 44, 24}
	sizeShort     = termSize{"short_80x20", 80, 20}
	sizeCramped   = termSize{"cramped_80x15", 80, 15}
	sizeWide160   = termSize{"ultrawide_160x50", 160, 50}
	goldenFormSet = []struct {
		size  termSize
		cases []string
	}{
		{sizeStandard, []string{"empty", "new_branch", "base_focused", "existing_branch", "in_use", "name_taken", "base_missing", "error", "creating", "copy_checking", "copy_dirty", "switch_clean", "switch_dirty"}},
		{sizeWide, []string{"empty", "new_branch", "base_focused", "existing_branch", "name_taken", "base_missing", "creating", "copy_dirty", "switch_dirty"}},
		{sizeWide160, []string{"new_branch"}},
		{sizeTiny, []string{"new_branch"}},
		{sizeShort, []string{"new_branch"}},
		{sizeCramped, []string{"new_branch"}},
	}
)

func TestGolden_BranchForm(t *testing.T) {
	for _, set := range goldenFormSet {
		for _, name := range set.cases {
			t.Run(set.size.name+"/"+name, func(t *testing.T) {
				m := goldenModel(t, set.size, withItems(5), withForm(formCaseNamed(t, name)))
				golden.RequireEqual(t, []byte(m.viewString()))
			})
		}
	}
}

func TestGolden_BranchForm_OverPRBrowser(t *testing.T) {
	for _, sz := range []termSize{sizeStandard, sizeWide} {
		t.Run(sz.name, func(t *testing.T) {
			m := goldenModel(t, sz, withItems(3), withPRData(), withForm(formCaseNamed(t, "new_branch")))
			golden.RequireEqual(t, []byte(m.viewString()))
		})
	}
}

// The help overlay and the footer hints describe the form as its key line
// does: every pick key, a Name field.
func TestBranchFormHelp_MatchesTheKeyLine(t *testing.T) {
	var text strings.Builder
	for _, sec := range helpSectionsBranchForm {
		text.WriteString(sec.note + "\n")
		for _, it := range sec.items {
			text.WriteString(it.key + " " + it.desc + "\n")
		}
	}
	for _, h := range NewHelpFooter().CompactFooter(ViewBranchForm).Hints {
		text.WriteString(h.Key + " " + h.Description + "\n")
	}
	got := text.String()
	if strings.Contains(strings.ToLower(got), "alias") {
		t.Errorf("help mentions an alias:\n%s", got)
	}
	for _, want := range []string{"↑/↓", "ctrl+j", "ctrl+n", "↑↓/ctrl+jk pick", "Name"} {
		if !strings.Contains(got, want) {
			t.Errorf("help is missing %q:\n%s", want, got)
		}
	}
}

// The key line fits the pane in its two rows, whole, from the 44-column
// floor to the 160-column right pane.
func TestBranchForm_KeyLineFitsThePane(t *testing.T) {
	s := newBranchForm(testFormOptions())
	two := "type · ↑↓/ctrl+jk pick\ntab next · enter create · esc cancel"
	tests := []struct {
		lay  formLayout
		want string
	}{
		{branchFormLayout(160, 50, 94), "type a branch name\ntype · ↑↓/ctrl+jk pick · tab next · enter create · esc cancel"},
		{branchFormLayout(80, 24, 78), "type a branch name\ntype · ↑↓/ctrl+jk pick · tab next · enter create · esc cancel"},
		{branchFormLayout(120, 40, 70), two},
		{branchFormLayout(110, 40, 64), two},
		{branchFormLayout(44, 24, 42), two},
	}
	for _, tt := range tests {
		lines := panelLines(s, tt.lay)
		got := strings.TrimRight(lines[len(lines)-2], " ") + "\n" + strings.TrimRight(lines[len(lines)-1], " ")
		if got != tt.want {
			t.Errorf("width %d: key line %q, want %q", tt.lay.Width, got, tt.want)
		}
	}
}
