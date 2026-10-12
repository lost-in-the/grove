package tui

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
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
	press(s, "tab")
	if got := strings.TrimSpace(s.Branch.Value()); got != "fix/login" {
		t.Errorf("branch = %q, want fix/login accepted from the suggestion", got)
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
	if s.keyLine() != "creating…" {
		t.Errorf("key line while busy = %q, want creating…", s.keyLine())
	}
}
