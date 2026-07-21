# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [0.10.0] - Unreleased

> **Upgrading:** Shell integration is now **version 9** — re-source your shell (`grove setup` / `eval "$(grove install zsh)"`) or `grove doctor` will nag. Two behavior changes to know about. **First:** `.grove/config.toml` is committable now (and should be committed — it's the shared project config). Older grove git-ignored it; the first command you run in an existing repo migrates the exclude file automatically and prints a one-time notice. Worktrees no longer receive per-worktree config symlinks — config always resolves from the main worktree — and `grove doctor` lists any legacy symlinks with exact cleanup commands. **Second:** hooks.toml actions marked `required` / `on_failure = "fail"` now genuinely abort the operation (create, switch, remove — CLI and dashboard alike). Everything else is drop-in.

### Security
- Hook command injection closed across **every** sink: command hooks spliced `{{.branch}}` — attacker-influenced via `grove fetch pr/<n>` — literally into `sh -c`, and the same value reached `docker:compose` / `docker:exec` handlers inside `bash -cil`. Values now travel by environment reference (into the container as `-e KEY=VALUE`), never spliced. The reference rewriter is quote-context-aware and models nested shell contexts — command substitution, POSIX arithmetic (`$(( … ))`, which broke on dash), backticks, and heredocs — so bare, double-, and single-quoted tokens all expand correctly (`pkill -f '{{.worktree}}'` works again) and none can execute, anywhere.
- `state.json` — and every atomic file write — respects the process umask again: an explicit chmod in the write path made grove's state world-readable `0644` on hosts running `umask 077`.

### Fixed
- State no longer fragments per worktree: `.grove` (state *and* config) resolves via git's common dir from any worktree or subdirectory, so `grove new`/`rm`/`last` run inside a worktree stopped reading and writing a phantom `state.json`. Config **writes** anchor there too — `grove config set` and the TUI settings editor from a linked worktree previously materialized a private, silently diverging config copy.
- `grove rm`/`to`/`rename` can no longer hit the wrong worktree: resolution uses precedence tiers (name → directory → branch) so a branch name can't shadow the worktree you actually named; plain `grove rm` never escalates to `os.RemoveAll` (that now requires `--force`), and a git-locked worktree is never force-deleted.
- Worktrees are born clean on fresh clones too: grove's machine-local files are recorded in `$GIT_COMMON_DIR/info/exclude` during worktree bootstrap, not just `grove init`.
- `grove graft` no longer panics on short SHAs; `grove diff --stat` no longer reports zero changes; `grove fork` skips the network fetch that could silently move the forked base; `grove fetch pr/<n>`, `grove new --from-branch`, and the dashboard create-from-base flow fast-forward a stale local branch to the remote tip instead of building the worktree from a stale commit.
- Switch flow: `grove last` behaves exactly like `grove to` (dirty handling, pre/post-switch hooks, `--json` emitted before any tmux relocation); switching to the worktree you're already in is a clean no-op; **any** aborted switch — a required pre- *or* post-switch hook failure, or a tmux error — pops its auto-stash back and records `last_worktree`/`last_accessed` only once the switch actually commits, instead of swallowing your changes for a switch that never happened. Switch hooks receive the resolved worktree short name as `{{.worktree}}`, not the raw argument (which may have matched by branch).
- Hooks: documented `pre_switch`/`post_switch`/`pre_create` recipes actually run; `pre_create` receives the branch the worktree will really be created on (not the raw `--branch`, empty in the default/`--from-branch`/`--mirror` flows); `{{.worktree_full}}` is the real directory name under custom `[naming]` patterns; command actions without `working_dir` — including `[[overrides]]` `extra_run` — run from the main worktree on `pre_create`/`post_remove` (the "new" path is guaranteed absent exactly there); hook output can no longer corrupt `grove new --json` stdout or the dashboard alt screen.
- `grove rm`/`grove trim` resolve their hooks.toml from the main worktree, not the current directory, so running them from inside another worktree can't execute that worktree's local hooks.
- `grove adopt` run from a subdirectory registers the worktree root, not the subdirectory (which previously became a bogus state entry).
- `grove sync --json` emits the result document (with the skip reason) before exiting non-zero — error paths previously produced empty stdout.
- Config layering: partial `[plugins.docker.external]` and `[…external.agent]` overrides merge field-by-field instead of wiping the section, and merging never mutates the underlying config layers.
- TUI ↔ CLI parity: dashboard deletes fire the same hooks.toml pre_remove **and post_remove** actions as `grove rm` (with `{{.main_path}}` set, so a `working_dir="main"` cleanup runs in the project root rather than a root-anchored path), honor required-hook aborts, and run docker slot teardown; dashboard creates fire the `pre_create` gate before creating; dashboard **forks** run the full bootstrap (excludes, SetupFiles, plugin + config post_create) instead of registering state and tmux only; plugin/docker output is captured off the alt screen instead of corrupting it. Bulk delete marks dirty candidates and counts them before the single confirm; list refreshes keep an active filter; esc clears a filter before quitting.
- Durability and races: state writes fsync before the atomic rename; docker slot reads happen under the file lock; the worktree manager's lazy caches are race-free under `go test -race`.
- Fewer subprocess spawns on `ls`, `here`, and the update check.

### Added
- Self-healing upgrade path: every command migrates a legacy exclude file, rekeys the main worktree's state entry from `"main"` to `"root"` (so upgraded repos stop silently missing root lookups), and shows the config-layout notice **once** — now keyed on the legacy per-worktree config symlinks a real upgrader actually carries, so it reaches them rather than only mid-development builds. `grove doctor` flags those symlinks with exact removal commands; `grove init` nudges `git add .grove && git commit`.
- `grove fork` validates worktree names like every other creation path — `grove fork root` is rejected instead of colliding with the main worktree's reserved state and tmux keys.
- `AGENTS.md` gained a config-layout upgrade section, and `docs/AGENT_GUIDE.md` a full walkthrough for agents assisting the upgrade (including the verbatim notice text for stderr pattern-matching).

### Changed
- Docker auto-start after worktree creation is one explicit knob shared by every surface: `plugins.docker.auto_up = true` opts in for `grove new` **and** the dashboard create flow (which previously never auto-started, #141), and agent stacks no longer flip it on implicitly — set `auto_up = true` to keep that behavior.
- The shell wrapper's capture path explicitly clears an inherited `GROVE_CD_FILE` (`ShellVersion` 8 → 9), and the binary now requires that file to already exist before writing (the wrapper `mktemp`s it), so a stale value leaked into a tmux pane from an `issues`/`prs` session no longer resurrects a dead temp file — directory switching falls through to the stdout `cd:` line instead of silently breaking.
- `git rev-parse --git-common-dir` is memoized per process, collapsing the duplicate resolutions each command previously spawned.
- Release workflow hardened (`set -euo pipefail`, verified downloads, non-empty tarball checks) and the Homebrew formula license corrected to **Apache-2.0** (template and published formula — companion tap PR).
- The audit trail for all of the above lives in [`AUDIT.md`](AUDIT.md) (findings `B1`–`B38`, `P*`, `D*`, review items `R1`–`R24`).

## [0.9.0] - 2026-07-16

> **Upgrading:** Shell integration is now **version 7** — re-source your shell (`grove setup` / `eval "$(grove install zsh)"`) or `grove doctor` will nag. Two behavior changes: the `w` alias is now **opt-in** (run `grove setup --alias` to keep it; the `alias` config key is gone and silently ignored if present), and interactive bare `grove` outside a project now prints a diagnosis and exits 10 instead of showing help. Everything else is drop-in.

### Fixed
- Bare `grove` in a directory outside any git repository no longer false-positives on the
  global `~/.grove` dir (debug logs, update-check cache) and launches the TUI against a
  non-repo. `FindRoot` never walks outside a git work tree, and interactive bare `grove`
  outside a project now prints the same "not a grove project" diagnosis as other commands
  (exit 10) instead of a raw doubly-wrapped git error with leaked terminal escape
  sequences (#138).
- Shell integration survives rc re-sourcing: the binary resolver now uses function-immune
  lookups (`whence -p` in zsh, `type -P` in bash) so a second `eval "$(grove install …)"`
  no longer resolves the previously-defined `grove()` wrapper function and permanently
  trips the recursion guard (`ShellVersion` bumped to 7 — re-source your shell) (#137).
- zsh integration sourced before `compinit` no longer errors with `command not found:
  compdef` at shell startup — completion registration is guarded and degrades to no tab
  completion (place the eval line after `compinit` for completion to register). README no
  longer claims Homebrew ships static completions; completions come from the shell
  integration line.

### Changed
- The `w` shorthand is now **opt-in**: `grove install <shell> --alias[=name]` (bare
  `--alias` means `w`), plumbed through `grove setup --alias`. The integration no longer
  claims `w` (which shadows the standard `w(1)` command) by default, and the unused
  per-project `alias` config field was removed — it was never read by the generator and a
  project config can't drive a shell-wide alias.
- `grove setup` is self-healing: it migrates a deprecated `eval "$(grove init …)"` line or
  an out-of-date `eval "$(grove install …)"` variant in place (keeping the rest of the rc
  file untouched) instead of only warning, so rc files never need hand-editing.
- The `grove-worktree-management` Claude Code plugin now ships from the shared
  [`lost-in-the/plugins`](https://github.com/lost-in-the/plugins) marketplace suite instead of
  this repo's own root marketplace. The suite references the skill via a `git-subdir` source,
  so installs fetch only `skills/grove-worktree-management/` (~80 KB) rather than caching the
  entire repo (~4 MB). The repo-root `.claude-plugin/` is removed; the plugin manifest now
  lives at `skills/grove-worktree-management/.claude-plugin/plugin.json` (the plugin root).
  New install: `/plugin marketplace add lost-in-the/plugins` then
  `/plugin install grove-plugin@lost-in-the-plugins`.
- The skill gained a **Version Preflight** step (run `grove version` + `grove --check-update`,
  operate only against the installed version) so agents on an older grove don't invoke commands
  or flags their binary lacks, and surface available updates to the user.

## [0.8.0] - 2026-07-09

> **Upgrading:** One breaking change — standalone `docker-compose` (Compose v1) support was removed; the docker plugin now requires the `docker` CLI with Compose v2 (#107). Otherwise drop-in: shell integration is unchanged, so no re-source is needed. Highlights: grove now ships as a **Claude Code plugin**, `[naming] pattern` controls worktree directory names (#104), and `grove to/new --no-tmux` land (#106). `grove last` on a fresh project is now a graceful no-op (exit 0) instead of an error (#132).

### Fixed (2026-07 repo audit — issues #109–#123)
- TUI create wizard: the typed new-branch name and the selected fork base are now actually used — previously `git worktree add -b <worktreeName>` ran from HEAD regardless of what the confirm screen showed (#109).
- TUI dashboard: `grove prs`/`grove issues` entry points now populate worktree badges and the "worktree exists" prompt; PR/issue detail no longer shows stale content after filtering; cancelled WIP checks can't corrupt the next overlay; the config overlay's save confirmation works instead of silently discarding on esc; the rename overlay validates names like create/fork (#110).
- `grove fetch` (and the `prs`/`issues` CLI path) runs the full worktree bootstrap — config symlink, state, hooks, Docker — instead of a bare `git worktree add` (#112).
- `grove last` recreates the tmux session instead of failing when it's gone; `down`/`logs`/`restart`/`up` resolve the worktree root via git instead of the cwd basename; `grove config --global` shows global values, not mislabeled merged config; `adopt` rejects repos that aren't worktrees of the current project; `ls --json` aggregates container statuses from all providers; `new --mirror` creates the documented `env/{name}` branch; `open` no longer prints a raw `cd:` directive without shell integration; `to`'s drift-reset handles single-quoted paths (#113).
- `grove fork --json` emits clean JSON (human output moved off stdout); `grove trim` fires the same pre/post-remove hooks as `rm` (Docker teardown included); `grove init --with-*` honors the naming pattern and registers state/hooks (#114).
- `grove rm` can no longer validate one worktree and remove another: `Remove` resolves names with the same matcher as `Find`, and `[protection]` is checked against the resolved name too (#115, #116).
- tmux operations use exact-match session targets, so `grove-foo` can't kill/attach `grove-foo-bar`; `grove repair` confirms each orphan session kill individually (#116).
- Layered config no longer resets explicit settings: a project/local file that omits a key no longer overrides an earlier layer's value with the default (#117).
- Concurrent grove processes no longer resurrect removed worktrees or clobber `LastWorktree` in state.json; `FindRoot` resolves symlinked cwds (macOS `/tmp`, `/var`) so the git-root boundary holds; linked worktrees discover the main worktree's `hooks.toml` instead of silently skipping project hooks; compose detection matches all four compose filenames (#118).
- `grove browse` PR lookup works again: the tracker called `gh pr view` with a nonexistent `--head` flag and always fell back to the compare page (#119).
- Update check actually persists its cache (the fire-and-forget goroutine died with the process), and failed/timed-out checks record the attempt so offline hosts don't block 300ms on every command (#119).
- Shell integration: `grove new` output is captured by the zsh/bash wrappers so its `cd:` directive is honored (`ShellVersion` bumped to 6 — re-source your shell); stale duplicate `shell/` wrappers are no longer shipped in release archives (#121).
- Docs realigned with the code across command specs, agent guides, and configuration/plugin/data-flow references — removed nonexistent flags and hooks, corrected defaults and placeholders (#111, #122, #123).

### Performance (2026-07 repo audit)
- `grove ls` and status displays stay within the <500ms budget in external Docker mode (300ms status timeout instead of 3s probe timeout); cold worktree listing no longer runs `git worktree list --porcelain` three times; `GetStatus` no longer double-execs `git rev-parse @{upstream}`; `grove here -q`/`--check-mount` skip enrichment they never display (#120).

### Removed
- Standalone `docker-compose` (Compose v1) support. The docker plugin now requires the `docker` CLI with Compose v2. The v1 fallback only accepted a single `--env-file`, silently bypassing the `.env` layering fix from #98 (v1 has been EOL since mid-2023) (closes #107).

### Added
- Grove now ships as a **Claude Code plugin**. The repo is its own single-plugin marketplace (`.claude-plugin/plugin.json` + `marketplace.json`, `source: "./"`), distributing the existing `grove-worktree-management` skill so agents can install grove guidance with `/plugin marketplace add lost-in-the/grove` then `/plugin install grove-plugin@grove-plugins`. Skill helper scripts are referenced via `${CLAUDE_PLUGIN_ROOT}` so they resolve from the installed plugin cache. See [README → Claude Code plugin](README.md#claude-code-plugin).
- `grove to --no-tmux` and `grove new --no-tmux` — per-invocation tmux suppression (no session creation, switch, or attach) without the isolated-Docker coupling of `GROVE_AGENT_MODE`. Hooks and Docker still run (closes #106).
- `[naming] pattern` now actually controls worktree directory names. Placeholders `{project}` and `{name}` (each required exactly once, literals limited to `[A-Za-z0-9._-]`), default `{project}-{name}`. Loaded via standard config layering (global → project → `config.local.toml`); short-name display, lookup, rename, adopt, and TUI create previews all honor the pattern. Invalid patterns warn on stderr and fall back to the default. Tmux session names intentionally keep the canonical `{project}-{name}` form. Previously the key was parsed and displayed but never applied (closes #104).

### Fixed
- `grove last` no longer errors on a fresh project: with no in-project `last_worktree`, it prints a graceful hint and exits 0 instead of chasing a stale cross-project session from the global `~/.config/grove/last_session` file, and `--json` emits a valid object (`{"switch_to":"","message":…}`) on that path. Also fixed `TestGetLastSession` writing `test-last-session` into the real user's global grove state (the test now isolates `HOME`) (#132).
- Command-reference documentation realigned with the code: `grove browse` is documented as implemented (it was wrongly listed as "not implemented"); the phantom `grove run` token was removed from the shell wrappers and docs; missing shell completions (`adopt`, `agent-help`, `browse`, `context`, `rename`) and command aliases (`fork` → `fo`, `new` → `n`) were added; and the bundled skill's `pr_review.py` no longer passes a nonexistent `grove fetch --repo` flag.
- Documented per-directory trust tools (mise, direnv) in the hooks reference, including why the untrusted-config errors appear and the security tradeoff of pre-trusting fetched third-party code.
- `grove to --peek` no longer relocates the caller's tmux client. Peek now skips tmux entirely (session creation, `switch-client`, and attach) in addition to hooks, matching its documented "observational" intent (closes #105).

## [0.7.1] - 2026-05-11

> **Upgrading:** No breaking changes. New surface added for adopting existing branches into worktrees, auditing per-worktree provisioning, and detecting Docker bind-mount drift. One latent bug fix for projects using `env_file` set to anything other than `.env`.

### Added
- `grove new --from-branch <branch>` — adopts an existing local or remote branch into a new worktree without creating a new branch (calls `git worktree add <path> <branch>`). Mutually exclusive with `--from`, `--branch`, and `--mirror`. Git's "branch already checked out" guardrail still fires when the branch is in use elsewhere (closes #95).
- `grove new --dirty` — when paired with `--from-branch`, transfers `git diff HEAD` (working tree + staged tracked changes) from the current worktree into the new one via `git apply`. Untracked files are intentionally excluded. Transferred changes land as unstaged in the destination; re-stage manually if needed. A patch that fails to apply (conflict, etc.) emits a warning rather than aborting — the new worktree is intact and the source is unmodified.
- `grove doctor <worktree>` — audits one existing worktree's `copy_files` / `symlink_files` / `symlink_dirs` entries against the project config. Classifies each entry as `ok`, `missing`, or `override` (user has replaced the configured type with something else). Read-only without `--fix` (closes #96).
- `grove doctor --all` — runs the per-worktree audit against every registered worktree.
- `grove doctor <worktree> --fix` (and `--all --fix`) — restores `missing` entries by copying or symlinking from the main worktree, matching the configured action type. Override entries are left untouched so user customizations aren't silently reverted.
- `grove here --check-mount` — compares the env-configured worktree (the path written to `env_file` by grove's external Docker mode) against each running container's actual bind-mount source. Exits non-zero (code `12` / `MountDrift`) when they disagree on any service, with a clear `Restart needed — run grove up to apply` hint. Use to script pre-test guards or CI pre-checks ("did anyone forget to grove up after switching?") (closes #97).
- `[plugins.docker.external].mount_dest` config option (default `/app`) — container path that grove inspects for the bind-mount source when checking drift. Override for projects whose source lives at a different container path (e.g. `/srv/app`, `/workspace`).

### Fixed
- `[plugins.docker.external] env_file` set to a non-default file (e.g. `.env.local`) no longer shadows the project's `.env` for compose interpolation. Grove now passes both `--env-file .env --env-file <configured>` when `.env` is present alongside, matching developer intuition (defaults from `.env`, per-worktree overrides from the configured file). Previously, variables defined only in `.env` would silently become blank during interpolation (issue #98).

## [0.7.0] - 2026-05-11

> **Upgrading:** No breaking config changes — all new fields have defaults and existing configs continue to work. Docker users should run `grove doctor` to surface host install commands that should now be `docker:compose` hooks (`grove doctor --fix` rewrites them automatically). See "Behavior changes" and "Migration / consumer-side" below.

### Behavior changes (read before upgrading)

- **`grove here --json`, `grove ls --json`, and `grove ps --json` field names changed to snake_case for consistency with `grove context --json`.** Scripts parsing these outputs need to update field-name lookups: `fullName → full_name` (here, ls), `agentSlot → agent_slot`, `agentURL → agent_url`, `shortHash → short_hash` (here), `composeProject → compose_project` (ps).
- **Post-create hook ordering inverted.** Plugin Go hooks now fire BEFORE config-driven hooks in `.grove/hooks.toml` (so containers are up by the time user setup commands run). Pre-existing hook setups that rely on the old ordering may need verification.
- **`grove test` defaults to `--no-deps`.** Tests that rely on `depends_on` services starting must opt in via `[test] include_deps = true` in `.grove/config.toml` or pass `--with-deps`.
- **External compose path resolution changed.** Relative paths in `[plugins.docker.external] path` resolve against the directory containing `.grove/`, not grove's CWD.
- **`grove up`/`grove ps` honor `non_blocking_services`.** A stack where only non-blocking services have exited cleanly is now treated as `up`, not `degraded`, and `grove up` no longer exits non-zero in that case.

### Added
- `grove adopt [path]` command — bootstraps a git worktree that grove doesn't know about (config symlink, state registration, post-create hooks).
- `grove context` command — prints full worktree context (branch, commit, remote tracking/sync, status, stash count, recent commits) for CLI/scripting use; `--json` flag emits structured machine-readable output (closes #16). JSON includes `has_remote` boolean to distinguish "no remote" from "remote, in sync" (0/0 ahead/behind).
- `grove repair` lifecycle command for fixing common worktree-state issues.
- `grove doctor --fix` rewrites flagged host install commands to `docker:compose` hooks in place.
- `grove doctor` Tier-2 "Worktree registration" check — reports drifted worktrees with a `grove adopt` hint. `doctor` now lists configured non-blocking services for external compose stacks and `stat()`s every entry in `copy_files`/`symlink_files`/`symlink_dirs`, surfacing typos that previously failed silently.
- Drift detection — running any grove command from a worktree not in `state.json` prints a non-fatal warning suggesting `grove adopt`.
- New hook action types `docker:compose` and `docker:exec` for routing config-driven hooks into containers (see `docs/CONFIGURATION_REFERENCE.md`). Action type names use a `pluginname:action` namespace convention.
- Pluggable hook action handler registry — plugins can register custom action types via `hooks.RegisterActionHandler` (idempotent, last-write-wins). See `docs/PLUGIN_DEVELOPMENT.md`.
- `grove init` now picks between `auto` (preview + confirm) and `walkthrough` (step-by-step) modes when running interactively. New flags: `--auto`, `--walkthrough`, `--yes`. Non-TTY behavior preserved as silent auto.
- Docker-aware project detection: when a compose file is present alongside Rails/Node/Python markers, install commands (`bundle install`, `npm install`, `pip install`) are auto-generated as `docker:compose` hooks instead of host commands. Service name inferred from `docker-compose.yml` (single service used, or first non-infra service). Dockerfile-only projects (no compose file) keep host commands and emit a manual-setup note rather than generating broken compose hooks.
- `grove doctor` detects host install commands inside a Docker project and stray `.grove/.grove-backup/` directories.
- Per-developer config overlay at `.grove/config.local.toml` (gitignored). Overrides values from the committed `.grove/config.toml` for individual developers — e.g., `[tmux] mode = "off"` for someone who prefers no tmux without changing team defaults. Precedence: defaults → global → `.grove/config.toml` → `.grove/config.local.toml` → env vars (closes #79).
- Optional update-available notification on command exit when a newer grove release is published (closes #35). Checks at most once per 24 hours, in a detached background process — never blocks command execution. Suppressed in CI, non-TTY contexts, and when `NO_UPDATE_NOTIFIER`, `GROVE_NO_UPDATE_NOTIFIER`, `GROVE_AGENT_MODE`, or `GROVE_NONINTERACTIVE` env vars are set, or when `--no-update-notifier` is passed. Use `--check-update` to force a synchronous check at any time.
- TUI shows a passive `↑ X.Y.Z → X.Y+1.Z press u` footer badge when a newer grove release is available, plus a richer modal (opened via `u`) showing all install methods (Brew, Go install, binary) and the changelog link (issue #77, PR #82). Reuses the existing `~/.grove/update-check.json` cache. Same opt-outs as the CLI box.
- `grove version` output now appends `(update available: X.Y.Z)` when a newer release is cached. Suppressed in non-TTY contexts and when update-notifier opt-outs are set.
- `[test]` config: `include_deps` (bool, default false) and `bind_mount` (string) — control `compose run` dependency resolution and worktree bind-mount path.
- `grove test --with-deps` and `--bind` flags — per-invocation overrides of `[test]` config.
- `[plugins.docker.external]` config: `non_blocking_services` — services allowed to exit (one-shot init, etc.) without marking the stack unhealthy.
- `[plugins.docker.external] env_file` config option — grove writes the configured env-var assignment additively to the named env file (appending or updating only the configured key).
- `[plugins.docker.external.agent] template_overlays` — multi-template overlay support, replacing the single-template-file limitation.
- `COMPOSE_PROJECT_NAME` and the configured `[plugins.docker.external] env_var` are now exported into agent worktree shells, so slot-routing tooling can target the correct container without manual wiring.
- `grove trim` accepts `prune` as an alias for git-flavored discoverability (issue #10).
- `symlink_files` documented in top-level README and CONFIGURATION_REFERENCE alongside `symlink_dirs`.
- README beta notice, edge install path (`go install ...@main`), and per-install-method update guidance (issue #11).
- TUI branch selector now includes remote-only branches; selecting a remote-only branch fetches from origin automatically.

### Changed
- **Performance:** TUI dashboard loads worktrees substantially faster on large projects. `tui.FetchWorktrees` ~6.7s → ~2.4s on a 38-worktree project; `grove which`/`grove here` ~2.5s → <0.2s. Commit-count and stash-count metrics now load asynchronously after first paint, with a generation counter to drop stale results from superseded fetches (#85).
- Hook execution order on worktree create: plugin Go hooks now fire **before** config-driven `[[hooks.post_create]]` so containers are up by the time user setup commands run. This removes a workaround in the `docker:compose` handler and lets `mode = "exec"` work without a stealth `compose up`.
- `grove trim`/`grove repair` confirmation prompts now respond to Ctrl+C and ESC instead of hanging on raw `fmt.Scanln` (issue #17). `trim` keeps its literal "yes" guard and continues to support scripted `echo yes | grove trim`.
- Update notification now uses contextual labels: `Run:` for shell commands (Brew, `go install`), `Download:` for the binary URL fallback. Previously rendered `Run: Visit https://...` which read awkwardly.
- When `grove test` exits non-zero with a connection-refused or DNS error (e.g. "connection refused", "no such host") and the user has not opted into `include_deps`, grove appends a hint pointing at `--with-deps` and `[test] include_deps = true`.
- `TestConfig.IncludeDeps` is now `*bool` so a project-level `false` can override a global `true`.
- `BootstrapWorktree` extracted from `setupCreatedWorktree` so `grove new` and `grove adopt` share the same post-`git worktree add` sequence.
- `grove adopt` strips the project prefix from directory names (e.g., `myproj-feature` → `feature`) so adopted worktrees match grove's naming convention.
- Service-health probe timeout raised from 1s to 3s to tolerate slow systems.
- Compose `--env-file` is now honored when reading the active-worktree env var (previously hardcoded to `.env`).
- Worktree ages now reflect real timestamps (no more "9999 days").
- `docs/COMMAND_SPECIFICATIONS.md` clarifies `grove context` exit codes: exit 10 only when the command runs outside any grove project; exit 1 when in a grove project but the current directory is not a registered worktree.

### Fixed
- TUI update-available opt-outs now also gate the Skip flow for full parity with the CLI box (issue #84, PR #83). Previously the Skip-cache gate didn't honor every documented opt-out env var.
- Agent docker strategy now applies the same error translation as local and external strategies — dependency-failure rewrites and `--with-deps` hints now surface for users on the agent strategy (closes #72).
- `teeBuffer` (Docker compose stderr capture) now correctly caps single writes larger than the 8KB sliding window — the previous path stored the full oversized chunk before trimming, briefly exceeding the cap.
- `grove test` translates `compose run` "service didn't complete successfully" errors into actionable grove-styled messages.
- `grove up` no longer silently swallows compose-up failures when the post-up health probe returns no statuses.
- `grove up` skips the post-up health probe when compose-up succeeded (previously paid up to 1s on every successful run).
- `grove rm --force` now actually forces removal via a 3-tier fallback and succeeds on worktrees containing non-empty untracked directories (e.g. `node_modules` left by a post-create hook). When git's own `worktree remove --force` refuses, grove falls back to removing the directory itself and pruning git's metadata (issues #24, #28).
- `Manager.Remove` refuses to remove the main worktree as a defense-in-depth backstop for the new `os.RemoveAll` fallback.
- `bundle install`/`npm install` post-create hooks no longer fail on the host for Docker-based dev stacks (issue #28).
- `grove trim` no longer reports "9999 days since last access" for worktrees missing state. `grove init` now stamps `created_at`/`last_accessed_at` on the main worktree, and `trim` falls back to the worktree's HEAD commit time (or "last access unknown") when no state timestamp is available (issue #9).
- State load backfills zero-valued `created_at`/`last_accessed_at` timestamps on worktrees from earlier versions, so upgraders no longer see lingering `"0001-01-01T00:00:00Z"` in their `state.json`.
- `grove adopt` refuses to "adopt" the main worktree (it is always registered).
- `grove adopt` errors out on detached HEAD instead of storing the literal `"HEAD"` as a branch name.
- Post-create hook execution failures are now logged to grove's debug log (previously discarded silently).
- `docs/COMMAND_SPECIFICATIONS.md` `grove init` section now documents the actual command (it had been showing `grove install <shell>` content). New init flags (`--auto`, `--walkthrough`, `--yes`) and Docker-aware install routing are now discoverable from the spec, the Docker plugin README, and the agent guide.
- `docs/TUI.md` keybind reference now includes `v` (view-mode toggle) and `u` (update modal).
- `grove rm` now clears `LastWorktree` in state.json when removing the worktree it points at, so `grove last` no longer returns a removed name.
- `IsWorktreeInState` resolves symlinks on both the stored path and the caller's path, so drift detection remains correct when state.json was written before symlink-aware path normalization.

### Migration / consumer-side

Downstream consumers integrating with grove via `.grove/config.toml` or `.grove/hooks.toml` should review the following after upgrading:

1. **Audit `.grove/hooks.toml` for host install commands on Dockerized stacks.** Hooks of `type = "command"` running things like `bundle install` or `npm install` against a project that uses Docker-based development should migrate to `type = "docker:compose"`. Run `grove doctor --fix` to apply the migration automatically.
2. **Bump downstream version pins** if you have tooling that gates on grove's version (e.g. a `MIN_VERSION` check in setup scripts) — bump to `0.7.0` once this release ships.
3. **Slot-routing tooling can now consume grove-emitted env vars.** Grove now exports `COMPOSE_PROJECT_NAME` and the configured `[plugins.docker.external] env_var` into agent worktree shells. Consumer-side binstubs and dev helpers that need to target the right container in agent worktrees can read these directly instead of inferring them.
4. **Discard manual env-file copy hooks.** If a consumer was copying an env file from parent to child via a `[[hooks.post_create]]` of `type = "copy"`, the new `[plugins.docker.external] env_file` config option supersedes that. Grove writes the configured env-var assignment additively (appending or updating only the configured key), so it doesn't propagate parent-worktree values into children incorrectly.
5. **Run `grove doctor` post-upgrade.** Doctor now `stat()`s every entry in `copy_files`/`symlink_files`/`symlink_dirs`, surfacing typos that previously failed silently.
6. **Update consumer-facing docs/skills** that reference older grove behavior (no `COMPOSE_PROJECT_NAME` export, single-template-file limit, no `--fix`, no `prune` alias, no slot-aware exports). The behaviors above are all new in this release.

### Internal
- `internal/grove.IsWorktreeInState` — shared helper for state.json drift detection.
- `state.Manager.Batch()` and the new `worktreeinfo` package extracted to consolidate git fan-out (#74, #85).
- Removed unused `matchesActive` parameter from external-status classifier; removed `_ = name` dead wiring in env-loader doctor checks; removed dead `BootstrapOpts.Now` injection field.
- Test fixtures in `internal/tui/update_overlay_test.go` now reference `version.Version` instead of a hardcoded `0.7.0-dev` literal, so they don't silently break on the next dev-cycle version bump.
- TUI render hot path no longer reallocates lipgloss styles per frame in update overlay and footer badge — promoted to package-level vars.
- Atomic-write helper extracted to `internal/fsutil` and used by state migration/backup paths and project-config writes. Crashes mid-write no longer corrupt `state.json` or user `.grove/config.toml`.
- CI's release workflow now runs `make test-integration` and `make lint` before publishing, mirroring the gate fix applied to the main CI workflow in PR #83.
- CI test matrix now includes macOS in addition to Linux, surfacing platform-specific issues (especially around symlink resolution and path handling) before release. The `plugins/docker` unit tests are excluded on the macOS runner (it has no Docker daemon and many of those tests don't gate on `exec.LookPath`); Docker-aware integration tests already self-skip when Docker isn't reachable.

### Documentation
- `docs/AGENT_GUIDE.md` updated to cover `grove context`, `--check-update`/`--no-update-notifier` persistent flags and `GROVE_NO_UPDATE_NOTIFIER`, and `grove adopt` edge cases (detached HEAD, already-registered).
- `docs/PLUGIN_DEVELOPMENT.md` now documents `docker:compose` and `docker:exec` action-type handler signatures and the post-create hook ordering invariant (plugin Go hooks fire before config-driven hooks).

## [0.6.1] - 2026-03-19

### Fixed
- TUI branch selector fetches remote branches before creating a worktree from a remote-only ref, so the new worktree's checkout doesn't fail on a missing local ref.

### Internal
- Repository-identity / context7 indexing config maintenance.

## [0.6.0] - 2026-03-18

### Added
- TUI: major UX overhaul — panels, Huh forms, scrollable detail view, streaming logs.
- TUI: context-sensitive help overlay with Glamour rendering (later replaced with manual lipgloss in the same release).
- TUI: PR/issue creation flow — exists prompt, fork support, wizard routing.
- TUI: open-in-browser (`B`) for PRs and issues, including from the detail panel.
- TUI: wizard UX — return-to-source navigation, branch badges, shift+tab nav.
- TUI: branch selector includes remote branches.
- Anomic-aphasia-friendly command aliases and renames for easier word-finding.

### Changed
- Repository moved to `github.com/lost-in-the/grove`. Homebrew install now uses the shorthand tap form.
- TUI markdown help renderer switched from Glamour to manual lipgloss for tighter control over layout.
- `teatest` upgraded to v2; goldens regenerated.

### Fixed
- Shell integration: prevent infinite recursion when the `grove` binary is not on PATH.
- TUI: name input no longer overlaps cursor with the placeholder.
- TUI: branch step split out so `j`/`k` work in text input without triggering navigation.
- TUI: `creationLogMsg` dispatched by source instead of fanning out to all logs.
- General CLI/TUI UX polish.

### Internal
- Constants extracted; prealloc and lint cleanup.
- Function complexity reduced across packages.
- TUI: dead code, unused functions, and duplicate PR/issue rendering removed.
- 62 tests added for UX polish coverage gaps.
- Autonomous optimization loop scripts added under `scripts/`.
- CI: release workflow uses `go-version-file` instead of a hardcoded Go version.

## [0.5.0] - 2026-03-10

### Added
- `grove which` command for operational context — shows current worktree, branch, project, and Docker status
- `-CC` tmux control mode for iTerm2 integration
- Configurable container lifecycle on `grove to` via `container_switch` config (restart, stop, none)
- `--branch` and `--from` flags for `grove new` — override branch name or base ref
- Auto-switch to new worktree after `grove new` (shell integration)
- `Find()` now matches worktrees by branch name in addition to short name
- Progress indicator during worktree deletion in the TUI
- Dynamic compact toggle label and preference persistence in the TUI
- VHS showcase GIF and demo fixture automation
- `symlink_files` option for external compose config
- Two-tier `grove doctor` with contextual errors and config symlink detection

### Fixed
- Dirty worktree handling no longer blocks switching in valid cases
- Key flash on TUI status badges resolved
- Worktree rename and checkout edge cases fixed
- TUI fork from root no longer skips the name input step
- PR browser no longer swallows the 'o' key when filter input is focused
- Command timeouts, signal handling, and correctness fixes
- General hardening: bug fixes, timeout enforcement, improved error handling

## [0.4.0] - 2026-03-04

### Added
- `grove agent-help` command: concise reference for AI agents — env vars, common commands, and tips for programmatic use
- `grove test` command: Run the configured test command in a worktree, with optional Docker service support for running tests in an ephemeral container
- Config resolution from `.grove` directory for secondary worktrees, so non-main worktrees correctly inherit project configuration
- Tmux mode setting (`auto`/`manual`/`off`) with shell auto-attach support, giving finer control over tmux session behavior
- Config overlay save confirmation and changed-field indicators in the TUI, making it clear which fields have been modified before saving
- External compose mode with plugin hook registry, enabling Docker services defined in a shared external directory to be managed per-worktree
- "For AI Agents" section in README pointing to `grove agent-help` and Agent Guide

### Fixed
- Shell integration binary resolution: use `command -v grove` instead of hardcoded `os.Executable()` path, which broke when installed via `go run` or after brew upgrades (ShellVersion bumped to 3 — re-run `grove setup` to update)

### Previously Added
- **Phase 5: Polish & Production Readiness**
  - GoReleaser configuration for automated releases
  - Homebrew formula for easy installation
  - GitHub Actions workflow for release automation
  - Shell integration files (grove.zsh, grove.bash)
  - Shell completions (zsh and bash)
  - Multi-platform binary builds (Linux, macOS, Windows)
  - Release notes automation
- **Phase 4: Issue Integration**
  - Tracker plugin with adapter pattern
  - GitHub adapter using `gh` CLI
  - `grove fetch pr/<number>` command
  - `grove fetch issue/<number>` command
  - `grove issues` command with fzf browsing
  - `grove prs` command with fzf browsing
  - Smart worktree naming from issue/PR metadata
  - Filtering options (state, labels, assignee, author)
- **Phase 3: Time Tracking**
  - Time tracking plugin with automatic session management
  - `grove time` command to show time for current or all worktrees
  - `grove time week` command for weekly summary
  - Hook integration for automatic time tracking on worktree switch
  - JSON output support for `grove time` commands
  - Notification system for macOS and Linux
- **Phase 2: State Management**
  - `grove freeze` and `grove resume` commands
  - State persistence for frozen worktrees
  - Docker integration with freeze/resume lifecycle
- **Phase 1: Docker Plugin**
  - Docker container management integrated with worktrees
  - `grove up`, `grove down`, `grove logs`, `grove restart` commands
  - Hook-based auto-start/stop functionality
- **Phase 0: Foundation**
  - Core commands: ls, new, to, rm, here, last
  - Shell integration for zsh and bash with cd directive
  - TOML configuration system
  - Git worktree operations
  - Tmux session management
  - Hook system foundation

### Changed
- Updated README with Homebrew installation as primary method
- Updated roadmap to mark all phases complete
- Improved installation documentation

### Fixed

## [0.1.0] - 2026-02-28

Initial release
