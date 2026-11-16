# TUI Redesign Phase 7a (Upgrade) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** The upgrade surfaces follow spec 12 (N9): a header badge `⭡x.y.z` replaces the footer badge, `g u` always opens the upgrade overlay (with the new version, or "up to date"), and bare `u` does nothing.

**Architecture:**

- `NewModel` reads the update-check cache whatever the opt-outs say, and records separately whether update notices are allowed (`updateNotify`). The badge needs both; `g u` needs only the cache.
- `Header` gains an `Upgrade` field drawn on the right before `n/total`. It never sheds.
- The footer badge (`appendUpdateBadge`, `RenderUpdateBadge`) is deleted.
- The `Update` key binding, its `handleKey` branch, and the `app.update` action are deleted. The overlay closes on `esc`.
- `UpdateOverlay` gains an up-to-date state, so `g u` never answers with a toast.

**Spec:** sections 12 and 5.5 (header item 5).

Phase 7b (config, N8) follows in its own plan and PR.

## Global Constraints

As in phase 6; the gate includes the integration suite. Expected golden churn: none on the dashboard (no golden model has an update cached); new upgrade overlay goldens.

## Decisions

| # | Decision | Why |
|---|---|---|
| 1 | **Opt-outs hide the badge only.** `NewModel` reads the cache always; `updateNotify = !updatecheck.Skip(...)` gates the badge. | Spec 12: "`g u` still opens and reports the cached state." |
| 2 | **The badge sits on the right, before `n/total`**, Warning color, and never sheds | Spec 5.5 orders it before the cursor position and keeps both at narrow widths. |
| 3 | **`g u` with nothing newer opens the overlay in an up-to-date state** (title `✓ grove is up to date`, current version, `esc close`) | Spec 12 says `g u` pushes the overlay or reports up to date; a toast here also never expired on an idle dashboard (phase 6 review). |
| 4 | **The overlay keeps all three install methods** (Brew, `go install`, binary) | The existing overlay chose this on purpose: the TUI cannot know which method installed grove as reliably as the user does, and there is room. Spec 12 says "the platform's install command"; recorded as a deviation. |
| 5 | **Bare `u` is unbound in every view**: the binding, the global branch in `handleKey`, and `app.update` go | Spec N9. `g u` and Find (`upgrade`) reach the overlay. |
| 6 | **The overlay closes on `esc` only**; its key line reads `esc close` | `u` no longer means anything. |
| 7 | **When the header still overflows with the name at its minimum, the left side truncates, not the whole line** | `ansi.Truncate` on the joined line clips from the right, so `n/total` would vanish. The badge makes this reachable: name, count, `! 2 · ✓ 1`, `⬍ attention ↑` plus `⭡0.12.0  10/10` exceed 42 inner cells. |
| 8 | **`openUpgrade` builds the overlay when the model has none** | The nil branch fell back to a toast; test models have no overlay, so it would be untested. |

## Review Focus

1. Opt-out set: no badge, `g u` still shows the cached version. Test: Task 2.
2. Bare `u` does nothing on the dashboard, in the PR and issue browsers, and in the overlay. Test: Task 3.
3. The header keeps the badge and `n/total` at 44 columns. Test: Task 2.

---

### Task 1: Workspace and baseline

Workspace `2026-10-12-tui-redesign-phase-7a`. Gate green.

### Task 2: Header badge, opt-out split, footer badge gone

- [ ] Failing tests:
  - `TestHeader_UpgradeBadge`: `⭡0.12.0` before `1/5` at 100 columns. At 44 columns, with the heaviest left side (`SortLabel: "attention"`, reversed, `Blocked: 2`, `Done: 1`, `Cursor: 10`, `Total: 10`), the badge and `10/10` both stay and the header is 44 cells.
  - Check whether the contrast test enumerates header pairs; if it does, add Warning, Danger, and Success on the header bar (the last two are phase 6's).
  - `TestDashboard_UpgradeBadgeOnlyWhenNotifying`: with a newer version cached and `updateNotify`, the header shows the badge and the footer shows no `press u`; with `updateNotify` false, neither shows.
  - Port `TestProgram_UpdateOverlay_GateRespectsSkip`: with `GROVE_NO_UPDATE_NOTIFIER=1`, `updateNotify` is false. (Cache contents on the host are not asserted.)
- [ ] Implement `Header.Upgrade`, `Model.updateNotify`; delete `appendUpdateBadge`, `RenderUpdateBadge`, and their styles if unused.
- [ ] Gate. Commit `feat(tui): upgrade badge in the header`.

### Task 3: Bare `u` unbound

- [ ] Failing tests: `TestDashboard_BareUOpensNothing` (update cached: `u` leaves the dashboard as it was, no toast); `TestUpgradeOverlay_UDoesNotClose` (`u` in the overlay leaves it open; `esc` closes).
- [ ] Port: `TestProgram_UpdateOverlay_OpensOnUWhenAvailable` reaches the overlay with `g u`; the omni test's skip list drops `app.update`.
- [ ] Delete `KeyMap.Update`, its `handleKey` branch, the `ViewUpdate` `u` close, and `app.update`. The overlay key line reads `esc close`. Update the comment above `dashboardActions` that names `u`.
- [ ] Gate. Commit `feat(tui): unbind bare u; g u opens the upgrade overlay`.

### Task 4: Up-to-date state

- [ ] Failing tests: `TestGU_UpToDateOpensOverlay` (nothing cached, no overlay on the model: `g u` pushes `ViewUpdate`, no toast); `TestGolden_Upgrade` (available, up to date) at 80x24.
- [ ] Port: `TestGStrip_UpgradeWithoutUpdateFlashes` asserts `ViewUpdate` and no toast.
- [ ] Implement the up-to-date view in `UpdateOverlay`; `openUpgrade` always pushes the overlay.
- [ ] Gate. Commit `feat(tui): g u reports up to date in the overlay`.

### Task 5: Docs

`docs/TUI.md` (update indicator section, keys table row for `u`), changelog. Grep `docs/` and `AGENTS.md` for `press u` and `` `u` `` first. Gate. Commit.

### Task 6: Hands-on

tmux scratch repo with a fake newer release in a scratch cache (`HOME` pointed at a scratch dir holding `.grove/update-check.json`): badge in the header, `u` does nothing, `g u` opens with the version, `esc` closes. With `GROVE_NO_UPDATE_NOTIFIER=1`: no badge, `g u` still shows the version. With no cache: `g u` says up to date.

## Done when

- Gate (with integration) and the full suite (known `TestIsCommandRunning` only) pass.
- Task 6 matches.
