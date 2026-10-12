# TUI Redesign Phase 5b (Omni, `?` as Omni, Message History) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** One search overlay on `space` that finds worktrees, actions, PRs, and issues; `?` opens it scoped to actions; warnings and errors are kept in a session history that omni can open.

**Architecture:**

- `omni.go` holds `OmniState` (input, target worktree, scope, cursor) and a pure `omniResults(query, sources) []omniGroup`. Sources are built from the model: list items, the action table (5a), and cached PRs and issues.
- Matching uses `sahilm/fuzzy` (promoted to a direct dependency). It reports byte offsets; highlighting converts them to rune positions.
- The overlay reuses the branch form's frame rules: fixed size, top-anchored, composited at `formTop`.
- Message history lives on `ToastModel`: every warning and error toast is appended (newest 50). The `messages` action opens a `ViewMessages` overlay.

**Spec:** sections 9 and the N22 note in section 14.

## Global Constraints

As in phase 5a. Expected golden churn: the dashboard footer (spec 5.1 hints), and the help overlay goldens for the dashboard (now omni). New goldens for omni and messages.

## Decisions

| # | Decision | Why |
|---|---|---|
| 1 | **`?` opens omni scoped to actions on the dashboard only.** Other views keep their help overlay until their keys join the action table. | Only the dashboard has a table (5a). A help list with nothing to run would be a regression for the PR browser, form, and config. |
| 2 | **PRs and issues come from a session cache** filled whenever the PR or issue browser fetches | Spec 9.2: omni never calls `gh`. The browsers clear their state on exit, so omni needs its own copy. |
| 3 | **`⏎` on a PR or issue opens its browser with the cursor on it** | The PR review view arrives in Phase 8; until then the browser is the closest place. |
| 4 | **Omni lists runnable actions only**: not the list-owned keys, not the strip openers (`g`, `m`), not `1`-`9`; disabled actions are hidden | A row must do something when chosen. `pin.switch` needs a digit. |
| 5 | **One result per row**, with the group name on the group's first row | The spec mockup puts a group on one line; rows make the cursor and highlighting legible and fit a fixed frame. |
| 6 | **The footer becomes the spec 5.1 set**: `↑↓ navigate · enter switch · ␣ find · tab detail · / filter · g goto · ? help` | `?` now lists every action, so the footer no longer needs `U`, `n`, `o`, `q`. It also stops the footer wrapping at 96 to 104 columns. |
| 7 | **"recent" uses `WorktreeItem.LastAccessed`** | Spec 19 item 5: the data exists in state and is already on the items. |
| 8 | **Each item matches several candidate strings and keeps the best**: worktrees on alias and branch; actions on label, each synonym, and key; PRs and issues on number, title, branch. The row shows the matched candidate, so a synonym hit reads `copy worktree · fork` with `fork` lit. | `sahilm/fuzzy` returns byte offsets into the one string it matched; highlighting is only right on text the row shows. |
| 9 | **Enter on an action pops omni, then runs the action**; if the target worktree is gone, it flashes and does nothing | Popping restores the cursor to the target (the frame recorded its path), so the action runs on it, and the action's own overlay does not stack on omni. |
| 10 | **`space` gets its own `Omni` binding** in `keys.go`; `Action.display()` returns `""` for an action with no key | `Toggle` is exempt from the drift test, so borrowing it would hide a missing entry. `app.messages` has no key. |
| 11 | **The dashboard help content moves into omni**: a `keyboard shortcuts` action (synonyms `help`, `keys`) opens the old help overlay | It holds content omni rows do not, such as the CLI companions. Nothing is lost. |

## Review Focus

1. Enter runs the chosen action on the target worktree captured when omni opened, even if the list changed underneath.
2. Rune-correct highlighting for non-ASCII aliases. Test: Task 2.
3. Fixed frame: typing never changes the frame size. Test: Task 4.
4. Opening omni never calls `gh` (no command returned that fetches). Test: Task 3.
5. Message history keeps warnings and errors only, newest first, capped at 50. Test: Task 5.

---

### Task 1: Workspace and baseline

Workspace `2026-10-11-tui-redesign-phase-5b`. Gate green.

### Task 2: Matching and results (pure)

- [ ] `go get github.com/sahilm/fuzzy` as a direct dependency (it is already in `go.sum`).
- [ ] Failing tests (`omni_test.go`):
  - `TestOmniResults_GroupsAndOrder`: query `ren` returns actions first (rename worktree), then worktrees (refactor), then PRs; at most 5 per group.
  - `TestOmniResults_Synonyms`: `fork` finds copy worktree; `checkout` finds switch branch; `trash` finds delete worktree.
  - `TestOmniResults_EmptyQuery`: groups `needs you` (only when agent-aware and nonempty), `pinned`, `recent` (by LastAccessed, newest first, excluding pinned and current).
  - `TestOmniResults_ActionsScope`: scope actions returns only actions, and with an empty query lists them all.
  - `TestHighlightRunes`: `highlightRunes("ćafé-bar", byteIdx)` marks the right runes for a query matching `é` and `b`.
  - `TestOmniResults_200ItemsFast`: 200 worktrees, 50 rounds of matching under 50ms total (a guard, not a benchmark).
- [ ] Also: `TestOmniResults_SynonymHitShowsSynonym` (query `fork`: the row's matched text is `fork`, and its highlight indexes fall inside it); `TestRegistry_LabelsAndSynonyms` covers an action with no key (`display()` is `""`).
- [ ] Implement `omni.go`: `omniSource`, `omniItem{Kind, Label, Matched, Detail, Ref}`, `omniGroup{Name string; Items []omniItem}`, `omniResults(query string, scope omniScope, src omniSource) []omniGroup`, `highlightRunes`. Candidate matching per decision 8.
- [ ] Gate. Commit `feat(tui): omni matching over worktrees, actions, PRs, and issues`.

### Task 3: The omni overlay

- [ ] Failing tests:
  - `TestOmni_SpaceOpens`: `space` pushes `ViewOmni` with the selected worktree as target; the returned command does not fetch (nil or a blink only).
  - `TestOmni_TypeFiltersAndEnterRunsAction`: type `rename`, `enter` opens the rename overlay for the target.
  - `TestOmni_EnterOnWorktreeSwitches`: type an alias, `enter` sets `switchTo` and quits.
  - `TestOmni_EnterOnPROpensBrowserAtIt`: with a cached PR `#42`, query `42`, `enter` opens the PR browser; after the browser's `prsFetchedMsg` arrives, the cursor is on #42 (a pending PR number, like `pendingSelect`). Same for an issue.
  - `TestOmni_RunsOnTargetAfterPop`: target `fix-bug`, list cursor moved elsewhere underneath, `rename` + `enter` opens rename for `fix-bug`; with the target deleted, it flashes and opens nothing.
  - `TestOmni_CtrlJKMovesAcrossGroups`, `TestOmni_EscCloses`.
  - `TestOmni_QuestionMarkScopesToActions`: `?` on the dashboard opens omni with scope actions; `?` in the PR browser still opens help.
- [ ] Implement `OmniState`, `ViewOmni` plumbing (renderFrame via `composeOverlayAt` at `formTop`, handleKey, help sections, hints, `TestHelpSectionsForAllViews`, `viewHasTextInput` true, window size), the session PR and issue caches, `omni.open` (`space`, `Omni` binding) and `app.shortcuts` (opens the old help overlay) in the action table. Omni has its own size function (`omniFrameSize`), not the branch form's.
- [ ] Gate. Commit `feat(tui): omni on space, and ? lists actions`.

### Task 4: Rendering

- [ ] Failing tests: `TestOmni_FrameSizeIsFixed` (empty, short, and long result sets at 80x24, 120x40, 60x24 render the same frame size); `TestOmni_HighlightsMatches` (matched runes styled); goldens `TestGolden_Omni` (empty, query, actions scope).
- [ ] Implement the view: input line `␣ ▏query`, rows with the group name column, cursor `❯`, a status line (`⏎ switch` / `⏎ run` / `⏎ open`), and the key line.
- [ ] Gate. Commit `feat(tui): draw omni in a fixed frame`.

### Task 5: Message history

- [ ] Failing tests: `TestToastHistory_KeepsWarningsAndErrors` (info and success are not kept), `TestToastHistory_NewestFirstCappedAt50`, `TestMessages_ActionOpensOverlay` (omni query `warnings` finds `messages`; enter pushes `ViewMessages` listing them), golden `TestGolden_Messages`.
- [ ] Implement `ToastModel.History()`; `ViewMessages` overlay (scrollable list: age via `shortAge` against `m.now`, level glyph, text) with the same plumbing checklist as `ViewOmni`; the `app.messages` action (no key; synonyms `warnings`, `errors`, `log`). The golden builds toasts with fixed `CreatedAt` values.
- [ ] Gate. Commit `feat(tui): keep warnings and errors and show them from omni`.

### Task 6: Footer and docs

- [ ] Footer per decision 6; `TestFooter_DashboardHintsFromRegistry` updated to the new literal. Expect row-count churn: the shorter footer wraps less (`TestGolden_Responsive_Layout/width_100` regains its row) and `TestGolden_Component_HelpFooter/dashboard` changes. `TestGolden_Dashboard_HelpExpanded` ports to omni's action scope.
- [ ] `docs/TUI.md` (omni, `?`, messages, footer), changelog.
- [ ] Gate. Commit.

### Task 7: Hands-on, tmux and herdr

`space` with an empty query, typing aliases and synonyms, running an action, switching, a PR from the cache, `?`, a warning toast then `messages`, 44 and 120 columns.

## Done when

- Gate, integration, and the full suite (known `TestIsCommandRunning` only) pass.
- Task 7 matches in tmux and herdr.
