# TUI Redesign Phase 8a (PR Review Data and Diff Model) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** The data layer for the PR review view (spec 13.3): a parsed diff model in `internal/git`, PR detail, review comments, and diff fetches in the tracker, and a PR browser that opens on the last cached list at once.

**Architecture:**

- `internal/git/diff.go` parses unified diff text once into files, hunks, and numbered lines. No rendering here; phase 8b renders and caches hunks.
- `plugins/tracker` gains `FetchPRDetail`, `FetchReviewComments`, and `FetchPRDiff`, each a thin `gh` call over a pure parse function that tests feed fixture JSON.
- The PR browser opens on the last list saved in the user cache directory, marked stale, and replaces it when the refresh lands.
- Deferred to 8b, where its first consumer lands: the per-PR detail cache keyed on `UpdatedAt` (spec 13.3).

**Spec:** section 13.3 (data), 13.5 (degraded states), 18 (PR review tests), 19 item 6.

## Verified before planning

- **Spec 19 item 6:** `charm.land/bubbles/v2@v2.1.1` has no `tree` package, so 8b keeps a custom rail. `charm.land/lipgloss/v2/tree` exists and can draw it.
- **`gh pr view --json reviewThreads` does not exist** in gh 2.103 ("Unknown JSON field"). Inline review threads come from REST: `gh api repos/{repo}/pulls/{n}/comments --paginate`, which returns `id`, `in_reply_to_id`, `path`, `line`, `start_line`, `original_line`, `side`, `diff_hunk`, `commit_id`, `user.login`, `body`, `created_at`. Without `--slurp`, `--paginate` prints one JSON array per page back to back, so the call uses `--paginate --slurp` and flattens. This corrects spec 13.3.
- `github.com/alecthomas/chroma/v2` (with a `diff` lexer) is already in `go.mod` as indirect; 8b may use it without a new module.

## Global Constraints

As in phase 7b; the gate includes the integration suite. Every command under 500ms (constraint in `CLAUDE.md`): nothing here may run `gh` synchronously in `Update` or at startup. No golden churn expected except the PR browser's stale line.

## Decisions

| # | Decision | Why |
|---|---|---|
| 1 | **Diff model types** (`internal/git`): `FileDiff{OldPath, NewPath string; Status FileStatus; Binary bool; Hunks []Hunk; Added, Removed int}`, `FileStatus` = `FileModified`, `FileAdded`, `FileDeleted`, `FileRenamed`; `Hunk{Header string; OldStart, OldLines, NewStart, NewLines int; Lines []DiffLine}`; `DiffLine{Kind LineKind; Text string; OldNo, NewNo int}` with `LineContext`, `LineAdded`, `LineRemoved`, `LineNoNewline`. `ParseDiff(diff string) ([]FileDiff, error)` | One parse, reused by every tab and by `g d` (local `git diff`) in 8d. Line numbers live on the line so thread anchors (8c) and `$EDITOR +N` (8d) need no recount. |
| 2 | **A hunk ends by its counts, not by prefixes.** The parser reads exactly `OldLines` old-side and `NewLines` new-side lines after a header | A removed line whose text is `-- x` reads as `--- x`, and an added `++ y` as `+++ y`; sniffing for file headers inside a hunk corrupts both. |
| 3 | **Path of a file** is `NewPath`, or `OldPath` when deleted. Quoted git paths (`"a/sp ace\t.go"`) are unquoted with `strconv.Unquote`; unquoted UTF-8 paths pass through | Local `git diff` (8d) quotes paths with control or non-ASCII bytes; `gh pr diff` from GitHub does not. |
| 4 | **Malformed input returns an error** naming the line number; an empty diff returns `nil, nil` | The view shows "diff unavailable" for one PR rather than a wrong diff. |
| 5 | **Tracker types:** `PRDetail{Number int; UpdatedAt time.Time; HeadSHA string; Comments []PRComment; Reviews []PRReview; Checks []PRCheck; Commits []PRCommit; Files []PRFile}`, `PRComment{Author, Body string; CreatedAt time.Time}`, `PRReview{Author, State, Body string; SubmittedAt time.Time}`, `PRCheck{Name, State, URL string}`, `PRFile{Path string; Additions, Deletions int}`, `ReviewComment{ID, InReplyTo int64; Path string; Line, StartLine, OriginalLine int; Side, DiffHunk, CommitID, Author, Body string; CreatedAt time.Time}` | Plain values the TUI can cache and compare. `PRCommit` already exists. |
| 6 | **`PRCheck.State`** is one word: a CheckRun's `conclusion` when `status` is `COMPLETED`, else its `status`; a StatusContext's `state`. Name is `name` or `context`; URL is `detailsUrl` or `targetUrl`. All upper case as gh returns them | `statusCheckRollup` mixes two GraphQL types; the checks tab needs one shape. |
| 7 | **Timeouts:** new `cmdexec.GHCLISlow = 60 * time.Second` for `FetchPRDiff` and `FetchReviewComments` (paginated); `FetchPRDetail` keeps `GHCLI` | Spec: "a longer timeout profile than the current 15s for large diffs". `runGH` gains a timeout parameter through a small `runGHTimeout`. |
| 8 | **PR list cache file:** `<os.UserCacheDir()>/grove/prs/<basename(projectRoot)>-<first 12 hex of sha256(projectRoot)>.json`, `{"saved_at": RFC3339, "prs": [...tracker.PullRequest]}`, written atomically (`fsutil.AtomicWriteFile`) by `fetchPRsCmd` after a successful list, never in `Update` | Spec: `p` shows the last cached list at once. Not `.grove/`: excludes are only refreshed by init, context, and bootstrap, so a new file there would make existing main worktrees read dirty. A corrupt or missing file, or no cache dir, is ignored. |
| 9 | **Stale display:** while a cached list is shown and the refresh runs, the list's first line is `<spinner> cached · refreshing` (TextMuted). A failed refresh keeps the list and shows `refresh failed: <err>` (ErrorText) on that line. No cache: today's `Loading PRs...` | The list never blanks for a slow or failed network call (spec 13.5: a failure in one part does not blank the others). |
| 10 | **The cache also seeds `m.prCache`** (used by omni) at TUI start, read off the main goroutine with the startup commands | Omni's PR results work before the browser is first opened, still without calling `gh` (spec 18: omni never calls `gh`). |

## Review Focus

1. A diff with a removed line `-- x` or an added `++ y` inside a hunk (decision 2): pinned in Task 1.
2. Rename plus edit, and a pure rename with no hunks (`similarity index 100%`): Task 1.
3. `\ No newline at end of file` after either side, which counts toward neither: Task 1.
4. A PR cache file written by a newer grove with unknown fields, or truncated mid-write: Task 3 (unknown fields load; truncated file is ignored).
5. A refresh that fails while the stale list is shown, then a second `p` press: Task 3 (list kept, error line, no double fetch while one is in flight).

---

### Task 1: Diff model

**Files:** Create `internal/git/diff.go`, `internal/git/diff_test.go`.

**Interfaces:** Produces decision 1's types and `ParseDiff`.

- [ ] **Step 1: Failing tests** (table, fixtures inline as raw strings): `TestParseDiff_ModifiedTwoHunks` (old/new numbers on every line of both hunks; `Added`/`Removed` totals), `_AddedFile` (`/dev/null` old side, `FileAdded`, `OldNo` 0), `_DeletedFile`, `_RenameWithEdit` and `_PureRename` (`FileRenamed`, `OldPath`/`NewPath`, no hunks for the pure one), `_Binary` (`Binary files a/x.png and b/x.png differ` → `Binary: true`, no hunks), `_NoNewlineMarker` (a `LineNoNewline` line, numbers unchanged), `_DashPrefixedContent` (decision 2: lines `--- x` and `+++ y` inside a hunk are a removed and an added line), `_QuotedPath` and `_RawUTF8Path`, `_Empty` (`nil, nil`), `_MalformedHunkHeader` (error mentions the line number).
- [ ] **Step 2:** `mise exec -- go test ./internal/git/ -run TestParseDiff -v` fails (undefined).
- [ ] **Step 3:** Implement `ParseDiff`. A file starts at `diff --git`; headers until the first `@@` or the next `diff --git`; hunk header regexp `^@@ -(\d+)(?:,(\d+))? \+(\d+)(?:,(\d+))? @@(.*)$` (an omitted count is 1).
- [ ] **Step 4:** Tests pass; commit `feat(git): parse unified diffs into files, hunks, and numbered lines`.

### Task 2: PR detail, review comments, and diff in the tracker

**Files:** Modify `plugins/tracker/github.go`, `plugins/tracker/tracker.go` (types), `internal/cmdexec/cmdexec.go` (`GHCLISlow`). Create `plugins/tracker/github_detail_test.go`, fixtures in `plugins/tracker/testdata/pr_detail.json` and `review_comments.json`.

**Interfaces:** Produces decision 5's types; `(*GitHubAdapter).FetchPRDetail(number int) (*PRDetail, error)`, `FetchReviewComments(number int) ([]ReviewComment, error)`, `FetchPRDiff(number int) (string, error)`; pure `parsePRDetail(data []byte) (*PRDetail, error)` and `parseReviewComments(data []byte) ([]ReviewComment, error)`.

- [ ] **Step 1: Capture fixtures** from real gh output (`gh pr view <n> --json number,updatedAt,headRefOid,comments,reviews,statusCheckRollup,commits,files`, and REST `gh api 'repos/lost-in-the/grove/pulls/comments?per_page=5'` for inline comments), trimmed to two of each and anonymized to `octocat`. The detail fixture must hold one CheckRun completed, one CheckRun in progress, and one StatusContext.
- [ ] **Step 2: Failing tests:** `TestParsePRDetail` (fields of decision 5; the three checks map per decision 6 to `SUCCESS`, `IN_PROGRESS`, the context's `state`); `TestParseReviewComments` (input is `--slurp` output, an array of two page arrays; a reply's `InReplyTo` is its parent's `ID`; `Line` 0 when GitHub sends `null` for an outdated comment, `OriginalLine` kept; `StartLine` set on a multi-line comment).
- [ ] **Step 3:** Implement. `FetchPRDetail` uses `runGH`; `FetchReviewComments` runs `gh api repos/<repo>/pulls/<n>/comments --paginate` and `FetchPRDiff` runs `gh pr diff <n>`, both with `GHCLISlow`.
- [ ] **Step 4:** Tests pass; commit `feat(tracker): fetch PR detail, review comments, and diffs`.

### Task 3: PR browser opens on the cached list

**Files:** Create `internal/tui/pr_cache.go`, `internal/tui/pr_cache_test.go`. Modify `internal/tui/model.go` (`enterPRView`, `handlePRsFetched`, startup commands), `internal/tui/view_prs.go` (`PRViewState.Stale`, `fetchPRsCmd` saves the cache), `internal/tui/view_prs_v2.go` (`renderPRList` line per decision 9), `docs/TUI.md`, `CHANGELOG.md`.

**Interfaces:** `prCachePath(projectRoot string) (string, error)`, `loadPRCache(projectRoot string) []*tracker.PullRequest` (nil on any error), `savePRCache(projectRoot string, prs []*tracker.PullRequest) error`, message `prCacheLoadedMsg{prs []*tracker.PullRequest}`. Tests set `XDG_CACHE_HOME` to a temp dir.

- [ ] **Step 1: Failing tests:** `TestPRCache_RoundTrip`; `_IgnoresCorruptFile` (truncated JSON → nil); `_KeepsUnknownFields` (extra keys load); `TestPRView_OpensOnCachedList` (with `m.prCache` set: `PRs` shown, `Loading` false, `Stale` true, a fetch command returned); `TestPRView_RefreshReplacesStaleList` (fetched → `Stale` false, file written); `TestPRView_FailedRefreshKeepsList` (error → list kept, line reads `refresh failed:`); `TestPRView_NoDoubleFetch` (`p`, `esc`, `p` while the first fetch is in flight returns no second fetch); `TestPRCachePath_PerProject` (two roots with the same basename get different files); a golden `TestGolden_PRList_Stale`.
- [ ] **Step 2:** Run; fail.
- [ ] **Step 3:** Implement decisions 8 to 10. Track the in-flight fetch on the model (`prFetching bool`), cleared in `handlePRsFetched`.
- [ ] **Step 4:** Tests pass; hands-on in tmux: open `p` twice in one session, the second opens instantly with `cached · refreshing`.
- [ ] **Step 5:** `docs/TUI.md` (PR browser: cached list), `CHANGELOG.md` (Changed: `p` opens at once on the last list). Commit `feat(tui): open the PR browser on the last cached list`.
