# Shell Integration

Grove needs a shell wrapper to work correctly. Without it, commands like `grove to` that are supposed to change your directory won't work — a subprocess cannot change its parent shell's working directory.

The shell wrapper is a function that replaces the `grove` binary in your shell session. It calls the binary, intercepts any directory-change directives in the output, and applies them in your current shell.

## Setup

The easiest way to set up shell integration is:

```bash
grove setup
```

This detects your shell, finds the rc file, and appends the eval line idempotently.

### Manual Setup

#### zsh

Add to your `~/.zshrc`:

```zsh
eval "$(grove install zsh)"
```

#### bash

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

Running `eval "$(grove install <shell>)"` installs two things:

1. **`grove` shell function** — wraps the binary and handles directory changes
2. **Tab completion** — for commands and worktree names

A shorthand alias is available opt-in via `--alias` (see [Alias](#alias)).

## How the Wrapper Works

The wrapper uses a **directives protocol** — the grove binary writes special lines to stdout that the shell function intercepts and acts on.

### Directive Commands (`grove new`, `grove to`, `grove last`, `grove fork`, `grove fetch`, `grove join`, `grove open`, `grove up`, `grove kick`)

These commands (and their aliases) can emit `cd:`, `tmux-attach:`, `tmux-attach-cc:`, or `env:` directives. The wrapper captures their stdout (stderr passes through to the terminal), scans it line-by-line, separates directives from normal output, and then:

1. Exports any environment variables
2. Executes any directory change
3. Prints normal output
4. Attaches to any tmux session

```bash
# The binary outputs this internally:
env:ADMIN_DIR=./admin-feature
cd:/Users/you/work/myproject-feature

# The shell wrapper intercepts it and runs:
export ADMIN_DIR=./admin-feature
cd /Users/you/work/myproject-feature
```

### TUI / Browser Mode (`grove` with no arguments, `grove issues`, `grove prs`)

The TUI and the interactive issue/PR browsers use a different mechanism because they render to the terminal (alt-screen) and their stdout is not captured — a raw `cd:` line would print literally instead of changing directory:

1. The wrapper creates a temporary file via `mktemp`
2. It sets `GROVE_CD_FILE` to the temp file path and launches the binary
3. After the binary exits, the wrapper reads the temp file for a path to switch to
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

Commands that never emit directives — `grove ls`, `grove logs`, `grove down`, `grove here`, etc. — run the binary directly without output capture:

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
| `cd:` | `cd:/path/to/dir` | Change directory |
| `tmux-attach:` | `tmux-attach:myproject-feature` | Attach to named tmux session |
| `tmux-attach-cc:` | `tmux-attach-cc:myproject-feature` | Attach using `tmux -CC` (iTerm2 control mode) |
| `env:` | `env:ADMIN_DIR=./admin-feature` | Export environment variable in shell |

> **Note:** `GROVE_CD:` appeared in older documentation but is not used by the current shell templates. The active directive is `cd:`. If you have scripts or tooling that rely on `GROVE_CD:`, update them to use `cd:` instead.

Lines that do not match any directive prefix are treated as normal output and printed to the terminal as-is.

### TUI Directive (file-based)

When `GROVE_CD_FILE` is set, the TUI and the issue/PR browsers write the target path to the file instead of printing a `cd:` directive. This is necessary because they render to the terminal and their stdout is not suitable for directive parsing.

Because `GROVE_CD_FILE` takes precedence, the wrapper's capture branch (to,
new, open, …) invokes the binary with `GROVE_CD_FILE=` explicitly cleared. A
tmux server started from an issues/prs invocation inherits that invocation's
(long-deleted) temp path into every later pane; without the clear, grove
would silently write the cd target to the stale file and the captured `cd:`
line the wrapper parses would never be emitted.

## Environment Variables

| Variable | Description |
|----------|-------------|
| `GROVE_SHELL` | Set to `1` by the wrapper. The binary uses this to enable directive output. Without it, commands print human-readable output only. |
| `GROVE_SHELL_VERSION` | Shell integration version number. The binary checks this and warns when the shell integration is outdated. |
| `GROVE_CD_FILE` | Path to a temp file where the TUI writes a directory to switch to. Set by the wrapper for bare `grove` invocations. |
| `GROVE_TUI` | Set to `0` to disable the TUI. When disabled, bare `grove` prints usage instead of launching the dashboard. |
| `GROVE_HIGH_CONTRAST` | Set to `1` to enable high-contrast mode in the TUI's form elements. |
| `GROVE_LOG` | Set to `1` to enable debug logging to `~/.grove/grove.log`. Set to a path to log to a custom file. |

## Tab Completion

The shell integration registers completion functions for both shells.

### zsh

Uses `compdef` with a `_grove_completion` function. Completion is registered automatically when you source the integration — but only if the completion system is loaded: registration is guarded by `(( $+functions[compdef] ))`, so sourcing the integration before `compinit` degrades to "no tab completion" instead of a startup error. For completion to work, keep the grove eval line **after** `compinit` in your `~/.zshrc`.

### bash

Uses `complete -F _grove_completion grove`. Works with or without bash-completion installed — falls back to basic `$COMP_WORDS` parsing if `_init_completion` is not available.

### What Gets Completed

| Position | Completions |
|----------|-------------|
| First argument | All grove commands |
| Second argument (after `to`, `rm`, `diff`, `sync`, `test`, `graft`, `join`, `open`) | Worktree short names |
| Second argument (after `install`) | `zsh`, `bash` |

Worktree names are fetched by running `grove ls -q` (quiet mode), which lists short names only.

## Alias

A shorthand alias is opt-in. Pass `--alias` to `grove install` (or `grove
setup`) — bare `--alias` means `w`, or pick your own name:

```bash
eval "$(grove install zsh --alias)"     # alias w=grove
eval "$(grove install zsh --alias=g)"   # alias g=grove
grove setup --alias                     # writes the aliased eval line to your rc
```

With the alias installed:

```bash
w to feature       # same as: grove to feature
w                  # same as: grove (opens TUI)
```

Alias names are validated (`[A-Za-z_][A-Za-z0-9_.-]*`) since they are
interpolated into shell code that your rc file evals.

## Recursion Guard

Both shell templates (`grove.zsh` and `grove.bash`) open with a guard that prevents infinite recursion:

```bash
if [[ -z "$__GROVE_BIN" || "$__GROVE_BIN" == "grove" ]]; then
    echo "grove: binary not found (is grove on your PATH?)" >&2
    return 127
fi
```

**What `__GROVE_BIN` is:** When `grove install <shell>` runs, it emits shell code that sets `__GROVE_BIN` to the resolved path of the grove binary — via `whence -p grove` (zsh) or `type -P grove` (bash). The shell function uses `$__GROVE_BIN` — not the bare word `grove` — to call the real binary.

**Why the lookup must be function-immune:** On an rc re-source, the `grove()` wrapper function from the previous eval is already defined. A plain `command -v grove` would resolve to that function and return the string `grove` instead of the binary path, permanently tripping the guard below for the rest of the shell session. `whence -p` / `type -P` search `$PATH` only, ignoring functions and aliases, which makes `eval "$(grove install <shell>)"` idempotent — re-sourcing your rc file is safe.

**What the guard prevents:** If the binary is not on `$PATH`, the lookup returns nothing and `__GROVE_BIN` ends up empty or is literally the string `"grove"`. Without the guard, calling `$__GROVE_BIN` would invoke the shell function again, looping forever.

**What users see:** When the guard fires, the shell prints `grove: binary not found (is grove on your PATH?)` to stderr and returns exit code 127. The command is a silent no-op from the user's perspective.

**How to diagnose:** If you see this error, the grove binary is not in `$PATH` at the time the shell integration was evaluated. Check:

```bash
which grove          # should print the binary path
echo "$__GROVE_BIN"  # should match the above path
```

If `which grove` works but `$__GROVE_BIN` is wrong, re-evaluate the integration:

```bash
eval "$(grove install zsh)"   # or bash
```

## Version Bumps

The constant `ShellVersion` in `internal/shell/version.go` tracks the shell integration template version. It is currently **7**.

When the shell wrapper behavior changes incompatibly — new directives, changed passthrough logic, new env vars — increment `ShellVersion`. The grove binary reads `GROVE_SHELL_VERSION` (set by the wrapper) in grove-context commands and `grove doctor`, and emits a warning when the running shell integration is older than `ShellVersion`.

**What users must do after a version bump:**

```bash
eval "$(grove install zsh)"   # or bash
source ~/.zshrc               # (or ~/.bashrc)
```

`grove setup` handles this automatically if run again. `grove doctor` will surface a version mismatch as a warning with the re-run command included in its output.

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

For zsh, registration requires `compinit` to have run **before** the grove eval line. Sourcing in the other order no longer errors — the integration silently skips registration — but completion won't work. Diagnose with:

```zsh
(( $+functions[compdef] )) && echo "compinit loaded" || echo "compinit NOT loaded — move it above the grove line"
```

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

### iTerm2 control mode (`tmux -CC`)

When `TERM_PROGRAM=iTerm2` and `tmux.control_mode` is enabled (default: true), grove uses `tmux -CC attach` instead of `tmux attach`. This integrates tmux sessions as native iTerm2 windows/tabs.

The shell wrapper handles the `tmux-attach-cc:` directive automatically. If you see issues:
1. Run `grove doctor` — it checks for `aggressive-resize` which conflicts with `-CC`
2. To disable: set `control_mode = false` in `[tmux]` config
3. Non-iTerm2 terminals always use standard `tmux attach` regardless of this setting
