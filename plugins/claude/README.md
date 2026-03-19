# Claude Code Plugin

The Claude Code plugin provides first-class devcontainer sandbox support for Claude Code and other AI agents in Grove-managed worktrees.

## Overview

When enabled, this plugin:

- **Scaffolds devcontainers** per worktree with the [Anthropic reference devcontainer feature](https://github.com/anthropics/devcontainer-features)
- **Configures network isolation** via an optional firewall script that restricts outbound connections
- **Forwards Claude config files** (CLAUDE.md, settings.json) into the container
- **Injects grove context** so agents inside containers understand grove commands and multi-agent patterns
- **Manages sandbox lifecycle** via `grove sandbox` commands

## Configuration

Add to `.grove/config.toml`:

```toml
[plugins.claude]
enabled = true
auto_start = false              # start Claude on grove open?
skip_permissions = true         # pass --dangerously-skip-permissions inside devcontainer
prompt = ""                     # optional headless prompt for CI / GROVE_AGENT_MODE
inject_grove_context = true     # auto-add grove instructions to CLAUDE.md (default: true)

[plugins.claude.devcontainer]
enabled = true                  # wrap Claude in a devcontainer for sandboxing
firewall = true                 # inject init-firewall.sh
allowed_domains = [             # extend the default allowlist
  "rubygems.org",
  "bundler.io",
]

[plugins.claude.permissions]
allowed_tools = ["Bash", "Read", "Write", "Edit"]  # restrict agent tools
allowed_mcps = ["github"]                            # MCP servers to enable
max_turns = 100                                      # max autonomous turns
```

## Commands

| Command | Description |
|---------|-------------|
| `grove sandbox new <worktree>` | Build devcontainer for a worktree |
| `grove sandbox start <worktree>` | Start the sandbox (detached) |
| `grove sandbox stop <worktree>` | Stop the sandbox |
| `grove sandbox status` | Table of running sandboxes |
| `grove sandbox status --json` | Machine-readable output |
| `grove sandbox exec <worktree> -- <cmd>` | Run a command inside the sandbox |
| `grove sandbox rm <worktree>` | Stop and remove sandbox + volumes |

## Hooks

The plugin registers hooks for:

- **`post_create`**: Scaffolds `.devcontainer/devcontainer.json`, generates firewall script, forwards config files, injects grove context into CLAUDE.md
- **`pre_remove`**: Stops and removes the devcontainer

## Default Allowed Domains

When firewall is enabled, these domains are always allowed:

- `api.anthropic.com`
- `sentry.io`, `statsig.anthropic.com`
- `github.com`, `api.github.com`
- `registry.npmjs.org`
- `pypi.org`, `files.pythonhosted.org`
- `proxy.golang.org`, `sum.golang.org`

Additional domains can be added via `allowed_domains` in the config.

## Agent Context Injection

When `inject_grove_context = true` (default), the plugin appends a `## Grove Tooling` section to the worktree's CLAUDE.md. This teaches agents inside the container how to:

- Create and manage worktrees
- Use sandbox operations
- Work with Docker isolated stacks
- Coordinate with other agents

The injected content is idempotent — marked with HTML comment delimiters to prevent duplication on updates.

## Doctor Checks

When the plugin is enabled, `grove doctor` verifies:

- Node.js available
- `claude` CLI in PATH
- `ANTHROPIC_API_KEY` set
- `devcontainer` CLI available (when devcontainer mode enabled)
- Firewall configuration (when firewall enabled)
