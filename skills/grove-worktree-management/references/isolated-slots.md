# Isolated Docker Slots

Read this file when running multiple agents in the same repository simultaneously, when you need each agent to have its own Docker stack with unique ports, or when `grove ps` output needs interpretation.

## What Isolated Slots Are

Isolated slots are separate Docker Compose stacks, one per agent session, running in parallel within the same repository. Each slot gets a unique port offset so services don't conflict. This lets N agents each run their own database, web server, and worker without colliding.

Slots are numbered from 0. The main (non-isolated) stack is not counted as a slot.

## Starting an Isolated Stack

**Allocate the next free slot automatically:**

```bash
grove up --isolated
```

Grove finds the lowest free slot via `grove ps --json`, starts the stack, and prints the slot number.

**Request a specific slot:**

```bash
grove up --isolated --slot 2
```

Use this when you need a deterministic slot (e.g., pre-configured port mappings). Fails if the slot is already occupied.

## Stopping an Isolated Stack

```bash
grove down --slot 2
```

Omitting `--slot` stops the main (non-isolated) stack for the current worktree.

## Inspecting Active Slots

```bash
grove ps --json
```

Output schema (one object per active slot):

```json
[
  {
    "slot": 0,
    "worktree": "project-feature",
    "compose_project": "project-feature-slot-0",
    "url": "http://localhost:3100"
  },
  {
    "slot": 1,
    "worktree": "project-other",
    "compose_project": "project-other-slot-1",
    "url": "http://localhost:3200"
  }
]
```

- `slot` — 0-based slot index
- `worktree` — full worktree name (`{project}-{name}`)
- `compose_project` — Docker Compose project name (`{project}-{worktree}-slot-{N}`)
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

## Safe Slot Selection with the Helper

Use `allocate_slot.py` to find the next free slot without actually starting Docker:

```bash
python3 "${CLAUDE_PLUGIN_ROOT:-skills/grove-worktree-management}/scripts/allocate_slot.py" --dry-run
# {"slot": 2, "available": true}
```

Without `--dry-run`, the script returns the slot number and exits — suitable for use in a setup script before calling `grove up --isolated --slot N`.

## Recommended Agent Setup Block

```bash
export GROVE_AGENT_MODE=1
export GROVE_NONINTERACTIVE=1
export GROVE_TUI=0

# Find the next free slot
SLOT=$(python3 "${CLAUDE_PLUGIN_ROOT:-skills/grove-worktree-management}/scripts/allocate_slot.py")

# Start the isolated stack
grove up --isolated --slot "$SLOT"

# ... do work ...

# Tear down when done
grove down --slot "$SLOT"
```

## Further Reading

See [`docs/AGENT_GUIDE.md`](https://github.com/lost-in-the/grove/blob/main/docs/AGENT_GUIDE.md) §7 for the complete multi-agent Docker pattern, including port offset tables and coordination strategies for agent teams.
