# Agent Prompts

This directory contains prompts for Claude Code agent-driven development.

## Available Prompts

### `tui-agent.md`

Prompt for implementing the TUI redesign according to `docs/TUI_IMPLEMENTATION_SPEC.md`.

**Features:**
- Enforces strict TDD (tests before implementation)
- One task per session
- Automatic checklist updates
- Structured completion signals

**Usage:**

```bash
# Single task (human-in-the-loop)
claude -p "$(cat prompts/tui-agent.md)"

# Autonomous loop
./scripts/tui-ralph.sh

# Dry run (show prompt without executing)
./scripts/tui-ralph.sh --dry-run
```

## Creating New Prompts

When creating prompts for agent development:

1. **Be specific** - Reference exact file paths and specs
2. **Enforce TDD** - Require tests before implementation
3. **One task focus** - Agents work better on focused tasks
4. **Completion signals** - Use `<promise>` tags for loop detection
5. **Update tracking** - Have agents update progress files

## Ralph Wiggum Pattern

The Ralph Wiggum pattern runs Claude in a loop, with each iteration:
1. Reading current progress
2. Picking a pending task
3. Implementing with TDD
4. Updating progress
5. Signaling completion

This allows autonomous progress through a task list while maintaining fresh context for each task.

See `scripts/tui-ralph.sh` for implementation.
