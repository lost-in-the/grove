# Grove TUI Redesign — Acceptance Criteria & Implementation Plan

**Status:** Planning (no code written yet — this document is the plan to review before build)
**Source design:** `Grove TUI Delivery Doc.dc.html` (decisions D1–D15) + `Grove TUI UX Research Review.dc.html` (R1–R9), from the Claude Design handoff bundle
**Design origin tickets:** #12, #13, #14, #18, #19 · `docs/WIZARD_UX_RESEARCH.md`
**Codebase investigated:** `internal/tui/` (model.go 71 KB), `plugins/tracker/`, `internal/theme/`, `internal/state/` at `0.10.0-dev`
**Toolchain:** verified `go build ./internal/tui/` is green at baseline before any change.

---

## 0 · How to read this document

The delivery doc is a *visual* spec — it says what the finished TUI should look like and do. This document is the *engineering* bridge: it maps every locked decision onto grove's actual code, states testable acceptance criteria, sequences the work into independently-shippable phases, and surfaces the gaps where the design under-specifies something the code will force us to decide.

The single most important finding up front: **much of this is already built.** The delivery doc reads as greenfield, but grove already has origin-aware wizard cancel, shift+tab back, in-use branch badges, config dirty-tracking, a gated upgrade modal, and detail-pane focus with a Primary border tint. The genuinely new/deep work is narrower than the doc implies. Section 2 is the reconciliation; read it first.

Sections:
- **§1** Tooling upgrade plan (surgical, released-only)
- **§2** Current-state reconciliation — what's built vs. partial vs. greenfield, per decision
- **§3** Acceptance criteria — testable, per decision, with verification method
- **§4** Phased implementation plan — mapped to files, each phase ships alone
- **§5** Visual verification & testing strategy — the state × size grid
- **§6** Decisions & open items — recommendations locked; one question with the design agent
- **§7** Appendices — colour-token map, key-rebind map, delta reconciliation, doc gaps

### 0.1 · Required inputs for a context-free agent

**This plan is the engineering execution guide, not the whole spec.** A fresh agent must load these companion inputs — all now co-located in this `docs/redesign/` directory so the branch is a self-contained handoff:

1. **`Grove-TUI-Delivery-Doc.html`** — the **normative visual spec**. This plan deliberately does *not* reproduce the terminal mockups (Fig 1, Fig 2, Fig A1–A4, Fig B1–B3); they are the pixel/glyph/layout source of truth. Read its source directly (don't render it — everything is in the HTML/CSS, per the handoff README). Every "→ [golden …]" criterion in §3 is validated against these figures.
2. **`Grove-TUI-UX-Research-Review.html`** — rationale for R1–R9 (goto layer, `-` last-worktree, filterable help, h/l aliases, naming pass). Explains *why* the keymap looks the way it does.
3. **`Grove-TUI-Wireframes.html`** — the exploration record (frame ids 1a…7a referenced throughout the delivery doc). Reference, not normative.
4. **The grove repo checkout itself** — every `file:line` reference in this plan resolves against `internal/tui/`, `plugins/tracker/`, `internal/theme/`, `internal/state/`. A fresh agent should re-read the cited functions before editing them; line numbers drift.
5. **`docs/WIZARD_UX_RESEARCH.md`** (already in-repo) — the D7 wizard decisions derive from it.

### 0.2 · Self-sufficiency verdict (honest)

**Sufficient to execute, once paired with input #1 and #4 above.** This document alone gives an agent the current-state map, the delta per decision, testable acceptance criteria, a file-mapped phase plan, the tooling decisions, and the test strategy — enough to *sequence and land* the work without re-deriving any of the investigation. What it intentionally does **not** carry, and where an agent must go to the companions: the exact visual mockups (delivery doc), and live line numbers / full data-struct shapes (the repo). It is **not** a stand-alone replacement for the delivery doc — treat the two as a pair. With both loaded, a context-free agent has everything needed to build phase by phase.

---

## 1 · Tooling upgrade plan (surgical, released-only)

grove is **not** on stale libraries — it already pins the newest `charm.land/*` v2 stack. "Get tooling up to date" here means a handful of patch bumps plus *exploiting a v2 API that's already available but unused*, not a migration.

### 1.1 Recommended bumps

| Library | Current | Target (released) | Bump | Why |
|---|---|---|---|---|
| `charm.land/lipgloss/v2` | v2.0.1 | **v2.0.5** (Jun 2026) | **Yes** | Accumulated `Compositor`/`Canvas` fixes — this is the D2/#19 engine |
| `charm.land/bubbletea/v2` | v2.0.2 | **v2.0.8** (Jul 2026) | Yes | 6 patch releases of runtime/input fixes, no API break |
| `charm.land/bubbles/v2` | v2.0.0 | **v2.1.1** (Jul 2026) | Yes | `textarea` DynamicHeight/Min/Max — useful for the custom config editor |
| `charm.land/huh/v2` | v2.0.3 | — | **No** | Already latest; and we're moving *off* huh for config (see §2 D8) |
| `x/exp/teatest/v2`, `x/exp/golden` | pseudo (Mar 2026) | pseudo (Jul 2026) | Optional | Ships pseudo-version only; refresh for fixes if convenient |
| `glamour`, `chroma/v2` | v0.10.0 / v2.14.0 | hold | No | Diff/`diff`-lexer highlighting already works at current pins |

Bump lipgloss, bubbletea, and bubbles **together** (shared `ultraviolet` render core). No prereleases anywhere.

### 1.2 The headline finding — compositor for #19 (D2)

lipgloss v2 ships a **cell-based `Compositor` / `Layer` / `Canvas`** API. It is already importable at the *current* v2.0.1 pin (`go doc charm.land/lipgloss/v2 Compositor` resolves today); the bump to v2.0.5 is only for fixes.

```go
bg    := lipgloss.NewLayer(dimStyle.Render(backgroundView)).X(0).Y(0).Z(0)
modal := lipgloss.NewLayer(modalBox).X(mx).Y(my).Z(1)
out   := lipgloss.NewCompositor(bg, modal).Render()
```

It is transparency-aware: a layer only occupies its own content rectangle at its `X/Y` offset — cells outside stay untouched. That is exactly the "dim the background, punch the modal, never reflow" behaviour #19 needs, and it **replaces the manual line-splicing** in `centerOverlay` that *is* bug #19.

One nuance to bake into the plan: there is **no alpha/opacity blend** — we dim the background ourselves (render the base layer's content through a faint/muted style) and place the modal as a higher-`Z` layer on top. The compositor owns position + z-order + transparency; we own the dim.

### 1.3 Where current tooling already suffices (no bump)

- **Which-key strip:** no charm library ships one. The `bubbles/v2` `help` + `key` packages are the intended primitive; render a custom docked strip on top. (D11)
- **Diff highlighting:** `chroma` includes a `diff` lexer at the pinned v2.14.0. glamour has no diff helper; fence + highlight. (D10)
- **Folder tree:** no released tree component in any charm lib (`filepicker` exists but isn't a tree). Build the rail on `viewport` + an indentation model. (D10)
- **Config form:** `huh/v2` cannot do "jump to any field, edit-in-place, per-field dirty tracking" — it's a sequential group→field flow. Build the editor custom on `bubbles/v2` `textinput`/`textarea`. (D8)

### 1.4 Accepted caveats

- `teatest`/`golden`/`ultraviolet` ship **pseudo-versions only** — "released-only" cannot mean a semver tag here; the current pins are already the correct form.
- grove carries **two lipglosses**: `charm.land/lipgloss/v2` (direct) and `github.com/charmbracelet/lipgloss` v1 (indirect, pulled by glamour). glamour v1.0.0 **still** requires lipgloss v1, so no glamour bump removes the island. Treat it as an isolated dependency that only styles rendered markdown; do not attempt to de-dupe as part of this work.

---

## 2 · Current-state reconciliation (build vs. partial vs. greenfield)

This is the correction to the delivery doc's implied scope. Effort is relative within this project.

| # | Decision | Status today | Effort | Primary files |
|---|---|---|---|---|
| D1 | View stack | **Greenfield** — flat `ActiveView` enum, ~12 hardcoded `esc→Dashboard`, no cursor/scroll snapshot | **XL** | `model.go` |
| D2 | Single compositor, uniform dim, no reflow (#19) | **Greenfield mechanism** but *now easy* via lipgloss `Compositor` | **M** | `model.go` `centerOverlay`/`overlayOnDashboard`/`compositeActiveOverlay` |
| D3 | One-line row spec | **Inverse today** — default V2 delegate is two-line, branch-primary/dir-secondary | **L** | `list_v2.go`, `model.go` layout math |
| D4 | `list_secondary` + `⎇` chip | **Greenfield** — no config, no chip, no prefix-equality rule; dir not in detail | **M** | `config.go`, `list_v2.go`, `detail_v2.go` |
| D5 | Vim-only nav, remove gutter/1-9, goto layer, `-`/`h`/`l`, header n/total | **Partial** — j/k exist; gutter+quick-switch exist (to remove); no goto/`-`/`h`/`l`; header shows count not position | **L** | `list_v2.go`, `list.go`, `model.go`, `keys.go`, `header.go` |
| D6 | Middle-truncation + shed order | **Greenfield** — only tail `truncate`, no floors, no shed cascade | **M** | `helpers.go`, `list_v2.go` |
| D7 | Wizard keys/origin-return | **≈ Done** — origin-return, shift+tab back, in-use badges, live path all present | **S** (reconcile only) | `overlay_create_v2.go`, `model.go` |
| D8 | Full-screen config, vim edit-in-place, `● n unsaved` | **Large delta** — centered huh form, boolean-only dirty, no `[1-4]` tabs, huh-native nav | **L** | `overlay_config.go`, `config_form.go` |
| D9 | Header upgrade badge + confirm | **Partial** — `u` already gated + informational; badge is in footer, not header; not in help | **S** | `header.go`, `help_v2.go`, `model.go` |
| D10 | PR review view (GitHub-parity tabs + diff) | **Greenfield feature** — gh plumbing + viewport reusable; diff parser, tree rail, folding, viewed-persistence new | **XL** | `plugins/tracker/github.go`, `view_prs*.go`, `messages.go`, `commands.go`, new `internal/git` diff, `prefs.go` |
| D11 | Prefix layers + which-key strip | **Greenfield** — zero prefix infrastructure; all keys flat | **L** | `keys.go`, `model.go` update loop, `help_v2.go` |
| D12 | Type-to-filter help + synonyms + highlight | **Greenfield** — help is a static viewport; `help_highlight_test.go` is a *footer key-flash*, unrelated | **M** | `help_overlay.go` |
| D13 | Naming/rebind (`b s`, `b p`, `w d`) | **Depends on D11** — real `b` collision (currently switch-branch); `w`/`g` free | **S** | `keys.go`, `handle*Key` dispatchers, help |
| D14 | Focus tint + cursor dim + width-gate | **≈ 60% built** — focus, Tab, Primary border, j/k scroll exist; missing width-gate, cursor-dim, `h`/`l`, named focus style | **S–M** | `model.go`, `theme_v2.go`, `list_v2.go` |
| D15 | Below-44 floor guard | **Greenfield** but trivial — state survives resize for free | **S** | `model.go` `viewContent`/`updateLayout` |

**Sequencing consequence:** the doc's build order is broadly right, but D7 is a reconciliation not a build, D2 is now cheap-and-first (unblocks clean overlays for everything after), and D10 is a program unto itself. §4 reflects this.

### 2.1 Load-bearing mechanics to preserve

- **The model is passed by value** through Bubble Tea's `Update`. The view stack, pending-prefix state, and last-worktree pointer all live on the struct and copy every update — keep them cheap (slices/ints, not deep clones).
- There are **three** compositing paths today (`overlayOnDashboard`, `compositeActiveOverlay` for help/update, `compositeToastOnHeader`) plus the PR/Issue panels' own detail rendering. D2 must unify them or #19 survives in whichever path is missed.
- Golden tests and `integration_test.go`/`model_test.go` assert on `m.activeView` and exact frames. D1/D2/D3/D5 force broad golden regeneration — that's expected churn, not a regression, but it must be reviewed deliberately (see §5).

---

## 3 · Acceptance criteria

Format: **AC-x.y** — *criterion* → **[verify: method]**. Verification methods are defined in §5. These supersede and expand the delivery doc's §9 list, adding the gap items the doc flagged but never specced.

### D1 — View stack
- **AC-1.1** `esc` from any view or overlay returns to the exact origin with the list cursor **and** scroll offset restored. → [teatest transition + unit assert on stack depth]
- **AC-1.2** Navigation nests correctly: PR list → PR review → `$EDITOR` and back pops one level per `esc`, never jumping to the dashboard. → [teatest]
- **AC-1.3** The list is the primary content at every width ≥44 cols; no single-column stacked mode exists. → [golden grid §5]
- **AC-1.4** At ≥110 cols the detail pane is docked; below 110 the pane collapses to a 1-line selection strip and `tab` pushes a full-screen detail sheet onto the stack. → [golden @110, @80]

### D2 — Compositor
- **AC-2.1** Opening any overlay (help, wizard, confirm, upgrade) dims the background uniformly and does **not** reflow or clip it — the config-screenshot bug #19 is gone: a 10-worktree dashboard still shows 10 dimmed rows behind a help overlay. → [themed golden of composited frame + teatest "base text still present"]
- **AC-2.2** Exactly one overlay z-layer is active at a time; `esc` pops exactly one level. → [teatest]
- **AC-2.3** The which-key strip is **not** rendered through the compositor — it is docked chrome (see D11). → [golden]

### D3 — Row spec
- **AC-3.1** Each worktree is one row: `[status dot] [cursor] name … right-aligned 11-col counts (↑n ↓nnn ~nn) · 3-col age (no "ago") · glyph tail`. → [golden @80]
- **AC-3.2** The status dot (`●`/`○`) and the cursor (`❯`) are distinct columns — selecting a row shows both, not one replacing the other. → [golden: selected row]
- **AC-3.3** The glyph tail (`⬢⬡◆`) never sheds at any width. → [golden grid]
- **AC-3.4** No row wraps at 80×24; the count block is right-aligned within a fixed 11-col field. → [golden @80]

### D4 — Secondary column
- **AC-4.1** Config key `list_secondary` accepts `branch | dir | off`, default `branch`. → [config unit test]
- **AC-4.2** With `branch`, the dim `⎇` chip renders **only when the branch differs from the name**; prefix-duplicates (`agent-…` name vs `main-agent-…` branch derived by last-segment) count as equal and show no chip. → [unit test on the predicate + golden]
- **AC-4.3** The worktree directory always appears in detail → Git, regardless of `list_secondary`. → [golden of detail pane]

### D5 — Navigation
- **AC-5.1** The number gutter is gone from both delegates and the `1`–`9` quick-switch handler is removed. → [golden + unit: digit key is a no-op on the list]
- **AC-5.2** `j/k` move, `ctrl+d/u` half-page; `G` jumps to bottom; bare `g` opens the goto layer and `g g` jumps to top (muscle-memory `gg` survives). → [teatest]
- **AC-5.3** `h`/`l` close/open the detail; `-` toggles to the last-active worktree and a second `-` returns; its target row carries a dim `↩`. → [teatest + golden of the marker]
- **AC-5.4** The header shows cursor position `n/total`, tracking the filtered set. → [golden with filter active]
- **AC-5.5** Arrow keys remain silent aliases for j/k. → [teatest]

### D6 — Truncation
- **AC-6.1** Names middle-truncate, always preserving the ticket-id prefix and the last segment (`gal-1349-migrate-…-read`, `mj-1475-…-shared-grid`). → [unit test on the helper]
- **AC-6.2** Columns shed right-to-left as width narrows: `⎇` chip → age → `~n` → `↑↓` pair → name middle-truncates to a 16-col floor; the chip sheds entirely before the name drops below 24 cols. → [golden grid @120/110/80/60/44]

### D7 — Wizard (reconcile)
- **AC-7.1** `⏎` advances, `shift+tab` steps back on every step, `esc` cancels from any step and returns to the origin view with cursor restored. → [teatest] *(already passing — add coverage)*
- **AC-7.2** In-use branches show a `[worktree-name]` badge in the picker. → [golden] *(already built)*
- **AC-7.3** The Name step shows the derived path live with a validity check. → [golden] *(already built)*
- **AC-7.4** `shift+tab` on the Name step steps back to the branch step (not cancels) even when opened from a PR/issue. → [teatest] *(fixes the current quirk)*

### D8 — Config
- **AC-8.1** Config is full-screen (replaces the view), not a centered modal. → [golden @80/110/120]
- **AC-8.2** `[1]`–`[4]` jump directly to General/Behavior/Plugins/Protection; `j/k` move between fields; `⏎` edits in place; `esc` prompts save/discard only when dirty. → [teatest]
- **AC-8.3** Each editable field shows a description line and, when changed, a `●` marker; the header shows `● n unsaved`. → [golden of a dirty state]
- **AC-8.4** Panel height is stable across sections. → [golden compare across tabs]

### D9 — Upgrade
- **AC-9.1** When an update is cached, the header shows a `⭡x.y.z` badge; `u` opens a confirm overlay and never fires blind; the binding is listed in `?` help. → [golden of header + help]

### D10 — PR review view
- **AC-10.1** `⏎` on a PR row and `g d` on a worktree push a review view onto the stack; `n` on a PR row opens the New Worktree wizard for that PR. → [teatest]
- **AC-10.2** Tabs `[1] conversation · [2] commits · [3] checks · [4] files` render with the `[key] label (count)` anatomy — key outside the raised block, count dim. → [golden per tab]
- **AC-10.3** Files tab: folder-tree rail with +/- rollups (`t` toggles, auto-hidden <100 cols) + hunk pane; `v` marks viewed (dim `✓`, **persisted per PR across restarts**); `⏎` opens `$EDITOR` at file:line and returns the view unchanged; `z` folds runs of removed lines. → [golden + teatest for editor round-trip + restart-persistence unit test]
- **AC-10.4** Commits tab: `⏎` scopes the files tab to one commit (header shows sha); `a` resets to all. → [teatest]
- **AC-10.5** Conversation tab: read-only timeline; `⏎` on a thread jumps to its file:line in the files tab; `B` opens the browser. → [teatest]
- **AC-10.6** When `gh` is missing/unauthenticated, the view degrades gracefully with a clear message; a per-tab load failure doesn't blank the whole view. → [teatest with stubbed gh]

### D11 — Layers / which-key
- **AC-11.1** Pressing a prefix (`b`/`w`/`g`) opens a docked which-key strip above the footer; the list viewport shrinks and the selection stays visible; the strip is never an overlay. → [golden]
- **AC-11.2** Input is never gated: `b s` executes instantly whether or not the strip has rendered. → [teatest: send `b` then `s` in one batch, assert action fired]
- **AC-11.3** A prefix followed by an unmapped key dismisses the layer without stranding the user. → [teatest]

### D12 — Help filter
- **AC-12.1** `?` opens help; typing filters across key, label, and description in real time; matched substrings are highlighted. → [teatest + golden of a filtered state]
- **AC-12.2** Entries carry plain-language synonyms so retrieval works by any remembered word ("tracking remote" finds `set upstream`; "copy" finds `fork`). → [unit test on the matcher]

### D13 — Naming / rebind
- **AC-13.1** `b` opens the branch layer (`s` switch branch here · `n` new from HEAD · `u` set upstream · `p` pull from worktree); `w` the worktree layer (`R` rename · `f` fork (copy) · `d` delete multiple); `g` the goto layer. → [teatest + help golden]
- **AC-13.2** No single-key action is silently lost in the migration — every former binding is reachable under its new layer or global key. → [help golden diff review]

### D14 — Focus
- **AC-14.1** At ≥110 cols, `tab`/`l` focuses the detail pane: its border tints Primary, the list cursor dims, `j/k` scroll the pane, `h`/`esc` return with the list cursor unchanged; the footer swaps to pane-scoped keys. → [themed golden + teatest]
- **AC-14.2** Below 110 cols, `tab` pushes the detail sheet instead of focusing in place. → [teatest @80]

### D15 — Floor guard
- **AC-15.1** Below 44 cols only a one-line guard renders ("grove needs ≥44 columns"); it never crashes or clips. → [golden @40×24]
- **AC-15.2** Widening restores the exact prior state (cursor, scroll, open overlay). → [teatest resize]

### Gap items (design-flagged, previously unspecced — see §6)
- **AC-G.1** Empty states render intentionally: 0 worktrees, no open PRs, no issues each show a labelled empty panel, not a blank frame. → [golden]
- **AC-G.2** Loading and `gh`-failure states for PR/issue/review loads show a spinner then a specific error, and never block quit. → [teatest]

### Non-regression (project-wide)
- **AC-N.1** `make lint test` passes; every command still completes <500 ms (CLAUDE.md budget). → [CI]
- **AC-N.2** `NO_COLOR`, `GROVE_NO_COLOR`, `GROVE_HIGH_CONTRAST`, and light-mode paths still render (structural goldens are NO_COLOR). → [existing Tier-1 goldens]

---

## 4 · Phased implementation plan

Each phase is independently shippable, lands its own golden/teatest coverage, and passes `make lint test`. Ordering front-loads the unblockers (compositor, then stack) and isolates the PR review program.

### Phase 0 — Tooling bumps *(S)*
Bump lipgloss→2.0.5, bubbletea→2.0.8, bubbles→2.1.1 together; `go mod tidy`; regenerate any goldens that shift from renderer fixes. Gate: full suite green. No behaviour change.

### Phase 1 — Compositor unification / #19 *(M · unblocks all overlays)*
Rewrite `centerOverlay` (`model.go:2296-2343`) on lipgloss `Compositor`/`Layer`: render the background once, dim it through a single muted style, place the overlay as a higher-`Z` layer at its centered offset. Fold `overlayOnDashboard` (`:2289`), `compositeActiveOverlay` (`:1971`), and `compositeToastOnHeader` into this one path so there is a single z-layer. Stop calling `renderDashboard()` inside the overlay path per frame — freeze the last dashboard frame as the base layer.
Delivers: D2. Tests: themed golden of a composited frame per overlay type; teatest asserting dimmed base still present. **High-value, visible bug fix, shippable alone.**

### Phase 2 — View stack *(XL · the deep refactor)*
Introduce a `viewStack []viewFrame` where a frame captures `{view, listIndex, listScroll, overlay state ptr}`. Replace the flat `activeView` field and the ~12 `m.activeView = ViewDashboard` sites with `push`/`pop`. Map the existing `CreateState.ReturnView` one-level semantics onto the stack. Make `esc` a generic pop; special-case dashboard-root esc (filter-clear → quit) as today. Snapshot/restore `m.list.Index()` + viewport offset on push/pop (there is no restore home today — this creates it).
Delivers: D1, AC-1.x. Tests: teatest push/pop per view; a `stackDepth()` getter for unit invariants; regenerate `activeView`-asserting tests to the stack API.
Risk: highest-touch. Land behind the compositor so overlays are already clean. Keep the `helpOverlay`/`updateOverlay` bool-flag system folded into the stack in the same phase (don't leave a second nav system).

### Phase 3 — Dashboard: row spec, responsive, nav, focus, guard *(L)*
- **D3/D6:** rewrite `WorktreeDelegateV2.Render` to one line (`Height 2→1`), NAME-primary, fixed 11-col right-aligned counts + 3-col age + glyph tail; split the status-dot and cursor columns; new `truncateMiddleKeepEnds` helper; a single ordered width-budget/shed pass replacing the per-column caps in `ComputeDelegateWidthsV2`. Add a `compactCommitAge` that drops "ago" and floors to 3 cols. Remove the two-line `renderDelegateLine2` and the `v` compact toggle.
- **D5:** delete the `numPrefix` blocks and `handleQuickSwitch` (`model.go:1197`); wire header `n/total` from `m.list.Index()`; add `-` (last-worktree) state + dim `↩` marker; add `h`/`l`. (The `g` goto layer lands in Phase 4 with the layer engine.)
- **D4:** add `list_secondary` to `TUIConfig` + merge/default; the prefix-equality predicate (reuse the `last_segment` derivation for consistency); render the `⎇` chip; add a Directory row to `renderGitSection`.
- **D14:** gate `tab`/`l` focus on `isWideLayout` (≥110); add `DetailBorderFocus` to `StyleSet`; wire `detailFocused` into the delegate so the cursor uses the (currently empty) `ListCursorDim`.
- **D15:** width guard at the top of `viewContent` + an early-return in `updateLayout` below 44 cols.
- **Responsive:** move the docked-pane breakpoint 100→110; rework `renderStackedBody` into strip + tab-sheet (the sheet is a stack push, so it depends on Phase 2).
Delivers: D3, D4, D5 (minus goto), D6, D14, D15. Tests: the golden size grid (§5) — this is where 44×24 and 110×40 presets are added.

### Phase 4 — Command layers, help filter, rebinds *(L)*
- **D11:** add `pendingPrefix` model state + a docked which-key strip that reserves height (reuse the `HelpFooter`/`CompactHeight` height-reserving pattern). Resolve the next key: match → execute; unmapped → dismiss; **never** enter a key-swallowing modal (that's what "input never gated" requires in a one-key-per-update loop).
- **D13:** rebind under layers — `b`(branch: `s/n/u/p`), `w`(worktree: `R/f/d`), `g`(goto: `g/p/i/c/d`). Resolve the real `b` collision (currently switch-branch). `s`(sync)→`b p`, `a`(bulk-delete)→`w d`.
- **D5 goto:** `g g`/`G` top/bottom via the goto layer.
- **D12:** add a query input + `synonyms []string` to `helpEntry`; filter+highlight across key/label/description/synonyms; bypass the per-view render cache when a query is present.
Delivers: D11, D12, D13, D5-goto. Tests: teatest chord timing (`b s` in one batch); help-filter golden + matcher unit tests.

### Phase 5 — Config, upgrade, wizard reconcile *(L)*
- **D8:** replace the huh form with a custom full-screen editor built on `bubbles/v2` `textinput`/`textarea`, reviving the dormant `ConfigField`/`EditBuffer` scaffolding already in `overlay_config.go`. `[1-4]` tab jumps, `j/k` field nav, `⏎` edit-in-place, description lines, `●` per-field + `● n unsaved` header. Keep the existing dirty→save/discard-on-esc prompt.
- **D9:** add a version field to `Header`; move/restyle the badge footer→header as `⭡x.y.z`; add `u` to the help sections.
- **D7:** reconcile the 6-internal-steps vs 3-dot-labels display; fix the shift+tab-from-Name quirk (AC-7.4). Mostly test-backfill.
Delivers: D7, D8, D9. Tests: config golden across tabs + dirty state; header/help goldens.

### Phase 6 — PR review view *(XL · sub-phased, the big feature)*
1. **Data layer:** extend `GitHubAdapter` with `FetchPRReview(n)` (`gh pr view n --json comments,reviews,reviewThreads,statusCheckRollup,commits,files`) and `FetchPRDiff(n)` (`gh pr diff n`), through the existing `runGH` choke point; new `{data,err}` messages following the `fetchPRsCmd` idiom with the `gen`-counter staleness guard. Add a longer `cmdexec` timeout profile (current GHCLI is 15s — too short for large diffs). Per-PR cache keyed on `UpdatedAt`.
2. **Diff model + parser:** a unified-diff parser (promote `go-udiff`, already in the module graph, or hand-roll `@@` parsing) into files→hunks→lines; live in `internal/git` (currently branch-only). Parse once into an immutable model; render each hunk to a cached string slice (per `renderDetailViewportCard`'s identity-keyed cache) so large diffs don't re-render per frame.
3. **Files tab:** folder-tree rail (path-trie + rollups) on `viewport`; `t` toggle + auto-hide <100 cols; hunk pane with `z` fold; `chroma` `diff`/language highlighting.
4. **Viewed persistence:** a `.grove/pr_review.json` (or a field on `UIPrefs` in `prefs.go`) mapping PR number → viewed paths (optionally + content hash so a changed file un-marks). Follows the lightweight prefs mechanism, not the versioned state schema.
5. **Commits / checks / conversation tabs:** commit scoping (`⏎`/`a`); check rollup; read-only timeline via `renderMarkdown`; thread→file:line anchoring (key threads by `(path, line)`; on jump locate file node then binary-search hunks by new-line range; fall back to the thread's embedded `diffHunk` when the anchor is stale).
6. **`$EDITOR` round-trip:** `tea.ExecProcess` to hand off the TTY and resume; handle editor line-flag portability (`+N file` vs `--line N`).
7. **Worktree `g d`:** local `git diff` against merge-base with the default branch, reusing the same diff component (no gh needed for this path).
Delivers: D10, AC-10.x, AC-G.2. Tests: golden per tab (mock PR data), teatest for editor round-trip and tab nav, restart-persistence unit test, stubbed-gh failure test.

### Stretch (explicitly puntable, per the doc)
Comment authoring/approve in the conversation tab; worktree-diff base-cycling (2a); `:` command palette.

### Cross-phase: empty/loading/error states (AC-G.1/2)
Fold intentional empty/loading/error panels into each phase that introduces a data surface (dashboard empty in P3, PR/issue/review in P6) rather than as a separate pass — the design left these to "follow convention," so we set the convention: labelled panel + spinner + specific error, quit never blocked.

---

## 5 · Visual verification & testing strategy

grove's harness gives three tools with a clear division of labour. The redesign is heavily visual, so this is first-class, not an afterthought.

- **Golden (Tier-1 NO_COLOR)** — `x/exp/golden` `RequireEqual` on `m.viewString()`, states built by mutating model fields via `testOpt` builders. **Default for every static state × size.** CI-enforced (`ci.yml` `go test` job fails on drift).
- **Golden (Tier-2 themed)** — `goldenModelThemed`, reserved for **colour-dependent** states: the dimmed overlay (#19), focus ring, active-tab highlight, which-key emphasis, dirty `●`. (Fix the noted drift: themed tests should call `RequireEqualEscape`.)
- **teatest/v2** — live key-driven, for **state-machine transitions** (stack push/pop, wizard advance, chord `b s`, filter, resize). Assert on text unique to each state — teatest streams incremental writes, so there's **no full-frame snapshot**.
- **VHS** — demo GIFs only; refresh `tapes/` after the redesign lands; never gate CI on it.

### 5.1 Foundational harness additions (do first, in Phase 3)
- Add presets `sizeMinimum{44,24}` and `sizeWideMin{110,40}` to the `termSize` block in `golden_test.go`; add width `110` to the `TestGolden_Responsive_Layout` sweep (currently jumps 100→120).
- Add a `sizeTooSmall{40,24}` (or 20×5) preset for the floor guard.
- Add state builders that don't exist: `withFilter(query)`, `withWhichKey(prefix)`, `withFocus(pane)`, and a resize helper; parameterise `withConfigOverlay(tab, fullscreen)`, `withPRData(tab)`, and extend `withCreateStep` for any new steps.

### 5.2 State × size verification grid

| State | Tool | Sizes | Notes |
|---|---|---|---|
| Dashboard row spec / shed order | Golden T1 | 44·60·80·110·120 | Core grid; proves one-row-per-worktree + shed cascade (AC-3, AC-6) |
| Dimmed overlay (#19) | Golden T2 + teatest | 80 | Colour effect; assert base still present behind |
| View-stack esc/push | teatest | 120 | + `stackDepth()` unit invariant |
| Which-key strip | Golden T1 | 80·60 | Component-level like `TestGolden_Component_HelpFooter` |
| Focused detail pane | Golden T1+T2 | 110·120 | Ring is colour → T2; cursor-dim |
| Help filter | teatest + Golden T1 | 80 | Type `/up`, assert filtered + highlighted |
| Full-screen config | Golden T1 | 80·110·120 | All 4 tabs + a dirty state; height stability |
| Wizard (per step) | Golden T1 + teatest | 80 | Extend `withCreateStep`; teatest walkthrough |
| PR review (per tab) | Golden T1+T2 | 100·120 | Active-tab highlight → T2; mock PR data |
| Floor guard | Golden T1 | 40×24 (+44 boundary) | Deterministic (no spinner) |
| Empty / loading / error | Golden T1 + teatest | 80 | 0 worktrees, no PRs, gh-missing |

### 5.3 Notes
- Golden churn from D1/D2/D3/D5 is **expected**; regenerate with `make test-update-golden` and review the diff deliberately (`make golden-diff`) — don't rubber-stamp.
- Determinism rules already documented in `docs/VISUAL_TESTING.md`: never tick the spinner; use far-future durations. Keep them.
- Update `docs/VISUAL_TESTING.md` alongside: new size presets, the preset-location note (they're in `golden_test.go`, not `golden_helpers_test.go`), and the `RequireEqualEscape` drift.
- Consider a CI failure-artifact upload of rendered output to speed golden-diff review (none today) — nice-to-have, not required.

---

## 6 · Decisions & open items

**As of this revision, every item below is LOCKED to its recommended answer and build may rely on it — EXCEPT Q5, which is open with the design agent** (`docs/redesign/QUESTIONS_FOR_DESIGN.md`). Q8 is a delivery-logistics note, not a build decision. If the design agent overrides Q5, only the upgrade-key binding in Appendix B changes.

1. **[LOCKED] Sort "age" vs current "recent".** The doc's §3 sort cycle is `name → age → dirty`, but grove currently cycles `name → recent(last-accessed) → dirty`, and `CommitAge` is a pre-formatted string, not a timestamp. **Recommend:** interpret "age" as last-commit age (matches the visible age column), replacing `recent`; it needs a raw commit timestamp added to `WorktreeItem` (today only the formatted string exists). If last-accessed sorting is still wanted, keep it as a 4th mode. *(Small data-layer add.)*

2. **[LOCKED] `g d` worktree-diff base.** `g d` opens the review view scoped to a worktree (not a PR), which needs a *local* git diff, not `gh`. Against what base? **Recommend:** merge-base with the configured default branch ("what this branch actually changes") — matches the wireframe's `b`-cycle default. Base-cycling (HEAD / merge-base / cross-worktree) is deferred to stretch, as the doc already puts it.

3. **[LOCKED] `list_secondary` — runtime toggle or config-only?** The freed `v` key previously toggled row density. **Recommend:** config-only (`branch|dir|off`), no runtime key — the doc frames it as a config and reuses `v` for PR "viewed." If a runtime cycle is wanted later, add it under the `g`/view layer, not a hot key.

4. **[LOCKED] Prefix-equality rule for the `⎇` chip.** "Prefix-duplicates count as equal" needs a precise definition. **Recommend:** reuse the existing `WorktreeNameFromBranch: last_segment` derivation — chip shows only when `name != lastSegment(branch)`. One rule, already in the config vocabulary, consistent with how names are derived.

5. **[OPEN — WITH DESIGN AGENT] Upgrade `u` vs `U` adjacency (R8).** `U` currently = switch-and-force-containers; `u` = upgrade. Keeping both risks a fat-finger force-start. **Recommend:** move upgrade off `u` into the `g` goto layer or a `?`-surfaced action, freeing the top-level letter; keep `U` as-is. See `docs/redesign/QUESTIONS_FOR_DESIGN.md` — sent to the design agent; this is the only item not locked.

6. **[LOCKED] `docs/grove-tui-context.md` is missing.** The delivery doc cites it as the colour/token source, but it doesn't exist in the repo. **Recommend:** don't block — grove's theme is already Catppuccin Mocha and the doc's hex map 1:1 onto `ColorScheme` (Appendix A). Optionally author the context doc from Appendix A during Phase 1 so future handoffs resolve.

7. **[LOCKED] Which-key strip: instant vs ~300 ms reveal.** The doc locked "zero debounce, strip renders immediately." **Recommend:** honour it — instant reveal, input never gated (`b s` fires regardless). No timeout auto-cancel; the layer clears on the next key (mapped→execute, unmapped→dismiss). This is the only model that satisfies "instant" in Bubble Tea's one-key-per-update loop.

8. **[NOTE — LOGISTICS] Delivery push access.** This environment is **read-only** on `lost-in-the/grove` (push → 403). **Recommend:** I've committed this plan to a local `design/tui-redesign-plan` branch and will hand you the branch + a patch/bundle; you (or a session with write scope) push it. Flagged so "into grove on a branch" isn't silently blocked.

9. **[LOCKED] PR review offline/degraded UX.** `gh` is a hard dependency (no API fallback). **Recommend:** reuse the existing `IsGHInstalled` gate; per-tab failure isolation (checks can fail while diff succeeds); a single "gh required" panel when absent. Specced as AC-10.6/AC-G.2 rather than left to convention.

---

## 7 · Appendices

### Appendix A — Colour-token map (delivery doc hex → grove `ColorScheme`)

The delivery doc's hardcoded hex are Catppuccin Mocha, which **is** grove's `DefaultColorScheme` (`internal/theme/colors.go`). Implement by **semantic token**, never by hardcoded hex — this preserves NO_COLOR / high-contrast / light-mode adaptation for free.

| Doc hex | Role in doc | grove token |
|---|---|---|
| `#1E1E2E` | panel background | `SurfaceBg` |
| `#181825` | header bar | `HeaderBg` |
| `#313244` | selected row | `SelectionBg` |
| `#45475A` | rules/borders | `SurfaceBorder` |
| `#CDD6F4` | body text | `SurfaceFg` / `TextNormal` |
| `#FFFFFF` | bright/name | `TextBright` |
| `#9399B2` | secondary text | `TextMuted` |
| `#7F849C` | dim/age/dir | `TextDim` |
| `#A78BFA` | cursor `❯`, key hints, focus border | `Primary` |
| `#38BDF8` / `#7AA2F7` | `↑` ahead | `Secondary` / `Info` |
| `#34D399` | `✓`, `◆`, current `●` | `Success` |
| `#FBBF24` | dirty `●`, `~n`, upgrade badge | `Warning` |
| `#F87171` | `↓` behind, deletions | `Danger` |

The `⎇`, `⬢⬡◆◇`, `↑↓`, `↩` glyphs are East-Asian-ambiguous width — **measure every alignment with `lipgloss.Width`, never `len`** (already the project rule in `docs/TUI.md`), and add a width unit test since the fixed 11/3-col fields depend on it.

### Appendix B — Key rebind map (current flat → target layered)

| Current | Action | Target |
|---|---|---|
| `b` | switch branch | `b s` (branch layer) |
| `s` | sync from worktree | `b p` (pull from worktree) |
| `n` | new worktree | `n` (global) + `b n` (new from HEAD) |
| `f` | fork | `w f` (fork (copy)) |
| `R` | rename | `w R` |
| `a` | bulk delete | `w d` (delete multiple) |
| `p` | PRs | `g p` |
| `i` | issues | `g i` |
| `c` | config | `g c` |
| `1-9` | quick-switch | **removed** |
| `v` | compact toggle | **removed** (freed for PR "viewed") |
| `g`/`G` | (detail-focus only) | goto layer: `g g` top / `G` bottom |
| — | last worktree | `-` |
| — | close/open detail | `h` / `l` |
| `u` | upgrade | see Q5 (recommend relocate) |

`w` and `g` are unbound today — free to claim. `b` is the only real collision.

### Appendix C — Delta-table reconciliation

The delivery doc's §11 delta table is accurate against the shipped TUI with two clarifications from the investigation: (a) the current docked-pane breakpoint is **>100**, not the ">100 side-by-side / stacked below" the table implies is a clean tier — the new tiers are 44-guard / <110 strip+sheet / ≥110 docked; (b) "wizard: enter/esc only → shift+tab back" is **already done** in the current code, so that row is a no-op/backfill, not a change.

### Appendix D — Documentation to update alongside code

Per CLAUDE.md ("check if changes require updating docs"): `docs/TUI.md` (layout tiers, keymap, sort modes, config nav), `docs/VISUAL_TESTING.md` (size presets + drift notes), `docs/CONFIGURATION_REFERENCE.md` (`list_secondary`), and optionally author `docs/grove-tui-context.md` (Q6). Update the CHANGELOG per the conventional-commit workflow.

---

*Prepared from the Claude Design handoff bundle against `lost-in-the/grove` @ `0.10.0-dev`. All file:line references verified against the cloned source; `go build ./internal/tui/` green at baseline.*
