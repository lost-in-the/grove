# Grove CLI Command Specification v3

## Overview

Grove is a worktree + tmux orchestration tool for Rails development. It manages the lifecycle of git worktrees with integrated Docker container isolation and GitHub workflow integration.

**Philosophy:** Worktrees are workspaces, branches are version control. Grove manages workspaces; branch creation is explicit via `--branch` flag.

---

## Directory Structure

After setup, a grove-managed project looks like:

```
~/projects/
  myapp/                         # Original clone (root worktree)
    .git/                        # The actual git directory
    .grove/                      # Grove project state
      config.yml                 # Project-specific config
      state.json                 # Frozen status, last session, etc.
    .envrc                       # Environment (COMPOSE_PROJECT_NAME, etc.)
    app/
    ...

  myapp-testing/                 # Worktree: dedicated test runner
    .git → ../myapp/.git         # Linked to shared git directory
    .envrc                       
    app/
    ...

  myapp-feature-auth/            # Worktree: feature work
    .git → ../myapp/.git
    .envrc
    app/
    ...
```

**Naming convention:** `{project}-{worktree-name}`

The root worktree (original clone) uses the project name alone (`myapp`). All other worktrees are sibling directories with the project name as prefix.

---

## Naming Rules

When creating worktrees, grove transforms names:

- Lowercase only
- Alphanumeric and hyphens only
- Spaces converted to hyphens
- Special characters stripped
- Maximum 50 characters (truncated with warning)

**Examples:**
| Input | Output |
|-------|--------|
| `auth` | `myapp-auth` |
| `Feature Auth` | `myapp-feature-auth` |
| `fix/login-bug` | `myapp-fix-login-bug` |
| `PR #123` | `myapp-pr-123` |

---

## Configuration

### Global Configuration

Location: `~/.config/grove/config.yml`

```yaml
# Default behaviors
switch:
  warn_dirty: true               # Warn when leaving dirty worktree

# Docker defaults
docker:
  enabled: true
  auto_up: false                 # Start containers on switch
  auto_down_on_freeze: true      # Stop containers when freezing

# GitHub CLI
github:
  default_worktree: scratch      # Where to checkout PRs
```

### Project Configuration

Location: `{project}/.grove/config.yml`

Overrides global config for this project.

```yaml
# Project identity
project: myapp                   # Used in worktree naming
default_branch: main             # Base for new worktrees/branches
```

---

## Command Reference

### Setup & Configuration

---

#### `grove setup`

Initialize grove in the current repository.

**Usage:**
```
grove setup [flags]
```

**Flags:**
| Flag | Description |
|------|-------------|
| `--with-testing` | Also create a `testing` worktree |
| `--with-scratch` | Also create a `scratch` worktree |
| `--full` | Create `testing`, `scratch`, and `hotfix` worktrees |

**Behavior:**

1. Verify current directory is a git repository
2. Detect project name from directory or remote
3. Create `.grove/` directory with default config
4. Generate `.envrc` for root worktree
5. Add `.grove/state.json` to `.gitignore`
6. If flags provided, create additional worktrees

**Note:** Existing worktrees (created manually via git) are not automatically adopted. Use grove commands to create new worktrees.

**Output:**
```
✓ Initialized grove in /home/dev/projects/myapp
  Project: myapp
  Root worktree: myapp (main)
  Config: .grove/config.yml

Next steps:
  grove new <name>             Create a worktree
  grove new <name> --branch    Create a worktree with new branch
  grove config --edit          Customize configuration
```

**Exit codes:**
- 0: Success
- 1: Not a git repository
- 2: Already initialized

---

#### `grove config`

View or edit configuration.

**Usage:**
```
grove config [flags]
grove config set <key> <value>
```

**Subcommands:**
| Subcommand | Description |
|------------|-------------|
| (none) | Show effective configuration |
| `set <key> <value>` | Set a configuration value |

**Flags:**
| Flag | Description |
|------|-------------|
| `--edit` | Open config file in `$EDITOR` |
| `--global` | Target global config |
| `--project` | Target project config |

**Output (no flags):**
```
Grove Configuration (myapp)
═══════════════════════════════════════════════════════

Source: ~/.config/grove/config.yml (global)
        ~/projects/myapp/.grove/config.yml (project)

Project
  name:           myapp
  default_branch: main
  root:           ~/projects/myapp

Switch
  warn_dirty:     true

Docker
  enabled:        true
  auto_up:        false
```

**Examples:**
```bash
grove config                           # Show all
grove config --edit                    # Edit project config
grove config --edit --global           # Edit global config
grove config set docker.auto_up true   # Set value
```

---

### Worktree Commands

---

#### `grove ls`

List all worktrees for the current project.

**Usage:**
```
grove ls [flags]
```

**Flags:**
| Flag | Description |
|------|-------------|
| `-a, --all` | Include frozen worktrees |
| `-p, --paths` | Output paths only (scriptable) |
| `-j, --json` | Output as JSON |
| `-q, --quiet` | Names only |

**Output (default):**
```
WORKTREES (myapp)
═══════════════════════════════════════════════════════════════════════════

  NAME            BRANCH          STATUS     DOCKER     PATH
  ─────────────────────────────────────────────────────────────────────────
• myapp           main            clean      running    ~/projects/myapp
  testing         main            clean      stopped    ~/projects/myapp-testing
  feature-auth    feature/auth    ✗ dirty    running    ~/projects/myapp-feature-auth
  (frozen) old    feature/old     clean      stopped    ~/projects/myapp-old

• = current worktree
✗ dirty = uncommitted changes
```

**Output (--paths):**
```
/home/dev/projects/myapp
/home/dev/projects/myapp-testing
/home/dev/projects/myapp-feature-auth
```

**Output (--json):**
```json
{
  "project": "myapp",
  "current": "myapp",
  "worktrees": [
    {
      "name": "myapp",
      "path": "/home/dev/projects/myapp",
      "branch": "main",
      "git_status": "clean",
      "docker_status": "running",
      "frozen": false,
      "current": true,
      "root": true
    }
  ]
}
```

**Git status values:** `clean`, `dirty`, `conflict`, `detached`

**Docker status values:** `running`, `stopped`, `partial`

---

#### `grove new`

Create a new worktree.

**Usage:**
```
grove new <name> [flags]
```

**Arguments:**
| Argument | Required | Description |
|----------|----------|-------------|
| `name` | Yes | Name for the worktree |

**Flags:**
| Flag | Description |
|------|-------------|
| `--branch [name]` | Create a new branch. If no name given, uses worktree name. |
| `--no-switch` | Don't switch to the new worktree after creation |

**Behavior:**

1. Validate worktree name (apply naming rules)
2. Check worktree doesn't already exist
3. Determine branch:
   - If `--branch`: create new branch (error if exists)
   - If `--branch <name>`: create branch with that name
   - Otherwise: checkout default branch (main)
4. Create worktree at `../{project}-{name}/`
5. Generate `.envrc` with COMPOSE_PROJECT_NAME
6. Create tmux session named `{project}-{name}`
7. Switch to new worktree (unless `--no-switch`)

**Examples:**
```bash
grove new testing
# Creates worktree 'testing' on main branch
# For: utility worktrees, test runners, scratch space

grove new auth --branch
# Creates worktree 'auth' with new branch 'auth'
# For: starting feature work

grove new auth --branch feature/authentication
# Creates worktree 'auth' with new branch 'feature/authentication'
# For: when you want a different branch name than worktree name
```

**Output:**
```
Creating worktree 'auth'...

✓ Created worktree at ~/projects/myapp-auth
✓ Created branch 'auth' from 'main'
✓ Generated .envrc
✓ Created tmux session 'myapp-auth'
✓ Switched to 'auth'
```

**Edge cases:**

| Scenario | Behavior |
|----------|----------|
| Worktree name exists | Error: "Worktree 'X' already exists. Use `grove to X` to switch." |
| `--branch` and branch exists | Error: "Branch 'X' already exists." |
| Invalid name characters | Auto-transform with warning |

**Exit codes:**
- 0: Success
- 1: Worktree already exists
- 2: Branch already exists
- 3: Git error
- 4: Invalid input

---

#### `grove to`

Switch to an existing worktree.

**Usage:**
```
grove to <name>
```

**Arguments:**
| Argument | Required | Description |
|----------|----------|-------------|
| `name` | Yes | Worktree name to switch to |

**Behavior:**

1. Verify worktree exists
2. If current worktree is dirty and `switch.warn_dirty` is true: show warning (non-blocking)
3. Record current worktree as "last" (for `grove last`)
4. If inside tmux: switch to target worktree's tmux session
5. If outside tmux: output shell directive to cd
6. If worktree was frozen: automatically resume (start containers)

**Output (clean switch):**
```
Switched to 'testing'
```

**Output (leaving dirty worktree):**
```
Note: Leaving 3 uncommitted files in 'feature-auth'
  M  app/models/user.rb
  M  app/controllers/sessions_controller.rb
  A  app/services/auth_service.rb

Switched to 'testing'
```

**Output (resuming frozen):**
```
Resuming frozen worktree 'old-feature'...
✓ Started containers
Switched to 'old-feature'
```

**Edge cases:**

| Scenario | Behavior |
|----------|----------|
| Worktree doesn't exist | Error: "Worktree 'X' not found. See `grove ls`." |
| Already in target | "Already in 'X'" (exit 0) |
| Worktree directory missing | Error: "Worktree 'X' directory missing. Run `grove repair`." |

---

#### `grove last`

Switch to the previously active worktree. Like `cd -` for worktrees.

**Usage:**
```
grove last
```

**Behavior:**

1. Read last worktree from `.grove/state.json`
2. Switch to it (same as `grove to`)

**Note:** This is project-scoped. You must be inside a grove project for it to work.

**Output:**
```
Switched to 'feature-auth' (previous)
```

**Edge cases:**

| Scenario | Behavior |
|----------|----------|
| No previous worktree | Error: "No previous worktree. Use `grove to <name>` first." |
| Previous was deleted | Error: "Previous worktree 'X' no longer exists." |

---

#### `grove here`

Show information about the current worktree.

**Usage:**
```
grove here [flags]
```

**Flags:**
| Flag | Description |
|------|-------------|
| `-q, --quiet` | Output worktree name only |
| `-j, --json` | Output as JSON |

**Output (default):**
```
Current Worktree: feature-auth
═══════════════════════════════════════════════════════

Path:       ~/projects/myapp-feature-auth
Branch:     feature/auth
Base:       main (3 commits ahead, 2 behind)

Git Status: ✗ dirty
  M  app/models/user.rb
  M  app/controllers/sessions_controller.rb
  A  app/services/auth_service.rb

Docker:     running
  web:      running (0:45:23)
  mysql:    running (0:45:20)
  redis:    running (0:45:20)
```

**Output (--quiet):**
```
feature-auth
```

---

#### `grove rm`

Remove a worktree and its associated resources.

**Usage:**
```
grove rm <name> [flags]
```

**Arguments:**
| Argument | Required | Description |
|----------|----------|-------------|
| `name` | Yes | Worktree to remove |

**Flags:**
| Flag | Description |
|------|-------------|
| `--force` | Remove even if worktree is dirty |
| `--keep-branch` | Don't delete the associated branch |

**Behavior:**

1. Verify worktree exists
2. Check if current worktree → Error (must switch away first)
3. Check if root worktree → Error (cannot remove root)
4. Check if dirty → Error unless `--force`
5. Stop Docker containers
6. Kill tmux session
7. Remove git worktree
8. Delete branch (unless `--keep-branch` or branch has unpushed commits)
9. Remove directory

**Output:**
```
Removing worktree 'old-feature'...

✓ Stopped containers
✓ Killed tmux session
✓ Removed worktree
✓ Deleted branch 'old-feature'

Removed 'old-feature'
```

**Edge cases:**

| Scenario | Behavior |
|----------|----------|
| Current worktree | Error: "Cannot remove current worktree. Switch away first." |
| Root worktree | Error: "Cannot remove root worktree." |
| Dirty worktree | Error with file list. Suggest `--force`. |
| Unpushed commits | Warning: "Branch has unpushed commits. Keeping branch." |

---

#### `grove fork`

Create a new worktree from the current state for A/B experimentation.

**Usage:**
```
grove fork <name> [flags]
```

**Arguments:**
| Argument | Required | Description |
|----------|----------|-------------|
| `name` | Yes | Name for the new worktree |

**Flags:**
| Flag | Description |
|------|-------------|
| `--no-switch` | Stay in current worktree |

**Behavior:**

1. Get current HEAD commit
2. Create new branch: `{current-branch}-{name}`
3. Create worktree checking out new branch
4. Copy uncommitted changes to new worktree
5. Generate .envrc, create tmux session
6. Switch to fork (unless `--no-switch`)

**Output:**
```
Forking current work to 'v2'...

Current: feature/auth @ abc1234

✓ Created branch 'feature/auth-v2' from abc1234
✓ Created worktree at ~/projects/myapp-feature-auth-v2
✓ Copied uncommitted changes (3 files)
✓ Generated .envrc
✓ Switched to 'feature-auth-v2'

You now have two worktrees:
  feature-auth      feature/auth       (original)
  feature-auth-v2   feature/auth-v2    (fork with your changes)
```

**Use case:**

You're working on approach A. Want to try approach B without losing A.

```bash
# In feature-auth, half-done work
grove fork v2

# Now you have:
# feature-auth/     → original (clean, your changes moved to fork)
# feature-auth-v2/  → fork (has your uncommitted changes)

# Try different approaches, compare:
grove compare feature-auth

# Keep whichever works, delete the other
grove rm feature-auth-v2
```

---

#### `grove compare`

Show diff between current worktree and another, including uncommitted changes.

**Usage:**
```
grove compare <name> [flags]
```

**Arguments:**
| Argument | Required | Description |
|----------|----------|-------------|
| `name` | Yes | Worktree to compare against |

**Flags:**
| Flag | Description |
|------|-------------|
| `--stat` | Show diffstat only |
| `--committed` | Compare commits only (ignore uncommitted) |

**Behavior:**

1. Get working tree state of current worktree
2. Get working tree state of target worktree
3. Diff including uncommitted changes in both

**Output (default):**
```
Comparing feature-auth-v2 ↔ feature-auth

Both have uncommitted changes.

diff --git a/app/models/user.rb b/app/models/user.rb
--- a/feature-auth-v2:app/models/user.rb
+++ b/feature-auth:app/models/user.rb
@@ -10,6 +10,8 @@ class User < ApplicationRecord
   def authenticate(password)
-    BCrypt::Password.new(password_digest) == password
+    return false if locked?
+    BCrypt::Password.new(password_digest) == password
   end
...
```

**Output (--stat):**
```
Comparing feature-auth-v2 ↔ feature-auth

 app/models/user.rb                    | 15 +++++++++------
 app/services/auth_service.rb          | 42 ++++++++++++++++++++++++++++
 app/controllers/sessions_controller.rb|  8 ++++----
 3 files changed, 55 insertions(+), 10 deletions(-)
```

---

#### `grove freeze`

Freeze a worktree (stop containers, mark as frozen, hide from default listings).

**Usage:**
```
grove freeze [name]
```

**Arguments:**
| Argument | Required | Description |
|----------|----------|-------------|
| `name` | No | Worktree to freeze (default: current) |

**Behavior:**

1. Stop Docker containers
2. Mark worktree as frozen in state

**Output:**
```
Freezing 'feature-auth'...

✓ Stopped containers
✓ Marked as frozen

Worktree 'feature-auth' is now frozen.
It won't appear in `grove ls` (use `grove ls --all`).
Resume with: grove resume feature-auth
```

---

#### `grove resume`

Resume a frozen worktree.

**Usage:**
```
grove resume <name>
```

**Arguments:**
| Argument | Required | Description |
|----------|----------|-------------|
| `name` | Yes | Frozen worktree to resume |

**Behavior:**

1. Verify worktree is frozen
2. Start Docker containers
3. Remove frozen mark
4. Switch to worktree

**Output:**
```
Resuming 'feature-auth'...

✓ Started containers
✓ Unmarked as frozen
✓ Switched to 'feature-auth'
```

---

### Docker Commands

---

#### `grove up`

Start Docker containers for the current worktree.

**Usage:**
```
grove up [flags]
```

**Flags:**
| Flag | Description |
|------|-------------|
| `--build` | Rebuild images before starting |

**Behavior:**

1. Verify in a grove worktree
2. Source `.envrc` for COMPOSE_PROJECT_NAME
3. Run `docker compose up -d`

**Output:**
```
Starting containers for 'feature-auth'...

✓ web      started
✓ mysql    started
✓ redis    started

Containers running. Logs: grove logs
```

---

#### `grove down`

Stop Docker containers for the current worktree.

**Usage:**
```
grove down [flags]
```

**Flags:**
| Flag | Description |
|------|-------------|
| `-v, --volumes` | Also remove volumes (destructive) |

**Behavior:**

1. Run `docker compose down`

**Output:**
```
Stopping containers for 'feature-auth'...

✓ web      stopped
✓ mysql    stopped  
✓ redis    stopped
```

---

#### `grove logs`

Tail container logs.

**Usage:**
```
grove logs [service] [flags]
```

**Arguments:**
| Argument | Required | Description |
|----------|----------|-------------|
| `service` | No | Specific service (default: all) |

**Flags:**
| Flag | Description |
|------|-------------|
| `-f, --follow` | Follow log output (default: true) |
| `-n, --tail <lines>` | Number of lines to show (default: 100) |

**Behavior:**

1. Run `docker compose logs -f [service]`

---

#### `grove restart`

Restart container services.

**Usage:**
```
grove restart [service]
```

**Arguments:**
| Argument | Required | Description |
|----------|----------|-------------|
| `service` | No | Specific service (default: all) |

**Behavior:**

1. Run `docker compose restart [service]`

---

#### `grove env`

Show or regenerate the worktree's environment configuration.

**Usage:**
```
grove env [flags]
```

**Flags:**
| Flag | Description |
|------|-------------|
| `--regenerate` | Regenerate .envrc file |

**Output (default):**
```
Environment for 'feature-auth'
═══════════════════════════════════════════════════════

COMPOSE_PROJECT_NAME=myapp-feature-auth

Source: ~/projects/myapp-feature-auth/.envrc
```

**Output (--regenerate):**
```
Regenerating .envrc for 'feature-auth'...

✓ Generated COMPOSE_PROJECT_NAME
✓ Wrote .envrc

Run `direnv allow` to apply changes.
```

**Note:** This command is useful for custom integrations. Developers can add their own environment variables to `.envrc` and use `--regenerate` to reset to defaults if needed.

---

### GitHub Commands

---

#### `grove pr`

Check out a pull request into a worktree for review.

**Usage:**
```
grove pr <number> [flags]
```

**Arguments:**
| Argument | Required | Description |
|----------|----------|-------------|
| `number` | Yes | PR number |

**Flags:**
| Flag | Description |
|------|-------------|
| `--worktree <name>` | Target worktree (default: `scratch` or create `pr-{number}`) |
| `--new` | Always create new worktree `pr-{number}` |

**Behavior:**

1. Fetch PR metadata from GitHub (branch name, title)
2. Fetch PR branch from origin
3. If `scratch` worktree exists and no `--new`: checkout PR branch there
4. Otherwise: create worktree `pr-{number}` with PR branch
5. Switch to worktree

**Output:**
```
Checking out PR #123: "Add user authentication"

✓ Fetched branch 'feature/auth' from origin
✓ Checked out in 'scratch' worktree
✓ Switched to 'scratch'

PR #123 ready for review at ~/projects/myapp-scratch
Branch: feature/auth
Author: @coworker
```

---

#### `grove issue`

Create a worktree from a GitHub issue.

**Usage:**
```
grove issue <number> [flags]
```

**Arguments:**
| Argument | Required | Description |
|----------|----------|-------------|
| `number` | Yes | Issue number |

**Flags:**
| Flag | Description |
|------|-------------|
| `--branch <name>` | Override branch name (default: from issue title) |
| `--worktree <name>` | Override worktree name (default: from issue) |

**Behavior:**

1. Fetch issue metadata from GitHub (title, labels)
2. Generate branch name: `issue-{number}-{slug}` (from title)
3. Generate worktree name: `issue-{number}`
4. Create worktree with new branch

**Output:**
```
Creating worktree from issue #456: "Fix session timeout handling"

✓ Created branch 'issue-456-fix-session-timeout' from 'main'
✓ Created worktree 'issue-456' at ~/projects/myapp-issue-456
✓ Generated .envrc
✓ Switched to 'issue-456'

Ready to work on issue #456.
```

---

#### `grove prs`

Browse open pull requests interactively.

**Usage:**
```
grove prs [flags]
```

**Flags:**
| Flag | Description |
|------|-------------|
| `--assigned` | Only PRs assigned to you |
| `--author <user>` | Filter by author |
| `--review-requested` | PRs where your review is requested |

**Behavior:**

1. Fetch PRs from GitHub
2. Display in fzf
3. On selection: run `grove pr <selected>`

---

#### `grove issues`

Browse open issues interactively.

**Usage:**
```
grove issues [flags]
```

**Flags:**
| Flag | Description |
|------|-------------|
| `--assigned` | Only issues assigned to you |
| `--label <label>` | Filter by label |

**Behavior:**

1. Fetch issues from GitHub
2. Display in fzf
3. On selection: run `grove issue <selected>`

---

### Maintenance Commands

---

#### `grove repair`

Detect and fix inconsistent state.

**Usage:**
```
grove repair [flags]
```

**Flags:**
| Flag | Description |
|------|-------------|
| `--dry-run` | Show what would be fixed without changing anything |

**Behavior:**

1. Scan for issues:
   - Orphaned tmux sessions (no worktree)
   - Missing worktree directories
   - Stale state entries
2. Report findings
3. Prompt to fix each issue (or fix all)

**Output:**
```
Scanning for issues...

Found 2 issues:

1. Orphaned tmux session: 'myapp-old-feature'
   No matching worktree exists.
   → Kill session? [y/N]

2. Missing worktree directory: 'feature-broken'
   State exists but ~/projects/myapp-feature-broken is missing.
   → Remove from state? [y/N]

Repair complete. Fixed 2 issues.
```

---

#### `grove clean`

Remove old or stale worktrees.

**Usage:**
```
grove clean [flags]
```

**Flags:**
| Flag | Description |
|------|-------------|
| `--older-than <days>` | Remove worktrees not accessed in N days (default: 30) |
| `--frozen` | Only clean frozen worktrees |
| `--dry-run` | Show what would be removed |

**Behavior:**

1. Find worktrees matching criteria
2. Show list with last access time, dirty status
3. Prompt for confirmation
4. Remove selected worktrees

**Output:**
```
Finding stale worktrees (not accessed in 30+ days)...

WORKTREE          LAST ACCESS   STATUS    BRANCH
───────────────────────────────────────────────────────
old-feature       45 days ago   frozen    feature/old
experiment        32 days ago   clean     experiment
pr-123            38 days ago   clean     feature/someone

Remove these 3 worktrees? [y/N] y

✓ Removed 'old-feature'
✓ Removed 'experiment'
✓ Removed 'pr-123'

Cleaned 3 worktrees.
```

---

## Shell Integration

Grove requires shell integration for directory switching to work outside tmux.

**Setup:**

```bash
# In ~/.zshrc
eval "$(grove init zsh)"

# Or for bash
eval "$(grove init bash)"
```

**What it does:**

Wraps the `grove` command to intercept `cd:` directives and actually change directories.

```bash
# grove outputs: cd:/home/dev/projects/myapp-testing
# Shell integration translates to: cd /home/dev/projects/myapp-testing
```

---

## Exit Codes

| Code | Meaning |
|------|---------|
| 0 | Success |
| 1 | Resource not found (worktree, branch) |
| 2 | Resource already exists |
| 3 | Git operation failed |
| 4 | Invalid input |
| 5 | Operation cancelled by user |
| 6 | External command failed (docker, gh) |
| 10 | Not in a grove project |
| 11 | Not in a worktree |

---

## Environment Variables

| Variable | Description |
|----------|-------------|
| `GROVE_CONFIG` | Override global config path |
| `GROVE_DIFF_TOOL` | Diff tool for `grove compare` |
| `GROVE_NO_COLOR` | Disable colored output |
| `GROVE_DEBUG` | Enable debug logging |

---

## File Locations

| Path | Purpose |
|------|---------|
| `~/.config/grove/config.yml` | Global configuration |
| `{project}/.grove/config.yml` | Project configuration |
| `{project}/.grove/state.json` | Runtime state (frozen, last worktree) |
| `{worktree}/.envrc` | Worktree environment |

---

## Command Summary

### Setup & Config
| Command | Purpose |
|---------|---------|
| `grove setup` | Initialize grove in repo |
| `grove config` | View/edit configuration |

### Worktree Management
| Command | Purpose |
|---------|---------|
| `grove ls` | List worktrees |
| `grove new <name>` | Create worktree on default branch |
| `grove new <name> --branch` | Create worktree with new branch |
| `grove to <name>` | Switch to worktree |
| `grove last` | Switch to previous worktree |
| `grove here` | Show current worktree info |
| `grove rm <name>` | Remove worktree |
| `grove fork <name>` | Fork current work (with uncommitted changes) |
| `grove compare <name>` | Diff current vs another worktree |
| `grove freeze [name]` | Freeze worktree |
| `grove resume <name>` | Resume frozen worktree |

### Docker
| Command | Purpose |
|---------|---------|
| `grove up` | Start containers |
| `grove down` | Stop containers |
| `grove logs [service]` | Tail logs |
| `grove restart [service]` | Restart services |
| `grove env` | Show/regenerate environment |

### GitHub
| Command | Purpose |
|---------|---------|
| `grove pr <number>` | Checkout PR |
| `grove issue <number>` | Create worktree from issue |
| `grove prs` | Browse PRs (fzf) |
| `grove issues` | Browse issues (fzf) |

### Maintenance
| Command | Purpose |
|---------|---------|
| `grove repair` | Fix broken state |
| `grove clean` | Remove stale worktrees |
