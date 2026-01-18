# Grove

Zero-friction worktree management for developers.

## Features

- **Fast Context Switching**: Switch between git worktrees in <500ms
- **Automatic Tmux Integration**: Create and manage tmux sessions for each worktree
- **Simple Commands**: Just 6 core commands to learn
- **Shell Integration**: Directory changing works seamlessly with zsh and bash
- **Configurable**: TOML-based configuration with sensible defaults

## Installation

### From Source

```bash
go install github.com/LeahArmstrong/grove-cli/cmd/grove@latest
```

### Build Locally

```bash
git clone https://github.com/LeahArmstrong/grove-cli
cd grove-cli
make build
sudo make install
```

## Quick Start

### 1. Set up shell integration

Add this to your `~/.zshrc` or `~/.bashrc`:

```bash
# For zsh
eval "$(grove init zsh)"

# For bash
eval "$(grove init bash)"
```

Then reload your shell:
```bash
source ~/.zshrc  # or ~/.bashrc
```

### 2. Create your first worktree

```bash
# Create a new worktree called "feature-login"
grove new feature-login

# Or use the alias
w new feature-login
```

### 3. Switch between worktrees

```bash
# Switch to a worktree
grove to feature-login

# Your directory changes automatically!
# A tmux session is created/attached automatically
```

### 4. List all worktrees

```bash
grove ls
```

### 5. See where you are

```bash
grove here
```

### 6. Remove a worktree

```bash
grove rm feature-login
```

## Commands

### Core Commands

- `grove ls` (or `w ls`) - List all worktrees with status
- `grove new <name>` - Create worktree + tmux session
- `grove to <name>` - Switch to worktree (creates tmux session if needed)
- `grove rm <name>` - Remove worktree + kill tmux session
- `grove here` - Show current worktree info
- `grove last` - Switch to previous worktree

### Configuration

- `grove config` - Show current configuration
- `grove version` - Show version information
- `grove init <shell>` - Generate shell integration code

### Docker Plugin Commands

- `grove up` - Start containers for current worktree
- `grove down` - Stop containers for current worktree
- `grove logs [service]` - View container logs
- `grove restart [service]` - Restart container service(s)

See [Docker Plugin Documentation](plugins/docker/README.md) for details.

## Configuration

Grove uses TOML configuration files. Configuration is loaded from:

1. Global: `~/.config/grove/config.toml`
2. Project: `.grove/config.toml` (overrides global)

### Example Configuration

```toml
# ~/.config/grove/config.toml

# Command alias (default: "w")
alias = "w"

# Directory where projects are stored
projects_dir = "~/projects"

# Default branch to base new worktrees on
default_base_branch = "main"

[switch]
# How to handle dirty worktrees: "auto-stash", "prompt", or "refuse"
dirty_handling = "prompt"

[naming]
# Pattern for naming worktrees
pattern = "{type}/{description}"

[tmux]
# Prefix for tmux session names
prefix = "grove-"
```

### Defaults

If no configuration file exists, Grove uses these defaults:

- `alias`: "w"
- `projects_dir`: "~/projects"
- `default_base_branch`: "main"
- `dirty_handling`: "prompt"
- `pattern`: "{type}/{description}"
- `tmux.prefix`: "grove-"

## How It Works

1. **Worktrees**: Grove uses git worktrees to create separate working directories for different branches
2. **Tmux Sessions**: Each worktree gets its own tmux session for state management
3. **Shell Integration**: The shell wrapper intercepts directory change commands
4. **Hook System**: Extensible plugin system for custom workflows (Phase 1+)

## Plugins

Grove supports plugins to extend functionality. Plugins can hook into worktree lifecycle events to provide custom behavior.

### Available Plugins

#### Docker Plugin

Automatically manages Docker containers for your worktrees.

**Features:**
- Auto-start containers when switching to a worktree
- Manual container control with `grove up`, `grove down`, `grove logs`, `grove restart`
- Works with docker-compose.yml files
- Support for both `docker compose` and `docker-compose` commands

**Quick Start:**
```bash
# Navigate to a worktree with docker-compose.yml
cd ~/projects/my-worktree

# Start containers
grove up

# View logs
grove logs

# Restart a service
grove restart web

# Stop containers
grove down
```

See [Docker Plugin Documentation](plugins/docker/README.md) for full details.

### Creating Custom Plugins

Plugins implement the `Plugin` interface and can register hooks to run at lifecycle events:

- `pre-create` / `post-create` - Before/after worktree creation
- `pre-switch` / `post-switch` - Before/after switching worktrees
- `pre-freeze` / `post-resume` - Before freezing/after resuming (Phase 2)
- `pre-remove` / `post-remove` - Before/after worktree removal

See the [Plugin Development Guide](docs/plugins.md) for more information (coming soon).

## Requirements

- Go 1.21 or later (for building)
- Git 2.30 or later (for worktree support)
- Tmux 3.0 or later (optional, for session management)
- zsh or bash (for shell integration)

## Development

```bash
# Run tests
make test

# Run linter
make lint

# Format code
make fmt

# Build binary
make build

# Install locally
make install
```

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md) for contribution guidelines.

## License

Apache 2.0 - see [LICENSE](LICENSE)

## Roadmap

### Phase 0: Foundation ✅
- Core commands (ls, new, to, rm, here, last)
- Shell integration (zsh, bash)
- Configuration system
- Tmux integration
- Hook system foundation

### Phase 1: Docker Plugin ✅
- Container lifecycle tied to worktrees
- Service management commands (up, down, logs, restart)
- Auto-start/stop integration with hooks
- Plugin system infrastructure

### Phase 2: State Management (Planned)
- Freeze/resume functionality
- Dirty worktree handling
- State persistence

### Phase 3: Time Tracking (Planned)
- Passive time tracking per worktree
- Weekly summaries
- Parallel test execution

### Phase 4: Issue Integration (Planned)
- GitHub PR/issue integration
- Linear integration
- Smart worktree naming

### Phase 5: Polish (Planned)
- TUI mode
- Template system
- Database plugin
- Homebrew formula

## FAQ

**Q: Why worktrees instead of branches?**
A: Worktrees let you work on multiple branches simultaneously without stashing or losing context.

**Q: Do I need tmux?**
A: No, but it's recommended. Grove works without tmux, but you'll miss out on session management.

**Q: Can I use a different command alias?**
A: Yes! Set `alias = "myalias"` in your config file.

**Q: How do I uninstall?**
A: Remove the binary (`rm $(which grove)`), remove shell integration from your rc file, and delete `~/.config/grove/`.

## Support

- 🐛 [Report a bug](https://github.com/LeahArmstrong/grove-cli/issues/new?template=bug_report.md)
- 💡 [Request a feature](https://github.com/LeahArmstrong/grove-cli/issues/new?template=feature_request.md)
- 📚 [Documentation](https://github.com/LeahArmstrong/grove-cli/tree/main/docs)
