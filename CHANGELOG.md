# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

## [0.7.0] - YYYY-MM-DD

> **Upgrading:** No breaking config changes — all new fields have defaults and existing configs continue to work. Docker users should run `grove doctor` to surface host install commands that should now be `docker:compose` hooks (`grove doctor --fix` rewrites them automatically). See "Behavior changes" and "Migration / consumer-side" below.

### Behavior changes (read before upgrading)

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
