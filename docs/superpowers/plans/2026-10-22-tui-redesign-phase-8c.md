# TUI Redesign Phase 8c (PR Review: Conversation, Commits, Checks) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Fill the review frame's tabs 1-3 (spec 13.2): a conversation timeline that jumps to a thread's line, a commit list that scopes the files tab to one commit, and a check rollup. The frame opens on tab 1.

**Architecture:** Each tab is a cursor list over items built once when its data lands, rendered into the body with the files tab's scroll rules. Markdown bodies go through `renderMarkdown` after `safeMarkdown` strips control characters. Scoping to a commit fetches that commit's diff over REST on demand and swaps the files tab's files until `a`.

**Spec:** 13.1-13.5, 6.4 (PR review keys), 18. Builds on 8a (`PRDetail`, `ReviewComment`) and 8b (frame, files tab, `safeText`, generation, session cache).

## Global Constraints

As in phase 8b. PR-controlled text never reaches the terminal raw: plain text through `safeText`, markdown through `safeMarkdown` then `renderMarkdown`.

## Decisions

| # | Decision | Why |
|---|---|---|
| 1 | **Default tab is 1** (conversation) | Spec 13.2 order; 8b opened on 4 only while 1-3 were placeholders. |
| 2 | **Conversation items**, oldest first: PR comment (`@author · age` then the body as markdown), review (`@author approved / requested changes / commented · age` then its body when non-empty), thread (`path:line` then each comment `@author: body`, replies under their root by `InReplyTo`). An outdated thread (`Line` 0) shows `path (outdated, was line N)` with N its `OriginalLine`. Markdown bodies render at the body width minus 4 and are indented 4, so code blocks stay inside the frame | Spec 13.2 timeline. |
| 3 | **Check states in the conversation:** one summary line at the top, `checks: 7 passed · 1 failed · 1 running`, Success/Danger/Warning counts. One helper `checkBucket(state string)`: passed = `SUCCESS`, `NEUTRAL`, `SKIPPED`; failed = `FAILURE`, `ERROR`, `TIMED_OUT`, `ACTION_REQUIRED`, `CANCELLED`, `STARTUP_FAILURE`; everything else running | Spec lists check states in tab 1; the full list is tab 3. 8a's `PRCheck.State` is already one upper-case word. |
| 4 | **In tabs 1-3, `j`/`k` move a cursor between items** (the item is reversed in its gutter); in tab 4 they still scroll, and the footer hints follow the tab. `⏎` on a thread switches to tab 4 at that thread's file with the line scrolled into view; on other items it does nothing | Spec 13.2 `⏎` on a thread. Anchor: `Path` + `Line`, matched against `NewNo` when `Side` is `RIGHT` and `OldNo` when `LEFT`. When `Line` is 0 (outdated) or not in the current diff, the first line of the thread's `DiffHunk` (its `@@` header; GitHub adds context lines after it) is matched by prefix against `Hunk.Header` (spec 13.4 fallback); with no match the file opens at its top. |
| 5 | **Commits tab:** one row per commit, `sha headline`, oldest first as gh returns them (checked: chronological on #186's 19 commits before relying on it); `⏎` scopes tab 4 to that commit (header line reads `files · <sha>` and `a all commits` joins the footer); `a` returns to the PR's diff | Spec 13.2. |
| 6 | **Commit diff:** `tracker.FetchCommitDiff(sha)` via `gh api repos/{repo}/commits/{sha}` with the diff media type and `--allow-escape-sequences`, through `apiArgs`; parsed with `git.ParseDiff`. State: `ScopedSHA string` and `ScopedFiles []git.FileDiff` on `PRReviewState`, cleared by `a` and by `setFiles`; `prCommitDiffMsg` carries the generation and is dropped when stale; the session cache keys commit diffs by `(number, sha)`. A failure shows `diff unavailable:` like the PR diff. **`v` while scoped is refused** with a toast `viewed marks apply to the whole PR`, so no commit-scoped hash reaches `ui_prefs.json` | REST, as 8b moved the PR diff (GraphQL rate limits). |
| 7 | **Checks tab:** rows sorted failing first, then running, then passing; glyph `✗` (Danger), `●` (Warning) for pending states, `✓` (Success); name; state in TextMuted. Two runs with the same name both show (a push and a pull_request run of one workflow, seen on #186) | Distinct runs; hiding one could hide a failure. |
| 8 | **`safeMarkdown`** (added in 8b's review fixes, with the PR browser's detail body) is used for every markdown body here | — |
| 9 | **Counts** stay as 8b defined them | — |

## Review Focus

1. A comment body holding `\x1b]52;c;…\a` or `\x1b[2J`: Task 1 pins `safeMarkdown`, Task 2 the conversation view, Task 1 the PR browser detail.
2. A thread on a line the current diff no longer has: Task 2 (falls back to the hunk header; with no match, the file at its top).
3. Scoping to a commit, then `J`/`K` and `v`: Task 3 (`v` is refused while scoped and `ui_prefs.json` is unchanged).
4. A PR with no comments, no reviews, no checks: each tab shows a labelled empty line (spec 14).
5. `a` when not scoped does nothing; `⏎` in the commits tab while a commit diff is loading shows `<spinner> loading <sha>`.

---

### Task 1: Checks tab and the default tab

**Files:** Create `internal/tui/pr_review_checks.go`, test. Modify `internal/tui/pr_review.go` (tab 3 body, default tab).

- [ ] Expected churn: `TestPRReview_TabKeysAndCounts` asserts the frame opens on tab 4; it now opens on 1. `TestGolden_PRReview/loading` re-renders on tab 1.
- [ ] Tests: `TestCheckBucket` (every state in decision 3's sets); `TestRenderChecks_OrderAndGlyphs` (fail, pending, pass order; duplicate names both shown); `TestPRReview_ChecksEmpty` (`no checks`); `TestPRReview_OpensOnConversation`.
- [ ] Implement decisions 1, 3 (helper), 7. Commit `feat(tui): checks tab, and the review opens on the conversation`.

### Task 2: Conversation tab and jumping to a thread

**Files:** Create `internal/tui/pr_review_conversation.go`, test. Modify `internal/tui/pr_review.go` (tab 1 body and keys).

**Interfaces:** `type convItem struct{ kind int; at time.Time; lines []string; path string; line int; hunk string }`; `func buildConversation(d *tracker.PRDetail, cs []tracker.ReviewComment, width int) []convItem`; `func (s *PRReviewState) jumpToThread(it convItem)`.

- [ ] Tests: `TestBuildConversation_OrderAndThreads` (time order; replies under their root; outdated label); `TestPRReview_ConversationNoRawEscape`; `TestPRReview_EnterOnThreadJumps` (tab 4, the thread's file, its line visible; one `RIGHT` thread and one `LEFT` thread on a removed line); `TestPRReview_OutdatedThreadFallsBackToHunk`; `TestPRReview_ConversationEmpty` (`no comments yet`); golden `TestGolden_PRReview/conversation_120x40`.
- [ ] Implement decisions 2-4. Commit `feat(tui): conversation tab, with enter jumping to a thread's line`.

### Task 3: Commits tab and scoping files to a commit

**Files:** Modify `plugins/tracker/github_detail.go` (`FetchCommitDiff`, `commitDiffArgs`), `internal/tui/pr_review_data.go` (`prCommitDiffMsg{gen int; sha string; files []git.FileDiff; err error}`), `internal/tui/pr_review.go`. Create `internal/tui/pr_review_commits.go`, tests.

- [ ] Tests: `TestCommitDiffArgs`; `TestPRReview_EnterScopesToCommit` (tab 4, header `files · abc1234`, `a` restores the PR diff); `TestPRReview_ViewedRefusedWhileScoped`; `TestPRReview_StaleCommitDiffDropped`; `TestPRReview_CommitDiffCachedBySha` (second scope fetches nothing); `TestPRReview_CommitScopeLoadingAndError`; golden `TestGolden_PRReview/commits_80x24`.
- [ ] Implement decisions 5, 6. Hands-on in tmux on a real PR (all four tabs, `⏎` on a commit, `a`). Docs (`docs/TUI.md` PR review section: tabs 1-3, `a`), changelog (Added entry: the three tabs). Commit `feat(tui): commits tab, scoping the files tab to one commit`.
