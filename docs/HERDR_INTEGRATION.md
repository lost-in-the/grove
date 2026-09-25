# Herdr Integration

**Status:** Shipped (unreleased). Verified against herdr **0.8.0** (protocol 19)
and **0.9.1** (protocol 22), and against herdr's source at `21d0ce60` (main,
after 0.9.1). CI runs the live validation suite against a pinned headless
herdr. `[mux] backend = "herdr"` selects it; `auto` picks it when grove is
running inside a herdr pane.

[Herdr](https://herdr.dev) is a terminal multiplexer built for coding agents: a
background server owns real terminals, clients attach to render them, and panes
survive detach. Same shape as tmux, but mouse-first and agent-aware — it detects
coding agents in panes and tracks `working` / `blocked` / `done` / `idle` state.

Source references below are to [herdrdev/herdr](https://github.com/herdrdev/herdr).
Findings were verified against its source and against live servers, not just
its docs.

---

## Two pieces, two directions

| Piece | Direction | Where | Needed? |
|---|---|---|---|
| **Backend** | grove → herdr | `internal/mux/herdr.go` | Yes, to use grove inside herdr. It is the herdr counterpart of the tmux integration. |
| **Plugin** | herdr → grove | `integrations/herdr/` | Optional. It only matters if worktrees are also created or removed through herdr's own UI. |

```
             grove (owns the worktrees)                      herdr (owns the terminals)
 grove new ─► git worktree add + bootstrap ───────────► herdr worktree open --path <checkout>
 grove to  ─► ─────────────────────────────────────────► herdr workspace focus <id>
 grove ls  ◄─ ◄──────────────────────────────────────── herdr workspace list (status, agent state)
 grove rm  ─► git worktree remove (protection rules) ──► herdr workspace close <id>

 herdr UI "new/remove worktree" ─► plugin hook ─► grove herdr-event (prompt, marker, state sync)
```

Without the backend, grove inside herdr would either nest tmux sessions inside
a herdr pane, or run with `backend = "off"` and only `cd` the current pane —
no workspace per worktree, no sync on `rm`/`rename`, no agent column. herdr's
own worktree support does not replace grove: its only worktree setting is
`[worktrees] directory` (checkouts land at `<dir>/<repo>/<branch-slug>`), with
no naming templates, no post-create hooks, no protection rules, no bootstrap —
unchanged from 0.8.0 through 0.9.1.

---

## The ownership boundary

herdr ships `herdr worktree list|create|open|remove`: worktrees are "normal
Herdr workspaces with Git checkout provenance." That overlaps grove's core
purpose, so the boundary is explicit:

> **Grove owns worktree lifecycle. Herdr is only a session/display backend.**

- **Grove never calls `herdr worktree create`.** It would hand naming and
  placement to herdr, breaking the canonical `{project}-{name}` rule.
- **Grove never calls `herdr worktree remove`.** It shells out to
  `git worktree remove`, bypassing grove's `[protection]` rules. Grove's
  removal path stays grove's; the herdr-side cleanup is `workspace close`,
  which closes herdr state and panes and leaves the checkout alone.
- **Grove never calls `herdr workspace create`.** Every session goes through
  `herdr worktree open`, which only *adopts* a checkout git already lists
  (`handle_worktree_open` resolves the path against the repo's worktree list
  and returns `worktree_not_found` otherwise). It is idempotent: a second open
  of the same checkout reuses the workspace, re-applies `--label`, and reports
  `already_open: true`.

**`worktree open` takes two directories, and they are not interchangeable.**
`--cwd` names the *source repository* whose worktree list is searched; `--path`
names the checkout to open. Passing the linked worktree as `--cwd` fails with
`linked_worktree_source`, and omitting `--cwd` falls back to the focused
workspace, which does not exist outside a client (`invalid_request`). So
`mux.Target` carries `Repo` — the repository's main checkout — alongside
`Path`. Sessions that already exist (kill, focus, rename) resolve without it.

### The repository workspace

herdr's sidebar groups a project's worktrees under the repository's own
*parent workspace*, keyed on the `repo_root` / `repo_key` provenance it
records. herdr materializes that workspace as a side effect of opening any
linked worktree, and `worktree open --path <repo root>` opens (or creates) it
directly (verified on 0.8.0 and 0.9.1). `grove to root` therefore reaches it
through the same call as every worktree. That is herdr's own bookkeeping —
grove still never `workspace create`s.

A path herdr refuses to open — `worktree_not_found` or `not_git_worktree` — is
adopted if some workspace already covers it, and otherwise reported unmanaged
(`mux.ErrUnmanaged`), so the command falls through to a plain `cd`.

Since herdr 0.9.0, closing a parent workspace while worktree workspaces are
open under it fails with `workspace_group_close_required` unless `--group` is
passed — which would close every one of them. Grove never passes `--group`;
it closes each worktree's workspace on its own, and names the refusal if it
ever meets it.

---

## Identity: the checkout path, not the name

Grove's tmux model is name-keyed. herdr workspaces have opaque ids (`w1`) plus
a **non-unique** label, and no lookup by label — but `workspace list` reports
each workspace's checkout path (`WorkspaceInfo.worktree.checkout_path`), its
focus, and its rolled-up agent status in one call. Grove already knows the
checkout path of every worktree, so:

- **Identity key = the checkout path**, compared canonicalized
  (`filepath.EvalSymlinks` on both sides; herdr uses `canonical_or_original`).
- **One call replaces `tmux list-sessions`** and feeds `grove ls`, the TUI,
  and every existence check. It is fetched once per command and shared through
  `mux.Index`; `herdr workspace list` averages ~3ms against a warm server.
- **The label is cosmetic.** It carries the canonical `{project}-{name}` for
  display, and a user renaming a workspace cannot desync grove.

**Caveat: herdr's paths do not follow a rename.** herdr captures a workspace's
paths when it opens and never updates them, so `grove rename` leaves
`worktree.checkout_path`, the workspace cwd, and the pane cwd all naming the
pre-rename directory. Grove self-heals, and the mechanism is load-bearing:
**`mux.Index` refuses to index a checkout path that is not on disk.** Without
that, a workspace still claiming `…/proj-beta` after its worktree became
`…/proj-gamma` would shadow the real workspace of whatever worktree later
occupied `…/proj-beta`. Dropping vanished paths leaves such a workspace
reachable by name only — exactly right, since `grove rename` relabels it.
(herdr's unreleased `6c62707a` stops a shell `cd` from moving a workspace's
recorded checkout; rename staleness is unaffected.) The plugin cannot
self-heal the same way — it only has herdr's context — and says so plainly.

---

## Session status: `active` / `open`

tmux reports `attached` / `detached`. herdr's `focused` flag is not that: it is
a single server-wide "current workspace" (`state.active`) that moves with any
CLI focus call or the most recently active client's view, stays set with **no
client attached at all**, and marks only one workspace when several clients
view different ones (0.9.0 per-client views). No public API lists attached
clients.

So herdr sessions report **`active`** (the server's focused workspace) and
**`open`** (exists, not focused). `grove ls`, `grove here`, their `--json`
`session` field, and the dashboard show those words; `mux.Status` has
`Foreground()` / `Background()` so the dashboard styles both vocabularies
alike.

---

## Named sessions

A herdr **session** is an independent server with its own socket, state, and
workspaces (`<config>/sessions/<name>/`), and no awareness of the others —
`worktree open` checks only its own server for an existing workspace. So the
same checkout opened under two sessions gets two workspaces, and closing one
leaves the other pointing at whatever grove later deletes. (Reproduced on
0.9.1 before grove handled it.)

**Which session grove talks to** follows herdr's own precedence: the global
`--session NAME` flag, then `HERDR_SOCKET_PATH` (set in every pane to its own
server), then `HERDR_SESSION`, then the default session. Grove uses the
ambient one, and addresses another with `--session`, which outranks
`HERDR_SOCKET_PATH` — so it works from inside a pane, where `HERDR_SESSION`
would not.

Grove looks across sessions only on the two paths where duplicates are made or
orphaned:

- **Opening, when the ambient session has no workspace for the checkout.**
  One `herdr session list --json` (bare JSON, no envelope; it probes each
  socket without a request) plus `herdr --session NAME workspace list` per
  other *running* session, matched on checkout path only — a label in a
  session grove never touched is not proof of identity — and never a
  workspace that now serves another worktree (see "Renamed worktrees" below).
  - Found, **outside herdr**: adopt it. `Attach` focuses it in its session and
    runs `herdr --session NAME`; the attach hint names the session; the
    command reports "Using '…' from herdr session '…'".
  - Found, **inside a herdr pane**: the CLI cannot move a client across
    sessions (a nested attach is refused by design) and a duplicate is the
    bug, so Ensure reports the target unmanaged with a hint naming the
    session, and the command just changes directory.
- **Removal.** `Kill` closes the checkout's workspace in the ambient session
  and in every other running one, even with the ambient server down. Since the
  checkout is usually already deleted, other sessions are matched on the
  recorded path (canonicalized through its surviving parent), never by label.
  `grove rm` and the dashboard delete no longer gate `Kill` on the ambient-only
  `Exists` — `mux.SessionLocator` marks backends where that would miss copies.

**Bounded cost.** herdr reports a session as running whenever its socket
accepts connections, so a wedged server (e.g. SIGSTOPped) still looks alive
and would hold each call for herdr's full 5s timeout — review of this feature
measured a frozen session stalling `grove to` and `grove rm` by 5s. Calls to
other sessions therefore run under a separate 250ms budget
(`cmdexec.HerdrPeer`; a healthy server answers in single-digit milliseconds),
and the sessions are listed concurrently, so the whole sweep costs one budget
however many there are. A session that misses it is skipped when opening, and
named in a warning on removal, since its copy may remain. Nothing is paid on
the common path; measured on 0.9.1, `grove to` takes 36–39ms and `grove rm`
35ms with a second healthy session.

Not handled: a **stopped** session cannot be reached (herdr drops such a
workspace's worktree link itself when it next starts, showing placeholder
panes), and `grove rename` relabels only the ambient session's workspace.

### Renamed worktrees

herdr's record of a workspace's checkout never follows a rename, so after
`grove rename a b` the workspace for `b` still claims `a`'s path. When a new
worktree later takes that path, everything that matches on the recorded path
would treat `b`'s workspace as the new `a`'s: herdr's own `worktree open`
would hand it over and re-label it, `grove to a` would `cd` its shell, and
`grove rm a` would close it.

The workspace's panes give it away — a process's working directory follows a
directory rename, so they sit in `b`. `movedOn` treats a workspace with a pane
inside **another live worktree of the same repository** as belonging to that
worktree: `Exists` does not count it, `Ensure` declines the target with a hint
rather than letting herdr take it over (herdr cannot open a second workspace
for the same path, so freeing it means closing that workspace), and `Kill` and
the cross-session lookups leave it alone. A pane that merely wandered off
(`cd /tmp`, or into the main checkout) is not evidence, so an ordinary
workspace is never mistaken for a moved one. It costs a `pane list` for the
candidate, plus one `git worktree list` only when a pane sits outside the
target.

---

## Naming the tab

Neither backend used to name anything below the session, so the terminal
titled the window after the running process — `grove`, or the whole
`grove to <name>` command line, in ghostty and cmux. Both backends now name it
from `Target.DisplayName()`:

- **herdr** — `tab rename <tab_id> <short>`; the tab id comes back in the
  `worktree open` response. A label that is not herdr's generated one was
  chosen by a person and is never overwritten. herdr's generated label is the
  tab's *position* plus one, while `TabInfo.number` is a stable counter that
  never renumbers, so grove treats any all-digit label as generated.
- **tmux** — `new-session -n <short>`, which also turns `automatic-rename` off.

Since herdr 0.8.2, `ui.window_title` also titles the outer terminal window
from the workspace label itself; the tab rename still names herdr's tab strip.
Both are best-effort.

---

## Attaching and switching

herdr's launch flags carry no workspace selector (no `--workspace` / `--cwd`;
`--no-session` was removed in 0.9.0), so attaching to a worktree is two steps:

```
herdr workspace focus <id>   # socket call, works with no client attached
herdr                        # blocking attach — starts on the focused workspace
```

Verified on 0.9.1 with real clients: a client that attaches after a focus call
starts on the focused workspace, and `workspace focus` from inside a pane moves
the attached client. It moves **every** attached client — herdr's CLI carries
no client identity, and only in-UI actions move a single client — so with
several clients viewing different workspaces, `grove to` moves them all.

Nested launches are refused (`should_block_nested`, `HERDR_ENV=1`), so from
inside a pane grove uses `workspace focus` alone — the analogue of tmux
`switch-client`.

The blocking attach must never run under shell integration: there grove's
stdout is the wrapper's command-substitution pipe, so the client would draw
into the pipe and the wrapper would parse its escape bytes as directives. herdr
has no wrapper directive the way tmux does (`tmux-attach:`), so with
`GROVE_SHELL=1` grove performs the directory switch, focuses the target
(`PrepareAttach`), and prints `Run: herdr` (or `herdr --session NAME`) instead
(`attachToSession`) — so running the hint lands on the target.

---

## When the server can't help

| Situation | herdr says | grove does |
|---|---|---|
| No server running | `server_not_running` | `Exists` → false, `Ensure` → unmanaged; commands fall back to a plain `cd`, silently. `grove ls` shows `none`. |
| Server and CLI protocols differ (every call after an upgrade, until the old server restarts) | `protocol_mismatch` | Same fallback, plus a one-line warning carrying herdr's message. |
| Worktree discovery slots full (herdr main, unreleased) | `worktree_busy` | One retry after 150ms, then the error. |
| Parent workspace with open worktrees | `workspace_group_close_required` | Never passes `--group`; explains the refusal. |

`grove doctor` checks the server with `herdr status server --json` (answers
without an envelope and exits 0 either way, on 0.8.0 and 0.9.1), which is the
one probe that tells "stopped" apart from "running, but needs a restart": it
reports `running`, `compatible`, and `restart_needed`.

---

## What herdr can't do

- **Popup.** herdr's popup placement (`PluginPanePlacement::Popup`) is reachable
  only by a plugin declaring a pane. Grove's plugin no longer declares one (see
  "The dashboard pane was removed" below), so under herdr `grove open` with
  `[session] popup = true` does a full-window workspace switch. The backend does
  not implement `mux.Popuper`, so the fallback is explicit, not a silent no-op.
- **Control mode.** `tmux -CC` has no herdr equivalent; `control_mode` is
  ignored.

---

## What grove gains: agent status

`WorkspaceInfo.agent_status` is a rollup over the workspace's panes: `idle`,
`working`, `blocked`, `done`, or `unknown`. `grove ls` (an AGENT column and a
JSON `agent` field) and the dashboard (a row badge and a detail row) show which
worktree has an agent waiting on input — something tmux cannot provide — at no
extra cost.

**`unknown` is the no-agent default**, not a rare "present but unclassified"
case: a workspace whose panes all sit at a shell prompt rolls up to `unknown`.
So grove treats only `idle`/`working`/`blocked`/`done` as a sighting
(`AgentStatus.Observed()`), and the column stays hidden when nothing was seen.

---

## Architecture

### `internal/mux`

`mux.Multiplexer` (`internal/mux/mux.go`) is the session surface every command
drives — `Ensure`, `EnsureWithCommand`, `Exists`, `List`, `Current`,
`AttachHint`, `Attach`, `Switch`, `Rename`, `Kill`, `PaneInfo`,
`SendCommand`. `mux.Target` carries `Name` (canonical, tmux's key), `Path`
(herdr's key), `Repo` (herdr's source repository), and `Short` (display name).

Optional capabilities are separate interfaces, checked at the call site, so a
gap stays explicit:

| Interface | Implemented by | Purpose |
|---|---|---|
| `Popuper` | tmux | `display-popup` overlay |
| `ControlModer` | tmux | iTerm2 `tmux -CC` |
| `AttachDirectiver` | tmux | hand attach to the shell wrapper |
| `SessionLocator` | herdr | sessions may live in another server; `Kill` / `KillEverywhere` close them everywhere |
| `AttachPreparer` | herdr | focus the target before printing an attach hint, since `herdr` cannot name one |
| `Adopter` | herdr | bring a newly adopted worktree's session in line |

Three backends: `TmuxBackend`, `HerdrBackend`, `OffBackend` (callers hold a
non-nil multiplexer unconditionally). Commands reach it through `ctx.Mux()`;
the TUI, which has no `GroveContext`, through `muxFor(cfg)`.

### The herdr backend

Every call shells out to the herdr CLI through `cmdexec` (timeout category
`Herdr`), running `$HERDR_BIN_PATH` when herdr exports it — its documented way
for panes and plugins to call back in, and protocol-compatible with the server
after a client-only update — and `herdr` from `PATH` otherwise.

| `Multiplexer` method | herdr implementation |
|---|---|
| `Available` | `exec.LookPath`, cached |
| `Inside` | `HERDR_ENV=1` |
| `Ensure` | another session's existing workspace if the ambient one has none (see Named sessions); else `worktree open --cwd R --path P --label N --no-focus`, then `tab rename` to the short name. A path herdr won't open: adopt a covering workspace, else `ErrUnmanaged` |
| `List` | `workspace list` → `Session{ID, Name, Path, Status, Agent}` |
| `Exists` | lookup in `List` via `mux.Index` (path, then label) |
| `Current` | `$HERDR_WORKSPACE_ID` → `workspace get` |
| `Attach` | `workspace focus <id>`, then exec `herdr` (with `--session` when adopted from another session) |
| `PrepareAttach` (`AttachPreparer`) | `workspace focus <id>` before an attach *hint* is printed (shell integration, manual mode), so the client the user starts lands on the target |
| `Switch` | `workspace focus <id>` |
| `Rename` | `workspace rename <id> <label>`, plus the tab when it carries a generated or grove-set label |
| `Kill` | `workspace close <id>` in every running session — **never** `worktree remove` |
| `PaneInfo` | `pane list --workspace <id>` + `pane process-info --pane <id>` |
| `SendCommand` | `pane run <pane> "<cmd>"` |

**The CLI contract.** Responses are single-line JSON envelopes —
`{"id":…,"result":…}` on stdout, `{"id":…,"error":{"code","message"}}` on
stderr with exit 1 (usage errors are plain text, exit 2). Grove reads both
streams together, scans for the line that parses as an envelope (an update
notice may share the stream), and decodes leniently so new fields never break
it. Details that cost real bugs:

- **Acknowledgement-only verbs print nothing on success** — `pane run` and
  `workspace report-metadata` exit 0 with empty output. Only their failures
  carry an envelope.
- **`pane process-info` nests its list** under `result.process_info`.
- `session list --json` and `status server --json` answer with bare JSON, no
  envelope.
- Every API command sends a protocol ping first, and the CLI sets no request
  timeout of its own — `cmdexec`'s budget is what bounds a wedged server.

**Caching.** A backend instance lives for one command, so pure reads
(`workspace list`, `pane list`, …) are cached for its lifetime and every other
verb drops the caches before running — one `grove to` used to spawn ten herdr
processes. The allow-list of cache-preserving verbs is closed: an unknown verb
counts as a mutation.

### Config

```toml
[mux]
backend = "auto"   # auto | tmux | herdr | off
```

`auto` resolves: `HERDR_ENV=1` → herdr; `TMUX` → tmux; else the first
available binary, tmux winning ties. The `[tmux]` block keeps working;
`tmux.mode = "off"` still maps to `backend = "off"`
(`Config.EffectiveMuxBackend()`).

---

## The herdr plugin

A herdr plugin is a `herdr-plugin.toml` manifest plus commands in any language.
Grove's lives in `integrations/herdr/` — not `plugins/`, which holds plugins
that extend *grove*; this one runs the other way. It calls hidden grove
subcommands, `grove herdr-event` and `grove herdr-action`.

**Scope: react to what herdr did.** You use grove inside herdr the way you
always do — from a shell — and the backend needs no plugin. The plugin exists
for the one thing the CLI cannot do: notice worktrees that appear or disappear
without grove's involvement.

| Manifest entry | What it does |
|---|---|
| `[[events]] worktree.created`, `worktree.opened` | If grove does not track the checkout: raise a herdr notification pointing at `grove adopt` (once per worktree — re-opens report `already_open`), and keep the `grove=untracked` sidebar token in step with grove's state on every open. |
| `[[events]] worktree.removed` | Drop grove's state entry, clear `last_worktree`, and (tmux backend) reap a session over the dead directory. No remove hooks, no git. |
| `[[startup]]` | Re-report untracked tokens after a server restart (herdr drops all tokens). |
| `[[actions]] status` | Report whether grove tracks the workspace's worktree. |

Both create/open events are needed: `herdr worktree create` fires only
`worktree.created`, `herdr worktree open` only `worktree.opened`, and the UI
"new worktree" dialog behaves like the CLI create path (verified with a probe
plugin on 0.8.0 and 0.9.1). `worktree.removed` fires only on herdr's own
removal — the grove-rm flow emits `workspace.closed` — so there is no loop.

**The sidebar marker.** A notification is off by default (`[ui.toast]
delivery`) and gone once dismissed. The durable signal is a workspace metadata
token, `workspace report-metadata <id> --source lost-in-the.grove --token
grove=untracked`, cleared with `--clear-token grove` once grove tracks the
worktree (`grove adopt` clears it directly, via `mux.Adopter`, and also gives
the workspace grove's canonical label). Reporters supply values only: the
token shows where the user's `[ui.sidebar.spaces]` rows name `$grove`. Grove
always reports under one fixed source, because herdr caps the distinct sources
a workspace sees over its lifetime.

**Execution environment.** herdr runs plugin commands as plain argv with the
*plugin directory* as cwd — after `herdr plugin install` that is a clone of
grove's repository, with a `.grove` of its own — so discovery always starts
from the path herdr names, never the cwd. It injects `HERDR_SOCKET_PATH`,
`HERDR_BIN_PATH`, `HERDR_ENV=1`, `HERDR_PLUGIN_ROOT`, and for hooks
`HERDR_PLUGIN_EVENT` (dotted name; `startup` for startup hooks) and
`HERDR_PLUGIN_EVENT_JSON` (`{"event": <snake_case>, "data": {...}}`, `data`
always carrying `workspace` with its worktree provenance and tokens). Hook
output goes to `herdr plugin log list` and nowhere else. A hook may call back
into the server: dispatch starts the process and returns rather than waiting.
Hooks run concurrently (up to 32) with no timeout.

An event name herdr does not know links with a warning and a hook that never
fires — it is not rejected — so `min_herdr_version` is the oldest version
actually tested (0.8.0). Install with `herdr plugin install
lost-in-the/grove/integrations/herdr --ref <grove tag>`; there is no `plugin
update`. herdr's marketplace lists public repos with the `herdr-plugin` GitHub
topic, showing the manifest's `version`.

### The dashboard pane was removed

The plugin first shipped a pane running grove's TUI. Making it *reachable*
(it ran `grove tui`, which is not a command) revealed it should not exist:

- It duplicated herdr's sidebar, which already lists every worktree, grouped by
  project, with agent status.
- It could not switch: grove's TUI switch quits the event loop and acts
  afterwards, so inside a pane it destroyed its own pane on every use.
- herdr runs plugin commands with the plugin directory as cwd, so it resolved
  the wrong project.
- The TUI switch path fires no pre/post-switch hooks, so docker start/stop
  would not run.

---

## Testing

- **Unit** (`internal/mux/herdr_test.go`, `cmd/grove/commands/herdr_plugin_test.go`):
  the backend takes an injectable runner, so its whole command contract runs
  against **captured herdr 0.9.1 responses** — envelopes, silent successes,
  error codes, event payloads. Invented fixtures are how the `pane run` and
  `process-info` bugs went unnoticed: the fakes answered the way grove
  expected. The fake understands `--session`, so named-session behavior is
  covered too. The negative assertions that matter most: `Ensure` never calls
  `worktree create` or `workspace create`, `Kill` never calls
  `worktree remove`, and never passes `--group`.
- **Golden** (`TestGolden_Dashboard_Herdr`): agent badges and `active`/`open`
  workspace badges.
- **Live** ([`scripts/validate-herdr.sh`](../scripts/validate-herdr.sh),
  `make test-herdr`): 35 checks against a real server — create, identity,
  idempotency, rename, switch, remove, the ownership boundary, `grove open`
  with a session command, drift correction, agent state (driven with
  `herdr pane report-agent`, so no real agent is needed), the repository
  workspace, named sessions, and with `--plugin` the untracked marker, adopt,
  and removal sync. `--stop-server` adds the degradation checks.
- **CI** (`herdr End-to-End`): installs a pinned, checksum-verified herdr,
  starts it headless, and runs the live suite with `--plugin --stop-server`.

**The live suite is safe against a live server, and that is a property to
preserve.** A herdr server is shared: the user's workspaces, their agents, and
possibly the pane running the script all appear in `workspace list`. The
script only ever closes workspaces whose checkout resolves inside its own
scratch directory — in every running session — starts and deletes its own
named session, refuses `--plugin` if the plugin is already installed, and
keeps the server-stopping checks behind `--stop-server`. It runs as if from
outside herdr (dropping a pane's `HERDR_SOCKET_PATH` / `HERDR_ENV` in favor of
naming the same session), and an `EXIT` trap unlinks the plugin and stops its
session even when interrupted.

---

## Verification log

**herdr 0.8.0** (Linux headless, then macOS arm64 with a live client and a
real `claude` agent): create/list/switch/rename/remove round-trip; attach lands
in the target workspace; real `blocked`/`idle`/`working` states tracked in
`ls`, JSON, row badge, and detail row; `/tmp` → `/private/tmp` canonicalization
resolves without duplicates; `control_mode` honored under tmux and ignored
under herdr; popup falls back to a switch; tmux behavior unchanged.

**herdr 0.9.1** (Linux headless, real clients through a pty): the full live
suite, all plugin hooks, per-client focus semantics, named sessions, protocol
mismatch against a 0.8.0 CLI, and the untracked marker across a server
restart.

Bugs that only live runs found:

1. **`worktree open` needs `--cwd <repo root>`.** Every session creation failed
   with `invalid_request` until `Target.Repo` was threaded through.
2. **`unknown` is the no-agent default**, so the AGENT column rendered
   "unknown" on every row. Fixed with `AgentStatus.Observed()`.
3. **herdr's provenance goes stale after a rename**, so the plugin's context
   pointed at a deleted directory. Fixed by resolving the first path that
   still exists.
4. **The dashboard pane ran `grove tui`, which is not a command.** Linking the
   manifest looked like verification because it parses either way. Now every
   declared argv is resolved against the real command set in a test.
5. **The adoption hook listened only for `worktree.opened`**, missing
   `herdr worktree create` — its primary case.
6. **`pane run` succeeds silently**, and grove took the silence for failure:
   `grove open` with `[session] command` ran the command, then reported
   "failed to create session".
7. **`pane process-info` nests its list under `process_info`**; grove read an
   always-empty list, so drift correction never ran under herdr.
8. **Named sessions duplicated and orphaned workspaces.**
9. **`focused` is not "attached"** — it is server-wide and set with no client.
10. **`worktree open --path <repo>` opens the parent workspace** — the docs had
    claimed grove declines the repository checkout.
11. **A wedged named session stalled `grove to` / `grove rm` by 5s** (found in
    review with a SIGSTOPped server) — now a 250ms peer budget, queried
    concurrently.
12. **A renamed worktree's workspace could be adopted, taken over, or closed**
    for a new worktree at its old path — now guarded by `movedOn`.

---

## Known limitations

- **Multi-client focus.** `grove to` moves every attached client; herdr's CLI
  cannot target one.
- **Stopped named sessions** cannot be reached, so their copy of a removed
  worktree's workspace is left for herdr to reconcile on restart.
- **`grove rename`** relabels only the ambient session's workspace.
- **A renamed worktree's workspace keeps its old path** in herdr's record, so a
  new worktree created at that path gets no workspace of its own until the
  renamed one's is closed; grove declines with a hint rather than take it over.
- **`done` agent state** comes from unit and golden tests only; `pane
  report-agent` cannot set it (herdr derives it), and live runs drove
  `blocked`/`idle`/`working`.
- **Windows**: grove ships windows builds and herdr supports Windows, but the
  integration is untested there; the manifest declares `linux` and `macos`
  only.

---

## Risks

**Herdr is young and moving fast.** Mitigated by lenient decoding, captured
fixtures, doctor's protocol check, and a CI job pinned to a herdr release —
bump `HERDR_VERSION` and `HERDR_SHA256` together in `.github/workflows/ci.yml`
to test a new one.

**Two sources of truth for worktrees.** Mitigated by the ownership rule above,
the plugin's hooks (prompt and marker on create/open, state sync on remove),
and never calling herdr's mutating worktree verbs. Stated loudly in `AGENTS.md`
so agents don't "helpfully" reach for `herdr worktree create`.

**Upside:** herdr runs on Windows, while grove's tmux path is unix-only — a
plausible future route to Windows support, and a reason not to hard-code unix
assumptions into `internal/mux`.
