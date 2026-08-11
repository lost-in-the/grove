# Grove plugin for herdr

A [herdr](https://herdr.dev) plugin that keeps grove's state in sync with
worktrees created through herdr.

Note the direction: `plugins/` in this repo holds plugins that extend *grove*
(docker, tracker). This one runs the other way — it extends *herdr*, and grove
is the thing being invoked.

## Scope: deliberately small

**This plugin is not how you use grove inside herdr.** You use grove from a
shell, exactly as you would without herdr — `grove` to pick a worktree, `grove
new` to make one. The herdr *backend* (`[mux] backend = "herdr"`) is what makes
those work against herdr workspaces, and it needs no plugin at all.

The plugin exists for the one thing the CLI cannot do: **react to something
herdr did.** If you create a worktree through herdr's own UI, grove never runs,
so its bootstrap never runs either — no state, no excludes, no post-create
hooks, no docker. Nothing in grove can notice that on its own.

There is deliberately **no dashboard pane**. herdr's sidebar already lists every
worktree as a workspace, so a grove picker inside herdr duplicates the primary
UI. It also cannot switch cleanly from a pane: grove's TUI exits as part of
switching, which kills the pane it is running in.

## What it provides

**Adoption prompt.** When a worktree appears in herdr that grove has no state
for, the `worktree.created` / `worktree.opened` hooks point at `grove adopt`.
It only reports. Adopting runs post-create hooks and docker auto-start, which
should be a deliberate choice rather than something a background event fires.

The prompt is raised as a **herdr notification**, not just written to the hook's
stderr. A hook's stdout and stderr go to `herdr plugin log list` and nowhere
else — there is no toast, badge, or overlay keyed on plugin output — so a hook
that only writes there has effectively said nothing. Verified against herdr
0.8.0 that a hook may call back into the socket API while the server is running
it, without deadlocking.

> **You may need to turn notifications on.** Delivery is herdr's setting, not
> grove's, and a plugin cannot override it. herdr ships `[ui.toast] delivery`
> commented out as `off`, in which case the request comes back
> `reason: "disabled"` and nothing appears. Set it in `~/.config/herdr/config.toml`:
>
> ```toml
> [ui.toast]
> delivery = "system"   # or "herdr" / "terminal"
> ```
>
> Grove treats suppression as normal rather than an error, and records the
> reason in its own log (`GROVE_LOG=1`) so a prompt that never appeared is
> still explainable.

Only one event per user action raises a notification. That is deliberate:
herdr's rate limiter drops near-simultaneous notifications (five within ~20ms
yielded one shown and four `rate_limited`), and `herdr worktree create` alone
fires five hook invocations in the same millisecond across all event types. Do
not add a second subscription that notifies.

Both events are subscribed because they are not interchangeable — verified
against herdr 0.8.0:

| herdr action | event fired |
|---|---|
| `herdr worktree create` | `worktree.created` |
| `herdr worktree open` | `worktree.opened` |

Subscribing to only one silently misses half the cases.

**Worktree status action.** Right-click a workspace → "Grove: worktree status"
reports whether grove tracks that checkout.

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

`min_herdr_version` is **required** in the manifest; omitting it fails the link
outright. Omitting `platforms` links with a warning.

That means the shell function installed by `grove install zsh|bash` is not
enough: it exists only inside an interactive shell, and herdr's server never
sees it. The plugin needs the grove **binary** resolvable on the `PATH` the
herdr server inherited, or its actions and hooks fail with "No such file or
directory".

### Binding a key to the status action

herdr does **not** read `[[keys.command]]` from a plugin manifest — only
`build`, `startup`, `actions`, `events`, `panes`, and `link_handlers` are
parsed, so a binding declared here is silently ignored. Put it in your own
`~/.config/herdr/config.toml` instead, referencing the globally-qualified id:

```toml
[[keys.command]]
key = "prefix+shift+s"
type = "plugin_action"
command = "lost-in-the.grove.status"
description = "Grove: worktree status"
```

Then `herdr server reload-config`.

## How grove and herdr divide the work

Grove owns worktree lifecycle; herdr owns terminals.

Grove creates checkouts using its own `[naming] pattern` and `projects_dir`,
then hands the path to herdr via `herdr worktree open --path`, which adopts an
existing checkout without creating one. Grove never calls `herdr worktree
create` (that would impose herdr's naming and placement) or `herdr worktree
remove` (that runs `git worktree remove`, bypassing grove's `[protection]`
rules). Closing a grove session is `herdr workspace close`, which drops the
panes and leaves the checkout alone.

Grove also never calls `herdr workspace create`. The repository's own workspace
— the one herdr's sidebar groups a project's worktrees under — appears on its
own when herdr opens any linked worktree, and managing it is herdr's job, not
grove's. Grove adopts it if it is there and otherwise just changes directory.

See [docs/HERDR_INTEGRATION.md](../../docs/HERDR_INTEGRATION.md) for the full
design.

## Commands this plugin calls

`grove herdr-action` and `grove herdr-event` are hidden subcommands — an
integration surface, not something to run by hand. They read herdr's
`HERDR_PLUGIN_CONTEXT_JSON` and `HERDR_PLUGIN_EVENT_JSON`.
