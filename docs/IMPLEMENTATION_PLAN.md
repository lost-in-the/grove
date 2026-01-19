# Grove v1.1 — Agent Implementation Plan

**Project:** Grove (worktree + tmux workflow manager)  
**Alias:** `w` (configurable)  
**License:** Apache 2.0 (permissive, patent protection, corporate-friendly for contributions)  
**Language:** Go  
**Repository Structure:** Monorepo with plugin architecture

---

## Table of Contents

1. [Project Overview](#project-overview)
2. [Technical Decisions](#technical-decisions)
3. [Repository Structure](#repository-structure)
4. [Development Philosophy](#development-philosophy)
5. [Claude Code Configuration](#claude-code-configuration)
6. [GitHub Workflow](#github-workflow)
7. [Phase Breakdown](#phase-breakdown)
8. [Task Specifications](#task-specifications)
9. [Testing Requirements](#testing-requirements)
10. [Documentation Requirements](#documentation-requirements)
11. [Open Source Requirements](#open-source-requirements)
12. [Ralph Wiggum Iteration Pattern](#ralph-wiggum-iteration-pattern)
13. [Completion Criteria](#completion-criteria)

### Related Documentation

- **[Command Specifications](docs/COMMAND_SPECIFICATIONS.md)** — Exhaustive behavior specs for every command
- **[Validation Checklist](docs/VALIDATION_CHECKLIST.md)** — Test cases to verify implementation correctness

---

## Project Overview

Grove is a zero-friction worktree + tmux manager for developers. It provides fast context switching between git worktrees with automatic tmux session management.

### Core Value Proposition

- **Single-character command:** `w` (configurable alias)
- **Sub-500ms execution:** Every command completes fast
- **State preservation:** Never lose work, always resumable
- **Modular plugins:** Core is open source, extensions via plugins

### Target Users

- Developers managing multiple concurrent features/bugs
- Teams using git worktrees for isolation
- Anyone who context-switches frequently between tasks

---

## Technical Decisions

### Language: Go

**Rationale:**
- Single binary distribution (no runtime dependencies)
- Trivial cross-compilation: `GOOS=linux GOARCH=amd64 go build`
- Proven shell integration pattern (used by direnv, fzf, lazygit)
- Fast compilation enables rapid iteration
- Strong standard library for CLI tooling

**References:**
- Dave Cheney's "Practical Go" principles: simplicity, readability, productivity
- SOLID principles adapted for Go (interface segregation, single responsibility)
- Table-driven tests as standard practice

### Repository: Monorepo

**Rationale:**
- First-party plugins share internal code
- Atomic releases (core + plugins always compatible)
- Single CI/CD pipeline
- Unified issue tracking

**Structure allows future polyrepo split** if community plugins emerge.

### Configuration: TOML

**Rationale:**
- Explicit typing prevents silent failures
- Parse errors include line numbers
- No YAML gotchas (Norway problem, implicit typing, indentation)
- Familiar to Go developers (Cargo.toml, pyproject.toml patterns)

---

## Repository Structure

```
grove/
├── .github/
│   ├── ISSUE_TEMPLATE/
│   │   ├── bug_report.md
│   │   ├── feature_request.md
│   │   └── task.md
│   ├── PULL_REQUEST_TEMPLATE.md
│   ├── workflows/
│   │   ├── ci.yml
│   │   ├── release.yml
│   │   └── test.yml
│   └── CODEOWNERS
├── .claude/
│   ├── CLAUDE.md
│   ├── CLAUDE.local.md.example
│   ├── settings.json
│   └── commands/
│       ├── commit-push-pr.md
│       ├── test-current.md
│       └── release.md
├── .goreleaser.yml
├── cmd/
│   └── grove/
│       └── main.go
├── internal/
│   ├── config/
│   │   ├── config.go
│   │   ├── config_test.go
│   │   └── defaults.go
│   ├── tmux/
│   │   ├── session.go
│   │   ├── session_test.go
│   │   └── commands.go
│   ├── worktree/
│   │   ├── worktree.go
│   │   ├── worktree_test.go
│   │   └── naming.go
│   ├── shell/
│   │   ├── integration.go
│   │   ├── zsh.go
│   │   └── bash.go
│   └── hooks/
│       ├── hooks.go
│       └── hooks_test.go
├── plugins/
│   ├── docker/
│   │   ├── plugin.go
│   │   ├── plugin_test.go
│   │   └── README.md
│   ├── db/
│   │   ├── plugin.go
│   │   ├── plugin_test.go
│   │   ├── mysql.go
│   │   ├── postgres.go
│   │   └── README.md
│   ├── tracker/
│   │   ├── plugin.go
│   │   ├── github.go
│   │   ├── linear.go
│   │   └── README.md
│   ├── time/
│   │   ├── plugin.go
│   │   └── README.md
│   └── test/
│       ├── plugin.go
│       └── README.md
├── shell/
│   ├── grove.zsh
│   ├── grove.bash
│   └── completions/
│       ├── _grove.zsh
│       └── grove.bash
├── docs/
│   └── (future: mkdocs site)
├── scripts/
│   ├── ralph.sh
│   ├── install.sh
│   └── dev-setup.sh
├── testdata/
│   └── fixtures/
├── CLAUDE.md
├── README.md
├── CONTRIBUTING.md
├── CHANGELOG.md
├── LICENSE
├── go.mod
├── go.sum
└── Makefile
```

---

## Development Philosophy

### Go Best Practices (Dave Cheney)

1. **Simplicity over cleverness**
   - "There are two ways of constructing a software design: One way is to make it so simple that there are obviously no deficiencies"
   - Avoid premature abstraction

2. **Code is written to be read**
   - Prioritize readability over write-time convenience
   - Clear naming over comments

3. **Table-driven tests**
   - Reduce duplication in test code
   - Each test case: input, expected output, name
   - Use subtests for better failure reporting

4. **Interface segregation**
   - "Require no more, promise no less"
   - Small interfaces over large ones
   - Accept interfaces, return concrete types

5. **Error handling**
   - Handle errors explicitly
   - Wrap errors with context
   - Don't panic in library code

### TDD Workflow

For every feature:
1. Write failing test first
2. Implement minimum code to pass
3. Refactor while keeping tests green
4. Commit test + implementation together

---

## Claude Code Configuration

### CLAUDE.md (Project Root)

```markdown
# Grove - Worktree Flow Manager

## Project Context
Grove is a Go CLI tool for managing git worktrees with tmux integration.
Target: developers who context-switch frequently between tasks.

## Build Commands
- `make build` - Build the binary
- `make test` - Run all tests
- `make test-unit` - Run unit tests only
- `make test-integration` - Run integration tests
- `make lint` - Run linters
- `make fmt` - Format code

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
```

### .claude/settings.json

```json
{
  "permissions": {
    "allow": [
      "Bash(go build:*)",
      "Bash(go test:*)",
      "Bash(go fmt:*)",
      "Bash(go vet:*)",
      "Bash(make:*)",
      "Bash(git add:*)",
      "Bash(git commit:*)",
      "Bash(git status)",
      "Bash(git diff:*)",
      "Bash(git log:*)",
      "Bash(gh issue:*)",
      "Bash(gh pr:*)",
      "Read(**/*.go)",
      "Read(**/*.md)",
      "Read(**/*.toml)",
      "Read(**/go.mod)",
      "Read(**/go.sum)",
      "Write(**/*.go)",
      "Write(**/*.md)",
      "Write(**/*_test.go)"
    ],
    "deny": [
      "Bash(rm -rf:*)",
      "Bash(sudo:*)",
      "Read(.env)",
      "Read(~/.ssh/**)"
    ]
  }
}
```

### .claude/commands/test-current.md

```markdown
Run tests for the current package and report results.

```bash
# Get current package
PACKAGE=$(go list ./...)
echo "Testing: $PACKAGE"
```

Run: `go test -v -race ./...`

If tests fail:
1. Identify the failing test
2. Show the relevant test code
3. Show the implementation being tested
4. Suggest fix
```

### .claude/commands/commit-push-pr.md

```markdown
Create a commit, push, and open PR.

```bash
# Get current status
git status --short
git diff --stat
```

1. Review changes and generate conventional commit message
2. Run `make lint test` before committing
3. Commit with message format: `type(scope): description`
4. Push to origin
5. Create PR with description from commit
```

### Subagents

**code-simplifier** - Runs after implementation to reduce complexity:
```markdown
Review the code changes and simplify:
1. Remove unnecessary abstractions
2. Inline functions called only once
3. Reduce nesting depth
4. Eliminate dead code
5. Ensure names are clear and consistent
```

**verify-tests** - Verifies test quality:
```markdown
Review test coverage:
1. Are all public functions tested?
2. Are edge cases covered?
3. Are error paths tested?
4. Are tests table-driven where appropriate?
5. Do test names describe what they test?
```

---

## GitHub Workflow

### Issue Templates

**Bug Report (.github/ISSUE_TEMPLATE/bug_report.md):**
```markdown
---
name: Bug Report
about: Report a bug in grove
labels: bug
---

## Description
[Clear description of the bug]

## Steps to Reproduce
1. 
2. 
3. 

## Expected Behavior
[What should happen]

## Actual Behavior
[What actually happens]

## Environment
- OS: 
- Shell: 
- tmux version: 
- grove version: 

## Additional Context
[Any other relevant information]
```

**Feature Request (.github/ISSUE_TEMPLATE/feature_request.md):**
```markdown
---
name: Feature Request
about: Suggest a new feature
labels: enhancement
---

## Problem
[What problem does this solve?]

## Proposed Solution
[How should it work?]

## Alternatives Considered
[What else did you consider?]

## Additional Context
[Any other relevant information]
```

**Task (.github/ISSUE_TEMPLATE/task.md):**
```markdown
---
name: Implementation Task
about: A specific implementation task
labels: task
---

## Task
[What needs to be done]

## Acceptance Criteria
- [ ] Criterion 1
- [ ] Criterion 2
- [ ] Criterion 3

## Technical Notes
[Implementation guidance]

## Dependencies
[Issues this depends on]
```

### PR Template (.github/PULL_REQUEST_TEMPLATE.md)

```markdown
## Summary
[Brief description of changes]

## Type
- [ ] feat: New feature
- [ ] fix: Bug fix
- [ ] docs: Documentation
- [ ] refactor: Code refactor
- [ ] test: Test changes
- [ ] chore: Maintenance

## Changes
- Change 1
- Change 2

## Testing
- [ ] Unit tests added/updated
- [ ] Integration tests added/updated (if applicable)
- [ ] Manual testing completed

## Checklist
- [ ] Code follows project style guidelines
- [ ] Self-review completed
- [ ] Documentation updated
- [ ] Tests pass locally (`make test`)
- [ ] Linter passes (`make lint`)

## Related Issues
Closes #

## Screenshots (if applicable)
```

### GitHub Project Board

Create project board with columns:
1. **Backlog** - All tasks not yet started
2. **Ready** - Refined and ready to work
3. **In Progress** - Currently being worked on
4. **Review** - PR open, awaiting review
5. **Done** - Merged to main

### Branch Strategy

- `main` - Production-ready code
- `type/description` - Feature/fix branches
  - `feat/core-commands`
  - `fix/tmux-session-cleanup`
  - `docs/readme-update`

---

## Phase Breakdown

### Phase 0: Foundation (Week 1)

**Goal:** Minimal working tool with 6 core commands

**Commands:**
- `w ls` - List worktrees
- `w new <name>` - Create worktree + tmux session
- `w to <name>` - Switch to worktree
- `w rm <name>` - Remove worktree
- `w here` - Show current worktree
- `w last` - Switch to previous worktree

**Deliverables:**
- [ ] Repository setup with full structure
- [ ] CI/CD pipeline (test, lint, build)
- [ ] Core CLI framework (Cobra)
- [ ] Config loading (TOML)
- [ ] Tmux session management
- [ ] Git worktree operations
- [ ] Shell integration (zsh + bash)
- [ ] Tab completion
- [ ] README with installation instructions

**Exit Criteria:**
- All 6 commands work end-to-end
- Tests pass with >80% coverage
- Can be installed via `go install`
- Used for real work for 3+ days

### Phase 1: Docker Plugin (Week 2)

**Goal:** Container lifecycle tied to worktrees

**Commands:**
- `w up` - Start containers
- `w down` - Stop containers
- `w logs [service]` - Tail logs
- `w restart [service]` - Restart service

**Deliverables:**
- [ ] Plugin interface definition
- [ ] Hook system (post-switch, pre-freeze, etc.)
- [ ] Docker plugin implementation
- [ ] Port allocation via direnv generation
- [ ] Plugin documentation

**Exit Criteria:**
- Docker starts/stops automatically on switch
- Port conflicts prevented
- Plugin can be disabled without breaking core

### Phase 2: State Management (Week 3)

**Goal:** True freeze/resume, dirty handling

**Commands:**
- `w freeze [name]` - Freeze worktree
- `w resume <name>` - Resume worktree
- `w repair [name]` - Fix corrupted state
- `w clean` - Remove old/orphaned resources
- `w doctor` - Diagnose issues

**Deliverables:**
- [ ] Freeze/resume with tmux state
- [ ] Dirty worktree detection
- [ ] Switch behavior config (auto-stash, prompt, refuse)
- [ ] Recovery commands
- [ ] State persistence

**Exit Criteria:**
- Can freeze Friday, resume Monday seamlessly
- Dirty handling works as configured
- Recovery from bad states possible

### Phase 3: Time & Testing (Week 4)

**Goal:** Passive time tracking, parallel testing

**Commands:**
- `w time` - Show time per worktree
- `w time week` - Weekly summary
- `w push <tree>` - Copy state to another tree
- `w push <tree> --test` - Push and run tests
- `w peek <tree>` - View test output

**Deliverables:**
- [ ] Time tracking plugin (hook-driven)
- [ ] Test plugin implementation
- [ ] Cross-worktree state copy
- [ ] Notification system

**Exit Criteria:**
- Time logged automatically on switch
- Tests run in background
- Notifications work (macOS initially)

### Phase 4: Issue Integration (Week 5)

**Goal:** Worktrees from issues/PRs

**Commands:**
- `w fetch pr/<num>` - Checkout PR to worktree
- `w fetch is/<num>` - Create worktree from issue
- `w issues` - Browse issues (fzf)
- `w prs` - Browse PRs (fzf)

**Deliverables:**
- [ ] Tracker plugin with adapter pattern
- [ ] GitHub adapter (via `gh` CLI)
- [ ] Linear adapter (optional)
- [ ] Smart naming from issue metadata
- [ ] fzf integration for browsing

**Exit Criteria:**
- Full issue-to-worktree workflow
- Naming follows configured pattern
- Works with GitHub out of box

### Phase 5: Polish (Week 6)

**Goal:** Production-ready for open source

**Deliverables:**
- [ ] TUI exploration mode (optional)
- [ ] Template system for worktree types
- [ ] Database plugin (MySQL + Postgres)
- [ ] Comprehensive documentation
- [ ] Release automation (GoReleaser)
- [ ] Homebrew formula

**Exit Criteria:**
- Ready for public announcement
- Installation works via Homebrew
- Documentation complete

---

## Task Specifications

### Task Format

Each task must include:

```markdown
## Task: [TASK-ID] [Title]

### Description
[What needs to be done]

### Acceptance Criteria
- [ ] Criterion 1 (testable)
- [ ] Criterion 2 (testable)
- [ ] Criterion 3 (testable)

### Technical Approach
[How to implement]

### Test Cases
1. Test case 1: [input] → [expected output]
2. Test case 2: [input] → [expected output]

### Files to Create/Modify
- `path/to/file.go` - [what changes]

### Dependencies
- Depends on: [TASK-ID]
- Blocks: [TASK-ID]

### Definition of Done
- [ ] Tests written (TDD - before implementation)
- [ ] Implementation complete
- [ ] Tests passing
- [ ] Code reviewed (self or peer)
- [ ] Documentation updated
- [ ] Committed with conventional message
```

### Phase 0 Tasks

#### TASK-001: Repository Initialization

**Description:** Set up the repository with full structure, CI/CD, and base configuration.

**Acceptance Criteria:**
- [ ] Repository created with structure from spec
- [ ] go.mod initialized with `github.com/[owner]/grove`
- [ ] Makefile with targets: build, test, lint, fmt, clean
- [ ] GitHub Actions for CI (test on push, lint on PR)
- [ ] .gitignore configured for Go
- [ ] LICENSE file (Apache 2.0)
- [ ] Empty CHANGELOG.md with format instructions

**Test Cases:**
1. `make build` produces binary
2. `make test` runs (even with no tests yet)
3. `make lint` passes on empty project
4. CI triggers on push to any branch

**Files to Create:**
- `go.mod`
- `Makefile`
- `.github/workflows/ci.yml`
- `.gitignore`
- `LICENSE`
- `CHANGELOG.md`
- `cmd/grove/main.go` (minimal)

**Definition of Done:**
- [ ] All files created
- [ ] CI passes on initial commit
- [ ] Can clone and run `make build`

---

#### TASK-002: CLI Framework Setup

**Description:** Implement base CLI using Cobra with root command and help.

**Acceptance Criteria:**
- [ ] Root command shows help when run without args
- [ ] Version flag works (`grove --version`)
- [ ] Help flag works (`grove --help`)
- [ ] Subcommand structure ready for additions

**Test Cases:**
1. `grove` with no args → shows help text
2. `grove --version` → shows version string
3. `grove --help` → shows usage information
4. `grove unknown` → shows error + help

**Technical Approach:**
```go
// cmd/grove/main.go
package main

import (
    "github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
    Use:   "grove",
    Short: "Zero-friction worktree management",
    Long:  `Grove manages git worktrees with tmux integration...`,
}

func main() {
    rootCmd.Execute()
}
```

**Files to Create/Modify:**
- `cmd/grove/main.go`
- `cmd/grove/root.go`
- `cmd/grove/version.go`
- `internal/version/version.go`

**Definition of Done:**
- [ ] Tests for CLI parsing
- [ ] Implementation complete
- [ ] `make build && ./grove --help` works

---

#### TASK-003: Configuration System

**Description:** Implement TOML configuration loading with defaults and validation.

**Acceptance Criteria:**
- [ ] Loads config from `~/.config/grove/config.toml`
- [ ] Falls back to sensible defaults if no config
- [ ] Supports project-level override (`.grove/config.toml`)
- [ ] Config validation with clear error messages
- [ ] `grove config` shows current configuration

**Test Cases:**
1. No config file → uses defaults
2. Valid config file → loads all values
3. Invalid TOML → clear error message with line number
4. Project override → merges with global
5. Invalid value (e.g., negative timeout) → validation error

**Technical Approach:**
```go
// internal/config/config.go
type Config struct {
    Alias           string        `toml:"alias"`
    ProjectsDir     string        `toml:"projects_dir"`
    DefaultBranch   string        `toml:"default_base_branch"`
    Switch          SwitchConfig  `toml:"switch"`
    Naming          NamingConfig  `toml:"naming"`
    Tmux            TmuxConfig    `toml:"tmux"`
}

type SwitchConfig struct {
    DirtyHandling string `toml:"dirty_handling"` // auto-stash, prompt, refuse
}
```

**Files to Create:**
- `internal/config/config.go`
- `internal/config/config_test.go`
- `internal/config/defaults.go`
- `internal/config/validate.go`

**Definition of Done:**
- [ ] Table-driven tests for all scenarios
- [ ] Implementation complete
- [ ] Config subcommand works

---

#### TASK-004: Worktree Operations

**Description:** Implement git worktree create, list, and remove operations.

**Acceptance Criteria:**
- [ ] Can create worktree with specified name
- [ ] Can list all worktrees with status
- [ ] Can remove worktree cleanly
- [ ] Handles errors gracefully (already exists, not found, etc.)
- [ ] Works with both named branches and detached HEAD

**Test Cases:**
1. Create worktree → directory created, git worktree list shows it
2. Create duplicate name → clear error
3. List worktrees → shows all with status (clean/dirty)
4. Remove existing → removes directory and git reference
5. Remove non-existent → clear error

**Technical Approach:**
Use `os/exec` to call git commands. Parse output for status.

**Files to Create:**
- `internal/worktree/worktree.go`
- `internal/worktree/worktree_test.go`
- `internal/worktree/git.go`

**Definition of Done:**
- [ ] Tests with mock git (or real git in temp dir)
- [ ] All CRUD operations work
- [ ] Error messages are helpful

---

#### TASK-005: Tmux Session Management

**Description:** Implement tmux session create, attach, switch, and kill.

**Acceptance Criteria:**
- [ ] Can create named tmux session
- [ ] Can attach to existing session
- [ ] Can switch between sessions
- [ ] Can kill session
- [ ] Handles "not in tmux" gracefully
- [ ] Stores "last session" for `w last`

**Test Cases:**
1. Create session → `tmux list-sessions` shows it
2. Create duplicate → attaches instead of error
3. Attach from outside tmux → new terminal attached
4. Switch within tmux → session changes
5. Kill session → removed from list
6. `w last` → switches to previous

**Technical Approach:**
```go
// internal/tmux/session.go
type Session struct {
    Name      string
    Path      string
    Attached  bool
    Windows   int
}

func (s *Session) Create() error
func (s *Session) Attach() error
func (s *Session) Kill() error
func SwitchTo(name string) error
func Current() (*Session, error)
```

**Files to Create:**
- `internal/tmux/session.go`
- `internal/tmux/session_test.go`
- `internal/tmux/commands.go`

**Definition of Done:**
- [ ] Tests (may need tmux running or mocks)
- [ ] All session operations work
- [ ] Last session tracking works

---

#### TASK-006: Shell Integration

**Description:** Implement shell integration for zsh and bash.

**Acceptance Criteria:**
- [ ] `grove init zsh` outputs shell function
- [ ] `grove init bash` outputs shell function
- [ ] Shell function wraps grove binary
- [ ] Enables directory change after switch
- [ ] Tab completion for worktree names

**Test Cases:**
1. `grove init zsh` → valid zsh code
2. `grove init bash` → valid bash code
3. Sourcing init + running `w to X` → changes directory
4. Tab completion suggests existing worktrees

**Technical Approach:**
```go
// grove init zsh outputs:
grove() {
    local output
    output=$(__grove_bin "$@")
    local exit_code=$?
    
    if [[ -n "$output" && "$output" == "cd:"* ]]; then
        cd "${output#cd:}"
    elif [[ -n "$output" ]]; then
        echo "$output"
    fi
    
    return $exit_code
}
```

**Files to Create:**
- `internal/shell/integration.go`
- `internal/shell/zsh.go`
- `internal/shell/bash.go`
- `shell/grove.zsh`
- `shell/grove.bash`
- `shell/completions/_grove.zsh`
- `shell/completions/grove.bash`

**Definition of Done:**
- [ ] Both shells work
- [ ] Completions work
- [ ] README documents setup

---

#### TASK-007: Core Commands Implementation

**Description:** Implement the 6 core commands: ls, new, to, rm, here, last.

**Acceptance Criteria:**
- [ ] `w ls` - Lists worktrees with status
- [ ] `w new <name>` - Creates worktree + tmux session
- [ ] `w to <name>` - Switches to worktree
- [ ] `w rm <name>` - Removes worktree + session
- [ ] `w here` - Shows current worktree
- [ ] `w last` - Switches to previous worktree
- [ ] All commands complete in <500ms

**Test Cases:**
Per command (table-driven):
1. Valid input → expected behavior
2. Missing argument → helpful error
3. Invalid argument → helpful error
4. Edge cases (no worktrees, already in target, etc.)

**Files to Create:**
- `cmd/grove/ls.go`
- `cmd/grove/new.go`
- `cmd/grove/to.go`
- `cmd/grove/rm.go`
- `cmd/grove/here.go`
- `cmd/grove/last.go`
- Tests for each

**Definition of Done:**
- [ ] All commands work
- [ ] Performance <500ms
- [ ] Help text for each command
- [ ] Errors are actionable

---

#### TASK-008: Hook System Foundation

**Description:** Implement hook system for plugin extensibility.

**Acceptance Criteria:**
- [ ] Hooks can be registered by plugins
- [ ] Hooks fire at appropriate lifecycle points
- [ ] Hook failures don't break core operations (logged, continue)
- [ ] Hooks run in defined order

**Hook Points:**
- `pre-create` - Before worktree creation
- `post-create` - After worktree creation
- `pre-switch` - Before switching away
- `post-switch` - After switching to
- `pre-freeze` - Before freezing
- `post-resume` - After resuming
- `pre-remove` - Before deletion
- `post-remove` - After deletion

**Technical Approach:**
```go
// internal/hooks/hooks.go
type Hook func(ctx *HookContext) error

type HookContext struct {
    Worktree    string
    PrevWorktree string
    Config      *config.Config
}

type Registry struct {
    hooks map[string][]Hook
}

func (r *Registry) Register(event string, hook Hook)
func (r *Registry) Fire(event string, ctx *HookContext) error
```

**Files to Create:**
- `internal/hooks/hooks.go`
- `internal/hooks/hooks_test.go`
- `internal/hooks/registry.go`

**Definition of Done:**
- [ ] Hook registration works
- [ ] Hooks fire at correct times
- [ ] Failure handling tested

---

### Phase 1-5 Tasks

[Similar detailed specifications for each task in phases 1-5. Create GitHub issues for each as work progresses.]

---

## Testing Requirements

### Test Structure

```
package_test.go     # Unit tests
package_integration_test.go  # Integration tests (build tag)
```

### Unit Tests

- **Coverage target:** 80% for `internal/` packages
- **Style:** Table-driven with subtests
- **Mocking:** Interface-based dependency injection
- **Assertions:** Standard library (`testing`) + `testify` for complex assertions

**Example:**
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
            errMsg:  "worktree name cannot be empty",
        },
        // ... more cases
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            // Setup
            // Execute
            // Assert
        })
    }
}
```

### Integration Tests

- Build tag: `//go:build integration`
- Run with: `go test -tags=integration ./...`
- Test real tmux, real git operations
- Use temp directories, clean up after

### Test Commands

```makefile
test:           ## Run all tests
	go test -race -cover ./...

test-unit:      ## Run unit tests only
	go test -short ./...

test-integration: ## Run integration tests
	go test -tags=integration -race ./...

test-coverage:  ## Generate coverage report
	go test -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html
```

---

## Documentation Requirements

### README.md Structure

```markdown
# Grove

Zero-friction worktree management for developers.

## Features
[Bullet list of key features]

## Installation

### Homebrew (macOS/Linux)
```bash
brew install [owner]/tap/grove
```

### Go Install
```bash
go install github.com/[owner]/grove@latest
```

### From Source
```bash
git clone https://github.com/[owner]/grove
cd grove
make install
```

## Quick Start
[5-minute getting started guide]

## Shell Setup
[Instructions for zsh/bash]

## Commands
[Command reference with examples]

## Configuration
[Config file format and options]

## Plugins
[Available plugins and how to use]

## Contributing
See [CONTRIBUTING.md](CONTRIBUTING.md)

## License
Apache 2.0 - see [LICENSE](LICENSE)
```

### Inline Documentation

- All exported functions must have doc comments
- Package-level doc in `doc.go` for each package
- Examples in `example_test.go` where helpful

```go
// Worktree represents a git worktree managed by grove.
// It tracks the worktree's path, associated tmux session,
// and current state (frozen, dirty, etc.).
type Worktree struct {
    // Name is the short name used in grove commands.
    Name string
    
    // Path is the absolute filesystem path to the worktree.
    Path string
    
    // ...
}

// Create creates a new worktree with the given name.
// It returns an error if a worktree with that name already exists
// or if the git operation fails.
func Create(name string, opts CreateOptions) (*Worktree, error) {
    // ...
}
```

---

## Open Source Requirements

### CONTRIBUTING.md

```markdown
# Contributing to Grove

## Code of Conduct
[Link to CODE_OF_CONDUCT.md or inline]

## Getting Started

### Prerequisites
- Go 1.21+
- tmux 3.0+
- git 2.30+

### Development Setup
```bash
git clone https://github.com/[owner]/grove
cd grove
make dev-setup
make test
```

## Making Changes

### Branch Naming
- `feat/description` - New features
- `fix/description` - Bug fixes
- `docs/description` - Documentation
- `refactor/description` - Code refactoring
- `test/description` - Test changes

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
feat(core): add w last command for quick switching
fix(tmux): handle session names with spaces
docs(readme): add homebrew installation instructions
```

### Pull Request Process

1. Fork the repository
2. Create your branch (`git checkout -b feat/amazing-feature`)
3. Write tests first (TDD)
4. Implement your changes
5. Ensure tests pass (`make test`)
6. Ensure linter passes (`make lint`)
7. Commit with conventional message
8. Push to your fork
9. Open PR against `main`

### PR Checklist
- [ ] Tests added/updated
- [ ] Documentation updated
- [ ] CHANGELOG.md updated (for features/fixes)
- [ ] Conventional commit message
- [ ] CI passes

## Code Style

- Run `make fmt` before committing
- Follow [Effective Go](https://golang.org/doc/effective_go)
- Follow [Dave Cheney's Practical Go](https://dave.cheney.net/practical-go)

## Testing

- Write tests first (TDD)
- Use table-driven tests
- Aim for 80%+ coverage
- Test error paths, not just happy paths

## Questions?

Open an issue or discussion!
```

### CHANGELOG.md Format

```markdown
# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added
- 

### Changed
- 

### Fixed
- 

## [0.1.0] - YYYY-MM-DD

### Added
- Initial release
- Core commands: ls, new, to, rm, here, last
- Shell integration for zsh and bash
- TOML configuration
- tmux session management
```

---

## Ralph Wiggum Iteration Pattern

### Bash Loop Approach

The original Ralph Wiggum pattern uses a bash loop that starts fresh Claude Code sessions for each iteration. This is preferred for long-running tasks because each iteration gets a fresh context window.

### scripts/ralph.sh

```bash
#!/usr/bin/env bash
set -euo pipefail

# Ralph Wiggum iteration loop for grove development
# Usage: ./scripts/ralph.sh "prompt" [max_iterations]

PROMPT="$1"
MAX_ITERATIONS="${2:-10}"
COMPLETION_PROMISE="${3:-PHASE_COMPLETE}"
ITERATION=0

# State files
PROGRESS_FILE="progress.md"
ACTIVITY_FILE="activity.md"

# Initialize progress tracking
if [[ ! -f "$PROGRESS_FILE" ]]; then
    cat > "$PROGRESS_FILE" << 'EOF'
# Grove Development Progress

## Current Phase
Phase 0: Foundation

## Tasks
- [ ] TASK-001: Repository Initialization
- [ ] TASK-002: CLI Framework Setup
- [ ] TASK-003: Configuration System
- [ ] TASK-004: Worktree Operations
- [ ] TASK-005: Tmux Session Management
- [ ] TASK-006: Shell Integration
- [ ] TASK-007: Core Commands Implementation
- [ ] TASK-008: Hook System Foundation

## Blockers
None

## Notes
EOF
fi

# Initialize activity log
if [[ ! -f "$ACTIVITY_FILE" ]]; then
    echo "# Activity Log" > "$ACTIVITY_FILE"
    echo "" >> "$ACTIVITY_FILE"
fi

echo "🐛 Starting Ralph Wiggum loop"
echo "   Prompt: $PROMPT"
echo "   Max iterations: $MAX_ITERATIONS"
echo "   Completion promise: $COMPLETION_PROMISE"
echo ""

while [[ $ITERATION -lt $MAX_ITERATIONS ]]; do
    ITERATION=$((ITERATION + 1))
    TIMESTAMP=$(date '+%Y-%m-%d %H:%M:%S')
    
    echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
    echo "🔄 Iteration $ITERATION of $MAX_ITERATIONS"
    echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
    
    # Log iteration start
    echo "" >> "$ACTIVITY_FILE"
    echo "## Iteration $ITERATION - $TIMESTAMP" >> "$ACTIVITY_FILE"
    
    # Build the full prompt
    FULL_PROMPT="@progress.md @activity.md @CLAUDE.md

$PROMPT

IMPORTANT:
1. First read progress.md to see current state
2. Read activity.md to see what was recently done
3. Pick ONE unchecked task from progress.md
4. Work on that task using TDD (tests first)
5. Update progress.md when task complete (check the box)
6. Log what you did in activity.md
7. If all tasks in current phase complete, output: <promise>$COMPLETION_PROMISE</promise>
8. If blocked, document blocker in progress.md and continue with different task

Current iteration: $ITERATION of $MAX_ITERATIONS"

    # Run Claude Code with the prompt
    # Using --print to capture output, -p for non-interactive
    OUTPUT=$(claude -p "$FULL_PROMPT" 2>&1) || true
    
    # Log output summary
    echo "Output summary logged" >> "$ACTIVITY_FILE"
    
    # Check for completion promise
    if echo "$OUTPUT" | grep -q "<promise>$COMPLETION_PROMISE</promise>"; then
        echo ""
        echo "✅ Completion promise found!"
        echo "   Ralph finished in $ITERATION iterations"
        
        # Log completion
        echo "" >> "$ACTIVITY_FILE"
        echo "### PHASE COMPLETE at iteration $ITERATION" >> "$ACTIVITY_FILE"
        
        exit 0
    fi
    
    # Check for blocked state
    if echo "$OUTPUT" | grep -q "<promise>BLOCKED</promise>"; then
        echo ""
        echo "⚠️  Ralph is blocked"
        echo "   Check progress.md for blockers"
        
        # Continue to next iteration to try different task
    fi
    
    echo ""
    echo "Iteration $ITERATION complete, continuing..."
    sleep 2  # Brief pause between iterations
done

echo ""
echo "⏱️  Max iterations reached ($MAX_ITERATIONS)"
echo "   Check progress.md for current state"
exit 1
```

### Usage

```bash
# Phase 0: Foundation
./scripts/ralph.sh "Implement Phase 0 of grove. Follow TDD, create GitHub issues for each task as you start them." 20 PHASE0_COMPLETE

# Phase 1: Docker Plugin
./scripts/ralph.sh "Implement Phase 1 of grove (Docker plugin). Follow TDD." 25 PHASE1_COMPLETE

# Single iteration (HITL mode)
./scripts/ralph.sh "Work on the next unchecked task in progress.md" 1
```

### progress.md (Tracked by Ralph)

```markdown
# Grove Development Progress

## Current Phase
Phase 0: Foundation

## Tasks
- [x] TASK-001: Repository Initialization
- [x] TASK-002: CLI Framework Setup
- [ ] TASK-003: Configuration System
- [ ] TASK-004: Worktree Operations
- [ ] TASK-005: Tmux Session Management
- [ ] TASK-006: Shell Integration
- [ ] TASK-007: Core Commands Implementation
- [ ] TASK-008: Hook System Foundation

## Blockers
None currently

## Notes
- Using Cobra for CLI framework
- TOML config with BurntSushi/toml
```

### activity.md (Iteration Log)

```markdown
# Activity Log

## Iteration 1 - 2024-01-15 09:00:00
- Created repository structure
- Initialized go.mod
- Set up GitHub Actions CI
- Created initial Makefile

## Iteration 2 - 2024-01-15 09:15:00
- Implemented CLI framework with Cobra
- Added root command with help
- Added version command
- Tests passing
```

---

## Completion Criteria

### Phase 0 Complete When:

- [ ] All 8 tasks checked off in progress.md
- [ ] `make test` passes with >80% coverage
- [ ] `make lint` passes with no errors
- [ ] All 6 core commands work end-to-end
- [ ] Shell integration works in zsh and bash
- [ ] README has installation + quick start
- [ ] CONTRIBUTING.md complete
- [ ] GitHub issues created for Phase 1 tasks
- [ ] Used for real work for 3+ days
- [ ] No critical bugs in issue tracker

### Project Complete When:

- [ ] All 5 phases complete
- [ ] Documentation site live (optional)
- [ ] Homebrew formula published
- [ ] GoReleaser automated releases working
- [ ] At least 3 real users (besides author)
- [ ] No P0 bugs open for 1 week

---

## Agent Instructions Summary

1. **Always start by reading:** `CLAUDE.md`, `progress.md`, `activity.md`
2. **Follow TDD:** Write tests before implementation
3. **One task at a time:** Pick one unchecked task, complete it fully
4. **Update tracking files:** Check off tasks, log activity
5. **Create GitHub issues:** As you start each task
6. **Conventional commits:** `type(scope): description`
7. **Run checks before commit:** `make lint test`
8. **Signal completion:** Output `<promise>PHASE_COMPLETE</promise>` when phase done
9. **Signal blockers:** Output `<promise>BLOCKED</promise>` with explanation if stuck
10. **Keep iterations focused:** Don't try to do everything at once

---

## Quick Reference

### Make Targets
```
make build      # Build binary
make test       # Run tests
make lint       # Run linter
make fmt        # Format code
make clean      # Clean build artifacts
make install    # Install locally
make release    # Create release (requires goreleaser)
```

### Key Files
```
CLAUDE.md           # Claude Code instructions
progress.md         # Current progress tracking
activity.md         # Iteration activity log
go.mod              # Go module definition
Makefile            # Build automation
```

### Conventional Commits
```
feat(scope):     # New feature
fix(scope):      # Bug fix
docs(scope):     # Documentation
test(scope):     # Tests
refactor(scope): # Refactoring
chore(scope):    # Maintenance
```
