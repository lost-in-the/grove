# Contributing to Grove

Thank you for your interest in contributing to Grove! This document provides guidelines for contributing to the project.

## Code of Conduct

Be respectful, inclusive, and professional. We're all here to build something great together.

## Getting Started

### Prerequisites

- Go 1.21 or later
- Git 2.30 or later
- Tmux 3.0 or later (for testing tmux features)
- Make

### Development Setup

1. Fork the repository
2. Clone your fork:
   ```bash
   git clone https://github.com/YOUR_USERNAME/grove-cli
   cd grove-cli
   ```
3. Add upstream remote:
   ```bash
   git remote add upstream https://github.com/LeahArmstrong/grove-cli
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
- [ ] CHANGELOG.md updated (for features/fixes)
- [ ] Code formatted (`make fmt`)
- [ ] Linter passes (`make lint`)
- [ ] Conventional commit message used
- [ ] No merge conflicts with `main`

## Development Commands

```bash
make build          # Build the binary
make test           # Run all tests
make test-verbose   # Run tests with verbose output
make test-coverage  # Generate coverage report
make lint           # Run linters
make fmt            # Format code
make clean          # Clean build artifacts
make install        # Install locally
```

## Project Structure

```
grove-cli/
├── cmd/
│   └── grove/
│       ├── main.go           # Entry point
│       └── commands/         # Command implementations
├── internal/
│   ├── config/              # Configuration loading
│   ├── worktree/            # Git worktree operations
│   ├── tmux/                # Tmux session management
│   ├── shell/               # Shell integration
│   ├── hooks/               # Hook system
│   └── version/             # Version info
├── shell/                   # Shell scripts (deprecated)
├── testdata/                # Test fixtures
├── Makefile                 # Build automation
├── go.mod                   # Go module definition
└── README.md                # Documentation
```

## Adding New Features

### For Core Features

1. Discuss in an issue first
2. Update the implementation plan if needed
3. Follow TDD: tests → implementation → refactor
4. Update documentation
5. Add to CHANGELOG.md

### For Plugins (Future)

1. Create plugin in `plugins/` directory
2. Implement hook interfaces
3. Add README.md in plugin directory
4. Register hooks in plugin init
5. Add tests

## Testing

### Unit Tests

Run with:
```bash
go test ./...
```

### Integration Tests

These test real git and tmux operations:
```bash
go test -tags=integration ./...
```

### Coverage

Generate coverage report:
```bash
make test-coverage
open coverage.html
```

## Documentation

- **Public functions**: Must have doc comments
- **Packages**: Should have package-level documentation
- **Complex logic**: Add inline comments explaining "why", not "what"
- **Examples**: Add to README.md for new features

## Getting Help

- **Questions**: Open a [Discussion](https://github.com/LeahArmstrong/grove-cli/discussions)
- **Bugs**: Open an [Issue](https://github.com/LeahArmstrong/grove-cli/issues)
- **Features**: Open a [Feature Request](https://github.com/LeahArmstrong/grove-cli/issues/new?template=feature_request.md)

## License

By contributing, you agree that your contributions will be licensed under the Apache 2.0 License.

## Recognition

Contributors will be:
- Listed in CHANGELOG.md for their contributions
- Mentioned in release notes
- Added to CONTRIBUTORS.md (if we create one)

Thank you for contributing to Grove! 🌳
