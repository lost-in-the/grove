# Wizard UX Research

Research findings for three specific UX problems in the create-worktree wizard.

---

## Problem 4: Context Loss When Wizard Opens on Dashboard

### Current Behavior

When a user selects a PR or issue and presses Enter, the wizard opens via `ViewCreate` — which renders the create overlay **on top of the dashboard** (via `overlayOnDashboard`). The PR/issue view state (`prState`/`issueState`) is preserved in memory but not visible. On cancel (Esc), the wizard sets `m.activeView = ViewDashboard` and nils `m.createState`, landing the user on the dashboard — not back in the PR/issue view they came from.

**Key code paths:**
- `openCreateWizardForPR()` in `view_prs.go:176` sets `m.activeView = ViewCreate` without saving the previous view
- `openCreateWizardForIssue()` in `view_issues.go:159` does the same
- Every cancel handler in the wizard (`handleBranchChoiceKey`, `handleNameKey`, etc.) unconditionally does `m.activeView = ViewDashboard`

### Research: How Do TUIs Handle "Return to Previous View"?

#### Pattern A: View Stack (Push/Pop)

Lazygit uses a **context stack** where each panel/popup is a context that gets pushed onto the stack. Dismissing pops back to whatever was underneath. The stack naturally remembers where the user came from without explicit tracking.

From [lazygit's Codebase Guide](https://github.com/jesseduffield/lazygit/blob/master/docs/dev/Codebase_Guide.md): contexts are managed per-panel on a stack, with `PushContext` and `PopContext` operations. When a popup appears, the active context is preserved below it. Escape pops the top context, revealing the previous one.

[Building Bubbletea Programs](https://leg100.github.io/en/posts/building-bubbletea-programs/) describes this pattern: "maintain a stack of previously visited models: when the user presses a key your program pushes another model onto the stack... when the user presses a key to go 'back' the model is 'popped' off the stack."

**Applicability to Grove:** Heavy refactor. Would require converting `ActiveView` from a single enum to a stack of views. The existing overlay-on-dashboard rendering pattern would need rethinking.

**Pros:**
- Generalizes to any depth of navigation (popup from popup)
- No explicit "return to" tracking needed
- Well-proven pattern (lazygit, PUG)

**Cons:**
- Significant architecture change
- Overkill if only the wizard needs this behavior
- Grove's existing overlay model doesn't naturally stack

#### Pattern B: `previousView` Field (Lightweight)

Store the originating `ActiveView` in the wizard state or the model. On cancel, restore it instead of hardcoding `ViewDashboard`.

```go
// In CreateState
type CreateState struct {
    // ... existing fields
    ReturnView ActiveView // Where to go on cancel
}

// On cancel, in each handler:
m.activeView = m.createState.ReturnView
m.createState = nil
```

**When opening from PR view:**
```go
func (m Model) openCreateWizardForPR(pr *tracker.PullRequest) (tea.Model, tea.Cmd) {
    // ... existing setup
    m.createState.ReturnView = ViewPRs  // remember where we came from
    m.activeView = ViewCreate
    return m, m.createState.NameInput.Focus()
}
```

**Applicability to Grove:** Minimal change. Fits existing architecture perfectly.

**Pros:**
- Tiny diff (add one field, change ~10 cancel handlers)
- No architecture changes
- PR/issue state is already preserved in memory (`prState`/`issueState` are not nilled when the wizard opens)

**Cons:**
- Only one level deep (no stack)
- Must remember to set `ReturnView` in every place that opens the wizard

#### Pattern C: Overlay ON TOP of PR/Issue View

Instead of always rendering the wizard over the dashboard, render it over the view that spawned it.

```go
// In viewForActiveView, replace:
case ViewCreate:
    return m.overlayOnDashboard(...)

// With:
case ViewCreate:
    if m.createState != nil && m.createState.Source == "pr" && m.prState != nil {
        bg := m.renderPRPanel()
        return centerOverlay(bg, renderCreateV2(...), m.width, m.height)
    }
    if m.createState != nil && m.createState.Source == "issue" && m.issueState != nil {
        bg := m.renderIssuePanel()
        return centerOverlay(bg, renderCreateV2(...), m.width, m.height)
    }
    return m.overlayOnDashboard(...)
```

**Applicability to Grove:** The `centerOverlay` function already exists and dims the background. It works with any rendered string as the background.

**Pros:**
- User sees the PR/issue list behind the wizard, maintaining mental context
- Combined with Pattern B, cancel returns to the exact view
- Visual continuity reinforces the relationship between the wizard and its source

**Cons:**
- PR panel rendering is more complex than dashboard (split panes, viewports)
- The dimmed PR panel behind the overlay might look cluttered in small terminals
- Slight rendering cost from generating two complex views

### Recommendation

**Use Pattern B + C together.** Add `ReturnView` to `CreateState`, and render the wizard overlay on top of the originating view when `Source` is "pr" or "issue". This is the lowest-effort change that solves both the navigation and visual context problems.

The `Source` field already exists in `CreateState` with values `""`, `"pr"`, and `"issue"` — it's used today in `handleNameKey` to decide back-navigation. Extending it to control overlay background and return destination is a natural evolution.

---

## Problem 5: Branch List Doesn't Show In-Use Branches

### Current Behavior

The branch selector (`renderCreateBranchSelectV2`) renders all branches identically. It has no awareness of which branches already have worktrees. The user discovers conflicts only after selecting a branch and advancing through additional steps.

**The data is already available:** `m.existingWorktreeItems()` returns all worktree items with their branch names. `m.worktreeBranchMap()` builds a `map[string]string` (branch -> worktree short name). These are used in the PR and issue views for duplicate detection.

### Research: TUI List Item Annotations

#### How gh-dash Annotates List Items

[gh-dash](https://github.com/dlvhdr/gh-dash) renders PR list items with inline status indicators:
- CI check status icons (pass/fail/pending)
- Review approval status
- Draft/ready state
- Unread notification indicators

These are rendered inline using Lipgloss styling — different foreground colors, Unicode symbols, and dim text for secondary information. Each `ItemDelegate` builds up a styled string with multiple segments.

#### How lazygit Shows Branch State

[lazygit](https://github.com/jesseduffield/lazygit) shows branch tracking status inline:
- Ahead/behind counts (e.g., `[+2/-1]`)
- Tracking remote indicator
- Current branch highlighting
- Worktree status (if branch is checked out in another worktree)

#### Bubbles List Custom Delegate Pattern

The Bubbles `list.Model` uses `ItemDelegate` for custom rendering. For a custom list (like Grove's hand-rolled branch selector), the pattern is simpler — you control the render loop directly.

#### Approach A: Inline Badge (Recommended)

Append a styled badge next to in-use branches in the render loop:

```go
// In renderCreateBranchSelectV2, inside the loop:
for i := start; i < end; i++ {
    branch := filtered[i]
    cursor := "  "
    if i == s.BranchCursor {
        cursor = Styles.ListCursor.Render("> ")
    }

    // Check if this branch is in use
    if wtName, inUse := s.WorktreeBranches[branch]; inUse {
        badge := Styles.DetailDim.Render(" [" + wtName + "]")
        b.WriteString(d.indent + cursor + branch + badge + "\n")
    } else {
        b.WriteString(d.indent + cursor + branch + "\n")
    }
}
```

**Requirements:**
- Pass `WorktreeBranches map[string]string` into `CreateState` when initializing the wizard
- The data is already computed by `m.worktreeBranchMap()` in the PR/issue views

**Pros:**
- User sees at a glance which branches are taken
- No structural change to the list
- Matches how lazygit shows branch metadata inline

**Cons:**
- Doesn't prevent selection — user can still pick an in-use branch
- Badge text might overflow on narrow terminals

#### Approach B: Grayed Out + Non-Selectable

Render in-use branches with dim/faint styling and skip them during cursor navigation:

```go
// Navigation: skip in-use branches
case key.Matches(msg, m.keys.Down):
    for s.BranchCursor < totalItems-1 {
        s.BranchCursor++
        if _, inUse := s.WorktreeBranches[filtered[s.BranchCursor]]; !inUse {
            break
        }
    }

// Rendering: dim in-use branches
if _, inUse := s.WorktreeBranches[branch]; inUse {
    line := Styles.TextMuted.Render(cursor + branch + " (in use)")
    b.WriteString(d.indent + line + "\n")
}
```

Note on fzf's `--disabled` flag: fzf's `--disabled` flag disables the **search/filter** functionality, not item selection. It makes fzf a simple selector rather than a fuzzy finder. There is no built-in fzf mechanism to disable selection of specific items. This is a custom behavior that must be implemented in any TUI.

**Pros:**
- Prevents errors entirely — can't select what you can't reach
- Very clear visual signal

**Cons:**
- Cursor-skipping feels weird (user presses down, cursor jumps two rows)
- If most branches are in use, navigation becomes confusing
- May hide information the user wants to see (which worktree is using a branch)

#### Approach C: Sectioned List (Available / In Use)

Split the branch list into two sections:

```
  Available branches:
  > feature/new-auth
    fix/login-bug
    refactor/api-client

  In use:
    main [grove-main]
    develop [grove-develop]
    feature/dashboard [grove-dashboard]
```

**Pros:**
- Cleanest visual separation
- User can still see in-use branches for reference
- Natural section boundary prevents accidental selection

**Cons:**
- More complex rendering and cursor math
- Filtering becomes tricky (filter across both sections?)
- Significant change to the branch list structure

### Recommendation

**Start with Approach A (inline badge), with Approach B as a future enhancement.** The inline badge is the lowest-effort change that gives the user critical information. It requires adding `WorktreeBranches` to `CreateState` and a small change to the render loop.

If user testing shows people still select in-use branches despite the badge, add Approach B's selection prevention. The cursor-skip behavior can be made less jarring by showing a brief toast or inline message when a skipped branch is encountered.

**Data flow for implementation:**
1. In `handleDashboardNewKey()`, add: `WorktreeBranches: m.worktreeBranchMap()`
2. In `prefillCreateStateForPR/Issue()`, accept and pass through the branch map
3. In `renderCreateBranchSelectV2()`, check `s.WorktreeBranches[branch]` during rendering
4. Optionally in `handleBranchSelectKey()`, block Enter on in-use branches with an inline error

---

## Problem 6: Navigation Between Wizard Steps

### Current Behavior

The wizard uses **Backspace** as the "go back" key (bound as `keys.Back`). This creates a conflict on text input steps:

1. **BranchCreate step** (`handleBranchCreateKey`): Backspace only goes back when `BranchNameInput.Value() == ""`. If you've typed anything, Backspace deletes characters. You must clear the entire input first.
2. **Name step** (`handleNameKey`): Same pattern — Backspace goes back only when `NameInput.Value() == ""`.
3. **Non-input steps** (BranchChoice, BranchSelect with filter off, BranchAction, Confirm): Backspace works immediately as "go back" because there's no text input consuming it.

The footer hints show `[backspace] back` on every step, which is misleading on text input steps.

### Research: How Do Terminal Apps Handle This?

#### Huh Form Navigation

[Huh](https://github.com/charmbracelet/huh) uses **Shift+Tab** to go to the previous field and **Tab/Enter** to go to the next field:

From huh's [keymap.go](https://github.com/charmbracelet/huh/blob/main/keymap.go):
- `NextField`: bound to `tab`, `enter`
- `PrevField`: bound to `shift+tab`
- `NextGroup`: bound to `tab`, `enter` (when on the last field of a group)
- `PrevGroup`: bound to `shift+tab` (when on the first field of a group)

This avoids the Backspace conflict entirely — Shift+Tab **never** conflicts with text editing because no text input component uses Shift+Tab.

**Known issue:** Huh has a [bug where Shift+Tab triggers validation on blur](https://github.com/charmbracelet/huh/issues/655), which can softlock the form if the current field is invalid. This was reported in May 2025. [Issue #540](https://github.com/charmbracelet/huh/issues/540) tracks the broader problem of backtracking when a group has validation errors.

#### Bubbletea v2 Enhanced Keyboard

[Bubbletea v2](https://github.com/charmbracelet/bubbletea/releases/tag/v2.0.0) supports progressive keyboard enhancements:
- **Shift+Tab**: Reliably detected in virtually all terminals. Traditional terminals send `\e[Z` (CSI Z) for Shift+Tab, and modern terminals with Kitty protocol send an even more explicit sequence.
- **Shift+Backspace**: Unreliable in many terminals. Traditional terminals often send the same code as Backspace. Kitty protocol terminals can distinguish it, but coverage is incomplete.
- **Ctrl+Backspace**: [Historically problematic](https://www.vinc17.net/unix/ctrl-backspace.en.html) — traditional terminals send the same code as `Ctrl+H` (ASCII 8) or DEL (127). Only Kitty-protocol terminals reliably distinguish Ctrl+Backspace from plain Backspace.

The `shift+tab` key is already defined in Grove's `KeyMap` as `ShiftTab` with the key string `"shift+tab"`.

#### How Other Terminal Wizards Handle Back-Navigation

| Tool | Forward | Back | Text input conflict? |
|------|---------|------|---------------------|
| **Huh forms** | Tab / Enter | Shift+Tab | No (Shift+Tab never conflicts) |
| **npm init** | Enter | No back navigation | N/A (can't go back) |
| **gh repo create** | Enter | No back navigation | N/A (linear flow only) |
| **gum** | Enter | Ctrl+C (abort) | N/A (single-step prompts) |
| **Claude guided prompts** | Tab | Shift+Tab | No |
| **Inquirer.js** | Enter | No back navigation | N/A |
| **lazygit popups** | Enter | Esc (dismiss) | N/A (popups are atomic) |

**Key finding:** Most terminal wizards **do not support back-navigation**. They are either single-step (gum, Inquirer) or linear-only (npm init, gh repo create). Huh and Claude's guided prompts are notable exceptions, both using Tab/Shift+Tab.

### Approach Analysis

#### Approach A: Shift+Tab for Back (Recommended)

Replace Backspace with Shift+Tab as the primary "go back" key on all wizard steps.

```go
// In handleBranchCreateKey:
case key.Matches(msg, m.keys.ShiftTab):
    // Always go back, regardless of input content
    s.Step = CreateStepBranchChoice
    return m, nil

// Backspace is now ONLY for text editing (handled by textinput.Update)
```

**On non-input steps** (BranchChoice, Confirm, BranchAction), Shift+Tab and Backspace can both trigger "go back" since there's no conflict.

**On input steps** (BranchCreate, Name), only Shift+Tab triggers "go back". Backspace always deletes text.

**Footer hints would change:**
- Non-input steps: `[shift+tab/backspace] back  [enter] next  [esc] cancel`
- Input steps: `[shift+tab] back  [enter] next  [esc] cancel`

**Pros:**
- Matches Huh forms and Claude guided prompts
- Never conflicts with text editing
- Shift+Tab is already in the KeyMap
- Simple implementation (check Shift+Tab before routing to textinput)
- Terminal compatibility is excellent (Shift+Tab works everywhere)

**Cons:**
- Less discoverable than Backspace (most users know Backspace = back)
- Adds a modifier key requirement
- Users on very old terminals might not have Shift+Tab support (rare)

#### Approach B: Keep Backspace, Add Shift+Tab as Alternative

Support both keys — Backspace goes back only when input is empty (current behavior), Shift+Tab always goes back.

```go
case key.Matches(msg, m.keys.ShiftTab):
    // Always go back
    s.Step = CreateStepBranchChoice
    return m, nil

case key.Matches(msg, m.keys.Back):
    if s.BranchNameInput.Value() == "" {
        s.Step = CreateStepBranchChoice
        return m, nil
    }
    // Let textinput handle backspace
    var cmd tea.Cmd
    s.BranchNameInput, cmd = s.BranchNameInput.Update(msg)
    return m, cmd
```

**Pros:**
- Backward compatible — existing Backspace behavior preserved
- Power users get Shift+Tab for unambiguous back
- Footer can show both: `[shift+tab] back  [enter] next  [esc] cancel`

**Cons:**
- Backspace still has the confusing dual-purpose behavior
- Two ways to do the same thing can be confusing
- Footer text becomes longer

#### Approach C: Blur Input on Back Key

When the user presses Backspace on an input step, first check if the input is focused. If it is, blur it (stop accepting text). On the next Backspace, navigate back. On any other key, re-focus the input.

```go
case key.Matches(msg, m.keys.Back):
    if s.BranchNameInput.Focused() && s.BranchNameInput.Value() != "" {
        s.BranchNameInput.Blur()  // Visual signal: cursor disappears
        return m, nil
    }
    if !s.BranchNameInput.Focused() || s.BranchNameInput.Value() == "" {
        s.Step = CreateStepBranchChoice
        return m, nil
    }
```

**Pros:**
- Uses only Backspace (no new keys to learn)
- Two-press pattern gives confirmation before navigating away
- Visual feedback (cursor disappears when blurred)

**Cons:**
- Novel UX pattern — no precedent in other tools
- Confusing: "I pressed Backspace and my cursor disappeared but text is still there"
- Re-focusing logic adds complexity
- Not discoverable

#### Approach D: Esc as Back, Double-Esc to Cancel

Use Esc to go back one step, and double-Esc (or Ctrl+C) to cancel the entire wizard.

**Pros:**
- Esc = "back out" is a common TUI convention
- Never conflicts with text editing

**Cons:**
- Breaks the current Esc = cancel convention used throughout Grove
- Double-Esc timing is hard to get right
- Inconsistent with other overlays (delete, fork, sync all use Esc to dismiss)

### Value Retention When Navigating Back

The user mentioned that navigating away and back should **retain field values**. This is partially implemented already:

- `CreateState` persists across step changes (it's a pointer stored on the model)
- `BranchNameInput`, `NameInput`, and `BranchFilterInput` are stored in `CreateState`
- Going forward preserves their values
- **Going back currently does NOT fully preserve values** — some back-navigation handlers reset inputs:
  - `handleBranchSelectKey` back: resets filter (`SetValue("")`) and cursor
  - `handleNameKey` back: only goes back when input is empty (so value is always "")

**Fix needed:** When navigating back, don't reset input values. When navigating forward again, the input should still show what the user previously typed. This means:
- Remove the `SetValue("")` call when going back from branch select
- Remove the `Value() == ""` guard when using Shift+Tab (since Shift+Tab is always unambiguous)

### Recommendation

**Use Approach B: Keep Backspace, add Shift+Tab as the primary back key.** This provides the unambiguous navigation the user wants while maintaining backward compatibility.

Implementation:
1. On **all wizard steps**, check `ShiftTab` before `Back` in the key handler
2. ShiftTab always navigates back regardless of input state
3. Backspace retains current behavior (back when empty, delete when not)
4. Update footer hints to show `[shift+tab] back` on input steps and `[shift+tab/backspace] back` on non-input steps
5. Preserve input values when navigating backward — don't reset on back

**Key mapping summary:**

| Step | Shift+Tab | Backspace | Enter | Esc |
|------|-----------|-----------|-------|-----|
| BranchChoice | Go back (dismiss) | Go back (dismiss) | Select option | Cancel wizard |
| BranchSelect | Go back to choice | Go back to choice | Select branch | Cancel wizard |
| BranchCreate | Go back to choice | Delete text / back if empty | Next step | Cancel wizard |
| BranchAction | Go back to select | Go back to select | Next step | Cancel wizard |
| Name | Go back to branch step | Delete text / back if empty | Next step | Cancel wizard |
| Confirm | Go back to name | Go back to name | Create worktree | Cancel wizard |

---

## Implementation Priority

| Problem | Effort | Impact | Recommended Order |
|---------|--------|--------|-------------------|
| **P4: ReturnView field** | Small (add field, change ~10 cancel handlers) | High (eliminates navigation dead end) | First |
| **P5: Branch badges** | Small (add map to state, modify render loop) | Medium (prevents user error) | Second |
| **P6: Shift+Tab navigation** | Medium (add ShiftTab checks to 6 handlers, update footers) | High (eliminates the Backspace frustration) | Third |
| **P4 bonus: Overlay on source view** | Medium (conditional background in viewForActiveView) | Medium (visual polish) | After P4 |

---

## Sources

### Lazygit
- [lazygit GitHub repository](https://github.com/jesseduffield/lazygit)
- [Codebase Guide](https://github.com/jesseduffield/lazygit/blob/master/docs/dev/Codebase_Guide.md) -- context stack pattern
- [Stagger popup panels PR #3694](https://github.com/jesseduffield/lazygit/pull/3694) -- popup offset/stacking

### Huh Forms
- [huh GitHub repository](https://github.com/charmbracelet/huh)
- [huh keymap.go](https://github.com/charmbracelet/huh/blob/main/keymap.go) -- Tab/Shift+Tab keybindings
- [huh/v2 Go documentation](https://pkg.go.dev/github.com/charmbracelet/huh/v2) -- NextGroup/PrevGroup API
- [Issue #540: Allow backtracking to previous group](https://github.com/charmbracelet/huh/issues/540)
- [Issue #655: Don't run validation on blur](https://github.com/charmbracelet/huh/issues/655)

### Bubbletea Patterns
- [Building Bubbletea Programs](https://leg100.github.io/en/posts/building-bubbletea-programs/) -- model stack, message routing
- [Overlay Composition Using Bubble Tea](https://lmika.org/2022/09/24/overlay-composition-using.html) -- scanline overlay technique
- [Bubbletea v2 Release Notes](https://github.com/charmbracelet/bubbletea/releases/tag/v2.0.0) -- enhanced keyboard
- [bubbletea-overlay package](https://pkg.go.dev/github.com/quickphosphat/bubbletea-overlay) -- overlay model wrapper

### gh-dash
- [gh-dash GitHub repository](https://github.com/dlvhdr/gh-dash) -- inline PR status badges

### Terminal Keyboard
- [Ctrl+Backspace in terminals](https://www.vinc17.net/unix/ctrl-backspace.en.html) -- escape sequence limitations
- [fzf --disabled flag](https://github.com/junegunn/fzf) -- disables filtering, not item selection
