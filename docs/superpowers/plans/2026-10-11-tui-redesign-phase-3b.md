# TUI Redesign Phase 3b (Branch Form, Copy and Switch Modes) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Move copy-worktree (`f`) and switch-branch (`b`) onto the branch form from phase 3a, and delete the fork and checkout wizards.

**Architecture:**

- `BranchFormState` gains two modes. Each mode has its own field order, suggestion rule, validation, and intent. One new field, Changes, is a choice row, not a text input.
- The fork command takes a small `forkRequest` instead of the wizard's `ForkState`, so the form can name the new branch.
- Both modes block `enter` until the uncommitted-changes check for their worktree returns.
- Completion handlers report errors into the form and close it on success, as New mode does.

**Tech Stack:** Go 1.26+, Bubble Tea v2, the phase 3a branch form, the phase 2 view stack.

**Spec:** `docs/superpowers/specs/2026-10-10-tui-redesign-design.md` section 10 (N7).

## Global Constraints

- Gate: `make lint test` with golangci-lint v2.14, run inside the sandbox. Only signed commits run outside it.
- Commits: `timeout --kill-after=5s 120s git -c commit.gpgsign=true commit ...`.
- **Expected golden churn:**
  - `TestGolden_Overlay_Fork` deleted.
  - New `TestGolden_BranchForm` cases for Copy and Switch added.
  - Task 5's "fork" to "copy" relabel: `TestGolden_Dashboard_HelpExpanded`, and any `TestGolden_Component_HelpFooter` or dashboard golden that shows the `f` hint.
  - Task 6: `TestGolden_Overlay_Config` cases that show the Behavior tab.
  - **New-mode branch form goldens must not change.** Any other change is a finding.
- Copy's branch check mirrors New mode's branch-name format check, so a bad name errors in the form.
- Read `checkoutBranchCmd` before writing Task 7 #5's expectation and the "will be stashed" wording.
- No `panic()`. Measure width with `lipgloss.Width`.

## Rules by mode

| | Copy (`f`) | Switch (`b`) |
|---|---|---|
| Title | `Copy <alias>` | `Switch branch of <alias>` |
| Fields (tab order) | Branch, Alias, Changes (only when dirty) | Branch |
| Branch default | `<source branch>-copy` | empty |
| Suggestions | none | local branches not checked out in any worktree |
| Branch rule | must not exist | must be local, not in use, not the current branch |
| Alias | derived from Branch until edited; same checks as New | not shown |
| Changes | `copy` (default), `move`, `leave`; `←`/`→`/`space` cycle | stashed automatically when dirty |
| `enter` | blocked until the changes check returns | same |
| Command | `forkWorktreeCmd(forkRequest)` | `checkoutBranchCmd(path, branch, stash)` |

## Decisions

| # | Decision | Why |
|---|---|---|
| 1 | **Copy's branch is an editable field**, default `<source>-copy` | Today it is forced to `<source>-<name>`. The spec gives Copy a Branch field. |
| 2 | **Changes defaults to `copy`** | Today the WIP step's first row is `move`, which cleans the source. Copy is non-destructive. |
| 3 | **Both modes block `enter` until the changes check returns, and on a failed check** | Today fork can skip the WIP step and move changes the user never chose to move |
| 4 | **Switch lists local branches only** | Matches today. `docs/TUI.md` wrongly says remote-only branches appear. |
| 5 | **The config overlay drops `skip_branch_notice` and `default_branch_action`** | Phase 3a review minor: they no longer do anything. The keys still parse. |
| 6 | **`f` is labelled "copy worktree"** in keys, help, footer, and docs | Spec N13. The CLI `grove fork` is unchanged. |
| 7 | **The PR and issue browsers keep the "worktree exists" prompt.** `enter` focuses it; `n` now opens Copy. | The prompt offers a choice the form does not (focus vs copy). Folding it in is a design change, not a cleanup. |
| 8 | **Phase 2 and 3a deferred minors land here** (Task 7) | They are small, they touch files this phase already edits, and the user asked that they not be dropped. |

## Review Focus

1. **A dirty source and `enter` pressed before the changes check returns.** Nothing may run. Test: Task 3, `TestCopyForm_EnterBlockedUntilWIPChecked`.
2. **A stale changes check from a form opened earlier for another worktree.** It must not mark this form clean. Test: Task 3, `TestCopyForm_IgnoresWIPForOtherPath`.
3. **Switching to the branch the worktree is already on, or to a branch checked out elsewhere.** Both are errors, never a `git switch` call. Test: Task 3, `TestSwitchForm_RejectsCurrentAndInUse`.
4. **Copy with a branch name that already exists.** Error in the form, not a late git failure. Test: Task 3, `TestCopyForm_RejectsExistingBranch`.
5. **A copy that created the worktree but failed later** (WIP apply or bootstrap). The form closes with a warning toast, as today. Test: Task 5, `TestCopyFormComplete_PartialSuccessWarns`.

---

## File Structure

| File | Change | Responsibility |
|---|---|---|
| `internal/tui/fork_cmd.go` | Create (move) | `WIPStrategy`, `forkRequest`, `forkCompleteMsg`, `forkWorktreeCmd`, its helpers |
| `internal/tui/checkout_cmd.go` | Create (move) | `checkoutCompleteMsg`, `checkoutBranchCmd` |
| `internal/tui/branch_form.go` | Modify | Modes, Changes field, WIP gating, intents, openers |
| `internal/tui/branch_form_view.go` | Modify | Mode-aware body rows |
| `internal/tui/branch_form_test.go`, `branch_form_view_test.go` | Modify | Mode tests, goldens |
| `internal/tui/model.go`, `helpers.go`, `help_overlay.go`, `help_v2.go`, `keys.go`, `overlay_config.go` | Modify | Wiring, labels, dead settings |
| `internal/tui/overlay_fork.go`, `overlay_checkout.go` | Delete | The wizards |
| `internal/tui/overlay_fork_test.go`, `overlay_checkout_test.go`, `testdata/TestGolden_Overlay_Fork/` | Delete after porting | Wizard tests |
| `docs/TUI.md`, `docs/VISUAL_TESTING.md`, `CHANGELOG.md` | Modify | Docs |

---

### Task 1: Workspace and baseline

Same as phase 3a Task 1, with workspace name `2026-10-11-tui-redesign-phase-3b`. Create `goenv.sh` (absolute `$TMPDIR` paths), `gate.sh`, and the ledger. Run the gate: green.

---

### Task 2: Fork and checkout commands into their own files

**Files:** Create `fork_cmd.go`, `checkout_cmd.go`; modify `overlay_fork.go`, `overlay_checkout.go`, `integration_test.go`.

**Interfaces:**
- Produces:

```go
// forkRequest is a copy of a worktree onto a new branch cut from its HEAD.
type forkRequest struct {
	Source WorktreeItem
	Name   string // the new worktree's alias
	Branch string // the new branch
	HasWIP bool
	WIP    WIPStrategy
}

func forkWorktreeCmd(mgr *worktree.Manager, stateMgr *state.Manager, cfg *config.Config, projectRoot string, req forkRequest) tea.Cmd
```

- [ ] **Step 1: Move**

Move unchanged from `overlay_fork.go` to `fork_cmd.go`: `WIPStrategy` and its constants, `forkCompleteMsg`, `forkWorktreeCmd`, `forkCaptureWIP`, `forkCreateBranchAndWorktree`, `forkApplyWIP`, and any helper only they use (`errWorktreeNotFound` stays where it is if other files use it).

Move unchanged from `overlay_checkout.go` to `checkout_cmd.go`: `checkoutCompleteMsg`, `checkoutBranchCmd`.

- [ ] **Step 2: Change the fork signature**

In `fork_cmd.go`, add `forkRequest` (above) and change `forkWorktreeCmd`, `forkCaptureWIP`, and `forkApplyWIP` to take `req forkRequest` instead of `forkState *ForkState`:

- `source := req.Source`, `name := req.Name`, `newBranchName := req.Branch`.
- `forkCaptureWIP(req)`: `if !req.HasWIP || (req.WIP != WIPMove && req.WIP != WIPCopy) { return nil, nil }`, then the patch from `req.Source.Path`.
- `forkApplyWIP(req, newPath, wipPatch)`: `if req.WIP != WIPMove { return nil }`; clean `req.Source.Path`.

The old wizard's confirm handler (in `overlay_fork.go`) keeps today's behavior by building the request:

```go
forkRequest{
	Source: s.Source,
	Name:   s.Name,
	Branch: fmt.Sprintf("%s-%s", s.Source.Branch, s.Name),
	HasWIP: s.HasWIP,
	WIP:    s.WIPStrategy,
}
```

- [ ] **Step 3: Port integration tests**

`grep -n 'forkWorktreeCmd(' internal/tui/*_test.go`. Each call that passes a `*ForkState` passes the equivalent `forkRequest`, with `Branch` set to `<source branch>-<name>` (the value the old code derived). No assertion changes.

- [ ] **Step 4: Gate and commit**

`$W/gate.sh` green; then `go test -tags=integration ./internal/tui/ -run 'Fork'` passes. No golden changes.

```bash
timeout --kill-after=5s 120s git -c commit.gpgsign=true commit -m "refactor(tui): give the fork command an explicit request

Moves the fork and checkout commands out of their wizard files, and
lets the caller name the new branch instead of deriving it."
```

---

### Task 3: Copy and Switch modes in the form state

**Files:** Modify `branch_form.go`, `branch_form_test.go`.

**Interfaces:**
- Consumes: `WIPStrategy`, `WIPMove`, `WIPCopy`, `WIPLeave` (Task 2).
- Produces: `formModeCopy`, `formModeSwitch`, `fieldChanges`, `intentCopy`, `intentSwitchBranch`; `formIntent` fields `WIP WIPStrategy`, `Stash bool`; `func newCopyForm(opts branchFormOptions, source WorktreeItem) *BranchFormState`; `func newSwitchForm(opts branchFormOptions, target WorktreeItem) *BranchFormState`; `func (s *BranchFormState) setWIP(path string, has bool, files []string, err error)`; fields `Source *WorktreeItem`, `HasWIP bool`, `WIPFiles []string`, `Changes WIPStrategy`; methods `wipChecked()`, `wipErrMsg()`.

- [ ] **Step 1: Failing tests**

Append to `internal/tui/branch_form_test.go`:

```go
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
	if s.Changes != WIPMove {
		t.Errorf("changes = %v, want move after right", s.Changes)
	}
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
```

Run: `mise exec -- go test ./internal/tui/ -run 'TestCopyForm|TestSwitchForm|TestBranchFormModes' -v`
Expected: FAIL to build, `undefined: newCopyForm`.

- [ ] **Step 2: Implement**

In `internal/tui/branch_form.go`:

1. Extend the enums:

```go
const (
	formModeNew formMode = iota
	formModeCopy
	formModeSwitch
)

const (
	fieldBranch formField = iota
	fieldBase
	fieldAlias
	fieldChanges
)

const (
	intentNone formIntentKind = iota
	intentSwitchTo
	intentCreateExisting
	intentCreateNew
	intentCopy
	intentSwitchBranch
)
```

2. `formIntent` gains `WIP WIPStrategy` and `Stash bool`.

3. `BranchFormState` gains:

```go
	Source   *WorktreeItem // copy: the worktree copied; switch: the worktree switched
	HasWIP   bool
	WIPFiles []string
	Changes  WIPStrategy

	wipDone bool
	wipErr  error
```

4. Add the constructors, WIP handling, and per-mode helpers:

```go
// newCopyForm copies source into a new worktree on a new branch cut from
// source's HEAD. Changes defaults to copy, which leaves the source as it is.
func newCopyForm(opts branchFormOptions, source WorktreeItem) *BranchFormState {
	s := newBranchForm(opts)
	s.Mode = formModeCopy
	s.Source = &source
	s.Changes = WIPCopy
	s.Branch.Placeholder = "new branch name"
	if source.Branch != "" {
		s.Branch.SetValue(source.Branch + "-copy")
		s.Branch.CursorEnd()
	}
	s.syncAlias()
	s.setFocus(fieldBranch)
	return s
}

// newSwitchForm switches target's checkout to another local branch.
func newSwitchForm(opts branchFormOptions, target WorktreeItem) *BranchFormState {
	s := newBranchForm(opts)
	s.Mode = formModeSwitch
	s.Source = &target
	s.Branch.Placeholder = "type to search local branches"
	s.setFocus(fieldBranch)
	return s
}

// setWIP records the uncommitted-changes check for the form's worktree. A
// result for any other path comes from an earlier form and is ignored.
func (s *BranchFormState) setWIP(path string, has bool, files []string, err error) {
	if s.Source == nil || s.Source.Path != path {
		return
	}
	s.wipDone, s.HasWIP, s.WIPFiles, s.wipErr = true, has, files, err
}

func (s *BranchFormState) wipChecked() bool { return s.wipDone }

func (s *BranchFormState) wipErrMsg() string {
	if s.wipErr == nil {
		return ""
	}
	return s.wipErr.Error()
}

var changesOrder = []WIPStrategy{WIPCopy, WIPMove, WIPLeave}

func changesLabel(w WIPStrategy) string {
	switch w {
	case WIPMove:
		return "move"
	case WIPLeave:
		return "leave"
	}
	return "copy"
}

func (s *BranchFormState) cycleChanges(delta int) {
	i := 0
	for j, w := range changesOrder {
		if w == s.Changes {
			i = j
		}
	}
	s.Changes = changesOrder[(i+delta+len(changesOrder))%len(changesOrder)]
}

func (s *BranchFormState) validateWIP() (formField, string, bool) {
	if s.wipErr != nil {
		return s.Focus, "could not check for uncommitted changes: " + s.wipErr.Error(), false
	}
	if !s.wipDone {
		return s.Focus, "checking for uncommitted changes…", false
	}
	return fieldBranch, "", true
}

func (s *BranchFormState) validateAlias() (formField, string, bool) {
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

func (s *BranchFormState) validateCopy() (formField, string, bool) {
	b := s.branchValue()
	if b == "" {
		return fieldBranch, "type a name for the new branch", false
	}
	if s.known(b) {
		return fieldBranch, fmt.Sprintf("branch %q already exists", b), false
	}
	if f, msg, ok := s.validateAlias(); !ok {
		return f, msg, false
	}
	return s.validateWIP()
}

func (s *BranchFormState) validateSwitch() (formField, string, bool) {
	b := s.branchValue()
	switch {
	case b == "":
		return fieldBranch, "type a branch name", false
	case b == s.Source.Branch:
		return fieldBranch, fmt.Sprintf("%s is already on %s", s.Source.ShortName, b), false
	case s.inUse[b] != nil:
		return fieldBranch, fmt.Sprintf("%s is checked out in %s", b, s.inUse[b].ShortName), false
	case !s.local[b]:
		return fieldBranch, fmt.Sprintf("no local branch %q", b), false
	}
	return s.validateWIP()
}

func (s *BranchFormState) switchCandidates(q string) []formSuggestion {
	ql := strings.ToLower(q)
	var out []formSuggestion
	for _, b := range s.order {
		if !s.local[b] || s.inUse[b] != nil {
			continue
		}
		if ql == "" || strings.Contains(strings.ToLower(b), ql) {
			out = append(out, formSuggestion{Branch: b, Note: "local"})
		}
	}
	return out
}

func (s *BranchFormState) busyVerb() string {
	switch s.Mode {
	case formModeCopy:
		return "copying…"
	case formModeSwitch:
		return "switching…"
	}
	return "creating…"
}
```

5. Make the existing methods mode-aware:

- `title()`:

```go
func (s *BranchFormState) title() string {
	switch s.Mode {
	case formModeCopy:
		return "Copy " + s.Source.ShortName
	case formModeSwitch:
		return "Switch branch of " + s.Source.ShortName
	}
	return "New worktree"
}
```

- `suggestions()`: at the top,

```go
	switch s.Mode {
	case formModeCopy:
		return nil
	case formModeSwitch:
		if s.Focus != fieldBranch {
			return nil
		}
		return s.switchCandidates(s.branchValue())
	}
```

- `fieldOrder()`: at the top,

```go
	switch s.Mode {
	case formModeCopy:
		if s.HasWIP {
			return []formField{fieldBranch, fieldAlias, fieldChanges}
		}
		return []formField{fieldBranch, fieldAlias}
	case formModeSwitch:
		return []formField{fieldBranch}
	}
```

- `setFocus(f)`: after blurring the inputs, `if f == fieldChanges { s.Focus, s.Pick = f, 0; return nil }`.
- `validate()`: at the top, `switch s.Mode { case formModeCopy: return s.validateCopy(); case formModeSwitch: return s.validateSwitch() }`. Replace New mode's alias block with `if f, msg, ok := s.validateAlias(); !ok { return f, msg, false }`.
- `intent()`: at the top,

```go
	switch s.Mode {
	case formModeCopy:
		in := formIntent{Kind: intentCopy, Alias: s.aliasValue(), Branch: s.branchValue(), Target: s.Source, WIP: WIPLeave}
		if s.HasWIP {
			in.WIP = s.Changes
		}
		return in
	case formModeSwitch:
		return formIntent{Kind: intentSwitchBranch, Branch: s.branchValue(), Target: s.Source, Stash: s.HasWIP}
	}
```

- `status()`: `if s.Busy { return s.busyVerb(), formStatusInfo }`. Guard the in-use message with `s.Mode == formModeNew &&`. Before the final `"✓ ready"`:

```go
	if s.HasWIP {
		switch s.Mode {
		case formModeCopy:
			return fmt.Sprintf("✓ ready · %d changed files will be %s", len(s.WIPFiles), map[WIPStrategy]string{WIPCopy: "copied", WIPMove: "moved", WIPLeave: "left behind"}[s.Changes]), formStatusReady
		case formModeSwitch:
			return fmt.Sprintf("✓ ready · %d changed files will be stashed", len(s.WIPFiles)), formStatusReady
		}
	}
```

- `keyLine()`:

```go
func (s *BranchFormState) keyLine() string {
	if s.Busy {
		return s.busyVerb()
	}
	if s.Log != nil {
		return "any key to edit · esc back"
	}
	act := "⏎ create"
	switch {
	case s.Mode == formModeCopy:
		act = "⏎ copy"
	case s.Mode == formModeSwitch:
		act = "⏎ switch"
	case s.inUse[s.branchValue()] != nil:
		act = "⏎ switch"
	}
	parts := []string{act}
	if len(s.fieldOrder()) > 1 {
		parts = append(parts, "tab field")
	}
	if s.Focus == fieldChanges {
		parts = append(parts, "←/→ changes")
	} else if s.Mode != formModeCopy {
		parts = append(parts, "ctrl+j/k pick")
	}
	return strings.Join(append(parts, "esc back"), " · ")
}
```

- `handleKey`: before `return formContinue, s.typeInto(msg)`, add

```go
	if s.Focus == fieldChanges {
		switch msg.String() {
		case "left":
			s.cycleChanges(-1)
		case "right", "space":
			s.cycleChanges(1)
		}
		return formContinue, nil
	}
```

- `typeInto`: at the top, `if s.Focus == fieldChanges { return nil }`.

- [ ] **Step 3: Run tests**

Run: `mise exec -- go test ./internal/tui/ -run 'TestCopyForm|TestSwitchForm|TestBranchFormModes|TestBranchForm' -v`
Expected: PASS, including every phase 3a `TestBranchForm_*` test.

- [ ] **Step 4: Gate and commit**

The gate's lint may flag `wipErrMsg` as unused until Task 4 renders it. If so, use it in `TestCopyForm_WIPCheckErrorBlocks` (`if s.wipErrMsg() != errTestBoom.Error()`), and ledger it.

```bash
timeout --kill-after=5s 120s git -c commit.gpgsign=true commit -m "feat(tui): add copy and switch modes to the branch form state"
```

---

### Task 4: Render Copy and Switch

**Files:** Modify `branch_form_view.go`, `branch_form_view_test.go`; add goldens.

**Body rows (9, unchanged count):** rows 0 to 4 are the Branch line and 4 suggestion rows. Rows 5 to 8 by mode:

| Row | New | Copy | Switch |
|---|---|---|---|
| 5 | Base | `From   <source alias> · <source branch>` (dim) | `In     <target alias>` (dim) |
| 6 | Alias | Alias | `Now    <current branch>` (dim) |
| 7 | preview | preview | blank |
| 8 | blank | Changes | Changes |

The Copy suggestion area shows one dim line, `new branch from <source alias>'s HEAD`, then blanks.

Changes row text:
- check pending: `checking…` (dim)
- check failed: `check failed` (error style)
- clean: `none` (dim)
- Copy, dirty: `● copy  ○ move  ○ leave · N files`, with `●` on `s.Changes`
- Switch, dirty: `stash N files before switching` (warning style)

Focus line: Branch 0; New Base 5; Alias 6; Changes 8.

- [ ] **Step 1: Failing tests**

Add to `formCases()` in `branch_form_view_test.go`:

```go
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
```

Append:

```go
func TestCopyForm_RendersChangesChoice(t *testing.T) {
	s := newCopyForm(testFormOptions(), testSource())
	s.setWIP("/r/auth", true, []string{"a.go", "b.go"}, nil)
	s.setWidth(120, 40)
	out := ansi.Strip(renderBranchForm(s, 120, 40, "⠋"))
	for _, want := range []string{"Copy auth", "● copy", "○ move", "○ leave", "2 files", "From"} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q:\n%s", want, out)
		}
	}
}

func TestSwitchForm_RendersStashNote(t *testing.T) {
	s := newSwitchForm(testFormOptions(), testSource())
	s.setWIP("/r/auth", true, []string{"a.go"}, nil)
	s.setWidth(120, 40)
	out := ansi.Strip(renderBranchForm(s, 120, 40, "⠋"))
	for _, want := range []string{"Switch branch of auth", "stash 1 files before switching", "feature/auth"} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q:\n%s", want, out)
		}
	}
}
```

Run: `mise exec -- go test ./internal/tui/ -run 'TestCopyForm_Renders|TestSwitchForm_Renders|TestBranchForm_FrameSizeIsFixed' -v`
Expected: the two render tests FAIL (rows not drawn). `FrameSizeIsFixed` may already pass; it guards the new cases.

- [ ] **Step 2: Implement**

In `bodyLines`, replace everything after the suggestion lines with a `switch s.Mode` that builds rows 5 to 8 per the table, and a `changesLine()` helper per the Changes rules. Keep New mode's output byte-identical: its rows 5 to 8 are exactly today's Base, Alias, preview, blank. Return the focus line per the table.

For Copy, `suggestionLines` returns the dim `new branch from …` line followed by blank rows (indent only), always `formSuggestRows` lines.

- [ ] **Step 3: Tests and goldens**

Run the two render tests: PASS. Then `-update` `TestGolden_BranchForm`, and check that `git status --short internal/tui/testdata` shows **only new** Copy/Switch files. Every existing New-mode golden must be unchanged. Read the new files with `cat -v`.

- [ ] **Step 4: Gate and commit**

```bash
timeout --kill-after=5s 120s git -c commit.gpgsign=true commit -m "feat(tui): render the copy and switch modes of the branch form"
```

---

### Task 5: Wire `f`, `b`, the exists prompt; delete the wizards

**Files:** Modify `branch_form.go`, `model.go`, `helpers.go`, `help_overlay.go`, `help_v2.go`, `keys.go`, tests; delete `overlay_fork.go`, `overlay_checkout.go`, their tests, `testdata/TestGolden_Overlay_Fork/`.

- [ ] **Step 1: Failing model tests**

Append to `branch_form_test.go`:

```go
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
```

In `navigation_test.go`, `TestExistsPromptFork_EscReturnsToPRBrowser` changes its first assertion to `m.activeView() != ViewBranchForm || m.branchForm == nil || m.branchForm.Mode != formModeCopy`.

Run them: FAIL (build or behavior).

- [ ] **Step 2: Openers and submit**

In `branch_form.go`, extract the option gathering from `openBranchForm` into `func (m Model) branchFormOptionsFor() branchFormOptions`, and make `openBranchForm` use it. Add:

```go
func (m Model) openCopyForm(source WorktreeItem) (tea.Model, tea.Cmd) {
	s := newCopyForm(m.branchFormOptionsFor(), source)
	s.setWidth(m.width, m.height)
	m.branchForm = s
	m.pushView(ViewBranchForm)
	return m, wipCheckCmd(source.Path)
}

func (m Model) openSwitchForm(target WorktreeItem) (tea.Model, tea.Cmd) {
	s := newSwitchForm(m.branchFormOptionsFor(), target)
	s.setWidth(m.width, m.height)
	m.branchForm = s
	m.pushView(ViewBranchForm)
	return m, wipCheckCmd(target.Path)
}
```

In `submitBranchForm`, add:

```go
	case intentCopy:
		if m.worktreeMgr == nil {
			s.Status = "cannot copy a worktree: no repository loaded"
			return m, nil
		}
		s.Busy, s.Status = true, ""
		s.Log = NewActivityLog(formLogWidth(m.width), 6)
		s.Log.AddLine(fmt.Sprintf("Copying %s to %s on branch %s...", in.Target.ShortName, in.Alias, in.Branch))
		return m, tea.Batch(m.spinner.Tick, forkWorktreeCmd(m.worktreeMgr, m.stateMgr, m.cfg, m.projectRoot,
			forkRequest{Source: *in.Target, Name: in.Alias, Branch: in.Branch, HasWIP: s.HasWIP, WIP: in.WIP}))
	case intentSwitchBranch:
		s.Busy, s.Status = true, ""
		s.Log = NewActivityLog(formLogWidth(m.width), 6)
		s.Log.AddLine(fmt.Sprintf("Switching %s to branch %s...", in.Target.ShortName, in.Branch))
		return m, tea.Batch(m.spinner.Tick, checkoutBranchCmd(in.Target.Path, in.Branch, in.Stash))
```

- [ ] **Step 3: Model wiring**

In `model.go`:

- `handleDashboardForkKey`: `item, ok := m.selectedItem(); if !ok { return m, nil }; return m.openCopyForm(item)`.
- `handleDashboardCheckoutKey`: keep the main/protected guard; open with `m.openSwitchForm(item)`.
- `handleWIPCheck`: body becomes `if m.branchForm != nil { m.branchForm.setWIP(msg.path, msg.hasWIP, msg.files, msg.err) }; return m, nil`.
- `handleForkComplete`:

```go
func (m Model) handleForkComplete(msg forkCompleteMsg) (tea.Model, tea.Cmd) {
	if msg.err != nil && msg.name == "" {
		if s := m.branchForm; s != nil {
			if s.Log != nil {
				s.Log.SetDone(msg.err)
			}
			s.setCreatingDone(msg.err.Error())
		}
		return m, nil
	}
	m.popToRoot()
	m.branchForm = nil
	m.pendingSelect = msg.name
	if msg.err != nil {
		m.toast.Show(NewToast(fmt.Sprintf("Copied to %q (warning: %s)", msg.name, msg.err), ToastWarning))
	} else {
		m.toast.Show(NewToast(fmt.Sprintf("Copied to %q", msg.name), ToastSuccess))
	}
	return m, tea.Batch(m.spinner.Tick, m.fetchWorktrees)
}
```

- `handleCheckoutComplete`: the same shape: an error goes into the form; success pops to root, clears the form, and toasts `Switched to branch %q`.
- Remove `forkState`, `checkoutState` fields; their clauses in `isOperationAnimating`; `ViewFork`/`ViewCheckout` cases in `handleKey`, `renderOverlay`, `viewHasTextInput`; `handleCheckoutBranches` and its `case checkoutBranchesMsg:`.
- Delete `ViewFork` and `ViewCheckout` from the `ActiveView` const block.
- In `helpers.go`, `handleExistsPromptKey`'s `n` branch: after `dismiss()` and the nil check, `return m.openCopyForm(*item)`.

- [ ] **Step 4: Delete the wizards, help, labels**

```bash
git rm -q internal/tui/overlay_fork.go internal/tui/overlay_checkout.go
```

- `help_overlay.go`: delete `helpSectionsFork`, `helpSectionsCheckout` and their cases. Change dashboard help lines that say `fork` to `copy worktree` (`grep -n '"Fork\|fork' internal/tui/help_overlay.go`).
- `help_v2.go`: delete the `ViewFork` and `ViewCheckout` hint cases.
- `keys.go`: the `Fork` binding's help becomes `key.WithHelp("f", "copy")`.

- [ ] **Step 5: Migrate tests**

Build tests: `mise exec -- go test -count=1 -run NONE ./internal/tui/`. Then:

**Port** (keep the behavior, drive the form):
- `TestForkCompleteMsg_Success/Error/PartialSuccess` → covered by `TestCopyFormComplete_*`; delete the originals.
- `TestForkOverlay_ConfirmWithNilManagers` → covered by `TestCopyFormSubmit_WithoutManagerReportsError`.
- `TestForkOverlay_WIPCheckErrorDoesNotSkip` → covered by `TestCopyForm_WIPCheckErrorBlocks`.
- `TestForkOverlay_RootWorktreeStartsOnName` → `f` on row 0 opens the copy form (add an assertion to `TestDashboardF_OpensCopyForm` for `Select(0)`).
- `TestCheckoutOverlay_MainWorktreeBlocked` → covered by `TestDashboardB_OpensSwitchFormButNotOnMain`.
- `TestCheckoutOverlay_WIPCheckIgnoresStalePath` → covered by `TestCopyForm_IgnoresWIPForOtherPath`; add a switch twin.
- `TestCheckoutOverlay_EnterBlockedBeforeWIPCheck` → add `TestSwitchForm_EnterBlockedUntilWIPChecked` (same shape as copy).
- `TestCheckoutOverlay_BranchAlreadyUsedShown` → covered by `TestSwitchForm_SuggestsFreeLocalBranchesOnly` and `_RejectsCurrentAndInUse`.
- `TestCheckoutCompleteMsg_Success/Error` → covered by `TestSwitchFormComplete`.
- `TestWIPStrategyConstants` → move into a new `fork_cmd_test.go` unchanged.

**Delete:** everything else in `overlay_fork_test.go` and `overlay_checkout_test.go` (step mechanics, wizard rendering, filter and list navigation, `BranchesMsg`), `withForkOverlay`, `TestGolden_Overlay_Fork` and its testdata. In `help_overlay_test.go`, remove `ViewFork` and `ViewCheckout` from `TestHelpSectionsForAllViews`. Fix any `help_v2_test.go` cases for those views.

Ledger each kept, ported, and deleted test by name.

- [ ] **Step 6: Run and gate**

`mise exec -- go test -count=1 ./internal/tui/` passes; `-tags=integration` passes; `$W/gate.sh` green. Golden changes: only `TestGolden_Overlay_Fork` deleted.

- [ ] **Step 7: Commit**

```bash
timeout --kill-after=5s 120s git -c commit.gpgsign=true commit -m "feat(tui): copy worktree and switch branch through the branch form

f and b, and the PR browser's exists prompt, open the branch form in
Copy and Switch modes. Both wait for the uncommitted-changes check
before enter does anything. Copy names the new branch, defaults to
copying changes, and never moves them unless chosen. Switch offers
only local branches no worktree has checked out.

Removes the fork and checkout wizards and their step tests."
```

---

### Task 6: Dead settings and docs

**Files:** `overlay_config.go`, its tests if any, `docs/TUI.md`, `docs/VISUAL_TESTING.md`, `CHANGELOG.md`.

- [ ] **Step 1: Config overlay**

Remove the `skip_branch_notice` and `default_branch_action` `ConfigField` entries from the field list in `overlay_config.go`, plus any save mapping that only they use. A test first: assert the Behavior tab's field keys do not include either. Watch it fail, then remove. Gate.

- [ ] **Step 2: `docs/TUI.md`**

- Key table: `f` → "Copy selected worktree (new branch from its HEAD)"; `b` → "Switch the selected worktree to another local branch".
- Replace `### Fork Worktree (\`f\`)` and `### Switch Branch (\`b\`)` with:

```markdown
### Copy Worktree (`f`)

Opens the New Worktree form in Copy mode for the selected worktree. Branch starts as `<branch>-copy` and must be a new branch; it is cut from the selected worktree's HEAD. Alias follows the branch until edited.

If the worktree has uncommitted changes, a Changes row appears: `copy` (default, keeps them in both), `move` (cleans the source), or `leave`. Use `←`/`→` or `space` on that row. `enter` waits until the changes check finishes.

### Switch Branch (`b`)

Opens the form in Switch mode for the selected worktree. Suggestions list local branches that no worktree has checked out. If the worktree has uncommitted changes, they are stashed before switching. Not available on the main or protected worktrees.
```

- Remove `skip_branch_notice` and `default_branch_action` from the config tab list.

- [ ] **Step 3: `docs/VISUAL_TESTING.md`**

Remove the `withCreateStep` and `withForkOverlay` builder mentions; add `TestGolden_BranchForm` as the example for overlay goldens.

- [ ] **Step 4: Changelog**

Under `## [Unreleased]` `### Changed`:

```markdown
- Copy worktree (`f`, formerly fork) and Switch branch (`b`) use the New Worktree form. Copy lets you name the new branch, copies uncommitted changes by default instead of moving them, and waits for the changes check before acting. Switch lists only local branches no worktree has checked out.
```

- [ ] **Step 5: Commit**

```bash
timeout --kill-after=5s 120s git -c commit.gpgsign=true commit -m "docs(tui): document copy and switch in the branch form"
```

---

### Task 7: Deferred minors from phases 2 and 3a

Each item is test-first where it has behavior. Phase 0-1's three deferred minors (wide-glyph row test, `docs/TUI.md` file map, glamour comment) are already done on main.

| # | Item | Source | Fix | Test |
|---|---|---|---|---|
| 1 | `n` opens the form with a non-blinking cursor: `newBranchForm`'s `setFocus(fieldBranch)` cmd is discarded | 3a review | `openBranchForm`, `openCopyForm`, `openSwitchForm` return the focus cmd, batched with any other cmd | `TestDashboardN_ReturnsFocusCmd`: cmd non-nil after `n` |
| 2 | `creating…` shows on both the status line and the key line while busy | 3a review | Busy key line becomes `esc hides · working` or similar; status keeps the verb. Pick wording that reads well in the golden. | `TestBranchForm_BusyVerbShownOnce`; `creating` goldens change (expected churn) |
| 3 | Busy at short heights: the log tail with the spinner scrolls out of view | 3a review | When `s.Log != nil`, `windowLines` keeps the last log line visible (focus on the log's last row) | `TestBranchForm_BusyLogTailVisibleAt80x20` |
| 4 | The issue entry no longer seeds a branch name | 3a review | `openBranchForm(branchFormPrefill{Branch: "issue-N"})`; alias derives from it | `TestIssueBrowser_SeedsBranchName` |
| 5 | `handleKey` calls `m.updateOverlay.Close()` without a nil check | Phase 2 review | `if m.updateOverlay != nil` | `TestUpdateView_KeyWithoutOverlayDoesNotPanic` (push `ViewUpdate` on `newTestModel`, press a key) |
| 6 | `helpOverlay.Active` and `updateOverlay.Active` are write-only; an async `popToRoot` leaves them `true` | Phase 2 review | Delete both fields and their writes; tests assert on `activeView()` instead | Compiler, plus updated tests |
| 7 | `docs/TUI.md:157` says every overlay opens "over the dimmed dashboard background" | Phase 2 review | "over the dimmed view beneath it" | Docs |
| 8 | `TestRenderCompactHighlightsMatchedKey` fails when `NO_COLOR` is inherited | Pre-existing | Reproduce with `NO_COLOR=1`. If the highlight is invisible without color, that is a real accessibility bug: give the highlight a non-color attribute (bold or reverse) so it stays visible. Otherwise pin the env in the test. | The existing test, run with and without `NO_COLOR=1` |

The other two 3a minors (`VISUAL_TESTING.md`, the dead config settings) are in Task 6.

- [ ] **Step 1:** Items 1 to 6, one commit each (`fix(tui): ...`), gate between.
- [ ] **Step 2:** Items 7 and 8, then gate, then commit.
- [ ] **Step 3:** Ledger each item as fixed, with its test name.

**Known and not in this phase** (pre-existing, recorded so they are not lost):
- Unconditional `popToRoot` on delete, rename, and bulk errors (`model.go`).
- Stale `prState` after create-from-PR success.
- `updateLayout` not re-run on push and pop.
- `ActivityLog.width` unused.
- Paste does not reach the rename and filter inputs.

---

### Task 8: Hands-on check, tmux and herdr

Scratch repo as in phase 3a (`$TMPDIR/grove-scratch`, worktrees `alpha` and a spare local branch `spare`). Use a private tmux socket (`tmux -L grove3b`), and a herdr tab you create. Close only what you create, including any herdr workspaces grove opens for new worktrees.

| # | Where | Keys | Expected |
|---|---|---|---|
| 1 | real repo | select a non-main worktree, `f`, `esc` | Copy form, `checking…` then Changes `none` or the choice; `esc` closes |
| 2 | real repo | select a non-main worktree, `b`, type an in-use branch | Switch form; in-use branches not suggested; typed in-use branch shows `is checked out in`; `esc` |
| 3 | scratch | dirty `alpha` (add a file), `f`, `enter` | Changes defaults to copy; new worktree has the file; `alpha` still has it |
| 4 | scratch | dirty `alpha`, `f`, `→` to move, `enter` | New worktree has the file; `alpha` is clean |
| 5 | scratch | dirty `alpha`, `b`, type `spare`, `enter` | Switches `alpha` to `spare`; the change is stashed (`git stash list`) |
| 6 | scratch | `f` with the form at 80x20 | Changes row and key line visible |

Ledger each result. A mismatch is a bug: failing test, fix, gate.

---

## Done when

- `make lint test` and `-tags=integration` pass.
- No production file mentions `ForkState`, `CheckoutState`, `ViewFork`, `ViewCheckout`, `handleForkKey`, or `renderCheckout`.
- New-mode goldens unchanged; only `TestGolden_Overlay_Fork` deleted and Copy/Switch goldens added.
- Task 7's eight items fixed and ledgered.
- Task 8's six paths match in tmux and herdr.
