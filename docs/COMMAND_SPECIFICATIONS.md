# Grove Command Specifications

## Document Purpose

This document provides exhaustive specifications for each grove command. Every behavior, edge case, and output format is explicitly defined to prevent ambiguity during implementation.

---

## Table of Contents

1. [Global Behaviors](#global-behaviors)
2. [Naming Conventions](#naming-conventions)
3. [Core Commands](#core-commands)
   - [grove ls](#grove-ls)
   - [grove new](#grove-new)
   - [grove to](#grove-to)
   - [grove rm](#grove-rm)
   - [grove here](#grove-here)
   - [grove last](#grove-last)
4. [State Commands](#state-commands)
   - [grove freeze](#grove-freeze)
   - [grove resume](#grove-resume)
5. [Docker Commands](#docker-commands)
   - [grove up](#grove-up)
   - [grove down](#grove-down)
   - [grove logs](#grove-logs)
   - [grove restart](#grove-restart)
6. [Utility Commands](#utility-commands)
   - [grove config](#grove-config)
   - [grove init](#grove-init)
   - [grove repair](#grove-repair)
   - [grove clean](#grove-clean)
   - [grove doctor](#grove-doctor)
7. [Exit Codes](#exit-codes)
8. [Output Formatting](#output-formatting)

---

## Global Behaviors

### Performance Requirement

**Every command MUST complete in <500ms** under normal conditions. If a command cannot complete in this time:
1. Show immediate feedback (spinner or progress indicator)
2. Log reason for delay if >1s

### Terminal Context Detection

Grove must detect its execution context:

| Context | Detection Method | Behavior |
|---------|------------------|----------|
| Inside tmux | `$TMUX` env var set | Use `tmux switch-client` |
| Outside tmux | `$TMUX` not set | Use `tmux attach-session` or spawn new terminal |
| Inside grove shell wrapper | `$GROVE_SHELL` set | Can output `cd:` directives |
| Direct binary execution | `$GROVE_SHELL` not set | Cannot change directory, print instructions instead |

### Project Context Detection

Grove operates in the context of a **project**. A project is identified by:

1. **Explicit:** `.grove/` directory in current or parent directory
2. **Implicit:** Bare git repository or git worktree root
3. **Fallback:** Current working directory name

The project name is extracted from:
```
Priority 1: .grove/config.toml → project.name
Priority 2: Git remote origin URL → repo name
Priority 3: Root directory name
```

**Example:**
```
/Users/egg/Work/grove-cli/.git  →  project = "grove-cli"
/Users/egg/Work/my-app/.grove/  →  project = (from config or "my-app")
```

### Worktree Naming Convention

**CRITICAL:** All worktrees created by grove follow this pattern:

```
{project}-{worktree-name}
```

**Examples:**
| Project | User Input | Worktree Directory | Tmux Session |
|---------|------------|-------------------|--------------|
| grove-cli | testing | grove-cli-testing | grove-cli-testing |
| grove-cli | feature-auth | grove-cli-feature-auth | grove-cli-feature-auth |
| my-app | hotfix-123 | my-app-hotfix-123 | my-app-hotfix-123 |

**Rationale:** 
- Prevents naming collisions across projects
- Makes `tmux list-sessions` meaningful
- Allows `grove ls` to filter by project

### Display Name vs Full Name

Commands should use **short names** for user-facing display within project context:

```
# grove ls (when in grove-cli project)
NAME          BRANCH    STATUS
testing       testing   clean      ← shows "testing" not "grove-cli-testing"
feature-auth  auth      dirty

# tmux list-sessions (shows full names)
grove-cli-testing: 1 windows
grove-cli-feature-auth: 1 windows
my-app-main: 1 windows
```

---

## Naming Conventions

### Worktree Names

**Input Validation Rules:**
- Alphanumeric, hyphens, underscores only
- No spaces (auto-convert to hyphens)
- No leading/trailing hyphens
- Max 50 characters
- Case-insensitive matching, lowercase storage

**Transformation Examples:**
| User Input | Stored Name | Reason |
|------------|-------------|--------|
| `testing` | `testing` | Valid |
| `Feature Auth` | `feature-auth` | Lowercase, space→hyphen |
| `fix/bug-123` | `fix-bug-123` | Slash→hyphen |
| `--testing--` | `testing` | Strip leading/trailing |
| `TESTING` | `testing` | Lowercase |

### Branch Names

When creating a worktree, grove creates a branch if needed:

```
Default pattern: {worktree-name}
Configurable: {type}/{description}

Examples:
  w new testing          →  branch: testing
  w new feature-auth     →  branch: feature-auth
  w new is/123           →  branch: is/123 (issue integration)
  w new pr/456           →  branch: pr/456 (PR integration)
```

### Directory Locations

Worktrees are created as **siblings** to the main project directory:

```
/Users/egg/Work/
├── grove-cli/              ← main project (where you run commands)
├── grove-cli-testing/      ← worktree created by `w new testing`
├── grove-cli-feature-auth/ ← worktree created by `w new feature-auth`
└── my-other-project/       ← unrelated project
```

**Configurable via:**
```toml
[worktrees]
location = "sibling"  # default: sibling to project
# location = "nested"  # alternative: inside .grove/worktrees/
```

---

## Core Commands

### grove ls

**Purpose:** List all worktrees for the current project with status information.

**Usage:**
```
grove ls [flags]

Flags:
  -a, --all       Include frozen worktrees
  -p, --paths     Show full paths only (scriptable output)
  -j, --json      Output as JSON
  -q, --quiet     Names only, one per line
```

**Behavior:**

1. Detect current project
2. Find all worktrees belonging to this project
3. For each worktree, determine:
   - Name (short name without project prefix)
   - Branch name
   - Git status (clean/dirty/conflict)
   - Tmux session status (attached/detached/none)
   - Docker status (if docker plugin enabled)
   - Frozen status
4. Display in table format

**Output Format (default):**
```
NAME            BRANCH          STATUS     TMUX        PATH
────────────────────────────────────────────────────────────────────────
• main          main            clean      attached    /Users/egg/Work/grove-cli
  testing       testing         clean      detached    /Users/egg/Work/grove-cli-testing
  feature-auth  feature/auth    dirty      none        /Users/egg/Work/grove-cli-feature-auth
  hotfix        main            clean      frozen      /Users/egg/Work/grove-cli-hotfix
```

**Column Definitions:**
- `•` = current worktree (determined by $PWD)
- `NAME` = short name (without project prefix)
- `BRANCH` = current branch
- `STATUS` = git status: `clean`, `dirty` (uncommitted changes), `conflict`, `detached`
- `TMUX` = session status: `attached` (you're in it), `detached` (exists, not attached), `none` (no session), `frozen`
- `PATH` = absolute path

**Output Format (--paths):**
```
/Users/egg/Work/grove-cli
/Users/egg/Work/grove-cli-testing
/Users/egg/Work/grove-cli-feature-auth
```

**Output Format (--json):**
```json
{
  "project": "grove-cli",
  "current": "main",
  "worktrees": [
    {
      "name": "main",
      "fullName": "grove-cli",
      "branch": "main",
      "path": "/Users/egg/Work/grove-cli",
      "status": "clean",
      "tmux": "attached",
      "frozen": false,
      "current": true
    }
  ]
}
```

**Edge Cases:**
| Scenario | Behavior |
|----------|----------|
| No worktrees exist | Show message: "No worktrees found for project 'X'. Create one with: grove new <name>" |
| Not in a git repo | Error: "Not in a git repository. Grove requires a git project." |
| Worktree dir missing | Show with status: `missing` and path in red |
| Git worktree corrupted | Show with status: `corrupted` |

**Exit Codes:**
- 0: Success
- 1: Not in a git repository
- 2: Other error

---

### grove new

**Purpose:** Create a new worktree with associated tmux session.

**Usage:**
```
grove new <name> [flags]

Arguments:
  name    Name for the new worktree (required)

Flags:
  -b, --branch <branch>    Branch to checkout (default: create new branch matching name)
  -f, --from <ref>         Create branch from this ref (default: main/master)
  -t, --template <name>    Use worktree template
  -n, --no-switch          Don't switch to new worktree after creation
      --no-tmux            Don't create tmux session
```

**Behavior:**

1. **Validate name:**
   - Apply naming transformations
   - Check for existing worktree with same name
   
2. **Determine branch:**
   - If `--branch` specified: use that branch
   - If branch exists: checkout existing branch
   - If branch doesn't exist: create from `--from` or default branch
   
3. **Create worktree:**
   ```bash
   git worktree add <path> <branch>
   # or
   git worktree add -b <new-branch> <path> <from-ref>
   ```

4. **Create tmux session** (unless `--no-tmux`):
   - Session name = `{project}-{name}`
   - Start in worktree directory
   - Apply default window layout
   
5. **Generate .envrc** (if direnv plugin enabled):
   - Set PORT based on allocation
   - Set DATABASE_URL if configured
   
6. **Switch to new worktree** (unless `--no-switch`):
   - If inside tmux: `tmux switch-client`
   - If outside tmux: output `cd:` directive or instructions

**Output (Success):**
```
✓ Created worktree 'testing' at /Users/egg/Work/grove-cli-testing
✓ Created branch 'testing' from 'main'
✓ Created tmux session 'grove-cli-testing'
✓ Switched to 'testing'
```

**Output (Already Exists):**
```
✗ Worktree 'testing' already exists

  Path:   /Users/egg/Work/grove-cli-testing
  Branch: testing
  Status: clean

To switch to it: grove to testing
To remove it:    grove rm testing
```
**Exit code: 1**

**Output (Branch Exists, No Worktree):**
```
✓ Created worktree 'feature-auth' at /Users/egg/Work/grove-cli-feature-auth
✓ Checked out existing branch 'feature/auth'
✓ Created tmux session 'grove-cli-feature-auth'
```

**Edge Cases:**

| Scenario | Behavior |
|----------|----------|
| Name already exists | Error with info about existing worktree (see above) |
| Branch exists, worktree doesn't | Create worktree with existing branch |
| Invalid name characters | Auto-transform and proceed with warning |
| Disk full | Error: "Failed to create worktree: {git error}" |
| No write permission | Error: "Cannot create worktree: permission denied at {path}" |
| Inside a worktree (not main) | Still works, creates sibling |
| Parent directory doesn't exist | Create parent directories |
| Tmux not installed | Warning, skip tmux session creation |
| Already have session with name | Attach to existing session |

**Exit Codes:**
- 0: Success (worktree created)
- 1: Worktree already exists
- 2: Git error (couldn't create worktree)
- 3: Invalid input

---

### grove to

**Purpose:** Switch to an existing worktree, activating its tmux session.

**Usage:**
```
grove to <name> [flags]

Arguments:
  name    Name of worktree to switch to (required)

Flags:
  -f, --force    Switch even if current worktree is dirty (stash changes)
```

**Behavior:**

1. **Find worktree:**
   - Search by short name first (within current project)
   - Fall back to full name match
   - Support partial matching if unambiguous

2. **Check current worktree status:**
   - If dirty and `dirty_handling = prompt`: Ask user
   - If dirty and `dirty_handling = auto-stash`: Stash automatically
   - If dirty and `dirty_handling = refuse`: Error unless `--force`

3. **Handle tmux session:**
   - If session exists and inside tmux: `tmux switch-client -t {session}`
   - If session exists and outside tmux: `tmux attach -t {session}`
   - If session doesn't exist: Create it, then attach/switch

4. **Change directory:**
   - If inside grove shell wrapper: Output `cd:{path}` directive
   - If direct execution: Include cd instruction in output

5. **Fire hooks:**
   - `pre-switch` on current worktree
   - `post-switch` on target worktree

6. **Update "last" tracking:**
   - Store current worktree as "last" for `grove last` command

**Output (Inside tmux, via shell wrapper):**
```
✓ Switched to 'testing'
```
*Shell wrapper changes directory automatically*

**Output (Inside tmux, direct execution):**
```
✓ Switched to tmux session 'grove-cli-testing'
```
*Tmux session switches, directory follows*

**Output (Outside tmux, via shell wrapper):**
```
✓ Attached to tmux session 'grove-cli-testing'
cd:/Users/egg/Work/grove-cli-testing
```
*New tmux client opens, shell changes directory after*

**Output (Outside tmux, direct execution):**
```
✓ Tmux session 'grove-cli-testing' ready

To attach: tmux attach -t grove-cli-testing
To enter directory: cd /Users/egg/Work/grove-cli-testing
```

**Output (Dirty worktree, prompt mode):**
```
⚠ Current worktree has uncommitted changes:

  M  src/main.go
  ?? src/new_file.go

What would you like to do?
  [s] Stash changes and switch
  [c] Commit changes first (opens editor)
  [a] Abort switch
  [f] Force switch (leave changes)

Choice [s/c/a/f]: 
```

**Output (Worktree not found):**
```
✗ Worktree 'testng' not found

Did you mean?
  • testing

Available worktrees:
  • main
  • testing
  • feature-auth

Create new: grove new testng
```
**Exit code: 1**

**Output (Ambiguous partial match):**
```
✗ Ambiguous worktree name 'test'

Matches:
  • testing
  • test-auth

Please be more specific.
```
**Exit code: 1**

**Edge Cases:**

| Scenario | Behavior |
|----------|----------|
| Already in target worktree | Message: "Already in 'testing'" (exit 0) |
| Worktree directory missing | Error: "Worktree directory missing. Run: grove repair testing" |
| Tmux not running | Start tmux server, create session, attach |
| Partial name matches one | Switch to it (e.g., `w to test` → `testing` if unambiguous) |
| Partial name matches multiple | Error with list of matches |
| Frozen worktree | Auto-resume, then switch |
| Session exists but wrong directory | Update session directory, then switch |

**Exit Codes:**
- 0: Success
- 1: Worktree not found
- 2: Aborted by user (dirty handling)
- 3: Other error

---

### grove rm

**Purpose:** Remove a worktree and its associated tmux session.

**Usage:**
```
grove rm <name> [flags]

Arguments:
  name    Name of worktree to remove (required)

Flags:
  -f, --force    Remove even if worktree has uncommitted changes
      --keep-branch    Don't delete the associated branch
```

**Behavior:**

1. **Find worktree** (same matching as `grove to`)

2. **Safety checks:**
   - Cannot remove current worktree (must switch away first)
   - Cannot remove if dirty (unless `--force`)
   - Cannot remove main worktree

3. **Kill tmux session** (if exists)

4. **Remove worktree:**
   ```bash
   git worktree remove <path>
   # or with --force:
   git worktree remove --force <path>
   ```

5. **Optionally delete branch** (unless `--keep-branch`):
   - Only if branch was created by grove
   - Only if branch is fully merged or user confirms

6. **Clean up:**
   - Remove from "last" tracking if it was last
   - Remove any grove metadata

**Output (Success):**
```
✓ Killed tmux session 'grove-cli-testing'
✓ Removed worktree 'testing'
✓ Deleted branch 'testing'
```

**Output (Has uncommitted changes):**
```
✗ Worktree 'testing' has uncommitted changes:

  M  src/main.go

To remove anyway: grove rm testing --force
To switch and commit: grove to testing
```
**Exit code: 1**

**Output (Trying to remove current):**
```
✗ Cannot remove current worktree

Switch to another worktree first: grove to main
```
**Exit code: 1**

**Output (Trying to remove main):**
```
✗ Cannot remove the main worktree

The main worktree is your primary project directory.
To remove the entire project, delete it manually.
```
**Exit code: 1**

**Output (Branch not fully merged):**
```
⚠ Branch 'testing' is not fully merged into 'main'

  3 commits ahead of main

Delete branch anyway? [y/N]: 
```

**Edge Cases:**

| Scenario | Behavior |
|----------|----------|
| Worktree doesn't exist | Error: "Worktree 'X' not found" |
| Directory already deleted | Clean up git worktree reference, report success |
| Tmux session doesn't exist | Skip, continue with worktree removal |
| Branch used by another worktree | Keep branch, just remove worktree |
| Currently in target worktree | Error: must switch away first |
| Removing "last" worktree | Clear "last" reference |

**Exit Codes:**
- 0: Success
- 1: Worktree not found or protected
- 2: Has uncommitted changes (without --force)
- 3: Other error

---

### grove here

**Purpose:** Show information about the current worktree.

**Usage:**
```
grove here [flags]

Flags:
  -q, --quiet    Just print the worktree name
  -j, --json     Output as JSON
```

**Behavior:**

1. Detect current worktree from `$PWD`
2. Gather status information
3. Display

**Output (Default):**
```
Worktree: testing
Project:  grove-cli
Branch:   testing
Path:     /Users/egg/Work/grove-cli-testing
Commit:   abc1234 (2 hours ago) Fix authentication bug
Status:   clean
Tmux:     attached (grove-cli-testing)
Docker:   running (3 containers)
```

**Output (Dirty):**
```
Worktree: testing
Project:  grove-cli
Branch:   testing
Path:     /Users/egg/Work/grove-cli-testing
Commit:   abc1234 (2 hours ago) Fix authentication bug
Status:   dirty
          M  src/auth.go
          ?? src/new_file.go
Tmux:     attached (grove-cli-testing)
Docker:   stopped
```

**Output (--quiet):**
```
testing
```

**Output (--json):**
```json
{
  "name": "testing",
  "fullName": "grove-cli-testing",
  "project": "grove-cli",
  "branch": "testing",
  "path": "/Users/egg/Work/grove-cli-testing",
  "commit": {
    "hash": "abc1234def5678",
    "shortHash": "abc1234",
    "message": "Fix authentication bug",
    "age": "2 hours ago"
  },
  "status": "dirty",
  "changes": ["M  src/auth.go", "?? src/new_file.go"],
  "tmux": {
    "session": "grove-cli-testing",
    "status": "attached"
  },
  "docker": {
    "status": "running",
    "containers": 3
  }
}
```

**Output (Not in a worktree):**
```
Not in a grove-managed worktree

Current directory: /Users/egg/Downloads

To see available worktrees: grove ls
To create a new worktree: grove new <name>
```
**Exit code: 1**

**Edge Cases:**

| Scenario | Behavior |
|----------|----------|
| In main project directory | Show main worktree info |
| In subdirectory of worktree | Detect parent worktree |
| Not in any git repo | Error with helpful message |
| In non-grove worktree | Show info but note "not managed by grove" |

**Exit Codes:**
- 0: Success (in a worktree)
- 1: Not in a worktree

---

### grove last

**Purpose:** Switch to the previously active worktree (like `cd -`).

**Usage:**
```
grove last [flags]

Flags:
  (inherits flags from 'grove to')
```

**Behavior:**

1. Read last worktree from state file (`~/.local/state/grove/last`)
2. If exists: equivalent to `grove to <last>`
3. If not exists: error

**State File Format:**
```
# ~/.local/state/grove/{project}/last
grove-cli-testing
```

**Output (Success):**
```
✓ Switched to 'testing' (previous worktree)
```

**Output (No previous):**
```
✗ No previous worktree recorded

This is your first switch in this session.
Use 'grove to <name>' to switch to a worktree.
```
**Exit code: 1**

**Output (Previous no longer exists):**
```
✗ Previous worktree 'old-feature' no longer exists

It may have been removed. Clearing history.
Use 'grove ls' to see available worktrees.
```
**Exit code: 1**

**Behavior Notes:**
- "Last" is tracked per-project
- Switching updates "last" to where you came FROM
- `grove last` twice in a row toggles between two worktrees
- Creating a new worktree and auto-switching updates "last"

**Exit Codes:**
- 0: Success
- 1: No previous worktree or it no longer exists

---

## State Commands

### grove freeze

**Purpose:** Freeze a worktree to conserve resources (stop containers, mark session).

**Usage:**
```
grove freeze [name] [flags]

Arguments:
  name    Worktree to freeze (default: current)

Flags:
      --all    Freeze all worktrees except current
```

**Behavior:**

1. Stop Docker containers (if docker plugin enabled)
2. Mark tmux session as frozen (custom variable)
3. Optionally detach from session
4. Record freeze state

**Output (Success):**
```
✓ Stopped 3 Docker containers
✓ Froze worktree 'testing'

To resume: grove resume testing
```

**Output (Already frozen):**
```
Worktree 'testing' is already frozen

To resume: grove resume testing
```

**Exit Codes:**
- 0: Success or already frozen
- 1: Worktree not found

---

### grove resume

**Purpose:** Resume a frozen worktree.

**Usage:**
```
grove resume <name> [flags]

Arguments:
  name    Worktree to resume (required)
```

**Behavior:**

1. Clear frozen state
2. Start Docker containers (if docker plugin)
3. Switch to worktree (equivalent to `grove to`)

**Output (Success):**
```
✓ Resumed worktree 'testing'
✓ Started 3 Docker containers
✓ Switched to 'testing'
```

**Output (Not frozen):**
```
Worktree 'testing' is not frozen

To switch to it: grove to testing
```

**Exit Codes:**
- 0: Success
- 1: Worktree not found or not frozen

---

## Docker Commands

**Note:** Docker commands are part of the Docker plugin (Phase 1). They should not be in the core binary for Phase 0.

### grove up

**Purpose:** Start Docker containers for the current worktree.

**Usage:**
```
grove up [services...] [flags]

Arguments:
  services    Specific services to start (default: all)

Flags:
  -d, --detach    Run in background (default: true)
      --build     Build images before starting
```

**Behavior:**

1. Find docker-compose.yml in worktree
2. Run `docker compose up` with worktree-specific project name
3. Use port allocation from .envrc

**Output:**
```
Starting containers for 'testing'...
  ✓ grove-cli-testing-db
  ✓ grove-cli-testing-redis
  ✓ grove-cli-testing-web

All containers running. Web available at http://localhost:3001
```

**Edge Cases:**

| Scenario | Behavior |
|----------|----------|
| No docker-compose.yml | Error: "No docker-compose.yml found" |
| Docker not running | Error: "Docker daemon not running" |
| Port conflict | Error with suggestion to check port allocation |
| Already running | Message: "Containers already running" |

---

### grove down

**Purpose:** Stop Docker containers for the current worktree.

**Usage:**
```
grove down [services...] [flags]

Arguments:
  services    Specific services to stop (default: all)

Flags:
  -v, --volumes    Also remove volumes
```

**Output:**
```
Stopping containers for 'testing'...
  ✓ grove-cli-testing-web
  ✓ grove-cli-testing-redis
  ✓ grove-cli-testing-db

All containers stopped.
```

---

### grove logs

**Purpose:** View Docker container logs.

**Usage:**
```
grove logs [service] [flags]

Arguments:
  service    Service to show logs for (default: all)

Flags:
  -f, --follow    Follow log output
  -n, --tail N    Number of lines to show (default: 100)
```

**Behavior:**

Pass through to `docker compose logs` with appropriate project name.

---

### grove restart

**Purpose:** Restart Docker containers.

**Usage:**
```
grove restart [services...] [flags]

Arguments:
  services    Services to restart (default: all)
```

---

## Utility Commands

### grove config

**Purpose:** Show or edit configuration.

**Usage:**
```
grove config [flags]

Flags:
      --edit     Open config file in $EDITOR
      --path     Just print config file path
  -j, --json     Output as JSON
```

**Output (Default):**
```
Configuration for 'grove-cli'

Global: ~/.config/grove/config.toml
Local:  /Users/egg/Work/grove-cli/.grove/config.toml (not found)

Settings:
  alias:            w
  projects_dir:     ~/projects
  default_branch:   main

Switch:
  dirty_handling:   prompt

Naming:
  pattern:          {project}-{name}
  max_length:       50

Tmux:
  prefix:           (none, uses naming pattern)
  layout:           default

Docker:
  enabled:          true
  auto_up:          true
  port_base:        3000
  port_range:       100
```

---

### grove init

**Purpose:** Generate shell integration code.

**Usage:**
```
grove init <shell>

Arguments:
  shell    Shell to generate for: zsh, bash
```

**Output (zsh):**
```bash
# Grove shell integration
# Add to ~/.zshrc: eval "$(grove init zsh)"

export GROVE_SHELL=1

grove() {
    local output
    output=$(__grove_bin "$@")
    local exit_code=$?
    
    # Handle cd directives
    if [[ "$output" == cd:* ]]; then
        cd "${output#cd:}"
    elif [[ -n "$output" ]]; then
        echo "$output"
    fi
    
    return $exit_code
}

# Alias
alias w='grove'

# Completions
source <(__grove_bin completion zsh)

__grove_bin() {
    command grove "$@"
}
```

**Usage Instructions:**
```
# Add to your shell config:

# For zsh (~/.zshrc):
eval "$(grove init zsh)"

# For bash (~/.bashrc):
eval "$(grove init bash)"
```

---

### grove repair

**Purpose:** Repair corrupted worktree state.

**Usage:**
```
grove repair [name] [flags]

Arguments:
  name    Worktree to repair (default: all)

Flags:
      --dry-run    Show what would be done
```

**Behavior:**

1. Check git worktree consistency
2. Verify directories exist
3. Verify tmux sessions match
4. Fix any discrepancies

**Output:**
```
Checking worktree 'testing'...
  ✓ Directory exists
  ✗ Tmux session missing → creating
  ✓ Git worktree valid
  
Repaired 1 issue.
```

---

### grove clean

**Purpose:** Remove old or orphaned worktrees.

**Usage:**
```
grove clean [flags]

Flags:
      --dry-run     Show what would be removed
      --older N     Remove worktrees not used in N days
  -f, --force       Don't prompt for confirmation
```

**Output:**
```
Found 3 worktrees to clean:

  old-feature     last used 45 days ago    dirty
  experiment      last used 30 days ago    clean
  temp            directory missing        -

Remove these worktrees? [y/N]: y

  ⚠ Skipped 'old-feature' (has uncommitted changes, use --force)
  ✓ Removed 'experiment'
  ✓ Cleaned up 'temp' reference

Cleaned 2 of 3 worktrees.
```

---

### grove doctor

**Purpose:** Diagnose common issues.

**Usage:**
```
grove doctor [flags]
```

**Output:**
```
Grove Doctor
============

Environment:
  ✓ Git version 2.40.0 (minimum: 2.30)
  ✓ Tmux version 3.3a (minimum: 3.0)
  ✓ Docker version 24.0.0
  ✗ Shell integration not detected
    → Add to ~/.zshrc: eval "$(grove init zsh)"

Configuration:
  ✓ Global config: ~/.config/grove/config.toml
  ✓ Config syntax valid

Project 'grove-cli':
  ✓ Git repository valid
  ✓ 4 worktrees found
  ⚠ 1 worktree has missing directory
    → Run: grove repair

Recommendations:
  1. Install shell integration for directory switching
  2. Run 'grove repair' to fix worktree issues
```

---

## Exit Codes

| Code | Meaning |
|------|---------|
| 0 | Success |
| 1 | User error (not found, invalid input) |
| 2 | Operation error (git failed, docker failed) |
| 3 | Permission error |
| 4 | Configuration error |
| 127 | Command not found |

---

## Output Formatting

### Colors (when TTY)

| Element | Color |
|---------|-------|
| Success (✓) | Green |
| Error (✗) | Red |
| Warning (⚠) | Yellow |
| Info | Default |
| Paths | Cyan |
| Commands | Bold |
| Emphasis | Bold |

### Icons

| Icon | Meaning |
|------|---------|
| ✓ | Success/complete |
| ✗ | Error/failed |
| ⚠ | Warning |
| • | Current item in list |
| → | Suggestion/action |

### No-TTY Mode

When output is not a TTY (piped, redirected):
- No colors
- No Unicode icons (use ASCII: [OK], [ERROR], [WARN], *, ->)
- No interactive prompts (fail with exit code)

Detect via: `os.Stdout.Fd()` and `term.IsTerminal()`

---

## Shell Integration Protocol

### cd Directive

When grove needs to change the shell's directory:

1. Grove outputs: `cd:/path/to/directory`
2. Shell wrapper captures output
3. Shell wrapper parses `cd:` prefix
4. Shell wrapper executes `cd /path/to/directory`
5. Any other output is echoed normally

**Implementation:**
```go
// In Go code
if shellIntegration {
    fmt.Printf("cd:%s\n", targetPath)
} else {
    fmt.Printf("To change directory: cd %s\n", targetPath)
}
```

```zsh
# In shell wrapper
grove() {
    local output
    output=$(__grove_bin "$@")
    local exit_code=$?
    
    if [[ "$output" == cd:* ]]; then
        cd "${output#cd:}"
    elif [[ -n "$output" ]]; then
        echo "$output"
    fi
    
    return $exit_code
}
```

### Environment Detection

Grove checks for shell wrapper:
```go
func hasShellIntegration() bool {
    return os.Getenv("GROVE_SHELL") == "1"
}
```

---

## State Storage

### Locations

| Data | Location | Format |
|------|----------|--------|
| Global config | `~/.config/grove/config.toml` | TOML |
| Project config | `.grove/config.toml` | TOML |
| Last worktree | `~/.local/state/grove/{project}/last` | Plain text |
| Worktree metadata | `.grove/worktrees/{name}.toml` | TOML |
| Freeze state | Tmux session variable `@grove_frozen` | Boolean |

### State File: last

```
# ~/.local/state/grove/grove-cli/last
grove-cli-testing
```

Updated on every `grove to` or `grove last` (before switch).

### State File: worktree metadata

```toml
# .grove/worktrees/testing.toml
[worktree]
name = "testing"
created = 2024-01-15T10:30:00Z
branch = "testing"
from = "main"

[time]
total_seconds = 14523
last_active = 2024-01-16T15:45:00Z
```
