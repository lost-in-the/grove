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
