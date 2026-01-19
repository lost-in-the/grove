# Plugin Development Guide

Grove's plugin system allows you to extend functionality by hooking into worktree lifecycle events.

## Plugin Interface

All plugins must implement the `Plugin` interface:

```go
type Plugin interface {
    Name() string
    Initialize() error
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
    "github.com/LeahArmstrong/grove-cli/internal/hooks"
    "github.com/LeahArmstrong/grove-cli/internal/plugins"
)

type MyPlugin struct {
    // Plugin state
}

func New() *MyPlugin {
    return &MyPlugin{}
}

func (p *MyPlugin) Name() string {
    return "myplugin"
}

func (p *MyPlugin) Initialize() error {
    // Register hooks
    hooks.Register(hooks.EventPostSwitch, p.onPostSwitch)
    return nil
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
    myplugin "github.com/LeahArmstrong/grove-cli/plugins/myplugin"
)

func initializePlugins() error {
    // Initialize your plugin
    if err := myplugin.New().Initialize(); err != nil {
        return fmt.Errorf("myplugin: %w", err)
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
    "github.com/LeahArmstrong/grove-cli/internal/hooks"
)

func TestPluginName(t *testing.T) {
    p := New()
    if p.Name() != "myplugin" {
        t.Errorf("expected name 'myplugin', got '%s'", p.Name())
    }
}

func TestInitialize(t *testing.T) {
    p := New()
    err := p.Initialize()
    if err != nil {
        t.Errorf("Initialize() failed: %v", err)
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

See: `plugins/docker/`

### Time Tracking Plugin

The time tracking plugin demonstrates:
- Persistent state management
- Hook-driven automation
- JSON data storage

See: `plugins/time/`

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

- [ ] Implements Plugin interface
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
