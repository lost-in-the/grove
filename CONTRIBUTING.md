# Contributing to Grove

Thank you for your interest in contributing to Grove! This document provides guidelines for contributing to the project.

## Code of Conduct

Be respectful, inclusive, and professional. We're all here to build something great together.

## Getting Started

### Prerequisites

- Go 1.25 or later
- Git 2.30 or later
- Tmux 3.0 or later (for testing tmux features)
- Make

### Development Setup

1. Fork the repository
2. Clone your fork:
   ```bash
   git clone https://github.com/YOUR_USERNAME/grove
   cd grove
   ```
3. Add upstream remote:
   ```bash
   git remote add upstream https://github.com/lost-in-the/grove
   ```
4. Install dependencies:
   ```bash
   go mod download
   ```
5. Build and test:
   ```bash
   make build
   make test
   ```

## Making Changes

### Branch Naming

Use descriptive branch names with a type prefix:

- `feat/description` - New features
- `fix/description` - Bug fixes
- `docs/description` - Documentation changes
- `refactor/description` - Code refactoring
- `test/description` - Test improvements
- `chore/description` - Maintenance tasks

### Commit Messages

We use [Conventional Commits](https://www.conventionalcommits.org/):

```
type(scope): description

[optional body]

[optional footer]
```

**Types:**
- `feat` - New feature
- `fix` - Bug fix
- `docs` - Documentation only
- `style` - Formatting, no code change
- `refactor` - Code change that neither fixes bug nor adds feature
- `test` - Adding/updating tests
- `chore` - Maintenance tasks

**Examples:**
```
feat(core): add grove last command for quick switching
fix(tmux): handle session names with spaces correctly
docs(readme): add installation instructions for homebrew
test(worktree): add tests for dirty status detection
```

### Code Style

- **Follow standard Go formatting**: Run `gofmt` and `go vet`
- **Use table-driven tests**: For testing multiple scenarios
- **Small interfaces**: Prefer 1-3 methods per interface
- **Error messages**: Lowercase, no trailing punctuation
- **Error wrapping**: Use `fmt.Errorf("context: %w", err)` for context
- **Interface discipline**: Accept interfaces, return concrete types
- **New dependencies**: Require explicit justification in the commit message

**Example:**
```go
// Good
func Create(name string) error {
    if name == "" {
        return fmt.Errorf("name cannot be empty")
    }
    // ...
}

// Bad
func Create(name string) error {
    if name == "" {
        return fmt.Errorf("Name cannot be empty.")  // Capitalized, has period
    }
    // ...
}
```

### Testing Requirements

- **Write tests first**: We follow TDD (Test-Driven Development)
- **Table-driven tests**: For multiple test cases
- **Minimum 80% coverage**: For core packages (`internal/`)
- **Test file location**: Next to source file (`foo.go` → `foo_test.go`)

**Example test:**
```go
func TestWorktreeCreate(t *testing.T) {
    tests := []struct {
        name    string
        input   string
        wantErr bool
        errMsg  string
    }{
        {
            name:    "valid name creates worktree",
            input:   "feature-auth",
            wantErr: false,
        },
        {
            name:    "empty name returns error",
            input:   "",
            wantErr: true,
            errMsg:  "name cannot be empty",
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            // Test implementation
        })
    }
}
```

### Architecture Rules

- **`cmd/`**: Entry points only, no business logic
- **`internal/`**: Core logic, not importable externally
- **`plugins/`**: Self-contained plugins implementing hook interfaces
- **Performance**: Every command must complete in <500ms

### Pull Request Process

1. **Sync with upstream**:
   ```bash
   git fetch upstream
   git rebase upstream/main
   ```

2. **Create your feature branch**:
   ```bash
   git checkout -b feat/amazing-feature
   ```

3. **Write tests first**:
   - Write failing tests that describe expected behavior
   - Run tests to confirm they fail
   - Implement the feature
   - Run tests to confirm they pass

4. **Ensure quality**:
   ```bash
   make fmt      # Format code
   make lint     # Run linter
   make test     # Run tests
   ```

5. **Commit with conventional message**:
   ```bash
   git add .
   git commit -m "feat(core): add amazing feature"
   ```

6. **Push to your fork**:
   ```bash
   git push origin feat/amazing-feature
   ```

7. **Open Pull Request**:
   - Go to GitHub and open a PR against `main`
   - Fill out the PR template
   - Link any related issues

### PR Checklist

Before submitting, ensure:

- [ ] Tests added/updated and passing
- [ ] Documentation updated (README, code comments)
- [ ] Code formatted (`make fmt`)
- [ ] Linter passes (`make lint`)
- [ ] Conventional commit message used
- [ ] No merge conflicts with `main`
- [ ] Golden files reviewed (`make golden-diff`) — no unintended visual regressions
- [ ] If changes affect `docs/`, updated in this same PR
- [ ] **Docs are current** — see [Pre-PR requirements](#pre-pr-requirements) below
- [ ] **Plugin impact investigated** — see [Claude Code plugin changes](#claude-code-plugin-changes) below

### Pre-PR requirements

These are hard gates — a PR that fails either is incomplete:

1. **Documentation must be current.** Any doc affected by the change — README, `docs/`, the
   `skills/grove-worktree-management/` skill, `CHANGELOG.md`, config references — must be
   updated in the *same* PR. Do not ship behavior that the docs still describe the old way.
2. **Investigate plugin-functionality impact.** If the change touches anything the distributed
   skill relies on (a `grove` command, flag, `--json` output field, env var, or config
   schema), investigate whether `skills/grove-worktree-management/` needs to change too — see
   below.

### Claude Code plugin changes

`skills/grove-worktree-management/` is distributed as the `grove-plugin` via the
[`lost-in-the/plugins`](https://github.com/lost-in-the/plugins) marketplace (a `git-subdir`
reference to this subtree). Because installed users may run an **older** grove than `main`:

- If a CLI change adds/removes/renames a command, flag, or `--json` field the skill documents,
  **update the skill in the same PR** (its command table is validated against
  [docs/COMMAND_SPECIFICATIONS.md](docs/COMMAND_SPECIFICATIONS.md)).
- The skill's Version Preflight tells agents to operate only against the installed version, so
  **land the skill change with the release that ships the capability** — never document a
  command in the skill before the version that provides it is released. Bump the skill's
  `version` in `.claude-plugin/plugin.json` when the skill changes.
- If the change requires a new marketplace entry, `ref` pin, or metadata edit, open a **paired
  PR against `lost-in-the/plugins`** and link it from this PR.

### Common Task Recipes

#### Adding a new command

1. Create `cmd/grove/commands/{command}.go`
2. Write tests in `cmd/grove/commands/{command}_test.go` **first** (TDD)
3. Register with Cobra in `cmd/grove/commands/root.go`
4. Update shell completions if the command takes worktree names as arguments
5. Add the command to `docs/COMMAND_SPECIFICATIONS.md`

#### Modifying worktree behavior

1. Check `internal/worktree/` for existing patterns first
2. Update `docs/COMMAND_SPECIFICATIONS.md` with the new behavior
3. Write test cases before implementing (TDD)

#### Working with shell integration

1. Review `docs/SHELL_INTEGRATION.md` and `internal/shell/`
2. Test both zsh AND bash paths
3. Output `cd:` directives from binary stdout; never call `cd` directly
4. Use `GROVE_SHELL=1` detection (set by the wrapper) to enable directive output

#### Behavioral invariants (do not break)

- Worktree names must follow `{project}-{name}` — enforced by naming logic in `internal/worktree/`
- `grove ls` displays **short** names; tmux session names use **full** names (`{project}-{name}`)
- Shell directives (`cd:`, `tmux-attach:`, `env:`) must be zero-ANSI — parsed literally by the wrapper
- Every command must complete in <500ms under normal conditions

## Development Commands

```bash
make build          # Build the binary
make test           # Run all tests
make test-verbose   # Run tests with verbose output
make test-coverage  # Generate coverage report
make test-integration  # Run integration tests (requires git)
make lint           # Run linters
make fmt            # Format code
make clean          # Clean build artifacts
make install        # Install locally
make golden-diff    # Update golden files and show visual changes
make golden-view TEST=TestGolden_Dashboard  # Print specific golden output
make test-fixture   # Create test fixture for live TUI testing
make tui-capture    # Capture live TUI state via tmux
```

## Development Guide

### Building Locally

`make build` produces the binary at `bin/grove`. `make install` copies it to `$GOPATH/bin` and codesigns it on macOS.

Release builds use `CGO_ENABLED=0` (via GoReleaser). Version information (`internal/version.Version`, `.Commit`, `.BuildDate`) is injected by ldflags at release time; dev builds show defaults from `internal/version/version.go`.

### Test Infrastructure

**Unit tests**: `make test` runs `go test -race -cover ./...` — the same command CI uses.

**Integration tests**: `make test-integration` runs tests tagged with `//go:build integration`. These require git and test real git operations. They're slower and not included in the default `make test`.

**Golden file tests**: Visual regression tests for the TUI. Golden files capture expected terminal output and fail when the output changes unexpectedly.
- `make golden-diff` — update golden files and show what changed (via `git diff`)
- `make golden-view TEST=TestGolden_Dashboard` — print a specific golden file's output
- See [docs/VISUAL_TESTING.md](docs/VISUAL_TESTING.md) for the full guide

**Test fixtures**: `make test-fixture` creates `/tmp/grove-test-fixture/` — a multi-worktree git repo for live TUI testing and tmux capture.

**Coverage**: `make test-coverage` generates `coverage.html`. Core packages (`internal/`) target 80% minimum.

### CI Pipeline

CI runs on push to `main` (and `copilot/**` branches) and on PRs to `main`. Three jobs run in parallel:

| Job | What it does |
|-----|-------------|
| **Test** | `go test -race -cover ./...` |
| **Lint** | golangci-lint (version pinned in CI) + `go vet` + `gofmt -s` check |
| **Build** | `make build` (binary compilation) |

All three use Go 1.25 with module cache keyed by `go.sum`. All three must pass for a PR to merge.

### Releases & Distribution

[GoReleaser](https://goreleaser.com) handles releases, triggered by pushing a `v*.*.*` tag.

**Platforms**: Linux (amd64/arm64), macOS (amd64/arm64), Windows (amd64)

**Distribution**:
- GitHub Releases — binary archives with LICENSE, README, CHANGELOG, and CONTRIBUTING (shell integration is generated at runtime via `grove setup` / `grove install`)
- Homebrew tap — `lost-in-the/homebrew-tap` (`brew install lost-in-the/tap/grove`)

**Test-only dependencies** (`teatest`, `golden`, etc.) are safe in `go.mod`. Go only compiles `_test.go` imports into test binaries, never into release builds. No action needed to exclude them.

## Project Structure

```
grove/
├── cmd/
│   └── grove/
│       ├── main.go           # Entry point
│       └── commands/         # Command implementations
├── internal/
│   ├── config/              # Configuration loading
│   ├── git/                 # Git operations
│   ├── hooks/               # Hook system
│   ├── shell/               # Shell integration
│   ├── tmux/                # Tmux session management
│   ├── tui/                 # Terminal UI (interactive mode)
│   ├── worktree/            # Git worktree operations
│   └── version/             # Version info
├── plugins/
│   ├── docker/              # Docker container lifecycle plugin
│   └── tracker/             # Issue tracker integration plugin
├── docs/                    # Extended documentation
├── Makefile                 # Build automation
├── go.mod                   # Go module definition
└── README.md                # Documentation
```

## Adding New Features

### For Core Features

1. Discuss in an issue first
2. Follow TDD: tests → implementation → refactor
3. Update documentation
4. Add to CHANGELOG.md

### For Plugins

1. Create plugin in `plugins/` directory
2. Implement the Plugin interface (`Name()`, `Init()`, `RegisterHooks()`, `Enabled()`)
3. Add README.md in plugin directory
4. Register hooks in plugin init
5. Add tests

## Documentation

- **Public functions**: Must have doc comments
- **Packages**: Should have package-level documentation
- **Complex logic**: Add inline comments explaining "why", not "what"
- **Examples**: Add to README.md for new features

## Platform Notes

### Windows

Grove is distributed for Windows (amd64) via GoReleaser, but a few features have OS-level constraints.

**Symlinks require elevated privileges.** `os.Symlink` on Windows fails unless either Developer Mode is enabled or the process is running as an administrator. This affects:

- `symlink_files` in `[plugins.docker.external]` config
- `symlink_dirs` in `[plugins.docker.external]` config
- The `type = "symlink"` hook action in `hooks.toml`

**Recommendation for Windows users and contributors:**

- Use `copy_files` (external config) or `type = "copy"` hooks instead of their symlink counterparts for files. Copies work without elevation. Directory sharing (`symlink_dirs`) has no copy equivalent yet, so it needs Developer Mode on Windows.
- If symlinks are important for your setup, enable Developer Mode in Windows Settings → Privacy & Security → For Developers, or run grove from an elevated terminal.
- When writing platform-sensitive tests, use `//go:build !windows` to skip symlink assertions on Windows, or use `t.Skip` with a runtime GOOS check.

See [docs/CONFIGURATION_REFERENCE.md](docs/CONFIGURATION_REFERENCE.md#pluginsdockerexternal) for the `symlink_files` / `symlink_dirs` reference and the cross-link to this section.

## Getting Help

- **Questions**: Open a [Discussion](https://github.com/lost-in-the/grove/discussions)
- **Bugs**: Open an [Issue](https://github.com/lost-in-the/grove/issues)
- **Features**: Open a [Feature Request](https://github.com/lost-in-the/grove/issues/new?template=feature_request.md)

## License

By contributing, you agree that your contributions will be licensed under the Apache 2.0 License.

## Recognition

Contributors will be:
- Listed in CHANGELOG.md for their contributions
- Mentioned in release notes
- Added to CONTRIBUTORS.md (if we create one)

Thank you for contributing to Grove! 🌳
