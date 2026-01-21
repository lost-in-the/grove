# Grove CLI — Consolidated Architectural Specification v1.0

**Document Purpose**: This is the canonical specification for Grove, a worktree orchestration CLI for developers. It consolidates design decisions from multiple iterations (v1-v4), external evaluations, and implementation learnings. All development should reference this document.

**Last Updated**: January 2026

---

## Table of Contents

1. [Philosophy and Principles](#1-philosophy-and-principles)
2. [Terminology](#2-terminology)
3. [Directory Structure](#3-directory-structure)
4. [Command Reference](#4-command-reference)
5. [State Management](#5-state-management)
6. [Configuration](#6-configuration)
7. [Plugin Architecture](#7-plugin-architecture)
8. [Shell Integration](#8-shell-integration)
9. [Safety and Protection](#9-safety-and-protection)
10. [Naming Rules](#10-naming-rules)
11. [Error Handling](#11-error-handling)
12. [Interactive Mode and Prompts](#12-interactive-mode-and-prompts)
13. [Output Formatting](#13-output-formatting)
14. [Platform Support](#14-platform-support)
15. [Implementation Phases](#15-implementation-phases)

---

## 1. Philosophy and Principles

### 1.1 Core Philosophy

**Worktrees are workspaces. Branches are version control.**

These are separate concerns. Grove manages workspaces (isolated development environments). By default, each worktree gets its own branch—this is the common workflow. Use `--checkout` to reuse an existing branch.

### 1.2 Design Principles

1. **Zero-friction context switching**: Core commands complete in <500ms. No menus or confirmations for safe operations.

2. **Cognitive accessibility**: Commands are discoverable and forgiving. Users with ADHD or memory challenges shouldn't need to recall complex git incantations.

3. **Safety-first defaults**: Destructive operations require explicit confirmation. Data preservation over convenience.

4. **Progressive disclosure**: Simple commands for common tasks. Flags and options for advanced use cases.

5. **Plugin extensibility**: Core handles worktree lifecycle. Plugins handle Docker, tmux, databases, and other integrations.

6. **Machine-friendly**: Core commands support `--json` output. Stable exit codes. Destructive write operations support `--dry-run`.

### 1.3 What Grove Is Not

- **Not a git replacement**: Grove orchestrates worktrees; users still use git for commits, pushes, merges
- **Not an IDE**: Grove manages environments; it doesn't edit code or run tests
- **Not Docker Compose**: Grove can orchestrate Docker via plugins, but isn't a container management tool

---

## 2. Terminology

| Term | Definition |
|------|------------|
| **Worktree** | A git worktree—a separate working directory that shares `.git` with the root repository. In Grove, synonymous with "workspace." |
| **Root worktree** | The original clone directory. Its name matches the project name (e.g., `myapp`). Can be referenced as `@root` in commands. Cannot be removed via Grove. |
| **Branch** | A git branch pointer. One branch can only be checked out in one worktree at a time (git constraint). |
| **Protected** | A worktree that cannot be removed without explicit `--force --unprotect` flags. |
| **Immutable** | A worktree that cannot receive changes from other worktrees (future feature). |
| **Environment worktree** | A worktree that mirrors a remote branch (e.g., `origin/main`) using a local ref. Read-only, used as a stable baseline for hotfixes or debugging. |
| **Fork** | Create a new worktree with a new branch from the current HEAD, optionally moving uncommitted changes. |
| **WIP** | Work in progress—uncommitted changes in a worktree. |

---

## 3. Directory Structure

### 3.1 Project Layout

After `grove setup`, a grove-managed project looks like:

```
~/projects/
  myapp/                           # Root worktree (original clone)
    .git/                          # The actual git directory
    .grove/                        # Grove project state
      config.toml                  # Project configuration (can be committed)
      state.json                   # Runtime state (gitignored)
    .envrc                         # Environment variables
    app/
    ...

  myapp-testing/                   # Worktree: utility workspace
    .git → ../myapp/.git           # Symlink to shared git directory
    .envrc
    app/
    ...

  myapp-feature-auth/              # Worktree: feature work
    .git → ../myapp/.git
    .envrc
    app/
    ...
```

### 3.2 Naming Convention

- Root worktree: `{project}` (directory name)
- Other worktrees: `{project}-{worktree-name}`
- All worktrees are sibling directories

### 3.3 Global Configuration Location

```
~/.config/grove/
  config.toml                      # Global configuration
  plugins/                         # Plugin directory (optional)
```

---

## 4. Command Reference

### 4.1 Command Overview

```
SETUP & CONFIG
  grove setup             Initialize grove in repository
  grove config            View/edit configuration
  grove init <shell>      Output shell integration code
  grove version           Show version information

WORKTREE LIFECYCLE (Core)
  grove new <name>        Create worktree with new branch
  grove to <name>         Switch to worktree
  grove last              Switch to previous worktree
  grove here              Show current worktree info
  grove ls                List worktrees
  grove rm <name>         Remove worktree

WORKTREE LIFECYCLE (Extended)
  grove fork <name>       Create parallel worktree from current state
  grove compare <name>    Diff current vs another worktree
  grove apply <name>      Apply changes from another worktree (future)

DOCKER (Plugin)
  grove up [name]         Start containers
  grove down [name]       Stop containers
  grove logs [service]    Tail container logs
  grove restart [service] Restart services

GITHUB (Plugin, requires gh CLI)
  grove fetch pr/<num>    Checkout PR for review
  grove fetch issue/<num> Create worktree from issue
  grove prs               Browse PRs interactively
  grove issues            Browse issues interactively

MAINTENANCE
  grove repair            Fix inconsistent state
  grove clean             Remove stale worktrees
  grove sync [name]       Update environment worktree mirrors
```

---

### 4.2 Setup & Configuration Commands

#### `grove setup`

Initialize Grove in the current git repository.

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
2. Check if `.grove/` already exists (error if so)
3. Detect project name from directory name or git remote
4. Create `.grove/config.toml` with project name
5. Create `.grove/state.json` with root worktree entry
6. Generate `.envrc` for root worktree
7. Add `.grove/state.json` to `.gitignore`
8. If flags provided, create additional worktrees via `grove new`

**Output:**
```
✓ Initialized grove in ~/projects/myapp
  Project: myapp
  Root: main branch

Next steps:
  grove new <name>           Create a workspace with new branch
  grove new <name> --checkout <branch>  Use existing branch
```

**Exit Codes:**
| Code | Meaning |
|------|---------|
| 0 | Success |
| 1 | Not a git repository |
| 2 | Already initialized |
| 10 | Run from within a worktree (not root) |

**Error: Running in worktree:**
```
Error: Cannot initialize grove from a worktree.

You're in: ~/projects/myapp-auth (a git worktree)
Root repo:  ~/projects/myapp

Run grove setup from the root repository:
  cd ~/projects/myapp && grove setup
```

Grove detects this by checking if `.git` is a file (worktree symlink) rather than a directory.

---

#### `grove config`

View or edit configuration.

**Usage:**
```
grove config [flags]
grove config get <key>
grove config set <key> <value> [flags]
```

**Subcommands:**
| Subcommand | Description |
|------------|-------------|
| *(none)* | Show effective configuration |
| `get <key>` | Get single value (dot notation: `plugins.docker.enabled`) |
| `set <key> <value>` | Set value in project config (or global with `--global`) |

**Flags:**
| Flag | Description |
|------|-------------|
| `--edit` | Open config file in `$EDITOR` |
| `--global` | Target global config |
| `--project` | Target project config (default for `set`) |
| `-j, --json` | Output as JSON |

**Examples:**
```bash
grove config                              # Show all effective settings
grove config get plugins.docker.enabled   # Get single value
grove config set default_base_branch develop  # Set in project config
grove config set alias g --global         # Set in global config
grove config --edit                       # Open in editor
```

**Output (no flags):**
```
Grove Configuration

Global: ~/.config/grove/config.toml
Project: ~/projects/myapp/.grove/config.toml

Effective settings:
  alias = "w"
  default_base_branch = "main"

  [switch]
  dirty_handling = "prompt"

  [plugins.docker]
  enabled = true
  auto_up = false
```

**Exit Codes:**
| Code | Meaning |
|------|---------|
| 0 | Success |
| 1 | Key not found (for `get`) |
| 4 | Invalid key or value |
| 10 | Not in a grove project (for project config operations without `--global`) |

---

#### `grove init <shell>`

Output shell integration code for the specified shell.

**Usage:**
```
grove init <shell>
```

**Arguments:**
| Argument | Required | Description |
|----------|----------|-------------|
| `shell` | Yes | Shell type: `zsh` or `bash` |

**Examples:**
```bash
grove init zsh    # Output zsh integration
grove init bash   # Output bash integration
```

**Output:** Shell-specific wrapper function. See [Section 8: Shell Integration](#8-shell-integration).

**Exit Codes:**
| Code | Meaning |
|------|---------|
| 0 | Success |
| 4 | Invalid or unsupported shell |

---

#### `grove version`

Show version information.

**Usage:**
```
grove version [flags]
```

**Flags:**
| Flag | Description |
|------|-------------|
| `-j, --json` | Output as JSON |

**Output:**
```
grove version 1.0.0
git commit: abc1234
built: 2025-01-15
```

**Output (--json):**
```json
{
  "version": "1.0.0",
  "commit": "abc1234",
  "built": "2025-01-15"
}
```

**Exit Codes:**
| Code | Meaning |
|------|---------|
| 0 | Success |

---

### 4.3 Core Worktree Commands

#### `grove new <name>`

Create a new worktree with a new branch.

**Usage:**
```
grove new <name> [flags]
```

**Arguments:**
| Argument | Required | Description |
|----------|----------|-------------|
| `name` | Yes | Name for the worktree (sanitized per naming rules) |

**Flags:**
| Flag | Description |
|------|-------------|
| `--branch <name>` | Use a different branch name than the worktree name |
| `--from <ref>` | Base ref for new branch (default: default branch) |
| `--checkout <branch>` | Checkout existing branch instead of creating new one. Must not be checked out elsewhere. |
| `--mirror <remote/branch>` | Create environment worktree that mirrors a remote branch (see Section 9.3) |
| `--no-switch` | Don't switch to new worktree after creation |
| `-j, --json` | Output as JSON (includes `switch_to` path for shell integration) |

**Default behavior:** Creates a new worktree with a **new branch** of the same name, based on the default branch. This is the most common workflow—each worktree gets its own branch for isolated work.

**Examples:**
```bash
grove new auth                    # Worktree 'auth', branch 'auth' from main
grove new auth --branch feat/auth # Worktree 'auth', branch 'feat/auth' from main
grove new auth --from develop     # Worktree 'auth', branch 'auth' from develop
grove new review --checkout pr-123 # Worktree 'review', existing branch 'pr-123'
grove new production --mirror origin/main  # Environment worktree mirroring origin/main
```

**Behavior:**

1. **Sanitize name** per naming rules (Section 10)
2. **Check worktree doesn't exist** (exit 2 if exists)
3. **Determine branch:**
   - If `--checkout <branch>`: use existing branch (exit 3 if checked out elsewhere)
   - Otherwise: create new branch named `<name>` (or `--branch` value) from `--from` ref or default branch
   - Exit 2 if branch already exists
4. **Create worktree** via `git worktree add`
5. **Generate `.envrc`** with `COMPOSE_PROJECT_NAME`
6. **Record in state** (`.grove/state.json`)
7. **Run plugin hooks** (`post-create`)
8. **Switch to worktree** (unless `--no-switch`)

**Output (success):**
```
Creating worktree 'auth'...

✓ Created ~/projects/myapp-auth
✓ Branch: auth (new, from main)
✓ Switched to 'auth'
```

**Output (--json):**
```json
{
  "name": "auth",
  "path": "/home/dev/projects/myapp-auth",
  "branch": "auth",
  "base_branch": "main",
  "created": true,
  "switch_to": "/home/dev/projects/myapp-auth"
}
```

**Exit Codes:**
| Code | Meaning |
|------|---------|
| 0 | Success |
| 2 | Worktree already exists, or branch already exists (see error message for resolution) |
| 3 | Git operation failed (branch checked out elsewhere, etc.) |
| 4 | Invalid input |
| 5 | Cancelled by user |
| 10 | Not in a grove project |

**Error: Branch already exists:**
```
Error: Branch 'auth' already exists.

Options:
  grove new auth --checkout auth     Use existing branch in new worktree
  grove new auth --branch auth-v2    Create worktree with different branch name

To see where 'auth' is checked out:
  git worktree list | grep auth
```

**Error: Branch checked out elsewhere:**
```
Error: Branch 'auth' is already checked out in 'myapp-auth'.

To switch to that worktree:
  grove to auth
```

---

#### `grove to <name>`

Switch to an existing worktree.

**Usage:**
```
grove to <name>
grove to @root
```

**Arguments:**
| Argument | Required | Description |
|----------|----------|-------------|
| `name` | Yes | Worktree name to switch to. Use `@root` to switch to the root worktree (original clone directory). |

**Flags:**
| Flag | Description |
|------|-------------|
| `-j, --json` | Output as JSON (includes `switch_to` path for shell integration) |

**Special names:**
- `@root` — Always resolves to the root worktree (the original clone directory)
- Project name (e.g., `myapp`) — Also refers to the root worktree

**Examples:**
```bash
grove to testing     # Switch to 'testing' worktree
grove to @root       # Switch to root worktree
grove to myapp       # Same as @root (if project is 'myapp')
```

**Behavior:**

1. **Verify worktree exists** (exit 1 if not)
2. **Check if already in target** (message and exit 0 if so)
3. **Warn if current worktree is dirty** (non-blocking, informational)
4. **Record current as "last"** in state
5. **Run plugin hooks** (`pre-switch`, `post-switch`)
6. **Output directory change directive** for shell wrapper

**Output (clean switch):**
```
Switched to 'testing'
```

**Output (leaving dirty worktree):**
```
Note: 3 uncommitted files in 'auth'
  M app/models/user.rb
  M app/controllers/auth_controller.rb
  A app/services/token_service.rb

Switched to 'testing'
```

**Output (switching to worktree with stopped containers):**
```
Switched to 'old-feature'

Note: Containers are not running.
  grove up    Start containers
```

**Output (--json):**
```json
{
  "name": "testing",
  "path": "/home/dev/projects/myapp-testing",
  "branch": "testing",
  "switch_to": "/home/dev/projects/myapp-testing"
}
```

**Exit Codes:**
| Code | Meaning |
|------|---------|
| 0 | Success (including "already there") |
| 1 | Worktree not found |
| 10 | Not in a grove project |
| 11 | Worktree directory missing (suggest `grove repair`) |

---

#### `grove last`

Switch to the previously active worktree.

**Usage:**
```
grove last [flags]
```

**Flags:**
| Flag | Description |
|------|-------------|
| `-j, --json` | Output as JSON (includes `switch_to` path for shell integration) |

**Behavior:**
1. Read `last_worktree` from state
2. Call `grove to <last_worktree>` internally

**Output:**
```
Switched to 'auth' (previous)
```

**Output (--json):**
```json
{
  "name": "auth",
  "path": "/home/dev/projects/myapp-auth",
  "branch": "feature/auth",
  "switch_to": "/home/dev/projects/myapp-auth"
}
```

**Exit Codes:**
| Code | Meaning |
|------|---------|
| 0 | Success |
| 1 | No previous worktree recorded |
| 10 | Not in a grove project |

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
| `-v, --verbose` | Show all metadata fields |

**Output (default):**
```
Worktree: auth
Branch:   feature/auth
Path:     ~/projects/myapp-auth

Git:      3 uncommitted files
Docker:   running (web, mysql, redis)
```

**Output (--verbose):**
```
Worktree: auth
Branch:   feature/auth
Path:     ~/projects/myapp-auth
Base:     main (5 ahead, 2 behind)

Git:      3 uncommitted files
  M app/models/user.rb
  M app/controllers/auth_controller.rb
  A app/services/token_service.rb

Docker:   running
  web     up 2h
  mysql   up 2h
  redis   up 2h

Created:  2025-01-15 09:30
Accessed: 2h ago
```

**Output (--quiet):**
```
auth
```

**Output (--json):**
```json
{
  "name": "auth",
  "path": "/home/dev/projects/myapp-auth",
  "branch": "feature/auth",
  "git_status": "dirty",
  "docker_status": "running",
  "created_at": "2025-01-15T09:30:00Z",
  "last_accessed_at": "2025-01-16T14:00:00Z"
}
```

**Flag precedence:** Output format flags (`--quiet`, `--json`, `--verbose`) are mutually exclusive. If multiple are provided, precedence is: `--json` > `--quiet` > `--verbose` > default.

**Exit Codes:**
| Code | Meaning |
|------|---------|
| 0 | Success |
| 10 | Not in a grove project |

---

#### `grove ls`

List all worktrees.

**Usage:**
```
grove ls [flags]
```

**Flags:**
| Flag | Description |
|------|-------------|
| `-p, --paths` | Output paths only (one per line) |
| `-j, --json` | Output as JSON |
| `-q, --quiet` | Names only (one per line) |

**Output (default):**
```
WORKTREES (myapp)

  NAME          BRANCH         GIT      DOCKER
• myapp         main           clean    running
  auth          feature/auth   dirty    running
  testing       testing        clean    stopped
  (env) prod    ← origin/main  synced   running

• = current
```

**Git status values:** `clean`, `dirty`, `conflict`, `detached`, `synced` (environment worktrees only)

**Docker status values:** `running`, `stopped`, `partial`, `n/a`

**Flag precedence:** Output format flags (`--paths`, `--json`, `--quiet`) are mutually exclusive. If multiple are provided, precedence is: `--json` > `--paths` > `--quiet` > default.

**Exit Codes:**
| Code | Meaning |
|------|---------|
| 0 | Success |
| 10 | Not in a grove project |

---

#### `grove rm <name>`

Remove a worktree.

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
| `--force` | Remove even if dirty or has unpushed commits |
| `--keep-branch` | Don't delete the associated branch |
| `--volumes` | Also remove Docker volumes (destructive) |
| `--unprotect` | Allow removal of protected worktree (requires `--force`) |

**Behavior:**

1. **Verify worktree exists** (exit 1 if not)
2. **Check not current worktree** (exit with message: "Switch away first")
3. **Check not root worktree** (exit with message: "Cannot remove root")
4. **Check not protected** (exit unless `--force --unprotect`)
5. **Check dirty status** (exit unless `--force`, show file list)
6. **Check unpushed commits** (warn and keep branch, or exit unless `--force`)
7. **Run plugin hooks** (`pre-remove`)
8. **Stop Docker containers** (if docker plugin enabled)
9. **Remove git worktree** via `git worktree remove`
10. **Delete branch** (unless `--keep-branch` or has unpushed commits without `--force`)
11. **Remove Docker volumes** (if `--volumes`)
12. **Run plugin hooks** (`post-remove`)
13. **Remove from state**

**Prompts (interactive mode):**

If dirty:
```
Worktree 'auth' has uncommitted changes:
  M app/models/user.rb
  M app/controllers/auth_controller.rb
  A app/services/token_service.rb

Remove anyway? (y/N): 
```

If unpushed commits:
```
Branch 'feature/auth' has 3 commits not pushed to origin.

Remove worktree and delete branch? (y/N): 
```

If `--volumes`:
```
This will delete all data in Docker volumes for 'auth'.
This cannot be undone.

Continue? (y/N): 
```

**Output:**
```
Removing 'old-feature'...

✓ Stopped containers
✓ Removed worktree
✓ Deleted branch 'feature/old'

Removed 'old-feature'
```

**Exit Codes:**
| Code | Meaning |
|------|---------|
| 0 | Success |
| 1 | Worktree not found |
| 5 | Cancelled by user |
| 7 | Cannot remove (current worktree, root worktree, or protected without flags) |
| 10 | Not in a grove project |

---

### 4.4 Extended Worktree Commands

#### `grove fork <name>`

Create a parallel worktree from current state for A/B experimentation.

**Alias:** `grove split`

**Usage:**
```
grove fork <name> [flags]
```

**Arguments:**
| Argument | Required | Description |
|----------|----------|-------------|
| `name` | Yes | Suffix for the new worktree |

**Flags:**
| Flag | Description |
|------|-------------|
| `--branch-name <name>` | Override branch name (default: `{current-branch}-{name}`) |
| `--move-wip` | Move uncommitted changes to fork |
| `--copy-wip` | Copy changes to fork as patch, keep in original |
| `--no-wip` | Fork starts clean, changes stay in original |
| `--no-switch` | Stay in current worktree |
| `-j, --json` | Output as JSON |

**Behavior:**

1. **Get current HEAD** commit and branch name
2. **Handle environment worktree:** If current worktree is an environment, fork creates a new branch from the mirror ref HEAD (e.g., `env/production`). This is the primary mechanism for hotfix workflows—fork from production to get exact production state.
3. **Determine new branch name:** `{current-branch}-{name}` or `--branch-name` value
4. **Check branch doesn't exist** (exit 2 if exists)
5. **Check for uncommitted changes:**
   - If WIP exists and no explicit flag: **prompt user**
   - If `--move-wip`: move changes to fork
   - If `--copy-wip`: create patch, apply to fork, keep original
   - If `--no-wip`: fork starts clean
   - Note: Environment worktrees are always clean, so WIP handling is skipped
6. **Create new branch** from current HEAD
7. **Create worktree** checking out new branch
8. **Apply WIP handling** per above
9. **Record `parent_worktree`** in state for lineage
10. **Run plugin hooks** (`post-create`)
11. **Switch to fork** (unless `--no-switch`)

**Prompt when WIP exists (interactive mode):**
```
You have uncommitted changes (3 files):
  M app/models/user.rb
  M app/controllers/auth_controller.rb
  A app/services/token_service.rb

How should they be handled?
  [1] Move to fork (original becomes clean)
  [2] Copy to fork (both have changes)
  [3] Leave in original (fork starts clean)
  [4] Cancel

Choice [1]: 
```

**Non-interactive default:** `--move-wip` (move to fork)

**WIP Handling Complexity:**

The `--copy-wip` and `--move-wip` options use `git stash` internally, which has known limitations:
- **Untracked files:** Requires `git stash -u` which may not capture all untracked content
- **Binary files:** Large binaries may cause stash performance issues
- **Environment files:** Files like `.env`, `config/credentials.yml.enc`, or database configs are often gitignored and won't transfer
- **Framework-specific state:** Rails encrypted credentials, Node `node_modules` state, Python virtualenvs, etc. don't transfer

**Recommendation:** For complex project state, prefer `--no-wip` and manually copy necessary files after fork creation. Grove will display a warning if untracked files exist when using WIP transfer options.

**Output:**
```
Forking 'auth' to 'auth-v2'...

Current: feature/auth @ abc1234
WIP: 3 files → moving to fork

✓ Created branch 'feature/auth-v2'
✓ Created ~/projects/myapp-auth-v2
✓ Moved uncommitted changes
✓ Switched to 'auth-v2'

You now have:
  auth      feature/auth       (clean)
  auth-v2   feature/auth-v2    (your changes)
```

**Output (--json):**
```json
{
  "name": "auth-v2",
  "path": "/home/dev/projects/myapp-auth-v2",
  "branch": "feature/auth-v2",
  "parent_worktree": "auth",
  "parent_commit": "abc1234",
  "wip_action": "move",
  "switch_to": "/home/dev/projects/myapp-auth-v2"
}
```

**Exit Codes:**
| Code | Meaning |
|------|---------|
| 0 | Success |
| 2 | Branch already exists |
| 3 | Git operation failed |
| 5 | Cancelled by user |
| 10 | Not in a grove project |

---

#### `grove compare <name>`

Show diff between current worktree and another. Primary use case is comparing divergent approaches after a `grove fork`.

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
| `--committed` | Compare commits only (ignore uncommitted changes) |
| `--wip` | Compare uncommitted changes only |
| `-j, --json` | Output as JSON (file list with change counts) |

**Use Cases:**
1. **A/B comparison after fork:** After `grove fork v2`, compare approaches between `auth` and `auth-v2`
2. **Drift detection:** Compare feature worktree against production environment worktree
3. **Review preparation:** See what's changed before creating a PR

**How it works:**
- **Committed changes:** Uses `git diff <their-branch>..<our-branch>` to show commit differences
- **Uncommitted changes (default):** Compares the full working state of both worktrees, including any uncommitted changes. Internally creates temporary stashes/commits to enable cross-worktree diff, then discards them. If one worktree is clean, only the other's uncommitted changes appear in the diff. If both are clean, this is equivalent to `--committed`.
- **`--wip` only:** Diffs only the uncommitted changes, ignoring commit history

**Output (default):** Full diff output

**Output (--stat):**
```
Comparing auth ↔ auth-v2

 app/models/user.rb            | 15 +++---
 app/services/auth_service.rb  | 42 ++++++++++
 3 files changed, 52 insertions(+), 5 deletions(-)
```

**Exit Codes:**
| Code | Meaning |
|------|---------|
| 0 | Success (differences found or not) |
| 1 | Worktree not found |
| 10 | Not in a grove project |

---

#### `grove apply <name>` *(Future)*

Apply changes from another worktree to the current worktree without switching. Cherry-picks or patches work between worktrees.

**Usage:**
```
grove apply <name> [flags]
```

**Arguments:**
| Argument | Required | Description |
|----------|----------|-------------|
| `name` | Yes | Source worktree to apply changes from |

**Flags:**
| Flag | Description |
|------|-------------|
| `--commits` | Apply only committed changes (cherry-pick) |
| `--wip` | Apply only uncommitted changes (patch) |
| `--pick <sha>` | Apply specific commit(s) |
| `--dry-run` | Show what would be applied |
| `-j, --json` | Output as JSON |

**Default behavior:** Without flags, applies all changes (commits since common ancestor + uncommitted changes).

**Use cases:**

1. **Hotfix propagation:** Apply a fix from `hotfix` worktree to `staging` without switching
   ```bash
   grove to staging
   grove apply hotfix --pick abc123
   ```

2. **Selective sync:** Pull specific commits from one feature branch into another
   ```bash
   grove apply auth-v2 --commits
   ```

3. **Review integration:** Apply reviewed changes from a `pr-123` worktree into your feature worktree
   ```bash
   grove apply pr-123 --wip
   ```

**Behavior:**

1. **Validate target:** Check current worktree is not immutable/environment
2. **Determine changes:**
   - `--commits`: Find commits in source not in current (since common ancestor)
   - `--wip`: Create patch from source's uncommitted changes
   - `--pick`: Resolve specific commit SHAs
3. **Preview:** Show what will be applied
4. **Apply:**
   - Commits: `git cherry-pick` each commit
   - WIP: `git apply` the patch
5. **Handle conflicts:** If conflicts occur, pause and show resolution instructions

**Output:**
```
Applying changes from 'hotfix' to 'staging'...

Commits to apply:
  abc1234 Fix critical auth bypass
  def5678 Update error messages

✓ Applied abc1234
✓ Applied def5678

Applied 2 commits from 'hotfix'
```

**Output (conflict):**
```
Applying changes from 'hotfix' to 'staging'...

✓ Applied abc1234
✗ Conflict applying def5678

Resolve conflicts, then:
  git cherry-pick --continue

Or abort:
  git cherry-pick --abort
```

**Exit Codes:**
| Code | Meaning |
|------|---------|
| 0 | Success |
| 1 | Worktree not found |
| 3 | Git operation failed (conflict, invalid commit) |
| 8 | Target is immutable or environment worktree |

---

### 4.5 Docker Commands (Plugin)

These commands are provided by the Docker plugin. See [Section 7.2](#72-docker-plugin).

#### `grove up [name]`

Start Docker containers for a worktree.

**Usage:**
```
grove up [name] [flags]
```

**Arguments:**
| Argument | Required | Description |
|----------|----------|-------------|
| `name` | No | Worktree to start containers for (default: current) |

**Flags:**
| Flag | Description |
|------|-------------|
| `--build` | Rebuild images before starting |
| `--all` | Start containers for all worktrees |
| `-j, --json` | Output as JSON |

**Behavior:**
1. Resolve target worktree (current if not specified)
2. Source `.envrc` for `COMPOSE_PROJECT_NAME`
3. Run `docker compose up -d` (or configured compose command)

**Output:**
```
Starting containers for 'auth'...

✓ web      started
✓ mysql    started
✓ redis    started

Logs: grove logs
```

**Exit Codes:**
| Code | Meaning |
|------|---------|
| 0 | Success |
| 1 | Worktree not found |
| 6 | Docker command failed |
| 10 | Not in a grove project |

---

#### `grove down [name]`

Stop Docker containers for a worktree.

**Usage:**
```
grove down [name] [flags]
```

**Arguments:**
| Argument | Required | Description |
|----------|----------|-------------|
| `name` | No | Worktree to stop containers for (default: current) |

**Flags:**
| Flag | Description |
|------|-------------|
| `-v, --volumes` | Also remove volumes (prompts for confirmation) |
| `--all` | Stop containers for all worktrees |
| `-j, --json` | Output as JSON |

**Exit Codes:**
| Code | Meaning |
|------|---------|
| 0 | Success |
| 1 | Worktree not found |
| 6 | Docker command failed |
| 10 | Not in a grove project |

---

#### `grove logs [service]`

Tail container logs.

**Usage:**
```
grove logs [service] [flags]
```

**Flags:**
| Flag | Description |
|------|-------------|
| `-f, --follow` | Follow output (default: true) |
| `-n, --tail <lines>` | Lines to show (default: 100) |

**Exit Codes:**
| Code | Meaning |
|------|---------|
| 0 | Success |
| 6 | Docker command failed |
| 10 | Not in a grove project |

---

#### `grove restart [service]`

Restart container services.

**Usage:**
```
grove restart [service]
```

**Exit Codes:**
| Code | Meaning |
|------|---------|
| 0 | Success |
| 6 | Docker command failed |
| 10 | Not in a grove project |

---

### 4.6 GitHub Commands (Plugin)

These commands are provided by the Tracker plugin. Requires `gh` CLI and `fzf`.

#### `grove fetch pr/<number>`

Checkout a PR for review.

**Usage:**
```
grove fetch pr/<number> [flags]
```

**Flags:**
| Flag | Description |
|------|-------------|
| `--worktree <name>` | Target worktree (default: `scratch`) |
| `--new` | Always create new worktree `pr-{number}` |

**Behavior:**
1. Fetch PR metadata via `gh pr view`
2. Fetch PR branch from origin
3. If `scratch` exists and not `--new`: checkout there
4. Otherwise: create `pr-{number}` worktree
5. Switch to worktree

**Output:**
```
Checking out PR #123: "Add user authentication"

✓ Fetched branch 'feature/auth'
✓ Checked out in 'scratch'
✓ Switched to 'scratch'

PR ready for review. Author: @coworker
```

**Exit Codes:**
| Code | Meaning |
|------|---------|
| 0 | Success |
| 1 | PR not found |
| 3 | Git operation failed (fetch failed, branch conflict) |
| 6 | `gh` CLI command failed |
| 10 | Not in a grove project |

---

#### `grove fetch issue/<number>`

Create worktree from GitHub issue.

**Usage:**
```
grove fetch issue/<number> [flags]
```

**Flags:**
| Flag | Description |
|------|-------------|
| `--branch <name>` | Override branch name |
| `--worktree <name>` | Override worktree name |

**Exit Codes:**
| Code | Meaning |
|------|---------|
| 0 | Success |
| 1 | Issue not found |
| 2 | Worktree or branch already exists |
| 6 | `gh` CLI command failed |
| 10 | Not in a grove project |

---

#### `grove prs` / `grove issues`

Interactive browser using `fzf`.

**Usage:**
```
grove prs [flags]
grove issues [flags]
```

**Flags:**
| Flag | Description |
|------|-------------|
| `--assigned` | Filter to assigned to you |
| `--author <user>` | Filter by author |
| `--label <label>` | Filter by label |
| `--state <state>` | Filter by state (open/closed/all) |

**Exit Codes:**
| Code | Meaning |
|------|---------|
| 0 | Success (or user exited fzf without selection) |
| 5 | Cancelled by user |
| 6 | `gh` or `fzf` command failed |

---

### 4.7 Maintenance Commands

#### `grove repair`

Detect and fix inconsistent state.

**Usage:**
```
grove repair [flags]
```

**Flags:**
| Flag | Description |
|------|-------------|
| `--dry-run` | Show what would be fixed |

**Behavior:**

Scans for and fixes:
- State entries for missing worktree directories
- Worktree directories not in state
- Orphaned tmux sessions (if tmux plugin enabled)
- Corrupted state file

**Output:**
```
Scanning for issues...

Found 2 issues:

1. Missing directory for 'old-feature'
   Path: ~/projects/myapp-old-feature (not found)
   → Remove from state? [Y/n]: 

2. Untracked worktree directory
   Path: ~/projects/myapp-experiment
   → Add to state? [Y/n]:

✓ Fixed 2 issues
```

**Exit Codes:**
| Code | Meaning |
|------|---------|
| 0 | Success (issues fixed or none found) |
| 5 | Cancelled by user |
| 10 | Not in a grove project |
| 11 | Unrecoverable state corruption |

---

#### `grove clean`

Remove stale worktrees.

**Usage:**
```
grove clean [flags]
```

**Flags:**
| Flag | Description |
|------|-------------|
| `--older-than <days>` | Threshold for stale (default: 30) |
| `--include-dirty` | Include dirty worktrees in candidates |
| `--dry-run` | Show what would be removed |

**Safety:** This command ALWAYS prompts for confirmation, even with `GROVE_NONINTERACTIVE=1`. Bulk worktree removal is too destructive for automated execution. Use `grove rm` in scripts if programmatic removal is required.

**Behavior:**

1. **Find candidates:**
   - Not accessed in `--older-than` days
   - If not `--include-dirty`: exclude dirty worktrees
   - **Always exclude:** root, protected, current, environment

2. **Display with full detail:**
```
Finding stale worktrees (not accessed in 30+ days)...

WORKTREE      LAST ACCESS   STATUS      BRANCH
old-feature   45 days ago   clean       feature/old
experiment    32 days ago   dirty       experiment
pr-123        38 days ago   unpushed    pr-123-review

Dirty worktree details:
  experiment:
    M app/models/user.rb
    A app/services/new_service.rb

Unpushed commit details:
  pr-123: 2 commits ahead of origin/pr-123-review

Skipped (protected): staging

Remove these 3 worktrees? [y/N]: 
```

3. **On confirmation:** Remove each worktree

**Exit Codes:**
| Code | Meaning |
|------|---------|
| 0 | Success (or no candidates found) |
| 5 | Cancelled by user |
| 10 | Not in a grove project |

---

#### `grove sync [name]`

Update environment worktree mirrors to match their remote branch.

**Usage:**
```
grove sync [name] [flags]
```

**Arguments:**
| Argument | Required | Description |
|----------|----------|-------------|
| `name` | No | Environment worktree to sync (default: current if environment, or all environments) |

**Flags:**
| Flag | Description |
|------|-------------|
| `--all` | Sync all environment worktrees |
| `-j, --json` | Output as JSON |

**Behavior:**

*Without arguments:*
- If current worktree is an environment: sync just that environment
- Otherwise: sync all environment worktrees

*With name:* sync only the specified environment worktree

1. Run `git fetch` to get latest remote refs
2. For each target environment worktree:
   - Fast-forward local mirror ref (`env/production`) to remote HEAD
   - Update worktree to new ref
3. Report any conflicts (should be rare since environments are read-only)

**Output:**
```
Syncing environment worktrees...

✓ production: origin/main abc1234 → def5678 (3 new commits)
✓ staging: origin/staging (already up to date)
```

**Exit Codes:**
| Code | Meaning |
|------|---------|
| 0 | Success |
| 1 | Worktree not found |
| 3 | Git operation failed (fetch failed, conflict) |
| 8 | Worktree is not an environment worktree |
| 10 | Not in a grove project |

---

## 5. State Management

### 5.1 State File Location

```
{project}/.grove/
  config.toml      # Project configuration (can be committed)
  state.json       # Runtime state (gitignored)
```

### 5.2 State Schema

```json
{
  "version": 1,
  "project": "myapp",
  "last_worktree": "auth",
  "worktrees": {
    "myapp": {
      "path": "/home/dev/projects/myapp",
      "branch": "main",
      "root": true,
      "docker_project": "myapp",
      "created_at": "2025-01-10T10:00:00Z",
      "last_accessed_at": "2025-01-16T14:30:00Z",
      "parent_worktree": null
    },
    "auth": {
      "path": "/home/dev/projects/myapp-auth",
      "branch": "feature/auth",
      "root": false,
      "docker_project": "myapp-auth",
      "created_at": "2025-01-15T09:30:00Z",
      "last_accessed_at": "2025-01-16T14:00:00Z",
      "parent_worktree": null
    },
    "auth-v2": {
      "path": "/home/dev/projects/myapp-auth-v2",
      "branch": "feature/auth-v2",
      "root": false,
      "docker_project": "myapp-auth-v2",
      "created_at": "2025-01-16T11:00:00Z",
      "last_accessed_at": "2025-01-16T14:30:00Z",
      "parent_worktree": "auth"
    },
    "production": {
      "path": "/home/dev/projects/myapp-production",
      "branch": "env/production",
      "root": false,
      "docker_project": "myapp-production",
      "created_at": "2025-01-10T10:00:00Z",
      "last_accessed_at": "2025-01-16T12:00:00Z",
      "parent_worktree": null,
      "environment": true,
      "mirror": "origin/main",
      "last_synced_at": "2025-01-16T12:00:00Z"
    }
  }
}
```

### 5.3 Field Definitions

**Root fields:**
| Field | Type | Description |
|-------|------|-------------|
| `version` | number | Schema version for migration |
| `project` | string | Project name |
| `last_worktree` | string | Last active worktree (for `grove last`) |
| `worktrees` | object | Map of worktree name → metadata |

**Worktree fields:**
| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `path` | string | Yes | Absolute path to worktree |
| `branch` | string | Yes | Branch name (or mirror ref like `env/production`) |
| `root` | boolean | Yes | Is this the root worktree? |
| `docker_project` | string | Yes | COMPOSE_PROJECT_NAME value |
| `created_at` | ISO8601 | Yes | Creation timestamp |
| `last_accessed_at` | ISO8601 | Yes | Last switch timestamp |
| `parent_worktree` | string | No | Parent name if forked |
| `environment` | boolean | No | Is this an environment worktree? |
| `mirror` | string | No | Remote branch this environment mirrors (e.g., `origin/main`) |
| `last_synced_at` | ISO8601 | No | Last sync timestamp for environment worktrees |

### 5.4 State File Integrity

**Atomic writes:** State changes use write-to-temp-then-rename pattern to prevent corruption.

**Locking:** File locking prevents concurrent modifications.

**Recovery:** If state is corrupted, `grove repair` attempts recovery. Worst case: delete `state.json` and run `grove setup` to reinitialize.

---

## 6. Configuration

### 6.1 Configuration Hierarchy

Configuration is loaded from (later overrides earlier):
1. Built-in defaults
2. Global: `~/.config/grove/config.toml`
3. Project: `{project}/.grove/config.toml`
4. Environment variables
5. Command-line flags

### 6.2 Full Configuration Reference

```toml
# ~/.config/grove/config.toml

# Command alias (shell integration creates this alias)
alias = "w"

# Directory where projects live (informational)
projects_dir = "~/projects"

# Default branch for new worktrees
default_base_branch = "main"

[switch]
# What to do when switching from dirty worktree
# Options: "warn" (show message, continue), "prompt" (ask), "refuse" (error)
dirty_handling = "warn"

[protection]
# Worktrees that cannot be removed without --force --unprotect
protected = []

# Worktrees that cannot receive changes from other worktrees
immutable = []

[plugins.docker]
enabled = true
# Start containers automatically on worktree switch
auto_up = false
# Docker compose command (auto-detected if not set)
compose_command = ""  # "docker compose" or "docker-compose"

[plugins.tmux]
enabled = true
# Prefix for tmux session names
prefix = ""
# Create session on worktree creation
auto_create = true
# Attach/switch on `grove to`
auto_attach = true
# Kill session on worktree removal
auto_kill = true

[plugins.tracker]
enabled = true
# Default worktree for PR checkouts
default_review_worktree = "scratch"

[environments]
# Environment worktrees that mirror remote branches (see Section 9.3)
# Each creates a local ref (env/<name>) that tracks the remote
production = { mirror = "origin/main" }
staging = { mirror = "origin/staging" }
```

### 6.3 Environment Variables

| Variable | Description |
|----------|-------------|
| `GROVE_CONFIG` | Override global config path |
| `GROVE_NO_COLOR` | Disable colored output |
| `GROVE_DEBUG` | Enable debug logging |
| `GROVE_NONINTERACTIVE` | Disable all prompts (error instead) |

---

## 7. Plugin Architecture

### 7.1 Plugin System Overview

Grove uses a plugin system to extend functionality. Plugins can:
- Hook into worktree lifecycle events
- Add new commands
- Store plugin-specific state

**Built-in plugins:**
- **Docker**: Container lifecycle management
- **Tmux**: Session management
- **Tracker**: GitHub PR/issue integration

### 7.2 Docker Plugin

**Purpose:** Manage Docker containers per worktree.

**Configuration:**
```toml
[plugins.docker]
enabled = true
auto_up = false
compose_command = ""
```

**Hooks:**
| Event | Behavior |
|-------|----------|
| `post-create` | Optionally start containers (if `auto_up`) |
| `post-switch` | Optionally start containers (if `auto_up`) |
| `pre-remove` | Stop containers |

**Commands provided:**
- `grove up`
- `grove down`
- `grove logs`
- `grove restart`

**Environment:** Uses `COMPOSE_PROJECT_NAME` from `.envrc` to isolate containers per worktree.

### 7.3 Tmux Plugin

**Purpose:** Manage tmux sessions per worktree.

**Configuration:**
```toml
[plugins.tmux]
enabled = true
prefix = ""
auto_create = true
auto_attach = true
auto_kill = true
```

**Session naming:**
- Root worktree: `{prefix}{project}` (e.g., `myapp` or `grove-myapp`)
- Other worktrees: `{prefix}{project}-{worktree}` (e.g., `myapp-auth` or `grove-myapp-auth`)

**Hooks:**
| Event | Behavior |
|-------|----------|
| `post-create` | Create tmux session |
| `post-switch` | If inside tmux: switch-client. If outside and `auto_attach`: attach. |
| `post-remove` | Kill tmux session |

### 7.4 Tracker Plugin

**Purpose:** GitHub PR/issue integration.

**Requirements:** `gh` CLI, `fzf`

**Configuration:**
```toml
[plugins.tracker]
enabled = true
default_review_worktree = "scratch"
```

**Commands provided:**
- `grove fetch pr/<number>`
- `grove fetch issue/<number>`
- `grove prs`
- `grove issues`

### 7.5 Plugin Hooks

Plugins implement hooks that are called at lifecycle events:

| Hook | When Called |
|------|-------------|
| `pre-create` | Before worktree creation |
| `post-create` | After worktree creation |
| `pre-switch` | Before switching worktrees |
| `post-switch` | After switching worktrees |
| `pre-remove` | Before removing worktree |
| `post-remove` | After removing worktree |

**Hook execution order:** Hooks are called in plugin registration order. `pre-*` hooks can abort the operation by returning an error.

---

## 8. Shell Integration

### 8.1 Purpose

Grove needs to change the shell's working directory when switching worktrees. Since a subprocess cannot change its parent's directory, Grove uses a shell wrapper.

### 8.2 Setup

Add to `~/.zshrc` or `~/.bashrc`:

```bash
eval "$(grove init zsh)"   # for zsh
eval "$(grove init bash)"  # for bash
```

Then reload:
```bash
source ~/.zshrc
```

### 8.3 How It Works

The `grove init` command outputs a shell function that:
1. Calls the real `grove` binary
2. Intercepts directory change directives from the output
3. Executes `cd` in the current shell

**Directory change protocol:**
- Commands that switch worktrees (`grove to`, `grove last`, `grove new`) output `GROVE_CD:/path/to/dir` when a directory change is needed
- The shell wrapper intercepts lines starting with `GROVE_CD:` and executes `cd` to the specified path
- All other output is passed through normally
- When `--json` is used, no `GROVE_CD:` directive is emitted; instead the JSON response includes a `switch_to` field with the target path

**Example wrapper (zsh):**
```zsh
grove() {
  local output exit_code
  output=$("$(command -v grove)" "$@")
  exit_code=$?

  if [[ "$output" == "GROVE_CD:"* ]]; then
    cd "${output#GROVE_CD:}"
  elif [[ -n "$output" ]]; then
    echo "$output"
  fi

  return $exit_code
}

alias w='grove'
```

**Note:** The `GROVE_CD:` prefix is intentionally unique to avoid collision with legitimate command output.

### 8.4 direnv Integration

Each worktree has an `.envrc` file:
```bash
export COMPOSE_PROJECT_NAME=myapp-auth
```

Run `direnv allow` once per worktree. Environment variables load automatically on `cd`.

---

## 9. Safety and Protection

### 9.1 Protected Worktrees

Protected worktrees cannot be removed without explicit flags.

**Configuration:**
```toml
[protection]
protected = ["staging", "production"]
```

**Behavior:**
- `grove rm staging` → Error with instructions
- `grove rm staging --force --unprotect` → Succeeds (with confirmation prompt)
- `grove clean` → Excludes protected worktrees entirely

**Error message:**
```
Error: Worktree 'staging' is protected.

To remove a protected worktree:
  grove rm staging --force --unprotect

This is a safeguard against accidental deletion.
```

### 9.2 Immutable Worktrees

Immutable worktrees cannot receive changes from other worktrees via `grove apply`. This protects critical baselines from accidental modification.

**Configuration:**
```toml
[protection]
immutable = ["staging", "production"]
```

**Behavior:**
- `grove apply hotfix --pick abc123` while in an immutable worktree → Error (exit 8)
- Direct git commands (`git cherry-pick`, `git merge`) are not blocked—this is a workflow convention

### 9.3 Environment Worktrees (Mirror Pattern)

Environment worktrees like `production` or `staging` serve as **environment anchors** — clean baselines that mirror a remote branch. They are not for commits or development; they're reference points.

**Problem:** Git allows only one worktree per branch. If root uses `main`, a production worktree cannot also use `main`.

**Solution:** Use a dedicated **mirror ref** that tracks the remote branch:

```bash
# Create production worktree with mirror ref
grove new production --mirror origin/main
```

This creates:
- Worktree: `myapp-production`
- Local ref: `env/production` (tracks `origin/main`)
- Auto-syncs on `git fetch`

**Configuration:**
```toml
[protection]
protected = ["production"]
immutable = ["production"]

[environments]
production = { mirror = "origin/main" }
staging = { mirror = "origin/staging" }
```

**Guardrails for environment worktrees:**
- Cannot commit directly via Grove commands (exit 8 with message)
- Cannot switch branches (the ref is locked to mirror target)
- Auto-updates when `grove sync` or `git fetch` runs
- Shows clear indicator in `grove ls` and `grove here`

**Note:** Grove cannot prevent direct `git commit` commands. Environment worktrees are a workflow convention, not a hard lock. Consider using a git pre-commit hook if enforcement is required.

**Use cases:**
1. **Hotfix baseline:** Fork from production to create hotfix with exact production state
2. **Debug reproduction:** Container environment matches production for debugging
3. **Comparison:** `grove compare production` shows drift from production state

**Output (`grove here` in production):**
```
Worktree: production (environment)
Mirrors:  origin/main @ abc1234
Path:     ~/projects/myapp-production

Last sync: 2 hours ago
Docker:   running (web, mysql, redis)

Note: This is a read-only environment mirror.
  grove fork hotfix    Create hotfix branch from this state
  grove sync           Update to latest origin/main
```

**`grove new` with --mirror flag:**
```bash
grove new production --mirror origin/main
```

Creates a special environment worktree:
1. Creates local ref `env/production` pointing to `origin/main` HEAD
2. Creates worktree checking out `env/production`
3. Sets `environment: true` and `mirror: "origin/main"` in state
4. Adds to `[environments]` config section

**Hotfix workflow from production:**
```bash
grove to production        # Switch to production baseline
grove fork hotfix          # Fork creates branch from env/production HEAD
                           # Now in hotfix with exact production state
# Fix the issue...
git push origin hotfix
# Create PR, merge, then:
grove to production
grove sync                 # Updates env/production to new origin/main
grove rm hotfix
```

### 9.4 Dirty Worktree Handling

When switching away from a dirty worktree:

**Configuration:**
```toml
[switch]
dirty_handling = "warn"  # Options: "warn", "prompt", "refuse"
```

| Value | Behavior |
|-------|----------|
| `warn` | Show warning, continue |
| `prompt` | Ask user to confirm |
| `refuse` | Error, require explicit action |

### 9.5 Destructive Operation Safeguards

Operations that can cause data loss always require confirmation:

| Operation | Safeguard |
|-----------|-----------|
| Remove dirty worktree | Prompt or `--force` |
| Remove with unpushed commits | Prompt (keeps branch by default) |
| Delete Docker volumes | Always prompts |
| Clean multiple worktrees | Always prompts |

---

## 10. Naming Rules

### 10.1 Worktree Name Transformation

Input names are transformed:
1. Convert to lowercase
2. Replace spaces with hyphens
3. Remove characters except alphanumeric and hyphens
4. Collapse multiple consecutive hyphens
5. Trim to 50 characters (warn if truncated)

**Examples:**
| Input | Transformed | Warning |
|-------|-------------|---------|
| `auth` | `auth` | None |
| `Feature Auth` | `feature-auth` | "Name transformed" |
| `fix/login-bug` | `fix-login-bug` | None |
| `PR #123` | `pr-123` | "Name transformed" |
| `my feature!!!` | `my-feature` | "Name transformed" |

### 10.2 Branch Name Handling

When `--branch` is used:
- Without name: use worktree name as branch name
- With name: use provided name (validate for git compatibility)

When forking:
- Default: `{current-branch}-{fork-suffix}`
- Override with `--branch-name`

### 10.3 COMPOSE_PROJECT_NAME

Format: `{project}-{worktree}` (hyphens preserved for Docker compatibility)

Examples:
- Root: `myapp`
- Worktree `auth`: `myapp-auth`
- Worktree `auth-v2`: `myapp-auth-v2`

---

## 11. Error Handling

### 11.1 Exit Codes

| Code | Meaning |
|------|---------|
| 0 | Success |
| 1 | Resource not found (worktree, branch, file) |
| 2 | Resource already exists |
| 3 | Git operation failed |
| 4 | Invalid input |
| 5 | Operation cancelled by user |
| 6 | External command failed (docker, gh, tmux) |
| 7 | Cannot remove worktree (current, root, or protected without flags) |
| 8 | Environment/immutable constraint violated (apply to protected, commit attempt, sync type mismatch) |
| 10 | Not in a grove project |
| 11 | Worktree directory missing (state inconsistency) |

### 11.2 Error Message Format

```
Error: <what went wrong>

<context or details if helpful>

<suggested actions>
```

**Example:**
```
Error: Worktree 'auth' not found.

Available worktrees:
  myapp, testing, scratch

To create 'auth':
  grove new auth
```

### 11.3 JSON Error Output

With `--json` flag on any command:

```json
{
  "error": true,
  "code": 1,
  "message": "Worktree 'auth' not found",
  "suggestions": ["grove new auth", "grove ls"]
}
```

---

## 12. Interactive Mode and Prompts

### 12.1 When to Prompt

Grove prompts **only** for:
- Destructive operations (delete dirty, delete with unpushed, delete volumes)
- Ambiguous situations requiring choice (branch unavailable)
- Explicit confirmation requests (clean multiple worktrees)

Grove **never** prompts for:
- Read operations
- Safe write operations
- Operations with explicit flags (`--force`)

### 12.2 Prompt Format

```
<statement of situation>
<details if relevant>

<question>? (y/N): 
```

**Default is always safe option** (N for destructive).

**Example:**
```
Worktree 'auth' has uncommitted changes:
  M app/models/user.rb
  M app/controllers/auth_controller.rb

Remove anyway? (y/N): 
```

### 12.3 Choice Prompts

For multiple options:
```
<statement of situation>

<options>
  [1] Option one (annotation)
  [2] Option two
  [3] Cancel

Choice [1]: 
```

Default shown in brackets.

### 12.4 Non-Interactive Mode

Triggered by:
- `GROVE_NONINTERACTIVE=1` environment variable
- Non-TTY stdin

In non-interactive mode:
- Prompts become errors with instructions for explicit flags (e.g., `--force`)
- Choice prompts use default or error

**Exception:** `grove clean` always prompts regardless of non-interactive mode. Bulk worktree removal requires explicit human confirmation. Use `grove rm` with `--force` in scripts for programmatic removal.

---

## 13. Output Formatting

### 13.1 Success Output

```
✓ Created ~/projects/myapp-auth
✓ Branch: auth (new, from main)
✓ Switched to 'auth'
```

### 13.2 Progress Output

```
Creating worktree 'auth'...

✓ Created directory
✓ Created branch
✓ Starting containers...
✓ Switched
```

### 13.3 Warnings

```
Note: 3 uncommitted files in 'auth'
  M app/models/user.rb
  M app/controllers/auth_controller.rb
  A app/services/token_service.rb
```

### 13.4 Errors

```
Error: Worktree 'auth' already exists.

To switch to it:
  grove to auth
```

### 13.5 Tables

```
WORKTREES (myapp)

  NAME          BRANCH         GIT      DOCKER
• myapp         main           clean    running
  auth          feature/auth   dirty    running
  testing       testing        clean    stopped
```

### 13.6 Color Usage

| Element | Color |
|---------|-------|
| Success checkmark (✓) | Green |
| Error | Red |
| Warning ("Note:") | Yellow |
| Worktree names | Cyan |
| Branch names | Magenta |
| Paths | Default |
| Table headers | Bold |

Disable with `GROVE_NO_COLOR=1` or `--no-color`.

---

## 14. Platform Support

### 14.1 Supported Platforms

- **macOS** (arm64, amd64)
- **Linux** (arm64, amd64)

### 14.2 Not Supported

- Windows (native)
- WSL (untested, may work)

### 14.3 Dependencies

**Required:**
- Git 2.30+ (for worktree support)
- Go 1.21+ (for building from source)

**Optional:**
- Tmux 3.0+ (for tmux plugin)
- Docker with Compose (for docker plugin)
- `gh` CLI (for tracker plugin)
- `fzf` (for interactive browsing)
- direnv (for automatic environment loading)

---

## 15. Implementation Phases

### Phase 0: Foundation
- Core commands: `ls`, `new`, `to`, `rm`, `here`, `last`
- Shell integration (zsh, bash)
- Configuration system (TOML)
- State management
- Hook system foundation

### Phase 1: Docker Plugin
- Container lifecycle commands: `up`, `down`, `logs`, `restart`
- Integration with worktree lifecycle hooks
- COMPOSE_PROJECT_NAME isolation

### Phase 2: State Management
- Dirty worktree handling
- State persistence improvements
- Remote container control (`grove up/down <name>`)

### Phase 3: Issue Integration
- GitHub PR/issue integration via `gh` CLI
- `grove fetch pr/<num>` and `grove fetch issue/<num>`
- Interactive browsing with `fzf`

### Phase 4: Polish
- Release automation (GoReleaser)
- Homebrew formula
- Shell completions
- Comprehensive documentation

### Phase 5: Extended Lifecycle
- `grove fork` command with WIP handling
- `grove compare` command
- `grove apply` command
- Branch availability prompting for `grove new`
- Protection configuration
- Environment worktrees with mirror refs

### Phase 6: Future Enhancements
- TUI dashboard (`grove tui`)
- Database plugin (opt-in)
- Multi-project support
- Team sync capabilities

---

## Appendix A: Quick Reference Card

```
CREATE & SWITCH
  grove new <name>           Create workspace with new branch
  grove new <name> --checkout <branch>  Use existing branch
  grove to <name>            Switch to workspace
  grove to @root             Switch to root workspace
  grove last                 Switch to previous

INFORMATION
  grove ls                   List workspaces
  grove here                 Current workspace info

EXPERIMENTATION
  grove fork <name>          Create parallel workspace
  grove compare <name>       Diff against another workspace
  grove apply <name>         Apply changes from another workspace

CLEANUP
  grove rm <name>            Remove workspace
  grove clean                Remove stale workspaces

ENVIRONMENTS
  grove new prod --mirror origin/main   Create environment mirror
  grove sync [name]          Update environment mirrors

DOCKER (if enabled)
  grove up                   Start containers
  grove down                 Stop containers
  grove logs                 View logs

GITHUB (if enabled)
  grove fetch pr/123         Checkout PR for review
  grove prs                  Browse PRs
```

---

## Appendix B: Common Workflows

### Starting New Feature Work

```bash
grove new auth
# Creates worktree 'auth' with branch 'auth' from main
# Switches to auth, starts containers (if configured)

# Do work...

grove to @root              # Switch back to root worktree when done
grove rm auth               # Clean up after merge
```

### A/B Experimentation

```bash
# In auth worktree with half-done work
grove fork v2               # Creates auth-v2 with your changes
                            # auth is now clean

# Try approach B in auth-v2
# Compare approaches
grove compare auth

# Keep one, delete other
grove rm auth               # If v2 is better
# or
grove to auth && grove rm auth-v2  # If original is better
```

### PR Review

```bash
grove fetch pr/123          # Checkout into scratch worktree
# Review, test, comment

grove to auth               # Return to your work
```

### Saving Resources

```bash
grove down auth             # Stop containers for 'auth' worktree
# Later...
grove to auth
grove up                    # Start containers again
```

### Production Environment & Hotfix

```bash
# One-time setup: create production mirror
grove new production --mirror origin/main

# When hotfix needed:
grove to production         # Switch to production baseline
grove sync                  # Ensure latest origin/main
grove fork hotfix           # Create hotfix from exact production state

# Fix the issue in hotfix worktree...
git push origin hotfix
# Create PR, merge to main

# Cleanup:
grove to production
grove sync                  # Update mirror to include hotfix
grove rm hotfix
```

---

## Appendix C: Troubleshooting

### "Worktree directory missing"

State and disk are out of sync. Run:
```bash
grove repair
```

### "Branch is already checked out"

Another worktree has that branch. Either:
- Switch to that worktree: `grove to <name>`
- Create with new branch: `grove new <name> --branch`
- Checkout different branch: `grove new <name> --checkout other-branch`

### Containers not starting

Check Docker plugin is enabled:
```bash
grove config | grep docker
```

Check compose file exists:
```bash
ls docker-compose.yml
```

Start manually:
```bash
grove up
```

### Shell integration not working

Verify wrapper is installed:
```bash
type grove
# Should show function definition, not just path
```

Re-run setup:
```bash
echo 'eval "$(grove init zsh)"' >> ~/.zshrc
source ~/.zshrc
```

---

*End of Specification*