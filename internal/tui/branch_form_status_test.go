package tui

import (
	"fmt"
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
)

func formNewNamed(branch string) *BranchFormState {
	s := newBranchForm(testFormOptions())
	s.Branch.SetValue(branch)
	s.syncAlias()
	return s
}

func formWithAlias(branch, alias string) *BranchFormState {
	s := formNewNamed(branch)
	s.Alias.SetValue(alias)
	s.aliasEdited = true
	return s
}

func formWithBase(base string, focus formField) *BranchFormState {
	s := formNewNamed("abc-12")
	s.Base.SetValue(base)
	s.setFocus(focus)
	return s
}

func copyFormWith(branch string, wipErr error) *BranchFormState {
	s := newCopyForm(testFormOptions(), testSource())
	s.setWIP("/r/auth", false, nil, wipErr)
	if branch != "-" {
		s.Branch.SetValue(branch)
		s.syncAlias()
	}
	return s
}

func switchFormWith(branch string) *BranchFormState {
	s := newSwitchForm(testFormOptions(), testSource())
	s.setWIP("/r/auth", false, nil, nil)
	s.Branch.SetValue(branch)
	s.Pick = s.defaultPick()
	return s
}

type statusCase struct {
	name  string
	s     *BranchFormState
	msg   string
	kind  formStatusKind
	fault formField // the field marked ✗; noField for none
}

// statusCases is every status the form can show, with its kind.
func statusCases() []statusCase {
	busy := formNewNamed("abc-12")
	busy.Busy = true
	failed := formNewNamed("abc-12")
	failed.Status = "boom"
	return []statusCase{
		{"new: nothing typed", newBranchForm(testFormOptions()), "type a branch name", formStatusHint, noField},
		{"new: ready", formNewNamed("abc-12"), "✓ ready", formStatusReady, noField},
		{"new: existing branch ready", formNewNamed("fix/login"), "✓ ready", formStatusReady, noField},
		{"new: in use", formNewNamed("feature/auth"), "in use by auth · enter switches there", formStatusNotice, noField},
		{"new: name taken", formNewNamed("fix/auth"), `name "auth" is taken by auth`, formStatusBlocker, fieldAlias},
		{"new: name invalid", formWithAlias("abc-12", "a b"), "name contains invalid characters", formStatusBlocker, fieldAlias},
		{"new: name reserved", formWithAlias("abc-12", "root"), `"root" is reserved for the main worktree`, formStatusBlocker, fieldAlias},
		{"new: name empty", formWithAlias("abc-12", ""), "type a name", formStatusHint, noField},
		{"new: base not found", formWithBase("nope", fieldAlias), `base branch "nope" not found`, formStatusBlocker, fieldBase},
		{"new: base not found, focused", formWithBase("nope", fieldBase), `base branch "nope" not found`, formStatusBlocker, fieldBase},
		{"new: base empty", formWithBase("", fieldBase), "type a base branch", formStatusHint, noField},
		{"new: base partial, enter takes the pick", formWithBase("fi", fieldBase), `base branch "fi" not found`, formStatusHint, noField},
		{"new: busy", busy, "creating…", formStatusBusy, noField},
		{"new: failed", failed, "boom", formStatusError, noField},
		{"copy: checking", newCopyForm(testFormOptions(), testSource()), "checking for uncommitted changes…", formStatusHint, noField},
		{"copy: check failed", copyFormWith("-", errTestBoom), "could not check for uncommitted changes: " + errTestBoom.Error(), formStatusBlocker, fieldChanges},
		{"copy: nothing typed", copyFormWith("", nil), "type a name for the new branch", formStatusHint, noField},
		{"copy: branch exists", copyFormWith("fix/login", nil), `branch "fix/login" already exists`, formStatusBlocker, fieldBranch},
		{"copy: ready", copyFormWith("-", nil), "✓ ready", formStatusReady, noField},
		{"switch: nothing typed", switchFormWith(""), "type a branch name", formStatusHint, noField},
		{"switch: already on it", switchFormWith("feature/auth"), "auth is already on feature/auth", formStatusBlocker, fieldBranch},
		{"switch: checked out elsewhere", switchFormWith("main"), "main is checked out in root", formStatusBlocker, fieldBranch},
		{"switch: no local branch", switchFormWith("remote-only"), `no local branch "remote-only"`, formStatusBlocker, fieldBranch},
		{"switch: partial, enter takes the pick", switchFormWith("logi"), "enter switches to fix/login", formStatusNotice, noField},
		{"switch: ready", switchFormWith("fix/login"), "✓ ready", formStatusReady, noField},
	}
}

func TestBranchForm_EveryStatusHasAKind(t *testing.T) {
	for _, tc := range statusCases() {
		msg, kind := tc.s.status()
		if msg != tc.msg || kind != tc.kind {
			t.Errorf("%s: status = %q (kind %d), want %q (kind %d)", tc.name, msg, kind, tc.msg, tc.kind)
		}
		if f, ok := tc.s.faultField(); (ok && f != tc.fault) || (!ok && tc.fault != noField) {
			t.Errorf("%s: fault = %d (%v), want %d", tc.name, f, ok, tc.fault)
		}
	}
}

// A blocker is a state enter cannot act on: enter must not be offered.
func TestBranchForm_BlockerFooterDropsEnter(t *testing.T) {
	for _, tc := range statusCases() {
		line := tc.s.keyLine()
		hasEnter := strings.Contains(line, "enter ")
		if tc.kind == formStatusBlocker && hasEnter {
			t.Errorf("%s: key line %q offers enter on a blocker", tc.name, line)
		}
		if (tc.kind == formStatusReady || tc.kind == formStatusNotice || tc.kind == formStatusError) && !hasEnter {
			t.Errorf("%s: key line %q drops enter, which acts here", tc.name, line)
		}
		if !tc.s.Busy && !strings.HasSuffix(line, "esc cancel") {
			t.Errorf("%s: key line %q lost esc cancel", tc.name, line)
		}
	}
}

func TestBranchForm_NoticeKeepsEnterSwitch(t *testing.T) {
	if line := formNewNamed("feature/auth").keyLine(); !strings.Contains(line, "enter switch") {
		t.Errorf("key line = %q, want enter switch for an in-use branch", line)
	}
	if got := formStatusStyle(formStatusNotice); got.Render("x") != Styles.WarningText.Render("x") {
		t.Error("the in-use notice lost the warning token")
	}
}

func TestBranchForm_BlockerStatusHasGlyphAndErrorToken(t *testing.T) {
	for _, tc := range []struct {
		kind formStatusKind
		want string
	}{
		{formStatusBlocker, Styles.ErrorText.Render("✗ taken")},
		{formStatusError, Styles.ErrorText.Render("✗ taken")},
		{formStatusHint, Styles.DetailDim.Render("taken")},
		{formStatusNotice, Styles.WarningText.Render("taken")},
		{formStatusReady, Styles.SuccessText.Render("taken")},
	} {
		if got := formStatusLine("taken", tc.kind); got != tc.want {
			t.Errorf("kind %d: status line = %q, want %q", tc.kind, got, tc.want)
		}
	}
}

func TestBranchForm_FieldAtFaultIsMarkedWithOrWithoutFocus(t *testing.T) {
	nameTaken := formNewNamed("fix/auth") // focus stays on Branch
	baseMissing := formWithBase("nope", fieldAlias)
	baseFocused := formWithBase("nope", fieldBase)
	copyExists := copyFormWith("fix/login", nil)
	copyExists.setFocus(fieldAlias)
	for _, tc := range []struct {
		name   string
		s      *BranchFormState
		marked string
		clean  []string
	}{
		{"name taken", nameTaken, "Name ✗", []string{"Branch ✗", "Base ✗"}},
		{"base missing", baseMissing, "Base ✗", []string{"Branch ✗", "Name ✗"}},
		{"base missing, focused", baseFocused, "Base ✗", []string{"Branch ✗", "Name ✗"}},
		{"copy branch exists", copyExists, "Branch ✗", []string{"Name ✗"}},
	} {
		lay := branchFormLayout(80, 24, 78)
		text := strings.Join(panelLines(tc.s, lay), "\n")
		if !strings.Contains(text, tc.marked) {
			t.Errorf("%s: want %q:\n%s", tc.name, tc.marked, text)
		}
		for _, c := range tc.clean {
			if strings.Contains(text, c) {
				t.Errorf("%s: %q is marked but is not at fault:\n%s", tc.name, c, text)
			}
		}
		f, _ := tc.s.faultField()
		label := map[formField]string{fieldBranch: "Branch", fieldBase: "Base", fieldAlias: "Name"}[f]
		want := Styles.ErrorText.Render(fmt.Sprintf("%-8s", label+" ✗"))
		if got := tc.s.label(label, f); !strings.Contains(got, want) {
			t.Errorf("%s: label = %q, want the error token around %q", tc.name, got, label+" ✗")
		}
	}
	// Nothing at fault, nothing marked.
	for _, s := range []*BranchFormState{newBranchForm(testFormOptions()), formNewNamed("feature/auth"), formWithBase("fi", fieldBase)} {
		if text := strings.Join(panelLines(s, branchFormLayout(80, 24, 78)), "\n"); strings.Contains(text, "✗") {
			t.Errorf("a ✗ without a blocker:\n%s", text)
		}
	}
}

func TestBranchForm_EnterOnABlockerFocusesTheFieldAtFault(t *testing.T) {
	nameTaken := formNewNamed("fix/auth")
	baseMissing := formWithBase("nope", fieldAlias)
	copyExists := copyFormWith("fix/login", nil)
	copyExists.setFocus(fieldAlias)
	for _, tc := range []struct {
		name string
		s    *BranchFormState
		want formField
	}{
		{"name taken, from Branch", nameTaken, fieldAlias},
		{"base missing, from Name", baseMissing, fieldBase},
		{"copy branch exists, from Name", copyExists, fieldBranch},
	} {
		if press(tc.s, "enter") != formContinue {
			t.Errorf("%s: enter submitted a blocker", tc.name)
		}
		if tc.s.Focus != tc.want {
			t.Errorf("%s: focus = %d after enter, want %d", tc.name, tc.s.Focus, tc.want)
		}
	}
}

// Checking whether enter would act must not change the form.
func TestBranchForm_StatusDoesNotTakeThePick(t *testing.T) {
	s := formWithBase("fi", fieldBase)
	_, _ = s.status()
	_ = s.keyLine()
	if got := s.baseValue(); got != "fi" {
		t.Errorf("Base = %q after rendering, want the typed text", got)
	}
}

func TestBranchForm_BlockerRendersAsOneStatusLine(t *testing.T) {
	s := formNewNamed("fix/auth")
	lines := panelLines(s, branchFormLayout(80, 24, 78))
	i := lineIndex(lines, `✗ name "auth" is taken by auth`)
	if i < 0 {
		t.Fatalf("no blocker status line:\n%s", strings.Join(lines, "\n"))
	}
	if keys := ansi.Strip(lines[i+1]); strings.Contains(keys, "enter") || !strings.Contains(keys, "esc cancel") {
		t.Errorf("key line under the blocker = %q", keys)
	}
}
