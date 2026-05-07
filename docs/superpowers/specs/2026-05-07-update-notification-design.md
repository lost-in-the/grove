# Update Notification — Design Spec

**Status**: Approved (brainstormed 2026-05-07)
**Closes**: [#35](https://github.com/lost-in-the/grove/issues/35)
**Author**: Leah Armstrong (synthesized via AI brainstorming)

## Overview

Print a single-line notice to stderr when a newer grove release is available, after the user's command output completes. Check at most once per 24 hours, in a detached background process so the user's command isn't slowed down. Skip in CI, non-TTY contexts, and when the user has opted out via env var or flag.

This spec covers the CLI experience plus a simple TUI-exit fallback. A richer in-TUI experience (status-bar badge + user-triggered modal via `u` keybind, per the lazygit pattern) is **out of scope** for this PR — see Out of Scope below.

## Goals

- Users on stale versions notice an upgrade is available within ~24h of release
- Zero perceptible cost on every command — the check fires asynchronously, the notification reads from cache
- Easy opt-out via env var, per-run flag, and (eventually) config
- No background daemons; reuse the natural cadence of grove invocations
- The notification is visually distinct (npm-style box) but never blocks the user

## Non-goals (out of scope for this PR)

- Rich TUI experience: persistent status-bar badge + user-triggered keybind modal. The research doc identifies this as the correct TUI pattern; it deserves its own design (longer interval, different placement, modal component, keybind plumbing). File as a separate issue.
- `--version` output annotation when an update is available. Touches more files; defer.
- Auto-update mechanism. Manual update only — we just inform.
- Pre-release / RC tracking. Only stable releases (GitHub `releases/latest` returns the latest non-prerelease).
- Per-user config file (`~/.grove/config.toml`). Grove currently has per-project config only; introducing per-user config is its own concern. v1 ships with env-var opt-out only.

## Architecture

### Package layout

```
internal/updatecheck/
├── check.go        # public API: MaybeNotify, RefreshAsync, CheckNow, Skip
├── cache.go        # ~/.grove/update-check.json read/write (atomic)
├── skip.go         # CI / TTY / env-var / version detection
├── github.go       # api.github.com/repos/lost-in-the/grove/releases/latest
├── install.go      # detect brew vs go-install vs binary
├── render.go       # box rendering with semver-severity color
└── *_test.go       # one _test.go per source file
```

### Public API

```go
// Skip returns true when update checking should be entirely suppressed
// (CI, non-TTY stderr, dev/unknown version, opt-out env vars or flag).
func Skip(noUpdateNotifierFlag bool) bool

// MaybeNotify reads the cache and prints a notification to w if the cached
// latest_version is newer than currentVersion. Always non-blocking. No-op if
// the cache is missing or stale.
func MaybeNotify(w io.Writer, currentVersion string)

// RefreshAsync starts a detached goroutine that fetches the latest GitHub
// release with a 2s timeout and atomically writes the result to the cache.
// Never blocks. Failures are silent — the cache stays at its previous state
// so the next invocation will retry.
func RefreshAsync(currentVersion string)

// CheckNow synchronously fetches and prints. Used by --check-update flag.
// Returns an error if the HTTP fetch fails (caller exits non-zero in that case).
func CheckNow(w io.Writer, currentVersion string) error
```

### Wiring

In `cmd/grove/commands/root.go`:

```go
var noUpdateNotifierFlag bool

var rootCmd = &cobra.Command{
    // ... existing fields ...
    PersistentPostRunE: func(cmd *cobra.Command, args []string) error {
        if updatecheck.Skip(noUpdateNotifierFlag) {
            return nil
        }
        updatecheck.MaybeNotify(os.Stderr, version.Version)
        updatecheck.RefreshAsync(version.Version)
        return nil
    },
}

func init() {
    rootCmd.PersistentFlags().BoolVar(&noUpdateNotifierFlag, "no-update-notifier", false,
        "suppress the new-release notification on this run")
    // --check-update is added as its own subcommand or persistent flag (see Flag UX below)
}
```

The TUI exit path is handled implicitly: `rootCmd.RunE` returns after `tui.Run()` returns; cobra fires `PersistentPostRunE` next; the box prints to stderr after the alt-screen has been released.

## Data flow

```
┌────────────────────────────────────────────────────────────────────────┐
│  User runs `grove ls` (or any subcommand, or bare `grove` for TUI)     │
└────────────────────────────────────────────────────────────────────────┘
                              │
                              ▼
            ┌──────────────────────────────────┐
            │ command's RunE produces output   │
            │ to stdout (or alt-screen TUI)    │
            └──────────────────────────────────┘
                              │
                              ▼
            ┌──────────────────────────────────┐
            │ PersistentPostRunE fires         │
            └──────────────────────────────────┘
                              │
                              ▼
                       ┌──────────────┐
                       │  Skip()?     │── yes ──► return (silent)
                       └──────────────┘
                              │ no
                              ▼
            ┌──────────────────────────────────┐
            │ MaybeNotify(stderr, current ver) │
            │   ├─ read ~/.grove/update-check  │
            │   ├─ if newer cached version,    │
            │   │    render box to stderr      │
            │   └─ else no-op                  │
            └──────────────────────────────────┘
                              │
                              ▼
            ┌──────────────────────────────────┐
            │ RefreshAsync(current ver)        │
            │   ├─ launch detached goroutine   │
            │   │    ├─ HTTP GET releases/     │
            │   │    │    latest (2s timeout)  │
            │   │    └─ atomic write cache     │
            │   └─ return immediately          │
            └──────────────────────────────────┘
                              │
                              ▼
                          (cmd exits)
```

The detached goroutine MAY be killed by process exit before the cache write completes. That is acceptable — the cache simply stays at its previous state and the next invocation retries.

## First-run behavior

The first time a user invokes grove:
1. Cache file doesn't exist → `MaybeNotify` reads, finds no cache, no-ops
2. `RefreshAsync` launches, HTTP succeeds, writes cache with current `latest_version`
3. User sees no notification (correct — we don't have one to show yet)

The second time the user invokes grove:
1. Cache exists → `MaybeNotify` reads, finds `latest_version > currentVersion` → renders box
2. `RefreshAsync` runs again only if `last_checked_at` is older than the interval

This naturally gives the "first-run grace" behavior the research doc calls for, with no explicit `first_run_completed` flag needed in the cache schema.

## Cache schema

Path: `~/.grove/update-check.json` (matches the existing `~/.grove/grove.log` location).

```json
{
  "version": 1,
  "last_checked_at": "2026-05-07T12:34:56Z",
  "latest_version": "0.6.0",
  "latest_url": "https://github.com/lost-in-the/grove/releases/tag/v0.6.0"
}
```

- `version: 1` — schema version, lets us migrate later without breaking older caches
- `last_checked_at` — ISO 8601, used to gate `RefreshAsync` to once per interval
- `latest_version` — bare semver (no `v` prefix), as returned by GitHub Releases API
- `latest_url` — direct link to the release page, included in the notification box

**Atomic writes**: write to `update-check.json.tmp`, then `os.Rename` to final path. Tolerate a missing or corrupt cache by treating it as "no cache" (same as a fresh install).

## Skip detection matrix

| Source | Condition | Rationale |
|---|---|---|
| ENV | `CI=true` | CI can't act on a notification; clutters logs |
| ENV | `GITHUB_ACTIONS=true` | Common CI env var that doesn't always set CI=true |
| ENV | `BUILDKITE=true` | Same |
| ENV | `CIRCLECI=true` | Same |
| ENV | `TRAVIS=true` | Same |
| ENV | `GROVE_AGENT_MODE=1` | Agent mode already suppresses interactive features |
| ENV | `GROVE_NONINTERACTIVE=1` | Explicit non-interactive opt-out |
| ENV | `NO_UPDATE_NOTIFIER=1` | sindresorhus/update-notifier convention; honor for interop |
| ENV | `GROVE_NO_UPDATE_NOTIFIER=1` | Project-specific opt-out |
| FLAG | `--no-update-notifier` | Per-invocation override |
| TTY | `term.IsTerminal(int(os.Stdout.Fd())) == false` | Piped/scripted invocations must stay clean. Stdout is the right check (matches npm convention): correctly suppresses during `eval "$(grove install zsh)"`, when piping subcommands like `grove ls --json | jq`, and when grove emits shell-wrapper directives that get captured by the wrapper. The notification *output* still goes to stderr — but the *gating* is on stdout. |
| VERSION | `version.Version == "unknown"` | Dev binary has no defined release |
| VERSION | `version.Version` ends in `-dev` (e.g. `0.7.0-dev`) | Default for unreleased local builds — see `internal/version/version.go` |
| VERSION | `version.Version` doesn't parse as semver | Custom-built binary |

The version-suffix check `-dev` is a simple string suffix test; we don't need a full prerelease parser for v1 since the only `-X` suffix grove ships with is `-dev`.

## Notification format

Plain-text rendering (no color):

```
╭───────────────────────────────────────────────────────────╮
│ Update available  0.5.0  →  0.6.0                         │
│ Run: brew upgrade lost-in-the/tap/grove                   │
│ Changelog: https://github.com/lost-in-the/grove/releases  │
╰───────────────────────────────────────────────────────────╯
```

Rendered via lipgloss (already a TUI dependency, so no new go.mod entry). When `NO_COLOR=1` is set or the terminal lacks color support, fall back to plain ASCII corners (`+--+ | | +--+`) for maximum compatibility.

**Severity coloring** by semver bump from current → latest:
- Major (X.y.z → (X+1).y.z): red border
- Minor (x.Y.z → x.(Y+1).z): yellow border
- Patch (x.y.Z → x.y.(Z+1)): dim/gray border

The box width adapts to fit the longest line of content. Output goes to stderr (never stdout).

## Install detection

```go
type InstallMethod int

const (
    InstallUnknown InstallMethod = iota
    InstallBrew
    InstallGoInstall
    InstallBinary
)

func DetectInstall() InstallMethod
```

Detection uses `os.Executable()` to get the running binary's path:
- Path under `/opt/homebrew/Cellar/grove/` or `/usr/local/Cellar/grove/` → `InstallBrew`
- Path under any directory ending in `/go/bin/` → `InstallGoInstall`
- Anything else → `InstallBinary` (catchall — could be manual download, custom path, etc.)

Update command per method (rendered as the box's "Run:" line):
- `InstallBrew` → `brew upgrade lost-in-the/tap/grove`
- `InstallGoInstall` → `go install github.com/lost-in-the/grove/cmd/grove@latest`
- `InstallBinary` → `Visit https://github.com/lost-in-the/grove/releases for the latest binary`
- `InstallUnknown` → fallback to the binary message

## Flag UX

Two new persistent flags on `rootCmd`:

```
--no-update-notifier   Suppress the update-available notification on this run.
                       Also honors NO_UPDATE_NOTIFIER and GROVE_NO_UPDATE_NOTIFIER env vars.

--check-update         Force a synchronous check now and print the result. Bypasses
                       the 24h cooldown. Useful for testing or on-demand verification.
                       Exits 0 even when up-to-date; exits non-zero only on HTTP failure.
```

`--check-update` triggers a synchronous fetch (not the cache-then-detached-refresh pattern), prints the box if newer, prints `grove is up to date (0.6.0)` if not. It's a normal cobra subcommand or persistent flag — pick whichever has lower ceremony in the existing codebase. Recommendation: use `grove update --check` as a subcommand of a future `grove update` command, OR a persistent flag for v1 simplicity. **v1: persistent flag.**

## Error handling

| Scenario | Behavior |
|---|---|
| Cache file missing | Treat as fresh install; no notification; refresh writes cache |
| Cache file corrupt JSON | Treat as missing; log to grove.log if `GROVE_LOG=1` |
| Cache schema version mismatch (future) | Treat as missing; refresh overwrites |
| HTTP timeout (>2s) | Silent failure; cache not updated; next invocation retries |
| HTTP 4xx/5xx | Silent failure; cache not updated |
| Atomic write failure | Silent failure; cache stays at previous state |
| Detached goroutine killed by process exit before cache write | Acceptable; cache stays at previous state; next invocation retries |
| `os.Executable()` fails | `DetectInstall` returns `InstallUnknown` → use fallback message |
| GitHub rate limit (60/h unauth) | Silent failure; cache not updated; next invocation retries |

Failures are deliberately silent because the user didn't ask for an update check — we're offering it. Spamming errors when the network's flaky would be worse than the silent retry.

## Testing strategy

| Aspect | Test approach |
|---|---|
| Skip detection matrix | Table-driven, mock env vars + TTY + version string |
| Cache atomic write | Tempdir tests; assert tmp + rename, byte-for-byte content |
| Cache corrupt-tolerance | Write garbage JSON; verify graceful degradation, no panic |
| GitHub API client | `httptest.Server`; assert URL path, headers, 2s timeout |
| Semver compare | Table-driven for major/minor/patch + invalid input + edge cases (0.0.x, x.y.0) |
| Install detection | Stub `os.Executable` via test helper; verify each branch |
| Render plain | Snapshot test of plain-text output (NO_COLOR=1) |
| Render colored | Lipgloss path tested by asserting ANSI escape sequences are present (not full visual match) |
| End-to-end | Tempdir HOME + httptest + asserted-TTY → invoke `MaybeNotify`+`RefreshAsync`, verify cache state and stderr |

The package writes its own `info_test.go` files alongside each source file (Go convention: `cache_test.go` next to `cache.go`). Total expected test coverage: ~80% line, with the rest being trivial getters and deferred goroutine cleanup.

## Files affected

**New** (in `internal/updatecheck/`):
- `check.go`, `cache.go`, `skip.go`, `github.go`, `install.go`, `render.go`
- One `*_test.go` per source file

**Modified**:
- `cmd/grove/commands/root.go` — add `PersistentPostRunE`, two persistent flags
- `CHANGELOG.md` — entry under `[Unreleased] ### Added`
- `docs/CONFIGURATION_REFERENCE.md` — document the new env vars and flags

## CHANGELOG entry

Under `[Unreleased] ### Added`:

> - Optional update-available notification on command exit when a newer grove release is published. Checks at most once per 24 hours, in a detached background process — never blocks command execution. Suppressed in CI, non-TTY contexts, and when `NO_UPDATE_NOTIFIER`, `GROVE_NO_UPDATE_NOTIFIER`, `GROVE_AGENT_MODE`, or `GROVE_NONINTERACTIVE` env vars are set, or when `--no-update-notifier` is passed. Use `--check-update` to force a synchronous check at any time.

## Acceptance criteria

- [ ] New `internal/updatecheck/` package with the four public functions, full unit test coverage
- [ ] Wired into `rootCmd.PersistentPostRunE`; runs after every command including the TUI
- [ ] Box renders with correct lipgloss styling and falls back gracefully when `NO_COLOR=1`
- [ ] All skip conditions verified by table-driven tests
- [ ] First run produces no notification but does populate the cache (verified end-to-end)
- [ ] Cache atomic-write verified with concurrent-write stress test (best effort) or at minimum a tempfile-then-rename verification test
- [ ] Brew, go-install, and binary install detection all tested
- [ ] `--no-update-notifier` flag and `--check-update` flag wired and tested
- [ ] CHANGELOG entry present
- [ ] `docs/CONFIGURATION_REFERENCE.md` updated with new env vars
- [ ] `make lint test` clean

## Open follow-ups (post-merge issues to file)

1. **Rich TUI update experience** — passive status-bar badge + `u`-keybind modal (per the lazygit pattern in the research doc). Different cadence (7-14 day check), different rendering, requires bubbletea component work.
2. **`--version` output annotation** — show "(update available: X.Y.Z)" appended to `grove --version` output when the cache indicates a newer release. Small but touches the version command.
3. **Per-user config file** (`~/.grove/config.toml`) — would let users set `[update_check] interval = "12h"` etc. without env vars. Bigger design conversation about whether grove wants per-user config.

## References

- [npm/update-notifier](https://github.com/yeoman/update-notifier) — canonical CLI implementation
- [oclif/plugin-warn-if-update-available](https://github.com/oclif/plugin-warn-if-update-available) — enterprise CLI reference
- [lazygit](https://github.com/jesseduffield/lazygit) — TUI reference (lazygit checks every 14 days, prompts via `u` keybind in status panel, suppresses for package-manager installs)
- [Command Line Interface Guidelines](https://clig.dev) — general CLI UX principles
- Issue [#35](https://github.com/lost-in-the/grove/issues/35)
