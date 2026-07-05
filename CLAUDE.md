# Grove - Worktree Flow Manager
**Unreleased. Do not hesitate to refactor if it can be improved.**

Go CLI for managing git worktrees with tmux integration. Config: `.grove/config.toml`.

## For Agents Using Grove

If you're an agent helping a user *use* grove (install, configure, run commands), start with [AGENTS.md](AGENTS.md) — it's the front door for agent-facing context.

---

## Key Documentation
- **[Command Specifications](docs/COMMAND_SPECIFICATIONS.md)** — behavior specs, naming, shell integration, expected outputs
- **[TUI Dashboard](docs/TUI.md)** — layout, components, interaction model, agent notes on file paths/patterns
- **[Shell Integration](docs/SHELL_INTEGRATION.md)** — shell wrapper protocol and environment setup
- **[Plugin Development](docs/PLUGIN_DEVELOPMENT.md)** — hook interfaces and plugin authoring
- **[Visual Testing](docs/VISUAL_TESTING.md)** — golden files, tmux capture, VHS tapes
- **[Agent Guide](docs/AGENT_GUIDE.md)** — installation, workflows, and strategies for AI agents
- **[Configuration Reference](docs/CONFIGURATION_REFERENCE.md)** — complete config.toml and hooks.toml reference

## Critical Rules

### Worktree Naming
Worktree directories follow the project's `[naming] pattern` (default `{project}-{name}`: `grove-testing` not `testing`). Patterns must contain `{project}` and `{name}` exactly once each; literals limited to `[A-Za-z0-9._-]`.
Tmux session names ALWAYS use canonical `{project}-{name}`, regardless of the directory pattern.
Project name derived from: config > git remote > directory name.

### Shell Integration Protocol
Commands that change directories output `cd:/path/to/dir` — shell wrapper intercepts when `GROVE_SHELL=1`.

### Display Rules
- `grove ls` shows SHORT names ("testing", "main") not full paths
- `grove here` shows: short name, branch, short SHA (7 chars), commit message, age
- Tmux sessions use FULL names: `grove-testing`

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
