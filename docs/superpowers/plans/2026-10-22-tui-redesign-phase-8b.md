# TUI Redesign Phase 8b (PR Review Frame and Files Tab) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** `⏎` on a PR opens a full-screen review frame (spec 13) whose files tab shows the PR's diff file by file, with a folder rail, viewed marks, and folding of removed runs.

**Architecture:**

- New `ViewPRReview` full-screen frame, pushed from the PR browser and from omni PR results. `PRReviewState` holds the PR, its loaded data, the active tab, the file cursor, and the rendered hunk cache.
- Opening fires three commands at once (`FetchPRDetail`, `FetchPRDiff`, `FetchReviewComments`); each result lands in its own message, so one failure never blanks the others (spec 13.5). A per-PR cache on the model, keyed on number and `UpdatedAt`, skips the fetches when the PR has not changed (spec 13.3, deferred from 8a).
- The files tab renders from `git.ParseDiff` output. Each file's lines are rendered once per (width, folded) and kept until either changes.
- Every fetch carries the review's generation number; a result whose generation is not the open review's is dropped (the `fetchDetailMetricsCmd(gen, …)` pattern).

**Spec:** 13.1-13.5, 6.4 (PR browser and PR review keys), 18 (PR review tests), 19 item 6.

## Global Constraints

As in phase 8a. Known cost for the PR body: each uncached open runs `gh auth status`, `gh repo view`, and three fetches (GraphQL has a secondary rate limit); the per-PR cache is the mitigation.

Also: No `gh` call in `Update`. Every diff line drawn must be free of raw control characters.

## Decisions

| # | Decision | Why |
|---|---|---|
| 1 | **PR browser keys:** `⏎` opens the review; `n` creates the worktree, with the old `⏎` path intact including the `ExistsPrompt` for a branch that already has a worktree. Footer, help, and `docs/TUI.md` change with it. The filter starts only on `/`, so `n` needs no guard | Spec 6.4, the one sanctioned key change. |
| 2 | **Omni PR result** pushes the review view instead of opening the browser at that PR | Spec 9.1 (`⏎` on a PR pushes the review view). |
| 3 | **Layout:** header bar `<project> ▸ #<n> <title>` (title truncated); tab row `[1] conversation n  [2] commits n  [3] checks n  [4] files n`, active tab raised and counts dim (the config tab style); body; footer | Spec 13.2 tab anatomy. Counts are `-` until their data arrives. |
| 4 | **Tabs 1-3 in this phase** show one muted line, `arrives in the next build`, and the frame opens on tab 4 | They land in 8c; grove is unreleased. 8c makes tab 1 the default. |
| 5 | **Files tab layout:** at 100+ columns a rail of 30% width (min 24) on the left, a `│` divider, the hunk pane on the right; below 100 the rail is hidden. `t` toggles it at any width | Spec 13.4. |
| 6 | **Rail:** a path tree drawn with `lipgloss/v2/tree`, folders collapsed into `a/b/` runs when they hold one child, each file `name +A -R`, folders the sum. The current file is reversed; viewed files carry `✓` | Spec 13.4 rollups; bubbles has no tree (19 item 6). |
| 7 | **Hunk pane:** a `viewport`; per hunk its `@@` header (Info), then lines `OLD NEW │ text` with right-aligned numbers, added lines Success, removed Danger, context TextNormal, no-newline markers TextMuted. A binary file shows `binary file`; a pure rename `renamed from <old>` | Line numbers come from the parser. Styling by kind instead of chroma's diff lexer: the parser already knows each line's kind, so a lexer adds nothing (deviation from 13.3, noted). |
| 8 | **Control characters** in diff text render visibly: C0 bytes as Unicode control pictures (ESC → `␛`, U+2400 + byte), DEL → `␡`, and C1 runes U+0080–U+009F (U+009B is a one-rune CSI) as `\u009b`-style text; tabs expand to 4-column stops; a trailing `\r` (CRLF files) is dropped before measuring width | 8a ledger: `gh pr diff` needs `--allow-escape-sequences`, so the text is verbatim. A PR must not be able to drive the terminal. |
| 9 | **Keys in the files tab:** `J` / `K` next / previous file (also `ctrl+n` / `ctrl+p`), `j` / `k` / `pgup` / `pgdn` / `g` / `G` scroll, `t` rail, `v` toggle viewed, `z` fold removed runs, `B` browser, `1`-`4` tabs, `esc` back | Spec 6.4. `⏎` (editor) is 8d. |
| 10 | **Fold:** `z` collapses each run of 4+ removed lines to `… N removed lines` (TextMuted); toggles for the whole PR | Spec 13.2 "fold removed runs". |
| 11 | **Viewed marks:** `UIPrefs.Viewed map[string]map[string]string`, PR number → path → first 16 hex of sha256 over, for each hunk, its `Header` then each line's kind byte and `Text` (`fileHash`). A mark whose hash differs from the current diff is not shown and is dropped at the next save | Spec 13.4: a file whose content changes loses its mark; survives a restart. |
| 12 | **Loading and errors:** before the diff arrives the pane shows `<spinner> loading diff`; a failed diff shows `diff unavailable: <err>`; `gh` missing shows one `gh required` panel for the whole frame | Spec 13.5. |
| 13 | **Per-PR cache:** `prReviewCache map[int]*prReviewData` on the model, keyed by number; reused when `UpdatedAt` from the PR list equals the cached detail's. Session only | Spec 13.3. Large diffs are not refetched on every open. |

## Carried from the 8a review

- For 8d's `g d`: run local `git diff` with `--src-prefix=a/ --dst-prefix=b/`; `diff.mnemonicPrefix` (set on the author's machine) otherwise gives `c/`/`w/` prefixes that `ParseDiff` keeps.
- `copy from/to` parses as `FileModified` with no hunks; show it like a pure rename only if a test needs it.

## Review Focus

1. A diff text containing `\x1b[2J` or `\x1b]52;` (clipboard write): Task 3 pins that the view holds no ESC byte from the diff.
2. A PR with 0 files (empty diff), and one with only binary files: Task 3 (`_Binary`) and Task 2 (an empty diff shows `no files changed`).
3. Resize while the files tab is open, across the 100-column rail threshold: Task 4 (rail hides, cache re-renders at the new width, frame keeps its height).
4. `J` past the last file and `K` before the first: Task 4 (stays put).
5. Reopening the same PR after it was updated, while the first open's fetches are still in flight: Task 2 (results from the older generation are dropped).
6. Frame size: the review frame fills the terminal exactly at 44x24, 80x24, 110x40, 120x40, and after resizing, as `checkConfigFrame` checks config: Task 4.

---

### Task 1: Frame and entry keys

**Files:** Create `internal/tui/pr_review.go`, `internal/tui/pr_review_test.go`. Modify `internal/tui/model.go` (view enum `ViewPRReview`, key routing, `renderFrame`, message cases), `internal/tui/view_prs.go` (`⏎` / `n`), `internal/tui/omni_view.go` (PR result), `internal/tui/help_v2.go`, `internal/tui/help_overlay.go`.

**Interfaces:** `type PRReviewState struct{ PR *tracker.PullRequest; Tab int /*1-4*/; Detail *tracker.PRDetail; Files []git.FileDiff; Comments []tracker.ReviewComment; DetailErr, DiffErr, CommentsErr error; Loading map[string]bool /*"detail","diff","comments"*/; FileIdx int; RailShown bool; Folded bool; ... }`; `func (m Model) openPRReview(pr *tracker.PullRequest) (tea.Model, tea.Cmd)`; messages `prDetailMsg{number int; detail *tracker.PRDetail; err error}`, `prDiffMsg{number int; files []git.FileDiff; err error}`, `prCommentsMsg{number int; comments []tracker.ReviewComment; err error}`.

- [ ] **Step 1: Failing tests:** `TestPRBrowser_EnterOpensReview`, `TestPRBrowser_NCreatesWorktree` (the old `⏎` tests, including the existing-worktree prompt, moved to `n`), `TestOmni_PRResultOpensReview`, `TestPRReview_EscPopsToBrowser` (spec 18 stack test: browser → review → back restores the browser cursor), `TestPRReview_TabKeysAndCounts` (`1`-`4` move the raised tab; counts `-` before data).
- [ ] **Step 2:** Run; fail.
- [ ] **Step 3:** Implement decisions 1-4 with stub data (no fetches yet).
- [ ] **Step 4:** Pass; commit `feat(tui): PR review frame, opened with enter on a PR`.

### Task 2: Loading the review's data

**Files:** Create `internal/tui/pr_review_data.go`, `internal/tui/pr_review_data_test.go`. Modify `internal/tui/pr_review.go`, `internal/tui/model.go` (message cases, `prReviewCache`).

- [ ] **Step 1: Failing tests:** `TestPRReview_StaleGenerationDropped` (Review Focus 5), `TestPRReview_CachedPRSkipsFetch` (same `UpdatedAt` returns no fetch commands), `TestPRReview_OneFailureKeepsOthers` (diff error, detail ok: counts filled, files tab says `diff unavailable:`), `TestPRReview_GHMissingPanel`, `TestPRReview_EmptyDiff` (`no files changed`).
- [ ] **Step 2-3:** Run, fail, implement decisions 12 and 13 with the generation counter. Fetch commands use `tracker.NewGitHubAdapter(repo)` after `IsGHInstalled` / `DetectRepo`, as `fetchPRsCmd` does.
- [ ] **Step 4:** Pass; goldens `TestGolden_PRReview/loading`; commit `feat(tui): load a PR's detail, diff, and review comments for the review`.

### Task 3: Hunk pane

**Files:** Create `internal/tui/pr_review_diff.go`, `internal/tui/pr_review_diff_test.go`.

**Interfaces:** `func renderFileDiff(f git.FileDiff, width int, folded bool) []string` (the cache key is width and folded); `func safeText(s string) string` (decision 8).

- [ ] **Step 1: Failing tests:** `TestSafeText` (ESC → `␛`, BEL → `␇`, DEL → `␡`, U+009B shown as text, tab to the next 4-stop, printable and non-ASCII unchanged); `TestRenderFileDiff_NumbersAndKinds` (gutter numbers right-aligned to the widest, `@@` header line, one line per diff line); `_Binary`, `_PureRename`, `_Fold` (a run of 5 removed lines becomes one `… 5 removed lines`, a run of 3 stays); `TestPRReview_ViewHasNoRawEscape` (a diff holding `\x1b[2J`, `\x1b]52;c;aGk=\x07`, and `\u009b2J`: the frame's view has none of `\x1b[2J`, `\x1b]52;`, or U+009B).
- [ ] **Step 2-4:** Run, implement, pass. Commit `feat(tui): render a PR file's hunks with line numbers, safely`.

### Task 4: Rail, file keys, viewed marks, fold

**Files:** Create `internal/tui/pr_review_rail.go`, `internal/tui/pr_review_rail_test.go`. Modify `internal/tui/prefs.go` (`Viewed`), `internal/tui/pr_review.go` (keys, layout).

**Interfaces:** `func renderRail(files []git.FileDiff, cur int, viewed map[string]bool, width, height int) string`; `func fileHash(f git.FileDiff) string`.

- [ ] **Step 1: Failing tests:** `TestRenderRail_TreeAndRollups` (folder sums, single-child folders joined, `✓` on viewed, current file marked); `TestPRReview_FileKeys` (`J`/`K`/`ctrl+n`/`ctrl+p`, clamped at both ends, viewport back to top on a file change); `TestPRReview_RailToggleAndWidth` (shown at 120, hidden at 80 and 44, `t` flips either); `TestPRReview_FrameFillsTerminal` (`checkConfigFrame`-style: line count equals height and no line exceeds width at 44x24, 80x24, 110x40, 120x40, and after resizing 120x40 → 80x24 → 120x40); `TestRenderRail_ScrollsToCurrent` (60 files, height 20, current file 50 on screen: the rail is a viewport around the tree); `TestPRReview_ViewedSurvivesRestart` (mark, rebuild the model from prefs, still marked); `TestPRReview_ViewedDroppedWhenFileChanges` (same path, different diff text → unmarked); `TestPRReview_FoldToggle`.
- [ ] **Step 2-4:** Run, implement decisions 5, 6, 9-11, pass. Goldens `TestGolden_PRReview/{files_rail_120x40,files_no_rail_80x24,folded}` and a themed golden for the raised tab and diff colors.
- [ ] **Step 5:** Hands-on in tmux on a real PR (`p`, `⏎`, `J`, `v`, `z`, `t`, resize, `esc`); `docs/TUI.md` (PR browser keys, new PR review section), `CHANGELOG.md` (Changed: `⏎` in the PR browser opens the review, `n` creates the worktree; Added: PR review files tab). Commit `feat(tui): file rail, viewed marks, and folding in the PR review`.
