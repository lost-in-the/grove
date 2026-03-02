# Docker Plugin

The Docker plugin provides automatic container lifecycle management for Grove worktrees. It supports two modes: **local** (each project has its own compose file) and **external** (services defined in a shared, central compose setup).

## Features

- **Automatic container management**: Start/stop containers when switching worktrees
- **Multi-compose file support**: Supports docker-compose.yml, docker-compose.yaml, compose.yml, compose.yaml
- **Service-level control**: Manage individual services or all at once
- **Log streaming**: Tail logs from running containers
- **Modern Docker Compose**: Works with both `docker compose` and `docker-compose`
- **External compose mode**: Manage projects whose Docker services live in a shared orchestrator directory

## Commands

### `grove up`

Start containers for the current worktree.

```bash
grove up              # Start containers in detached mode (default)
grove up --detach=false  # Start containers in foreground
w up                  # Using alias
```

**Options:**
- `-d, --detach` - Run containers in the background (default: true)

### `grove down`

Stop containers for the current worktree.

```bash
grove down   # Stop all containers
w down       # Using alias
```

### `grove logs [service]`

View logs from running containers.

```bash
grove logs           # Show logs from all services
grove logs web       # Show logs from 'web' service only
grove logs -f=false  # Show logs without following
w logs db            # Using alias
```

**Options:**
- `-f, --follow` - Follow log output (default: true)

### `grove restart [service]`

Restart container services.

```bash
grove restart        # Restart all services
grove restart web    # Restart 'web' service only
w restart db         # Using alias
```

## Modes

### Local Mode (default)

Each worktree has its own `docker-compose.yml`. Commands run in the worktree directory. This is the default behavior when no `mode` is specified.

### External Mode

For projects whose Docker services are defined in a shared, external compose setup (e.g., a central `shared-infra` directory that orchestrates multiple apps). In this mode:

- Commands run in the **external compose directory**, not the worktree
- An **environment variable** (e.g., `APP_DIR`) points the compose config to the active worktree
- Only the **configured services** are managed (shared infra like MySQL/Redis is untouched)
- `grove down` uses `docker compose stop` (not `down`) to preserve the shared network
- `grove new` **copies credentials** and **creates symlinks** from the main worktree
- `auto_stop` defaults to **true** (prevents stale services pointing to the wrong worktree)

### Isolated Stack Mode

Isolated stacks allow multiple independent Docker environments to run simultaneously for the same worktree. This is designed for AI agent workflows where several agents need their own database and service instances without conflicting with each other or the developer's active stack.

**Starting an isolated stack:**

```bash
grove up --isolated          # Allocate a slot and start services
grove up --isolated --slot 3 # Use a specific slot number
```

Each isolated stack gets:
- A unique compose project name (`{project}-{worktree}-slot-{N}`)
- Its own set of containers with port offsets based on the slot number
- An isolated database volume

**Checking active stacks:**

```bash
grove agent-status           # Human-readable table of active isolated stacks
grove agent-status --json    # Machine-readable JSON output
```

**Stopping an isolated stack:**

```bash
grove down --slot 3          # Stop a specific isolated stack
grove down                   # Auto-detects and stops the current stack
```

#### Configuration

Configure isolated stacks in `.grove/config.toml`:

```toml
[plugins.docker.agent]
max_slots = 5           # Maximum concurrent isolated stacks (default: 5)
network = "shared"      # Docker network for isolated stacks
url_pattern = "http://localhost:{port}"  # URL template for service discovery
```

#### Agent Config Reference

| Field | Default | Description |
|-------|---------|-------------|
| `max_slots` | `5` | Maximum number of concurrent isolated stacks |
| `network` | `"shared"` | Docker network name for inter-service communication |
| `url_pattern` | `"http://localhost:{port}"` | URL template; `{port}` is replaced with the allocated port |

#### Example Workflow

```bash
# Developer is working normally
grove to feature-x
grove up                     # Start dev stack

# Agent 1 needs its own environment
grove up --isolated          # Gets slot 1, starts services on offset ports

# Agent 2 also needs an environment
grove up --isolated          # Gets slot 2, different ports

# Check what's running
grove agent-status
# SLOT  PROJECT                    STATUS   PORTS
# 1     myapp-feature-x-slot-1     running  3101, 3111
# 2     myapp-feature-x-slot-2     running  3102, 3112

# Clean up
grove down --slot 1
grove down --slot 2
```

## Hook Integration

The Docker plugin integrates with Grove's hook system to automatically manage containers:

### Post-Switch Hook (Auto-Start)

**Local mode**: Starts containers if a docker-compose file exists in the worktree and auto-start is enabled (default: true).

**External mode**: Runs `docker compose up -d <services>` in the external compose directory with the env var pointing to the new worktree.

### Pre-Switch Hook (Auto-Stop)

**Local mode**: Optionally stops containers (default: false).

**External mode**: Runs `docker compose stop <services>` to stop the app services (default: true).

### Post-Create Hook

**Local mode**: No-op.

**External mode**: Copies configured credential files and creates symlinks from the main worktree into the new worktree.

## Configuration

### Local Mode

Configure in `~/.config/grove/config.toml` or `.grove/config.toml`:

```toml
[plugins.docker]
enabled = true
auto_start = true
auto_stop = false
```

### External Mode

Configure in your project's `.grove/config.toml`:

```toml
[plugins.docker]
enabled = true
auto_start = true
auto_stop = true
mode = "external"

[plugins.docker.external]
# Path to the shared compose directory
path = "~/projects/shared-infra"

# Environment variable that the compose YAML reads to find this app
env_var = "APP_DIR"

# Services to start/stop/restart (only these, not shared infra)
services = [
  "app", "app_worker", "app_esbuild",
]

# Files to copy from main worktree on grove new (credentials, config)
copy_files = [
  "config/credentials/development.key",
  "config/credentials/test.key",
  "config/master.key",
  "config/settings.local.yml",
]

# Directories to symlink from main worktree on grove new
symlink_dirs = [
  "vendor/bundle",
  "node_modules",
]
```

### External Mode Config Reference

| Field | Required | Description |
|-------|----------|-------------|
| `path` | Yes | Absolute path to the external compose directory (supports `~`) |
| `env_var` | Yes | Environment variable name the compose YAML reads |
| `services` | Yes | List of service names to manage |
| `copy_files` | No | Files to copy from main worktree on `grove new` |
| `symlink_dirs` | No | Directories to symlink from main worktree on `grove new` |

### Defaults

**Local mode:**
- `enabled`: true (if docker is available)
- `auto_start`: true
- `auto_stop`: false

**External mode:**
- `enabled`: true (if docker is available)
- `auto_start`: true
- `auto_stop`: true

## Requirements

The Docker plugin requires one of the following:

- **Docker with Compose V2**: `docker compose` command available
- **Docker Compose V1**: `docker-compose` command available

The plugin automatically detects which version is available and uses the appropriate command.

## How It Works

### Local Mode

1. **Detection**: The plugin looks for docker-compose files in your worktree directory
2. **Command Execution**: Runs docker-compose commands in the worktree directory
3. **Hook Integration**: Registers hooks to run at appropriate lifecycle events

### External Mode

1. **Configuration**: Reads `mode = "external"` and the `[plugins.docker.external]` table
2. **Command Execution**: Runs docker-compose in the external compose directory with the env var set
3. **Service Scoping**: Only manages the configured services, leaving shared infrastructure running
4. **Worktree Setup**: On `grove new`, copies credentials and creates symlinks from the main worktree

## Supported Compose Files (Local Mode)

The plugin looks for these files (in order):
- `docker-compose.yml`
- `docker-compose.yaml`
- `compose.yml`
- `compose.yaml`

In external mode, the compose file is expected to exist in the external compose directory.

## Examples

### Local Mode: Basic Workflow

```bash
# Create a new worktree with a docker-compose.yml
w new feature-api

# Switch to it (containers auto-start if docker-compose.yml exists)
w to feature-api

# Check logs
w logs

# Restart a specific service
w restart api

# Switch away (containers keep running by default)
w last

# Manually stop containers in a worktree
cd ~/projects/feature-api
w down
```

### External Mode: Multi-App Development

```bash
# Create a new worktree — credentials are copied, symlinks created
w new feature-x
#   copied config/credentials/development.key
#   copied config/master.key
#   symlinked vendor/bundle
#   symlinked node_modules

# Switch to it — stops app services, restarts with APP_DIR=./app-feature-x
w to feature-x

# Check app logs from the external compose
w logs app

# Switch back to main — stops services, restarts with APP_DIR=./app
w to main

# Manually stop app services
w down
```

### Local Mode: Development Workflow

```bash
# Start working on a feature
w new feature-auth
w to feature-auth

# Containers start automatically
# Make code changes...

# Check logs for your service
w logs auth-service

# Restart after configuration changes
w restart auth-service

# Switch to a different task (containers keep running)
w to bugfix-login

# Come back later
w to feature-auth
# Containers are still running, ready to go
```

## Troubleshooting

### "docker or docker-compose not found"

The plugin is automatically disabled if neither `docker` nor `docker-compose` is found in your PATH.

**Solution**: Install Docker and ensure it's in your PATH.

### "no docker-compose file found"

In local mode, the plugin requires a docker-compose file in the worktree directory.

**Solution**: Add a `docker-compose.yml` file to your worktree, or configure external mode if services are defined elsewhere.

### Containers don't start automatically

Check that auto-start is enabled in your configuration.

**Solution**: 
1. Check your `~/.config/grove/config.toml` file
2. Ensure `auto_start = true` (or omit it for the default)
3. Or manually start containers with `grove up`

Example config:
```toml
[plugins.docker]
auto_start = true
```

## Port Conflicts

When working with multiple worktrees that run containers, you may encounter port conflicts. Best practices:

1. **Use unique ports per worktree**: Modify your docker-compose.yml to use different ports
2. **Stop containers when not in use**: Use `grove down` when switching away
3. **Enable auto-stop**: Set `auto_stop = true` in your config:
   ```toml
   [plugins.docker]
   auto_stop = true
   ```

## grove test Integration

The Docker plugin integrates with `grove test` to run test commands in an ephemeral container without switching your active dev stack. Configure a service name in `.grove/config.toml`:

```toml
[test]
command = "bin/rails test"
service = "app"   # Docker service to run tests in
```

When `service` is set, `grove test <worktree>` spawns a fresh container in the target worktree's Docker environment. This lets you run tests against a feature branch while your current worktree's containers remain active.

## Planned Features

- [ ] Multi-app external mode: Extend external compose support to additional apps
- [ ] Port conflict detection and automatic prevention
- [ ] Environment variable generation via direnv integration
- [ ] Status command to show running containers per worktree
- [ ] Advanced port allocation strategy to prevent conflicts automatically

## Plugin API

The Docker plugin implements the `plugins.Plugin` interface:

```go
type Plugin interface {
    Name() string
    Init(cfg *config.Config) error
    RegisterHooks(registry *hooks.Registry) error
    Enabled() bool
}
```

See the [Plugin Development Guide](../../docs/PLUGIN_DEVELOPMENT.md) for more information on creating plugins.

## License

Apache 2.0 - see [LICENSE](../../LICENSE)
