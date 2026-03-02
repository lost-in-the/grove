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
   - [grove open](#grove-open)
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
   - [grove ps](#grove-ps)
6. [Worktree Flow Commands](#worktree-flow-commands)
   - [grove fork](#grove-fork)
   - [grove sync](#grove-sync)
   - [grove compare](#grove-compare)
   - [grove apply](#grove-apply)
   - [grove test](#grove-test)
7. [Utility Commands](#utility-commands)
   - [grove config](#grove-config)
   - [grove init](#grove-init)
   - [grove repair](#grove-repair)
   - [grove clean](#grove-clean)
   - [grove doctor](#grove-doctor)
8. [Exit Codes](#exit-codes)
9. [Output Formatting](#output-formatting)

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
| TUI mode | `$GROVE_CD_FILE` set | TUI writes target path to file; shell wrapper reads and applies cd |

**`GROVE_CD_FILE`:** For TUI invocations (`grove` with no args), the shell wrapper creates a temp file via `mktemp`, sets `GROVE_CD_FILE` to its path, and reads it after the TUI exits to perform the directory change. This is necessary because the TUI runs in alt-screen mode where stdout is not suitable for directive parsing.

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
~/projects/grove-cli/.git  →  project = "grove-cli"
~/projects/my-app/.grove/  →  project = (from config or "my-app")
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
~/projects/
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
• main          main            clean      attached    ~/projects/grove-cli
  testing       testing         clean      detached    ~/projects/grove-cli-testing
  feature-auth  feature/auth    dirty      none        ~/projects/grove-cli-feature-auth
  hotfix        main            clean      frozen      ~/projects/grove-cli-hotfix
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
~/projects/grove-cli
~/projects/grove-cli-testing
~/projects/grove-cli-feature-auth
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
      "path": "~/projects/grove-cli",
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

**Purpose:** Create a new worktree with associated tmux session and Docker stack.

**Aliases:** `spawn` (implies `--json` output)

**Usage:**
```
grove new <name> [flags]

Arguments:
  name    Name for the new worktree (required)

Flags:
  -j, --json           Output as JSON with switch_to field
      --mirror <ref>   Create environment worktree tracking a remote branch (e.g., origin/main)
      --no-docker      Skip Docker auto-start
```

**Behavior:**

1. **Validate name:**
   - Apply naming transformations
   - Check for existing worktree with same name
   
2. **Create worktree:**
   - If `--mirror` specified: verify remote branch exists, create environment worktree tracking it
   - Otherwise: create new worktree with branch matching the name
   ```bash
   git worktree add <path> <branch>
   ```

3. **Symlink config** from main worktree to new worktree directory.

4. **Register in state** (`AddWorktree`) with path, branch, created/accessed timestamps.

5. **Create tmux session:**
   - Session name = `{project}-{name}`
   - Start in worktree directory

6. **Execute post-create hooks** (user-configured and plugin hooks).

7. **Auto-start Docker** (unless `--no-docker`):
   - Only runs when `shouldAutoDocker()` returns true: agent stacks configured (`plugins.docker.external.agent.enabled = true`) or `plugins.docker.auto_up = true`
   - Calls `docker.Up()` for the new worktree path

**Output (Success):**
```
✓ Created worktree 'testing'
✓ Created tmux session 'grove-cli-testing'
✓ Docker stack started
```

**Output (Already Exists):**
```
✗ Worktree 'testing' already exists

  Path:   ~/projects/grove-cli-testing
  Branch: testing
  Status: clean

To switch to it: grove to testing
To remove it:    grove rm testing
```
**Exit code: 1**

**Output (Branch Exists, No Worktree):**
```
✓ Created worktree 'feature-auth' at ~/projects/grove-cli-feature-auth
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

### grove open

**Purpose:** Open a worktree session — create if needed, launch configured command, attach.

**Usage:**
```
grove open <name> [flags]

Arguments:
  name    Name of worktree to open (required)

Flags:
      --no-create     Only attach to existing worktree, error if not found
      --command <cmd>  Override session command
      --no-popup       Skip popup, use tmux switch/attach instead
  -j, --json           Output as JSON
```

**Behavior:**

1. **Ensure worktree exists:**
   - If worktree found: proceed
   - If not found and `--no-create` not set: create worktree (same as `grove new`)
   - If not found and `--no-create` set: error

2. **Ensure tmux session exists:**
   - Create session with configured command (from `[session]` config or `--command`)
   - If session already exists: check if command is running

3. **Launch configured command:**
   - If session has a shell and command not running: `tmux send-keys` the command
   - If command already running: skip (idempotent)

4. **Attach:**
   - If `popup = true` in config and inside tmux: `tmux display-popup`
   - If inside tmux: `tmux switch-client`
   - If outside tmux: `tmux attach-session`

**Configuration:**
```toml
[session]
command = "claude"     # What to run in sessions (default: "" = $SHELL)
popup = true           # Use tmux display-popup for grove open
popup_width = "80%"
popup_height = "80%"
```

**Output (New worktree):**
```
✓ Created worktree 'feature-x'
✓ Created session 'grove-cli-feature-x' running 'claude'
```

**Output (Existing worktree, reattach):**
```
✓ Launched 'claude' in existing session
```
*Popup opens or session switches*

**Edge Cases:**

| Scenario | Behavior |
|----------|----------|
| Worktree exists, session exists, command running | Reattach only (fully idempotent) |
| Worktree exists, no tmux session | Create session and launch |
| No `[session]` config | Opens shell session (same as `grove to`) |
| `--no-create` and worktree missing | Error with suggestion |
| Tmux not available | Emit cd directive, skip session management |
| Popup outside tmux | Fall back to tmux attach |

**Exit Codes:**
- 0: Success
- 1: Worktree not found (with `--no-create`)

---

### grove to

**Purpose:** Switch to an existing worktree, activating its tmux session.

**Usage:**
```
grove to <name> [flags]

Arguments:
  name    Name of worktree to switch to (required)

Flags:
  -j, --json   Output as JSON with switch_to field
      --peek   Lightweight switch: skip hooks (no Docker side effects)
```

**Behavior:**

1. **Find worktree:**
   - Search by short name first (within current project)
   - Fall back to full name match
   - Error if worktree is stale (directory missing)

2. **Handle tmux session:**
   - If session exists and inside tmux: `tmux switch-client -t {session}`
   - If session exists and outside tmux: `tmux attach -t {session}`
   - If session doesn't exist: Create it, then attach/switch

3. **Fire pre-switch hooks** (unless `--peek`).

4. **Switch tmux session** (if inside tmux): `tmux switch-client -t {session}`

5. **Fire post-switch hooks** (Docker start, etc.) before tmux switch so progress is visible in current session (unless `--peek`).

6. **Change directory:**
   - If inside tmux: session switch handles navigation
   - If shell integration active: Output `cd:{path}` directive, and `tmux-attach:` directive in auto mode
   - If direct execution: Print cd instructions

7. **Update "last" tracking:**
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
cd:~/projects/grove-cli-testing
```
*New tmux client opens, shell changes directory after*

**Output (Outside tmux, direct execution):**
```
✓ Tmux session 'grove-cli-testing' ready

To attach: tmux attach -t grove-cli-testing
To enter directory: cd ~/projects/grove-cli-testing
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
| Session exists but wrong directory | Detect drift and correct per `tmux.on_switch` setting |

**Directory Drift Detection:**

When switching to a worktree whose tmux session already exists (inside tmux), grove detects if the session's active pane has drifted from the worktree root. Behavior is controlled by `tmux.on_switch` config:

| `tmux.on_switch` | Behavior |
|------------------|----------|
| `"reset"` (default) | Send `cd "<worktree-path>"` to the session pane |
| `"warn"` | Print a warning about the drift, leave directory unchanged |
| `"ignore"` | Do nothing, leave directory unchanged |

Drift is only corrected when the pane is a shell (not running a program).

**Exit Codes:**
- 0: Success
- 1: Worktree not found
- 2: Other error

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
Path:     ~/projects/grove-cli-testing
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
Path:     ~/projects/grove-cli-testing
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
  "path": "~/projects/grove-cli-testing",
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

Current directory: ~/Downloads

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

1. Read last worktree from `.grove/state.json` (`State.LastWorktree` field)
2. If exists: equivalent to `grove to <last>`
3. If not exists: error

**State File Format:**
```json
// .grove/state.json (last_worktree field)
{
  "last_worktree": "testing"
}
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
      --isolated  Start an isolated stack (for parallel agents)
      --slot N    Use a specific slot number (implies --isolated)
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

### grove ps

**Purpose:** Show active Docker stacks.

**Aliases:** `agent-status` (hidden, backward compatibility)

**Usage:**
```
grove ps [flags]

Flags:
  -j, --json    Output as JSON (includes compose project names)
```

**Behavior:**

1. Read stack state from slot manager
2. Display results with `#N` reference IDs and URLs

**Output (Default):**
```
STACKS (2/5)

  #1  feature-x      http://localhost:3101
  #2  bugfix-y       http://localhost:3102
```

**Output (No active stacks):**
```
ℹ No active stacks
```

**Output (Not configured):**
```
Stacks not configured for this project

To enable, add to .grove/config.toml:

  [plugins.docker.external.agent]
  enabled = true
  services = ["app"]
  template_path = "agent-stacks/template.yml"
```

**Output (--json):**
```json
[
  {
    "slot": 1,
    "worktree": "feature-x",
    "composeProject": "myapp-agent-1",
    "url": "http://localhost:3101"
  }
]
```

**Edge Cases:**

| Scenario | Behavior |
|----------|----------|
| Stacks not configured | Help text with config example |
| No active stacks | Info message |

**Exit Codes:**
- 0: Success

---

## Worktree Flow Commands

### grove fork

**Purpose:** Fork the current worktree into a new one, branching from the current HEAD.

**Aliases:** `split`

**Usage:**
```
grove fork <name> [flags]

Arguments:
  name    Name for the new forked worktree (required)

Flags:
      --branch-name <name>    Override the generated branch name
      --move-wip              Move uncommitted changes to fork (current becomes clean)
      --copy-wip              Copy uncommitted changes to both worktrees
      --no-wip                Fork starts clean (leave changes in current)
  -n, --no-switch             Stay in current worktree after forking
  -j, --json                  Output as JSON
```

**Behavior:**

1. **Determine branch name:**
   - Default: `{current-branch}-{name}`
   - If `--branch-name` specified: use that name
   - Error if branch already exists

2. **Handle uncommitted changes (WIP):**
   - If WIP exists and no WIP flag provided: prompt user (interactive only)
   - `--move-wip`: capture patch, reset current, apply patch to fork
   - `--copy-wip`: capture patch, apply to fork without touching current
   - `--no-wip`: fork starts from clean HEAD
   - Non-interactive mode without WIP flag: error

3. **Create worktree** from current HEAD (or mirror ref for environment worktrees)

4. **Register in state** with parent worktree tracked

5. **Fire `post-create` hook**

6. **Create tmux session** with session name `{project}-{name}`

7. **Switch to fork** (unless `--no-switch`):
   - Update last_worktree before switching
   - Output `cd:` directive if shell integration present

**WIP Prompt (interactive):**
```
⚠ Uncommitted changes detected (3 files):
  M  src/auth.go
  ?? src/new_file.go
  M  src/user.go

How do you want to handle them?
  1. Move to fork (fork starts with changes, current becomes clean)
  2. Copy to fork (both have changes)
  3. Leave in current (fork starts clean)
  4. Cancel

Choice [1-4]:
```

**Output (Success):**
```
✓ Created worktree 'hotfix' with branch 'main-hotfix'
✓ Moved uncommitted changes to fork
✓ Created tmux session 'grove-cli-hotfix'
cd:~/projects/grove-cli-hotfix
```

**Output (--json):**
```json
{
  "name": "hotfix",
  "branch": "main-hotfix",
  "path": "~/projects/grove-cli-hotfix",
  "parent": "main",
  "created": true,
  "switch_to": "~/projects/grove-cli-hotfix"
}
```

**Edge Cases:**

| Scenario | Behavior |
|----------|----------|
| Branch name already exists | Error: "branch 'X' already exists" (exit ResourceExists) |
| Non-interactive with WIP | Error: "uncommitted changes detected; use --move-wip, --copy-wip, or --no-wip" |
| Environment worktree as parent | Forks from mirror branch HEAD, not current HEAD |
| `--move-wip` + `--copy-wip` | Error: flags are mutually exclusive |
| `--no-switch` | Print "To switch to the new worktree: grove to <name>" |

**Exit Codes:**
- 0: Success
- 1: User canceled (WIP prompt)
- 2: Branch already exists
- 3: Git operation failed

---

### grove sync

**Purpose:** Sync environment worktrees with their remote tracking branches (mirrors).

**Usage:**
```
grove sync [name] [flags]

Arguments:
  name    Environment worktree to sync (default: current worktree)

Flags:
      --all    Sync all environment worktrees
  -j, --json   Output as JSON
```

**Behavior:**

1. **Determine targets:**
   - If `--all`: sync all worktrees marked as environment worktrees in state
   - If `<name>` given: sync that specific worktree
   - If neither: sync current worktree (must be an environment worktree)

2. **For each target:**
   - Verify it is an environment worktree with a mirror configured
   - Capture current HEAD commit
   - Run `git fetch --prune`
   - Run `git merge --ff-only <mirror>` (fast-forward only — no merges)
   - Capture new HEAD commit
   - Count commits synced
   - Update `last_synced_at` in state

3. **Output results** (or skip if --json)

**Output (Up to date):**
```
✓ 'production' is up to date with origin/main
```

**Output (Updated):**
```
✓ Synced 'production' (origin/main) - 3 new commit(s)
```

**Output (--json):**
```json
{
  "synced": [
    {
      "name": "production",
      "mirror": "origin/main",
      "old_commit": "abc1234",
      "new_commit": "def5678",
      "commits_ahead": 3
    }
  ],
  "skipped": []
}
```

**Edge Cases:**

| Scenario | Behavior |
|----------|----------|
| Current worktree is not an environment worktree | Error with suggestion to use `grove sync <name>` or `--all` |
| Named worktree is not an environment worktree | Error: "not an environment worktree" |
| Worktree has local commits (fast-forward fails) | Skip with warning: "fast-forward failed (may have local changes)" |
| Fetch fails (network error) | Skip with warning, continue with other targets |
| No environment worktrees with `--all` | Message: "No environment worktrees found." |

**Exit Codes:**
- 0: Success
- 1: Not an environment worktree (ConstraintViolated)
- 2: Git operation failed

---

### grove compare

**Purpose:** Compare the current worktree with another, showing commit and WIP differences.

**Usage:**
```
grove compare <name> [flags]

Arguments:
  name    Target worktree to compare against (required)

Flags:
      --stat        Show diffstat summary
      --committed   Show only committed differences (commits ahead)
      --wip         Show only uncommitted differences
  -j, --json        Output as JSON
```

**Behavior:**

1. Get current worktree and find target worktree by name
2. Determine what to show:
   - Default (no flags): show both commits and WIP
   - `--committed`: commits only
   - `--wip`: uncommitted changes only
   - `--stat`: also include diffstat
3. **Commits:** `git log {target-branch}..HEAD` — commits in current not in target
4. **WIP:** staged, unstaged, and untracked files in current worktree
5. **Stats:** `git diff --stat {target-branch}` for file/line counts

**Output (Default):**
```
Comparing main → testing
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

Commits ahead of testing (2):
  abc1234 feat: add user authentication (2 hours ago)
  def5678 fix: handle edge case in login (1 hour ago)

Uncommitted changes:
  Staged (1):
    + src/new_feature.go
  Unstaged (2):
    M src/auth.go
    M src/user.go
  Untracked (1):
    ? tmp/debug.log
```

**Output (No differences):**
```
Comparing main → feature
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
No differences found
```

**Output (--json):**
```json
{
  "current": "main",
  "target": "feature",
  "commits": [
    {
      "sha": "abc1234...",
      "message": "feat: add user authentication",
      "author": "Jane Dev",
      "age": "2 hours ago"
    }
  ],
  "wip": {
    "staged": ["src/new_feature.go"],
    "unstaged": ["src/auth.go"],
    "untracked": ["tmp/debug.log"]
  },
  "stats": null,
  "has_diff": true
}
```

**Edge Cases:**

| Scenario | Behavior |
|----------|----------|
| Target worktree not found | Error: "worktree 'X' not found" |
| `--committed` and `--wip` together | Error: flags are mutually exclusive |
| Diverged branches (no common ancestor) | Commits section may be empty with no error |

**Exit Codes:**
- 0: Success (differences found or not)
- 1: Worktree not found or error

---

### grove apply

**Purpose:** Apply commits or uncommitted changes from another worktree to the current one.

**Usage:**
```
grove apply <name> [flags]

Arguments:
  name    Source worktree to apply changes from (required)

Flags:
      --commits         Apply only committed changes (cherry-pick)
      --wip             Apply only uncommitted changes (patch)
      --pick <sha,...>  Apply specific commit(s) by SHA
      --dry-run         Show what would be applied without making changes
  -j, --json            Output as JSON
```

**Behavior:**

1. **Validate target (current worktree):**
   - Check if target is immutable (configured in `.grove/config.toml` under `[protection]`)
   - Error if immutable

2. **Find source worktree** by name

3. **Determine what to apply:**
   - Default (no flags): apply both commits and WIP
   - `--commits`: cherry-pick commits since common ancestor
   - `--wip`: apply uncommitted changes as patch
   - `--pick <sha>`: cherry-pick specific commits by SHA (comma-separated)
   - `--pick` implies `--commits` only (no WIP)

4. **Apply commits:**
   - Find merge base between current and source branch
   - `git cherry-pick` each commit from source since merge base
   - On conflict: error with instructions to resolve or abort

5. **Apply WIP:**
   - Create patch from source's uncommitted changes
   - Apply with `git apply` to current worktree

**Output (Default):**
```
Applying changes from feature-auth → main
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

Commits to apply (2):
  abc1234 feat: add user authentication
  def5678 fix: handle edge case in login

Applying commits...
  ✓ abc1234
  ✓ def5678

Uncommitted changes to apply (1 files):
  M src/work-in-progress.go

✓ Uncommitted changes applied

✓ Changes applied successfully
```

**Output (--dry-run):**
```
Applying changes from feature-auth → main
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

Commits to apply (2):
  abc1234 feat: add user authentication
  def5678 fix: handle edge case in login

Uncommitted changes to apply (1 files):
  M src/work-in-progress.go

[Dry run - no changes were made]
```

**Output (Cherry-pick conflict):**
```
✗ Conflict applying abc1234

[git output]

To resolve:
  1. Fix conflicts in the affected files
  2. git add <resolved-files>
  3. git cherry-pick --continue

Or to abort:
  git cherry-pick --abort
```
**Exit code: GitOperationFailed**

**Edge Cases:**

| Scenario | Behavior |
|----------|----------|
| Target is immutable | Error: "worktree 'X' is immutable and cannot receive changes" |
| Source not found | Error: "worktree 'X' not found" |
| No commits to apply | Message: "No commits to apply" |
| No WIP in source | Message: "No uncommitted changes to apply" |
| `--commits` and `--wip` together | Error: flags are mutually exclusive |
| Cherry-pick conflict | Error with instructions; exit GitOperationFailed |

**Exit Codes:**
- 0: Success
- 1: Target is immutable (ConstraintViolated)
- 2: Source worktree not found (ResourceNotFound)
- 3: Git operation failed (cherry-pick conflict, patch failure)

---

### grove test

**Purpose:** Run the configured test command in a worktree's directory.

**Usage:**
```
grove test <name> [args...] [flags]

Arguments:
  name      Name of worktree to run tests in (required)
  args...   Extra arguments appended to the configured test command
```

**Configuration:**

The test command is configured in `.grove/config.toml`:

```toml
[test]
command = "bin/rails test"

# Optional: run in a Docker service container
service = "app"
```

**Behavior:**

1. Verify `[test] command` is configured — error if missing
2. Find target worktree by name
3. Append extra `args` to the configured command
4. **Local mode** (no `service` configured):
   - Run command directly in the worktree directory via `sh -c`
   - stdout/stderr/stdin pass through
5. **Docker mode** (`service` configured):
   - Use Docker plugin's `Run()` to execute in an ephemeral container
   - The container mounts the worktree directory
6. Exit with the same exit code as the test command

**Examples:**
```bash
# Run all tests in another worktree
grove test my-feature

# Run specific test file (appended to configured command)
grove test my-feature spec/models/user_spec.rb

# Run with glob (expanded in worktree context)
grove test my-feature 'spec/**/*_spec.rb'
```

**Output:**
Test output passes through directly (stdout/stderr passthrough). No grove-specific output is added.

**Error (no test command configured):**
```
no test command configured

Add a [test] section to .grove/config.toml:

  [test]
  command = "bin/rails test"
```

**Edge Cases:**

| Scenario | Behavior |
|----------|----------|
| No `[test] command` in config | Error with instructions to configure |
| Worktree not found | Error: "worktree 'X' not found" |
| Test command exits non-zero | grove exits with same exit code |
| Docker mode, docker unavailable | Docker plugin initialization error |

**Exit Codes:**
- 0: Tests pass
- N: Same exit code as the test command
- 1: Configuration or worktree not found error

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
Local:  ~/projects/grove-cli/.grove/config.toml (not found)

Settings:
  alias:            w
  projects_dir:     ~/projects
  default_branch:   main

Switch:
  dirty_handling:   prompt

Naming:
  pattern:          {type}/{description}
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

**Purpose:** Remove worktrees that haven't been accessed recently.

**Usage:**
```
grove clean [flags]

Flags:
      --older-than N      Remove worktrees not accessed in N days (default: 30)
      --include-dirty     Include dirty worktrees (default: excluded)
      --dry-run           Show what would be cleaned without making changes
      --keep-branches     Do not delete associated branches
      --delete-branches   Delete associated branches without prompting
```

**Always excluded from cleanup:**
- Root/main worktree
- Current worktree
- Protected worktrees (configured in `config.toml`)
- Environment worktrees (created with `--mirror`)
- Dirty worktrees (unless `--include-dirty`)

**Behavior:**

1. List all worktrees; apply exclusion rules
2. Display eligible worktrees
3. If `--dry-run`: show list and exit (no changes)
4. **Always prompt** for confirmation (type `yes` to proceed — required even in non-interactive mode)
5. For each cleanable worktree:
   - Run `grove rm`-equivalent (removes worktree + state entry)
   - Kill tmux session if exists
6. After removal: prompt to delete associated branches
   - `--delete-branches`: delete without prompting
   - `--keep-branches`: skip branch deletion
   - Default: interactive prompt showing branch merge status

**Output:**
```
Found 2 worktree(s) eligible for cleanup:

  old-feature (feature/auth) - 45 days since last access [dirty]
  experiment (experiment) - 32 days since last access

This will permanently remove 2 worktree(s) and their associated tmux sessions.
Type 'yes' to confirm: yes

  Removed 'old-feature'
  Removed 'experiment'

Cleanup complete: 2 removed, 0 failed

Associated branches:
  ⚠ feature/auth (2 unpushed commits)
  • experiment (merged, safe to delete)

Delete 2 associated branch(es)? [Y/n]: y
  Deleted branch 'feature/auth'
  Deleted branch 'experiment'
Deleted 2 branch(es)
```

**Output (Nothing eligible):**
```
No worktrees eligible for cleanup.

Excluded worktrees:
  main - root worktree
  testing - accessed 5 days ago (threshold: 30)
  production - environment worktree
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

