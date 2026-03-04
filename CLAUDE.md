# Grove - Worktree Flow Manager
**Unreleased. Do not hesitate to refactor if it can be improved.**

Go 1.24 CLI for managing git worktrees with tmux integration. Config: `.grove/config.toml`.

## Key Documentation
- **[Command Specifications](docs/COMMAND_SPECIFICATIONS.md)** — behavior specs, naming, shell integration, expected outputs
- **[TUI Dashboard](docs/TUI.md)** — layout, components, interaction model, agent notes on file paths/patterns
- **[Shell Integration](docs/SHELL_INTEGRATION.md)** — shell wrapper protocol and environment setup
- **[Plugin Development](docs/PLUGIN_DEVELOPMENT.md)** — hook interfaces and plugin authoring
- **[Visual Testing](docs/VISUAL_TESTING.md)** — golden files, tmux capture, VHS tapes

## Critical Rules

### Worktree Naming
Worktrees MUST follow `{project}-{name}`: `grove-cli-testing` not `testing`.
Project name derived from: git remote > directory name > config.

### Shell Integration Protocol
Commands that change directories output `cd:/path/to/dir` — shell wrapper intercepts when `GROVE_SHELL=1`.

### Display Rules
- `grove ls` shows SHORT names ("testing", "main") not full paths
- `grove here` shows: short name, branch, short SHA (7 chars), commit message, age
- Tmux sessions use FULL names: `grove-cli-testing`

## Architecture
- `cmd/` — entry points only, no business logic
- `internal/` — core logic, not importable externally
- `plugins/` — self-contained, implementing hook interfaces (each has own README)
- Wrap errors with context: `fmt.Errorf("operation failed: %w", err)`

## Testing
- TDD: write tests before implementation
- Run a single test: `go test ./internal/foo/ -run TestName -v`
- Visual iteration: see [docs/VISUAL_TESTING.md](docs/VISUAL_TESTING.md)

## Constraints
- Every command must complete in <500ms
- Shell integration must work in both zsh and bash
- Test tmux operations with mock when possible
- No `panic()` except truly unrecoverable situations
- Run `make lint test` before committing
- Before committing, check if changes require updating any docs in `docs/`

## Git Workflow
- Conventional commits: `type(scope): description`
- Branch naming: `type/short-description`
- Squash merge to main
