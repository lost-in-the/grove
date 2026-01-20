# Grove-CLI Tool Breakdown

Comprehensive documentation of every Grove command's behavior, system changes, and operational safety analysis.

**Generated**: 2026-01-20
**Grove Version**: 0.1.0-dev

---

## Table of Contents

- [Overview](#overview)
- [Core Lifecycle Commands](#core-lifecycle-commands)
  - [grove init](#grove-init)
  - [grove new](#grove-new)
  - [grove rm](#grove-rm)
  - [grove ls](#grove-ls)
  - [grove here](#grove-here)
- [Navigation Commands](#navigation-commands)
  - [grove to](#grove-to)
  - [grove last](#grove-last)
  - [grove freeze](#grove-freeze)
  - [grove resume](#grove-resume)
- [Git Operations](#git-operations)
  - [grove fetch](#grove-fetch)
  - [grove up](#grove-up)
  - [grove down](#grove-down)
  - [grove logs](#grove-logs)
  - [grove browse (issues/prs)](#grove-browse)
- [Session & Config](#session--config)
  - [grove restart](#grove-restart)
  - [grove time](#grove-time)
  - [grove config](#grove-config)
  - [grove version](#grove-version)
- [Internal Architecture](#internal-architecture)
  - [Package Overview](#package-overview)
  - [worktree Package](#worktree-package)
  - [tmux Package](#tmux-package)
  - [shell Package](#shell-package)
  - [state Package](#state-package)
  - [hooks Package](#hooks-package)
  - [config Package](#config-package)
  - [plugins Package](#plugins-package)
- [Plugins](#plugins)
  - [Docker Plugin](#docker-plugin)
  - [Time Plugin](#time-plugin)
  - [Tracker Plugin](#tracker-plugin)
- [Safety Summary](#safety-summary)

---

## Overview

Grove is a CLI tool for managing git worktrees with tmux integration. It provides:

- **Worktree lifecycle management** (create, list, remove, switch)
- **Tmux session automation** (auto-create sessions per worktree)
- **Shell integration** (directory switching via `cd:` protocol)
- **State persistence** (frozen worktrees, time tracking)
- **Plugin system** (Docker, time tracking, GitHub integration)

### Naming Convention

All worktrees follow the `{project}-{name}` pattern:
- Directory: `grove-cli-testing`
- Tmux session: `grove-cli-testing`
- Display name: `testing` (short form)

### Shell Integration Protocol

Commands that change directories output `cd:/path/to/dir` which the shell wrapper intercepts when `GROVE_SHELL=1` is set.

---

## Core Lifecycle Commands

### grove init

**Purpose:** Generate shell integration code for zsh or bash.

**Dependencies:** `internal/shell`

#### Flowchart

```mermaid
flowchart TD
    A["grove init [shell]"] -->|shell arg provided| B{shell type?}
    A -->|no shell arg| C["Return help message"]

    B -->|zsh| D["Call shell.GenerateZshIntegration"]
    B -->|bash| E["Call shell.GenerateBashIntegration"]
    B -->|invalid| F["Return error: unsupported shell"]

    D --> G["Read zsh template from embedded file"]
    E --> H["Read bash template from embedded file"]

    G --> I["Inject binary path and header into template"]
    H --> I

    I --> J["Print integration script to stdout"]
    J --> K["Exit 0"]
    C --> L["Exit 0 with help"]
    F --> M["Exit 1 with error"]
```

#### System Changes

| Change Type | Description | Condition |
|-------------|-------------|-----------|
| Stdout | Prints shell integration script | Always (when successful) |
| No Files | No persistent changes on disk | Command is read-only output |
| No Tmux | Does not interact with tmux | N/A |
| No Git | Does not interact with git | N/A |

#### Operational Safety

**Data Loss Risk:** None

**What can go wrong:**
- Binary path resolution fails (falls back to "grove" in PATH)
- Invalid shell type specified (returns error with list of supported shells)

**Edge Cases:**
- Already integrated into shell config (re-running is safe, idempotent)

#### Flags and Options

| Flag | Type | Effect |
|------|------|--------|
| `[shell]` | Positional arg | Shell type: `zsh` or `bash` (required) |

---

### grove new

**Purpose:** Create a new git worktree with a new branch and optional tmux session.

**Dependencies:** `internal/worktree`, `internal/tmux`

#### Flowchart

```mermaid
flowchart TD
    A["grove new <name>"] --> B{name empty?}
    B -->|yes| C["Error: name cannot be empty"]
    B -->|no| D["Initialize WorktreeManager"]

    D --> E{Manager init failed?}
    E -->|yes| F["Error: failed to initialize"]
    E -->|no| G["Check if worktree already exists"]

    G --> H{Worktree exists?}
    H -->|yes| I["Error: worktree already exists"]
    H -->|no| J["Get full worktree name<br/>Format: {project}-{name}"]

    J --> K["Execute: git worktree add<br/>-b {name} {path}/{fullname}"]
    K --> L{Create successful?}
    L -->|no| M["Error: failed to create"]
    L -->|yes| N["Print success message"]

    N --> O{tmux available?}
    O -->|no| P["Exit 0"]
    O -->|yes| Q["Generate tmux session name"]

    Q --> R["Execute: tmux new-session<br/>-d -s {session} -c {path}"]
    R --> S{Session creation failed?}
    S -->|yes| T["Print warning"]
    S -->|no| U["Print success: session created"]

    T --> V["Exit 0"]
    U --> V
```

#### System Changes

| Change Type | Description | Condition |
|-------------|-------------|-----------|
| Directory | Creates new worktree directory | Always on success |
| Git | Executes `git worktree add -b {name}` | Always on success |
| Git | Creates new branch with worktree | Always on success |
| Tmux | Creates detached tmux session | When tmux available |
| Filesystem | All files copied from main worktree | Git operation side effect |

#### Operational Safety

**Data Loss Risk:** Low
- Only creates new resources, no deletion
- Idempotency check prevents accidental overwrites

**What can go wrong:**
- Name already exists (error with recovery suggestions)
- Not in a git repo (WorktreeManager initialization fails)
- Permission denied on parent directory
- Disk full

**Recovery:**
- Use `grove rm <name>` to clean up partial creation

#### Flags and Options

| Flag | Type | Effect |
|------|------|--------|
| `<name>` | Positional arg | Worktree name (becomes branch name) |

---

### grove rm

**Purpose:** Remove a git worktree and kill its associated tmux session.

**Dependencies:** `internal/worktree`, `internal/tmux`

#### Flowchart

```mermaid
flowchart TD
    A["grove rm <name>"] --> B{name empty?}
    B -->|yes| C["Error: name cannot be empty"]
    B -->|no| D["Initialize WorktreeManager"]

    D --> E{Manager init failed?}
    E -->|yes| F["Error: failed to initialize"]
    E -->|no| G{tmux available?}

    G -->|no| H["Skip tmux session removal"]
    G -->|yes| I["Check if tmux session exists"]

    I --> J{Session exists?}
    J -->|no| K["Skip killing session"]
    J -->|yes| L["Execute: tmux kill-session -t {session}"]

    L --> M{Kill successful?}
    M -->|no| N["Print warning"]
    M -->|yes| O["Print success: killed session"]

    H --> P["Execute: git worktree remove {path}"]
    K --> P
    N --> P
    O --> P

    P --> Q{Remove successful?}
    Q -->|no| R["Try force remove: git worktree remove --force"]
    Q -->|yes| S["Print success: removed"]

    R --> T{Force remove successful?}
    T -->|no| U["Error: failed to remove"]
    T -->|yes| S

    S --> V["Exit 0"]
```

#### System Changes

| Change Type | Description | Condition |
|-------------|-------------|-----------|
| Directory | Deletes worktree directory | When git remove succeeds |
| Git | Executes `git worktree remove {path}` | Always attempted first |
| Git | Executes `git worktree remove --force` | If normal remove fails |
| Git | Executes `git worktree prune` | If worktree is prunable (stale) |
| Tmux | Kills tmux session | When session exists and tmux available |

#### Operational Safety

**Data Loss Risk:** HIGH (destructive)
- Deletes entire worktree directory and uncommitted changes
- **NO confirmation prompt**
- **NO backup created**

**What can go wrong:**
- Worktree has uncommitted changes (deleted without warning)
- Terminal inside removed worktree (shell may break)
- Multiple users accessing same worktree

**Recovery:**
- Uncommitted changes: Lost unless backed up externally
- Use `git reflog` to recover deleted branch refs

#### Flags and Options

| Flag | Type | Effect |
|------|------|--------|
| `<name>` | Positional arg | Worktree name to remove |

**Aliases:** `grove remove`, `grove delete`

---

### grove ls

**Purpose:** List all worktrees with status, branch info, and tmux session status.

**Dependencies:** `internal/worktree`, `internal/tmux`, `internal/state`

#### Flowchart

```mermaid
flowchart TD
    A["grove ls [flags]"] --> B["Initialize WorktreeManager"]
    B --> C{Manager init failed?}
    C -->|yes| D["Error: failed to initialize"]
    C -->|no| E["Execute: git worktree list --porcelain"]

    E --> F{List failed?}
    F -->|yes| G["Error: failed to list"]
    F -->|no| H{Any worktrees?}

    H -->|no| I{Quiet/Path/JSON?}
    I -->|yes| J["Silent exit"]
    I -->|no| K["Print: No worktrees found"]

    H -->|yes| L["Get project name, current worktree"]
    L --> M["Initialize state manager for frozen status"]

    M --> N{tmux available?}
    N -->|yes| O["Fetch tmux sessions"]
    N -->|no| P["Skip tmux status"]

    O --> Q["Filter frozen worktrees (--all flag?)"]
    P --> Q

    Q --> R{Output format?}
    R -->|--paths| S["Print path per line"]
    R -->|--quiet| T["Print name per line"]
    R -->|--json| U["Format JSON output"]
    R -->|default| V["Format table output"]

    S --> W["Exit 0"]
    T --> W
    U --> W
    V --> W
```

#### System Changes

| Change Type | Description | Condition |
|-------------|-------------|-----------|
| Stdout | Formatted list output | Always on success |
| No Modifications | Read-only operation | All modes |

#### Operational Safety

**Data Loss Risk:** None (read-only)

**Status Indicators:**
- `●` = current worktree
- `clean` = no uncommitted changes
- `dirty` = uncommitted changes present
- `stale` = directory missing (prunable)
- Tmux: `attached` / `detached` / `frozen` / `none`

#### Flags and Options

| Flag | Short | Type | Default | Effect |
|------|-------|------|---------|--------|
| `--all` | `-a` | Boolean | false | Include frozen worktrees |
| `--paths` | `-p` | Boolean | false | Show full paths only |
| `--json` | `-j` | Boolean | false | Output as JSON |
| `--quiet` | `-q` | Boolean | false | Names only, one per line |

---

### grove here

**Purpose:** Display detailed information about the current worktree.

**Dependencies:** `internal/worktree`, `internal/tmux`

#### Flowchart

```mermaid
flowchart TD
    A["grove here [flags]"] --> B["Initialize WorktreeManager"]
    B --> C{Manager init failed?}
    C -->|yes| D["Error: failed to initialize"]
    C -->|no| E["Get current worktree"]

    E --> F{Get current failed?}
    F -->|yes| G["Error: not in a worktree"]
    F -->|no| H["Extract display name"]

    H --> I{--quiet flag?}
    I -->|yes| J["Print name, Exit 0"]
    I -->|no| K["Get project name, session name"]

    K --> L["Check session status"]
    L --> M["Gather commit info: git log -1"]
    M --> N["Get dirty files if dirty"]

    N --> O{--json flag?}
    O -->|yes| P["Format JSON output"]
    O -->|no| Q["Format text output"]

    P --> R["Exit 0"]
    Q --> R
```

#### System Changes

| Change Type | Description | Condition |
|-------------|-------------|-----------|
| Stdout | Formatted display of current worktree info | Always |
| Git Operations | `git log -1` for commit info | To gather display data |
| Git Operations | `git status --porcelain` for dirty files | Only if worktree is dirty |

#### Operational Safety

**Data Loss Risk:** None (read-only)

#### Flags and Options

| Flag | Short | Type | Default | Effect |
|------|-------|------|---------|--------|
| `--quiet` | `-q` | Boolean | false | Just print worktree name |
| `--json` | `-j` | Boolean | false | Output as JSON |

---

## Navigation Commands

### grove to

**Purpose:** Switch to a worktree by short name, create tmux session if needed, and output directory path for shell integration.

**Dependencies:** `internal/worktree`, `internal/tmux`, `internal/hooks`

#### Flowchart

```mermaid
flowchart TD
    A["grove to <name>"] --> B{Name empty?}
    B -->|Yes| C["Error: empty name"]
    B -->|No| D["Init worktree Manager"]

    D --> E{Manager init succeeds?}
    E -->|No| F["Error: manager init failed"]
    E -->|Yes| G["Find worktree by name"]

    G --> H{Worktree found?}
    H -->|No| I["Error: worktree not found"]
    H -->|Yes| J{Worktree stale?}

    J -->|Yes| K["Error: stale, run 'grove rm'"]
    J -->|No| L["Get current worktree"]

    L --> M["Fire PreSwitch hook"]
    M --> N{Inside tmux?}
    N -->|Yes| O["Store current session as last"]
    N -->|No| O

    O --> P{Tmux available?}
    P -->|No| Q["Output only directory path"]
    P -->|Yes| R["Generate tmux session name"]

    R --> S["Check if session exists"]
    S --> T{Session exists?}
    T -->|No| U["Create new tmux session"]
    T -->|Yes| V["Session ready"]

    U --> W{Inside tmux?}
    V --> W
    W -->|Yes| X["Switch to session"]
    W -->|No| Y["Show attach instructions"]

    X --> Z["Check GROVE_SHELL env"]
    Y --> Z

    Z --> AA{GROVE_SHELL=1?}
    AA -->|Yes| AB["Output: cd:/path/to/worktree"]
    AA -->|No| AC["Output: shell integration help"]

    AB --> AD["Fire PostSwitch hook"]
    AC --> AD
    AD --> AE["Success return"]
```

#### System Changes

| Change Type | Description | Condition |
|-------------|-------------|-----------|
| State | Current session stored in `~/.config/grove/last_session` | When inside tmux |
| Tmux | New session created with name `{project}-{name}` | When session doesn't exist |
| Tmux | Switches to target session | When inside tmux |
| Directory | Outputs `cd:/path` protocol line | When `GROVE_SHELL=1` |
| Hooks | Fires `pre-switch` and `post-switch` hooks | Always |

#### Operational Safety

**Data Loss Risk:** Low
- No destructive operations
- Directory switching is non-destructive

**What can go wrong:**
- Stale worktree directory removed externally (caught by `IsPrunable` check)
- Tmux session already exists (properly checked before creation)
- Shell integration not configured (falls back to helpful instructions)

#### Flags and Options

| Flag | Type | Effect |
|------|------|--------|
| `<name>` | Positional arg | Worktree short name to switch to |
| `GROVE_SHELL` | Env var | When `=1`, outputs `cd:/path` for shell wrapper |

---

### grove last

**Purpose:** Switch to the last worktree you were working in (tmux session-based history).

**Dependencies:** `internal/tmux`, `internal/worktree`

#### Flowchart

```mermaid
flowchart TD
    A["grove last"] --> B["Retrieve last session from storage"]
    B --> C{Session file exists?}
    C -->|No| D["Error: no last session found"]
    C -->|Yes| E["Parse session name"]

    E --> F["Init worktree Manager"]
    F --> G{Manager init succeeds?}
    G -->|No| H["Error: manager init failed"]
    G -->|Yes| I["Extract worktree name from session"]

    I --> J["List all worktrees"]
    J --> K["Search for worktree by name"]

    K --> L{Worktree found?}
    L -->|No| M["Error: worktree not found"]
    L -->|Yes| N{Inside tmux?}

    N -->|Yes| O["Store current session as last"]
    N -->|No| O

    O --> P{Tmux available?}
    P -->|Yes| Q["Switch to stored session name"]
    P -->|No| R["Check GROVE_SHELL env"]

    Q --> R
    R --> S{GROVE_SHELL=1?}
    S -->|Yes| T["Output: cd:/path/to/worktree"]
    S -->|No| U["Output: shell integration help"]

    T --> V["Success"]
    U --> V
```

#### System Changes

| Change Type | Description | Condition |
|-------------|-------------|-----------|
| State | Current session stored in `~/.config/grove/last_session` | When inside tmux |
| Tmux | Switches to stored last session | When inside tmux |
| Directory | Outputs `cd:/path` protocol line | When `GROVE_SHELL=1` |

#### Operational Safety

**Data Loss Risk:** Low
- Read-only operation (no state modification except last-session tracking)

**What can go wrong:**
- No previous session history (file doesn't exist on first use)
- Last session doesn't match any worktree (worktree was deleted)

**Edge Cases:**
- Using `last` when not inside tmux (still works if GROVE_SHELL=1)
- Switching back and forth (each call updates last_session)

---

### grove freeze

**Purpose:** Mark a worktree as frozen (inactive) and stop related services like Docker containers.

**Dependencies:** `internal/state`, `internal/worktree`, `internal/hooks`, `plugins/docker`

#### Flowchart

```mermaid
flowchart TD
    A["grove freeze [name] [--all]"] --> B{Args analysis}
    B -->|--all flag set| C["Collect all except current"]
    B -->|name provided| D["Collect single named worktree"]
    B -->|no args| E["Collect current worktree only"]

    C --> F["Init state manager"]
    D --> F
    E --> F

    F --> G{Docker plugin enabled?}
    G -->|Yes| H["Docker plugin ready"]
    G -->|No| I["Docker plugin skipped"]

    H --> J["For each target worktree"]
    I --> J

    J --> K["Fire pre-freeze hook"]
    K --> L{Docker available?}

    L -->|Yes| M["Stop containers"]
    L -->|No| N["Skip docker"]

    M --> O["Mark frozen in state"]
    N --> O

    O --> P{State save succeeds?}
    P -->|No| Q["Error: state save failed"]
    P -->|Yes| R["Frozen message"]

    R --> S["Print summary"]
    S --> T["Done"]
```

#### System Changes

| Change Type | Description | Condition |
|-------------|-------------|-----------|
| State | Worktree added to `~/.config/grove/state/frozen.json` | For each targeted worktree |
| Hooks | Fires `pre-freeze` hook | For each worktree |
| Docker | Stops containers via `docker-compose down` | When docker plugin enabled |

#### Operational Safety

**Data Loss Risk:** Low (state change is idempotent)
- Freeze is purely metadata marking
- Docker stop is reversible via `resume`

**What can go wrong:**
- Docker containers fail to stop (logged as warning, freeze still completes)
- Hook execution fails (logged as warning, freeze still completes)

#### Flags and Options

| Flag | Type | Effect |
|------|------|--------|
| `[name]` | Optional positional | Specific worktree to freeze |
| `--all` | Boolean | Freeze all except current |

---

### grove resume

**Purpose:** Clear frozen state for a worktree and restart related services like Docker containers.

**Dependencies:** `internal/state`, `internal/worktree`, `internal/hooks`, `internal/tmux`, `plugins/docker`

#### Flowchart

```mermaid
flowchart TD
    A["grove resume <name>"] --> B{Name empty?}
    B -->|Yes| C["Error: empty name"]
    B -->|No| D["Init worktree Manager"]

    D --> E["Find worktree by name"]
    E --> F{Worktree found?}
    F -->|No| G["Error: worktree not found"]
    F -->|Yes| H["Clear frozen state"]

    H --> I["Fire post-resume hook"]
    I --> J{Docker plugin enabled?}

    J -->|Yes| K["Start containers"]
    J -->|No| L["Skip docker"]

    K --> M["Print resumed message"]
    L --> M

    M --> N{Tmux available?}
    N -->|No| O["Check GROVE_SHELL"]
    N -->|Yes| P["Create/switch to session"]

    P --> O
    O --> Q{GROVE_SHELL=1?}
    Q -->|Yes| R["Output: cd:/path"]
    Q -->|No| S["Output: shell help"]

    R --> T["Success"]
    S --> T
```

#### System Changes

| Change Type | Description | Condition |
|-------------|-------------|-----------|
| State | Worktree removed from frozen set | Always |
| Hooks | Fires `post-resume` hook | After state cleared |
| Docker | Starts containers via `docker-compose up` | When docker plugin enabled |
| Tmux | Session created if doesn't exist | When tmux available |
| Directory | Outputs `cd:/path` | When `GROVE_SHELL=1` |

#### Operational Safety

**Data Loss Risk:** Low
- Reverse operation of freeze
- Docker start is safe if containers already running (idempotent)

---

## Git Operations

### grove fetch

**Purpose:** Create worktree from GitHub issue or pull request with automatic naming and branch checkout.

**Dependencies:** `internal/worktree`, `internal/tmux`, `plugins/tracker`

#### Flowchart

```mermaid
flowchart TD
    A["grove fetch pr/123 or issue/456"] --> B["Parse Argument"]
    B --> C{Valid Format?}
    C -->|Invalid| D["Error: Invalid Format"]

    C -->|Valid| E["Check gh CLI Installed"]
    E --> F{gh Available?}
    F -->|No| G["Error: gh CLI Not Installed"]

    F -->|Yes| H["Detect Repository"]
    H --> I["Create GitHub Adapter"]

    I --> J{Item Type?}
    J -->|PR| K["Fetch PR Metadata via gh CLI"]
    J -->|Issue| L["Fetch Issue Metadata via gh CLI"]

    K --> M["Generate Worktree Name"]
    L --> M

    M --> N{Worktree Already Exists?}
    N -->|Yes| O["Error: Worktree Exists"]
    N -->|No| P["Create Worktree"]

    P --> Q["Check Tmux Available"]
    Q --> R{Tmux Available?}
    R -->|No| S["Warn: Tmux Unavailable"]
    R -->|Yes| T["Create Tmux Session"]

    S --> U["Check GROVE_SHELL"]
    T --> U

    U --> V{GROVE_SHELL=1?}
    V -->|Yes| W["Output cd:/path"]
    V -->|No| X["Print Instructions"]

    W --> Y["Exit 0"]
    X --> Y
```

#### System Changes

| Change Type | Description | Condition |
|-------------|-------------|-----------|
| Git | Creates new worktree directory | Always (on success) |
| Git | Checks out PR branch or creates new issue branch | Always |
| Tmux | Creates tmux session | If tmux available |
| GitHub | Fetches PR/issue metadata via `gh` CLI | For both PR and issue types |
| Shell | Outputs `cd:` directive | If `GROVE_SHELL=1` |

#### Operational Safety

**Data Loss Risk:** Low
- Fetch only creates new worktrees, no existing data modified

**External Dependencies:**
- `gh` CLI must be installed and authenticated

---

### grove up

**Purpose:** Start Docker containers for the current worktree using docker-compose.

**Dependencies:** `plugins/docker`

#### Flowchart

```mermaid
flowchart TD
    A["grove up"] --> B["Get Current Directory"]
    B --> C["Create Docker Plugin Instance"]
    C --> D["Initialize Docker Plugin"]

    D --> E{Plugin Init Success?}
    E -->|No| F["Error: Plugin Init Failed"]
    E -->|Yes| G["Call plugin.Up"]

    G --> H{Docker Up Success?}
    H -->|No| I["Error: Containers Failed to Start"]
    H -->|Yes| J{Detach Flag?}

    J -->|True| K["Print: Containers Started (detached)"]
    J -->|False| L["Containers Running (foreground)"]

    K --> M["Exit 0"]
    L --> M
```

#### System Changes

| Change Type | Description | Condition |
|-------------|-------------|-----------|
| Docker | Starts services from docker-compose.yml | Always |
| Docker | Allocates ports per worktree config | Always |
| Docker | Creates networks for service communication | Always |
| Process | Keeps containers running | When `--detach=true` |

#### Operational Safety

**Data Loss Risk:** Medium
- Volumes persist unless explicitly removed

**What can go wrong:**
- docker-compose.yml not found
- Docker daemon not running
- Port conflicts

#### Flags and Options

| Flag | Short | Type | Default | Effect |
|------|-------|------|---------|--------|
| `--detach` | `-d` | Boolean | `true` | Run containers in background |

---

### grove down

**Purpose:** Stop Docker containers for the current worktree.

**Dependencies:** `plugins/docker`

#### Flowchart

```mermaid
flowchart TD
    A["grove down"] --> B["Get Current Directory"]
    B --> C["Create Docker Plugin Instance"]
    C --> D["Initialize Docker Plugin"]

    D --> E{Plugin Init Success?}
    E -->|No| F["Error: Plugin Init Failed"]
    E -->|Yes| G["Call plugin.Down"]

    G --> H{Docker Down Success?}
    H -->|No| I["Error: Containers Failed to Stop"]
    H -->|Yes| J["Print: Containers Stopped"]

    J --> K["Exit 0"]
```

#### System Changes

| Change Type | Description | Condition |
|-------------|-------------|-----------|
| Docker | Stops all running services | Always |
| Docker | Removes container instances | After services stop |
| Docker | Preserves networks and volumes | Always |

#### Operational Safety

**Data Loss Risk:** Low
- Containers stopped but volumes preserved

---

### grove logs

**Purpose:** View Docker container logs with optional follow mode.

**Dependencies:** `plugins/docker`

#### System Changes

| Change Type | Description | Condition |
|-------------|-------------|-----------|
| Process | Streams output from containers | Always |
| Terminal | Blocks with continuous output | When `--follow=true` |

#### Operational Safety

**Data Loss Risk:** None (read-only)

#### Flags and Options

| Flag | Short | Type | Default | Effect |
|------|-------|------|---------|--------|
| `--follow` | `-f` | Boolean | `true` | Follow log output like `tail -f` |
| `[service]` | Positional | String | all | Service name to show logs from |

---

### grove browse

**Purpose:** Interactive GitHub issue/PR browser using fzf for fuzzy selection.

**Dependencies:** `plugins/tracker`, external `fzf`

#### System Changes

| Change Type | Description | Condition |
|-------------|-------------|-----------|
| GitHub API | Fetches issue/PR list | Always |
| Process | Spawns fzf subprocess | When issues/PRs found |
| Terminal | Blocks with interactive UI | Until user selects or cancels |
| Git | Creates worktree from selection | When user confirms (via fetch) |

#### Operational Safety

**Data Loss Risk:** Low
- Browse only reads from GitHub
- Can cancel at any time with Ctrl-C

**External Dependencies:**
- `gh` CLI must be installed and authenticated
- `fzf` must be installed

---

## Session & Config

### grove restart

**Purpose:** Restart Docker container services for the current worktree.

**Dependencies:** `plugins/docker`

#### System Changes

| Change Type | Description | Condition |
|-------------|-------------|-----------|
| Docker Container | Restarts specified service(s) | On `plugin.Restart()` execution |
| Process | Docker processes (PID) change on restart | Running service instances replaced |

#### Operational Safety

**Data Loss Risk:** Low
- Restart may lose in-container modifications not persisted to volumes

---

### grove time

**Purpose:** Display time tracking information for worktrees.

**Dependencies:** `plugins/time`, `internal/worktree`

#### Flowchart

```mermaid
flowchart TD
    A["grove time [subcommand] [flags]"] --> B{Subcommand?}
    B -->|week| C["Show weekly summary"]
    B -->|none| D["Show time for current/all"]

    D --> E["Initialize TimeTracker"]
    E --> F{--all flag?}
    F -->|true| G["Show all worktrees time"]
    F -->|false| H["Show current worktree time"]

    G --> I{--json flag?}
    H --> I

    I -->|true| J["Output JSON"]
    I -->|false| K["Output formatted table"]

    J --> L["Exit 0"]
    K --> L
```

#### System Changes

| Change Type | Description | Condition |
|-------------|-------------|-----------|
| File (Read) | Reads `~/.config/grove/state/time.json` | Always |
| Memory | Loads all time entries | No streaming |

#### Operational Safety

**Data Loss Risk:** Low (read-only display command)

#### Flags and Options

| Flag | Type | Default | Effect |
|------|------|---------|--------|
| `--all` | Boolean | false | Show time for ALL worktrees |
| `--json` | Boolean | false | Output as JSON |
| `week` | Subcommand | N/A | Show weekly time summary |

---

### grove config

**Purpose:** Display current grove configuration.

**Dependencies:** `internal/config`

#### System Changes

| Change Type | Description | Condition |
|-------------|-------------|-----------|
| File (Read) | Reads `~/.config/grove/config.toml` | On command |
| File (Read) | Reads `./.grove/config.toml` | If project config exists |

#### Operational Safety

**Data Loss Risk:** None (read-only)

---

### grove version

**Purpose:** Print version information of the grove binary.

**Dependencies:** `internal/version`

#### System Changes

| Change Type | Description | Condition |
|-------------|-------------|-----------|
| Stdout | Writes version string | Always |

#### Operational Safety

**Data Loss Risk:** None (informational)

#### Flags and Options

| Flag | Short | Type | Default | Effect |
|------|-------|------|---------|--------|
| `--verbose` | `-v` | Boolean | false | Show full version with commit and build date |

---

## Internal Architecture

### Package Overview

```mermaid
graph TB
    subgraph "Command Layer"
        cmds["cmd/commands"]
    end

    subgraph "Core Layer"
        wt["worktree"]
        tmux["tmux"]
        shell["shell"]
    end

    subgraph "State Layer"
        state["state"]
        config["config"]
    end

    subgraph "Extension Layer"
        hooks["hooks"]
        plugins["plugins"]
    end

    cmds --> wt
    cmds --> tmux
    cmds --> state
    cmds --> hooks
    cmds --> shell
    cmds --> config

    wt --> config
    tmux --> config
    hooks --> config
    plugins --> hooks
    plugins --> config
```

### worktree Package

**Location:** `internal/worktree/`

**Purpose:** Manages git worktree lifecycle operations.

**Key Types:**
- `Worktree` - Represents a worktree with metadata
- `Manager` - Main worktree management interface

**Key Functions:**
| Function | Behavior | System Changes |
|----------|----------|----------------|
| `NewManager()` | Detects repo root, initializes manager | None |
| `Create()` | Creates worktree with new branch | Creates directory, git metadata |
| `List()` | Lists all worktrees | None (read-only) |
| `Find()` | Searches by name | None (read-only) |
| `Remove()` | Removes worktree | Deletes directory, git metadata |
| `GetCurrent()` | Gets current worktree info | None (read-only) |
| `TmuxSessionName()` | Generates tmux session name | None |

### tmux Package

**Location:** `internal/tmux/`

**Purpose:** Manages tmux session lifecycle.

**Key Functions:**
| Function | Behavior | System Changes |
|----------|----------|----------------|
| `IsInsideTmux()` | Checks TMUX env var | None |
| `IsTmuxAvailable()` | Checks if tmux binary exists | None |
| `CreateSession()` | Creates detached session | Creates tmux session |
| `SwitchSession()` | Switches session | Changes tmux context |
| `KillSession()` | Kills tmux session | Destroys tmux session |
| `StoreLastSession()` | Stores session name | Writes file |
| `GetLastSession()` | Reads session name | None |

**State Storage:** `~/.config/grove/last_session`

### shell Package

**Location:** `internal/shell/`

**Purpose:** Generates shell integration code.

**Shell Integration Protocol:**
```bash
# Grove binary outputs: cd:/path/to/grove-cli-testing
# Shell wrapper detects GROVE_SHELL=1 and executes the cd
```

### state Package

**Location:** `internal/state/`

**Purpose:** Persists frozen worktree state.

**State File:** `~/.config/grove/state/frozen.json`

**Key Functions:**
| Function | Behavior | System Changes |
|----------|----------|----------------|
| `Freeze()` | Adds worktree to frozen map | Writes JSON file |
| `Resume()` | Removes from frozen map | Writes JSON file |
| `IsFrozen()` | Checks frozen status | None |
| `ListFrozen()` | Returns all frozen | None |

### hooks Package

**Location:** `internal/hooks/`

**Purpose:** Event system for plugins and extensions.

**Hook Events:**
| Event | When Triggered | Data Passed |
|-------|----------------|-------------|
| `pre-create` | Before creating worktree | Worktree name, Config |
| `post-create` | After creating worktree | Worktree name, Config |
| `pre-switch` | Before switching worktree | Worktree, PrevWorktree, Config |
| `post-switch` | After switching worktree | Worktree, PrevWorktree, Config |
| `pre-freeze` | Before freezing | Worktree, Config |
| `post-resume` | After resuming | Worktree, Config |
| `pre-remove` | Before removing | Worktree, Config |
| `post-remove` | After removing | Worktree, Config |

### config Package

**Location:** `internal/config/`

**Purpose:** Configuration management with defaults, loading, merging.

**Configuration Cascade:**
```
Defaults → Global (~/.config/grove/config.toml) → Project (./.grove/config.toml)
```

### plugins Package

**Location:** `internal/plugins/`

**Purpose:** Plugin system for extensibility.

**Plugin Interface:**
```go
type Plugin interface {
    Name() string
    Init(cfg *config.Config) error
    RegisterHooks(registry *hooks.Registry) error
    Enabled() bool
}
```

---

## Plugins

### Docker Plugin

**Location:** `plugins/docker/`

**Purpose:** Automate Docker container lifecycle management across worktrees.

**Hooks Registered:**
- `EventPostSwitch`: Auto-start containers
- `EventPreSwitch`: Auto-stop containers (optional)

**Configuration:**
```toml
[plugins.docker]
enabled = true
auto_start = true
auto_stop = false
```

**System Changes:**
| Change Type | Trigger |
|-------------|---------|
| Containers started | PostSwitch + auto_start enabled |
| Containers stopped | PreSwitch + auto_stop enabled |

**External Dependencies:** Docker daemon, `docker` or `docker-compose` CLI

### Time Plugin

**Location:** `plugins/time/`

**Purpose:** Passively record time spent in each worktree.

**Hooks Registered:**
- `EventPostSwitch`: Start/end sessions
- `EventPreFreeze`: End session
- `EventPostResume`: Start session

**State File:** `~/.config/grove/state/time.json`

**Data Format:**
```json
{
  "entries": [
    {
      "worktree": "feature-auth",
      "start_time": "2024-01-15T09:00:00Z",
      "end_time": "2024-01-15T11:30:00Z",
      "duration_seconds": 9000
    }
  ],
  "active_sessions": {
    "feature-auth": "2024-01-15T14:00:00Z"
  }
}
```

### Tracker Plugin

**Location:** `plugins/tracker/`

**Purpose:** Integrate with issue tracking systems (GitHub).

**Hooks Registered:** None (utility library)

**External Dependencies:** `gh` CLI (authenticated)

**Supported Operations:**
- `FetchIssue(number)` - Get issue metadata
- `FetchPR(number)` - Get PR metadata
- `ListIssues(opts)` - List issues with filters
- `ListPRs(opts)` - List PRs with filters

---

## Safety Summary

### Risk Levels by Command

| Command | Risk Level | Key Concern |
|---------|------------|-------------|
| `grove init` | None | Read-only output |
| `grove new` | Low | Creates new resources only |
| `grove rm` | **HIGH** | Deletes without confirmation |
| `grove ls` | None | Read-only |
| `grove here` | None | Read-only |
| `grove to` | Low | State changes reversible |
| `grove last` | Low | State changes reversible |
| `grove freeze` | Low | Idempotent state change |
| `grove resume` | Low | Reverse of freeze |
| `grove fetch` | Low | Creates new resources |
| `grove up` | Medium | Docker state changes |
| `grove down` | Low | Graceful shutdown |
| `grove logs` | None | Read-only |
| `grove restart` | Low | Container restart |
| `grove time` | None | Read-only display |
| `grove config` | None | Read-only |
| `grove version` | None | Informational |

### Critical Safety Notes

1. **`grove rm` is destructive** - No confirmation, deletes uncommitted changes
2. **Shell integration required** - Without `GROVE_SHELL=1`, directory changes don't work
3. **Docker auto_stop disabled by default** - To prevent port conflicts across worktrees
4. **Time tracking is passive** - Sessions auto-start/stop on worktree switches
5. **Atomic state writes** - Prevents corruption on crash

### State File Locations

| Purpose | Location |
|---------|----------|
| Last tmux session | `~/.config/grove/last_session` |
| Frozen worktrees | `~/.config/grove/state/frozen.json` |
| Time tracking | `~/.config/grove/state/time.json` |
| Global config | `~/.config/grove/config.toml` |
| Project config | `./.grove/config.toml` |

---

*Generated by Grove-CLI Tool Breakdown Analysis*
