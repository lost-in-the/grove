# Herdr Integration — Design & Implementation Plan

**Status:** Implemented and verified against herdr **0.8.0** (protocol 19)
running headless. `[mux] backend = "herdr"` selects it; `auto` picks it when
grove is running inside a herdr pane.

[Herdr](https://herdr.dev) is a terminal multiplexer built for coding agents: a
background server owns real terminals, clients attach to render them, and panes
survive detach. Same shape as tmux, but mouse-first and agent-aware — it detects
coding agents in panes and tracks `working` / `blocked` / `done` / `idle` state.

This document plans herdr support as a second multiplexer backend alongside tmux.

Findings below were verified against herdr's source
([herdrdev/herdr](https://github.com/herdrdev/herdr) @ `2863b71`), not just its
docs. Source references use that revision.

---

## The strategic question: herdr already does worktrees

Herdr ships `herdr worktree list|create|open|remove`. Worktrees are "normal Herdr
workspaces with Git checkout provenance." `worktree create` makes the checkout,
opens it as a workspace, and groups it under the parent repo — and without
`--path` it places checkouts at `<worktrees.directory>/<repo>/<branch-slug>`.

That overlaps grove's core purpose, so the boundary has to be explicit:

> **Grove owns worktree lifecycle. Herdr is only a session/display backend.**

Grove creates the checkout using its own `[naming] pattern` and `projects_dir`,
then calls `herdr worktree open` to *adopt* it.

Verified: `handle_worktree_open` (`src/app/api/worktrees.rs:75`) resolves the path
against the repo's **existing** git worktree list and returns `worktree_not_found`
if it isn't registered. It never creates a checkout. Adoption is safe.

**`worktree open` takes two different directories, and they are not
interchangeable.** `--cwd` (or `--workspace`) names the *source repository*
whose worktree list is searched; `--path` names the checkout to open. Passing
the linked worktree as `--cwd` fails:

```
{"error":{"code":"linked_worktree_source",
  "message":"New and open worktree actions start from the repo parent workspace."}}
```

and omitting `--cwd` entirely falls back to the focused workspace, which does
not exist when grove runs outside a client:

```
{"error":{"code":"invalid_request",
  "message":"workspace_id or cwd is required when no workspace is active"}}
```

So `mux.Target` carries `Repo` — the repository's main checkout — alongside
`Path`. Sessions that already exist (kill, focus, rename) resolve without it.

**Grove must never call `herdr worktree create` or `herdr worktree remove`.**
`worktree create` would hand naming and placement to herdr, breaking the
canonical `{project}-{name}` rule. `worktree remove` shells out to
`git worktree remove`, bypassing grove's protection rules
(`[protection] protected` / `immutable`). Grove's removal path stays grove's;
the herdr-side cleanup is `workspace close`, which closes herdr state and panes
only and leaves the checkout on disk.

---

## Key finding: identity is the checkout path, not the name

Grove's tmux model is name-keyed throughout: `CreateSession(name, path)` is
idempotent, `SessionExists(name)`, `KillSession(name)`. Herdr workspaces have
opaque IDs (`w1`) plus a **non-unique** label, and there is no lookup-by-label —
which initially looks like a serious impedance mismatch.

It isn't, because herdr exposes the checkout path directly:

```rust
// src/api/schema/workspaces.rs:52
pub struct WorkspaceInfo {
    pub workspace_id: String,
    pub label: String,
    pub focused: bool,
    pub agent_status: AgentStatus,
    pub tokens: HashMap<String, String>,
    pub worktree: Option<WorkspaceWorktreeInfo>,  // .checkout_path
}
```

`workspace_info` (`src/app/creation.rs:492`) populates `worktree` from
`ws.worktree_space()` for any workspace whose cwd is a git checkout. So a single
`herdr workspace list` returns, for every workspace: id, label, focus, **checkout
path**, and **agent status**.

Grove already knows the checkout path of every worktree. So:

- **Identity key = canonicalized checkout path.** No label matching, no
  reconciliation table, no metadata tokens needed.
- **One call replaces `tmux list-sessions`** and feeds `grove ls`, the TUI, and
  every `SessionExists` check.
- `label` becomes purely cosmetic — it carries the canonical `{project}-{name}`
  for display, and a user renaming a workspace cannot desync grove.

`herdr worktree open` is also **idempotent**: it checks
`open_workspace_idx_for_checkout` first, reuses the existing workspace, re-applies
`--label`, and reports `already_open: true`. That maps exactly onto grove's
idempotent `CreateSession`. Confirmed live — a second open of the same checkout
returns `already_open: true` with the same workspace id.

The **main checkout** works through the same call: `worktree open --cwd R
--path R` reuses the repo's parent workspace and reports `already_open: true`.
The `workspace create` fallback grove keeps for `worktree_not_found` /
`not_git_worktree` is therefore a safety net for non-worktree directories, not
the main-checkout path.

**Caveat: herdr's workspace paths do not follow a rename.** herdr captures a
workspace's paths when it opens and never updates them, so `grove rename` leaves
*all three* stale at once — `worktree.checkout_path`, the workspace cwd, and the
pane's cwd all name the pre-rename directory. Verified live.

Grove's own lookups self-heal: `mux.Index` falls back to the session name, and
`grove rename` has just relabelled the workspace to match, so `grove ls` and
`grove to` keep resolving the worktree correctly.

The plugin cannot self-heal the same way — it only has herdr's context, and
every path in it is dead. It says so plainly rather than reporting the dead path
as "not a grove project"; the fix is to close or reopen the workspace. The stale
pane cwd is the same situation tmux has after a rename, which
`[tmux] on_switch = "reset"` already handles.

Canonicalize before comparing — herdr uses `canonical_or_original`, so grove
should `filepath.EvalSymlinks` on both sides.

`workspace report-metadata --token NAME=VALUE` exists and would let grove stamp
its own identity tokens (surfaced as `WorkspaceInfo.tokens`). It is **not needed**
given path keying; noted only as an escape hatch if path keying ever proves
insufficient.

---

## Capability gaps

### Popup requires the plugin surface

Grove's `session` command uses `tmux display-popup`. Herdr has no equivalent CLI
verb — but it does have popup placement, reachable **only** by registering a
plugin with a declared pane:

```rust
// src/api/schema/plugins.rs:445
pub enum PluginPanePlacement { Overlay, Popup, Split, Tab, Zoomed }
```

This is why the herdr plugin below is load-bearing rather than a nice-to-have:
it is the only route to popup/overlay placement, and therefore the only way
`grove session` and the grove TUI keep working as overlays under herdr.

### No control mode

`ShouldUseControlMode` / `AttachSessionControlMode` (iTerm2 `tmux -CC`) is
tmux-specific. The herdr backend simply won't implement it.

### Attach is two steps

`herdr`'s launch flags are `--session`, `--remote`, `--no-session`,
`--default-config`, `--skill` (`src/main.rs:712`). There is **no**
`--workspace` / `--cwd` targeting flag, so attaching to a specific worktree is:

```
herdr workspace focus <id>   # socket call, works before attaching
herdr                        # blocking attach
```

Nested launches are blocked by design — `should_block_nested`
(`src/main.rs:442`) refuses when `HERDR_ENV=1` unless
`experimental.allow_nested` is set. So from inside a pane, grove must use
`workspace focus` alone. That is a clean analogue of the existing
`SwitchSession` path.

---

## What grove gains

Not just parity. `WorkspaceInfo.agent_status` is a rollup over the workspace's
panes, with values `Idle | Working | Blocked | Done | Unknown`
(`src/api/schema/common.rs:151`).

That means `grove ls` and the TUI dashboard can show **which worktree has an
agent waiting on input** — the single most useful piece of state in a
multi-worktree agent workflow, and something tmux cannot provide at any price.
It arrives free in the same `workspace list` call grove already needs.

One thing the schema does not tell you: **`unknown` is the no-agent default**,
not a rare "present but unclassified" case. A workspace whose panes are all
sitting at a shell prompt rolls up to `unknown`. So grove treats only
`idle`/`working`/`blocked`/`done` as an actual sighting
(`AgentStatus.Observed()`); otherwise the AGENT column would render the word
"unknown" on every row of every listing.

---

## Architecture

Grove has no multiplexer abstraction today; `internal/tmux` is imported directly
by 19 files. The package is small (436 LOC, 20 exported functions) and real usage
concentrates in `cmd/grove/commands/to.go`, `helpers.go`, and
`internal/tui/model.go`, so extraction is mechanical.

### `internal/mux`

```go
// Target identifies a worktree session in backend-neutral terms.
// Name is the canonical {project}-{name}; Path is the checkout path.
// tmux keys on Name, herdr keys on Path — both are always supplied.
type Target struct {
    Name string
    Path string
}

type Status string // "attached", "detached", "none"

type AgentStatus string // "idle", "working", "blocked", "done", "unknown", ""

type Session struct {
    Name    string
    Path    string
    Status  Status
    Agent   AgentStatus // "" when the backend cannot report it
    Windows int
}

type Multiplexer interface {
    Name() string                                  // "tmux" | "herdr"
    Available() bool                               // binary present
    Inside() bool                                  // running inside this mux

    Ensure(Target) error                           // idempotent create/adopt
    Exists(Target) (bool, error)
    List() ([]Session, error)
    Current() (string, error)

    Attach(Target) error                           // blocking
    Switch(Target) error                           // from inside
    Rename(old, new Target) error
    Kill(Target) error

    PaneInfo(Target) (*PaneInfo, error)
    SendCommand(Target, string) error
}
```

Optional capabilities are separate interfaces, checked at the call site — this is
how the popup gap stays explicit rather than becoming a silent no-op:

```go
type Popuper interface{ Popup(t Target, width, height string) error }
type ControlModer interface{ AttachControlMode(t Target) error }
```

```go
if p, ok := m.(mux.Popuper); ok {
    return p.Popup(target, w, h)
}
// otherwise fall back to a full-window attach and say so
```

### Backends

`internal/mux/tmux` wraps today's `internal/tmux` unchanged and implements
`Popuper` + `ControlModer`.

`internal/mux/herdr` shells out to `herdr` through `cmdexec` with a new `Herdr`
timeout category. Responses are single-line JSON:

```json
{"id":"cli:workspace:list","result":{"workspaces":[…]}}
{"id":"cli:workspace:list","error":{"code":"server_not_running","message":"…"}}
```

Errors go to stderr with exit 1; CLI syntax errors exit 2
(`src/api/schema/response.rs:25`). Decode leniently — ignore unknown fields so
herdr releases don't break grove.

| `Multiplexer` method | herdr implementation |
|---|---|
| `Available` | `exec.LookPath("herdr")`, cached like `IsTmuxAvailable` |
| `Inside` | `HERDR_ENV=1` |
| `Ensure` | `worktree open --path P --label N --no-focus`; fall back to `workspace create --cwd P --label N --no-focus` when P is not a registered worktree (e.g. the main checkout) |
| `List` | `workspace list` → map `worktree.checkout_path` → `Session` |
| `Exists` | lookup in `List` by canonical path |
| `Current` | `$HERDR_WORKSPACE_ID` → `workspace get` |
| `Attach` | `workspace focus <id>`, then exec `herdr` (blocking) |
| `Switch` | `workspace focus <id>` |
| `Rename` | `workspace rename <id> <label>` |
| `Kill` | `workspace close <id>` — **never** `worktree remove` |
| `PaneInfo` | `pane list --workspace <id>` |
| `SendCommand` | `pane run <root-pane-id> "<cmd>"` |

`List` is called once per grove invocation and cached, mirroring
`loadTmuxSessions()`. A dead socket fails fast with `server_not_running`
(`src/cli/server_not_running.rs`) rather than hanging, so the <500ms budget holds.

**Cold start.** `workspace focus` needs a live server. `herdr server` (bare) runs
headless, so grove can spawn it detached and poll for the socket. Simpler first
cut: if no server is running, print the actionable error herdr already provides
and let the user run `herdr`. Start there; add auto-spawn only if it proves
annoying.

### Config

```toml
[mux]
backend = "auto"   # auto | tmux | herdr | off
```

`auto` resolves: `HERDR_ENV=1` → herdr; `TMUX` → tmux; else first available
binary, tmux winning ties for backward compatibility.

The existing `[tmux]` block (`mode`, `prefix`, `on_switch`, `control_mode`) stays
as a deprecated alias — `mergeTmuxConfig` already provides the machinery, and
`tmux.mode = "off"` must keep mapping to `mux.backend = "off"`.

Naming rule extension for CLAUDE.md: *tmux session names and herdr workspace
labels both always use the canonical `{project}-{name}`, regardless of the
directory pattern.*

---

## The herdr plugin

**Viable and worth doing** — and not optional, since it is the only route to
popup placement.

Herdr plugins are a `herdr-plugin.toml` manifest plus executables in any
language; herdr keeps per-plugin config/state dirs and does not sandbox
execution. Install is `herdr plugin install <owner>/<repo>[/<subdir>]` or
`herdr plugin link <path>` for development.

```toml
id = "lost-in-the.grove"
name = "Grove"
version = "0.1.0"
min_herdr_version = "0.7.0"
description = "Worktree flow manager"
platforms = ["linux", "macos"]

# Popup placement — the reason this plugin exists.
[[panes]]
id = "dashboard"
title = "Grove dashboard"
placement = "popup"
width = "80%"
height = "80%"
command = ["grove", "tui"]

[[actions]]
id = "new"
title = "Grove: new worktree"
contexts = ["workspace"]
command = ["grove", "herdr-action", "new"]

[[actions]]
id = "switch"
title = "Grove: switch worktree"
contexts = ["workspace", "global"]
command = ["grove", "herdr-action", "switch"]

# Close the loop when the user drives worktrees from herdr's own UI.
[[events]]
on = "worktree.opened"
command = ["grove", "herdr-event"]

[[events]]
on = "workspace.closed"
command = ["grove", "herdr-event"]
```

Verified manifest shapes: `PluginManifestPane`, `PluginManifestAction`,
`PluginManifestEventHook` (`src/api/schema/plugins.rs:244–279`). Action contexts
are `Global | Workspace | Tab | Pane | Selection` (`:355`).

Plugin processes receive `HERDR_PLUGIN_CONTEXT_JSON` (and
`HERDR_PLUGIN_EVENT_JSON` for hooks) carrying `workspace_id`, `workspace_label`,
`workspace_cwd`, and the full `worktree` block with `checkout_path` and
`repo_root` — everything grove needs, with no extra socket round-trip.

Available event hooks include `worktree.created`, `worktree.opened`,
`worktree.removed`, `workspace.closed`, `workspace.renamed`, and
`pane.agent_status_changed` (`src/api/schema/events.rs:194`).

Three things the plugin buys:

1. **Popup/overlay placement** — restores `grove session` and hosts the TUI.
2. **Mouse-first entry points** — right-click a workspace for grove actions,
   matching how herdr users actually work.
3. **Overlap reconciliation** — if a user creates a worktree through *herdr's*
   UI, the `worktree.opened` hook lets grove notice it, run its own hooks, and
   keep the docker/tracker plugins in sync. This is what makes the two tools
   coexist instead of quietly diverging.

**Repo placement:** not `plugins/` — that directory means *grove* plugins
(docker, tracker) and the direction is inverted here. Use `integrations/herdr/`,
with its own README per the existing plugin convention.

---

## What shipped

**`internal/mux`** — the interface above, plus `mux.Index`, the last-session
store (moved out of `internal/tmux`, which is now purely the low-level tmux
wrapper), and three backends: `TmuxBackend`, `HerdrBackend`, `OffBackend`.
`OffBackend` means callers hold a non-nil multiplexer unconditionally and never
branch on "is there one".

**Call sites** — all 19 former `internal/tmux` importers now go through
`ctx.Mux()` (commands) or `muxFor(cfg)` (TUI, which has no `GroveContext`).

**Config** — `[mux] backend`, validated, merged, defaulted to `auto`.
`Config.EffectiveMuxBackend()` folds in legacy `tmux.mode = "off"`.

**Agent status** — `mux.AgentStatus` flows into `grove ls` (an AGENT column and
a JSON `agent` field, both appearing only when a backend reports one) and the
TUI (a row badge and a detail row).

**Plugin** — `integrations/herdr/` with the popup dashboard pane, a workspace
status action, and a `worktree.opened` hook, backed by the hidden
`grove herdr-action` / `grove herdr-event` subcommands.

**Doctor** — optional `herdr` binary check, plus a server-reachability check
that runs only when herdr is the resolved backend.

### Still to do

- **Golden files.** The TUI agent badge has unit tests but no VHS/golden
  coverage (see [VISUAL_TESTING.md](VISUAL_TESTING.md)); capturing those needs a
  coding agent running in a herdr pane to produce a non-`unknown` state.
- **Backend-parameterized integration tests.** The three tests in
  `tests/integration/` still shell out to tmux directly.

### Testing

The herdr backend takes an injectable runner, so its whole command contract is
tested against canned JSON without a herdr server: envelope decoding (including
noise on the stream and unknown fields), path-keyed resolution, the
`worktree open` → `workspace create` fallback, and the two negative assertions
that matter most — `Ensure` never calls `worktree create`, `Kill` never calls
`worktree remove`.

Fixtures are shaped from the real schema: compact single-line JSON, the
internally-tagged `"type"` field, and `agent_status` in snake_case.

Not yet covered: the three integration tests that shell out to tmux
(`tests/integration/`) are still tmux-only and would need backend
parameterization to exercise herdr end to end.

### Docs

~290 tmux mentions across 13 files. Heaviest:
[COMMAND_SPECIFICATIONS.md](COMMAND_SPECIFICATIONS.md) (73),
[AGENT_GUIDE.md](AGENT_GUIDE.md) (41),
[CONFIGURATION_REFERENCE.md](CONFIGURATION_REFERENCE.md) (27),
[DATA_FLOWS.md](DATA_FLOWS.md) (28). This is the real cost of the work — plan it
as a phase, not a cleanup.

---

## Risks

**Herdr is young and moving fast.** The CLI surface may shift under us. Mitigate
with a `min_herdr_version` floor checked in `doctor`, lenient JSON decoding, and
keeping the backend opt-in until it settles.

**Two sources of truth for worktrees.** Mitigated by the ownership rule above,
the `worktree.opened` hook, and never calling herdr's mutating worktree verbs.
Worth stating loudly in `AGENTS.md` so agents don't "helpfully" reach for
`herdr worktree create`.

**Popup regression.** Between Phase 2 and Phase 4, `grove session` degrades to a
full-window attach under herdr. Acceptable if the fallback message is explicit.

**Upside worth noting:** herdr has a Windows preview, while grove's tmux path is
unix-only. A herdr backend is a plausible future route to Windows support. Out of
scope here, but it argues for not hard-coding unix assumptions into `internal/mux`.

---

## Verification

Run against herdr 0.8.0 (protocol 19) with `herdr server` headless, driving a
real grove project with two worktrees. Everything below was executed, not
reasoned about.

These checks are automated in [`scripts/validate-herdr.sh`](../scripts/validate-herdr.sh):

```bash
herdr server &                 # or attach a client with `herdr`
scripts/validate-herdr.sh      # --keep leaves the scratch project behind
```

It builds grove from the checkout, works entirely in a throwaway repo under
`$TMPDIR`, and exits non-zero on any failure. Its last check stops the herdr
server, so don't point it at one you're using.

| Flow | Result |
|---|---|
| `grove new` | creates the checkout, adopts it as a workspace, labels it `demo-feature-a` |
| `grove ls` | resolves every worktree **by checkout path**; attached/detached correct |
| `grove rm` | git removes the checkout, `workspace close` drops the session; checkout deleted by grove, not herdr |
| `grove rename` | relabels the workspace; later lookups self-heal via the name fallback |
| `grove to` (inside herdr) | focuses the right workspace, does not attach |
| Plugin manifest | `herdr plugin link` accepts it; actions, events, and panes all parse |
| Plugin action | `Grove: worktree status` runs and reports correctly |
| Event hook | fires on a worktree grove doesn't track; stays silent on one it does |
| Dead server | `server_not_running`; `grove ls` degrades to "no session", no hang |

Three bugs surfaced that no amount of source reading had caught:

1. **`worktree open` needs `--cwd <repo root>`.** Every session creation failed
   with `invalid_request` until `Target.Repo` was threaded through. This was the
   real one — the feature did not work at all without it.
2. **`unknown` is the no-agent default**, so the AGENT column rendered "unknown"
   on every row. Fixed with `AgentStatus.Observed()`.
3. **herdr's checkout provenance goes stale after a rename**, so the plugin's
   context pointed at a deleted directory. Fixed by resolving to the first
   directory that still exists.

**Latency.** `herdr workspace list` averages **3ms** against a warm server;
`grove ls` end to end averages **52ms**, comfortably inside the 500ms budget.
The listing is fetched once per command and shared through `mux.Index`.

### Still unverified

- **`Attach` (focus-then-exec).** `Switch` is verified; the full attach path
  needs a TTY, which a headless server cannot provide. The ordering it depends
  on — focus before attach — is verified as a socket call.
- **Named sessions.** `HERDR_SESSION` scoping is untested; grove currently talks
  to whichever session the environment selects, which may need explicit scoping
  if a user runs several.
- **Real agent states.** `blocked`/`working`/`done` were exercised through unit
  tests and a stub, not by running a coding agent inside a pane, so the golden
  files for the TUI badge are still uncaptured.
- **macOS.** CI runs the full unit and integration suites on macOS and they pass,
  but the live herdr flows above were exercised only on Linux — CI runners have
  no herdr installed, so they cover the tmux backend and the herdr backend's
  unit-level contract, not a real server.
