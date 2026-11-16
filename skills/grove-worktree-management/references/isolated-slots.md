# Isolated Docker Slots

Read this file when running multiple agents in the same repository simultaneously, when you need each agent to have its own Docker stack with unique ports, or when `grove ps` output needs interpretation.

## What Isolated Slots Are

Isolated slots are separate Docker Compose stacks, one per agent session, running in parallel within the same repository. Each slot gets a unique port offset so services don't conflict. This lets N agents each run their own database, web server, and worker without colliding.

Slots are numbered from 1. The main (non-isolated) stack is not a slot (slot 0 means "none").

In agent mode (`GROVE_AGENT_MODE=1` with agent stacks enabled), `grove to` does not repoint the shared stack. A worktree without a slot gets an `agent mode:` notice on stderr, so start an isolated stack for it with `grove up --isolated`.

## Starting an Isolated Stack

**Allocate the next free slot automatically:**

```bash
grove up --isolated
```

Grove picks the lowest free slot, starts the stack, and prints the slot number. There is no flag to request a specific slot.

## Stopping an Isolated Stack

```bash
grove down
```

Run it from the worktree. Grove detects that the worktree has an isolated stack and stops that one; with no slot it stops the shared stack.

## Inspecting Active Slots

```bash
grove ps --json
```

Output schema (one object per active slot):

```json
[
  {
    "slot": 1,
    "worktree": "project-feature",
    "compose_project": "project-agent-1",
    "url": "http://localhost:3100"
  },
  {
    "slot": 2,
    "worktree": "project-other",
    "compose_project": "project-agent-2",
    "url": "http://localhost:3200"
  }
]
```

- `slot` — 1-based slot number
- `worktree` — full worktree name (`{project}-{name}`)
- `compose_project` — Docker Compose project name (`{project}-agent-{N}`)
- `url` — service URL if configured; omitted if not applicable

## Configuration

Cap the number of concurrent isolated slots in `.grove/config.toml`:

```toml
[plugins.docker.external.agent]
max_slots = 4
```

Attempting `grove up --isolated` when all slots are occupied returns an error.

## Port Offset Scheme

Each slot receives a unique port offset applied to all services in the stack. The exact offset values and base ports are repo-specific — see [`docs/AGENT_GUIDE.md`](https://github.com/lost-in-the/grove/blob/main/docs/AGENT_GUIDE.md) §7 for the complete port allocation pattern and how to read the offset for a given slot.

## Recommended Agent Setup Block

```bash
export GROVE_AGENT_MODE=1
export GROVE_NONINTERACTIVE=1
export GROVE_TUI=0

# Start the isolated stack (grove allocates the slot)
grove up --isolated

# ... do work ...

# Tear down when done
grove down
```

## Further Reading

See [`docs/AGENT_GUIDE.md`](https://github.com/lost-in-the/grove/blob/main/docs/AGENT_GUIDE.md) §7 for the complete multi-agent Docker pattern, including port offset tables and coordination strategies for agent teams.
