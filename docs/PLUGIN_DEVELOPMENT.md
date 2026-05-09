# Plugin Development Guide

Grove's plugin system allows you to extend functionality by hooking into worktree lifecycle events.

## Plugin Interface

All plugins must implement the `Plugin` interface:

```go
type Plugin interface {
    // Name returns the plugin identifier (e.g., "docker", "tracker").
    Name() string

    // Init initializes the plugin with grove configuration.
    // Called once at startup. Return an error to signal the plugin
    // is unavailable (e.g., required tool not found).
    Init(cfg *config.Config) error

    // RegisterHooks registers the plugin's hook handlers with the registry.
    // Called after Init succeeds.
    RegisterHooks(registry *hooks.Registry) error

    // Enabled returns whether the plugin is currently active.
    // Plugins may disable themselves when tools are missing or
    // the feature is turned off in config.
    Enabled() bool
}
```

## Hook System

Plugins register hooks to run at specific lifecycle events:

### Available Hook Events

- `pre-create` - Before worktree creation
- `post-create` - After worktree creation
- `pre-switch` - Before switching away from worktree
- `post-switch` - After switching to worktree
- `pre-freeze` - Before freezing worktree
- `post-resume` - After resuming frozen worktree
- `pre-remove` - Before removing worktree
- `post-remove` - After worktree removal

### Hook Context

Hooks receive a `Context` object with information about the operation:

```go
type Context struct {
    Worktree     string  // Target worktree name
    PrevWorktree string  // Previous worktree (for switch operations)
    Data         map[string]interface{}  // Additional plugin data
}
```

## Adding a Custom Action Type to hooks.toml

Beyond the built-in lifecycle hooks above, plugins can extend the
**config-driven** hooks system (`hooks.toml`) with custom action types. The
docker plugin uses this to add `type = "docker:compose"` and `type =
"docker:exec"`.

Register a handler in your plugin's `Init()`:

```go
import "github.com/lost-in-the/grove/internal/hooks"

func (p *MyPlugin) Init(cfg *config.Config) error {
    // ... your existing init ...
    hooks.RegisterActionHandler("myplugin:run", p.runHandler)
    return nil
}

func (p *MyPlugin) runHandler(
    action *hooks.HookAction,
    ctx *hooks.ExecutionContext,
    vars *hooks.Variables,
) error {
    if action.Command == "" {
        return fmt.Errorf("myplugin:run hook: 'command' is required")
    }
    command := vars.Interpolate(action.Command)
    // ... do work ...
    return nil
}
```

Users can then write:

```toml
[[hooks.post_create]]
type    = "myplugin:run"
command = "echo hello"
```

**Naming convention.** Use `pluginname:action` (colon-separated namespace).
This keeps user-facing types attributable and avoids collisions when multiple
plugins implement similar runners (`docker:exec` vs a hypothetical
`podman:exec`). The colon is purely a convention enforced by docs, not a
parser hook.

**Idempotent registration.** `RegisterActionHandler` is last-write-wins: re-
registering the same type swaps in the new handler. Plugin `Init()` can run
repeatedly across tests without bookkeeping, and the trade-off is that two
plugins claiming the same name will silently overwrite each other (use
namespaced names to avoid this).

**Disabled-plugin diagnostics.** If a user references your action type but
the plugin isn't loaded, the executor returns a generic "unknown hook action
type" error. The docker plugin extends `disabledTypeHint` in
`internal/hooks/executor.go` to give a more actionable hint for
`docker:compose` and `docker:exec` — consider adding to that switch if your
plugin is similarly load-conditional.

### Hook ordering invariant

When implementing custom action handlers, know which fires first:

1. **Plugin Go hooks** (registered via `Plugin.RegisterHooks` against
   `hooks.EventPostCreate`, etc.) fire **before** any `hooks.toml` actions.
2. **Config-driven hooks** (`[[hooks.post_create]]` entries in `hooks.toml`,
   including `docker:compose` and `docker:exec` action types) run after.

This ordering is what makes `type = "docker:compose"` with `mode = "exec"`
viable on `post_create` — the docker plugin's Go hook brings the local stack
up first, so by the time the config hook fires there's a running container
to exec into. A handler you add can rely on the same guarantee for resources
its plugin provisions.

The invariant is enforced in `internal/worktree/bootstrap.go` (`hooks.Fire`
runs before `hookExecutor.Execute`). Don't try to reorder by re-firing
`EventPostCreate` from a config-driven handler.

### Built-in action types: `docker:compose` and `docker:exec`

The docker plugin registers two action types that plugin authors can
reference for parity. Both are implemented in `plugins/docker/`.

**`docker:compose`** — runs a command via `docker compose run --rm` (default)
or `docker compose exec` against a service in the worktree's compose file.
Implementation: `plugins/docker/hook_compose.go`.

```toml
[[hooks.post_create]]
type       = "docker:compose"
service    = "app"                    # required: compose service name
command    = "bundle install --quiet" # required: shell command run inside the container
mode       = "run"                    # optional: "run" (default, ephemeral) | "exec" (existing container)
timeout    = 300                      # optional: seconds (default 60); applied by executor, not the handler
on_failure = "warn"                   # optional: "fail" (default) | "warn" | "ignore"
```

Field reference (matches `HookAction` in `internal/hooks/config.go`):

| Field | Required | Notes |
|-------|----------|-------|
| `service` | yes | Compose service name; supports `${VAR}` interpolation |
| `command` | yes | Shell command run inside the container |
| `mode` | no | `"run"` brings up service deps on demand; `"exec"` requires a running container and fails with a hint if the stack is down |
| `timeout` | no | Per-action timeout (seconds), enforced by the hook executor |
| `on_failure` | no | Standard error-handling field shared with all action types |

The handler returns a targeted error when no compose file is present at the
worktree path, suggesting `type = "docker:exec"` as the alternative.

**`docker:exec`** — runs a command directly via `docker exec` in an
externally-managed container that grove doesn't lifecycle (e.g., a long-
running dev shell started outside grove). Bypasses compose entirely.
Implementation: `plugins/docker/hook_docker_exec.go`.

```toml
[[hooks.post_create]]
type       = "docker:exec"
container  = "app-dev"                # required: docker container name (not service name)
command    = "bin/setup"              # required: command to run
shell      = "bash -lc"               # optional: shell wrapper (default "bash -lc"); plain command + flags only, no quotes
timeout    = 60                       # optional: seconds (default 60)
on_failure = "fail"
```

Differences from `docker:compose mode = "exec"`:

| | `docker:compose` (`mode = "exec"`) | `docker:exec` |
|---|---|---|
| Targets | A compose **service** in the worktree's compose file | A raw docker **container** by name |
| Requires compose file | Yes | No |
| Brings up dependencies | No (errors if stack is down) | No (errors if container isn't running) |
| Use when | The container is part of grove's managed compose stack | The container is started/managed outside grove |

The handler pre-checks that the container is running (3s-bounded
`docker inspect`) and returns an actionable error rather than the noisy
`docker exec` "no such container" output.

## Creating a Plugin

### 1. Create Plugin Package

Create a new directory under `plugins/`:

```bash
mkdir plugins/myplugin
cd plugins/myplugin
```

### 2. Implement Plugin Interface

```go
// plugins/myplugin/plugin.go
package myplugin

import (
    "github.com/lost-in-the/grove/internal/config"
    "github.com/lost-in-the/grove/internal/hooks"
)

type MyPlugin struct {
    cfg     *config.Config
    enabled bool
}

func New() *MyPlugin {
    return &MyPlugin{enabled: true}
}

func (p *MyPlugin) Name() string {
    return "myplugin"
}

func (p *MyPlugin) Init(cfg *config.Config) error {
    p.cfg = cfg
    // Check prerequisites, set p.enabled = false if unavailable
    return nil
}

func (p *MyPlugin) RegisterHooks(registry *hooks.Registry) error {
    registry.Register(hooks.EventPostSwitch, p.onPostSwitch)
    return nil
}

func (p *MyPlugin) Enabled() bool {
    return p.enabled
}

func (p *MyPlugin) onPostSwitch(ctx *hooks.Context) error {
    // Handle post-switch event
    return nil
}
```

### 3. Register Plugin

Add your plugin to the main initialization in `cmd/grove/main.go`:

```go
import (
    myplugin "github.com/lost-in-the/grove/plugins/myplugin"
)

func initializePlugins(cfg *config.Config, registry *hooks.Registry) error {
    p := myplugin.New()
    if err := p.Init(cfg); err != nil {
        return fmt.Errorf("myplugin: %w", err)
    }
    if p.Enabled() {
        if err := p.RegisterHooks(registry); err != nil {
            return fmt.Errorf("myplugin hooks: %w", err)
        }
    }
    return nil
}
```

### 4. Add Tests

Create comprehensive tests for your plugin:

```go
// plugins/myplugin/plugin_test.go
package myplugin

import (
    "testing"
    "github.com/lost-in-the/grove/internal/hooks"
)

func TestPluginName(t *testing.T) {
    p := New()
    if p.Name() != "myplugin" {
        t.Errorf("expected name 'myplugin', got '%s'", p.Name())
    }
}

func TestInit(t *testing.T) {
    p := New()
    err := p.Init(nil)
    if err != nil {
        t.Errorf("Init() failed: %v", err)
    }
}

func TestEnabled(t *testing.T) {
    p := New()
    _ = p.Init(nil)
    if !p.Enabled() {
        t.Error("expected plugin to be enabled")
    }
}
```

### 5. Add Documentation

Create a README.md in your plugin directory:

```markdown
# MyPlugin

Brief description of what your plugin does.

## Features

- Feature 1
- Feature 2

## Configuration

Describe any configuration options.

## Usage

Show examples of using your plugin.
```

## Example Plugins

### Docker Plugin

The Docker plugin demonstrates:
- Managing external services (Docker containers)
- Graceful degradation when tools are missing
- Configuration via compose files
- Multiple mode strategies (local vs external)

See: `plugins/docker/`

### Tracker Plugin

The tracker plugin demonstrates:
- Adapter pattern for multiple backends
- Registry pattern for extensibility
- Integration with external CLIs (gh, fzf)

See: `plugins/tracker/`

## Best Practices

### 1. Error Handling

- Wrap errors with context: `fmt.Errorf("failed to do X: %w", err)`
- Don't panic - return errors instead
- Log warnings for non-critical failures
- Allow operations to continue if plugin fails

### 2. State Management

- Store state in `~/.config/grove/state/`
- Use JSON for simple state
- Implement thread-safe operations with mutexes
- Use atomic file writes (write to temp, then rename)

### 3. External Dependencies

- Check if external tools exist before using them
- Provide helpful error messages if tools are missing
- Document all external dependencies in README
- Consider making dependencies optional

### 4. Testing

- Write table-driven tests
- Test error paths, not just happy paths
- Mock external dependencies
- Aim for >60% coverage

### 5. Performance

- Keep hook handlers fast (<100ms)
- Don't block on network I/O
- Cache results when possible
- Consider async operations for slow tasks

### 6. Configuration

- Use the global config system if possible
- Provide sensible defaults
- Document all configuration options
- Validate configuration on initialization

## Plugin Checklist

Before submitting a plugin:

- [ ] Implements Plugin interface (`Name()`, `Init()`, `RegisterHooks()`, `Enabled()`)
- [ ] Has comprehensive tests (>60% coverage)
- [ ] Has README.md with examples
- [ ] Handles errors gracefully
- [ ] Documents external dependencies
- [ ] Follows Go best practices
- [ ] Runs in <100ms for hook handlers
- [ ] Doesn't break when tools are missing

## Getting Help

- Review existing plugins for examples
- Check the [CONTRIBUTING guide](../CONTRIBUTING.md)
- Open an issue for questions
- Join discussions on GitHub

## Future Enhancements

Ideas for plugin system improvements:

- Plugin discovery/loading from external directories
- Plugin marketplace
- Plugin configuration via TOML
- Plugin versioning
- Plugin dependencies
