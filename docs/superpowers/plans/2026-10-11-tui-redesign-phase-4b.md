# TUI Redesign Phase 4b (Width Tiers, Pins, Last-Active) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Finish the dashboard phase: width tiers (detail pane at 110+, selection line and detail sheet below, a guard under 44), pins with the `m` strip and `1`-`9`, and `-` to the last-active worktree.

**Architecture:**

- Pins are pure functions over a `map[string]string` (slot digit to alias), stored in `UIPrefs`. The model only calls them and saves.
- Prefs are saved load-modify-save, so one setting never wipes another.
- The `m` strip is a docked block above the footer, not a frame. The list shrinks by its height. Phase 5 generalizes strips; 4b draws only this one.
- The detail sheet is a full-screen frame on the view stack (`ViewDetailSheet`), reusing the detail viewport.
- The guard is a check at the top of `viewContent`; state is untouched.

**Spec:** `docs/superpowers/specs/2026-10-10-tui-redesign-design.md` sections 5.6, 6.2 (`-`, `1`-`9`, `h`/`l`), 8.

## Global Constraints

As in phase 4a: gate in the sandbox, signed commits outside it, `goimports -local github.com/lost-in-the/grove`. Expected golden churn: stacked-layout dashboard goldens (selection line replaces the stacked detail), and any golden with pins (new). Wide goldens change only where the breakpoint moved (100 to 110).

## Decisions

| # | Decision | Why |
|---|---|---|
| 1 | **Positional quick-switch is removed**; `1`-`9` mean pin slots only | Spec N5, N18. Rows lost their numbers in 4a. |
| 2 | **`↩` marks the last-active worktree in the cursor column** of unselected rows | The cursor column is blank on those rows, so no new column. The selected row shows `❯`. |
| 3 | **Last-active comes from `state.LastWorktree`**, read when the model starts | Same source as `grove last`. |
| 4 | **An empty slot flashes as an info toast** | Toasts are the existing flash; Phase 5 adds message history. |
| 5 | **The selection line is `alias · ⎇ branch · commit subject`**, dim, truncated to width | Spec 5.6 and the 80x24 mockup in 5.1. |
| 6 | **The PR and issue browsers use the same 110 breakpoint** | One breakpoint for every split view. |
| 7 | **The `Layout` enum becomes the spec's three tiers**: guard (<44), list (44 to 109), split (110+) | The old Narrow/Medium/Wide boundaries (80, 100) mean nothing after this phase. `TestNarrowLayoutBreakpoints` is rewritten to the new boundaries. |
| 8 | **Pins are set from the list only**; `m` in the focused detail pane does nothing | `handleDashboardDetailKey` runs first. Pinning while reading the detail pane is not a case worth a second path. |
| 9 | **Last-active matches `ShortName` or `displayName()`** | The CLI records `mgr.DisplayNameForPath(path)`. `-` on the current worktree quits without a switch, like a pin. |

## Review Focus

1. Pin table semantics (spec 8.1): pin, re-pin same slot unpins, move from another worktree, `m 0`, one slot per worktree. Test: Task 3.
2. Prefs never clobber: toggling density keeps pins; pinning keeps density. Test: Task 2.
3. Prune only after a successful fetch; a failed fetch keeps pins. Test: Task 4.
4. `1`-`9` with a filter applied work; while typing in the filter they type. Test: Task 4.
5. The guard keeps all state: shrink under 44 and widen back, the cursor, view stack, and form contents are unchanged. Test: Task 7.

---

### Task 1: Workspace and baseline

Workspace `2026-10-11-tui-redesign-phase-4b`. Gate green.

### Task 2: Prefs load-modify-save, with pins

- [ ] Failing tests: `TestUIPrefs_UpdateKeepsOtherFields` (save pins, then a density update, both survive on disk), `TestUIPrefs_PinsRoundTrip`, `TestToggleCompact_KeepsPins` (model with pins, press `v`, reload prefs: pins intact).
- [ ] Implement `UIPrefs.Pins map[string]string \`json:"pins,omitempty"\``; `updateUIPrefs(root string, fn func(*UIPrefs)) error` (load or zero value, apply, save); `toggleCompactMode` uses it.
- [ ] Gate. Commit `fix(tui): save UI prefs without dropping other settings`.

### Task 3: Pin rules (pure)

`pins.go`:

```go
type pinSet map[string]string // slot "1".."9" -> alias

func (p pinSet) slotOf(alias string) string           // "" when unpinned
func (p pinSet) pin(slot, alias string) pinSet        // spec 8.1 table; slot "0" unpins alias
func (p pinSet) rename(old, new string) pinSet
func (p pinSet) drop(alias string) pinSet
func (p pinSet) prune(aliases map[string]bool) pinSet // keep only existing aliases
```

All return a new map; inputs are never mutated.

- [ ] Failing tests (`pins_test.go`), one per spec 8.1 row: pin to empty slot; same slot again unpins; slot held by another moves it (the other loses the slot); pinning a worktree already in slot 2 to slot 5 leaves only slot 5; `0` unpins from any slot; rename keeps the slot; drop removes; prune keeps existing; inputs unchanged.
- [ ] Implement. Gate. Commit `feat(tui): pin rules for slots 1 to 9`.

### Task 4: Pin keys, the `m` strip, and upkeep

- [ ] Failing model tests:
  - `TestPins_MThenDigitPins`: `m`, `3` pins the selected worktree; prefs on disk have it (use `t.TempDir()` as project root with a `.grove` dir).
  - `TestPins_MShowsStrip`: after `m`, the view contains ` m pin to slot · <alias> is selected` and the slot row with `·` for empty slots; after `esc`, it is gone and nothing changed.
  - `TestPins_MOtherKeyCloses`: `m`, `x` closes the strip, no change.
  - `TestPins_DigitSwitches`: with slot 2 pinned to `fix-bug`, `2` sets `switchTo` to its path and quits; `2` on the current worktree quits without `switchTo`.
  - `TestPins_EmptySlotFlashes`: `4` with slot 4 empty shows toast `slot 4 is empty · m 4 to pin` and does not quit.
  - `TestPins_DigitWithFilterApplied` works; `TestPins_DigitWhileTypingFilterTypes`.
  - `TestPins_RenameMovesPin`, `TestPins_DeleteDropsPin` (through `renameCompleteMsg` and `worktreeDeletedMsg`).
  - `TestPins_PrunedOnlyAfterSuccessfulFetch`: a fetch error keeps a pin to a missing alias; a successful fetch without that alias prunes it.
  - `TestPins_NoPositionalQuickSwitch`: with no pins, `2` does not switch.
- [ ] Also: `TestPins_StripRestoresListHeight` (list height with the strip open is smaller; after `esc` it is back to the original) and golden `TestGolden_Dashboard_PinStrip`.
- [ ] Implement: `Model.pins pinSet` loaded at start; `pendingM bool` (cleared in `pushView` like `pendingG`); `handlePinKey` before motion keys; `renderPinStrip(width)`; `updateLayout` runs on open and close and subtracts the strip height while `pendingM`; `clampBodyToHeight` counts the strip; delete `handleQuickSwitch`.
- [ ] Gate. Commit `feat(tui): pin worktrees to 1-9 with m and a slot strip`.

### Task 5: The pin column

- [ ] Failing tests: `TestRowDelegate_PinColumnShowsSlot` (with `PinOf` set, the pinned row starts with its digit, others with two blanks); `TestRowDelegate_NoPinColumnWithoutPins`; width exactness still holds 44..200 with pins and agent columns.
- [ ] Implement: `rowDelegate.PinOf map[string]string` (alias to slot); `Pins = len(pins) > 0`; digit in `TextMuted`. Golden `TestGolden_Dashboard_Pinned`.
- [ ] Gate. Commit `feat(tui): show pin slots in a lead column`.

### Task 6: `-` and the last-active marker

- [ ] Failing tests: `TestLastActive_DashSwitches` (`-` with last = `fix-bug` sets `switchTo` and quits); `TestLastActive_CurrentQuitsWithoutSwitch`; `TestLastActive_NoneFlashes` (toast `no previous worktree`); `TestRowDelegate_LastActiveMarker` (`↩` in the cursor column of the unselected last-active row; selected row shows `❯`).
- [ ] Implement: `Model.lastActive string` from `stateMgr.GetLastWorktree()` in `NewModel`; `rowDelegate.LastActive string`; `-` binding.
- [ ] Gate. Commit `feat(tui): - switches to the last-active worktree`.

### Task 7: Width tiers

- [ ] Failing tests:
  - `TestLayout_DetailPaneFrom110`: at 110 the dashboard is side by side; at 109 it is not.
  - `TestLayout_SelectionLineBelow110`: at 80x24 the line above the footer is `<alias> · ⎇ <branch> · <subject>` for the selected row, and there is no stacked detail card.
  - `TestDetailSheet_TabAndLOpenEscCloses`: at 80 columns `tab` and `l` push `ViewDetailSheet`; it renders the detail full screen; `j`/`k` scroll; `esc`, `h`, `tab` pop back with the cursor kept.
  - `TestGuard_Under44`: at 43 columns the screen is one line `grove needs ≥44 columns`; widening to 80 restores the same view, cursor, and an open branch form's text.
- [ ] Before writing the tests: `grep -n "detailFocused\|withSize(80" internal/tui/*_test.go`. Tests that press `tab` below 110 and assert `detailFocused` port to the sheet.
- [ ] Implement:
  - `responsive.go`: the three spec tiers replace Narrow/Medium/Wide; rewrite `TestNarrowLayoutBreakpoints`.
  - `renderSelectionLine`; the stacked detail card goes.
  - `tab`/`l` in `handleDashboardKey` branch on tier: focus the pane at 110+, push the sheet below. `handleDashboardDetailKey` and `dashboardDetailFocusedHints` are 110+ only.
  - `ViewDetailSheet` plumbing: full-screen case in `renderFrame` (beside `ViewPRs`), `handleKey` dispatch, `helpSectionsFor`, `CompactHints`, `TestHelpSectionsForAllViews`, `viewHasTextInput` (false), and `handleWindowSize` resizing its viewport.
  - Guard in `viewContent`.
- [ ] Goldens: stacked dashboards churn (selection line); add `TestGolden_DetailSheet`, `TestGolden_Guard`.
- [ ] Gate. Commit `feat(tui): width tiers with a selection line, detail sheet, and 44-column guard`.

### Task 8: Docs and changelog

`docs/TUI.md` (layout tiers, pins, `m`, `-`, keys table, file map), help overlay entries, changelog. Gate. Commit.

### Task 9: Hands-on, tmux and herdr

| # | Check |
|---|---|
| 1 | Resize 140 to 40 and back: detail pane, selection line, guard, restore |
| 2 | `tab` at 80 opens the sheet; `esc` returns |
| 3 | `m 2` pins; strip shows aliases; `2` switches; restart keeps the pin |
| 4 | `m 2` on another worktree moves it; `m 0` unpins; `4` on empty flashes |
| 5 | Rename and delete a pinned worktree in a scratch repo |
| 6 | `-` goes to the previous worktree; its row shows `↩` |

Pins and `-` are checked only in the scratch repo: they write `.grove/ui_prefs.json`.

## Done when

- Gate, integration, and the full suite (known `TestIsCommandRunning` only) pass.
- No production reference to `handleQuickSwitch`.
- Task 9 matches in tmux and herdr.
