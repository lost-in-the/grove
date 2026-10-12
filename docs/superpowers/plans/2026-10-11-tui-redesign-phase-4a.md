# TUI Redesign Phase 4a (Dashboard Rows, Header, Sort, Keys, Readability) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace the dashboard's two row delegates with the spec's one-line row, and land the header, sort, age, branch chip, density, list paging keys, and readability rules. Also make toasts visible again.

**Architecture:**

- One delegate, `rowDelegate`, renders a row from a pure layout step (`layoutRow`). The layout step decides which columns show at a width (the shed order) and how wide the alias is. Rendering only paints.
- `v` adds a second line (the commit message) to the same delegate. There is no second delegate.
- Toasts become a top-right layer drawn over the finished frame, instead of a header composite that never fit.
- The palette becomes data (hex pairs), so the contrast test reads the real values.

**Tech Stack:** Go 1.26+, Bubble Tea v2, lipgloss v2, bubbles v2 list.

**Spec:** `docs/superpowers/specs/2026-10-10-tui-redesign-design.md` sections 5.1 to 5.5, 5.7 to 5.9, 6.2 (list paging keys).

**Phase 4b** (next plan) covers width tiers (selection line, detail sheet, 44-column guard), pins and the `m` strip, and `-`.

## Global Constraints

- Gate: `make lint test` with golangci-lint v2.14, inside the sandbox. Only signed commits run outside it.
- Commits: `timeout --kill-after=5s 120s git -c commit.gpgsign=true commit ...`.
- **Expected golden churn:** every dashboard golden changes (new rows and header). `TestGolden_Dashboard_SortModes/recent` becomes `age`. Overlay goldens change only where the dashboard shows behind the dim. Branch form goldens must not change. Ledger each churned golden directory by name.
- Measure every column with `lipgloss.Width`. No `len` on display strings.
- No text renders in `SurfaceDim` or `SurfaceBorder`; those are for rules and borders.

## Decisions

| # | Decision | Why |
|---|---|---|
| 1 | **Quick-switch `1`-`9` by row position keeps working until 4b,** but rows no longer show the numbers | Pins replace it in 4b. One phase with a hidden key beats shipping half of pins. |
| 2 | **The pin column is not drawn in 4a** | No pins exist until 4b. The layout step takes a `pins bool` so 4b only flips it. |
| 3 | **The agent column draws when `agentAware`**: true when the first fetch reports the herdr backend, and never changed after | Spec 7.1 ("computed once"). Taken from `fetchContext.backend`, not resolved in `NewModel`: resolving can probe herdr and would slow startup. Today's tail word `◆ blocked` moves to the lead glyph (5.1). The header triage summary and attention sort stay in Phase 6. |
| 4 | **`gg` is a two-key chord without a strip** in 4a | Phase 5 adds the `g` strip. `g` then any other key does nothing. |
| 5 | **The upgrade badge stays in the footer until Phase 7** | N9 moves it to the header with `g u`. |
| 6 | **Sort `recent` (last accessed) is replaced by `age` (last commit)** | Spec 5.7. Needs a raw commit time on `WorktreeItem`. |
| 7 | **`compact_mode` pref: unset means one line** | Spec 5.4. A saved `false` keeps two lines. `tui.compact_list` behaves the same. |
| 8 | **Toasts draw as a compositor layer over rows 0 to 2, right-aligned, without dimming** | They have not rendered since `ToastModel.View` started padding to full width. Spec 15.3 puts toasts on the compositor at Z2; the lipgloss compositor also avoids SGR state bleeding into the box. |
| 9 | **Rows render age from `CommitTime` against an injectable clock** | Goldens must not drift with the wall clock, and test items must carry a commit time or the age column is never exercised. |
| 10 | **`u` stops paging the list** | Today, with no cached update, bare `u` falls through to the bubbles list's previous-page binding. The Task 9 rebind removes that; spec 6.2 reserves `u`. |

## Review Focus

1. **Shed order at every width from 44 to 200.** Lead columns and the tail never shed; the chip goes first, then age, then counts collapse, then the alias truncates to 16. Test: Task 4, `TestLayoutRow_ShedOrder` and `TestLayoutRow_NeverOverflows` (every width 44..200, every fixture row, `lipgloss.Width(row) == width`).
2. **Selection background is continuous across the row**, including padding and glyph cells. Test: Task 5, `TestRowDelegate_SelectedRowHasNoBareCells`.
3. **Cursor and status are separate columns.** The selected row still shows its status dot. Test: Task 5, `TestRowDelegate_SelectedKeepsStatus`.
4. **Contrast is measured on the real palette**, against `SurfaceBg` and `SelectionBg`, in dark, light, and high contrast. Test: Task 10.
5. **`G`, `gg`, `ctrl+d`, `ctrl+u` move the cursor; `h l b u f d` no longer page the list.** Test: Task 9.

---

## File Structure

| File | Change | Responsibility |
|---|---|---|
| `internal/tui/compositor.go`, `toast.go`, `model.go`, `view_prs_v2.go` | Modify | Toast layer |
| `internal/worktreeinfo/info.go`, `internal/tui/data.go` | Modify | `CommitTime` |
| `internal/tui/age.go` (+test) | Create | `shortAge` |
| `internal/tui/row_layout.go` (+test) | Create | `layoutRow`, `middleTruncate` |
| `internal/tui/row_delegate.go` (+test) | Create | `rowDelegate` |
| `internal/tui/list.go`, `list_v2.go` | Delete (after porting helpers) | Old delegates |
| `internal/tui/header.go` (+test) | Modify | Header 5.5 |
| `internal/tui/sort.go` (+tests) | Modify | name, age, dirty; reverse |
| `internal/tui/prefs.go`, `internal/config/config.go` | Modify | Density default, `list_secondary` |
| `internal/tui/worktree_list.go`, `keys.go`, `model.go` | Modify | Paging keys |
| `internal/theme/colors.go`, `accessibility_test.go` | Modify | Palette as data, contrast test |
| `internal/tui/detail_v2.go`, `model.go` | Modify | Focus label |
| `docs/TUI.md`, `docs/CONFIGURATION_REFERENCE.md`, `CHANGELOG.md` | Modify | Docs |

---

### Task 1: Workspace and baseline

As in phase 3b, workspace `2026-10-11-tui-redesign-phase-4a`. Gate green at the plan commit.

### Task 2: Toasts as a top-right layer

- [ ] **Step 1: Failing tests**
  - `TestToast_VisibleOnDashboard`: `newTestModel(withItems(3), withSize(80, 24), withToastVisible("Deleted testing", ToastSuccess))`; `ansi.Strip(m.viewString())` contains `Deleted testing`, and line 0 still starts with the project name (the toast covers only the right side).
  - `TestToast_VisibleOverPRBrowser`: same with `withPRData()`.
  - Run: FAIL (no toast text).
- [ ] **Step 2: Implement**
  - `ToastModel.View(width)` returns the natural box (no left padding).
  - `compositor.go`: `composeLayerAt(bg, layer string, w, h, x, y int) string`: the same lipgloss `Compositor` path as `composeOverlayAt`, without the dim, placing the layer at column `x`, row `y`. The toast goes at `x = width - lipgloss.Width(box)`, `y = 0`.
  - `renderDashboard` and `view_prs_v2.go` stop calling `compositeToastOnHeader`; `View()` applies the toast layer last, after any overlay, so it is the top layer. Delete `compositeToastOnHeader` and `applyToastToHeader`.
- [ ] **Step 3:** Update `TestGolden_Dashboard_WithToast` (it should now show the toast; read it). Gate. Commit `fix(tui): draw toasts again, as a top-right layer`.

### Task 3: Commit time and the age format

- [ ] **Step 1: Failing tests** (`internal/tui/age_test.go`):

```go
func TestShortAge(t *testing.T) {
	now := time.Date(2026, 10, 11, 12, 0, 0, 0, time.UTC)
	for _, c := range []struct {
		ago  time.Duration
		want string
	}{
		{0, "now"}, {59 * time.Second, "now"},
		{time.Minute, "1m"}, {59 * time.Minute, "59m"},
		{time.Hour, "1h"}, {23 * time.Hour, "23h"},
		{24 * time.Hour, "1d"}, {6 * 24 * time.Hour, "6d"},
		{7 * 24 * time.Hour, "1w"}, {52 * 7 * 24 * time.Hour, "52w"},
		{365 * 24 * time.Hour, "1y"}, {800 * 24 * time.Hour, "2y"},
		{-time.Hour, "now"},
	} {
		if got := shortAge(now.Add(-c.ago), now); got != c.want {
			t.Errorf("%v ago: got %q, want %q", c.ago, got, c.want)
		}
	}
	if got := shortAge(time.Time{}, now); got != "" {
		t.Errorf("zero time: got %q, want empty", got)
	}
}
```

  Note `52w` vs `1y`: 52 weeks (364 days) is weeks; 365 days and up is years.
- [ ] **Step 2: Implement**
  - `worktreeinfo.HeadAndRecentCommits`: add `%ct` as a fifth field; `HeadCommitInfo.Time time.Time`.
  - `WorktreeItem.CommitTime time.Time`, set in `enrichGitInfo`. `CommitAge` stays for the detail pane.
  - `age.go`: `shortAge(t, now time.Time) string` per spec 5.3. At most 3 cells.
  - Test fixtures: a package-level `testNow` instant; `makeTestItems` sets `CommitTime: testNow.Add(-time.Duration(i) * time.Hour)`.
  - Delete `compactAge` and its test once Task 5 stops using it (lint flags it).
- [ ] **Step 3:** Gate. Commit `feat(tui): record commit time and format age in three cells`.

### Task 4: Row layout (pure)

`row_layout.go`:

```go
// rowCols is what one row could show, already as plain text.
type rowCols struct {
	Pin, Agent bool   // lead columns present (project-wide, not per row)
	Alias      string
	Chip       string // "" when no chip
	Ahead, Behind, Dirty int // counts; display order is ↑ ↓ ~, collapse keeps ↓ then ~ then ↑
	Age        string
}

// rowLayout is the decided shape of a row at a width.
type rowLayout struct {
	AliasWidth  int
	ShowChip    bool
	ChipWidth   int
	Counts      []string // display order ↑ ↓ ~, after collapse
	ShowAge     bool
}

func layoutRow(c rowCols, width int) rowLayout
func middleTruncate(s string, w int) string
```

Fixed widths: lead columns 2 each (pin, agent when present; cursor; status), counts 11, age 3, tail 4, single-space gaps between alias, chip, counts, age, tail.

- [ ] **Step 1: Failing tests**
  - `TestMiddleTruncate`. Rule: keep the last `-` segment (the tail) and as much of the head as fits, joined by `…`. The head always includes a leading ticket id (`^[a-z]+-\d+`) when the width allows. Cases:
    - fits: `("fix-bug", 16) == "fix-bug"`;
    - ticket kept: `("abc-1349-migrate-the-read-path", 24) == "abc-1349-migrate-…-path"` (width 24);
    - no ticket: `("refactor-the-whole-thing", 16) == "refactor-th…-thing"` trimmed so the result is exactly 16 cells;
    - tail longer than the width: plain end truncation with `…`;
    - the result is exactly `w` cells whenever truncation happens.
  - `TestLayoutRow_ShedOrder`: one fixture with a long alias, a chip, three counts, and an age. Assert at decreasing widths that the chip goes first (and before the alias would drop under 24), then age, then counts collapse to one (`↓` first, then `~`, then `↑`), then the alias shrinks to 16.
  - `TestLayoutRow_NeverOverflows`: for widths 44..200 and fixtures (short alias, long alias, no counts, all counts, with and without pin and agent), the sum of column widths plus gaps equals the width or less, and the alias width is at least 16 whenever width ≥ 44.
- [ ] **Step 2:** Implement. The alias column is flex: it gets the space left after the fixed columns and the chip; the chip is flex too, and gets what the alias does not need, up to its text width.
- [ ] **Step 3:** Gate. Commit `feat(tui): lay out dashboard rows with a fixed shed order`.

### Task 5: The row delegate

`row_delegate.go`: `rowDelegate{TwoLine, Pins, AgentAware bool; Secondary string; Now func() time.Time}` (`Now` defaults to `time.Now`; `newTestModel` and `goldenModel` pin it to `testNow`); `Height()` is 1 or 2; `Spacing()` 0. `Render` builds `rowCols` from the item, calls `layoutRow`, and paints:

- agent glyph: `!` Danger (blocked), `▸` Warning (working), `✓` Success (done), blank otherwise;
- cursor `❯` on the selected row, blank otherwise;
- status: `●` Success (current), `●` Warning (dirty), `✗` Danger (stale), `○` TextMuted (clean);
- alias in NormalItem (CurrentItem when current), middle-truncated;
- chip `⎇ branch` in TextMuted;
- counts right-aligned in 11 cells: `↓` Danger, `~` Warning, `↑` Success;
- age right-aligned in 3 cells, TextMuted;
- tail: `⬢`/`⬡` session, then `◆`/`◇` container, in 4 cells.
- Two-line mode: line 2 is the commit message in TextMuted, indented to the alias column.
- Selected rows carry `SelectionBg` on every segment, as today's `withBg` does.

Chip rule (5.8): with `Secondary == "branch"`, show `⎇ <branch>` only when `worktree.DeriveWorktreeName(branch, "last_segment") != alias`. With `"dir"`, show the directory base name. With `"off"`, none.

- [ ] **Step 1: Failing tests** (render with `ansi.Strip` and `lipgloss.Width`):
  - `TestRowDelegate_OneLineAnatomy`: at 80 columns a dirty, behind, tmux-active item renders as `  ❯ ● root            ⎇ main ...  ↓11 ~13   2d  ⬢ ◆` shape: assert column positions (cursor at cell 0, status at 2, alias at 4), the chip, counts, age, and tail glyphs.
  - `TestRowDelegate_SelectedKeepsStatus`: the selected dirty row shows both `❯` and `●`.
  - `TestRowDelegate_SelectedRowHasNoBareCells`: every cell of the selected row's raw output is covered by a background SGR (parse with `ansi` or assert no `\x1b[m` followed by a plain space).
  - `TestRowDelegate_ChipOnlyWhenBranchDiffers`: alias `login` on `feature/login` has no chip; alias `auth` on `feat/oauth` has one; `Secondary: "off"` never has one.
  - `TestRowDelegate_AgentColumnOnlyWhenAware`, `TestRowDelegate_AgentGlyphs`.
  - `TestRowDelegate_TwoLineShowsCommit`.
  - `TestRowDelegate_WidthIsExact`: for widths 44..160, every rendered line is exactly the list width.
- [ ] **Step 2: Implement and wire.** `Model.agentAware` is set from the first `worktreesMsg` whose fetch context names a backend, and never changed after (`TestAgentAware_SetOnceFromFirstFetch`: a later fetch reporting tmux does not clear it). `model.go`: one delegate; `computeColumnWidths` becomes `refreshDelegate()` (sets `TwoLine` from density, `AgentAware`, `Secondary` from config). Delete `list.go`'s `WorktreeDelegate`, `ComputeDelegateWidths`, the list column header, and `list_v2.go`'s `WorktreeDelegateV2`. Keep and move any helper the detail pane or tests still use. `delegateHeight()` reads the delegate.
- [ ] **Step 3:** Port or delete the old delegate tests; ledger each. Update dashboard goldens; read three of them (80x24, 120x40, narrow) and check the anatomy by eye.
- [ ] **Step 4:** Gate. Commit `feat(tui): one-line dashboard rows with lead glyphs and a fixed tail`.

### Task 6: Density default and `list_secondary`

- [ ] **Step 1: Failing tests**
  - `TestDensity_DefaultIsOneLine`: no prefs, no config: delegate height 1.
  - `TestDensity_SavedFalseKeepsTwoLines`: prefs `compact_mode: false`: height 2.
  - `TestDensity_VToggles`.
  - config: `TestConfig_ListSecondary` parses `branch`, `dir`, `off`; default `branch`; an invalid value fails validation with a message naming the three values.
- [ ] **Step 2: Implement.** `config.TUIConfig.ListSecondary string \`toml:"list_secondary"\``, merge and validate like the others. Startup density: config `compact_list` when set, else prefs `compact_mode` when set, else one line.
- [ ] **Step 3:** Gate. Commit `feat(tui): one-line rows by default; list_secondary picks the chip`.

### Task 7: Header

New header, left to right with ` · `: project, count (`5 worktrees`), sort (`⬍ name`, or `⬍ name ↑` when reversed), then right-aligned `n/total` for the filtered set. Drop `on <branch>`, the container target, and the right-side current name (the current row's green dot and the detail pane carry them).

- [ ] **Step 1: Failing tests** (`header_test.go`): new format at 80; `n/total` tracks the cursor and the filtered count; at 44 columns the count drops its word (`5`) and the project name truncates with `…`, while `n/total` stays.
- [ ] **Step 2:** Implement; `buildDashboardHeader` passes `Cursor`, `Total`, `SortLabel`, `SortReversed`.
- [ ] **Step 3:** Gate. Commit `feat(tui): header shows sort and cursor position`.

### Task 8: Sort

- [ ] **Step 1: Failing tests**
  - `TestSort_CycleIsNameAgeDirty`.
  - `TestSort_AgeNewestFirst` uses `CommitTime`; ties break by alias.
  - `TestSort_Reverse`: `O` reverses every mode; the main worktree stays first either way.
- [ ] **Step 2:** Implement: `SortByAge` replaces `SortByLastAccessed`; `Model.sortReversed bool`; `keys.SortReverse` on `O`. Port `sort_test.go` and `sort_order_test.go`.
- [ ] **Step 3:** Gate. Commit `feat(tui): sort by name, age, or dirty, and reverse with O`.

### Task 9: List paging keys and motion

- [ ] **Step 1: Failing tests**
  - `TestWorktreeList_PagingKeys`: the list's `PrevPage` keys are exactly `pgup`; `NextPage` exactly `pgdown`; `GoToStart` `home`; `GoToEnd` `end`.
  - `TestDashboard_MotionKeys` (12 items, 80x24): `G` selects the last row; `g` `g` selects the first; `g` then `x` changes nothing; `ctrl+d` moves down half the visible rows, `ctrl+u` back.
  - `TestDashboard_ListNoLongerPagesOnLetters`: `l`, `b`, `u`, `f`, `d` do not change the page (`f`, `d`, `b` open their own actions; assert the list's `Paginator.Page` is unchanged).
  - `TestDashboard_LAndHFocusDetail`: at 120 columns `l` focuses the detail pane, `h` returns.
  - `TestDashboard_MotionKeysTypeIntoFilter`: while the list is filtering, `g`, `G`, `h`, `l` are typed into the filter; `pendingG` is cleared by any key, including a filter keystroke.
  - `TestDashboard_UNoLongerPages`: with no update cached, `u` leaves the page unchanged.
- [ ] **Step 2: Implement.** `newWorktreeList` sets the four bindings. `Model.pendingG bool`. `G`, `ctrl+d`, `ctrl+u`, `h`, `l` in `handleDashboardKey` before `routeToList`, guarded by `m.list.FilterState() == list.Unfiltered` (the same guard `handleQuickSwitch` uses).
- [ ] **Step 3:** Gate. Commit `feat(tui): vim motion keys; the list pages only on pgup/pgdown`.

### Task 10: Readability (N21)

- [ ] **Step 1: Palette as data.** `theme.Palette` holds `{Dark, Light string}` per token; `DefaultColorScheme` and `HighContrastColorScheme` build from `DefaultPalette` and `HighContrastPalette`. No values change in this step. Gate.
- [ ] **Step 2: Failing test** (expect dark `TextMuted` and `Danger` on `SelectionBg` to fail, about 4.4:1, as well as `TextDim`; goldens are NO_COLOR, so palette changes churn none) `TestPalette_TextContrast`: for each text token (Primary, Secondary, Success, Warning, Danger, Info, TextNormal, TextBright, TextMuted, TextDim), each palette (default dark, default light, high contrast), and each background (`SurfaceBg`, `SelectionBg`, `HeaderBg`), `ContrastRatio >= 4.5`. Run: FAIL; ledger which pairs fail and by how much.
- [ ] **Step 3: Fix the palette** with the smallest changes that pass. Prefer lightening dark-mode text and darkening light-mode text over changing backgrounds. Update the old WCAG tests to read the palette instead of duplicated hex values.
- [ ] **Step 4: Focus label.** `TestDetail_FocusedTitleSaysSo`: the focused detail pane's title contains `detail · focused`; unfocused does not. Implement in the detail frame title.
- [ ] **Step 5:** Gate. Commit `feat(tui): 4.5:1 contrast on every theme and a focus label`.

### Task 11: Docs and changelog

`docs/TUI.md`: row anatomy, header, sort, density, new keys, `list_secondary`. `docs/CONFIGURATION_REFERENCE.md`: `list_secondary`, `compact_list` default. Changelog under Unreleased. Gate. Commit.

### Task 12: Hands-on, tmux and herdr

| # | Check |
|---|---|
| 1 | 80x24 and 120x40: rows match the anatomy; selected row keeps its dot; tail glyphs line up |
| 2 | Narrow the terminal from 120 to 44: chip goes, then age, then counts collapse, then alias truncates |
| 3 | `v` adds and removes the commit line; restart keeps the choice |
| 4 | `o` cycles name, age, dirty; `O` reverses; header shows both |
| 5 | `G`, `gg`, `ctrl+d`, `ctrl+u`, `l`, `h` |
| 6 | A delete or copy shows its toast top-right |
| 7 | Under herdr: the agent column appears; under tmux it does not |
| 8 | `NO_COLOR=1` and `GROVE_LIGHT_MODE=1`: rows still read |

## Done when

- Gate and integration pass; full suite outside the sandbox has only the known `TestIsCommandRunning`.
- No production reference to `WorktreeDelegate`, `WorktreeDelegateV2`, `SortByLastAccessed`, or `compositeToastOnHeader`.
- The contrast test covers the real palette and passes.
- Task 12 matches in tmux and herdr.
