# TUI Redesign Phase 7b (Config) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** The config editor follows spec 11 (N8): a full-screen frame with one huh form per section, grove-drawn section tabs, a fixed height, grove-drawn errors and keys, and an unsaved count measured against the values loaded from disk.

**Architecture:**

- `ViewConfig` becomes a full-screen frame in `renderFrame`, like `ViewDetailSheet`. The bordered, 60-to-80-column overlay goes away.
- `ConfigState` holds four `*huh.Form`s, one group each, and the active section. Grove switches sections; huh never sees the others.
- Every form uses `WithHeight(h)`, `WithShowHelp(false)`, `WithShowErrors(false)`, and the full frame width. A resize re-applies both.
- Each field keeps its loaded value (`ConfigField.Loaded`, renamed from `Default`, which already held the loaded value). The unsaved count is recomputed after every update.

**Spec:** section 11, 6.x config keys (`ctrl+n` / `ctrl+p`, `1`-`4`), 18 (config tests), 19 item 1.

## Verified before planning (spec 19 item 1)

A scratch program built the four sections as separate huh 2.0.3 forms with `WithHeight(14)` (and 20), no help, no errors, and tabbed through every field, feeding huh's commands back. Every view was exactly 14 (20) rows, in every section, after every tab, and with a validation error present. Without `WithHeight` the sections measured 11, 15, 14, and 7 rows. No fallback viewport is needed.

Two more probes shaped decisions 4 and 6:
- `tab` on a section's last field does nothing; `enter` there completes the form (`StateCompleted`).
- `enter` on an invalid last field keeps the form open and `Form.Errors()` reports the error.

## Global Constraints

As in phase 7a; the gate includes the integration suite. Expected golden churn: `TestGolden_Overlay_Config` is replaced by `TestGolden_Config` (sections, sizes).

## Decisions

| # | Decision | Why |
|---|---|---|
| 1 | **Layout, top to bottom:** header bar `<project> ▸ settings` with `● n unsaved` on the right when dirty; the tab row `[1] general  [2] behavior  [3] plugins  [4] protection`, active tab raised (selection background, bright text); a blank row; the form at height `h`; a status line; the footer | Spec 11. The project name, not "grove", matches the PR and issue frames and says whose settings these are. |
| 2 | **Narrow tab row:** when the full row does not fit, inactive tabs shrink to `[n]` | 44 columns cannot fit 54 cells of tabs. |
| 3 | **`h` = terminal height minus header, tabs, blank, status, and the footer's real height, at least 1**; the frame still goes through `clampBodyToHeight` | The footer can wrap at narrow widths; measuring it keeps the frame exact. `WithHeight` ignores `h <= 0` and would fall back to the natural height. |
| 4 | **Status line:** the active form's first error (Danger), blank otherwise | Spec 11: grove draws `Form.Errors()`. huh already draws each field's description under its title, so the line repeats nothing. The row is always there, so an error never moves the footer. |
| 5 | **Section keys:** `ctrl+n` / `ctrl+p` from anywhere, wrapping, consumed by grove before the form; `1`-`4` only when the focused field is not a `*huh.Input`. Config selects are built with filtering off. | Spec 11 and the config keys row. Digits stay typeable. huh's select binds `ctrl+n` / `ctrl+p` to down and up and `/` to a filter; three options need neither, so losing them is intended. |
| 6 | **Completing a section** (`enter` on its last field) moves to the next section, rebuilding the completed form from the current values; completing the last section behaves like `esc` (prompt when dirty, close when clean). `tab` on the last field stays, as huh does. | A completed huh form ignores input, so it must be rebuilt to revisit. The old single form saved on completion without a prompt; the prompt is safer and matches `esc`. |
| 10 | **The save prompt is a small centered confirm over the frame**: `Save changes?`, `⏎ save · esc discard` | Spec 15: small confirms are centered. The old prompt replaced the overlay body, which a full-screen frame does not have. |
| 7 | **Dirty is a count, recomputed after every update**, of fields whose value differs from `Loaded` | Today `Dirty` only ever turns on, so undoing a change still prompts. The spec's diagnosis (compares to defaults) was checked: `Default` already held the loaded value; the real defects are the sticky flag and no count. |
| 8 | **`?` types into a focused config text input** (`viewHasTextInput` covers config) | Today `?` in a config input opens help. Same rule as the digits. |
| 9 | **Unused legacy inline-edit fields leave `ConfigState`** (`Cursor`, `Editing`, `EditBuffer`, `EditCursorPos`, `EditOptionCursor`, `EditOriginalValue`) | Nothing reads them since the huh form; the repo is unreleased. |

## Review Focus

1. Height: identical frame height across all four sections, after tabbing, with an error, at 80x24 and 120x40. Test: Task 3.
2. Dirty count against loaded values, including undo back to the loaded value. Test: Task 4.
3. Digits and `?` type into inputs; switch sections elsewhere. Test: Task 5.
4. Save and discard paths unchanged in effect: only changed fields are written. Test: Task 4.

---

### Task 1: Workspace and baseline

Workspace `2026-10-12-tui-redesign-phase-7b`. Gate green.

### Task 2: Per-section forms and state

- [ ] Failing tests: `TestConfig_FormPerSection` (four forms after load, each with its section's fields); `TestConfigField_LoadedIsTheDiskValue`.
- [ ] Add the test helper `sendConfigKeys(m, keys...)`: it sends each key through `Update`, then runs the returned commands with a short timeout, expands batches, skips ticks, and feeds the messages back, but only while `ViewConfig` is active. huh moves focus through commands, so `sendKey` alone cannot tab; stopping when config closes keeps a save command from ever running in a test.
- [ ] Implement `buildSectionForm(fields, width, height)`; `ConfigState.Forms [ConfigTabCount]*huh.Form`, `Active`; rename `Default` to `Loaded`; remove the legacy fields; port existing config tests to the new state (list each in the ledger).
- [ ] Gate. Commit `refactor(tui): one huh form per config section`.

### Task 3: Full-screen frame

- [ ] Failing tests: `TestConfig_HeightStableAcrossSections` (frame line count equals the terminal height and the form block is the same height in every section, after tab, and with a validation error, at 80x24 and 120x40); `TestConfig_HeaderAndTabs` (project name, `▸ settings`, four tabs, narrow tabs at 44); `TestGolden_Config` (each section at 80x24, general at 120x40 and 44x24, dirty header).
- [ ] Implement the frame in `renderFrame`, the tab row, the status line, the footer hints for `ViewConfig`, and resize handling.
- [ ] Gate. Commit `feat(tui): config as a full-screen frame with section tabs`.

### Task 4: Dirty count and save

- [ ] Failing tests: `TestConfig_UnsavedCountAgainstLoaded` (change one field: `● 1 unsaved`; change it back: clean, and `esc` closes without a prompt); `TestConfig_SaveWritesOnlyChanged` (the save command's updates hold only the changed key). Check what `config.SetProjectConfigValues` does with an unquoted list value (`main, staging`); if it writes raw text, log it as pre-existing, do not widen the phase.
- [ ] Implement the count and wire it to the header and `esc`.
- [ ] Gate. Commit `fix(tui): count unsaved config changes against the loaded values`.

### Task 5: Section keys

- [ ] Failing tests: `TestConfig_CtrlNPWrap`; `TestConfig_DigitsJumpOutsideInputs` (on a confirm field `3` jumps to plugins; on an input `3` is typed); `TestConfig_QuestionMarkTypesInInput`; `TestConfig_CompletingSectionMovesOn` (tab past the last field of general lands on behavior; past protection with changes opens the prompt).
- [ ] Implement.
- [ ] Gate. Commit `feat(tui): config section keys`.

### Task 6: Docs

`docs/TUI.md` (config section, keys), help overlay `helpSectionsConfig`, changelog, spec 19 item 1 result. Gate. Commit.

### Task 7: Hands-on

tmux scratch repo, 80x24 and 120x40: open with `g c`; every section; tab through; `ctrl+n/p`; digits on a confirm and in an input; `?` in an input; change and undo (count goes to 0, `esc` closes); change and save (only that key written to `.grove/config.toml`); discard. herdr: open and close once.

## Done when

- Gate (with integration) and the full suite (known `TestIsCommandRunning` only) pass.
- Task 7 matches.
