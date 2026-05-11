---
name: grove-worktree-management
description: Use grove for all git worktree management when the project has a `.grove/` directory, when the user mentions worktrees, tmux, parallel work, PR review, Docker per branch, or when an agent needs to switch context without disrupting the user's current work. Always prefer this skill over manual `git worktree add` — grove handles naming, tmux, Docker, and hooks atomically. Use even when the user doesn't say "grove" by name.
---

# Grove Worktree Management

## When to Use Grove

Use grove instead of manual git worktree commands when:
- The project has a `.grove/` directory (already configured)
- The user needs Docker containers per worktree
- You need to review a PR without disrupting the user's current context
- Multiple features or PRs need parallel development environments
- The user mentions tmux, worktrees, or parallel development

If grove is not installed, refer to [`docs/AGENT_GUIDE.md`](../../docs/AGENT_GUIDE.md) for installation steps.

## Agent Mode Setup

Before any grove command:

```bash
export GROVE_AGENT_MODE=1        # suppress tmux takeover; implies NO_UPDATE_NOTIFIER
export GROVE_NONINTERACTIVE=1    # auto-accept all prompts
export GROVE_TUI=0               # disable dashboard
```

> `GROVE_AGENT_MODE=1` suppresses tmux in `grove to` but not session creation in
> `grove new`. To fully disable tmux, set `[tmux] mode = "off"` in config.

## Command Quick Reference

| Command | Purpose | `--json` | Mutates | Aliases |
|---------|---------|:--------:|:-------:|---------|
| `grove new [name]` | Create worktree + branch + Docker + tmux | — | ✓ | `spawn`, `n` |
| `grove to [name]` | Switch context; `--peek` skips hooks/tmux | — | ✓ | `switch`, `t` |
| `grove fetch <pr\|issue>/<n>` | Worktree from GitHub PR/issue (needs `gh`) | — | ✓ | `f` |
| `grove ls` | List all worktrees | ✓ | — | `list`, `l` |
| `grove here` | Current worktree status | ✓ | — | `h` |
| `grove context` | Rich status (ahead/behind, stash, recent commits) | ✓ | — | — |
| `grove test <name>` | Run tests in another worktree | — | — | `tt` |
| `grove ps` | List active isolated Docker slots | ✓ | — | `agent-status` |
| `grove up` | Start Docker; `--isolated [--slot N]` for agent stacks | — | ✓ | `u` |
| `grove down` | Stop Docker; `--slot N` for isolated stacks | — | ✓ | — |
| `grove rm [name]` | Remove worktree + branch + tmux + Docker | — | ✓ | `remove`, `delete` |
| `grove doctor [worktree]` | Health check | — | — | — |
| `grove adopt [path]` | Bring raw git worktree under grove management | — | ✓ | — |
| `grove sync [name]` | Sync branch with upstream | — | ✓ | `s` |
| `grove last` | Switch to previous worktree | — | ✓ | `la` |
| `grove diff [name]` | Compare branches | — | — | `compare`, `d` |
| `grove graft <name>` | Apply changes from another worktree | — | ✓ | `apply`, `g` |
| `grove trim` | Remove stale/merged worktrees | — | ✓ | `prune`, `clean`, `tm` |
| `grove join [name]` | Attach to tmux session | — | — | `attach`, `a`, `j` |

## Critical Rules

- **Worktree naming:** `{project}-{name}` — enforced. `grove ls` shows short names; tmux uses full names.
- **Read-only switching:** `grove to <name> --peek` — skips hooks and tmux. Safe for PR review.
- **Shell directives:** Without `GROVE_SHELL=1`, grove emits `cd:`, `tmux-attach:`, `env:` lines raw. Filter them: `grove to x 2>&1 | grep -vE '^(cd:|tmux-attach(-cc)?:|env:)'`
- **`grove new` and tmux:** `GROVE_AGENT_MODE=1` does NOT suppress tmux session creation in `grove new`. Set `[tmux] mode = "off"` in config to prevent this.
- **Trust:** `.grove/hooks.toml` runs as `sh -c` with full env. Run `grove doctor` and check hooks before `grove new`/`grove fetch` in an unfamiliar repo.

## Deterministic Helpers

For common operations where getting the logic right matters, run these Python scripts:

| Script | Purpose | Invocation |
|--------|---------|-----------|
| `probe_state.py` | Normalized status from `grove here --json` + `grove ls --json` | `python skills/grove-worktree-management/scripts/probe_state.py` |
| `strip_directives.py` | Filter directive lines from grove stdout | `grove to x 2>&1 \| python skills/grove-worktree-management/scripts/strip_directives.py` |
| `allocate_slot.py` | Find lowest free isolated Docker slot | `python skills/grove-worktree-management/scripts/allocate_slot.py [--dry-run]` |
| `audit_hooks.py` | Summarize hooks that would run in this repo | `python skills/grove-worktree-management/scripts/audit_hooks.py` |
| `pr_review.py` | Orchestrate PR fetch + peek switch | `python skills/grove-worktree-management/scripts/pr_review.py <PR#> [--dry-run]` |

All scripts: Python 3 stdlib only, `--help` flag, JSON to stdout on success.

## Deeper Context

For topics that need more than a quick reference, read these files on demand:

- **Shell directive protocol** — `references/shell-directives.md`
- **Multi-agent isolated Docker slots** — `references/isolated-slots.md`
- **Trust posture for unfamiliar repos** — `references/trust-model.md`
- **Common workflow recipes** — `references/workflows.md`
- **Full command surface with all aliases** — `references/commands.md`
