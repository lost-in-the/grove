# TUI Redesign Phase 8d (Editor and Local Diff) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Finish spec 13: `⏎` in the files tab opens `$EDITOR` at the file and line, and `g d` (or `d` in the focused detail pane) reviews a worktree's own changes against its merge-base with the default branch, with no `gh`.

**Architecture:** The editor opens through `tea.ExecProcess(c *exec.Cmd, fn tea.ExecCallback)` (checked in bubbletea v2.0.10), which hands the terminal over and resumes the frame unchanged. A local review is a `PRReviewState` with `Local` set: no PR, no fetches but one local `git diff`, only the files tab, and a header naming the worktree.

**Spec:** 13.1 (`g d`, `d`), 13.4 (editor), 13.5 (`g d` works without `gh`), 18 (editor round trip). Carried notes from 8a: run local `git diff` with `--src-prefix=a/ --dst-prefix=b/` (`diff.mnemonicPrefix` otherwise changes the prefixes `ParseDiff` strips).

## Global Constraints

As in phase 8c. No command over 500ms in `Update`: the local diff runs in a `tea.Cmd`.

## Decisions

| # | Decision | Why |
|---|---|---|
| 1 | **Editor command:** `$VISUAL`, else `$EDITOR`, else `vi`, split with `strings.Fields` so `code -w` works. Line flag by the command's base name: `code`, `code-insiders`, `cursor`, `codium` → `-g file:N`; `subl`, `zed`, `mate` → `file:N`; `idea`, `goland`, `webstorm` → `--line N file`; anything else → `+N file`. A quoted path inside `$EDITOR` (`"/Apps/Some Editor" -w`) is split wrongly by `strings.Fields`; rare, noted in a comment, not handled | Spec 13.4: `+N file` by default, other editors their own flag. |
| 2 | **Which file:** for a local review, the worktree's path joined with the file's path. For a PR review, the worktree whose branch is the PR's head branch (`existingWorktreeItems()`, `Branch == PR.Branch`, its `Path`); with none, a toast `no worktree on <branch> · n in the PR browser creates one` and no editor | The diff's file only exists, at that content, in a checkout of the PR's branch. |
| 3 | **Which line:** the new-side number of the first line in view that has one (context or added); a view with none, line 1. A deleted file shows a toast `file was deleted in this change` and opens nothing; a binary file opens at line 1 | The files tab has no line cursor; the top of the view is what the reader is looking at. |
| 4 | **Round trip:** `tea.ExecProcess(cmd, func(err error) tea.Msg { return editorClosedMsg{err} })`; an error shows a toast `editor: <err>`; the review frame, tab, file, and scroll are unchanged after return | Spec 13.4. |
| 5 | **Local review entry:** `goto.diff` in the g strip (`g d`, label `diff against <default>`, enabled with a selected worktree), `d` while the docked detail pane is focused, and `d` in the full-screen detail sheet below 110 columns (`handleDetailSheetKey`). The dashboard's `d` still deletes when the list has focus. All open the review for the selected worktree | Spec 13.1. |
| 6 | **Local diff:** `git -C <worktree> merge-base HEAD <default>` then `git -C <worktree> diff --no-color --no-ext-diff -M --src-prefix=a/ --dst-prefix=b/ <base>` (working tree against the merge-base, so uncommitted edits to tracked files show; untracked files show once added, which the docs say), parsed by `git.ParseDiff`. `<default>` is the config's default branch; when the merge-base fails, the error shows in the files tab | Spec 13.1 "a diff of the worktree against its merge-base with the default branch". |
| 7 | **Local frame:** `PR` is a non-nil placeholder (`Number: 0`, `Title` the worktree's name) with `Local` set, so no `s.PR` read can panic. Header `<project> ▸ <worktree> vs <default>`; the tab row shows only `[4] files N`; `1`-`3`, `B`, `a`, and `enter` on a non-file tab do nothing; `v` shows a toast (marks are keyed by PR number). `cacheReviewIfComplete` and `openPRReview`'s cache lookup skip local reviews, so no diff lands under key 0 | No PR, so no conversation, commits, checks, or URL. |

## Review Focus

1. `$EDITOR` set to a command with arguments (`code -w`), and unset: Task 1.
2. A PR review with no local worktree on its branch: Task 1 (toast, no exec).
3. `d` with the list focused still deletes; with the detail pane focused it reviews: Task 2.
4. A worktree on the default branch itself, so the diff is empty: Task 2 (`no files changed`).
5. A local review's file path holding a space: the editor gets one argument for it, not two: Task 1.

---

### Task 1: Open the editor

**Files:** Create `internal/tui/pr_review_editor.go`, test. Modify `internal/tui/pr_review.go` (`enter` in the files tab), `internal/tui/model.go` (`editorClosedMsg`).

**Interfaces:** `func editorArgs(editor, file string, line int) (name string, args []string)`; `func (m Model) openInEditor() (Model, tea.Cmd)`.

- [ ] Tests: `TestEditorArgs` (one row per decision 1 family, `code -w` keeps `-w`, a path with a space stays one argument); `TestEditorLine` (decision 3); `TestPRReview_EditorDeletedFile` (toast, nil command); `TestPRReview_EnterOpensEditor` (with `VISUAL` set, the returned command is non-nil and the model is unchanged; build the `*exec.Cmd` through a seam `execEditor` the test replaces, and assert its args); `TestPRReview_EditorWithoutWorktree` (toast, nil command); `TestPRReview_EditorClosedKeepsFrame`.
- [ ] Implement decisions 1-4. Commit `feat(tui): enter in the PR review opens the file in your editor`.

### Task 2: Local review with g d and d

**Files:** Create `internal/tui/local_review.go`, test. Modify `internal/tui/actions.go` (`goto.diff`), `internal/tui/detail_v2.go` or the focused-detail key path (`d`), `internal/tui/pr_review.go` (`Local` fields, header, tab row, key guards), `docs/TUI.md`, `CHANGELOG.md`, help.

**Interfaces:** `PRReviewState.Local *localTarget` with `localTarget{Name, Path, Base string}`; `func (m Model) openLocalReview(item WorktreeItem) (tea.Model, tea.Cmd)`; `localDiffCmd(gen int, path, base string) tea.Cmd` returning `prDiffMsg`.

- [ ] Tests (integration-tagged where they need a real repo): `TestLocalDiff_AgainstMergeBase` (a fixture repo with a branch commit and an uncommitted edit: both files listed; prefixes stripped even with `diff.mnemonicPrefix=true` and `diff.noprefix=true` set in the fixture's config); `TestGStrip_DiffOpensLocalReview`; `TestDetailFocused_DOpensLocalReview` and `TestDashboard_DStillDeletes`; `TestLocalReview_OnlyFilesTab` (tab row, `1`-`3`, `B`, `v`); `TestLocalReview_EveryKeySafe` (`1`-`4`, `B`, `v`, `a`, `enter`, `J`, `t`, `z` without a panic); `TestLocalReview_NotCached`; `TestDetailSheet_DOpensLocalReview` (80x24); `TestLocalReview_EmptyDiff`; golden `TestGolden_LocalReview`.
- [ ] Implement decisions 5-7. Hands-on in tmux in the scratch clone: `g d` on a worktree with an edit, `⏎` with `EDITOR=cat`-like harmless editor (`EDITOR=true`), return. Docs (`docs/TUI.md` g strip row, detail pane keys, PR review: editor and local review), changelog. Commit `feat(tui): g d reviews a worktree's changes against the default branch`.
