# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added
- `grove adopt [path]` command — bootstraps a git worktree that grove doesn't know about (config symlink, state registration, post-create hooks)
- Drift detection — running any grove command from a worktree not in `state.json` prints a non-fatal warning suggesting `grove adopt`
- `grove doctor` Tier-2 "Worktree registration" check — reports drifted worktrees with a `grove adopt` hint
- `[test]` config: `include_deps` (bool, default false) and `bind_mount` (string) — control `compose run` dependency resolution and worktree bind-mount path
- `grove test --with-deps` and `--bind` flags — per-invocation overrides of `[test]` config
- `[plugins.docker.external]` config: `non_blocking_services` — services allowed to exit (one-shot init, etc.) without marking the stack unhealthy
- `grove doctor` now lists configured non-blocking services for external compose stacks
- `internal/grove.IsWorktreeInState` — shared helper for state.json drift detection

### Changed
- **Behavior change:** `grove test` now passes `--no-deps` to `compose run` by default. Tests that rely on dependency services starting (e.g., a database) need either `[test] include_deps = true` in config or the `--with-deps` flag.
- **Behavior change:** `grove up` against an external stack tolerates failures of services listed in `non_blocking_services` — the command no longer exits non-zero when only one-shot init services failed.
- **Behavior change:** `grove ps` external-stack status is now driven by `docker compose ps --format json` and the `non_blocking_services` list. A stack with only non-blocking services exited cleanly is `up`, not `degraded`.
- `TestConfig.IncludeDeps` is now `*bool` so a project-level `false` can override a global `true`
- `BootstrapWorktree` extracted from `setupCreatedWorktree` so `grove new` and `grove adopt` share the same post-`git worktree add` sequence
- `grove adopt` strips the project prefix from directory names (e.g., `myproj-feature` → `feature`) so adopted worktrees match grove's naming convention
- Service-health probe timeout raised from 1s to 3s to tolerate slow systems
- Compose `--env-file` is honored when reading the active-worktree env var (previously hardcoded to `.env`)

### Fixed
- `grove test` now translates `compose run` "service didn't complete successfully" errors into actionable grove-styled messages
- `grove up` no longer silently swallows compose-up failures when the post-up health probe returns no statuses
- `grove up` skips the post-up health probe when compose-up succeeded (previously paid up to 1s on every successful run)
- Post-create hook execution failures are now logged to grove's debug log (previously discarded silently)
- `grove adopt` refuses to "adopt" the main worktree (it is always registered)
- `grove adopt` errors out on detached HEAD instead of storing the literal `"HEAD"` as a branch name
- Removed unused `matchesActive` parameter from external-status classifier; removed `_ = name` dead wiring in env-loader doctor checks; removed dead `BootstrapOpts.Now` injection field

## [0.7.0] - 2026-05-04

> **Upgrading:** No breaking config changes. Docker users should run `grove doctor` to surface host install commands that should now be `docker:compose` hooks (`grove doctor --fix` rewrites them automatically).

### Added
- New hook action types `docker:compose` and `docker:exec` for routing config-driven hooks into containers (see `docs/CONFIGURATION_REFERENCE.md`). Action type names use a `pluginname:action` namespace convention.
- Pluggable hook action handler registry — plugins can register custom action types via `hooks.RegisterActionHandler` (idempotent, last-write-wins). See `docs/PLUGIN_DEVELOPMENT.md`.
- `grove init` now picks between `auto` (preview + confirm) and `walkthrough` (step-by-step) modes when running interactively. New flags: `--auto`, `--walkthrough`, `--yes`. Non-TTY behavior preserved as silent auto.
- Docker-aware project detection: when a compose file is present alongside Rails/Node/Python markers, install commands (`bundle install`, `npm install`, `pip install`) are auto-generated as `docker:compose` hooks instead of host commands. Service name inferred from `docker-compose.yml` (single service used, or first non-infra service). Dockerfile-only projects (no compose file) keep host commands and emit a manual-setup note rather than generating broken compose hooks.
- `grove doctor` now detects host install commands inside a Docker project and stray `.grove/.grove-backup/` directories. New `grove doctor --fix` rewrites flagged host install commands to `docker:compose` hooks in place.
- `symlink_files` documented in top-level README and CONFIGURATION_REFERENCE alongside `symlink_dirs`.
- `grove trim` now accepts `prune` as an alias for git-flavored discoverability (issue #10).
- README beta notice, edge install path (`go install ...@main`), and per-install-method update guidance (issue #11).
- TUI branch selector now includes remote-only branches; selecting a remote-only branch fetches from origin automatically.

### Changed
- Hook execution order on worktree create: plugin Go hooks now fire **before** config-driven `[[hooks.post_create]]` so containers are up by the time user setup commands run. This removes a workaround in the new `docker:compose` handler and lets `mode = "exec"` work without a stealth `compose up`.
- `grove trim`/`grove repair` confirmation prompts now respond to Ctrl+C and ESC instead of hanging on raw `fmt.Scanln` (issue #17). `trim` keeps its literal "yes" guard and continues to support scripted `echo yes | grove trim`.

### Fixed
- `bundle install`/`npm install` post-create hooks no longer fail on the host for Docker-based dev stacks (issue #28). Downstream: `grove rm --force` no longer hits non-empty `node_modules` conflicts when `symlink_dirs` is configured (issue #24).
- `grove rm --force` now succeeds on worktrees containing non-empty untracked directories (e.g. `node_modules` left by a post-create hook). When git's own `worktree remove --force` refuses, grove falls back to removing the directory itself and pruning git's metadata (issue #24).
- `Manager.Remove` refuses to remove the main worktree as a defense-in-depth backstop for the new `os.RemoveAll` fallback.
- `grove trim` no longer reports "9999 days since last access" for worktrees missing state. `grove init` now stamps `created_at`/`last_accessed_at` on the main worktree, and `trim` falls back to the worktree's HEAD commit time (or "last access unknown") when no state timestamp is available (issue #9).
- State load now backfills zero-valued `created_at`/`last_accessed_at` timestamps on worktrees from earlier versions, so upgraders no longer see lingering `"0001-01-01T00:00:00Z"` in their `state.json`.
- `docs/COMMAND_SPECIFICATIONS.md` `grove init` section now documents the actual command (it had been showing `grove install <shell>` content). New init flags (`--auto`, `--walkthrough`, `--yes`) and Docker-aware install routing are now discoverable from the spec, the Docker plugin README, and the agent guide.

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
