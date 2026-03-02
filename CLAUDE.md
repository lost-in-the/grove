# Grove - Worktree Flow Manager
**This application is unreleased. Do not hesitate to make changes or completely refactor logic if it can be improved**

## Project Context
Grove is a Go CLI tool for managing git worktrees with tmux integration.
Target: developers who context-switch frequently between tasks.

## Key Documentation
- **[Command Specifications](docs/COMMAND_SPECIFICATIONS.md)** — Exhaustive behavior specs for every command including naming conventions, shell integration protocol, and expected outputs
- **[TUI Dashboard](docs/TUI.md)** — TUI layout, components, and interaction model
- **[Shell Integration](docs/SHELL_INTEGRATION.md)** — Shell wrapper protocol and environment setup
- **[Plugin Development](docs/PLUGIN_DEVELOPMENT.md)** — Hook interfaces and plugin authoring guide
- **[Example Configs](examples/)** — Example grove configurations
- **[Visual Testing](docs/VISUAL_TESTING.md)** — Golden files, tmux capture, VHS tapes for TUI visual iteration

## Critical Implementation Rules

### Worktree Naming Convention
Worktrees MUST follow the `{project}-{name}` pattern:
- `grove-cli-testing` not `testing`
- `grove-cli-feature-auth` not `feature-auth`
- Project name derived from: git remote → directory name → config

### Shell Integration Protocol
Commands that change directories output `cd:/path/to/dir` which the shell wrapper intercepts:
```bash
# Binary outputs: cd:~/projects/grove-cli-testing
# Shell wrapper detects GROVE_SHELL=1 and executes the cd
```

### Display Rules
- `grove ls` shows SHORT names ("testing", "main") not full paths
- `grove here` shows: short name, branch, short SHA (7 chars), commit message, age
- Tmux sessions use FULL names: `grove-cli-testing`

## Build Commands
- `make build` - Build the binary
- `make test` - Run all tests
- `make test-verbose` - Run tests with verbose output
- `make test-coverage` - Generate coverage report
- `make lint` - Run linters
- `make fmt` - Format code
- `make clean` - Clean build artifacts
- `make install` - Install locally

## TUI Stack

The TUI is built on **Bubbletea v2** (Elm Architecture) with the Charm ecosystem:
- `charm.land/bubbletea/v2` — framework (Model/Update/View)
- `charm.land/lipgloss/v2` — styling (ANSI-aware widths, colors, borders)
- `charm.land/bubbles/v2` — components (list, textinput, viewport)

Key patterns:
- **ANSI-aware string measurement** — always use `lipgloss.Width(s)` not `len(s)` for visible width of styled text
- **Style.Render()** for applying styles to text, not `Style.SetString()` + `Style.String()`
- `internal/theme/colors.go` — centralized `ColorScheme` with semantic colors
- `internal/tui/theme_v2.go` — `StyleSet` with lipgloss styles built from the color scheme
- `internal/tui/list_v2.go` — **default** two-line delegate with indicator/status columns (`NewWorktreeDelegateV2`)
- `internal/tui/list.go` — compact single-line V1 delegate with column headers (`NewWorktreeDelegate`)
- The `v` key toggles between V2 (default) and V1 compact modes at runtime; can also be set permanently via `tui.compact_list = true` in config

## Code Style
- Follow standard Go formatting (gofmt)
- Use table-driven tests
- Prefer composition over inheritance
- Small interfaces (1-3 methods)
- Error messages: lowercase, no punctuation
- Wrap errors with context: `fmt.Errorf("operation failed: %w", err)`

## Architecture Rules
- `cmd/` - Entry points only, no business logic
- `internal/` - Core logic, not importable externally
- `plugins/` - Self-contained plugins implementing hook interfaces
- Each plugin must have its own README.md

## Testing Requirements
- TDD: Write tests before implementation
- Table-driven tests for multiple cases
- Test file next to source: `foo.go` → `foo_test.go`
- Minimum 80% coverage for core packages
- Integration tests in `_test.go` files with build tag `//go:build integration`

## Visual Iteration

Golden files are the primary tool for visual feedback on TUI changes:
- `make golden-diff` — update golden files and show what changed
- `make golden-view TEST=TestGolden_Dashboard` — print specific golden output
- `make tui-capture` — capture live TUI state via tmux
- `make tui-capture-keys KEYS="j j Enter"` — capture after key sequence

**When to use what**:
- Golden files: regression detection, design iteration, CI validation
- tmux capture: live interaction, key sequences, agent visual feedback
- VHS: demo recordings, documentation GIFs

See [docs/VISUAL_TESTING.md](docs/VISUAL_TESTING.md) for the full guide.

## Git Workflow
- Conventional commits: `type(scope): description`
- Types: feat, fix, docs, style, refactor, test, chore
- Branch naming: `type/short-description`
- Squash merge to main

## DO NOT
- Use `panic()` except in truly unrecoverable situations
- Add dependencies without justification in commit message
- Skip tests for "simple" changes
- Use `interface{}` without type assertions
- Commit without running `make lint test`

## IMPORTANT
- Every command must complete in <500ms
- Shell integration must work in both zsh and bash
- Test tmux operations with mock when possible
- Document all public functions

## Documentation Review

Before committing, check if any documentation files need updating:
- If command behavior changed → update `docs/COMMAND_SPECIFICATIONS.md`
- If TUI layout/keybindings changed → update `docs/TUI.md`
- If shell integration changed → update `docs/SHELL_INTEGRATION.md`
- If plugin interfaces changed → update `docs/PLUGIN_DEVELOPMENT.md`
- If golden test patterns changed → update `docs/VISUAL_TESTING.md`
- If build/test/CI process changed → update `CONTRIBUTING.md`
- If public API or config changed → update `README.md`

Cross-reference doc content against the actual code for accuracy before committing.
