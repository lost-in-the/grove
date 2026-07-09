# Trust Model for Grove Hooks

Read this file before running `grove new`, `grove fetch`, or `grove to` in a repository you did not set up yourself, or when evaluating whether a grove command is safe to run autonomously.

## The Threat

`.grove/hooks.toml` configures shell commands that grove runs automatically at lifecycle events. These run via `sh -c` with your full user environment — including any credentials, API tokens, SSH keys, and cloud provider auth that are present in the shell. A malicious or misconfigured hook can exfiltrate secrets, modify files outside the worktree, or make network calls.

## Which Commands Run Hooks

| Command | Hooks that fire |
|---------|----------------|
| `grove new` | `post_create` |
| `grove fetch` | `post_create` |
| `grove to` | `pre_switch` (leaving), `post_switch` (entering) |
| `grove rm` | `pre_remove` |
| `grove up` | Docker plugin may fire `post_switch`-equivalent actions |
| `grove down` | Docker plugin may fire `pre_switch`-equivalent actions |

## Which Commands Are Safe (No Hooks)

These commands never run hooks and are safe to use in any repo without inspection:

- `grove ls` — reads git and tmux state only
- `grove here` — reads current worktree state only
- `grove context` — reads git history and tracking info only
- `grove ps` — reads Docker state only
- `grove doctor` — runs health checks, does not execute user-defined hooks

## Pre-Flight Checklist

Before running `grove new` or `grove fetch` in an unfamiliar repository:

1. **Run `grove doctor`** — outputs health status for the current worktree. Yellow warnings often indicate hook configuration that deserves inspection; red means something is broken.

2. **Read `.grove/hooks.toml` manually** — look at every `command` field. Watch for network calls (`curl`, `wget`), credential access (`op`, `vault`, `aws`), or writes outside the project directory.

3. **Read `.grove/config.toml`** — check `[plugins.docker]` action blocks for commands that run when Docker starts or stops. These can be as powerful as hooks.

4. **Verify trust in the repo author** — if you did not write the grove config and cannot verify the author is trusted, do not run mutating grove commands.

## Interpreting `grove doctor` Output

- **Green / OK** — configuration is valid and expected; no hook warnings
- **Yellow / Warning** — something is unusual; often a hook command that references external tooling or network resources. Read the warning text.
- **Red / Error** — configuration is broken or a required dependency is missing. Do not proceed until resolved.

## Safe Read-Only Alternatives

When you need to inspect a worktree without running hooks:

- `grove to <name> --peek` — switches filesystem context but skips ALL pre/post_switch hooks and does not attach tmux. Safe for reading files, running `grep`, or examining state.
- `grove test <name>` — runs tests in another worktree's directory but does NOT run switch hooks. Only the test runner itself runs.

## Further Reading

See [`docs/CONFIGURATION_REFERENCE.md`](https://github.com/lost-in-the/grove/blob/main/docs/CONFIGURATION_REFERENCE.md) for the complete `hooks.toml` schema, including all event types, `on_failure` modes, and the `working_dir` field options.
