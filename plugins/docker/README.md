# Docker Plugin

The Docker plugin provides automatic container lifecycle management for Grove worktrees.

## Features

- **Automatic container management**: Start/stop containers when switching worktrees
- **Multi-compose file support**: Supports docker-compose.yml, docker-compose.yaml, compose.yml, compose.yaml
- **Service-level control**: Manage individual services or all at once
- **Log streaming**: Tail logs from running containers
- **Modern Docker Compose**: Works with both `docker compose` and `docker-compose`

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

## Hook Integration

The Docker plugin integrates with Grove's hook system to automatically manage containers:

### Post-Switch Hook (Auto-Start)

When you switch to a worktree, the plugin automatically starts containers if:
- A docker-compose file exists in the worktree
- Auto-start is enabled in configuration (default: true)

### Pre-Switch Hook (Auto-Stop)

When you switch away from a worktree, the plugin can optionally stop containers if:
- A docker-compose file exists in the worktree
- Auto-stop is enabled in configuration (default: false)

## Configuration

The Docker plugin can be configured in your `~/.config/grove/config.toml`:

```toml
[plugins.docker]
# Enable/disable the plugin
enabled = true

# Auto-start containers when switching to a worktree
auto_start = true

# Auto-stop containers when switching away from a worktree
auto_stop = false
```

### Defaults

If not specified in your configuration file, these defaults are used:
- `enabled`: true (if docker is available)
- `auto_start`: true
- `auto_stop`: false

## Requirements

The Docker plugin requires one of the following:

- **Docker with Compose V2**: `docker compose` command available
- **Docker Compose V1**: `docker-compose` command available

The plugin automatically detects which version is available and uses the appropriate command.

## How It Works

1. **Detection**: The plugin looks for docker-compose files in your worktree directory
2. **Command Execution**: Runs docker-compose commands in the worktree directory
3. **Hook Integration**: Registers hooks to run at appropriate lifecycle events

## Supported Compose Files

The plugin looks for these files (in order):
- `docker-compose.yml`
- `docker-compose.yaml`
- `compose.yml`
- `compose.yaml`

## Examples

### Basic Workflow

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

### Development Workflow

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

The plugin requires a docker-compose file in the worktree directory.

**Solution**: Add a `docker-compose.yml` file to your worktree.

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

## Planned Features

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

See the [Plugin Development Guide](../../docs/plugins.md) for more information on creating plugins.

## License

Apache 2.0 - see [LICENSE](../../LICENSE)
