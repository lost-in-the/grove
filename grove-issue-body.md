## Summary

Add a `plugins/claude` plugin and `grove sandbox` command group to give Claude Code (and other AI agents) first-class, sandboxed devcontainer support — analogous to how the Docker plugin manages container lifecycle today.

Grove already has strong agent primitives (`GROVE_AGENT_MODE`, `grove up --isolated`, `grove agent-status`). This proposal extends that foundation with network-isolated devcontainers per worktree so that `--dangerously-skip-permissions` is safe to use by default, without any manual devcontainer wiring.

-----

## Background

The [Anthropic devcontainer reference](https://code.claude.com/docs/en/devcontainer) recommends running Claude Code inside a devcontainer with a firewall script (`init-firewall.sh`) that restricts outbound connections to a trusted allowlist. This makes `--dangerously-skip-permissions` safe for unattended agent runs. Today, wiring this up with Grove requires manual devcontainer scaffolding and session config — there's no first-class integration.

The existing isolated stack machinery (`grove up --isolated`, slot-based port offsets, `grove agent-status`) is a direct precedent and model for this feature.

-----

## Proposed Changes

### 1. `plugins/claude` — Claude Code Plugin

A new plugin following the existing `plugins.Plugin` interface (`Name() / Init() / RegisterHooks() / Enabled()`).

**Responsibilities:**

- On `post_create`: scaffold a `.devcontainer/devcontainer.json` scoped to the worktree, using the Anthropic reference devcontainer feature (`ghcr.io/anthropics/devcontainer-features/claude-code:1.0`)
- Optionally inject `init-firewall.sh` with a configurable domain allowlist
- On `grove open` / `GROVE_AGENT_MODE=1`: start Claude Code inside the devcontainer automatically
- On `grove rm`: stop and remove the scoped devcontainer

**Config extension in `.grove/config.toml`:**

```toml
[plugins.claude]
enabled = true
auto_start = false          # start Claude on grove open?
skip_permissions = true     # pass --dangerously-skip-permissions inside devcontainer
prompt = ""                 # optional headless prompt for CI / GROVE_AGENT_MODE

[plugins.claude.devcontainer]
enabled = true              # wrap Claude in a devcontainer for sandboxing
firewall = true             # inject init-firewall.sh
allowed_domains = [         # extend the default allowlist (npm, GitHub, Anthropic API already included)
  "rubygems.org",
  "bundler.io",
  "index.rubygems.org"
]
```

**Design note:** Grove should *compose* the Anthropic reference devcontainer feature rather than owning the Dockerfile or firewall script. This keeps Grove's surface area small and ensures users automatically benefit from upstream security improvements.

-----

### 2. `grove sandbox` — Devcontainer Lifecycle Commands

New command group for explicit sandbox management, parallel to `grove up/down/logs` for Docker.

|Command                                 |Behavior                                     |
|----------------------------------------|---------------------------------------------|
|`grove sandbox new <worktree>`          |Build devcontainer for a worktree            |
|`grove sandbox start <worktree>`        |Start it (detached)                          |
|`grove sandbox stop <worktree>`         |Stop it                                      |
|`grove sandbox status`                  |Table of running sandboxes, ports, Claude PID|
|`grove sandbox status --json`           |Machine-readable output for CI/scripts       |
|`grove sandbox exec <worktree> -- <cmd>`|Run a command inside the sandbox             |
|`grove sandbox rm <worktree>`           |Stop and remove sandbox + volumes            |

This follows the same UX pattern as `grove agent-status` and `grove up --isolated`. The `grove sandbox status` output should be consumable by other agents (same `--json` convention as `grove agent-status --json`).

-----

### 3. `grove doctor` Extension

Extend the existing `grove doctor` health check to cover the Claude Code / devcontainer stack:

- Node.js / npm present (Claude Code requirement)
- `@anthropic-ai/claude-code` available in PATH or devcontainer
- `ANTHROPIC_API_KEY` set in environment or shell config
- devcontainer CLI (`devcontainer`) available
- Firewall script present and executable (when `firewall = true`)

Example output:

```
✓ Grove binary (/opt/homebrew/bin/grove)
✓ Shell integration (v3, current)
✓ Git (2.43.0)
✓ Tmux (3.4)
✓ Docker running (v27.1.1)
✓ Config (loaded)
  Claude Code plugin
  ✓ Node.js (v22.1.0)
  ✓ claude-code (1.x.x)
  ✓ ANTHROPIC_API_KEY (set)
  ✓ devcontainer CLI (0.x.x)
  ✓ Firewall script (present, executable)
```

-----

## What This Is NOT

- Grove should not own or reimplement the firewall script — scaffold it from the Anthropic reference devcontainer, don't vendor it
- This is not a replacement for `grove up --isolated` — Docker isolated stacks and devcontainer sandboxes serve different purposes and can coexist (Claude inside a devcontainer, talking to an isolated Docker stack)
- No new tmux semantics needed — `GROVE_AGENT_MODE=1` already suppresses attachment; the sandbox plugin just adds a devcontainer wrapper around the Claude process

-----

## Rough Implementation Path

1. Add `plugins/claude/` directory with plugin skeleton (mirrors `plugins/docker/` structure)
1. Implement `post_create` hook: detect devcontainer CLI, scaffold `.devcontainer/` into worktree
1. Add `grove sandbox` command group in `cmd/`
1. Extend `grove doctor` checks
1. Add `grove init` detection: if project type is Rails/Node/Go, suggest enabling the Claude plugin
1. Docs: `docs/CLAUDE_PLUGIN.md` and update `docs/AGENT_GUIDE.md`

-----

## References

- [Anthropic devcontainer reference](https://code.claude.com/docs/en/devcontainer)
- [Anthropic devcontainer feature (`ghcr.io/anthropics/devcontainer-features/claude-code`)](https://github.com/anthropics/devcontainer-features)
- [trailofbits/claude-code-devcontainer](https://github.com/trailofbits/claude-code-devcontainer) — community sandbox implementation worth reviewing
- Grove's existing isolated stack docs: `plugins/docker/README.md` § Isolated Stack Mode
- Grove's existing agent docs: `docs/AGENT_GUIDE.md`
