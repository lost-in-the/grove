# Grove Command Reference

Read this file when you need the complete command surface including rarely-used commands and all aliases. SKILL.md has a quick reference for day-to-day commands; this file has the full picture.

## Full Command Table

| Canonical | Aliases | `--json` | Mutates | Description |
|-----------|---------|:--------:|:-------:|-------------|
| `grove new [name]` | `spawn`, `n` | ✓ | ✓ | Create worktree + branch + its session; `--up` also brings the stack |
| `grove to [name]` | `switch`, `t` | ✓ | ✓ | Move to a worktree; does not touch the stack by default (`[switch] container_switch` set to `auto` or `prompt` changes that). `--up` also brings it, `--here` stays in this shell, `--no-hooks` skips hooks, `--peek` is short for `--here --no-hooks` |
| `grove fetch <ref>` | `f` | ✓ | ✓ | Create worktree from PR (`pr/N`) or issue (`issue/N`); requires `gh` CLI |
| `grove ls` | `list`, `l` | ✓ | — | List all worktrees with status, branch, session, containers |
| `grove here` | `h` | ✓ | — | Current worktree status (name, branch, SHA, changes, session, agent slot) |
| `grove context` | — | ✓ | — | Rich status: ahead/behind, stash count, recent commits, tracking branch |
| `grove test <name> [args...]` | `tt` | — | — | Run the configured test command in another worktree; extra args are appended to it |
| `grove ps` | `agent-status` | ✓ | — | List active isolated Docker slots (slot, worktree, compose project, URL); `[]` when there are none |
| `grove up` | `u` | — | ✓ | Bring the stack to the worktree you are in; `--isolated` for an agent stack (slot auto-allocated) |
| `grove down` | `do` | — | ✓ | Stop the stack; detects and stops this worktree's isolated stack |
| `grove rm [name]` | `remove`, `delete` | — | ✓ | Remove worktree, branch, session, and Docker stack |
| `grove doctor [worktree]` | — | — | — | Health check; without arg checks current worktree |
| `grove adopt [path]` | — | — | ✓ | Bring a raw `git worktree add` worktree under grove management |
| `grove sync [name]` | `s` | ✓ | ✓ | Fast-forward an environment worktree (made with `grove new --mirror`) to its remote branch. Without a name it syncs the current worktree, and exits 8 if that is not an environment worktree. `--all` syncs every environment worktree |
| `grove last` | `la` | ✓ | ✓ | Move to the previous worktree (toggles) |
| `grove diff [name]` | `compare`, `d` | ✓ | — | Compare the current worktree against another (commits and uncommitted changes) |
| `grove graft <name>` | `apply`, `g` | ✓ | ✓ | Apply commits and uncommitted changes from another worktree into the current one |
| `grove trim` | `prune`, `clean`, `tm` | — | ✓ | Remove worktrees not accessed in 30 days; always asks you to type `yes` |
| `grove join [name]` | `attach`, `a`, `j` | ✓ | — | Join the session for a worktree without changing directory |
| `grove fork [name]` | `split`, `fo` | ✓ | ✓ | Fork current worktree into a new one (new branch; can move or copy uncommitted changes) |
| `grove open [name]` | `o` | ✓ | ✓ | Open a worktree session, creating the worktree and session if needed |
| `grove logs [service]` | `lo` | — | — | Tail Docker logs for a service in the current worktree's stack; defaults to all services |
| `grove kick [services...]` | `restart`, `k` | — | ✓ | Restart services in the current worktree's stack; all services when none are named |
| `grove rename [old] [new]` | — | — | ✓ | Rename a worktree: its directory, state entry, and session |
| `grove prs` | — | — | ✓ | Browse pull requests in the TUI (`--fzf` for fzf); selecting one creates a worktree |
| `grove issues` | — | — | ✓ | Browse issues in the TUI (`--fzf` for fzf); selecting one creates a worktree |
| `grove browse` | `b` | ✓ | — | Open the current worktree's PR or issue in the browser; `--json` prints the URL only |
| `grove which` | `status` | ✓ | — | Show where you are and the state of the stack |
| `grove config` | — | — | ✓ | Show or edit grove configuration (`.grove/config.toml`) |
| `grove init` | — | — | ✓ | Initialize the current git repository as a grove project |
| `grove install [shell]` | — | — | — | Print shell integration code for zsh or bash |
| `grove setup` | — | — | ✓ | Interactively detect shell and install shell integration |
| `grove repair` | — | — | ✓ | Detect and repair inconsistencies between grove state and worktrees |
| `grove agent-help` | — | — | — | Print a quick reference for AI agents using grove |
| `grove version` | — | — | — | Print grove's version and build information; `grove --version` prints the same line |

## Flags by Command

Taken from `--help` of the built binary. `-h/--help` and the two global flags (`--check-update`, `--no-update-notifier`) are left out.

| Command | Flags |
|---------|-------|
| `new` | `-b/--branch`, `-f/--from`, `--from-branch`, `--dirty`, `--mirror`, `--here`, `-j/--json`, `-n/--no-switch`, `-u/--up`, `--no-docker` |
| `fork` | `--branch-name`, `--move-wip`, `--copy-wip`, `--no-wip`, `--here`, `-j/--json`, `-n/--no-switch`, `-u/--up`, `--no-docker` |
| `fetch` | `--here`, `-j/--json`, `-n/--no-switch`, `-u/--up`, `--no-docker` |
| `open` | `--command`, `--no-create`, `--no-popup`, `--here`, `-j/--json`, `-u/--up`, `--no-docker` |
| `to`, `last` | `--here`, `--no-hooks`, `--peek`, `-j/--json`, `-u/--up` |
| `ls` | `-j/--json`, `-p/--paths`, `-q/--quiet` |
| `here` | `-j/--json`, `-q/--quiet`, `--check-mount`, `--require-current` |
| `context`, `which`, `join`, `ps`, `browse` | `-j/--json` |
| `diff` | `--committed`, `--wip`, `--stat`, `-j/--json` |
| `graft` | `--commits`, `--wip`, `--pick`, `--dry-run`, `-j/--json` |
| `sync` | `--all`, `-j/--json` |
| `rm` | `-f/--force`, `--unprotect`, `--dry-run`, `--keep-branch`, `--delete-branch` |
| `trim` | `--older-than`, `--include-dirty`, `--dry-run`, `--keep-branches`, `--delete-branches` |
| `up` | `-d/--detach`, `--isolated` |
| `logs` | `-f/--follow` |
| `test` | `--with-deps`, `--bind` |
| `doctor` | `--all`, `--fix` |
| `repair` | `--dry-run` |
| `prs`, `issues` | `--state`, `--label`, `--assignee`, `--author`, `--limit`, `--fzf` |
| `config` | `-e/--edit`, `-g/--global`, `--hooks` |
| `init` | `--with-testing`, `--with-scratch`, `--full`, `--no-hooks`, `--auto`, `--walkthrough`, `--yes` |
| `install`, `setup` | `--alias` |
| `version` | `-v/--verbose` |
| `adopt`, `down`, `kick`, `rename`, `agent-help` | none |

## Notes

**Ref format for `grove fetch`:** `pr/42` or `issue/17`. The `gh` CLI must be installed and authenticated.

**`grove new` flags:**
- `--from-branch <branch>` — create worktree tracking an existing branch instead of creating a new one
- `--dirty` — preserve uncommitted changes when creating the worktree

**`grove to` flags:**
- `--up` (`-u`) — also bring the stack to the worktree you move to. Same flag on `grove last` and `grove new`. Without it, `grove to` never touches Docker (unless `[switch] container_switch` is `auto` or `prompt`).
- `--here`: placement "here" for this run: stay in the shell or pane you are in, change directory only. Hooks still run. Also on `last`, `new`, `fork`, `open`, `fetch`.
- `--no-hooks`: skip pre/post_switch hooks and the dirty check. Cannot combine with `--up`.
- `--peek`: short for `--here --no-hooks`. Safe for PR review and parallel inspection.

**`grove trim`:**
- It selects worktrees by last access age, not by merged branches. The default is 30 days: change it with `--older-than N`.
- It always asks you to type `yes`. No flag skips that. A script can pipe it: `echo yes | grove trim`.
- `--dry-run` lists what would be removed and changes nothing.
- The main, current, protected, and environment worktrees are never removed. Dirty worktrees need `--include-dirty`.

**Create flags (`new`, `fork`, `fetch`, `open`):**
- `--up` (`-u`): also bring the stack to the worktree. `--no-docker`: do not bring it, even with `auto_up`. The two cannot be combined.
- `--no-switch` (`-n`) on `new`, `fork`, `fetch`: stay where you are.
- `--json` (`-j`): print the result object. `switch_to` holds the path, and is left out with `--no-switch`.

**`grove up` flags:**
- `--isolated` — start a separate Docker stack for this agent session with unique port offsets (slot is allocated automatically, numbered from 1)

**`grove down`:** no slot flag; it detects an isolated stack for the current worktree.

**JSON output:** Commands with `--json` emit structured JSON to stdout. Pipe to `jq` or use the `probe_state.py` helper to get a normalized view across multiple commands. `--json` always has the short form `-j`.

**JSON errors:** when a command run with `--json` fails, stderr holds one object: `{"error": true, "code": <exit code>, "message": "..."}`. `sync` and `graft` also print their result document on stdout.

**Aliases are interchangeable:** `grove list`, `grove ls`, and `grove l` behave identically. Canonical names are used in documentation for clarity.

**Mutating commands** change filesystem, git refs, sessions, or Docker state. Non-mutating commands are safe to run at any time without side effects.
