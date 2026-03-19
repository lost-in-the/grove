# Claude Code Plugin

Grove's Claude Code plugin provides network-isolated devcontainer sandboxes per worktree, making `--dangerously-skip-permissions` safe for unattended agent runs.

## Quick Start

1. Enable the plugin in `.grove/config.toml`:

```toml
[plugins.claude]
enabled = true

[plugins.claude.devcontainer]
firewall = true
```

2. Create a worktree — the devcontainer is scaffolded automatically:

```bash
grove new feature-auth
```

3. Build and start the sandbox:

```bash
grove sandbox new feature-auth
grove sandbox start feature-auth
```

4. Run Claude Code inside the sandbox:

```bash
grove sandbox exec feature-auth -- claude --dangerously-skip-permissions
```

## Configuration Reference

See [Configuration Reference](CONFIGURATION_REFERENCE.md) for the complete `[plugins.claude]` section.

### Plugin Options

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| `enabled` | bool | `false` | Enable the Claude Code plugin |
| `auto_start` | bool | `false` | Start Claude on `grove open` |
| `skip_permissions` | bool | `true` | Pass `--dangerously-skip-permissions` inside devcontainer |
| `prompt` | string | `""` | Headless prompt for CI / `GROVE_AGENT_MODE` |
| `inject_grove_context` | bool | `true` | Auto-add grove instructions to CLAUDE.md |

### Devcontainer Options

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| `devcontainer.enabled` | bool | `true` | Wrap Claude in a devcontainer |
| `devcontainer.firewall` | bool | `false` | Inject init-firewall.sh |
| `devcontainer.allowed_domains` | []string | `[]` | Additional domains for the firewall allowlist |

### Permission Options

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| `permissions.allowed_tools` | []string | all | Restrict which tools the agent can use |
| `permissions.allowed_mcps` | []string | all | MCP servers to enable |
| `permissions.max_turns` | int | `0` | Max autonomous turns (0 = unlimited) |

## Architecture

The plugin follows Grove's standard plugin interface:

```
plugins/claude/
├── plugin.go          # Plugin interface implementation
├── devcontainer.go    # Devcontainer scaffolding + firewall
├── config.go          # Config file forwarding + permissions
├── agent_context.go   # Grove context injection into CLAUDE.md
├── sandbox.go         # Sandbox lifecycle operations
├── status.go          # StatusProvider for grove ls / TUI
├── plugin_test.go     # Tests
└── README.md          # Plugin documentation
```

## How Config Files Are Managed

When a devcontainer is scaffolded:

1. **CLAUDE.md** from the project root is bind-mounted read-only into the container
2. **~/.claude/** (user settings) is bind-mounted so user preferences apply
3. **Permission restrictions** from `[plugins.claude.permissions]` generate a `claude-settings.json` in `.devcontainer/`
4. **Grove context** is injected into CLAUDE.md so agents understand grove commands

## Multi-Agent Patterns

The plugin works with Grove's existing agent infrastructure:

- Each worktree gets its own sandbox — agents work in complete isolation
- `GROVE_AGENT_MODE=1` and `GROVE_NONINTERACTIVE=1` are set automatically in the container
- Agents can create additional worktrees via `grove new` from inside the container
- `grove sandbox status --json` provides machine-readable status for orchestration
- Docker isolated stacks (`grove up --isolated`) can coexist with devcontainer sandboxes
