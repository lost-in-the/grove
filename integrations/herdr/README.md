# Grove plugin for herdr

A [herdr](https://herdr.dev) plugin that surfaces grove inside herdr's
mouse-first UI.

Note the direction: `plugins/` in this repo holds plugins that extend *grove*
(docker, tracker). This one runs the other way — it extends *herdr*, and grove
is the thing being invoked.

## What it provides

**Dashboard popup.** `prefix`-menu → Grove dashboard opens the grove TUI (bare
`grove`) as an 80% overlay. This is the plugin's main reason to exist: herdr has no
`display-popup` equivalent on the CLI, and popup placement is only reachable
through a plugin's declared pane. Without the plugin installed, `grove open`
with `[session] popup = true` falls back to a full-window switch under herdr.

**Worktree status action.** Right-click a workspace → "Grove: worktree status"
reports whether grove tracks that checkout.

**Adoption prompt.** When a worktree is opened in herdr that grove has no state
for — typically one created through herdr's own `worktree create` rather than
`grove new` — the `worktree.opened` hook points at `grove adopt`. It only
reports. Adopting runs post-create hooks and docker auto-start, which should be
a deliberate choice rather than something a background event triggers.

## Install

```bash
herdr plugin install lost-in-the/grove/integrations/herdr
```

For local development against a checkout:

```bash
herdr plugin link /path/to/grove/integrations/herdr
```

Verify with `herdr plugin list`. `grove` must be on `PATH` — herdr runs plugin
commands as plain argv without shell expansion.

That means the shell function installed by `grove install zsh|bash` is not
enough: it exists only inside an interactive shell, and herdr's server never
sees it. The plugin needs the grove **binary** resolvable on the `PATH` the
herdr server inherited, or its panes and actions fail with "No such file or
directory".

## How grove and herdr divide the work

Grove owns worktree lifecycle; herdr owns terminals.

Grove creates checkouts using its own `[naming] pattern` and `projects_dir`,
then hands the path to herdr via `herdr worktree open --path`, which adopts an
existing checkout without creating one. Grove never calls `herdr worktree
create` (that would impose herdr's naming and placement) or `herdr worktree
remove` (that runs `git worktree remove`, bypassing grove's `[protection]`
rules). Closing a grove session is `herdr workspace close`, which drops the
panes and leaves the checkout alone.

See [docs/HERDR_INTEGRATION.md](../../docs/HERDR_INTEGRATION.md) for the full
design.

## Commands this plugin calls

`grove herdr-action` and `grove herdr-event` are hidden subcommands — an
integration surface, not something to run by hand. They read herdr's
`HERDR_PLUGIN_CONTEXT_JSON` and `HERDR_PLUGIN_EVENT_JSON`.
