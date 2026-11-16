# Grove Command Reference

Read this file when you need the complete command surface including rarely-used commands and all aliases. SKILL.md has a quick reference for day-to-day commands; this file has the full picture.

## Full Command Table

| Canonical | Aliases | `--json` | Mutates | Description |
|-----------|---------|:--------:|:-------:|-------------|
| `grove new [name]` | `spawn`, `n` | — | ✓ | Create worktree + branch + tmux session; `--up` also brings the stack |
| `grove to [name]` | `switch`, `t` | — | ✓ | Move to a worktree; does not touch the stack by default (`[switch] container_switch` set to `auto` or `prompt` changes that). `--up` also brings it, `--peek` skips hooks and tmux |
| `grove fetch <ref>` | `f` | — | ✓ | Create worktree from PR (`pr/N`) or issue (`issue/N`); requires `gh` CLI |
| `grove ls` | `list`, `l` | ✓ | — | List all worktrees with status, branch, tmux, containers |
| `grove here` | `h` | ✓ | — | Current worktree status (name, branch, SHA, changes, tmux, agent slot) |
| `grove context` | — | ✓ | — | Rich status: ahead/behind, stash count, recent commits, tracking branch |
| `grove test <name>` | `tt` | — | — | Run tests in another worktree; args after `--` are passed to test runner |
| `grove ps` | `agent-status` | ✓ | — | List active isolated Docker slots (slot, worktree, compose project, URL) |
| `grove up` | `u` | — | ✓ | Bring the stack to the worktree you are in; `--isolated` for an agent stack (slot auto-allocated) |
| `grove down` | `do` | — | ✓ | Stop Docker stack; detects and stops this worktree's isolated stack |
| `grove rm [name]` | `remove`, `delete` | — | ✓ | Remove worktree, branch, tmux session, and Docker stack |
| `grove doctor [worktree]` | — | — | — | Health check; without arg checks current worktree |
| `grove adopt [path]` | — | — | ✓ | Bring a raw `git worktree add` worktree under grove management |
| `grove sync [name]` | `s` | — | ✓ | Sync branch with upstream; without arg syncs current worktree |
| `grove last` | `la` | — | ✓ | Switch to the previous worktree (toggles) |
| `grove diff [name]` | `compare`, `d` | — | — | Compare current branch against another worktree's branch |
| `grove graft <name>` | `apply`, `g` | — | ✓ | Apply uncommitted changes from another worktree into current |
| `grove trim` | `prune`, `clean`, `tm` | — | ✓ | Remove stale or merged worktrees; prompts by default |
| `grove join [name]` | `attach`, `a`, `j` | — | — | Attach to the tmux session for a worktree |
| `grove fork [name]` | `split`, `fo` | — | ✓ | Fork current worktree into a new one (branch + copy uncommitted state) |
| `grove open [name]` | `o` | ✓ | ✓ | Open a worktree session, creating the worktree/tmux session if needed |
| `grove logs [service]` | `lo` | — | — | Tail Docker logs for a service in the current worktree's stack; defaults to all services |
| `grove kick [service]` | `restart`, `k` | — | ✓ | Restart a Docker service in the current worktree's stack |
| `grove rename [old] [new]` | — | — | ✓ | Rename a worktree and update tmux session, branch, Docker project |
| `grove prs` | — | — | — | Browse open pull requests; opens in `gh` or browser |
| `grove issues` | — | — | — | Browse open issues; opens in `gh` or browser |
| `grove browse` | `b` | — | — | Open the current worktree's PR or issue in the browser |
| `grove which` | `status` | ✓ | — | Show where you are and the state of the stack |
| `grove config` | — | — | ✓ | Show or edit grove configuration (`.grove/config.toml`) |
| `grove init` | — | — | ✓ | Initialize the current git repository as a grove project |
| `grove install [shell]` | — | — | — | Print shell integration code for zsh or bash |
| `grove setup` | — | — | ✓ | Interactively detect shell and install shell integration |
| `grove repair` | — | — | ✓ | Detect and repair inconsistencies between grove state and worktrees |
| `grove agent-help` | — | — | — | Print a quick reference for AI agents using grove |
| `grove version` | — | — | — | Print grove's version and build information |

## Notes

**Ref format for `grove fetch`:** `pr/42` or `issue/17`. The `gh` CLI must be installed and authenticated.

**`grove new` flags:**
- `--from-branch <branch>` — create worktree tracking an existing branch instead of creating a new one
- `--dirty` — preserve uncommitted changes when creating the worktree

**`grove to` flags:**
- `--up` (`-u`) — also bring the stack to the worktree you move to. Same flag on `grove last` and `grove new`. Without it, `grove to` never touches Docker (unless `[switch] container_switch` is `auto` or `prompt`).
- `--peek` — read-only move: skips pre/post_switch hooks and does not attach tmux. Safe for PR review and parallel inspection.

**`grove trim` flags:**
- `--all` — skip confirmation prompts and remove all stale worktrees at once. Use carefully.

**`grove up` flags:**
- `--isolated` — start a separate Docker stack for this agent session with unique port offsets (slot is allocated automatically, numbered from 1)

**`grove down`:** no slot flag; it detects an isolated stack for the current worktree.

**JSON output:** Commands with `--json` emit structured JSON to stdout. Pipe to `jq` or use the `probe_state.py` helper to get a normalized view across multiple commands.

**Aliases are interchangeable:** `grove list`, `grove ls`, and `grove l` behave identically. Canonical names are used in documentation for clarity.

**Mutating commands** change filesystem, git refs, tmux sessions, or Docker state. Non-mutating commands are safe to run at any time without side effects.
