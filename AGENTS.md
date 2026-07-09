# AGENTS.md — Using Grove from an AI Agent

> **Scope:** This file is for agents whose user has grove installed and is asking them to
> invoke it. If you are developing grove itself, see [`CONTRIBUTING.md`](CONTRIBUTING.md)
> and [`CLAUDE.md`](CLAUDE.md) for codebase rules.

Grove is a Go CLI that manages git worktrees with tmux + Docker integration. Every
command completes in <500ms. Always prefer `--json` flags — all status output is
machine-parseable.

---

## TL;DR

- Export the env block below before any grove invocation
- Always use `--json` for status queries; never scrape human-readable output
- Worktree directories default to `{project}-{name}` (e.g., `myapp-pr-42`); projects can override via `[naming] pattern` — check `.grove/config.toml`
- `grove ls` and `grove here` show **short** names; tmux sessions use **full** names
- Read the **Trust model** section before `grove new`/`grove fetch` in an unfamiliar repo
- `skills/grove-worktree-management/` has deterministic Python helpers for common tasks

---

## Agent Mode Setup (REQUIRED)

```bash
export GROVE_AGENT_MODE=1        # suppress tmux takeover; implies NO_UPDATE_NOTIFIER
export GROVE_NONINTERACTIVE=1    # auto-accept all prompts
export GROVE_TUI=0               # disable dashboard; bare `grove` prints usage
```

> `GROVE_AGENT_MODE=1` suppresses tmux attachment in `grove to` but **not** session
> creation in `grove new` — and it also forces the isolated Docker strategy. For
> per-invocation tmux suppression without the Docker coupling, pass `--no-tmux` to
> `grove to` / `grove new` (on `new` it also skips session creation). To fully
> disable tmux, set `[tmux] mode = "off"` in `.grove/config.toml`.

---

## Commands You'll Actually Call

| Command | Purpose | `--json` | Mutates |
|---------|---------|:--------:|:-------:|
| `grove new [name]` | Create worktree + branch + Docker + tmux (aliases: `spawn`, `n`) | — | ✓ |
| `grove to [name]` | Switch context atomically; `--peek` skips hooks/tmux, `--no-tmux` skips tmux only (aliases: `switch`, `t`) | — | ✓ |
| `grove fetch <pr\|issue>/<n>` | Create worktree from GitHub PR/issue; needs `gh` (aliases: `f`) | — | ✓ |
| `grove ls` | List all worktrees in this project (aliases: `list`, `l`) | ✓ | — |
| `grove here` | Current worktree status (aliases: `h`) | ✓ | — |
| `grove context` | Rich status: ahead/behind, stash count, recent commits | ✓ | — |
| `grove test <name>` | Run configured tests in another worktree without switching (aliases: `tt`) | — | — |
| `grove ps` | List active isolated Docker slots (aliases: `agent-status`) | ✓ | — |
| `grove up` | Start Docker stack; `--isolated` for per-agent stacks (aliases: `u`) | — | ✓ |
| `grove down` | Stop Docker stack; isolated stacks are auto-detected | — | ✓ |
| `grove rm [name]` | Remove worktree + branch + tmux + Docker (aliases: `remove`, `delete`) | — | ✓ |
| `grove doctor [worktree]` | Health check; inspect output before `grove new` in unknown repos | — | — |

For the full flag and output specification of any command, see
[`docs/COMMAND_SPECIFICATIONS.md`](docs/COMMAND_SPECIFICATIONS.md).

---

## Reading Grove Output

### Prefer `--json`

`grove here --json` returns: `name`, `full_name`, `project`, `branch`, `path`,
`commit.sha`, `commit.message`, `status`, `changes[]`, `tmux.session`, `environment`,
`agent_slot`, `agent_url`.

`grove context --json` adds: `tracking_branch`, `has_remote`, `ahead`, `behind`,
`stash_count`, `recent_commits[]` — prefer it when you need richer repo state.

### Shell-Integration Directive Lines

When grove runs under the user's shell wrapper (`GROVE_SHELL=1`), it emits special
lines on stdout that the wrapper intercepts and acts on. When you shell out to grove
**without** the wrapper active, these lines appear raw in stdout — filter them:

```bash
grove to feature 2>&1 | grep -vE '^(cd:|tmux-attach(-cc)?:|env:)'
```

Directives: `cd:/abs/path` (directory change), `tmux-attach:<session>` (attach tmux),
`tmux-attach-cc:<session>` (iTerm2 control mode), `env:KEY=VALUE` (export variable).
See [`docs/SHELL_INTEGRATION.md`](docs/SHELL_INTEGRATION.md) for the full protocol.

---

## Trust Model

`.grove/hooks.toml` defines hooks that run via `sh -c` with the **full user environment**
forwarded — including credentials and API tokens in your env. `grove new`, `grove to`,
and `grove fetch` will execute these hooks without further confirmation.

**In an unfamiliar repo:** run `grove doctor` and inspect `.grove/hooks.toml` before
allowing any worktree creation or switching. Do not assume a repo's hooks are safe.

---

## Common Workflows

### Review a GitHub PR non-destructively

```bash
grove fetch pr/42          # creates worktree pr-42-<title-slug> (needs `gh`)
                           # fetch prints the generated name — use that, don't guess it
grove to pr-42-fix-login-bug --peek # cd only — skips hooks and tmux entirely
# read files, run grep, etc.
grove rm pr-42-fix-login-bug        # teardown
```

`grove fetch` names the worktree `pr-<N>-<title-slug>` (slug from the PR title). Take
the exact name from fetch's own output or `grove ls`, never hardcode it.

### New feature branch

`grove new auth` creates the worktree, branch, Docker stack, and tmux session atomically.
Clean up when done: `grove rm auth`.

### Run tests in another worktree without leaving current

`grove test cache-redesign` runs the configured test command there. Extra args are
appended verbatim (no `--` separator): `grove test cache-redesign -run TestFoo`.

### Cheap state probes

`grove here --json`, `grove ls --json`, and `grove context --json` all complete in
<500ms. Probe freely to understand repository state before acting.

---

## Multi-Agent Parallelism

For parallel agents on the same repository, each needs an isolated Docker stack with
unique port offsets:

```bash
grove ps --json                     # discover active slots: [{slot, worktree, compose_project, url}]
grove up --isolated                 # start an isolated stack (auto-allocates a slot)
# ... work in this worktree ...
grove down                          # release the stack (isolated stacks auto-detected)
```

The `[plugins.docker.external.agent] max_slots` config value caps the number of
concurrent slots. See [`docs/AGENT_GUIDE.md` §7](docs/AGENT_GUIDE.md#7-agent-strategy-guide)
for the full multi-agent workflow.

---

## Worktree Naming

Worktree directories follow the project's `[naming] pattern` (in `.grove/config.toml`),
default `{project}-{name}` — e.g., `myapp-auth`, `myapp-pr-42`. The pattern must contain
`{project}` and `{name}` exactly once each. The project name comes from project config,
then the git remote URL, then the directory name. `grove ls` shows the **short** name
(`auth`); tmux session names always use the canonical `{project}-{name}` form
(`myapp-auth`) regardless of the directory pattern.

## Drift + Adopt

Worktrees created by raw `git worktree add` instead of `grove new` trigger a non-fatal
drift warning on any grove command. Run `grove adopt [path]` to bring them under grove
management (idempotent — safe to run multiple times).

## PATH Gotcha (Non-Interactive Contexts)

The shell wrapper resolves the binary via `command -v grove`. Grove must be on `PATH`
in `~/.zshenv` (not `~/.zprofile`) — login-only exports are invisible to non-login
shells, CI pipelines, and tool invocations from editors like Claude Code.

## Config Layering

Defaults → `~/.config/grove/config.toml` → `.grove/config.toml` →
`.grove/config.local.toml` (gitignored, per-developer overlay) → env overrides.
`[protection]` lists are **unioned** across layers — branches are never silently dropped.
Full schema: [`docs/CONFIGURATION_REFERENCE.md`](docs/CONFIGURATION_REFERENCE.md).

## Plugins

**Docker plugin** — three modes: `local` (per-worktree compose file), `external`
(shared compose dir, `APP_DIR` env injection), `isolated` (per-slot port-offset stacks
for parallel agent work). [`plugins/docker/README.md`](plugins/docker/README.md).

**Tracker plugin** — GitHub PRs and issues via `gh` CLI. Powers `grove fetch`,
`grove prs`, and `grove issues`. [`plugins/tracker/README.md`](plugins/tracker/README.md).

---

## Where to Read More

- [`docs/AGENT_GUIDE.md`](docs/AGENT_GUIDE.md) — installation, workflows, Docker strategies, multi-agent guide
- [`docs/COMMAND_SPECIFICATIONS.md`](docs/COMMAND_SPECIFICATIONS.md) — every flag, exit code, and output format
- [`docs/CONFIGURATION_REFERENCE.md`](docs/CONFIGURATION_REFERENCE.md) — full TOML schema for all config files
- [`docs/SHELL_INTEGRATION.md`](docs/SHELL_INTEGRATION.md) — directive protocol and shell wrapper internals
- [`plugins/docker/README.md`](plugins/docker/README.md) — Docker plugin modes and config
- [`plugins/tracker/README.md`](plugins/tracker/README.md) — GitHub tracker plugin
- [`skills/grove-worktree-management/`](skills/grove-worktree-management/) — Claude skill with Python helpers. Installable as a Claude Code plugin: `/plugin marketplace add lost-in-the/grove` then `/plugin install grove-plugin@grove-plugins` (see [README → Claude Code plugin](README.md#claude-code-plugin)).

---

_If you find yourself adding more than ~10 lines to this file, you're probably writing
[`docs/AGENT_GUIDE.md`](docs/AGENT_GUIDE.md) content — put it there and link out._
