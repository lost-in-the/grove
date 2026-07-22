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
brew install lost-in-the/tap/grove
```

**go install:**
```bash
go install github.com/lost-in-the/grove/cmd/grove@latest
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

**Important: PATH setup for non-login shells**

The `eval "$(grove install zsh)"` line resolves the grove binary via a PATH-only lookup (`whence -p grove` in zsh, `type -P grove` in bash). This requires the binary's directory to be in PATH for all shell types — not just login shells.

| File | When sourced | Use for |
|------|-------------|---------|
| `~/.zshenv` | Every zsh invocation | PATH setup, `brew shellenv` — ensures tools are available everywhere |
| `~/.zprofile` | Login shells only | One-time session setup (ssh-agent, etc.) |
| `~/.zshrc` | Interactive shells | Shell integration (`eval "$(grove install zsh)"`), aliases, prompt |

If you installed grove via Homebrew, ensure `eval "$(brew shellenv)"` is in `~/.zshenv`, not `~/.zprofile`. Otherwise, non-interactive tools (CI, Claude Code, scripts) won't find the grove binary.

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
# Auto-detects project type (Rails, Node, Go, Python, Docker), generates
# .grove/config.toml + .grove/hooks.toml with sensible defaults.

grove doctor
# Checks git version, tmux, gh CLI, Docker, shell integration, config.
```

**For scripted / CI / agent use** — pick a mode explicitly so init never blocks on a prompt:

```bash
grove init --auto --yes      # Generate hooks.toml from detection, skip preview
grove init --no-hooks         # Initialize state only, no hooks.toml
```

`--auto` is also the default in non-TTY contexts, but passing it explicitly makes scripts robust against future default changes. `--walkthrough` exists for interactive review of detected hooks but isn't appropriate for headless agents.

**Docker-aware routing.** When `grove init` detects a `docker-compose.yml` alongside a Rails/Node/Python marker, it auto-generates install commands as `docker:compose` hooks rather than host commands — so `bundle install` / `npm install` / `pip install` run inside the container on the next `grove new`. `grove doctor --fix` will rewrite stale host-install commands to `docker:compose` form for projects initialized before this routing existed. See [plugins/docker/README.md](../plugins/docker/README.md#config-driven-action-types) for the action-type reference.

---

## 3. Core Workflows

### Create a Worktree

```bash
grove new <name>
```

What happens:
1. Creates a git worktree at `../{project}-{name}/`
2. Creates a branch named `{name}` (override with `--branch`; `[naming] pattern` controls the directory name, not the branch)
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

### List All Worktrees

```bash
grove ls
```

Displays a table of all managed worktrees with their associated branches, status, and tmux state:

```
  NAME             BRANCH           STATUS     TMUX        PATH
  ──────────────────────────────────────────────────────────────────
● main             main             clean      attached    ~/projects/myapp
  feature-auth     feature/auth     dirty      detached    ~/projects/myapp-feature-auth
  pr-42            fix/login-bug    clean      none        ~/projects/myapp-pr-42
```

Columns:
- **●** — indicates the current worktree
- **NAME** — short display name (without project prefix)
- **BRANCH** — the git branch checked out in that worktree
- **STATUS** — `clean`, `dirty`, or `stale`
- **TMUX** — `attached`, `detached`, or `none`
- **PATH** — absolute worktree path

**Output modes:**

| Flag | Output |
|------|--------|
| (default) | Table with name, branch, status, tmux, path |
| `--json` / `-j` | JSON array with all fields (`name`, `branch`, `status`, `tmux`, `path`, `current`, etc.) |
| `--paths` / `-p` | One absolute path per line |
| `--quiet` / `-q` | One short name per line |

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

### Inspect the Current Worktree

```bash
grove here              # short summary
grove here --json       # machine-readable
grove context           # full context (sidebar-style)
grove context --json    # richer JSON
```

`grove here` is the cheap option — name, branch, short SHA, status, tmux/Docker state. Useful for "where am I?" probes between commands.

`grove context` returns a richer payload tailored for agents that need to make decisions without re-running git themselves: tracking branch, ahead/behind counts, stash count, and the last 5 commits. **Prefer `grove context --json` over `grove here --json`** when you need any of:

- Sync state vs remote (`ahead`, `behind`, `has_remote`)
- Stash awareness (`stash_count`)
- Recent history (`recent_commits`) without shelling out to `git log`

Both commands exit non-zero when not inside a grove-managed worktree.

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
# Created worktree 'pr-42-auth-refactor' from branch 'feature/auth-refactor'
```

The worktree is named `pr-<N>-<title-slug>` (issue fetches use `issue-<N>-<title-slug>`).
Take the exact name from fetch's output or `grove ls` — don't guess it.

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

**Diff:**
```bash
grove diff <name>
# Shows diff between current worktree and <name>
```

**Graft commits or WIP from another worktree:**
```bash
grove graft <name>
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

Protected worktrees (`main`, `develop`, or any name in `[protection] protected`) require `--force` to remove. Immutable worktrees block `grove graft` and `grove sync` but can still be removed with `grove rm`.

**Remove stale worktrees in bulk:**
```bash
grove trim                 # removes worktrees not accessed in 30 days
grove trim --older-than 7  # shorter threshold
```

The `clean` alias also works (`grove clean --older-than 7`), but `trim` is the canonical name.

Stale = `LastAccessedAt` older than threshold and no dirty changes. Protected worktrees are skipped.

### When a Worktree Drifts (created via `git worktree add`)

If a worktree was created outside grove (typically by running `git worktree add` directly), grove won't have it in state and won't have run its bootstrap hooks. Symptoms:

- `grove ls` doesn't list the worktree.
- Docker auto-start and post-create hooks never ran, so credentials/env files may be missing.

Grove detects drift automatically — running any command from a drifted worktree prints a non-fatal warning:

```
⚠ this worktree (project-feature) wasn't created by grove and isn't registered in state
  run 'grove adopt' to bootstrap it (registers state, records excludes, runs hooks)
```

`grove doctor` also reports drift in its Tier-2 project checks.

**Fix:** `cd` into the worktree and run:

```bash
grove adopt
```

`grove adopt` is idempotent — running it twice on the same worktree prints "already registered" and exits successfully. To adopt a worktree from outside it:

```bash
grove adopt /path/to/other-worktree
```

The short name is derived from the directory by stripping the project prefix (`project-feature` → `feature`), matching the convention `grove new` produces.

**Edge cases:**

| Scenario | Behavior |
|----------|----------|
| Already adopted | Prints `worktree "<name>" is already registered (path: ...)` and exits 0. Safe to call repeatedly. |
| Detached HEAD | Errors out with `worktree is in detached HEAD state; check out a branch first` rather than registering with the literal branch `HEAD`. Run `git checkout -b <branch>` (or `git switch -c <branch>`) inside the worktree first, then re-run `grove adopt`. |
| Path outside the project | Returns an error — adopt only registers worktrees that belong to the current grove project. |

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

You can add custom `post_switch` hooks alongside Docker's automatic behavior:

```toml
# .grove/hooks.toml — run after every worktree switch
[[hooks.post_switch]]
type        = "command"
command     = "bin/rails db:migrate"
working_dir = "new"
on_failure  = "warn"

[[hooks.post_switch]]
type        = "command"
command     = "git pull origin main --ff-only"
on_failure  = "warn"
```

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

grove ps                     # table of active stacks
grove ps --json              # machine-readable

grove down                   # stop the stack (isolated stacks auto-detected from cwd)
grove ps                     # show running stacks with reference IDs and URLs
```

`agent-status` is a back-compat alias for `ps` and still works (`grove agent-status --json`).

Each isolated stack gets a unique compose project name (`{project}-agent-{N}`) and port-offset containers.

Configure max concurrent stacks:
```toml
[plugins.docker.external.agent]
max_slots = 5
network = "shared"
url_pattern = "http://localhost:{slot}"  # {slot} is the slot number; empty disables URL display
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
# Worktree directory naming template
# Must contain {project} and {name} exactly once each; literals limited to [A-Za-z0-9._-]
# Branch names are not affected (the branch is named after the worktree, or --branch)
# pattern = "{project}-{name}"  # default

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
# url_pattern = "http://localhost:{slot}"  # {slot} = slot number; empty disables URL display

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
| `GROVE_NO_UPDATE_NOTIFIER` | unset | Set to `1` to suppress the "update available" annotation across all grove commands. Also honors the standard `NO_UPDATE_NOTIFIER`. Implied by `GROVE_AGENT_MODE=1` and common CI vars. |

### Persistent flags (apply to every command)

| Flag | Purpose |
|------|---------|
| `--no-update-notifier` | Suppress the update annotation for this invocation only. Useful for one-off scripted runs without exporting the env var. |
| `--check-update` | Force a synchronous update check, bypassing the cache and the standard opt-outs. Prints either an update box or `grove is up to date (X.Y.Z)` and exits. Use when an agent explicitly wants to know the latest version. |

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
grove ps --json
# [{"slot":1,"worktree":"agent-task-1","compose_project":"myapp-agent-1","url":"http://localhost:3101"},
#  {"slot":2,"worktree":"agent-task-2","compose_project":"myapp-agent-2","url":"http://localhost:3102"}]
# (`url` is omitted when no url_pattern is configured)

# When done, clean up (run `grove down` inside each worktree — its stack is auto-detected)
cd ../myapp-agent-task-1
grove down
grove rm agent-task-1

cd ../myapp-agent-task-2
grove down
grove rm agent-task-2
```

### Recommended: Non-Destructive Code Review

Review a PR without touching the developer's active environment:

```bash
export GROVE_AGENT_MODE=1

# Fetch the PR branch as a new worktree (no switch, no hooks that affect active stack)
grove fetch pr/42
# Created worktree 'pr-42-fix-login-bug' ... — use the exact name it prints

# Peek into it without firing hooks
grove to pr-42-fix-login-bug --peek

# Run tests in the PR worktree without leaving current context
grove test pr-42-fix-login-bug
# or with extra args (appended verbatim — no `--` separator):
grove test pr-42-fix-login-bug --tag focus

# When done
grove rm pr-42-fix-login-bug
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

### Security

Grove runs `command` hooks via `sh -c` with the full parent-process environment forwarded. There is no sandboxing — hook commands can read environment variables, write files, make network requests, and spawn subprocesses.

**For agents running grove autonomously on unfamiliar repositories:**

- **Treat `hooks.toml` as untrusted input until reviewed.** A malicious or misconfigured hooks file can exfiltrate secrets, modify files outside the worktree, or run arbitrary code — the same risk as running `make` or `npm install` in an unknown repo.
- **Run `grove doctor` before `grove new` on unknown repos.** `grove doctor` validates config and hooks file syntax; reviewing its output gives you a chance to inspect what hooks will fire before any worktree is created.
- **Inspect `.grove/hooks.toml` before the first `grove new` or `grove to`.** Pay particular attention to `type = "command"` hooks, which are the most open-ended.
- **Environment variables are visible to hooks.** If your agent session has credentials or API tokens in the environment, `command` hooks can read them. Scope credentials to the minimum needed for the task.
- **`copy` and `symlink` hooks are lower risk** but can still expose sensitive files from the main worktree. Verify paths in `copy_files`, `symlink_files`, and `symlink_dirs` if security matters.

For the general (non-agent) trust model, see [Trust model in README.md](../README.md#hooks-trust-model).

---

## 8. Troubleshooting

### `grove doctor`

Always run this first. It works in two tiers:

**Tier 1 — System checks (run anywhere, no grove project required):**
- Grove binary resolution (with PATH hints if not found)
- Shell integration version
- Git, tmux, gh CLI availability
- Docker availability and daemon status

**Tier 2 — Project checks (only when inside a grove project):**
- Config validity and symlink health across all worktrees
- External Docker mode configuration
- Env file loader detection and configuration
- Agent stack configuration and slot usage

```bash
grove doctor
# ✓ Grove binary (/opt/homebrew/bin/grove)
# ✓ Shell integration (v3, current)
# ✓ Git (2.43.0)
# ✓ Tmux (tmux 3.4)
# ✓ GitHub CLI (found)
# ✓ Docker available (found in PATH)
# ✓ Docker running (v27.1.1)
#
# ℹ Project: ~/projects/myapp
# ✓ Config (loaded)
# ✓ Config symlinks (4 worktrees checked)
# ✗ Env file loader not configured
#   Run: echo 'dotenv_if_exists .env.local' > .envrc && direnv allow
```

Run `grove doctor` outside a grove project to diagnose installation and PATH issues without needing to be in a project first.

### Common Issues

**`grove: command not found`**

The binary is not in `$PATH`. Verify:
```bash
which grove || echo "not found"
# For Homebrew: check brew --prefix, ensure /opt/homebrew/bin or /usr/local/bin is in PATH
# For go install: ensure $GOPATH/bin (usually ~/go/bin) is in PATH
```

**`permission denied` or empty output when running `grove`**

The shell wrapper resolved `__GROVE_BIN` to an empty string because the grove binary isn't in PATH for this shell session. Common cause: Homebrew's `brew shellenv` is in `~/.zprofile` (login shells only) instead of `~/.zshenv` (all shells).

Non-login shells (Claude Code, `zsh -c`, cron, scripts) never source `.zprofile`, so `/opt/homebrew/bin` is missing from PATH.

Fix:
```bash
# Move brew shellenv from ~/.zprofile to ~/.zshenv
# In ~/.zshenv (runs for ALL zsh invocations):
eval "$(/opt/homebrew/bin/brew shellenv)"

# Remove the same line from ~/.zprofile
```

Verify:
```bash
zsh -c 'command -v grove'    # should print a path
# If using chezmoi, edit the source files and run chezmoi apply
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

**`not a grove project — main worktree has no .grove directory` when inside a worktree**

You're in a secondary worktree (e.g., `admin-feature-x`) but the main worktree (e.g., `admin`) was never initialized with `grove init`:
```bash
# Check where the main worktree is
git worktree list | head -1

# Initialize from there
cd /path/to/main/worktree
grove init
```

After initializing, secondary worktrees will be detected via grove's main-worktree fallback.

**`config symlink broken` warning**

Only affects worktrees created by older grove versions, which placed a
`.grove/config.toml` symlink in each worktree. Current grove resolves config
directly from the main worktree (via git's common dir) and creates no
per-worktree copies; a broken legacy symlink means the main worktree's
`.grove` was deleted or never created:
```bash
# Check what the symlink points to
ls -la .grove/config.toml

# Fix: initialize the main worktree, or simply delete the stale symlink
grove init   # from the main worktree, if .grove is missing entirely
```

### Upgrading across the config-layout change

Older grove versions git-ignored `.grove/config.toml` (so it could never be
committed) and planted a `config.toml` symlink in every worktree. Current
grove treats `config.toml`/`hooks.toml` as **committable project files**,
resolves config from the main worktree on every command, and keeps only
genuinely machine-local files (`state.json`, `state.lock`, `ui_prefs.json`,
`.envrc`, `config.local.toml`) in the git exclude.

The upgrade is self-healing: the first grove command in a legacy repo rewrites
the exclude block and prints this one-time stderr notice. It fires when the repo
still carries the pre-0.10 layout — either a mid-development exclude block that
listed `config.toml`, or (the case real upgraders hit) the legacy per-worktree
`.grove/config.toml` symlinks left in existing worktrees — and is recorded via a
machine-local sentinel so it shows at most once per clone:

```
grove: .grove/config.toml is now a committable project file — commit it to share config with your team
grove: existing worktrees may carry a legacy .grove/config.toml symlink (shows as untracked) — run 'grove doctor' for cleanup steps
```

Agents: this notice (and every grove notice) goes to **stderr**, so `--json`
stdout contracts are unaffected — never treat it as command output, and don't
re-trigger work because of it. If a user asks for help after upgrading, walk
this checklist:

1. **`git add .grove/config.toml` refused as "ignored"** — the repo hasn't run
   a current-grove command yet. Run any grove command (e.g. `grove ls`) to
   auto-migrate, or remove the `.grove/config.toml` line from the grove block
   in `.git/info/exclude` by hand.
2. **A worktree shows untracked or typechanged `.grove/config.toml`** — legacy
   symlink debris. In that worktree:
   ```bash
   rm .grove/config.toml                     # delete the symlink
   git checkout -- .grove/config.toml        # only if the project commits it
   rmdir .grove 2>/dev/null || true          # drop the dir if now empty
   ```
   `grove doctor` lists every affected worktree.
3. **Project config not yet shared** — from the main worktree:
   `git add .grove && git commit` (config.toml + hooks.toml are the
   committable pair; machine-local files are already excluded).
4. **Config seems wrong from a worktree** — confirm the grove binary is
   current (`grove version`); old binaries still read the worktree-local
   path.

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
