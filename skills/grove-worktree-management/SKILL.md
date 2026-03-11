---
name: grove-worktree-management
description: Use grove for git worktree management with tmux, Docker, and GitHub integration. Replaces manual worktree + Docker + tmux orchestration with single commands.
---

# Grove Worktree Management

## When to Use Grove

Use grove instead of manual git worktree commands when:
- The project has a .grove/ directory (already configured)
- You need Docker containers per worktree
- You're working on multiple features/PRs simultaneously
- You need to fork, compare, or apply changes between branches
- The user mentions tmux, worktrees, or parallel development

## Quick Reference

| Task | Command |
|------|---------|
| Create worktree | grove new <name> |
| Switch worktree | grove to <name> |
| Switch back | grove last |
| Create from PR | grove fetch pr/<number> |
| Create from issue | grove fetch issue/<number> |
| Fork current work | grove fork <name> [--move-wip] |
| Apply from another | grove apply <name> [--wip] |
| Compare branches | grove compare <name> |
| Run tests elsewhere | grove test <name> [args] |
| Start Docker | grove up [--isolated] |
| Stop Docker | grove down |
| Cleanup stale | grove clean |
| Health check | grove doctor |

## Critical Rules

- Shell integration MUST be active (GROVE_SHELL=1). If grove prints cd: lines instead of changing directories, shell integration isn't loaded.
- Worktree names follow {project}-{name}. Don't create worktrees manually — grove manages naming, tmux, and hooks.
- grove to can attach tmux and take over the terminal. Agents should set [tmux] mode = "manual" or "off" in .grove/config.toml, or use grove to --peek to skip tmux/hooks entirely.
- Use grove to --peek for read-only switching (skips Docker hooks and tmux).
- For parallel agents, use grove up --isolated — each gets unique ports.
- Config lives in .grove/config.toml (project) and ~/.config/grove/config.toml (global).
- For external Docker with env_file = ".env.local": set up mise or direnv in the compose directory so manual docker compose commands see the worktree path.

## Hooks (.grove/hooks.toml)

Configure lifecycle hooks in `.grove/hooks.toml`:

| Event | Common use |
|-------|-----------|
| `post_create` | Copy .env, symlink node_modules, bundle install |
| `post_switch` | git pull, run migrations, rebuild assets |
| `pre_switch` | Stop background processes |
| `pre_remove` | Docker compose stop |

```toml
# Example: run migrations after every switch
[[hooks.post_switch]]
type        = "command"
command     = "bin/rails db:migrate"
working_dir = "new"
on_failure  = "warn"
```

## Docker Modes

- **local**: Each worktree has its own docker-compose.yml
- **external**: Shared compose dir, grove injects APP_DIR env var per worktree
- **agent**: Isolated stacks with auto port offsets for parallel work

## Workflow: Multi-Agent Development

1. Each agent: grove new <task-name> or grove fetch pr/<N>
2. Each agent: grove up --isolated (own Docker stack)
3. Share work: grove apply <other-worktree> --wip
4. Cleanup: grove rm <name> (removes worktree + branch + tmux + Docker)
