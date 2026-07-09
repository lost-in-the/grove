# Shell Directive Protocol

Read this file when grove is emitting raw `cd:` or `tmux-attach:` lines, when you need to parse grove output programmatically, or when you're integrating grove into a script that doesn't use the shell wrapper.

## Why Directives Exist

A subprocess cannot change its parent shell's working directory or attach to tmux on its behalf. Grove solves this with a shell wrapper (`GROVE_SHELL=1`) that intercepts directive lines from the binary's stdout and acts on them. Without the wrapper active, these lines appear raw in output.

## Directive Types

| Directive | Format | When Emitted | What the Wrapper Does |
|-----------|--------|-------------|----------------------|
| `cd:` | `cd:/abs/path/to/worktree` | After switching worktrees | Runs `cd` in the parent shell |
| `tmux-attach:` | `tmux-attach:session-name` | After creating or switching to a tmux session | Runs `tmux attach-session -t session-name` |
| `tmux-attach-cc:` | `tmux-attach-cc:session-name` | Same, but for client-control mode (iTerm2) | Runs `tmux -CC attach-session -t session-name` |
| `env:` | `env:KEY=VALUE` | When a variable needs to propagate to the parent shell | Runs `export KEY=VALUE` in the parent shell |

**Examples:**

```
cd:/Users/you/project-feature
tmux-attach:project-feature
env:GROVE_CURRENT=feature
```

## Which Commands Emit Directives

Commands that change location or session state emit directives:

- `grove to` / `grove switch` / `grove t`
- `grove last` / `grove la`
- `grove new` / `grove n` (not `spawn` — that alias implies `--json` and returns the path in JSON instead of a raw `cd:` line)
- `grove fork` / `grove split` / `grove fo`
- `grove fetch`
- `grove join` / `grove attach`
- `grove open`
- `grove up` (may emit env vars)
- `grove kick` (may emit env vars)

Commands that are read-only or operate on other worktrees emit no directives:

- `grove ls`, `grove here`, `grove context`, `grove ps`
- `grove test`, `grove diff`, `grove down`
- `grove doctor`, `grove logs`

## Agent Context: Filtering Directives

Without `GROVE_SHELL=1`, directive lines appear as plain stdout. Agents must filter them to avoid treating them as command output:

```bash
grove to feature 2>&1 | grep -vE '^(cd:|tmux-attach(-cc)?:|env:)'
```

Or use the provided helper, which handles this and returns structured JSON:

```bash
grove to feature 2>&1 | python3 "${CLAUDE_PLUGIN_ROOT:-skills/grove-worktree-management}/scripts/strip_directives.py"
```

**Parsing rule:** Any line that does NOT start with `cd:`, `tmux-attach:`, `tmux-attach-cc:`, or `env:` is normal output (progress messages, warnings, errors). Directives are always the only content on their line.

## TUI File-Based Mechanism

When the TUI dashboard is active (alt-screen mode), writing `cd:` to stdout would corrupt the display. Instead, grove writes the target path to a file specified by `GROVE_CD_FILE`:

```bash
# Grove writes: /Users/you/project-feature
# to the path in $GROVE_CD_FILE
# The wrapper reads it after the TUI exits and runs cd
```

This is handled transparently by the shell wrapper. Agents running with `GROVE_TUI=0` never encounter this path.

## Recursion Guard

Inside the shell wrapper, the grove binary is invoked as `$__GROVE_BIN` rather than `grove`. This prevents recursive wrapper activation when the wrapper itself calls the binary. Do not override `__GROVE_BIN` in agent scripts.

## Further Reading

See [`docs/SHELL_INTEGRATION.md`](https://github.com/lost-in-the/grove/blob/main/docs/SHELL_INTEGRATION.md) for the full wrapper source, the exact grep patterns, and the wrapper installation instructions for zsh and bash.
