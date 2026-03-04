# Grove Configuration Reference

Single source of truth for all Grove configuration options. For command behavior, see
[COMMAND_SPECIFICATIONS.md](COMMAND_SPECIFICATIONS.md). For shell integration, see
[SHELL_INTEGRATION.md](SHELL_INTEGRATION.md).

---

## File Locations

| File | Purpose |
|------|---------|
| `~/.config/grove/config.toml` | Global config — applies to all projects |
| `~/.config/grove/hooks.toml` | Global hooks — run in every project |
| `.grove/config.toml` | Project config — overrides global |
| `.grove/hooks.toml` | Project hooks — merged with (or replaces) global hooks |
| `.grove/state.json` | Per-project runtime state managed by grove — do not edit |

**Load order:** defaults → global config → project config → environment variable overrides.

Project settings override global settings for all scalar fields. For `[protection]` lists,
project entries are unioned with global entries (global protections always apply).

**`GROVE_CONFIG`** overrides the global config file path at runtime:
```sh
GROVE_CONFIG=~/work/shared-grove.toml grove ls
```

---

## Complete .grove/config.toml Reference

### Top-Level Fields

```toml
# Human-readable project name. Used as the prefix in worktree directory names.
# Defaults to: git remote origin repo name, or the repo directory name.
# Example: "my-app" produces worktrees named "my-app-feature", "my-app-main", etc.
project_name = "my-app"            # string

# Shell alias registered by grove shell integration.
# Default: "w"
alias = "w"                        # string

# Directory where worktrees are created as siblings of the main repo.
# Default: ~/projects
projects_dir = "~/projects"        # string

# Default base branch for new worktrees when no branch is specified.
# Default: "main"
default_base_branch = "main"       # string
```

---

### [switch]

Controls behavior when switching to a worktree that has uncommitted changes in the current one.

```toml
[switch]
# How to handle a dirty working tree on switch.
# "prompt"     — ask the user what to do (default)
# "auto-stash" — automatically stash changes before switching, unstash on return
# "refuse"     — abort the switch if changes are present
dirty_handling = "prompt"          # string: prompt | auto-stash | refuse
```

---

### [naming]

Controls how new worktree names are suggested when branching.

```toml
[naming]
# Pattern template for suggested worktree names.
# Tokens: {type} (branch type, e.g. "feat"), {description} (branch slug)
# Default: "{type}/{description}"
pattern = "{type}/{description}"   # string
```

---

### [tmux]

Controls tmux session management. Requires tmux to be installed.

```toml
[tmux]
# When to attach/create tmux sessions.
# "auto"   — create and attach sessions automatically when tmux is running (default)
# "manual" — never auto-attach; use `grove attach` explicitly
# "off"    — disable tmux integration entirely
mode = "auto"                      # string: auto | manual | off

# Prefix for tmux session names. If set, sessions are named "{prefix}-{worktree}".
# Default: "" (no prefix; session name equals the full worktree name)
prefix = ""                        # string

# How grove handles directory drift when switching tmux sessions.
# "reset"  — cd the session to the worktree root on switch (default)
# "warn"   — warn if the session is not in the expected directory
# "ignore" — do nothing; leave the session wherever it is
on_switch = "reset"                # string: reset | warn | ignore
```

---

### [test]

Configures `grove test` command behavior.

```toml
[test]
# Command to run when `grove test` is invoked.
# Supports shell syntax. Runs in the worktree root by default.
# Example: "bin/rspec" or "go test ./..."
command = "go test ./..."          # string

# Docker Compose service name to run tests inside.
# When set, grove runs the command via `docker compose exec <service> <command>`.
# Requires the docker plugin to be configured.
service = "app"                    # string
```

---

### [session]

Configures `grove open` command behavior (opening a shell session in a worktree).

```toml
[session]
# Shell or command to launch in the worktree.
# Default: $SHELL
command = "/bin/zsh"               # string

# Whether to open the session in a tmux popup instead of a new window.
# Default: false
popup = false                      # bool

# Width of the tmux popup (tmux-style: pixels or percentage).
# Default: "80%"
popup_width = "80%"                # string

# Height of the tmux popup.
# Default: "80%"
popup_height = "80%"               # string
```

---

### [protection]

Controls which worktrees cannot be removed or modified.

```toml
[protection]
# Worktrees that require --force --unprotect to remove.
# Use short names (not full directory names).
# Default: [] (empty)
protected = ["main", "staging"]    # string array

# Worktrees that cannot have changes applied or synced (grove apply, grove sync).
# Does not prevent removal — use protected for that.
# Default: [] (empty)
immutable = ["production"]         # string array
```

**Merge behavior:** when both global and project configs define protection lists, they are
merged. Entries from both sources are preserved (union). This means global protections
always apply even if the project config sets its own list.

---

### [tui]

Controls TUI (interactive dashboard) behavior.

```toml
[tui]
# Suppress the "branch already exists" notice shown when creating a worktree
# for an existing branch.
# Default: false (show the notice)
skip_branch_notice = false         # bool

# Action to take when skip_branch_notice is true and a branch conflict is encountered.
# "split" — create worktree from the existing branch
# "fork"  — create a new branch derived from the existing one
# Default: "" (prompt the user)
default_branch_action = "split"    # string: split | fork

# How to derive a worktree name from a branch name in the TUI.
# "last_segment" — use the part after the last "/" (e.g., "feat/my-thing" → "my-thing")
# Default: "last_segment"
worktree_name_from_branch = "last_segment"  # string: last_segment

# Use single-line compact list items in the TUI worktree list.
# Default: false (multi-line items with detail)
compact_list = false               # bool
```

---

### [plugins.docker]

Controls Docker Compose integration. Requires the docker plugin and Docker to be installed.

```toml
[plugins.docker]
# Enable or disable the docker plugin entirely.
# Default: true
enabled = true                     # bool

# Automatically start services when switching to a worktree.
# Default: true
auto_start = true                  # bool

# Automatically stop services when switching away from a worktree.
# Default: false (local mode), true (external mode)
auto_stop = false                  # bool

# Auto-start Docker on `grove new` (runs `docker compose up` after worktree creation).
# Default: false (true when agent stacks are configured and enabled)
auto_up = false                    # bool

# Docker Compose mode.
# ""        or "local"    — compose files live in the worktree itself
# "external"              — services are managed by a shared compose setup outside the repo
# Default: "" (local)
mode = "local"                     # string: local | external
```

#### [plugins.docker.external]

Required when `mode = "external"`. Configures the shared compose setup.

```toml
[plugins.docker.external]
# Path to the external Docker Compose directory (the one with docker-compose.yml).
# Supports ~ expansion. Required.
path = "~/work/compose-dev"        # string (required)

# Environment variable name that grove sets to the worktree path.
# The external compose file reads this to know which codebase to mount.
# Example: if env_var = "APP_DIR", grove sets APP_DIR=/path/to/worktree.
# Required.
env_var = "APP_DIR"                # string (required)

# File in the compose directory where grove writes the env var value.
# Grove passes --env-file to every compose call automatically.
# Set to ".env.local" to avoid dirtying a git-tracked .env.
# Default: ".env"
env_file = ".env"                  # string

# Docker Compose service names that grove manages (start/stop) for this worktree.
# Required.
services = ["web", "worker"]       # string array (required)

# Files to copy from the main worktree into each new worktree on create.
# Useful for credential files, local config, etc. that are gitignored.
# Paths are relative to the worktree root.
# Default: [] (nothing copied)
copy_files = [".env.local", "config/master.key"]  # string array

# Directories to symlink from the main worktree into each new worktree on create.
# Useful for large build caches (node_modules, vendor/) that should be shared.
# Paths are relative to the worktree root.
# Default: [] (nothing symlinked)
symlink_dirs = ["node_modules", "vendor"]         # string array
```

#### [plugins.docker.external.agent]

Optional. Enables agent stack support — isolated per-worktree service instances for
concurrent AI agent workloads. Nested under `[plugins.docker.external]`.

```toml
[plugins.docker.external.agent]
# Enable agent stack mode.
# Default: false
enabled = true                     # bool

# Maximum number of concurrent agent slots (isolated stacks).
# Default: 5
max_slots = 5                      # int

# Service names in the agent template compose file.
# Required when enabled = true.
services = ["web", "worker"]       # string array (required when enabled)

# Path to the agent compose template file.
# Required when enabled = true.
template_path = "~/work/compose-dev/agent-compose.yml"  # string (required when enabled)

# URL pattern for accessing agent services.
# {port} is replaced with the allocated port.
# Default: "http://localhost:{port}"
url_pattern = "http://localhost:{port}"  # string

# External Docker network that agent stacks attach to.
# The network must already exist before grove can create agent stacks.
# Default: "shared"
network = "shared"                 # string
```

---

## Hooks Configuration (.grove/hooks.toml)

Hooks run shell commands, copy files, or render templates at worktree lifecycle events.
Hooks are defined in a separate file from main config.

**Load and merge behavior:**
- Global hooks (`~/.config/grove/hooks.toml`) and project hooks (`.grove/hooks.toml`) are
  both loaded and **merged** — global hooks run first, then project hooks append.
- Set `override_<event> = true` in the project hooks file to replace global hooks for that
  event entirely instead of appending.

### Hook Events

| Event | When it fires |
|-------|--------------|
| `pre_create` | Before a new worktree directory is created |
| `post_create` | After the worktree is created and checked out |
| `pre_switch` | Before switching to a worktree |
| `post_switch` | After switching to a worktree |
| `pre_remove` | Before removing a worktree |
| `post_remove` | After the worktree has been removed |

### Hook Action Types

#### copy

Copies a file or directory from the main worktree into the new worktree.

```toml
[[hooks.post_create]]
type     = "copy"
from     = ".env.local"        # source path, relative to main worktree
to       = ".env.local"        # destination path, relative to new worktree
required = false               # bool: if true, failure aborts the operation (default: false)
on_failure = "warn"            # "warn" (default) | "fail" | "ignore"
```

#### symlink

Creates a symlink in the new worktree pointing to a path in the main worktree.

```toml
[[hooks.post_create]]
type = "symlink"
from = "node_modules"          # source path, relative to main worktree
to   = "node_modules"          # symlink path, relative to new worktree
```

#### command

Runs a shell command.

```toml
[[hooks.post_create]]
type        = "command"
command     = "bundle install" # shell command string; supports {{.variable}} interpolation
working_dir = "new"            # "new" (default) | "main" | absolute path
timeout     = 300              # seconds before the command is killed (default: 60)
on_failure  = "warn"           # "warn" (default) | "fail" | "ignore"
```

#### template

Renders a template file from the main worktree into the new worktree.
Uses `{{.variable}}` syntax (see template variables below).

```toml
[[hooks.post_create]]
type = "template"
from = ".env.template"         # template source, relative to main worktree
to   = ".env"                  # rendered output, relative to new worktree
vars = { APP_PORT = "{{.port}}", ENV = "development" }  # extra variables
```

### Template Variables

These variables are available in `command`, `from`, `to`, and template file contents:

| Variable | Description |
|----------|-------------|
| `{{.worktree}}` | Short worktree name (e.g., `"feature"`) |
| `{{.worktree_full}}` | Full worktree directory name (e.g., `"my-app-feature"`) |
| `{{.branch}}` | Branch name (e.g., `"feat/my-feature"`) |
| `{{.project}}` | Project name (e.g., `"my-app"`) |
| `{{.main_path}}` | Absolute path to the main worktree |
| `{{.new_path}}` | Absolute path to the new/target worktree |
| `{{.prev_path}}` | Absolute path to the previous worktree (switch events only) |
| `{{.port}}` | Allocated port number, if any |
| `{{.user}}` | Current OS username |
| `{{.timestamp}}` | Unix timestamp (integer) |
| `{{.date}}` | ISO date string (`YYYY-MM-DD`) |

### Per-Branch/Worktree Overrides

Use `[[overrides]]` to apply different hook behavior for specific branches or worktrees.
The `match` field is a glob pattern tested against both the branch name and worktree name.

```toml
[[overrides]]
match      = "hotfix/*"    # glob pattern; tested against branch and worktree name
skip_hooks = true          # bool: skip all hooks for matching branches

[[overrides]]
match      = "release-*"
skip       = ["command"]   # string array: skip specific action types for this branch
extra_copy = ["release.env"]    # string array: additional files to copy
extra_run  = ["make release-prep"]  # string array: additional commands to run
```

### Complete hooks.toml Example

```toml
# .grove/hooks.toml

[hooks]
# Override global post_create hooks for this project (instead of appending)
override_post_create = true

[[hooks.post_create]]
type     = "copy"
from     = ".env.example"
to       = ".env"
on_failure = "warn"

[[hooks.post_create]]
type     = "copy"
from     = "config/master.key"
to       = "config/master.key"
required = true

[[hooks.post_create]]
type        = "command"
command     = "bundle install"
working_dir = "new"
timeout     = 300
on_failure  = "fail"

[[hooks.post_switch]]
type    = "command"
command = "echo 'Switched to {{.worktree}} on {{.branch}}'"

# Skip all hooks for dependabot branches
[[overrides]]
match      = "dependabot/*"
skip_hooks = true
```

---

## Environment Variables

These are read at runtime and never written to config files.

| Variable | Values | Purpose |
|----------|--------|---------|
| `GROVE_SHELL` | `1` | Set by shell integration. Enables the directive protocol (cd:, env:, tmux-attach:). |
| `GROVE_SHELL_VERSION` | integer | Set by shell integration. Binary warns if shell is outdated. |
| `GROVE_TUI` | `0` to disable | Set to `0` to disable TUI mode. Bare `grove` shows help instead. Default: enabled. |
| `GROVE_AGENT_MODE` | any non-empty | Forces isolated Docker strategy and suppresses tmux for AI agent workloads. |
| `GROVE_LOG` | `1` or path | Enable command-level logging. `1` writes to `~/.grove/grove.log`. |
| `GROVE_DEBUG` | `1` or path | Enable TUI debug logging. `1` writes to `~/.grove-debug.log`. |
| `GROVE_NO_COLOR` | any non-empty | Disable all colored output. Also respected: `NO_COLOR`. |
| `GROVE_HIGH_CONTRAST` | any non-empty | Use high-contrast color scheme in TUI. |
| `GROVE_LIGHT_MODE` | `1` / `0` | Override terminal light/dark detection. `1` = light mode, `0` = dark mode. |
| `GROVE_NONINTERACTIVE` | any non-empty | Disable all interactive prompts. Commands fail or use defaults instead. |
| `GROVE_CONFIG` | file path | Override the global config file path (default: `~/.config/grove/config.toml`). |
| `GROVE_CD_FILE` | file path | Temp file used by TUI for directory handoff to shell wrapper. Managed by grove. |

---

## Example Configurations

### Minimal Setup

```toml
# .grove/config.toml — minimal config for a project with tmux
project_name = "my-app"

[tmux]
mode = "auto"
```

### Rails Project with External Docker

```toml
# .grove/config.toml — Rails project using a shared compose setup

project_name = "rails-app"
default_base_branch = "main"

[switch]
dirty_handling = "auto-stash"

[tmux]
mode = "auto"
on_switch = "reset"

[test]
command = "bin/rspec"
service = "app"

[protection]
protected = ["main", "staging"]

[plugins.docker]
enabled    = true
auto_start = true
auto_stop  = true
mode       = "external"

[plugins.docker.external]
path     = "~/work/compose-dev"
env_var  = "APP_DIR"
env_file = ".env.local"
services = ["web", "worker", "jobs"]
copy_files   = [".env.local", "config/master.key", "config/credentials.yml.enc"]
symlink_dirs = ["storage"]
```

### Multi-Agent CI (Agent Stacks)

```toml
# .grove/config.toml — high-concurrency agent workloads with isolated stacks

project_name = "my-app"

[switch]
dirty_handling = "auto-stash"

[tmux]
mode = "off"          # tmux suppressed; agents don't need sessions

[plugins.docker]
enabled    = true
auto_start = true
auto_stop  = true
mode       = "external"

[plugins.docker.external]
path     = "~/work/compose-dev"
env_var  = "APP_DIR"
services = ["web", "worker"]

[plugins.docker.external.agent]
enabled       = true
max_slots     = 8
services      = ["web", "worker"]
template_path = "~/work/compose-dev/agent-compose.yml"
url_pattern   = "http://localhost:{port}"
network       = "shared"
```

### Node.js Project with Local Docker

```toml
# .grove/config.toml — Node project with local compose files in each worktree

project_name = "node-app"
default_base_branch = "main"

[tmux]
mode = "auto"

[test]
command = "npm test"

[plugins.docker]
enabled    = true
auto_start = true
auto_stop  = false
mode       = "local"   # docker-compose.yml lives in each worktree
```
