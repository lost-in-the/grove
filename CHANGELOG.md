# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

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
