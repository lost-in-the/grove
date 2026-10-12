# TUI Redesign Phase 5a (Action Registry and the `g` Strip) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** One table of dashboard actions drives key dispatch, the footer, and both strips. The `g` prefix gets its strip.

**Architecture:**

- `actions.go` holds `Action` values: key, layer (`""`, `"g"`), label, synonyms, an enabled check, and a run function. Synonyms are unused until omni (5b) but are filled in now.
- `handleDashboardKey` looks the key up in the table. An action with no run function belongs to the list (`j`, `k`, `/`, paging), so the key goes to the list.
- The footer's dashboard hints are the table's entries marked for the footer, in order.
- Strips share one renderer: a title line and wrapped `key label` entries. The `m` strip moves onto it; the `g` strip lists the table's `g`-layer actions.

**Spec:** sections 6.2, 6.3, 15.4. Phase 5b covers omni, `?` as omni, and message history.

## Global Constraints

As in phase 4b. Expected golden churn: dashboard footers (`g goto` hint), and any golden showing a strip (shared renderer). No behavior change to existing keys: the existing key tests are the regression net and must pass unchanged.

## Decisions

| # | Decision | Why |
|---|---|---|
| 1 | **The registry covers the dashboard only** in 5a | Other views keep their handlers. 5b scopes actions by view when `?` becomes omni. |
| 2 | **`g u` opens the update modal when an update is cached, else flashes `grove is up to date`** | Spec 6.3 says `g u` is always present. Bare `u` stays until Phase 7 (N9). |
| 3 | **`g a` and `g d` are not added yet** | They need agent triage (Phase 6) and the PR review view (Phase 8). |
| 4 | **Overlay-only bindings are exempt from the registry test** | `y`/`n` confirm, `space` toggle, `backspace`, `shift+tab` belong to overlays, not the dashboard. The test lists them by name. |
| 5 | **The footer gains `g goto`** | Spec 5.1 mockup. `␣ find` arrives with omni in 5b. It can re-wrap the footer at narrow widths, which changes list height; ledger any row-count churn in 60 and 44 goldens. |
| 6 | **Keys matched as raw strings in 4a/4b get `keys.go` bindings**: `g`, `G`, `ctrl+d`, `ctrl+u`, `l`, `h`, `m`, `-`, `B`, `1`-`9` | Otherwise the drift test passes whether or not they are in the table. |
| 7 | **An action has `Keys []string`**, like `key.Binding`, with `Display` for the footer | `up`/`k`, `down`/`j`, and `1`-`9` are one action each. `esc` is one action whose `Run` clears an applied filter or quits. |
| 8 | **Disabled actions are consumed**; only actions with no `Run` route to the list | Today `b` on the main worktree is swallowed. Falling through would hand it to the list. |
| 9 | **`?` and `u` have registry entries but keep their dispatch in `handleKey`** | They run in every view, before the dashboard handler. The entries feed the footer and the drift test. |
| 10 | **The `g` layer lookup lives with the chord routing at the top of `handleKey`** | Since the 4b review fix, a pending `g` or `m` owns the next key there, before help and update. |

## Review Focus

1. Every existing dashboard key does exactly what it did. Test: the existing suite, unchanged.
2. The registry and the key map cannot drift. Test: `TestRegistry_EveryKeyMapBindingHasAction`.
3. `gg` stays instant and `g` + unmapped key does nothing, with the strip now drawn. Test: Task 4.
4. Strip height: the list shrinks while a strip is open and recovers after. Test: Task 4.

---

### Task 1: Workspace and baseline

Workspace `2026-10-11-tui-redesign-phase-5a`. Gate green.

### Task 2: The action table

- [ ] Failing tests (`actions_test.go`):
  - `TestRegistry_EveryKeyMapBindingHasAction`: reflect over `DefaultKeyMap()`; every key of every binding, except the decision 4 exemptions, appears in some registry action's `Keys`.
  - `TestRegistry_EveryActionKeyHasBinding`: the reverse; every registry key appears in some `keys.go` binding.
  - `TestRegistry_NoDuplicateKeysPerLayer`.
  - `TestRegistry_EveryActionHasLabelAndSynonymsWhereRelevant`: labels non-empty; copy, switch branch, sync, delete carry the spec 9.4 synonyms.
- [ ] Add `keys.go` bindings for the raw-string keys (decision 6).
- [ ] Implement `actions.go`: `type Action struct { ID string; Keys []string; Display, Layer, Label string; Synonyms []string; Footer int; Enabled func(Model) bool; Run func(Model) (tea.Model, tea.Cmd) }` and `dashboardActions() []Action`, one entry per dashboard action, `Run` wrapping today's handlers, including their `helpFooter.SetHighlight` calls. `Display` is the footer key text (`↑↓`, `enter`). `Footer` is the footer order (0 = not shown).
- [ ] Gate. Commit `feat(tui): one table of dashboard actions`.

### Task 3: Dispatch and footer from the table

- [ ] Failing tests:
  - `TestDispatch_DisabledActionIsConsumed`: `b` on the main worktree opens nothing and the key does not reach the list.
  - `TestFooter_DashboardHintsFromRegistry`: `CompactHints(ViewDashboard)` equals the footer-marked actions in order and includes `g goto`.
- [ ] Implement: `handleDashboardKey` replaces its switch with a table lookup; list-owned actions route to the list. `CompactHints(ViewDashboard)` returns `dashboardFooterHints()`.
- [ ] Existing key tests pass unchanged. Footer goldens churn (`g goto`).
- [ ] Gate. Commit `refactor(tui): dispatch dashboard keys and footer hints from the action table`.

### Task 4: Shared strips and the `g` strip

- [ ] Failing tests:
  - `TestGStrip_ShowsActions`: after `g`, the view shows ` g goto` and `g top of list`, `p pull requests`, `i issues`, `c config`, `u upgrade grove`, `esc close`.
  - `TestGStrip_RunsAliases`: `g p` opens the PR browser, `g c` the config; `g i` the issues browser.
  - `TestGStrip_GGStillTop` and `TestGStrip_UnmappedCloses` (`g x`: strip gone, nothing else).
  - `TestGStrip_ListHeightRecovers`.
  - `TestGStrip_UpgradeWithoutUpdateFlashes`.
  - `TestStrip_FitsWidth` for both strips at 44 columns.
  - Goldens `TestGolden_Dashboard_GStrip`; the `m` strip golden may churn from the shared renderer (ledger it).
- [ ] Implement `strip.go`: `stripLines(title string, entries []stripEntry, width int) []string`; the `m` strip and the `g` strip build entries and call it. `stripHeight` covers whichever strip is open.
- [ ] Gate. Commit `feat(tui): the g strip, drawn from the action table`.

### Task 5: Docs

`docs/TUI.md` (g strip, registry in the agent notes and file map), help overlay entry for `g`, changelog. Gate. Commit.

### Task 6: Hands-on, tmux and herdr

Every dashboard key once (smoke), `g` strip and each alias, `gg`, `g x`, `m` strip, 44 columns with each strip.

## Done when

- Gate, integration, and full suite (known `TestIsCommandRunning` only) pass.
- `handleDashboardKey` has no per-key switch (`?` and `u` stay in `handleKey`, decision 9).
- Task 6 matches in tmux and herdr.
