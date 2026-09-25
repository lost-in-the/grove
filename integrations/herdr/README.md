# Grove plugin for herdr

A [herdr](https://herdr.dev) plugin that keeps grove's state in sync with
worktrees created or removed through herdr.

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
hooks, no docker. If you remove one through herdr, grove's record of it goes
stale the same way. Nothing in grove can notice either on its own.

Verified against herdr 0.8.0 and 0.9.1.

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
that only writes there has effectively said nothing. A hook may call back into
the socket API while the server is running it: herdr starts hook processes and
returns rather than waiting on them.

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

**Sidebar marker.** A notification is gone once dismissed — and off by default.
The durable signal is a workspace metadata token: the hook sets
`grove=untracked` on the workspace of every worktree grove does not track, and
clears it as soon as grove does (`grove adopt` clears it directly, and the next
open of the worktree re-checks). herdr drops every token when its server
restarts, so the plugin's startup hook reports them again.

herdr shows a custom token only where your own sidebar layout names it —
reporters supply values, styling stays yours. Add `$grove` to your space rows
in `~/.config/herdr/config.toml`. Starting from herdr's defaults:

```toml
[ui.sidebar.spaces]
rows = [
  ["state_icon", "workspace"],
  ["branch", "git_status", { token = "$grove", fg = "#e5c07b" }],
]
```

The token exists only on untracked worktrees, so on every other row it — and
its separator — simply disappears. Then `herdr server reload-config`.

Only one event per user action raises a notification. That is deliberate:
herdr's rate limiter drops near-simultaneous notifications (five within ~20ms
yielded one shown and four `rate_limited`), and `herdr worktree create` alone
fires five hook invocations in the same millisecond across all event types. Do
not add a second subscription that notifies.

Both events are subscribed because they are not interchangeable — verified
against herdr 0.8.0 and 0.9.1:

| herdr action | event fired |
|---|---|
| `herdr worktree create` | `worktree.created` |
| `herdr worktree open` | `worktree.opened` |
| UI "new worktree" dialog | `worktree.created` (same as the CLI) |

Subscribing to only one silently misses half the cases.

**Removal sync.** When a worktree is removed *through herdr* (UI or `herdr
worktree remove`), the `worktree.removed` hook drops grove's record of it:
the `.grove/state.json` entry goes away, `last_worktree` is cleared if it
pointed there (so `grove last` doesn't error on a checkout that no longer
exists), and on the tmux backend a session left over the dead directory is
killed. herdr's removal is already a clean `git worktree remove`, so the hook
never touches git.

Deliberate non-actions, mirroring the adoption prompt's philosophy:

- **No remove hooks fire** — user or plugin. A background event must not run
  side effects you didn't opt into. In particular, **a running docker stack is
  not stopped and its slot record is dropped with the state entry** — stop
  stacks before removing through herdr, or use `grove rm`, which runs the full
  teardown. The hook logs the docker project name before dropping the record.
- **No notification** — nothing is actionable; the cleanup is the whole story.
  It is written to the hook's stderr, which `herdr plugin log list` captures.
- **The branch is left alone**, matching herdr's own removal semantics.

This direction has no loop with `grove rm`: `worktree.removed` fires only when
herdr itself removes a worktree. The grove-rm flow — git removes the checkout,
then grove closes the herdr workspace — fires only `workspace.closed`, which
the plugin does not subscribe to (verified on 0.8.0 and 0.9.1).

**Worktree status action.** Right-click a workspace → "Grove: worktree status"
reports whether grove tracks that checkout.

## Install

```bash
herdr plugin install lost-in-the/grove/integrations/herdr --ref v0.11.0
```

Pin `--ref` to the tag of the grove release you have installed (`grove
version`; the plugin first shipped in v0.11.0). The plugin calls grove's hidden `herdr-event` / `herdr-action`
subcommands, so the manifest and the binary should come from the same release;
without `--ref` herdr installs the default branch. There is no `herdr plugin
update` — reinstall with a new `--ref` when you upgrade grove.

For local development against a checkout:

```bash
herdr plugin link /path/to/grove/integrations/herdr
```

Verify with `herdr plugin list`. `grove` must be on `PATH` — herdr runs plugin
commands as plain argv without shell expansion. (Calls back into herdr go
through `$HERDR_BIN_PATH`, which herdr sets for plugin processes, so `herdr`
itself need not be on that `PATH`.)

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
— the one herdr's sidebar groups a project's worktrees under — is herdr's: it
appears when herdr opens any linked worktree, and `grove to root` reaches it
through the same `worktree open` call grove uses for every worktree, which
herdr answers by opening that parent workspace.

See [docs/HERDR_INTEGRATION.md](../../docs/HERDR_INTEGRATION.md) for the full
design.

## Commands this plugin calls

`grove herdr-action` and `grove herdr-event` are hidden subcommands — an
integration surface, not something to run by hand. They read herdr's
`HERDR_PLUGIN_CONTEXT_JSON` and `HERDR_PLUGIN_EVENT_JSON`.
