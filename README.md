# Grove

Zero-friction worktree management for developers.

## Features

- **Interactive TUI Dashboard** — run `grove` with no args to browse, create, and manage worktrees in a full-screen terminal UI
- **Fast Context Switching** — switch between git worktrees in <500ms with automatic directory changes
- **Tmux Integration** — each worktree gets its own tmux session, created and attached automatically
- **Docker Support** — start, stop, and tail containers scoped to each worktree
- **GitHub Integration** — create worktrees from issues and PRs; browse with an interactive TUI or fzf
- **Worktree Lifecycle Hooks** — auto-copy files, run setup commands, or trigger custom scripts on create/switch
- **Shell Integration** — `grove to <name>` actually changes your directory, with tab completion and a `w` alias
- **Configurable** — TOML-based config with global and per-project overrides, tmux mode control, and protection rules

## Installation

### Homebrew (macOS/Linux)

```bash
brew tap LeahArmstrong/tap
brew install grove
```

Recommended — includes automatic updates and shell completions.

### Go Install

```bash
go install github.com/LeahArmstrong/grove-cli/cmd/grove@latest
```

### From Release Binaries

Download the latest release for your platform from the [releases page](https://github.com/LeahArmstrong/grove-cli/releases).

```bash
# macOS (arm64)
curl -L https://github.com/LeahArmstrong/grove-cli/releases/latest/download/grove-cli_v1.0.0_Darwin_arm64.tar.gz | tar xz
sudo mv grove /usr/local/bin/
```

### Build from Source

```bash
git clone https://github.com/LeahArmstrong/grove-cli
cd grove-cli
make build
sudo make install
```

## Quick Start

**1. Set up shell integration**

Add this to your `~/.zshrc` or `~/.bashrc`:

```bash
# zsh
eval "$(grove install zsh)"

# bash
eval "$(grove install bash)"
```

Reload your shell: `source ~/.zshrc` (or `~/.bashrc`)

This enables directory switching, tab completion, and the `w` alias.

**2. Initialize a project**

```bash
cd my-project
grove init
```

Auto-detects project type (Rails, Node, Go, Python, Docker) and generates `.grove/hooks.toml` with sensible defaults.

**3. Create and switch worktrees**

```bash
grove new feature-login    # Create worktree + tmux session
grove to feature-login     # Switch (changes directory automatically)
grove ls                   # List all worktrees
grove here                 # Show current worktree info
```

**4. Launch the TUI**

```bash
grove    # Opens interactive dashboard (inside a grove project)
```

## Commands

### Core

| Command | Description |
|---------|-------------|
| `grove ls` | List all worktrees with status |
| `grove new <name>` | Create a worktree and tmux session |
| `grove new <name> --mirror origin/main` | Create an environment worktree tracking a remote branch |
| `grove to <name>` | Switch to a worktree (changes directory, attaches tmux) |
| `grove rm <name>` | Remove a worktree and kill its tmux session |
| `grove here` | Show current worktree info (branch, SHA, age, status) |
| `grove last` | Switch to the previous worktree |
| `grove fork <name>` | Fork the current worktree into a new one (optionally move/copy WIP) |
| `grove compare <name>` | Compare current worktree with another (commits and WIP) |
| `grove apply <name>` | Cherry-pick commits or apply WIP from another worktree |
| `grove sync [name]` | Fast-forward environment worktrees from their remote mirrors |
| `grove clean` | Remove worktrees not accessed in N days (default: 30) |
| `grove test <name> [args]` | Run the configured test command in a specific worktree |

### GitHub / Issue Tracker

Requires `gh` CLI. See [Tracker Plugin](plugins/tracker/README.md) for details.

| Command | Description |
|---------|-------------|
| `grove fetch pr/<number>` | Create a worktree from a GitHub PR |
| `grove fetch issue/<number>` | Create a worktree from a GitHub issue |
| `grove issues` | Browse open issues in TUI (or `--fzf`) and create a worktree |
| `grove prs` | Browse open PRs in TUI (or `--fzf`) and create a worktree |

### Docker

Requires a `docker-compose.yml` in the worktree. See [Docker Plugin](plugins/docker/README.md) for details.

| Command | Description |
|---------|-------------|
| `grove up` | Start Docker containers for the current worktree |
| `grove down` | Stop Docker containers for the current worktree |
| `grove logs [service]` | Tail container logs |
| `grove restart [service]` | Restart container(s) |
| `grove up --isolated` | Start an isolated Docker stack (for parallel agents) |
| `grove agent-status` | Show active isolated stacks |

### Utility

| Command | Description |
|---------|-------------|
| `grove config` | Show merged configuration |
| `grove config --edit` | Open project config in `$EDITOR` |
| `grove config --global` | Show or edit global config |
| `grove config --hooks` | Show or edit hooks configuration |
| `grove init` | Initialize a grove project in the current git repo |
| `grove install <shell>` | Print shell integration code (eval this in your rc file) |
| `grove repair` | Detect and fix state/worktree inconsistencies |
| `grove doctor` | Check system health and configuration |
| `grove version` | Show version information |

## Configuration

Grove uses TOML configuration files loaded in this order (later overrides earlier):

1. Global: `~/.config/grove/config.toml`
2. Project: `.grove/config.toml`

```toml
# .grove/config.toml

project_name = "my-project"

[switch]
# How to handle dirty worktrees when switching: "auto-stash", "prompt", or "refuse"
dirty_handling = "prompt"

[naming]
# Pattern for worktree directory names
pattern = "{project}-{name}"

[tmux]
# Tmux integration mode: "auto" (default), "manual", or "off"
mode = "auto"

[test]
# Command to run via 'grove test <worktree>'
command = "bin/rails test"
# Optional: run in a Docker service instead of locally
# service = "app"

[plugins.docker]
enabled = true
auto_start = true   # Start containers when switching to a worktree
auto_stop = false   # Stop containers when switching away

[protection]
# Worktrees that cannot receive changes via 'grove apply'
immutable = ["main", "production"]
```

**Tmux modes:**
- `auto` — grove creates and attaches tmux sessions automatically
- `manual` — grove creates sessions but does not auto-attach
- `off` — no tmux integration

**Environment variables:**
- `GROVE_TUI=0` — disable TUI; bare `grove` shows help instead
- `GROVE_SHELL=1` — set by shell integration to enable directory switching
- `GROVE_LOG=1` — enable debug logging to `~/.grove/grove.log`
- `GROVE_LOG=/path/to/file` — enable debug logging to a custom path

## Plugins

### Docker Plugin

Manages Docker Compose containers scoped to each worktree. Supports auto-start/stop on switch, per-worktree container isolation, and an "external compose" mode for monorepos where the compose file lives outside the worktree.

See [plugins/docker/README.md](plugins/docker/README.md) for full configuration.

### Tracker Plugin

GitHub integration for creating worktrees from issues and PRs. Worktree names are auto-generated from issue/PR metadata.

See [plugins/tracker/README.md](plugins/tracker/README.md) for full configuration.

### Hooks

Grove runs hooks at lifecycle events to automate per-worktree setup. Configure in `.grove/hooks.toml` (auto-generated by `grove init`):

```toml
[[hooks.post_create]]
type = "copy"
from = ".env.example"
to = ".env"
required = false

[[hooks.post_create]]
type = "symlink"
from = "vendor/bundle"
to = "vendor/bundle"

[[hooks.post_create]]
type = "command"
command = "bundle install"
timeout = 300
on_failure = "warn"
```

Supported events: `post_create`, `pre_switch`, `post_switch`, `pre_remove`, `post_remove`.

See [docs/PLUGIN_DEVELOPMENT.md](docs/PLUGIN_DEVELOPMENT.md) for writing custom plugins.

## Shell Integration

Shell integration enables directory switching, tab completion, and the `w` alias.

```bash
# Add to ~/.zshrc or ~/.bashrc:
eval "$(grove install zsh)"   # or bash
```

See [docs/SHELL_INTEGRATION.md](docs/SHELL_INTEGRATION.md) for details on how the integration works and advanced configuration.

## TUI Dashboard

Run `grove` with no arguments inside a grove project:

```
 grove
  ❯ feature-login    main          3m ago    ● dirty  ⬡ tmux
    feature-auth     auth          1h ago    ✓ clean  ⬡ tmux
    hotfix-css       fix/css       2d ago    ✓ clean
────────────────────────────────────────────────────────────
  feature-login · b3a1f2c · "add login form validation"
  branch: feat/login  ↑2 ↓0  3 minutes ago
  M cmd/login.go
  M internal/auth/session.go
  + templates/login.html
 [enter] switch  [n] new  [d] delete  [/] filter  [?] help  [q] quit
```

Navigation: `j`/`k` or arrow keys. `/` to filter. `?` for full keybindings.

See [docs/TUI.md](docs/TUI.md) for the full reference.

## Requirements

- Git 2.30 or later
- Go 1.21 or later (for building from source)
- Tmux 3.0 or later (optional, for session management)
- zsh or bash (for shell integration)
- `gh` CLI (optional, for GitHub integration)

## Development

```bash
make build    # Build binary
make test     # Run tests
make lint     # Run linter
make fmt      # Format code
make install  # Install locally
```

See [CONTRIBUTING.md](CONTRIBUTING.md) for contribution guidelines.

## License

Apache 2.0 — see [LICENSE](LICENSE)
