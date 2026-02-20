# GitHub Copilot Instructions for Grove

This document provides context for GitHub Copilot when working in this repository.

## Project Overview

Grove is a Go CLI tool for managing git worktrees with tmux integration. It provides zero-friction context switching for developers who work on multiple features simultaneously.

**Primary command:** `grove` (aliased as `w`)  
**Performance requirement:** All commands must complete in <500ms

## Key Documentation

Before making changes, review these specifications:

- **[docs/COMMAND_SPECIFICATIONS.md](../docs/COMMAND_SPECIFICATIONS.md)** — Complete behavior specs for every command
- **[docs/TUI.md](../docs/TUI.md)** — TUI dashboard documentation
- **[docs/SHELL_INTEGRATION.md](../docs/SHELL_INTEGRATION.md)** — Shell integration deep dive
- **[docs/PLUGIN_DEVELOPMENT.md](../docs/PLUGIN_DEVELOPMENT.md)** — Plugin development guide
- **[CLAUDE.md](../CLAUDE.md)** — AI assistant context (also useful for Copilot)

## Critical Implementation Rules

### 1. Worktree Naming Convention

Worktrees MUST use the `{project}-{name}` pattern:

```
✓ grove-cli-testing
✓ grove-cli-feature-auth
✗ testing
✗ feature-auth
```

Project name detection priority:
1. Git remote URL (extract repo name)
2. Parent directory name
3. Explicit config setting

### 2. Shell Integration Protocol

Commands that change directories use a special protocol:

```bash
# Binary outputs to stdout:
cd:/path/to/worktree

# Shell wrapper (with GROVE_SHELL=1) intercepts and executes:
cd "/path/to/worktree"
```

When implementing commands that switch worktrees, output `cd:` prefix, never call `cd` directly.

### 3. Display Conventions

| Context | Format |
|---------|--------|
| `grove ls` output | Short names: "testing", "main" |
| `grove here` output | Short name + branch + short SHA (7 chars) + commit message + age |
| Tmux session names | Full names: "grove-cli-testing" |
| Error messages | Lowercase, no punctuation, wrap with context |

## Architecture

```
grove/
├── cmd/grove/      # Entry points only - NO business logic
├── internal/       # Core logic - not importable externally
│   ├── config/     # TOML configuration
│   ├── worktree/   # Git worktree operations
│   ├── tmux/       # Tmux session management
│   ├── shell/      # Shell integration (zsh/bash)
│   └── hooks/      # Plugin hook system
└── plugins/        # Self-contained plugins with own READMEs
```

## Code Style

```go
// DO: Small interfaces
type Switcher interface {
    SwitchTo(name string) error
}

// DO: Wrap errors with context
return fmt.Errorf("failed to create worktree %q: %w", name, err)

// DO: Table-driven tests
func TestCreate(t *testing.T) {
    tests := []struct {
        name    string
        input   string
        wantErr bool
    }{
        {"valid", "feature-x", false},
        {"empty", "", true},
    }
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            // ...
        })
    }
}

// DON'T: panic() except truly unrecoverable
// DON'T: interface{} without type assertions
// DON'T: Skip tests for "simple" changes
```

## Testing Requirements

1. **TDD mandatory** — Write failing tests before implementation
2. **Table-driven** — Use subtests for multiple cases
3. **80% coverage** — Required for `internal/` packages
4. **Integration tests** — Use `//go:build integration` tag

## Git Conventions

**Commits:** `type(scope): description`
- Types: `feat`, `fix`, `docs`, `style`, `refactor`, `test`, `chore`
- Example: `feat(worktree): implement grove new with project prefix`

**Branches:** `type/short-description`
- Example: `fix/shell-integration-cd-protocol`

## Common Tasks

### Adding a new command

1. Create `cmd/grove/{command}.go`
2. Write tests in `cmd/grove/{command}_test.go` FIRST
3. Register with Cobra in `root.go`
4. Update shell completions if needed
5. Add to docs/COMMAND_SPECIFICATIONS.md

### Modifying worktree behavior

1. Check `internal/worktree/` for existing patterns
2. Update `docs/COMMAND_SPECIFICATIONS.md` with new behavior
3. Add test cases for the new behavior
4. Implement with TDD

### Working with shell integration

1. Review `internal/shell/integration.go`
2. Test both zsh AND bash paths
3. Use `GROVE_SHELL=1` detection pattern
4. Output `cd:` directives, never execute cd directly

## Build Commands

```bash
make build          # Build binary
make test           # Run all tests
make test-verbose   # With verbose output
make lint           # Run linters
make fmt            # Format code
make install        # Install to $GOPATH/bin
```

## Known Issues

See GitHub Issues for current implementation problems. Key areas:
- Worktree naming needs project prefix implementation
- Shell integration `cd:` protocol needs completion
- `grove here` display format needs enrichment
