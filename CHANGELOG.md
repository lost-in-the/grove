# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added
- **Phase 3: Time Tracking**
  - Time tracking plugin with automatic session management
  - `grove time` command to show time for current or all worktrees
  - `grove time week` command for weekly summary
  - Hook integration for automatic time tracking on worktree switch
  - JSON output support for `grove time` commands
  - Notification system for macOS and Linux
- Initial Phase 0 foundation implementation
- Core commands: ls, new, to, rm, here, last
- Shell integration for zsh and bash
- TOML configuration system
- Git worktree operations
- Tmux session management
- Hook system foundation
- **Phase 1: Docker Plugin**
  - Docker container management integrated with worktrees
  - `grove up`, `grove down`, `grove logs`, `grove restart` commands
- **Phase 2: State Management**
  - `grove freeze` and `grove resume` commands
  - State persistence for frozen worktrees

### Changed

### Fixed

## [0.1.0] - TBD

Initial release
