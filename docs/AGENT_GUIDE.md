# Grove Agent Guide

Reference for AI agents helping users install, configure, or use Grove. Developers will find this useful too, but the structure and depth are calibrated for automated tooling.

**Quick links:**
- [Command Specifications](COMMAND_SPECIFICATIONS.md) — full flag/output specs
- [Shell Integration](SHELL_INTEGRATION.md) — directive protocol details
- [Configuration Reference](CONFIGURATION_REFERENCE.md) — all config fields
- [Docker Plugin README](../plugins/docker/README.md) — Docker mode details
- [Plugin Development](PLUGIN_DEVELOPMENT.md) — hook interfaces

---

## Table of Contents

1. [What Grove Replaces](#1-what-grove-replaces)
2. [Installation](#2-installation)
3. [Core Workflows](#3-core-workflows)
4. [Docker Strategies](#4-docker-strategies)
5. [Configuration Quick Reference](#5-configuration-quick-reference)
6. [Environment Variables](#6-environment-variables)
7. [Agent Strategy Guide](#7-agent-strategy-guide)
8. [Troubleshooting](#8-troubleshooting)

---

## 1. What Grove Replaces

Grove automates the mechanical work of context-switching between git branches — creating worktrees, managing tmux sessions, wiring Docker, and running setup hooks.

### Working on a GitHub PR

**Before Grove:**
```bash
gh pr view 42
git fetch origin
git worktree add ../myapp-pr-42 origin/pr-42-branch
cd ../myapp-pr-42
cp ../myapp/.env.local .env.local
ln -s ../myapp/node_modules node_modules
ln -s ../myapp/vendor/bundle vendor/bundle
docker compose up -d
tmux new-session -d -s myapp-pr-42
tmux attach -t myapp-pr-42
# 9 commands, easy to forget a step
```

**After Grove:**
```bash
grove fetch pr/42
# Creates worktree, copies credentials, symlinks deps,
# starts Docker, opens tmux session — one command
```

### Creating a Feature Branch

**Before:**
```bash
git fetch origin
git worktree add ../myapp-auth main
cd ../myapp-auth
git checkout -b feature/auth
cp ../.env.local .env.local
docker compose up -d
tmux new-session -d -s myapp-auth
```

**After:**
```bash
grove new auth
# Worktree + branch + hooks + Docker + tmux session
```

### Switching Context

**Before:**
```bash
docker compose stop           # stop current containers
tmux switch-client -t myapp-auth
cd ../myapp-auth
docker compose up -d          # start new containers
```

**After:**
```bash
grove to auth
# Atomic: pre_switch hooks, dir change, post_switch hooks,
# Docker stop/start, tmux attach — in the right order
```

### Cleaning Up

**Before:**
```bash
docker compose down
tmux kill-session -t myapp-pr-42
git worktree remove ../myapp-pr-42
git branch -d pr-42-branch
```

**After:**
```bash
grove rm pr-42
# Confirms, then removes worktree + branch + tmux + Docker
```

---

## 2. Installation

### Prerequisites Check

Before installing Grove, verify:

```bash
git --version     # Need 2.30+
go version        # Need 1.24+ (build only, not runtime)
tmux -V           # 3.0+ (optional, needed for session management)
gh --version      # optional, needed for grove fetch pr/N and grove prs/issues
```

### Install Methods

**Homebrew (preferred):**
```bash
brew tap LeahArmstrong/tap
brew install grove
```

**go install:**
```bash
go install github.com/LeahArmstrong/grove-cli/cmd/grove@latest
```

**Release binaries:**
Download from GitHub Releases. Place the binary somewhere on `$PATH` (e.g., `/usr/local/bin/grove`).

### Shell Integration Setup

Grove needs a shell function wrapper so commands like `grove to` can change your working directory. Without it, directory changes silently have no effect.

**Automatic (recommended):**
```bash
grove setup
# Detects your shell, finds rc file, appends eval line idempotently
source ~/.zshrc   # or ~/.bashrc
```

**Manual — zsh:**
```bash
echo 'eval "$(grove install zsh)"' >> ~/.zshrc
source ~/.zshrc
```

**Manual — bash:**
```bash
echo 'eval "$(grove install bash)"' >> ~/.bashrc
source ~/.bashrc
```

### Verifying Installation

```bash
grove version
# grove v1.x.x

type grove
# grove is a shell function
# If this says "grove is /usr/local/bin/grove", shell integration is not loaded.

grove doctor
# Runs all health checks — see §8 Troubleshooting
```

### Project Initialization

```bash
cd ~/projects/myapp
grove init
# Creates .grove/config.toml with defaults

grove doctor
# Checks git version, tmux, gh CLI, Docker, shell integration, config
```

---

## 3. Core Workflows

### Create a Worktree

```bash
grove new <name>
```

What happens:
1. Creates a git worktree at `../{project}-{name}/`
2. Creates a branch named `{name}` (configurable via `[naming] pattern`)
3. Fires `post_create` hooks (copy credentials, symlink deps, etc.)
4. Creates a tmux session named `{project}-{name}` (if tmux mode is not `off`)

```bash
grove new auth
# Created worktree: myapp-auth
# Created branch: auth
# Copied: config/master.key
# Symlinked: node_modules
# Created tmux session: myapp-auth
```

Output: emits `cd:/abs/path/to/myapp-auth` directive — shell wrapper changes directory.

Hooks fired: `post_create`

### Switch Context Atomically

```bash
grove to <name>
grove to <name> --peek    # lightweight, skips hooks
```

> **AGENT WARNING — tmux takeover.** By default, `grove to` attaches the terminal to the target tmux session. In an agent context this takes over the terminal. See §7 Agent Strategy Guide for how to prevent this.

What happens (full switch):
1. Fires `pre_switch` hooks (Docker auto-stop in external mode, custom commands)
2. Changes directory to target worktree
3. Fires `post_switch` hooks (Docker auto-start, env: directives, custom commands)
4. Emits `tmux-attach:{session}` directive — shell wrapper attaches tmux

`--peek` skips all hooks and tmux. Use it for quick inspection or when hooks are expensive.

Hooks fired: `pre_switch`, `post_switch`

### Quick-Switch to Previous

```bash
grove last
```

Switches back to the previous worktree (tracked in `.grove/state.json`). Same hook/directive behavior as `grove to`.

### Work from a GitHub PR or Issue

```bash
grove fetch pr/42
grove fetch issue/123
```

Requires `gh` CLI authenticated to the repo.

What happens:
1. Fetches the PR branch (or creates a branch from the issue title)
2. Creates a worktree from that branch
3. Fires `post_create` hooks (credentials, symlinks, Docker setup)

```bash
grove fetch pr/42
# Fetched branch: feature/auth-refactor (PR #42)
# Created worktree: myapp-pr-42
```

Interactive browsers: `grove prs`, `grove issues` — TUI lists with fuzzy search, press Enter to fetch.

### Fork Current Work

```bash
grove fork <name>
grove fork <name> --move-wip    # stash → new branch, leave current clean
grove fork <name> --copy-wip    # copy uncommitted changes to new worktree
grove fork <name> --no-wip      # fork only committed changes (default)
```

Creates a new worktree branching from the current one. Useful for exploring alternatives without losing work-in-progress.

Hooks fired: `post_create`

### Share Changes Between Worktrees

**Compare:**
```bash
grove compare <name>
# Shows diff between current worktree and <name>
```

**Apply commits or WIP from another worktree:**
```bash
grove apply <name>
# Cherry-picks commits from <name> onto current branch,
# or applies stashed WIP if no commits differ
```

**Sync from remote:**
```bash
grove sync           # fast-forward current worktree from remote
grove sync <name>    # fast-forward a specific worktree
```

### Run Tests Without Switching

```bash
grove test <name> [args...]
```

Runs the configured test command in `<name>`'s worktree without switching context. Your current worktree and Docker stack are untouched.

Configure in `.grove/config.toml`:
```toml
[test]
command = "bin/rails test"
service = "app"    # Docker service to exec into (optional)
```

When `service` is set, grove spawns an ephemeral container in the target worktree's Docker environment.

### Clean Up

**Remove one worktree:**
```bash
grove rm <name>
# Prompts for confirmation, then:
# stops Docker, kills tmux session, removes worktree, deletes branch
```

Protected worktrees (`main`, `develop`, or any name in `[protection] protected`) require `--force` to remove. Immutable worktrees block `grove apply` and `grove sync` but can still be removed with `grove rm`.

**Remove stale worktrees in bulk:**
```bash
grove clean           # removes worktrees not accessed in 30 days
grove clean --days 7  # shorter threshold
```

Stale = `LastAccessedAt` older than threshold and no dirty changes. Protected worktrees are skipped.

---

## 4. Docker Strategies

See [plugins/docker/README.md](../plugins/docker/README.md) for full configuration reference.

### Local Mode

Each worktree has its own `docker-compose.yml`. Grove detects it automatically.

```toml
# .grove/config.toml
[plugins.docker]
enabled = true
auto_start = true    # start on grove to
auto_stop = false    # stop on grove to (false = keep running)
```

Hooks:
- `post_switch` → `docker compose up -d` in the target worktree
- `pre_switch` → `docker compose stop` (only if `auto_stop = true`)

Note: `auto_stop` defaults to `false` in local mode. Multiple worktrees can have containers running simultaneously, which causes port conflicts if they use the same ports. Enable `auto_stop = true` or use different ports per worktree.

### External Mode

One shared compose directory orchestrates services for all worktrees. Grove writes the active worktree path to an env file and passes `--env-file` to every compose command.

```toml
# .grove/config.toml
[plugins.docker]
enabled = true
auto_start = true
auto_stop = true      # external mode default: true
mode = "external"

[plugins.docker.external]
path = "~/projects/shared-infra"    # shared compose directory
env_var = "APP_DIR"                  # variable compose reads for worktree path
env_file = ".env.local"              # file grove writes in the compose directory
services = ["app", "app_worker"]     # only these services are managed
copy_files = [
  "config/credentials/development.key",
  "config/master.key",
]
symlink_dirs = ["vendor/bundle", "node_modules"]
```

On `grove to feature-x`:
1. Runs `docker compose stop app app_worker` in `~/projects/shared-infra`
2. Writes `APP_DIR=/abs/path/myapp-feature-x` to `~/projects/shared-infra/.env.local`
3. Runs `docker compose --env-file .env.local up -d app app_worker`

On `grove new feature-x`:
- Copies each file in `copy_files` from the main worktree
- Symlinks each dir in `symlink_dirs` from the main worktree

### Env File Loaders (direnv / mise)

**When you need this:** Grove passes `--env-file` automatically for its own commands. You only need a loader if you also run `docker compose` commands manually in the compose directory (e.g., `docker compose logs app`).

**Option A — direnv:**
```bash
brew install direnv
# Add to ~/.zshrc: eval "$(direnv hook zsh)"

# In the compose directory:
echo 'dotenv_if_exists .env.local' > ~/projects/shared-infra/.envrc
cd ~/projects/shared-infra && direnv allow
```

**Option B — mise:**
```bash
brew install mise
# Add to ~/.zshrc: eval "$(mise activate zsh)"

# In the compose directory:
cat > ~/projects/shared-infra/.mise.toml << 'EOF'
[env]
_.file = ".env.local"
EOF
cd ~/projects/shared-infra && mise trust
```

Run `grove doctor` to verify loader detection.

### Agent Isolation Mode

Multiple agents can each run their own independent Docker stack with port offsets so they don't conflict.

```bash
grove up --isolated          # allocate next available slot
grove up --isolated --slot 3 # specific slot

grove agent-status           # table of active stacks
grove agent-status --json    # machine-readable

grove down --slot 2          # stop specific stack
grove ps                     # show running stacks with reference IDs and URLs
```

Each isolated stack gets a unique compose project name (`{project}-{worktree}-slot-{N}`) and port-offset containers.

Configure max concurrent stacks:
```toml
[plugins.docker.external.agent]
max_slots = 5
network = "shared"
url_pattern = "http://localhost:{port}"
```

See §7 for agent workflow patterns using isolated stacks.

---

## 5. Configuration Quick Reference

Config is loaded in layers (later overrides earlier):
1. Built-in defaults
2. `~/.config/grove/config.toml` (global, overridable with `GROVE_CONFIG`)
3. `.grove/config.toml` (project-level)
4. Environment variable overrides (runtime-only, not persisted)

```toml
# .grove/config.toml

# Project identity
# name = "myapp"        # default: derived from git remote or directory name

[switch]
# What to do when switching away from a dirty worktree
# "prompt" (default) | "auto-stash" | "refuse"
dirty_handling = "prompt"

[naming]
# Branch name pattern when creating worktrees
# Tokens: {type}, {description}
# Default creates branch matching the worktree short name
# pattern = "{type}/{description}"

[tmux]
# "auto" (default) | "manual" | "off"
# auto: grove manages sessions automatically
# manual: grove creates sessions but doesn't attach
# off: no tmux integration
mode = "auto"
# prefix = ""   # prefix for tmux session names (rarely needed)

[test]
# command = "bin/rails test"   # command to run for grove test
# service = "app"              # Docker service to exec into (optional)

[session]
# command = ""    # command to run when opening a new session
# popup = false   # open in tmux popup instead of new window

[plugins.docker]
enabled = true
auto_start = true
auto_stop = false
# mode = "local"   # "local" (default) | "external"

# Only needed for external mode:
# [plugins.docker.external]
# path = "~/projects/shared-infra"
# env_var = "APP_DIR"
# env_file = ".env.local"
# services = ["app"]
# copy_files = []
# symlink_dirs = []

# Only needed for isolated/agent stacks:
# [plugins.docker.external.agent]
# max_slots = 5
# network = "shared"
# url_pattern = "http://localhost:{port}"

[protection]
# Worktrees that require --force to remove
# protected = ["main", "develop"]
# Worktrees that cannot be removed at all
# immutable = []

[tui]
# skip_branch_notice = false
# default_branch_action = "split"  # "split" | "fork"
# worktree_name_from_branch = "last_segment"
```

Run `grove config` to see the merged effective config for the current project.

---

## 6. Environment Variables

| Variable | Default | Purpose |
|----------|---------|---------|
| `GROVE_SHELL` | unset | Set to `1` by the shell wrapper. Enables directive output (`cd:`, `tmux-attach:`, `env:`). Binary without this set prints human-readable output only. |
| `GROVE_SHELL_VERSION` | unset | Shell integration version. Binary warns when integration is outdated. |
| `GROVE_AGENT_MODE` | unset | Set to `1` to suppress tmux operations automatically. Recommended for all agent contexts. |
| `GROVE_TUI` | `1` | Set to `0` to disable TUI. Bare `grove` with no arguments prints usage instead of launching the dashboard. |
| `GROVE_CD_FILE` | unset | Path to temp file for TUI directory handoff. Set by the wrapper; TUI writes target path here. |
| `GROVE_HIGH_CONTRAST` | unset | Set to `1` for high-contrast TUI form elements. |
| `GROVE_LOG` | unset | Set to `1` to log to `~/.grove/grove.log`. Set to a path for a custom log file. |
| `GROVE_NO_COLOR` | unset | Set to `1` to disable color output. |
| `GROVE_DEBUG` | unset | Set to `1` for verbose debug output. |
| `GROVE_NONINTERACTIVE` | unset | Set to `1` to skip all interactive prompts (auto-accept defaults). |
| `GROVE_CONFIG` | unset | Override path to global config file. Replaces `~/.config/grove/config.toml`. |

---

## 7. Agent Strategy Guide

### When to Use Grove vs Manual Worktrees

| Scenario | Recommendation |
|----------|---------------|
| User already has Grove configured | Use Grove — don't bypass it |
| Fresh repo, no `.grove/` directory | `grove init` first, then Grove commands |
| Read-only code review, no side effects | `grove fetch pr/N` then `grove to <name> --peek` |
| Need parallel isolated Docker stacks | Grove with `grove up --isolated` |
| Just need to read a file in another branch | `git show branch:path/to/file` — no worktree needed |
| CI/CD environment | Grove with `GROVE_AGENT_MODE=1` — tmux suppressed, Docker isolation available |
| User asked you to help with Grove setup | Use Grove — that's the task |

### Recommended: Agent Workflow for Parallel Development

This pattern lets multiple agents work on separate worktrees simultaneously without interfering with each other or the developer's active session.

```bash
# Agent environment setup (once per session)
export GROVE_AGENT_MODE=1        # suppress tmux takeover
export GROVE_NONINTERACTIVE=1    # no prompts

# Each agent creates its own worktree
grove new agent-task-1
grove new agent-task-2

# Each agent starts an isolated Docker stack
cd ../myapp-agent-task-1
grove up --isolated               # gets slot 1
# → containers running on offset ports

cd ../myapp-agent-task-2
grove up --isolated               # gets slot 2
# → containers running on different offset ports

# Check what's running
grove agent-status --json
# [{"slot":1,"project":"myapp-agent-task-1-slot-1","status":"running","ports":[3101,3111]},
#  {"slot":2,"project":"myapp-agent-task-2-slot-2","status":"running","ports":[3102,3112]}]

# When done, clean up
grove down --slot 1
grove rm agent-task-1

grove down --slot 2
grove rm agent-task-2
```

### Recommended: Non-Destructive Code Review

Review a PR without touching the developer's active environment:

```bash
export GROVE_AGENT_MODE=1

# Fetch the PR branch as a new worktree (no switch, no hooks that affect active stack)
grove fetch pr/42

# Peek into it without firing hooks
grove to pr-42 --peek

# Run tests in the PR worktree without leaving current context
grove test pr-42
# or with extra args:
grove test pr-42 -- --tag focus

# When done
grove rm pr-42
```

### Configuring for Agent Use

Recommended `.grove/config.toml` additions for projects that agents use:

```toml
[switch]
# Agents should not be blocked by dirty worktrees
dirty_handling = "auto-stash"

[tmux]
# Agents should not attach to tmux sessions
# "manual" creates sessions without attaching; "off" disables entirely
mode = "manual"

[plugins.docker]
# Stop containers when switching away — prevents stale stacks
auto_stop = true

[plugins.docker.external.agent]
max_slots = 5
```

Or set per-session without modifying config:
```bash
export GROVE_AGENT_MODE=1        # disables tmux attachment
export GROVE_NONINTERACTIVE=1    # auto-accept all prompts
```

`GROVE_AGENT_MODE=1` is the lowest-friction option — it suppresses tmux attachment in `grove to` and activates isolated Docker without requiring config changes. Note that `grove new` still creates tmux sessions; set `[tmux] mode = "off"` in config to fully disable tmux.

---

## 8. Troubleshooting

### `grove doctor`

Always run this first. It checks:

- Git version (need 2.30+)
- Shell integration loaded (`grove` is a function, not a binary)
- Shell integration version matches binary
- Tmux availability and version
- `gh` CLI availability and auth status
- Docker availability
- Project config validity
- For external Docker mode: env file target, loader detection, loader configuration

```bash
grove doctor
# ✓ Git 2.43.0
# ✓ Shell integration loaded (v3)
# ✓ Tmux 3.4
# ✓ gh CLI authenticated
# ✓ Docker available
# ✓ Project config valid
# ✗ Env file loader not configured
#   Run: echo 'dotenv_if_exists .env.local' > .envrc && direnv allow
```

### Common Issues

**`grove: command not found`**

The binary is not in `$PATH`. Verify:
```bash
which grove || echo "not found"
# For Homebrew: check brew --prefix, ensure /opt/homebrew/bin or /usr/local/bin is in PATH
# For go install: ensure $GOPATH/bin (usually ~/go/bin) is in PATH
```

**`grove to` does not change directory**

Shell integration not loaded. The wrapper is absent — calling the binary directly cannot change your shell's directory:
```bash
type grove
# Should say: grove is a shell function
# If it says: grove is /usr/local/bin/grove — the wrapper is not active
```
Fix: add `eval "$(grove install zsh)"` to `~/.zshrc` and `source ~/.zshrc`.

**`not a grove project` / `no .grove directory found`**

Run from inside the project directory, or initialize:
```bash
cd ~/projects/myapp
grove init
```

**Port conflicts between worktrees (local Docker mode)**

Two worktrees running containers that bind the same host ports:
- Enable `auto_stop = true` so switching stops the previous stack
- Or use `grove up --isolated` for independent port-offset stacks
- Or configure different ports in each worktree's `docker-compose.yml`

**Stale tmux sessions**

If a session exists with a worktree name but the worktree was removed manually:
```bash
grove repair
# Detects and resolves state inconsistencies including orphaned tmux sessions
```

Or clean manually:
```bash
tmux kill-session -t myapp-stale-feature
```

**Shell integration version mismatch warning**

The `grove` binary is newer than the shell function. Re-source the integration:
```bash
eval "$(grove install zsh)"    # re-evaluate, no need to edit .zshrc
```

Or let `grove setup` handle it — it updates the eval line if needed.

**`grove to` attaches tmux and takes over terminal (agent context)**

Set `GROVE_AGENT_MODE=1` in your environment, or add `mode = "manual"` or `mode = "off"` under `[tmux]` in `.grove/config.toml`. See §7 for the full agent configuration pattern.
