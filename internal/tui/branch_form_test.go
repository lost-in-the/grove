package tui

import (
	"errors"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
)

func testFormOptions() branchFormOptions {
	return branchFormOptions{
		ProjectName:   "grove",
		NamePattern:   "{project}-{name}",
		DefaultBase:   "main",
		AllBranches:   []string{"main", "feature/auth", "fix/login", "remote-only"},
		LocalBranches: []string{"main", "feature/auth", "fix/login"},
		Worktrees: []WorktreeItem{
			{ShortName: "root", Branch: "main", Path: "/r/root"},
			{ShortName: "auth", Branch: "feature/auth", Path: "/r/auth"},
		},
	}
}

func typeInto(t *testing.T, s *BranchFormState, text string) {
	t.Helper()
	for _, r := range text {
		s.handleKey(tea.KeyPressMsg{Code: r, Text: string(r)}, DefaultKeyMap())
	}
}

func press(s *BranchFormState, k string) formResult {
	res, _ := s.handleKey(makeKeyMsg(k), DefaultKeyMap())
	return res
}

func TestBranchForm_SuggestsMatchingBranchesWithNotes(t *testing.T) {
	s := newBranchForm(testFormOptions())
	typeInto(t, s, "o")
	got := map[string]string{}
	for _, sg := range s.suggestions() {
		got[sg.Branch] = sg.Note
	}
	if got["remote-only"] != "remote" {
		t.Errorf("remote-only note = %q, want remote", got["remote-only"])
	}
	if got["fix/login"] != "existing" {
		t.Errorf("fix/login note = %q, want existing", got["fix/login"])
	}
	if _, ok := got["feature/auth"]; ok {
		t.Error("feature/auth has no 'o'; it must not be listed")
	}
}

func TestBranchForm_OffersNewRowWhenNoExactMatch(t *testing.T) {
	s := newBranchForm(testFormOptions())
	typeInto(t, s, "abc-12")
	sg := s.suggestions()
	if len(sg) == 0 || !sg[len(sg)-1].New || sg[len(sg)-1].Branch != "abc-12" {
		t.Fatalf("last suggestion = %+v, want a new-branch row for abc-12", sg)
	}
	s2 := newBranchForm(testFormOptions())
	typeInto(t, s2, "main")
	for _, x := range s2.suggestions() {
		if x.New {
			t.Error("exact match must not offer a new-branch row")
		}
	}
}

func TestBranchForm_InUseBranchSwitches(t *testing.T) {
	s := newBranchForm(testFormOptions())
	typeInto(t, s, "feature/auth")
	in := s.intent()
	if in.Kind != intentSwitchTo || in.Target == nil || in.Target.ShortName != "auth" {
		t.Fatalf("intent = %+v, want switch to auth", in)
	}
	if msg, _ := s.status(); msg != "in use by auth · ⏎ switches there" {
		t.Errorf("status = %q", msg)
	}
	if press(s, "enter") != formSubmit {
		t.Error("enter on an in-use branch should submit the switch")
	}
}

func TestBranchForm_DerivesAliasFromBranch(t *testing.T) {
	s := newBranchForm(testFormOptions())
	typeInto(t, s, "fix/login")
	if got := s.aliasValue(); got != "login" {
		t.Errorf("alias = %q, want login", got)
	}
}

func TestBranchForm_EditedAliasIsKept(t *testing.T) {
	s := newBranchForm(testFormOptions())
	typeInto(t, s, "fix/login")
	press(s, "tab")
	typeInto(t, s, "2")
	press(s, "shift+tab")
	typeInto(t, s, "x")
	if got := s.aliasValue(); got != "login2" {
		t.Errorf("alias = %q, want login2 (edited alias must survive branch edits)", got)
	}
}

func TestBranchForm_ClearedAliasStaysEmpty(t *testing.T) {
	s := newBranchForm(testFormOptions())
	typeInto(t, s, "fix/login")
	press(s, "tab")
	for range "login" {
		press(s, "backspace")
	}
	if got := s.aliasValue(); got != "" {
		t.Fatalf("alias = %q, want empty after clearing", got)
	}
	if msg, kind := s.status(); kind != formStatusHint || msg != "type an alias" {
		t.Errorf("status = %q (%v), want hint 'type an alias'", msg, kind)
	}
}

func TestBranchForm_NewBranchUsesBaseAsRef(t *testing.T) {
	s := newBranchForm(testFormOptions())
	typeInto(t, s, "abc-12")
	in := s.intent()
	if in.Kind != intentCreateNew || in.Branch != "abc-12" || in.FromRef != "main" || in.Alias != "abc-12" {
		t.Errorf("intent = %+v, want new abc-12 from main", in)
	}
}

func TestBranchForm_RemoteOnlyBaseGetsOriginPrefix(t *testing.T) {
	s := newBranchForm(testFormOptions())
	typeInto(t, s, "abc-12")
	press(s, "tab")
	for range "main" {
		press(s, "backspace")
	}
	typeInto(t, s, "remote-only")
	if in := s.intent(); in.FromRef != "origin/remote-only" {
		t.Errorf("FromRef = %q, want origin/remote-only", in.FromRef)
	}
}

func TestBranchForm_ExistingBranchSkipsBase(t *testing.T) {
	s := newBranchForm(testFormOptions())
	typeInto(t, s, "fix/login")
	press(s, "tab")
	if s.Focus != fieldAlias {
		t.Errorf("focus after tab = %v, want alias (base is n/a for an existing branch)", s.Focus)
	}
	if in := s.intent(); in.Kind != intentCreateExisting || in.Branch != "fix/login" {
		t.Errorf("intent = %+v, want create from existing fix/login", in)
	}
}

func TestBranchForm_UnknownBaseIsInvalid(t *testing.T) {
	s := newBranchForm(testFormOptions())
	typeInto(t, s, "abc-12")
	press(s, "tab")
	for range "main" {
		press(s, "backspace")
	}
	typeInto(t, s, "nope")
	if press(s, "enter") != formContinue {
		t.Fatal("enter with an unknown base must not submit")
	}
	if s.Focus != fieldBase {
		t.Errorf("focus = %v, want base", s.Focus)
	}
}

func TestBranchForm_DuplicateAliasIsAnError(t *testing.T) {
	s := newBranchForm(testFormOptions())
	typeInto(t, s, "fix/auth")
	press(s, "tab")
	press(s, "tab")
	if s.aliasValue() != "auth" {
		t.Fatalf("alias = %q, want auth", s.aliasValue())
	}
	if press(s, "enter") != formContinue {
		t.Fatal("a taken alias must not submit")
	}
	if msg, _ := s.status(); !strings.Contains(msg, "taken by auth") {
		t.Errorf("status = %q, want it to name the worktree using the alias", msg)
	}
}

func TestBranchForm_TabAcceptsHighlightedSuggestion(t *testing.T) {
	s := newBranchForm(testFormOptions())
	typeInto(t, s, "logi")
	press(s, "up")
	press(s, "tab")
	if got := strings.TrimSpace(s.Branch.Value()); got != "fix/login" {
		t.Errorf("branch = %q, want fix/login accepted from the row moved to", got)
	}
}

func TestBranchForm_ArrowsMovePick(t *testing.T) {
	s := newBranchForm(testFormOptions())
	press(s, "down")
	press(s, "down")
	if s.Pick != 2 {
		t.Errorf("pick = %d, want 2", s.Pick)
	}
	press(s, "up")
	if s.Pick != 1 {
		t.Errorf("pick = %d, want 1", s.Pick)
	}
}

func TestBranchForm_JAndKAreTyped(t *testing.T) {
	s := newBranchForm(testFormOptions())
	typeInto(t, s, "jk")
	if got := s.Branch.Value(); got != "jk" {
		t.Errorf("branch = %q, want jk typed, not used for navigation", got)
	}
}

func TestBranchForm_IgnoresKeysWhileBusy(t *testing.T) {
	s := newBranchForm(testFormOptions())
	s.Busy = true
	if press(s, "esc") != formContinue {
		t.Error("esc while creating must not cancel")
	}
}

func TestBranchForm_EscCancels(t *testing.T) {
	s := newBranchForm(testFormOptions())
	if press(s, "esc") != formCancel {
		t.Error("esc should cancel")
	}
}

func TestBranchForm_PrefillRemoteBranch(t *testing.T) {
	s := newBranchForm(testFormOptions())
	s.applyPrefill(branchFormPrefill{Branch: "pr-branch", RemoteBranch: true, FocusAlias: true})
	if in := s.intent(); in.Kind != intentCreateExisting || in.Branch != "pr-branch" {
		t.Errorf("intent = %+v, want create from existing pr-branch", in)
	}
	if s.Focus != fieldAlias {
		t.Errorf("focus = %v, want alias", s.Focus)
	}
}

func TestBranchForm_PrefillAliasCountsAsEdited(t *testing.T) {
	s := newBranchForm(testFormOptions())
	s.applyPrefill(branchFormPrefill{Alias: "issue-7"})
	typeInto(t, s, "fix/login")
	if got := s.aliasValue(); got != "issue-7" {
		t.Errorf("alias = %q, want issue-7 kept", got)
	}
}

func TestBranchForm_SetCreatingDoneShowsError(t *testing.T) {
	s := newBranchForm(testFormOptions())
	s.Busy = true
	s.setCreatingDone("boom")
	if s.Busy {
		t.Error("still busy after done")
	}
	if msg, kind := s.status(); kind != formStatusError || msg != "boom" {
		t.Errorf("status = %q (%v), want error boom", msg, kind)
	}
}

func TestBranchForm_KeyLineNamesTheAction(t *testing.T) {
	s := newBranchForm(testFormOptions())
	if s.title() != "New worktree" {
		t.Errorf("title = %q", s.title())
	}
	if !strings.HasPrefix(s.keyLine(), "⏎ create") {
		t.Errorf("key line = %q, want it to start with ⏎ create", s.keyLine())
	}
	typeInto(t, s, "feature/auth")
	if !strings.HasPrefix(s.keyLine(), "⏎ switch") {
		t.Errorf("key line = %q, want ⏎ switch for an in-use branch", s.keyLine())
	}
	s.Busy = true
	if s.keyLine() != "wait for this to finish" {
		t.Errorf("key line while busy = %q, want the wait note (status names the work)", s.keyLine())
	}
}

func TestDashboardN_OpensBranchForm(t *testing.T) {
	m := newTestModel(withItems(3), withSize(120, 40))
	next, _ := m.Update(makeKeyMsg("n"))
	m = next.(Model)
	if m.activeView() != ViewBranchForm || m.branchForm == nil {
		t.Fatalf("after n: view=%v form=%v, want ViewBranchForm", m.activeView(), m.branchForm != nil)
	}
	next, _ = m.Update(makeKeyMsg("esc"))
	m = next.(Model)
	if m.activeView() != ViewDashboard || m.branchForm != nil {
		t.Errorf("after esc: view=%v form nil=%v", m.activeView(), m.branchForm == nil)
	}
}

func TestOpenBranchFormForPR_TreatsBranchAsRemote(t *testing.T) {
	m := newTestModel(withItems(3), withSize(120, 40), withPRData())
	next, _ := m.openBranchForm(branchFormPrefill{Branch: "never-fetched", RemoteBranch: true, FocusAlias: true})
	m = next.(Model)
	if in := m.branchForm.intent(); in.Kind != intentCreateExisting || in.Branch != "never-fetched" {
		t.Errorf("intent = %+v, want create from existing never-fetched", in)
	}
}

// Without a worktree manager, submitting reports an error instead of
// starting a creation goroutine that would dereference the nil manager.
func TestBranchFormSubmit_WithoutManagerReportsError(t *testing.T) {
	m := newTestModel(withItems(3), withSize(120, 40))
	next, _ := m.openBranchForm(branchFormPrefill{Branch: "brand-new"})
	m = next.(Model)
	m.branchForm.local["main"] = true
	m.branchForm.Base.SetValue("main")
	next, _ = m.Update(makeKeyMsg("enter"))
	m = next.(Model)
	if m.branchForm.Busy {
		t.Fatal("creation started without a worktree manager")
	}
	if _, kind := m.branchForm.status(); kind != formStatusError {
		t.Errorf("status kind = %v, want an error", kind)
	}
}

func TestBranchFormCreationDone_SuccessReturnsToDashboard(t *testing.T) {
	m := newTestModel(withItems(3), withSize(120, 40))
	next, _ := m.openBranchForm(branchFormPrefill{})
	m = next.(Model)
	m.branchForm.Busy = true
	next, _ = m.Update(creationDoneMsg{source: "create", name: "brand-new"})
	m = next.(Model)
	if m.activeView() != ViewDashboard || m.branchForm != nil || m.pendingSelect != "brand-new" {
		t.Errorf("view=%v form nil=%v pendingSelect=%q", m.activeView(), m.branchForm == nil, m.pendingSelect)
	}
}

func TestBranchFormCreationDone_ErrorStaysOpen(t *testing.T) {
	m := newTestModel(withItems(3), withSize(120, 40))
	next, _ := m.openBranchForm(branchFormPrefill{})
	m = next.(Model)
	m.branchForm.Busy = true
	next, _ = m.Update(creationDoneMsg{source: "create", err: errTestBoom})
	m = next.(Model)
	if m.activeView() != ViewBranchForm || m.branchForm.Busy {
		t.Fatalf("view=%v busy=%v, want form open and idle", m.activeView(), m.branchForm.Busy)
	}
	if msg, kind := m.branchForm.status(); kind != formStatusError || msg != errTestBoom.Error() {
		t.Errorf("status = %q (%v)", msg, kind)
	}
}

func TestBranchFormInUse_SwitchesAndQuits(t *testing.T) {
	m := newTestModel(withItems(3), withSize(120, 40))
	next, _ := m.openBranchForm(branchFormPrefill{Branch: "feature-auth"})
	m = next.(Model)
	next, cmd := m.Update(makeKeyMsg("enter"))
	m = next.(Model)
	if m.switchTo != "/tmp/test-project-feature-auth" || cmd == nil {
		t.Errorf("switchTo=%q cmd=%v, want a switch to feature-auth and quit", m.switchTo, cmd != nil)
	}
}

var errTestBoom = errors.New("boom")

// Keys typed on the dashboard after n reach the form's Branch field through
// Update, including j and k, which navigate everywhere else.
func TestBranchForm_TypingThroughUpdateReachesBranch(t *testing.T) {
	m := newTestModel(withItems(3), withSize(120, 40))
	m = sendKey(m, "n")
	for _, k := range []string{"f", "i", "x", "j", "k"} {
		m = sendKey(m, k)
	}
	if got := m.branchForm.Branch.Value(); got != "fixjk" {
		t.Errorf("branch = %q, want fixjk", got)
	}
}

// --- Final review fixes ---

func TestBranchForm_PickDefaultsToExactMatch(t *testing.T) {
	opts := testFormOptions()
	opts.AllBranches = append([]string{"domain-x"}, opts.AllBranches...)
	opts.LocalBranches = append([]string{"domain-x"}, opts.LocalBranches...)
	s := newBranchForm(opts)
	typeInto(t, s, "main")
	press(s, "tab")
	if got := s.branchValue(); got != "main" {
		t.Errorf("branch after tab = %q, want main kept (exact match is highlighted)", got)
	}
}

func TestBranchForm_PickDefaultsToNewRowForPartial(t *testing.T) {
	s := newBranchForm(testFormOptions())
	typeInto(t, s, "fea")
	sg := s.suggestions()
	if s.Pick < 0 || s.Pick >= len(sg) || !sg[s.Pick].New {
		t.Errorf("highlighted row = %+v, want the + new row matching what enter will do", sg)
	}
}

func TestBranchForm_EnterAcceptsMovedPick(t *testing.T) {
	s := newBranchForm(testFormOptions())
	typeInto(t, s, "logi")
	press(s, "up")
	if press(s, "enter") != formSubmit {
		t.Fatal("enter should submit")
	}
	if in := s.intent(); in.Kind != intentCreateExisting || in.Branch != "fix/login" {
		t.Errorf("intent = %+v, want the highlighted fix/login", in)
	}
}

func TestBranchForm_PlaceholderAndLongInputRender(t *testing.T) {
	s := newBranchForm(testFormOptions())
	s.setWidth(80, 24)
	if out := ansi.Strip(renderBranchForm(s, 80, 24, "⠋")); !strings.Contains(out, "type to search or create") {
		t.Errorf("placeholder missing:\n%s", out)
	}
	long := strings.Repeat("x", 70) + "-tail"
	typeInto(t, s, long)
	if out := ansi.Strip(renderBranchForm(s, 80, 24, "⠋")); !strings.Contains(out, "x-tail") {
		t.Errorf("the end of a long branch name is not visible:\n%s", out)
	}
}

func TestBranchFormPaste_FillsFocusedField(t *testing.T) {
	m := newTestModel(withItems(3), withSize(120, 40))
	m = sendKey(m, "n")
	next, _ := m.Update(tea.PasteMsg{Content: "pasted-branch"})
	m = next.(Model)
	if got := m.branchForm.branchValue(); got != "pasted-branch" {
		t.Fatalf("branch = %q, want pasted-branch", got)
	}
	if got := m.branchForm.aliasValue(); got != "pasted-branch" {
		t.Errorf("alias = %q, want it derived from the pasted branch", got)
	}
}

func TestPickDefaultBase(t *testing.T) {
	known := func(b string) bool { return b == "master" || b == "develop" }
	cases := []struct{ name, configured, detected, mainBranch, want string }{
		{"config wins", "develop", "master", "master", "develop"},
		{"detected when known", "", "master", "develop", "master"},
		{"main worktree when detection fails", "", "", "master", "master"},
		{"main worktree when detected is unknown", "", "trunk", "develop", "develop"},
		{"literal main last", "", "", "", "main"},
	}
	for _, c := range cases {
		if got := pickDefaultBase(c.configured, c.detected, c.mainBranch, known); got != c.want {
			t.Errorf("%s: got %q, want %q", c.name, got, c.want)
		}
	}
}

func TestBranchForm_FailedCreationShowsLogUntilAKey(t *testing.T) {
	s := newBranchForm(testFormOptions())
	typeInto(t, s, "abc-12")
	s.Busy = true
	s.Log = NewActivityLog(60, 6)
	s.Log.AddLine("Creating worktree 'abc-12'...")
	s.Log.SetDone(errTestBoom)
	s.setCreatingDone("fatal: a very long git error that the status line cannot hold in full")
	out := ansi.Strip(renderBranchForm(s, 80, 24, "⠋"))
	if !strings.Contains(out, "Creating worktree 'abc-12'") {
		t.Errorf("activity log hidden after a failure:\n%s", out)
	}
	if !strings.Contains(s.keyLine(), "any key to edit") {
		t.Errorf("key line = %q, want a hint to dismiss the log", s.keyLine())
	}
	if press(s, "x") != formContinue || s.Log != nil || s.branchValue() != "abc-12" {
		t.Errorf("first key should only dismiss the log: log=%v branch=%q", s.Log != nil, s.branchValue())
	}
}

func testSource() WorktreeItem {
	return WorktreeItem{ShortName: "auth", Branch: "feature/auth", Path: "/r/auth"}
}

func TestCopyForm_DefaultsAndDerivedAlias(t *testing.T) {
	s := newCopyForm(testFormOptions(), testSource())
	if s.title() != "Copy auth" {
		t.Errorf("title = %q", s.title())
	}
	if got := s.branchValue(); got != "feature/auth-copy" {
		t.Errorf("branch = %q, want feature/auth-copy", got)
	}
	if got := s.aliasValue(); got != "auth-copy" {
		t.Errorf("alias = %q, want auth-copy", got)
	}
	if s.Changes != WIPCopy {
		t.Errorf("changes = %v, want copy (non-destructive default)", s.Changes)
	}
	if len(s.suggestions()) != 0 {
		t.Error("copy mode offers no branch suggestions")
	}
}

func TestCopyForm_EnterBlockedUntilWIPChecked(t *testing.T) {
	s := newCopyForm(testFormOptions(), testSource())
	if press(s, "enter") != formContinue {
		t.Fatal("enter before the changes check must not submit")
	}
	if msg, _ := s.status(); msg != "checking for uncommitted changes…" {
		t.Errorf("status = %q", msg)
	}
	s.setWIP("/r/auth", false, nil, nil)
	if press(s, "enter") != formSubmit {
		t.Error("enter after a clean check should submit")
	}
}

func TestCopyForm_WIPCheckErrorBlocks(t *testing.T) {
	s := newCopyForm(testFormOptions(), testSource())
	s.setWIP("/r/auth", false, nil, errTestBoom)
	if press(s, "enter") != formContinue {
		t.Fatal("a failed changes check must block submit")
	}
	if msg, _ := s.status(); !strings.Contains(msg, "could not check") {
		t.Errorf("status = %q", msg)
	}
}

func TestCopyForm_IgnoresWIPForOtherPath(t *testing.T) {
	s := newCopyForm(testFormOptions(), testSource())
	s.setWIP("/r/other", false, nil, nil)
	if s.wipChecked() {
		t.Error("a check for another worktree must be ignored")
	}
}

func TestCopyForm_RejectsExistingBranch(t *testing.T) {
	s := newCopyForm(testFormOptions(), testSource())
	s.setWIP("/r/auth", false, nil, nil)
	s.Branch.SetValue("fix/login")
	s.syncAlias()
	if press(s, "enter") != formContinue {
		t.Fatal("an existing branch must not submit")
	}
	if msg, _ := s.status(); msg != `branch "fix/login" already exists` {
		t.Errorf("status = %q", msg)
	}
}

func TestCopyForm_DirtyAddsChangesFieldAndCycles(t *testing.T) {
	s := newCopyForm(testFormOptions(), testSource())
	s.setWIP("/r/auth", true, []string{"a.go", "b.go"}, nil)
	press(s, "tab")
	press(s, "tab")
	if s.Focus != fieldChanges {
		t.Fatalf("focus = %v, want changes", s.Focus)
	}
	press(s, "right")
	if got := changesLabel(s.Changes); got != "move" {
		t.Errorf("changes = %s, want move after right", got)
	}
	press(s, "space")
	press(s, "space")
	if got := changesLabel(s.Changes); got != "copy" {
		t.Errorf("changes = %s, want copy after cycling round", got)
	}
	press(s, "left")
	if got := changesLabel(s.Changes); got != "leave" {
		t.Errorf("changes = %s, want leave after left from copy", got)
	}
	press(s, "left")
	press(s, "x")
	if got := s.aliasValue(); got != "auth-copy" {
		t.Errorf("typing on the changes row changed the alias to %q", got)
	}
	in := s.intent()
	if in.Kind != intentCopy || in.Branch != "feature/auth-copy" || in.Alias != "auth-copy" || in.WIP != WIPMove {
		t.Errorf("intent = %+v", in)
	}
}

func TestCopyForm_CleanSourceLeavesChanges(t *testing.T) {
	s := newCopyForm(testFormOptions(), testSource())
	s.setWIP("/r/auth", false, nil, nil)
	if in := s.intent(); in.WIP != WIPLeave {
		t.Errorf("WIP = %v, want leave for a clean source", in.WIP)
	}
	press(s, "tab")
	press(s, "tab")
	if s.Focus == fieldChanges {
		t.Error("a clean source has no changes field")
	}
}

func TestSwitchForm_SuggestsFreeLocalBranchesOnly(t *testing.T) {
	s := newSwitchForm(testFormOptions(), testSource())
	got := map[string]bool{}
	for _, sg := range s.suggestions() {
		got[sg.Branch] = true
	}
	if !got["fix/login"] {
		t.Error("free local branch fix/login should be offered")
	}
	for _, b := range []string{"main", "feature/auth", "remote-only"} {
		if got[b] {
			t.Errorf("%s must not be offered (in use or not local)", b)
		}
	}
}

func TestSwitchForm_RejectsCurrentAndInUse(t *testing.T) {
	s := newSwitchForm(testFormOptions(), testSource())
	s.setWIP("/r/auth", false, nil, nil)
	for branch, want := range map[string]string{
		"feature/auth": "auth is already on feature/auth",
		"main":         "main is checked out in root",
		"remote-only":  `no local branch "remote-only"`,
	} {
		s.Branch.SetValue(branch)
		if press(s, "enter") != formContinue {
			t.Errorf("%s: submitted", branch)
		}
		if msg, _ := s.status(); msg != want {
			t.Errorf("%s: status = %q, want %q", branch, msg, want)
		}
	}
}

func TestSwitchForm_StashesWhenDirty(t *testing.T) {
	s := newSwitchForm(testFormOptions(), testSource())
	s.setWIP("/r/auth", true, []string{"a.go"}, nil)
	typeInto(t, s, "fix/login")
	if press(s, "enter") != formSubmit {
		t.Fatal("expected submit")
	}
	in := s.intent()
	if in.Kind != intentSwitchBranch || in.Branch != "fix/login" || !in.Stash || in.Target.Path != "/r/auth" {
		t.Errorf("intent = %+v", in)
	}
}

func TestBranchFormModes_KeyLines(t *testing.T) {
	c := newCopyForm(testFormOptions(), testSource())
	if !strings.HasPrefix(c.keyLine(), "⏎ copy") {
		t.Errorf("copy key line = %q", c.keyLine())
	}
	sw := newSwitchForm(testFormOptions(), testSource())
	if !strings.HasPrefix(sw.keyLine(), "⏎ switch") {
		t.Errorf("switch key line = %q", sw.keyLine())
	}
	n := newBranchForm(testFormOptions())
	if n.keyLine() != "⏎ create · tab field · ctrl+j/k pick · esc back" {
		t.Errorf("new key line changed: %q", n.keyLine())
	}
}

// In Switch mode there is nothing to create, so enter on a partial match
// takes the highlighted branch.
func TestSwitchForm_EnterAcceptsFirstMatch(t *testing.T) {
	s := newSwitchForm(testFormOptions(), testSource())
	s.setWIP("/r/auth", false, nil, nil)
	typeInto(t, s, "fi")
	if press(s, "enter") != formSubmit {
		t.Fatal("expected submit")
	}
	if in := s.intent(); in.Branch != "fix/login" {
		t.Errorf("branch = %q, want fix/login", in.Branch)
	}
}

func TestSwitchForm_EnterBlockedUntilWIPChecked(t *testing.T) {
	s := newSwitchForm(testFormOptions(), testSource())
	typeInto(t, s, "fix/login")
	if press(s, "enter") != formContinue {
		t.Fatal("enter before the changes check must not submit")
	}
	s.setWIP("/r/other", false, nil, nil)
	if press(s, "enter") != formContinue {
		t.Fatal("a check for another worktree must not unblock enter")
	}
	s.setWIP("/r/auth", false, nil, nil)
	if press(s, "enter") != formSubmit {
		t.Error("enter after a clean check should submit")
	}
}

func TestDashboardF_OpensCopyForm(t *testing.T) {
	m := newTestModel(withItems(3), withSize(120, 40))
	m.list.Select(1)
	next, cmd := m.Update(makeKeyMsg("f"))
	m = next.(Model)
	if m.activeView() != ViewBranchForm || m.branchForm == nil || m.branchForm.Mode != formModeCopy {
		t.Fatalf("view=%v form=%+v, want the copy form", m.activeView(), m.branchForm)
	}
	if cmd == nil {
		t.Error("expected the changes check command")
	}

	root := newTestModel(withItems(3), withSize(120, 40))
	root.list.Select(0)
	next, _ = root.Update(makeKeyMsg("f"))
	if f := next.(Model).branchForm; f == nil || f.Mode != formModeCopy {
		t.Error("f on the main worktree should open the copy form too")
	}
}

func TestDashboardB_OpensSwitchFormButNotOnMain(t *testing.T) {
	m := newTestModel(withItems(3), withSize(120, 40))
	m.list.Select(0)
	next, _ := m.Update(makeKeyMsg("b"))
	if next.(Model).activeView() != ViewDashboard {
		t.Fatal("b on the main worktree must not open the switch form")
	}
	m.list.Select(1)
	next, _ = m.Update(makeKeyMsg("b"))
	m = next.(Model)
	if m.branchForm == nil || m.branchForm.Mode != formModeSwitch {
		t.Fatalf("form = %+v, want the switch form", m.branchForm)
	}
}

func TestWIPCheck_RoutesToForm(t *testing.T) {
	m := newTestModel(withItems(3), withSize(120, 40))
	m.list.Select(1)
	next, _ := m.Update(makeKeyMsg("f"))
	m = next.(Model)
	path := m.branchForm.Source.Path
	next, _ = m.Update(wipCheckMsg{path: path, hasWIP: true, files: []string{"x"}})
	m = next.(Model)
	if !m.branchForm.wipChecked() || !m.branchForm.HasWIP {
		t.Error("changes check did not reach the form")
	}
}

func TestCopyFormComplete_Success(t *testing.T) {
	m := newTestModel(withItems(3), withSize(120, 40))
	m.branchForm = newCopyForm(testFormOptions(), testSource())
	m.setViewForTest(ViewBranchForm)
	m.branchForm.Busy = true
	next, _ := m.Update(forkCompleteMsg{name: "auth-copy", path: "/r/auth-copy"})
	m = next.(Model)
	if m.activeView() != ViewDashboard || m.branchForm != nil || m.pendingSelect != "auth-copy" {
		t.Errorf("view=%v form nil=%v pendingSelect=%q", m.activeView(), m.branchForm == nil, m.pendingSelect)
	}
}

func TestCopyFormComplete_ErrorStaysOpen(t *testing.T) {
	m := newTestModel(withItems(3), withSize(120, 40))
	m.branchForm = newCopyForm(testFormOptions(), testSource())
	m.setViewForTest(ViewBranchForm)
	m.branchForm.Busy = true
	next, _ := m.Update(forkCompleteMsg{err: errTestBoom})
	m = next.(Model)
	if m.activeView() != ViewBranchForm || m.branchForm.Busy {
		t.Fatalf("view=%v busy=%v", m.activeView(), m.branchForm.Busy)
	}
	if msg, kind := m.branchForm.status(); kind != formStatusError || msg != "boom" {
		t.Errorf("status = %q (%v)", msg, kind)
	}
}

func TestCopyFormComplete_PartialSuccessWarns(t *testing.T) {
	m := newTestModel(withItems(3), withSize(120, 40))
	m.branchForm = newCopyForm(testFormOptions(), testSource())
	m.setViewForTest(ViewBranchForm)
	m.branchForm.Busy = true
	next, _ := m.Update(forkCompleteMsg{name: "auth-copy", err: errTestBoom})
	m = next.(Model)
	if m.branchForm != nil || m.activeView() != ViewDashboard {
		t.Fatal("a created-but-warned copy closes the form")
	}
	if !strings.Contains(m.toast.Message(), "warning") {
		t.Errorf("toast = %q, want a warning", m.toast.Message())
	}
}

func TestSwitchFormComplete(t *testing.T) {
	m := newTestModel(withItems(3), withSize(120, 40))
	m.branchForm = newSwitchForm(testFormOptions(), testSource())
	m.setViewForTest(ViewBranchForm)
	m.branchForm.Busy = true
	next, _ := m.Update(checkoutCompleteMsg{err: errTestBoom})
	m = next.(Model)
	if m.branchForm == nil || m.branchForm.Busy {
		t.Fatal("an error keeps the switch form open and idle")
	}
	m.branchForm.Busy = true
	next, _ = m.Update(checkoutCompleteMsg{branch: "fix/login"})
	m = next.(Model)
	if m.branchForm != nil || m.activeView() != ViewDashboard {
		t.Error("success closes the switch form")
	}
}

func TestCopyFormSubmit_WithoutManagerReportsError(t *testing.T) {
	m := newTestModel(withItems(3), withSize(120, 40))
	m.branchForm = newCopyForm(testFormOptions(), testSource())
	m.setViewForTest(ViewBranchForm)
	m.branchForm.setWIP("/r/auth", false, nil, nil)
	next, _ := m.Update(makeKeyMsg("enter"))
	m = next.(Model)
	if m.branchForm.Busy {
		t.Error("copy started without a worktree manager")
	}
}

// The cursor blinks from the moment the form opens, not after the first key.
func TestDashboardN_ReturnsFocusCmd(t *testing.T) {
	m := newTestModel(withItems(3), withSize(120, 40))
	_, cmd := m.Update(makeKeyMsg("n"))
	if cmd == nil {
		t.Error("n returned no command; the Branch cursor will not blink")
	}
}

// While busy the status line names the work; the key line must not repeat it.
func TestBranchForm_BusyVerbShownOnce(t *testing.T) {
	s := newBranchForm(testFormOptions())
	s.Branch.SetValue("abc-12")
	s.syncAlias()
	s.Busy = true
	s.Log = NewActivityLog(formLogWidth(80), 6)
	s.Log.AddLine("Creating worktree 'abc-12'...")
	out := ansi.Strip(renderBranchForm(s, 80, 24, "⠋"))
	if n := strings.Count(out, "creating…"); n != 1 {
		t.Errorf("creating… appears %d times, want 1:\n%s", n, out)
	}
}

// On a short terminal the newest log line, which carries the spinner, stays
// in view.
func TestBranchForm_BusyLogTailVisibleWhenShort(t *testing.T) {
	s := newBranchForm(testFormOptions())
	s.Branch.SetValue("abc-12")
	s.syncAlias()
	s.Busy = true
	s.Log = NewActivityLog(formLogWidth(80), 6)
	for _, l := range []string{"one", "two", "three", "four", "newest step"} {
		s.Log.AddLine(l)
	}
	out := ansi.Strip(renderBranchForm(s, 80, 10, "⠋"))
	if !strings.Contains(out, "⠋ newest step") {
		t.Errorf("newest log line scrolled out of view:\n%s", out)
	}
}

func TestIssueBrowser_SeedsBranchName(t *testing.T) {
	m := newTestModel(withItems(3), withSize(120, 40), withIssueData())
	next, _ := m.Update(makeKeyMsg("enter"))
	m = next.(Model)
	if m.branchForm == nil {
		t.Fatal("enter on an issue did not open the form")
	}
	if got := m.branchForm.branchValue(); got != "issue-101" {
		t.Errorf("branch = %q, want issue-101", got)
	}
	if got := m.branchForm.aliasValue(); got != "issue-101" {
		t.Errorf("alias = %q, want issue-101", got)
	}
}
