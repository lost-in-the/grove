# Tracker Plugin

The tracker plugin provides integration with issue tracking systems, enabling worktree creation from issues and pull requests.

## Features

- **Issue/PR Fetching**: Create worktrees directly from GitHub issues and PRs
- **Smart Naming**: Automatically generate worktree names from issue/PR metadata
- **Interactive Browsing**: Use fzf to browse and select issues/PRs
- **Adapter Pattern**: Extensible architecture supporting multiple trackers

## Supported Trackers

### GitHub

The GitHub adapter uses the `gh` CLI tool to interact with GitHub repositories.

**Requirements:**
- `gh` CLI installed and authenticated
- Repository with GitHub remote

**Installation:**
```bash
# Install gh CLI
brew install gh  # macOS
# Or visit https://cli.github.com/

# Authenticate
gh auth login
```

## Commands

### Fetch from Issue/PR

Create a worktree from a specific issue or pull request:

```bash
# Fetch PR
grove fetch pr/123

# Fetch issue
grove fetch issue/456
grove fetch is/456  # Shorthand
```

**Behavior:**
- For PRs: Checks out the PR branch
- For issues: Creates a new branch named after the issue
- Worktree name: `{type}-{number}-{slug}`
  - Example: `pr-123-fix-authentication-bug`
- Automatically creates tmux session
- Provides cd directive for shell integration

### Browse Issues

Interactively browse and select issues using fzf:

```bash
# Browse all open issues
grove issues

# Filter by state
grove issues --state all
grove issues --state closed

# Filter by label
grove issues --label bug
grove issues --label bug --label critical

# Filter by assignee/author
grove issues --assignee username
grove issues --author contributor

# Limit results
grove issues --limit 50
```

**Interface:**
- Use arrow keys to navigate
- Press Enter to select
- Press Ctrl-C to cancel
- Selected issue creates a new worktree

### Browse Pull Requests

Interactively browse and select PRs using fzf:

```bash
# Browse all open PRs
grove prs

# Filter by state
grove prs --state all
grove prs --state merged

# Filter by label
grove prs --label feature
grove prs --label needs-review

# Filter by assignee/author
grove prs --assignee username
grove prs --author contributor

# Limit results
grove prs --limit 50
```

**Interface:**
- Use arrow keys to navigate
- Press Enter to select and create worktree
- Press Ctrl-C to cancel
- Shows branch name for each PR

## Worktree Naming

Worktree names are automatically generated from issue/PR metadata:

**Format:** `{type}-{number}-{slug}`

**Examples:**
- Issue #123 "Fix authentication bug" → `issue-123-fix-authentication-bug`
- PR #456 "Add user API" → `pr-456-add-user-api`
- Issue #789 "Update dependencies to v2.0" → `issue-789-update-dependencies-to-v20`

**Slug Generation:**
- Converts title to lowercase
- Replaces spaces with hyphens
- Removes special characters
- Truncates to 40 characters max
- Removes trailing hyphens

## Architecture

### Tracker Interface

```go
type Tracker interface {
    Name() string
    FetchIssue(number int) (*Issue, error)
    FetchPR(number int) (*PullRequest, error)
    ListIssues(opts ListOptions) ([]*Issue, error)
    ListPRs(opts ListOptions) ([]*PullRequest, error)
}
```

### Registration

Trackers are registered at startup:

```go
import "github.com/lost-in-the/grove/plugins/tracker"

// Register a tracker
tracker.Register("github", NewGitHubAdapter())

// Get a registered tracker
t, err := tracker.Get("github")

// List all trackers
names := tracker.List()
```

### Adding a New Tracker

To add support for another issue tracker (e.g., Linear, Jira):

1. Create a new file (e.g., `linear.go`)
2. Implement the `Tracker` interface
3. Register the tracker in the plugin init
4. Update documentation

Example:

```go
type LinearAdapter struct {
    // ...
}

func (l *LinearAdapter) Name() string {
    return "linear"
}

func (l *LinearAdapter) FetchIssue(number int) (*Issue, error) {
    // Implementation using Linear API
}

// ... implement other methods

// In init or main:
tracker.Register("linear", NewLinearAdapter())
```

## Data Types

### Issue

```go
type Issue struct {
    Number    int
    Title     string
    Body      string
    State     string      // "open", "closed"
    Author    string
    Labels    []string
    CreatedAt time.Time
    UpdatedAt time.Time
    URL       string
}
```

### PullRequest

```go
type PullRequest struct {
    Number     int
    Title      string
    Body       string
    State      string      // "open", "closed", "merged"
    Author     string
    Labels     []string
    Branch     string      // PR branch name
    BaseBranch string      // Target branch
    CreatedAt  time.Time
    UpdatedAt  time.Time
    URL        string
}
```

### ListOptions

```go
type ListOptions struct {
    State    string      // "open", "closed", "all"
    Labels   []string    // Filter by labels
    Assignee string      // Filter by assignee
    Author   string      // Filter by author
    Limit    int         // Max results (default: 30)
}
```

## Performance

All tracker operations aim for <500ms completion:

- `fetch pr/123`: ~200ms (gh CLI call + git operations)
- `fetch issue/456`: ~150ms (gh CLI call + git operations)
- `grove issues`: ~300ms (gh CLI list + fzf startup)
- `grove prs`: ~300ms (gh CLI list + fzf startup)

## Error Handling

The tracker plugin provides clear error messages:

```
✗ gh CLI not installed or not authenticated

Install: https://cli.github.com/
Authenticate: gh auth login
```

```
✗ fzf not installed

Install: https://github.com/junegunn/fzf#installation
```

```
✗ failed to detect repository

Make sure you're in a git repository with a GitHub remote
```

## Examples

### Complete Workflow

```bash
# Browse issues
grove issues --label bug

# Select issue #123 from fzf
# → Creates worktree: issue-123-fix-auth-bug
# → Creates branch: issue-123-fix-auth-bug
# → Creates tmux session: grove-issue-123-fix-auth-bug
# → Changes directory (with shell integration)

# Make changes, commit, push
git commit -m "fix: resolve authentication bug"
git push -u origin issue-123-fix-auth-bug

# Create PR
gh pr create

# Clean up
grove rm issue-123-fix-auth-bug
```

### PR Review Workflow

```bash
# Browse PRs needing review
grove prs --label needs-review

# Select PR #456 from fzf
# → Creates worktree: pr-456-add-user-api
# → Checks out branch: add-user-api
# → Creates tmux session: grove-pr-456-add-user-api

# Review, test, comment
grove to main  # Switch back to main

# Clean up after merge
grove rm pr-456-add-user-api
```

## Limitations

- Currently only GitHub is supported
- Requires `gh` CLI and `fzf` to be installed
- Relies on git remote URL for repo detection
- PRs must be fetchable (public or have access)


## Testing

Run tests:

```bash
# Unit tests
make test

# Integration tests (requires gh CLI)
go test -v ./plugins/tracker/...
```

Note: Integration tests require:
- `gh` CLI authenticated
- Access to a GitHub repository
- Internet connection

## Troubleshooting

**Issue: "gh CLI not installed"**
- Install: `brew install gh` or visit https://cli.github.com/
- Authenticate: `gh auth login`

**Issue: "fzf not installed"**
- Install: `brew install fzf` or visit https://github.com/junegunn/fzf

**Issue: "failed to detect repository"**
- Ensure you're in a git repository: `git status`
- Ensure GitHub remote exists: `git remote -v`
- Try: `gh repo view` to test detection

**Issue: "failed to create worktree from branch"**
- PR branch may not exist remotely
- Try fetching manually: `git fetch origin`
- Check PR is open and branch exists

## License

Apache 2.0 - See LICENSE file in repository root.
