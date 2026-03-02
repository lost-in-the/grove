# Shell Integration

Grove needs a shell wrapper to work correctly. Without it, commands like `grove to` that are supposed to change your directory won't work — a subprocess cannot change its parent shell's working directory.

The shell wrapper is a function that replaces the `grove` binary in your shell session. It calls the binary, intercepts any directory-change directives in the output, and applies them in your current shell.

## Setup

### zsh

Add to your `~/.zshrc`:

```zsh
eval "$(grove install zsh)"
```

### bash

Add to your `~/.bashrc`:

```bash
eval "$(grove install bash)"
```

After adding the line, restart your shell or source the config file:

```bash
source ~/.zshrc    # zsh
source ~/.bashrc   # bash
```

### Verifying Setup

```bash
type grove
# Should output: grove is a shell function
```

## What the Integration Does

Running `eval "$(grove install <shell>)"` installs three things:

1. **`grove` shell function** — wraps the binary and handles directory changes
2. **Tab completion** — for commands and worktree names
3. **`w` alias** — shorthand for `grove`

## How the Wrapper Works

The wrapper uses a **directives protocol** — the grove binary writes special lines to stdout that the shell function intercepts and acts on.

### Directive Commands (`grove to`, `grove last`, `grove fork`, `grove fetch`, `grove attach`, `grove open`)

These six commands can emit `cd:` or `tmux-attach:` directives. The wrapper captures their stdout (stderr passes through to the terminal), scans it line-by-line, separates directives from normal output, and then:

1. Executes any directory change
2. Prints normal output
3. Attaches to any tmux session

```bash
# The binary outputs this internally:
cd:/Users/you/work/myproject-feature

# The shell wrapper intercepts it and runs:
cd /Users/you/work/myproject-feature
```

### TUI Mode (`grove` with no arguments)

The TUI uses a different mechanism because it runs in alt-screen mode and stdout is not captured:

1. The wrapper creates a temporary file via `mktemp`
2. It sets `GROVE_CD_FILE` to the temp file path and launches the binary
3. After the TUI exits, the wrapper reads the temp file for a path to switch to
4. It performs the `cd` and cleans up the temp file

```bash
# Wrapper logic (simplified):
cd_file=$(mktemp /tmp/grove-cd.XXXXXX)
GROVE_SHELL=1 GROVE_CD_FILE="$cd_file" grove_binary
if [[ -s "$cd_file" ]]; then
    cd "$(cat "$cd_file")"
fi
rm -f "$cd_file"
```

### All Other Commands (passthrough)

Commands that never emit directives — `grove ls`, `grove logs`, `grove test`, `grove up`, `grove here`, etc. — run the binary directly without output capture:

```bash
GROVE_SHELL=1 "$__GROVE_BIN" "$@"
```

This means:
- **Streaming works correctly** — `grove logs -f` output appears in real time
- **Stderr stays separate** — error messages don't get mixed into stdout
- **Exit codes propagate directly** — no intermediate capture logic

## Directives Reference

The grove binary communicates with the shell wrapper through directive lines — special prefixes on stdout lines.

| Directive | Example | Action |
|-----------|---------|--------|
| `GROVE_CD:` | `GROVE_CD:/path/to/dir` | Change directory (current) |
| `cd:` | `cd:/path/to/dir` | Change directory (legacy, same effect) |
| `tmux-attach:` | `tmux-attach:myproject-feature` | Attach to named tmux session |

Lines that do not match any directive prefix are treated as normal output and printed to the terminal as-is.

### TUI Directive (file-based)

When `GROVE_CD_FILE` is set, the TUI writes the target path to the file instead of printing a `cd:` directive. This is necessary because the TUI runs in alt-screen mode and its stdout is not suitable for directive parsing.

## Environment Variables

| Variable | Description |
|----------|-------------|
| `GROVE_SHELL` | Set to `1` by the wrapper. The binary uses this to enable directive output. Without it, commands print human-readable output only. |
| `GROVE_CD_FILE` | Path to a temp file where the TUI writes a directory to switch to. Set by the wrapper for bare `grove` invocations. |
| `GROVE_TUI` | Set to `0` to disable the TUI. When disabled, bare `grove` prints usage instead of launching the dashboard. |
| `GROVE_HIGH_CONTRAST` | Set to `1` to enable high-contrast mode in the TUI's form elements. |
| `GROVE_LOG` | Set to `1` to enable debug logging to `~/.grove/grove.log`. Set to a path to log to a custom file. |

## Tab Completion

The shell integration registers completion functions for both shells.

### zsh

Uses `compdef` with a `_grove_completion` function. Completion is registered automatically when you source the integration.

### bash

Uses `complete -F _grove_completion grove`. Works with or without bash-completion installed — falls back to basic `$COMP_WORDS` parsing if `_init_completion` is not available.

### What Gets Completed

| Position | Completions |
|----------|-------------|
| First argument | All grove commands |
| Second argument (after `to`, `rm`, `compare`, `sync`, `test`, `apply`, `attach`, `open`) | Worktree short names |
| Second argument (after `install`) | `zsh`, `bash` |

Worktree names are fetched by running `grove ls -q` (quiet mode), which lists short names only.

## Alias

The integration sets `w` as an alias for `grove`:

```bash
w to feature       # same as: grove to feature
w                  # same as: grove (opens TUI)
```

## Troubleshooting

### `grove to` doesn't change directory

The shell wrapper is not active. Verify:

```bash
type grove
# Should print: grove is a shell function
# If it prints a path like /usr/local/bin/grove, the wrapper is not loaded.
```

Add `eval "$(grove install zsh)"` (or bash) to your shell config and restart.

### Tab completion not working

Check that your shell loaded the integration by running `type _grove_completion`. If not found, the eval line may not have run.

For zsh, ensure `compinit` has been called before or after the grove integration line — order matters for some zsh setups.

### TUI closes but doesn't change directory

The TUI uses a temp file (`GROVE_CD_FILE`) for directory handoff. If the temp directory is full or unwritable, the handoff silently fails. Check:

```bash
mktemp "${TMPDIR:-/tmp}/grove-cd.XXXXXX"
```

If this fails, fix your `$TMPDIR` or `/tmp` permissions.

### `GROVE_SHELL` not set — directives not printed

The `grove` binary only prints `cd:` and `tmux-attach:` directives when it detects `GROVE_SHELL=1`. If you call the binary directly (bypassing the shell function), it prints human-readable output without directives. This is by design.

### Tmux session not attaching

`tmux-attach:` directives are only acted on when the wrapper detects the directive in output. Ensure:
1. `tmux` is installed and in `$PATH`
2. `grove install zsh/bash` was run after installing tmux
3. The tmux mode in config is not set to `off` (`grove config` → Behavior → `tmux_mode`)
