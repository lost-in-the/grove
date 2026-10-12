# TUI Redesign Phase 3a (Branch Form, New Mode) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace the six-step create wizard with one fixed-frame form (Branch, Base, Alias) that never moves, never grows, and shows what will happen as you type.

**Architecture:**

- `branch_form.go` holds the form's state and every rule as plain methods: suggestions, alias derivation, validation, and the "intent" the form will act on. It is tested without a `Model`.
- `branch_form_view.go` renders the form at an exact width and height for any terminal size.
- The model opens the form from `n`, the PR browser, and the issue browser, and maps its intent onto the existing `createWorktreeCmd`.
- The overlay is top-anchored through a new `composeOverlayAt`.
- Phase 3b adds the Copy and Switch modes and deletes the fork and switch-branch wizards. Until then, `f` and `b` keep their old wizards.

**Tech Stack:** Go 1.26+, Bubble Tea v2, `charm.land/bubbles/v2/textinput`, lipgloss v2, the Phase 1 compositor, the Phase 2 view stack.

**Spec:** `docs/superpowers/specs/2026-10-10-tui-redesign-design.md` (section 10, N7).

## Global Constraints

- Every command completes in under 500ms (CLAUDE.md). Opening the form runs two local `git` branch listings, as the wizard does today.
- No `panic()`. Measure width with `lipgloss.Width` or `ansi.StringWidth`.
- Gate: `make lint test` with golangci-lint v2.14.
- Commits are signed: `timeout --kill-after=5s 120s git -c commit.gpgsign=true commit ...`, run outside the sandbox. This session's parent injects `commit.gpgsign=false`.
- **Expected golden churn:** `TestGolden_Overlay_Create` and the orphan `TestGolden_Overlay_Create/branch_step.golden` are deleted, and `TestGolden_BranchForm` is added. **Any other golden change is a finding.**

## Decisions

| # | Decision | Why |
|---|---|---|
| 1 | **Base defaults to the configured default branch** (`ResolveDefaultBranch`) and is always passed as `fromRef` | Spec 10.3. Today a new branch starts from the main worktree's HEAD; that differs when the main checkout sits on a feature branch. |
| 2 | **A remote-only Base resolves to `origin/<name>`** | `ListAllBranches` strips `origin/`; a bare name fails `rev-parse --verify` |
| 3 | **An alias that names an existing worktree is an error in the status line** | Spec 10.3. Today a duplicate name silently switches to that worktree and exits. |
| 4 | **An in-use branch switches to its worktree on `⏎`**, with the status line `in use by <alias> · ⏎ switches there` | Today's behavior, kept. A UX risk to revisit: `⏎` exits the TUI. |
| 5 | **`esc` does nothing while creating** | Today `esc` closes the wizard mid-create while the goroutine keeps running |
| 6 | **The PR browser's branch is treated as remote even if not yet fetched** | Otherwise an unfetched PR branch would be created as a new branch from Base |
| 7 | **Alias derivation uses `tui.worktree_name_from_branch`** for every entry point | Today the PR prefill ignores it |
| 8 | **`skip_branch_notice` and `default_branch_action` are no longer used by the TUI** | The split/fork question is gone. The keys still parse, so configs do not break. Documented in 3a. |

## Rules by mode (3a builds New; 3b adds the rest)

| | New | Copy (3b) | Switch (3b) |
|---|---|---|---|
| Branch suggestions | Local and remote-only branches; `+ new "<text>"` when no exact match | None: Branch must be new | Local only, in-use excluded |
| In-use branch | Allowed: `⏎` switches there | Validation error | Excluded |
| Base | Shown when the branch is new; default branch | Hidden (source HEAD) | Hidden |
| Alias | Derived from the branch's last segment until edited | Same | Hidden |
| Changes | Hidden | move / copy / leave; default **copy**; `⏎` blocked until the WIP check returns | stash, automatic when dirty; `⏎` blocked until the WIP check returns |

## Review Focus

1. **A branch typed exactly as an in-use branch** must offer to switch, never create a second worktree on it. Test: Task 4, `TestBranchForm_InUseBranchSwitches`.
2. **The PR's branch is not in the local branch list** (never fetched). The form must still use it as an existing branch. Test: Task 6, `TestOpenBranchFormForPR_TreatsBranchAsRemote`.
3. **Short terminal (80x20) or narrow terminal (60x24)**: the title, status, and key line stay visible, and the frame size never changes between states. Test: Task 5, `TestBranchForm_FrameSizeIsFixed`.
4. **Clearing the alias by hand** must not refill it from the branch. Test: Task 4, `TestBranchForm_ClearedAliasStaysEmpty`.
5. **`esc` while creating** must not close the form or drop the activity log. Test: Task 4, `TestBranchForm_IgnoresKeysWhileBusy`.

---

## File Structure

| File | Change | Responsibility |
|---|---|---|
| `internal/tui/overlay_layout.go` | Create (move) | `overlayIndent`, `padToHeight`, `indentBlock`, `calcOverlayWidth`, `overlayDims`, `calcOverlayDims` |
| `internal/tui/compositor.go` | Modify | `composeOverlayAt` (top-anchored) |
| `internal/tui/branch_form.go` | Create | Form state, rules, key handling |
| `internal/tui/branch_form_view.go` | Create | Fixed-frame rendering |
| `internal/tui/branch_form_test.go` | Create | Rules and key tests |
| `internal/tui/branch_form_view_test.go` | Create | Frame size tests and goldens |
| `internal/tui/model.go`, `view_prs.go`, `view_issues.go`, `help_overlay.go`, `help_v2.go`, `messages.go`, `view_prs_v2.go` | Modify | Wiring; delete the wizard |
| `internal/tui/overlay_create.go`, `overlay_create_v2.go` | Delete | The old wizard |
| Wizard tests (see Task 6) | Modify or delete | Keep backend tests; delete step-UI tests |
| `docs/TUI.md`, `docs/CONFIGURATION_REFERENCE.md`, `docs/WIZARD_UX_RESEARCH.md`, `CHANGELOG.md` | Modify | Docs |

---

### Task 1: Workspace and baseline

**Files:** none changed.

- [ ] **Step 1: Workspace, environment, and gate**

```bash
SKILLS=/home/$USER/.claude/plugins/cache/claude-plugins-official/superpowers/6.4.1/skills
W=$($SKILLS/subagent-driven-development/scripts/sdd-workspace docs/superpowers/plans/2026-10-11-tui-redesign-phase-3a.md)
cat > $W/goenv.sh <<EOF
export GOPATH=$TMPDIR/gopath GOMODCACHE=$TMPDIR/gopath/pkg/mod GOCACHE=$TMPDIR/gocache GOFLAGS=-modcacherw
export GOLANGCI_LINT_CACHE=$TMPDIR/golangci-cache TMUX_TMPDIR=$TMPDIR
EOF
cat > $W/gate.sh <<'EOF'
#!/usr/bin/env bash
set -e
W=.superpowers/sdd/2026-10-11-tui-redesign-phase-3a
. $W/goenv.sh
mise exec -- $TMPDIR/gopath/bin/golangci-lint run --timeout=5m
mise exec -- go test -race -cover $(mise exec -- go list ./... | grep -v -E 'internal/(shell|tmux)$')
EOF
chmod +x $W/gate.sh
```

Install golangci-lint v2.14 if `$TMPDIR/gopath/bin/golangci-lint` is missing: `mise exec -- env GOBIN=$TMPDIR/gopath/bin go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.14.0`.

- [ ] **Step 2: Baseline**

Run: `$W/gate.sh`
Expected: `0 issues.`, every package `ok`.

---

### Task 2: Move the shared overlay helpers

`overlay_create_v2.go` holds helpers that sync, rename, config, and the exists prompt also use. Task 6 deletes that file, so the helpers move first.

**Files:**
- Create: `internal/tui/overlay_layout.go`
- Modify: `internal/tui/overlay_create_v2.go`

**Interfaces:**
- Produces: the same names, unchanged: `overlayIndent`, `padToHeight`, `indentBlock`, `calcOverlayWidth`, `overlayDims`, `calcOverlayDims`.

- [ ] **Step 1: Move the declarations**

Cut, unchanged, from `overlay_create_v2.go` into a new `overlay_layout.go` (`package tui`, importing what they need):

- the `overlayIndent` const and its comment
- `padToHeight`, `indentBlock`, `calcOverlayWidth`
- the `overlayDims` type
- `calcOverlayDims`

- [ ] **Step 2: Gate and commit**

Run: `$W/gate.sh`. Expected: green, no golden changes. A pure move has no new test; the existing suite is the check.

```bash
git add internal/tui/overlay_layout.go internal/tui/overlay_create_v2.go
timeout --kill-after=5s 120s git -c commit.gpgsign=true commit -m "refactor(tui): move shared overlay layout helpers into their own file"
```

---

### Task 3: Top-anchored overlays

**Files:**
- Modify: `internal/tui/compositor.go`
- Modify: `internal/tui/compositor_test.go`

**Interfaces:**
- Produces: `func composeOverlayAt(bg, overlay string, width, height, top int) string`. A `top` below 0 centers vertically. `composeOverlay` becomes `composeOverlayAt(bg, overlay, width, height, -1)`.

- [ ] **Step 1: Failing tests**

Append to `internal/tui/compositor_test.go`:

```go
func TestComposeOverlayAt_AnchorsAtRow(t *testing.T) {
	bg := strings.TrimSuffix(strings.Repeat("..........\n", 6), "\n")
	lines := strings.Split(ansi.Strip(composeOverlayAt(bg, "XX", 10, 6, 1)), "\n")
	if lines[1] != "....XX...." {
		t.Errorf("row 1 = %q, want the overlay at the top anchor", lines[1])
	}
	if lines[2] != ".........." {
		t.Errorf("row 2 = %q, want background", lines[2])
	}
}

func TestComposeOverlayAt_ShiftsUpWhenTooTall(t *testing.T) {
	overlay := "A\nB\nC\nD"
	lines := strings.Split(ansi.Strip(composeOverlayAt("", overlay, 4, 5, 3)), "\n")
	if len(lines) != 5 || strings.TrimSpace(lines[4]) != "D" {
		t.Errorf("got %q; want 5 rows ending with D", lines)
	}
}

func TestComposeOverlayAt_NegativeTopCenters(t *testing.T) {
	if composeOverlayAt("bg", "X", 10, 5, -1) != composeOverlay("bg", "X", 10, 5) {
		t.Error("top -1 should match composeOverlay")
	}
}
```

Run: `mise exec -- go test ./internal/tui/ -run TestComposeOverlayAt -v`
Expected: FAIL, `undefined: composeOverlayAt`.

- [ ] **Step 2: Implement**

In `internal/tui/compositor.go`, replace `composeOverlay` with:

```go
// composeOverlay draws overlay centered over a dimmed copy of bg.
func composeOverlay(bg, overlay string, width, height int) string {
	return composeOverlayAt(bg, overlay, width, height, -1)
}

// composeOverlayAt draws overlay over a dimmed copy of bg, horizontally
// centered, with its top edge at row top (top < 0 centers vertically). An
// overlay that would run off the bottom moves up. The result is exactly
// height lines, none wider than width. Background cells outside the
// overlay's rectangle are kept.
func composeOverlayAt(bg, overlay string, width, height, top int) string {
	if width <= 0 || height <= 0 {
		return overlay
	}
	box := strings.Join(fitLines(overlay, width, min(lipgloss.Height(overlay), height)), "\n")
	boxH := lipgloss.Height(box)
	x := max(0, (width-lipgloss.Width(box))/2)
	y := (height - boxH) / 2
	if top >= 0 {
		y = top
	}
	y = max(0, min(y, height-boxH))
	return lipgloss.NewCompositor(
		lipgloss.NewLayer(dimBackdrop(bg, width, height)),
		lipgloss.NewLayer(box).X(x).Y(y).Z(1),
	).Render()
}
```

Run: `mise exec -- go test ./internal/tui/ -run 'TestComposeOverlay' -v`
Expected: PASS, including every existing `TestComposeOverlay_*`.

- [ ] **Step 3: Gate and commit**

`$W/gate.sh` green, no golden changes.

```bash
git add internal/tui/compositor.go internal/tui/compositor_test.go
timeout --kill-after=5s 120s git -c commit.gpgsign=true commit -m "feat(tui): anchor an overlay at a fixed row"
```

---

### Task 4: Form state and rules

**Files:**
- Create: `internal/tui/branch_form.go`
- Create: `internal/tui/branch_form_test.go`

**Interfaces:**
- Consumes: `KeyMap` (keys.go), `WorktreeItem`, `ValidateWorktreeName(string) string`, `checkDuplicateWorktree(string, []WorktreeItem) *WorktreeItem`, `worktree.DeriveWorktreeName(branch, strategy string) string`, `ActivityLog`.
- Produces, used by Tasks 5 and 6:
  - `type branchFormOptions struct { ProjectName, NamePattern, NameStrategy, DefaultBase string; AllBranches, LocalBranches []string; Worktrees []WorktreeItem }`
  - `type branchFormPrefill struct { Branch, Alias string; RemoteBranch, FocusAlias bool }`
  - `func newBranchForm(opts branchFormOptions) *BranchFormState`
  - `func (s *BranchFormState) applyPrefill(p branchFormPrefill) tea.Cmd`
  - `func (s *BranchFormState) handleKey(msg tea.KeyPressMsg, keys KeyMap) (formResult, tea.Cmd)` with `formContinue`, `formSubmit`, `formCancel`
  - `func (s *BranchFormState) intent() formIntent` with kinds `intentSwitchTo`, `intentCreateExisting`, `intentCreateNew`
  - `func (s *BranchFormState) status() (string, formStatusKind)`
  - `func (s *BranchFormState) suggestions() []formSuggestion`
  - It implements `creationTracker`: `getActivityLog()`, `setCreatingDone(string)`.
  - Fields read by the view: `Branch`, `Base`, `Alias` (textinputs), `Focus`, `Pick`, `Busy`, `Log`; methods `isNewBranch()`, `aliasValue()`, `title()`, `keyLine()`.

- [ ] **Step 1: Failing tests**

Create `internal/tui/branch_form_test.go`:

```go
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
```

Run: `mise exec -- go test ./internal/tui/ -run TestBranchForm -v`
Expected: FAIL, `undefined: newBranchForm`.

- [ ] **Step 2: Implement the form**

Create `internal/tui/branch_form.go`:

```go
package tui

import (
	"fmt"
	"strings"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"

	"github.com/lost-in-the/grove/internal/worktree"
)

type formMode int

const (
	formModeNew formMode = iota
)

type formField int

const (
	fieldBranch formField = iota
	fieldBase
	fieldAlias
)

type formResult int

const (
	formContinue formResult = iota
	formSubmit
	formCancel
)

type formIntentKind int

const (
	intentNone formIntentKind = iota
	intentSwitchTo
	intentCreateExisting
	intentCreateNew
)

type formIntent struct {
	Kind    formIntentKind
	Alias   string
	Branch  string
	FromRef string
	Target  *WorktreeItem
}

type formStatusKind int

const (
	formStatusHint formStatusKind = iota
	formStatusReady
	formStatusInfo
	formStatusError
)

type formSuggestion struct {
	Branch string
	Note   string
	New    bool
}

type branchFormOptions struct {
	ProjectName   string
	NamePattern   string
	NameStrategy  string
	DefaultBase   string
	AllBranches   []string
	LocalBranches []string
	Worktrees     []WorktreeItem
}

type branchFormPrefill struct {
	Branch       string
	Alias        string
	RemoteBranch bool
	FocusAlias   bool
}

// BranchFormState is the single-screen form behind new, copy, and switch.
// Every rule lives here so it can be tested without a Model.
type BranchFormState struct {
	Mode formMode
	opts branchFormOptions

	order      []string
	local      map[string]bool
	remoteOnly map[string]bool
	inUse      map[string]*WorktreeItem

	Branch textinput.Model
	Base   textinput.Model
	Alias  textinput.Model

	aliasEdited bool
	Focus       formField
	Pick        int

	Busy   bool
	Log    *ActivityLog
	Status string
}

func newBranchForm(opts branchFormOptions) *BranchFormState {
	s := &BranchFormState{
		Mode:       formModeNew,
		opts:       opts,
		local:      map[string]bool{},
		remoteOnly: map[string]bool{},
		inUse:      map[string]*WorktreeItem{},
	}
	for _, b := range opts.LocalBranches {
		s.local[b] = true
	}
	seen := map[string]bool{}
	for _, b := range opts.AllBranches {
		if seen[b] {
			continue
		}
		seen[b] = true
		s.order = append(s.order, b)
		if !s.local[b] {
			s.remoteOnly[b] = true
		}
	}
	for i := range opts.Worktrees {
		if b := opts.Worktrees[i].Branch; b != "" {
			s.inUse[b] = &opts.Worktrees[i]
		}
	}
	s.Branch = newFormInput("type to search or create")
	s.Base = newFormInput("")
	s.Alias = newFormInput("")
	s.Base.SetValue(opts.DefaultBase)
	s.setFocus(fieldBranch)
	return s
}

func newFormInput(placeholder string) textinput.Model {
	ti := textinput.New()
	ti.Prompt = ""
	ti.Placeholder = placeholder
	ti.CharLimit = 100
	return ti
}

func (s *BranchFormState) applyPrefill(p branchFormPrefill) tea.Cmd {
	if p.Branch != "" {
		if p.RemoteBranch && !s.known(p.Branch) {
			s.remoteOnly[p.Branch] = true
			s.order = append(s.order, p.Branch)
		}
		s.Branch.SetValue(p.Branch)
		s.Branch.CursorEnd()
		s.syncAlias()
	}
	if p.Alias != "" {
		s.Alias.SetValue(p.Alias)
		s.Alias.CursorEnd()
		s.aliasEdited = true
	}
	if p.FocusAlias {
		return s.setFocus(fieldAlias)
	}
	return nil
}

func (s *BranchFormState) title() string { return "New worktree" }

func (s *BranchFormState) branchValue() string { return strings.TrimSpace(s.Branch.Value()) }
func (s *BranchFormState) baseValue() string   { return strings.TrimSpace(s.Base.Value()) }
func (s *BranchFormState) aliasValue() string  { return strings.TrimSpace(s.Alias.Value()) }

func (s *BranchFormState) known(b string) bool { return s.local[b] || s.remoteOnly[b] }

// isNewBranch reports whether the Branch field names a branch that does not
// exist yet, which is the only case where Base applies.
func (s *BranchFormState) isNewBranch() bool {
	b := s.branchValue()
	return b != "" && !s.known(b)
}

func (s *BranchFormState) derivedAlias() string {
	if b := s.branchValue(); b != "" {
		return worktree.DeriveWorktreeName(b, s.opts.NameStrategy)
	}
	return ""
}

func (s *BranchFormState) syncAlias() {
	if s.aliasEdited {
		return
	}
	s.Alias.SetValue(s.derivedAlias())
	s.Alias.CursorEnd()
}

func (s *BranchFormState) noteFor(b string) string {
	if w := s.inUse[b]; w != nil {
		return "in use: " + w.ShortName
	}
	if s.remoteOnly[b] {
		return "remote"
	}
	return "existing"
}

func (s *BranchFormState) matchBranches(q string) []formSuggestion {
	ql := strings.ToLower(q)
	var out []formSuggestion
	for _, b := range s.order {
		if ql == "" || strings.Contains(strings.ToLower(b), ql) {
			out = append(out, formSuggestion{Branch: b, Note: s.noteFor(b)})
		}
	}
	return out
}

// suggestions lists the rows for the focused field.
func (s *BranchFormState) suggestions() []formSuggestion {
	switch s.Focus {
	case fieldBranch:
		q := s.branchValue()
		out := s.matchBranches(q)
		if q != "" && !s.known(q) {
			out = append(out, formSuggestion{Branch: q, Note: "new from " + s.baseValue(), New: true})
		}
		return out
	case fieldBase:
		return s.matchBranches(s.baseValue())
	}
	return nil
}

func (s *BranchFormState) fieldOrder() []formField {
	if s.isNewBranch() {
		return []formField{fieldBranch, fieldBase, fieldAlias}
	}
	return []formField{fieldBranch, fieldAlias}
}

func (s *BranchFormState) input(f formField) *textinput.Model {
	switch f {
	case fieldBase:
		return &s.Base
	case fieldAlias:
		return &s.Alias
	}
	return &s.Branch
}

func (s *BranchFormState) setFocus(f formField) tea.Cmd {
	s.Branch.Blur()
	s.Base.Blur()
	s.Alias.Blur()
	s.Focus = f
	s.Pick = 0
	return s.input(f).Focus()
}

func (s *BranchFormState) moveFocus(delta int) tea.Cmd {
	order := s.fieldOrder()
	idx := 0
	for i, f := range order {
		if f == s.Focus {
			idx = i
		}
	}
	return s.setFocus(order[(idx+delta+len(order))%len(order)])
}

func (s *BranchFormState) movePick(delta int) {
	n := len(s.suggestions())
	if n == 0 {
		s.Pick = 0
		return
	}
	s.Pick = max(0, min(s.Pick+delta, n-1))
}

// acceptPick copies the highlighted suggestion into the focused field.
func (s *BranchFormState) acceptPick() {
	sg := s.suggestions()
	if s.Pick < 0 || s.Pick >= len(sg) || sg[s.Pick].New {
		return
	}
	in := s.input(s.Focus)
	in.SetValue(sg[s.Pick].Branch)
	in.CursorEnd()
	if s.Focus == fieldBranch {
		s.syncAlias()
	}
}

// validate returns the first invalid field and why. An in-use branch is
// valid: submitting it switches to the worktree already using it.
func (s *BranchFormState) validate() (formField, string, bool) {
	b := s.branchValue()
	if b == "" {
		return fieldBranch, "type a branch name", false
	}
	if s.inUse[b] != nil {
		return fieldBranch, "", true
	}
	if !s.known(b) {
		base := s.baseValue()
		if base == "" || !s.known(base) {
			return fieldBase, fmt.Sprintf("base branch %q not found", base), false
		}
	}
	a := s.aliasValue()
	if a == "" {
		return fieldAlias, "type an alias", false
	}
	if msg := ValidateWorktreeName(a); msg != "" {
		return fieldAlias, msg, false
	}
	if dup := checkDuplicateWorktree(a, s.opts.Worktrees); dup != nil {
		return fieldAlias, fmt.Sprintf("alias %q is taken by %s", a, dup.ShortName), false
	}
	return fieldBranch, "", true
}

func (s *BranchFormState) intent() formIntent {
	b := s.branchValue()
	if w := s.inUse[b]; w != nil {
		return formIntent{Kind: intentSwitchTo, Branch: b, Target: w}
	}
	a := s.aliasValue()
	if s.known(b) {
		return formIntent{Kind: intentCreateExisting, Alias: a, Branch: b}
	}
	ref := s.baseValue()
	if s.remoteOnly[ref] {
		ref = "origin/" + ref
	}
	return formIntent{Kind: intentCreateNew, Alias: a, Branch: b, FromRef: ref}
}

func (s *BranchFormState) status() (string, formStatusKind) {
	if s.Busy {
		return "creating…", formStatusInfo
	}
	if s.Status != "" {
		return s.Status, formStatusError
	}
	if w := s.inUse[s.branchValue()]; w != nil {
		return fmt.Sprintf("in use by %s · ⏎ switches there", w.ShortName), formStatusInfo
	}
	if _, msg, ok := s.validate(); !ok {
		return msg, formStatusHint
	}
	return "✓ ready", formStatusReady
}

func (s *BranchFormState) keyLine() string {
	if s.Busy {
		return "creating…"
	}
	act := "⏎ create"
	if s.inUse[s.branchValue()] != nil {
		act = "⏎ switch"
	}
	return act + " · tab field · ctrl+j/k pick · esc back"
}

// handleKey applies one key. Navigation keys are explicit strings, never the
// global Up/Down bindings, because those include j and k, which must type.
func (s *BranchFormState) handleKey(msg tea.KeyPressMsg, keys KeyMap) (formResult, tea.Cmd) {
	if s.Busy {
		return formContinue, nil
	}
	switch k := msg.String(); {
	case key.Matches(msg, keys.Escape):
		return formCancel, nil
	case k == "tab":
		s.acceptPick()
		return formContinue, s.moveFocus(1)
	case k == "shift+tab":
		return formContinue, s.moveFocus(-1)
	case k == "up" || k == "ctrl+k":
		s.movePick(-1)
		return formContinue, nil
	case k == "down" || k == "ctrl+j":
		s.movePick(1)
		return formContinue, nil
	case k == "enter":
		if f, _, ok := s.validate(); !ok {
			return formContinue, s.setFocus(f)
		}
		s.Status = ""
		return formSubmit, nil
	}
	return formContinue, s.typeInto(msg)
}

func (s *BranchFormState) typeInto(msg tea.KeyPressMsg) tea.Cmd {
	in := s.input(s.Focus)
	var cmd tea.Cmd
	*in, cmd = in.Update(msg)
	s.Status = ""
	switch s.Focus {
	case fieldBranch:
		s.Pick = 0
		s.syncAlias()
	case fieldBase:
		s.Pick = 0
	case fieldAlias:
		s.aliasEdited = s.aliasValue() != s.derivedAlias()
	}
	return cmd
}

func (s *BranchFormState) getActivityLog() *ActivityLog { return s.Log }

func (s *BranchFormState) setCreatingDone(errMsg string) {
	s.Busy = false
	s.Status = errMsg
}
```

- [ ] **Step 3: Run the tests**

Run: `mise exec -- go test ./internal/tui/ -run TestBranchForm -v`
Expected: PASS.

Likely mismatches and their meaning:

- `TestBranchForm_ArrowsMovePick` needs at least 3 suggestions with an empty query. `order` has 4, so `Pick` reaches 2.
- If `makeKeyMsg("shift+tab")` or `makeKeyMsg("backspace")` does not produce `shift+tab`/`backspace` from `msg.String()`, extend `makeKeyMsg` in `helpers_test.go` with `case "shift+tab": return tea.KeyPressMsg{Code: tea.KeyTab, Mod: tea.ModShift}`. `backspace` already exists. Record it in the ledger.

- [ ] **Step 4: Gate and commit**

```bash
git add internal/tui/branch_form.go internal/tui/branch_form_test.go internal/tui/helpers_test.go
timeout --kill-after=5s 120s git -c commit.gpgsign=true commit -m "feat(tui): add the branch form state with suggestion and validation rules

Not wired in yet. One form decides from what is typed whether to
create from an existing branch, create a new branch from Base, or
switch to the worktree already using the branch."
```

---

### Task 5: Fixed-frame rendering

**Files:**
- Create: `internal/tui/branch_form_view.go`
- Create: `internal/tui/branch_form_view_test.go`

**Interfaces:**
- Consumes: Task 4.
- Produces:
  - `const formTop = 2` (the frame's top row)
  - `func branchFormSize(termW, termH int) (int, int)`
  - `func renderBranchForm(s *BranchFormState, termW, termH int, spinner string) string`
  - `func formLogWidth(termW int) int`

**Frame budget:** header 2 rows + frame + footer 1 row must fit 80x20, so the frame is at most 17 rows. The design height is 14: border 2, title 1, body 9, status 1, keys 1. Below 14 available rows the body scrolls and the rest stays.

- [ ] **Step 1: Failing tests**

Create `internal/tui/branch_form_view_test.go`:

```go
package tui

import (
	"strings"
	"testing"

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
			s.Log = NewActivityLog(formLogWidth(80), 6)
			s.Log.AddLine("Creating worktree 'abc-12'...")
			return s
		}},
	}
}

var formSizes = []termSize{
	{"short_80x20", 80, 20},
	{"standard_80x24", 80, 24},
	{"narrow_60x24", 60, 24},
	{"wide_120x40", 120, 40},
}

func TestBranchForm_FrameSizeIsFixed(t *testing.T) {
	for _, sz := range formSizes {
		wantW, wantH := branchFormSize(sz.width, sz.height)
		for _, c := range formCases() {
			out := renderBranchForm(c.build(), sz.width, sz.height, "⠋")
			if w, h := lipgloss.Width(out), lipgloss.Height(out); w != wantW || h != wantH {
				t.Errorf("%s/%s: frame %dx%d, want %dx%d", sz.name, c.name, w, h, wantW, wantH)
			}
		}
	}
}

func TestBranchForm_FitsEightyByTwenty(t *testing.T) {
	_, h := branchFormSize(80, 20)
	if formTop+h+1 > 20 {
		t.Errorf("frame of %d rows at top %d leaves no room for the footer in 20 rows", h, formTop)
	}
}

func TestBranchForm_KeyLineVisibleWhenShort(t *testing.T) {
	out := ansi.Strip(renderBranchForm(newBranchForm(testFormOptions()), 80, 12, "⠋"))
	if !strings.Contains(out, "esc back") {
		t.Errorf("key line missing at 80x12:\n%s", out)
	}
}

func TestBranchForm_PreviewShowsFolderAndSession(t *testing.T) {
	s := newBranchForm(testFormOptions())
	s.Branch.SetValue("fix/login")
	s.syncAlias()
	out := ansi.Strip(renderBranchForm(s, 120, 40, "⠋"))
	if !strings.Contains(out, "grove-login") {
		t.Errorf("preview missing folder name:\n%s", out)
	}
}

func TestGolden_BranchForm(t *testing.T) {
	for _, sz := range formSizes {
		for _, c := range formCases() {
			t.Run(sz.name+"/"+c.name, func(t *testing.T) {
				goldenModel(t, sz)
				golden.RequireEqual(t, []byte(renderBranchForm(c.build(), sz.width, sz.height, "⠋")))
			})
		}
	}
}
```

`goldenModel` is called only for its NO_COLOR setup and lock; the render does not use the model.

Run: `mise exec -- go test ./internal/tui/ -run 'TestBranchForm_(Frame|Fits|KeyLine|Preview)' -v`
Expected: FAIL, `undefined: renderBranchForm`.

- [ ] **Step 2: Implement**

Create `internal/tui/branch_form_view.go`:

```go
package tui

import (
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/lost-in-the/grove/internal/worktree"
)

const (
	formTop         = 2
	formMinWidth    = 56
	formMaxWidth    = 76
	formHeight      = 14
	formSuggestRows = 4
	formLabelWidth  = 9
)

// branchFormSize is the frame's width and height for a terminal size. It
// depends only on the terminal, never on the form's state, so the frame
// cannot change size while the user works in it.
func branchFormSize(termW, termH int) (int, int) {
	w := min(max(termW*60/100, formMinWidth), formMaxWidth)
	w = min(w, termW-2)
	h := min(formHeight, termH-formTop-1)
	return max(w, 24), max(h, 6)
}

func formLogWidth(termW int) int {
	w, _ := branchFormSize(termW, formHeight+formTop+1)
	return w - 4
}

// renderBranchForm draws the form at exactly branchFormSize. The title,
// status line, and key line are pinned; the body scrolls between them to
// keep the focused field visible when the terminal is short.
func renderBranchForm(s *BranchFormState, termW, termH int, spinner string) string {
	w, h := branchFormSize(termW, termH)
	inner := w - 4
	bodyRows := h - 5

	body, focusLine := s.bodyLines(inner, spinner)
	body = windowLines(body, bodyRows, focusLine)

	msg, kind := s.status()
	lines := []string{Styles.OverlayTitle.Render(s.title())}
	lines = append(lines, body...)
	lines = append(lines, formStatusStyle(kind).Render(msg), Styles.Footer.Render(s.keyLine()))
	for i, l := range lines {
		lines[i] = " " + padCells(l, inner) + " "
	}
	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(Colors.Success).
		Render(strings.Join(lines, "\n"))
}

func formStatusStyle(kind formStatusKind) lipgloss.Style {
	switch kind {
	case formStatusError:
		return Styles.ErrorText
	case formStatusReady:
		return Styles.SuccessText
	case formStatusInfo:
		return Styles.WarningText
	}
	return Styles.DetailDim
}

// padCells truncates or pads s to exactly n terminal cells.
func padCells(s string, n int) string {
	s = ansi.Truncate(s, n, "…")
	if pad := n - lipgloss.Width(s); pad > 0 {
		s += strings.Repeat(" ", pad)
	}
	return s
}

// windowLines returns exactly rows lines of lines, scrolled so that line
// focus stays visible.
func windowLines(lines []string, rows, focus int) []string {
	if len(lines) <= rows {
		for len(lines) < rows {
			lines = append(lines, "")
		}
		return lines
	}
	start := max(0, min(focus-rows/2, len(lines)-rows))
	return lines[start : start+rows]
}

func (s *BranchFormState) label(name string, f formField) string {
	marker := "  "
	if s.Focus == f {
		marker = Styles.ListCursor.Render("❯ ")
	}
	return marker + Styles.DetailLabel.Render(fmt.Sprintf("%-7s", name))
}

// bodyLines returns the body and the index of the focused field's line.
func (s *BranchFormState) bodyLines(inner int, spinner string) ([]string, int) {
	if s.Busy && s.Log != nil {
		return strings.Split(s.Log.View(spinner), "\n"), 0
	}
	indent := strings.Repeat(" ", formLabelWidth)
	lines := []string{s.label("Branch", fieldBranch) + s.Branch.View()}
	lines = append(lines, s.suggestionLines(indent, inner)...)
	if s.isNewBranch() {
		lines = append(lines, s.label("Base", fieldBase)+s.Base.View())
	} else {
		lines = append(lines, s.label("Base", fieldBase)+Styles.DetailDim.Render("n/a · existing branch"))
	}
	lines = append(lines, s.label("Alias", fieldAlias)+s.Alias.View())
	preview := ""
	if a := s.aliasValue(); a != "" {
		preview = fmt.Sprintf("→ %s · session %s",
			worktree.InterpolateNamePattern(s.opts.NamePattern, s.opts.ProjectName, a),
			worktree.TmuxSessionName(s.opts.ProjectName, a))
	}
	lines = append(lines, indent+Styles.DetailDim.Render(preview), "")

	focus := 0
	switch s.Focus {
	case fieldBase:
		focus = 1 + formSuggestRows
	case fieldAlias:
		focus = 2 + formSuggestRows
	}
	return lines, focus
}

// suggestionLines always returns formSuggestRows lines, so the body never
// changes height.
func (s *BranchFormState) suggestionLines(indent string, inner int) []string {
	out := make([]string, formSuggestRows)
	sg := s.suggestions()
	start := max(0, min(s.Pick-formSuggestRows+1, len(sg)-formSuggestRows))
	for i := 0; i < formSuggestRows && start+i < len(sg); i++ {
		x := sg[start+i]
		cursor := "  "
		if start+i == s.Pick {
			cursor = Styles.ListCursor.Render("❯ ")
		}
		name := x.Branch
		if x.New {
			name = fmt.Sprintf("+ new %q", x.Branch)
		}
		nameW := max(10, inner-formLabelWidth-2-lipgloss.Width(x.Note)-2)
		out[i] = indent + cursor + padCells(name, nameW) + "  " + Styles.DetailDim.Render(x.Note)
	}
	for i := range out {
		if out[i] == "" {
			out[i] = indent
		}
	}
	return out
}
```

Run: `mise exec -- go test ./internal/tui/ -run 'TestBranchForm_(Frame|Fits|KeyLine|Preview)' -v`
Expected: PASS.

- [ ] **Step 3: Goldens**

Run: `mise exec -- go test ./internal/tui/ -run TestGolden_BranchForm -update`, then read every new file under `testdata/TestGolden_BranchForm/` with `cat -v`. Check by eye:

- The frame is the same size in every state at a given terminal size.
- At 80x20 the status and key lines are present.
- The in-use case reads `in use by auth · ⏎ switches there` and `⏎ switch`.
- The creating case shows the activity log line, `creating…` twice (status and keys), and no fields.

Run again without `-update`: PASS.

- [ ] **Step 4: Gate and commit**

`$W/gate.sh` green. `git status --short internal/tui/testdata` shows only the new `TestGolden_BranchForm` directory.

```bash
git add internal/tui/branch_form_view.go internal/tui/branch_form_view_test.go internal/tui/testdata/TestGolden_BranchForm
timeout --kill-after=5s 120s git -c commit.gpgsign=true commit -m "feat(tui): render the branch form in a fixed frame

The frame's size depends only on the terminal. When rows are short, the
title, status, and key line stay put and the fields scroll."
```

---

### Task 6: Wire New mode in and delete the create wizard

**Files:**
- Modify: `internal/tui/model.go`, `view_prs.go`, `view_issues.go`, `view_prs_v2.go`, `help_overlay.go`, `help_v2.go`, `messages.go`, and the test files listed below
- Delete: `internal/tui/overlay_create.go`, `internal/tui/overlay_create_v2.go`, `internal/tui/testdata/TestGolden_Overlay_Create/`

**Interfaces:**
- Consumes: Tasks 3 to 5; `git.ListAllBranches`, `git.ListLocalBranches`, `createWorktreeCmd`, `ResolveDefaultBranch`, `m.worktreeNamePattern()`, `m.existingWorktreeItems()`.
- Produces: `ActiveView` value `ViewBranchForm` (renamed from `ViewCreate`); `Model.branchForm *BranchFormState` (replaces `createState`); `func (m Model) openBranchForm(p branchFormPrefill) (tea.Model, tea.Cmd)`. Phase 3b extends these.

- [ ] **Step 1: Failing model-level tests**

Append to `internal/tui/branch_form_test.go`:

```go
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

func TestBranchFormSubmit_StartsCreation(t *testing.T) {
	m := newTestModel(withItems(3), withSize(120, 40))
	next, _ := m.openBranchForm(branchFormPrefill{Branch: "brand-new"})
	m = next.(Model)
	m.branchForm.opts.DefaultBase = "main"
	m.branchForm.local["main"] = true
	m.branchForm.Base.SetValue("main")
	next, cmd := m.Update(makeKeyMsg("enter"))
	m = next.(Model)
	if !m.branchForm.Busy || m.branchForm.Log == nil || cmd == nil {
		t.Errorf("after enter: busy=%v log=%v cmd=%v, want creation started", m.branchForm.Busy, m.branchForm.Log != nil, cmd != nil)
	}
	if !m.isOperationAnimating() {
		t.Error("spinner should animate while creating")
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
```

Add `var errTestBoom = errors.New("boom")` to the test file and `"errors"` to its imports. `makeTestItems` gives worktree `feature-auth` the branch `feature-auth`, so typing that branch is an in-use case.

Run: `mise exec -- go test ./internal/tui/ -run 'TestDashboardN_OpensBranchForm|TestOpenBranchFormForPR|TestBranchForm(Submit|CreationDone|InUse)' -v`
Expected: FAIL to build: `undefined: ViewBranchForm`, `m.branchForm undefined`, `m.openBranchForm undefined`.

- [ ] **Step 2: Rename the view and swap the field**

```bash
cd internal/tui
mise exec -- gofmt -r 'ViewCreate -> ViewBranchForm' -w *.go
```

Then in `model.go`: rename the field `createState *CreateState` to `branchForm *BranchFormState`. Every remaining `m.createState` reference is old wizard code that Step 4 deletes or Step 3 replaces.

- [ ] **Step 3: Open, key, submit, render**

Add to `internal/tui/branch_form.go`:

```go
// openBranchForm opens the form in New mode over the current view.
func (m Model) openBranchForm(p branchFormPrefill) (tea.Model, tea.Cmd) {
	all, err := git.ListAllBranches(m.projectRoot)
	if err != nil {
		tuilog.Printf("warning: failed to list branches: %v", err)
	}
	local, err := git.ListLocalBranches(m.projectRoot)
	if err != nil {
		tuilog.Printf("warning: failed to list local branches: %v", err)
	}
	strategy := ""
	if m.cfg != nil {
		strategy = m.cfg.TUI.WorktreeNameFromBranch
	}
	s := newBranchForm(branchFormOptions{
		ProjectName:   m.projectName,
		NamePattern:   m.worktreeNamePattern(),
		NameStrategy:  strategy,
		DefaultBase:   ResolveDefaultBranch(m.cfg),
		AllBranches:   all,
		LocalBranches: local,
		Worktrees:     m.existingWorktreeItems(),
	})
	cmd := s.applyPrefill(p)
	m.branchForm = s
	m.pushView(ViewBranchForm)
	return m, cmd
}

func (m Model) handleBranchFormKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	s := m.branchForm
	if s == nil {
		m.popView()
		return m, nil
	}
	res, cmd := s.handleKey(msg, m.keys)
	switch res {
	case formCancel:
		m.branchForm = nil
		m.popView()
		return m, nil
	case formSubmit:
		return m.submitBranchForm(s)
	}
	return m, cmd
}

func (m Model) submitBranchForm(s *BranchFormState) (tea.Model, tea.Cmd) {
	in := s.intent()
	switch in.Kind {
	case intentSwitchTo:
		m.switchTo = in.Target.Path
		m.switchToDisplayName = in.Target.displayName()
		m.branchForm = nil
		m.popToRoot()
		return m, tea.Quit
	case intentCreateExisting, intentCreateNew:
		s.Busy = true
		s.Status = ""
		s.Log = NewActivityLog(formLogWidth(m.width), 6)
		existing, newBranch := "", ""
		if in.Kind == intentCreateExisting {
			existing = in.Branch
		} else {
			newBranch = in.Branch
		}
		return m, tea.Batch(m.spinner.Tick,
			createWorktreeCmd(m.worktreeMgr, m.stateMgr, m.cfg, m.projectRoot, in.Alias, existing, newBranch, in.FromRef))
	}
	return m, nil
}
```

Add the imports `github.com/lost-in-the/grove/internal/git` and `github.com/lost-in-the/grove/internal/tuilog`.

In `model.go`:

1. `handleDashboardNewKey` body becomes `return m.openBranchForm(branchFormPrefill{})`.
2. `handleKey`'s dispatch: `case ViewBranchForm: return m.handleBranchFormKey(msg)`.
3. `creationTrackerForSource`: the `"create"` case returns `m.branchForm, "", func() { m.branchForm = nil }` when `m.branchForm != nil`. Delete the `"pr"` and `"issue"` cases.
4. `isOperationAnimating`: replace the `createState` clause with `(m.branchForm != nil && m.branchForm.Busy)`.
5. `renderOverlay`: `case ViewBranchForm: if m.branchForm != nil { return renderBranchForm(m.branchForm, m.width, m.height, m.spinner.View()) }`.
6. `renderFrame`: before the final `composeOverlay`, add

```go
	if v == ViewBranchForm {
		return composeOverlayAt(m.renderFrame(i-1), overlay, m.width, m.height, formTop)
	}
```

In `view_prs.go`, `openCreateWizardForPR` body becomes `return m.openBranchForm(branchFormPrefill{Branch: pr.Branch, RemoteBranch: true, FocusAlias: true})`.

In `view_issues.go`, `openCreateWizardForIssue` body becomes `return m.openBranchForm(branchFormPrefill{Alias: fmt.Sprintf("issue-%d", issue.Number)})`.

- [ ] **Step 4: Delete the wizard**

1. `git rm internal/tui/overlay_create.go internal/tui/overlay_create_v2.go`.
2. Delete these functions from `model.go`: `handleCreateKey`, `handleBranchChoiceKey`, `handleBranchSelectKey`, `handleBranchCreateKey`, `handleBranchActionKey`, `handleNameKey`, `handleConfirmKey`, `selectExistingBranch`, `startCreate`, `handleWorktreeCreated`, and the `case worktreeCreatedMsg:` dispatch line.
3. Delete `worktreeCreatedMsg` from `messages.go`.
4. Delete `prefillCreateStateForPR` and `prefillCreateStateForIssue` from `helpers.go`.
5. Delete `renderCreatingDetail` (`view_prs_v2.go`, `view_issues.go`) and its call sites, and the `getActivityLog`/`setCreatingDone` methods on `PRViewState` and `IssueViewState` along with any `Creating`/`ActivityLog`/`Error` fields only they used. Delete each only after `grep` shows no remaining reader.
6. In `help_overlay.go`, rename `helpSectionsCreate` to `helpSectionsBranchForm` and replace its entries with:

```go
var helpSectionsBranchForm = []helpSection{
	{title: "New Worktree", note: "Type a branch to search or create. The form shows what will happen.", items: []helpEntry{
		{"tab / shift+tab", "Next / previous field"},
		{"ctrl+j / ctrl+k", "Move through suggestions"},
		{"enter", "Create, or switch to a branch's worktree"},
		{"esc", "Close"},
	}},
}
```

Match the struct literal shape of the surrounding sections exactly. Read `helpSectionsDelete` first.

7. In `help_v2.go`, the hints for `ViewBranchForm` (the old create hints) become `⏎ create`, `tab field`, `ctrl+j/k pick`, `esc back`.

A helper for function deletion, run from the repo root:

```bash
cat > $W/delete_funcs.py <<'EOF'
import re, sys
path, names = sys.argv[1], sys.argv[2:]
s = open(path).read()
for n in names:
    m = re.search(r'(?m)^(//[^\n]*\n)*func (\([^)]*\) )?' + re.escape(n) + r'\(', s)
    if not m:
        sys.exit(f'{path}: {n} not found')
    end = s.index('\n}\n', m.start()) + 3
    s = s[:m.start()] + s[end:].lstrip('\n')
open(path, 'w').write(s)
print(f'{path}: deleted {len(names)}')
EOF
python3 $W/delete_funcs.py internal/tui/model.go handleCreateKey handleBranchChoiceKey handleBranchSelectKey handleBranchCreateKey handleBranchActionKey handleNameKey handleConfirmKey selectExistingBranch startCreate handleWorktreeCreated
```

The regex also removes each function's leading comment block.

- [ ] **Step 5: Build, then migrate tests**

Run: `mise exec -- go build ./... && mise exec -- go vet ./internal/tui/`. Fix production compile errors first; they are leftover references to deleted code.

Then `mise exec -- go vet ./internal/tui/` lists test compile errors. For each failing test function:

**Keep, porting to the form where needed:**
- `TestCreateWorktreeCmd_RemoteBranch_UsesCreateFromBranch` (from `overlay_create_v2_test.go`): move unchanged into a new `internal/tui/create_cmd_test.go`.
- `creation_streaming_test.go`: keep every test except `TestCreationDoneMsg_PR` (dead path). Where a test builds a `CreateState` as the tracker, build `newBranchForm(testFormOptions())` with `Busy = true` and set `m.branchForm` instead.
- `duplicate_validation_test.go`: keep the two `checkDuplicateWorktree` tests; delete the two render tests.
- `integration_test.go`: keep the backend tests (pre-create abort, base branch, new-branch name, bootstrap failure). Port any that open the wizard to `openBranchForm` plus typing.
- `TestToastSuperseding` (`model_test.go`): it uses `worktreeCreatedMsg` only to trigger a success toast. Use `creationDoneMsg{source: "create", name: ...}` with `m.branchForm` set and `Busy = true`.
- The "n opens the create view" assertion in `model_test.go` becomes `ViewBranchForm`.
- `stress_test.go` create cases: rewrite each as "open the form, send its key sequence, assert no panic and the view is `ViewBranchForm` or `ViewDashboard`." Keep the crash-safety intent.
- `program_test.go` teatest create: drive `n`, type a branch, `esc`; assert the form's title appears and disappears.

**Delete:** every remaining test that asserts wizard steps, the stepper inside create, or create render output. That includes all of `overlay_create_v2_test.go` except the moved backend test, `TestWorktreeCreatedMsgWithError`, `TestUpdate_WorktreeCreatedMsg`, `TestCreationDoneMsg_PR`, and `withCreateStep` in `golden_helpers_test.go`.

Delete `TestGolden_Overlay_Create` from `golden_test.go` and `git rm -r internal/tui/testdata/TestGolden_Overlay_Create`.

Record in the ledger each test kept, ported, or deleted, by name.

- [ ] **Step 6: Run the new tests and the gate**

Run: `mise exec -- go test ./internal/tui/ -run 'TestBranchForm|TestDashboardN|TestOpenBranchForm|TestCreation|TestToastSuperseding' -v`
Expected: PASS.

Run: `$W/gate.sh`
Expected: green. `git status --short internal/tui/testdata` shows only the deleted `TestGolden_Overlay_Create`.

- [ ] **Step 7: Commit**

```bash
git add -A internal/tui
timeout --kill-after=5s 120s git -c commit.gpgsign=true commit -m "feat(tui): replace the create wizard with the branch form

n, the PR browser, and the issue browser open one fixed-frame form:
Branch, Base, and Alias. Typing a branch shows whether it exists, is
remote, or is already in use, and the alias and folder preview follow
it. Base defaults to the configured default branch. esc does nothing
while creating. A taken alias is an error instead of a silent switch."
```

---

### Task 7: Docs

**Files:** `docs/TUI.md`, `docs/CONFIGURATION_REFERENCE.md`, `docs/WIZARD_UX_RESEARCH.md`, `CHANGELOG.md`

- [ ] **Step 1: `docs/TUI.md`**

Replace the `### New Worktree (\`n\`)` section with:

```markdown
### New Worktree (`n`)

One form, three fields. It stays the same size and position while you use it.

- **Branch:** type to search local and remote branches. Each row says whether the branch exists, is remote, or is already in use by a worktree. A name that matches nothing is created as a new branch.
- **Base:** for a new branch only. Defaults to the configured default branch.
- **Alias:** the worktree's short name, derived from the branch until you edit it. The line below shows the folder and session name.

The status line says what `enter` will do. If the branch is already in use, `enter` switches to that worktree instead of creating one.

| Key | Action |
|---|---|
| `tab` / `shift+tab` | Next / previous field (`tab` on Branch accepts the highlighted row) |
| `ctrl+j` / `ctrl+k`, `↓` / `↑` | Move through suggestions |
| `enter` | Create, or switch to an in-use branch's worktree |
| `esc` | Close (does nothing while creating) |

The PR browser opens this form with the PR's branch filled in. The issue browser fills in the alias `issue-N`.
```

In the wizard keys table (`### Overlay / Wizard Keys`), add a note that the New Worktree form uses its own keys, listed in its section.

- [ ] **Step 2: `docs/CONFIGURATION_REFERENCE.md`**

For `skip_branch_notice` and `default_branch_action` under `[tui]`, add: "No longer used: the New Worktree form has no split/fork question. The keys are still accepted." Correct `default_base_branch` to say it sets the Base default of the New Worktree form.

- [ ] **Step 3: `docs/WIZARD_UX_RESEARCH.md`**

Add at the top: "Superseded by the branch form (redesign spec section 10, phase 3). Kept as research history."

- [ ] **Step 4: `CHANGELOG.md`**

Under `## [Unreleased]`, `### Changed`:

```markdown
- The New Worktree wizard is now a single form: Branch, Base, and Alias. It never moves or changes size, and its status line says what `enter` will do. A new branch now starts from the configured default branch, not from whatever the main checkout has checked out. An alias already used by another worktree is reported as an error instead of switching to that worktree.
```

- [ ] **Step 5: Commit**

```bash
git add docs/TUI.md docs/CONFIGURATION_REFERENCE.md docs/WIZARD_UX_RESEARCH.md CHANGELOG.md
timeout --kill-after=5s 120s git -c commit.gpgsign=true commit -m "docs(tui): document the New Worktree form"
```

---

### Task 8: Hands-on check, in and out of herdr

**Files:** none changed.

- [ ] **Step 1: Build and a scratch repo**

```bash
mise exec -- go build -o $TMPDIR/grove-dev ./cmd/grove
S=$TMPDIR/grove-scratch; rm -rf "$S"; mkdir -p "$S" && cd "$S" && git init -q -b main && git commit -q --allow-empty -m one && git commit -q --allow-empty -m two
git branch feature/login && git branch fix/typo
$TMPDIR/grove-dev new alpha >/dev/null 2>&1 || true
```

Use `trash` for any later cleanup under `/home`; `$TMPDIR` scratch can be removed with `find "$S" -mindepth 1 -delete`.

- [ ] **Step 2: Paths, run in a tmux session you create (outside herdr) and in a herdr tab you create**

| # | Where | Keys | Expected |
|---|---|---|---|
| 1 | real repo | `n`, type `fea`, `esc` | Form at a fixed top row; rows below the header stay visible beside it; suggestions update; `esc` closes |
| 2 | real repo | `n`, type the branch of an existing worktree | Status reads `in use by <alias> · ⏎ switches there`; do **not** press `⏎` |
| 3 | scratch | `n`, type `fix/typo`, `⏎` | Creates worktree `typo`; returns to the dashboard with it selected |
| 4 | scratch | `n`, type `brand-new`, `⏎` | Creates a new branch from `main`; the activity log shows inside the same frame |
| 5 | scratch | `n`, type `feature/login`, `tab`, change alias to `alpha`, `⏎` | Status error: alias taken by alpha; nothing created |
| 6 | scratch | resize to 80x20 with the form open | Title, status, and key line stay visible; frame does not jump |
| 7 | real repo | `p`, `⏎` on a PR, `esc` | Form opens over the PR browser with the PR branch filled; `esc` returns to the PR browser |

Record each in the ledger as `Task 8: <env> path N: <observed>`. A mismatch is a bug: write a failing test, fix, gate.

- [ ] **Step 3: Clean up**

Quit grove with `q`. Close only the tmux session and herdr tab you created.

---

## Done when

- `make lint test` passes (golangci-lint v2.14).
- No production file mentions `CreateState`, `ViewCreate`, `worktreeCreatedMsg`, or `prefillCreateStateFor`.
- Golden changes: only `TestGolden_Overlay_Create` deleted and `TestGolden_BranchForm` added.
- Task 8's seven paths match, in tmux and in herdr.
