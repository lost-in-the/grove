# Visual Testing Guide

Grove's TUI visual regression testing has three tools: golden files for deterministic
regression detection, tmux capture for live interaction verification, and VHS for demo
recordings. This guide covers when and how to use each.

---

## Golden File Tests

### How they work

Golden tests use [`charmbracelet/x/exp/golden`](https://pkg.go.dev/github.com/charmbracelet/x/exp/golden)
to compare rendered TUI output against committed reference files. On first run with
`-update`, the library writes the current output to a `.golden` file. On subsequent
runs, it reads the file and fails if the output differs.

- Golden files live in `internal/tui/testdata/`
- Files are named after the test function and subtest: `TestGolden_Dashboard/standard_80x24.golden`
- Flat tests (no subtests) use the function name directly: `TestGolden_Dashboard_Empty.golden`

### Two tiers

**Tier 1 — Structural (NO_COLOR)**

Uses `goldenModel()`. Forces `NO_COLOR=1`, applies `noColorScheme()`, and calls
`golden.RequireEqual`. These tests verify layout, alignment, column widths, and
content without any ANSI escape codes. They are the primary regression tests — easy
to read in diffs, stable across terminal themes.

```go
func TestGolden_Dashboard(t *testing.T) {
    for _, size := range allSizes {
        t.Run(size.name, func(t *testing.T) {
            m := goldenModel(t, size, withItems(5))
            golden.RequireEqual(t, []byte(m.View()))
        })
    }
}
```

**Tier 2 — Themed (with ANSI)**

Uses `goldenModelThemed()`. Applies `defaultColorScheme()` (not `AdaptiveColor`,
to avoid terminal-dependent resolution) and calls `golden.RequireEqualEscape`. These
tests verify that color application is correct — badge colors, border styles, focus
highlights. They are more brittle than structural tests and should be used sparingly.

```go
func TestGolden_Themed_Dashboard(t *testing.T) {
    m := goldenModelThemed(t, sizeStandard, withItems(5))
    golden.RequireEqualEscape(t, []byte(m.View()), true)
}
```

### Directory structure

```
internal/tui/testdata/
  TestGolden_Dashboard/
    narrow_60x24.golden
    standard_80x24.golden
    wide_120x40.golden
    ultrawide_160x40.golden
  TestGolden_Dashboard_Empty.golden
  TestGolden_Dashboard_Loading.golden
  TestGolden_Dashboard_WithToast.golden
  TestGolden_Dashboard_SortModes/
    name.golden
    recent.golden
    dirty.golden
  TestGolden_Dashboard_HelpExpanded/
    standard_80x24.golden
    wide_120x40.golden
  TestGolden_Overlay_Delete/
    default.golden
    with_warnings.golden
  TestGolden_Overlay_Create/
    branch_step.golden
    name_step.golden
    confirm_step.golden
  TestGolden_Overlay_Bulk/
    with_items.golden
    empty.golden
  TestGolden_Overlay_PRs/with_data.golden
  TestGolden_Overlay_Issues/with_data.golden
  TestGolden_Overlay_Fork/confirm.golden
  TestGolden_Overlay_Sync/source_step.golden
  TestGolden_Overlay_Config/general_tab.golden
  TestGolden_Component_Header/
    narrow_60x24.golden
    standard_80x24.golden
  TestGolden_Component_Stepper/
    step_0.golden  step_1.golden  step_2.golden  complete.golden
  TestGolden_Component_Toast/
    success.golden  warning.golden  error.golden  info.golden
  TestGolden_Component_HelpFooter/
    dashboard.golden  delete.golden  create.golden
  TestGolden_Responsive_Layout/
    width_50.golden  width_60.golden  width_80.golden
    width_100.golden  width_120.golden  width_160.golden
  TestGolden_Themed_Dashboard.golden
  TestGolden_Themed_StatusBadges.golden
  TestGolden_Themed_OverlayBorders.golden
```

### Terminal size presets

Defined in `internal/tui/golden_helpers_test.go`:

| Variable | Name | Dimensions |
|----------|------|------------|
| `sizeNarrow` | `narrow_60x24` | 60 x 24 |
| `sizeStandard` | `standard_80x24` | 80 x 24 |
| `sizeWide` | `wide_120x40` | 120 x 40 |
| `sizeUltraWide` | `ultrawide_160x40` | 160 x 40 |

`allSizes` is a slice containing all four, used by tests that validate responsive layout.

### State builder opts

All opts are functional options (`testOpt = func(*Model)`) defined in
`internal/tui/golden_helpers_test.go` and `internal/tui/helpers_test.go`.

| Opt | Source file | What it sets up |
|-----|-------------|-----------------|
| `withItems(n int)` | `helpers_test.go` | Populates the list with n `WorktreeItem` values |
| `withLoading()` | `helpers_test.go` | Sets `m.loading = true` |
| `withSize(w, h int)` | `helpers_test.go` | Sets dimensions and calls `updateLayout()` (applied automatically by `goldenModel`) |
| `withDeleteOverlay(warnings ...string)` | `golden_helpers_test.go` | Opens delete overlay with optional warning strings |
| `withCreateStep(step CreateStep)` | `golden_helpers_test.go` | Opens create overlay at `CreateStepBranch`, `CreateStepName`, or `CreateStepConfirm` |
| `withBulkOverlay(n int)` | `golden_helpers_test.go` | Opens bulk delete overlay with n items, first and third pre-selected |
| `withPRData()` | `golden_helpers_test.go` | Opens PR browser overlay with 3 mock PRs |
| `withIssueData()` | `golden_helpers_test.go` | Opens issue browser overlay with 3 mock issues |
| `withForkOverlay()` | `golden_helpers_test.go` | Opens fork overlay at `ForkStepConfirm` |
| `withSyncOverlay()` | `golden_helpers_test.go` | Opens sync overlay at `SyncStepSource` with 3 source items |
| `withConfigOverlay()` | `golden_helpers_test.go` | Opens config overlay with deterministic fields across all 4 tabs |
| `withToastVisible(msg string, level ToastLevel)` | `golden_helpers_test.go` | Shows a toast with 24h expiry (never expires during test) |
| `withHelpExpanded()` | `golden_helpers_test.go` | Expands the help footer |
| `withSortMode(mode SortMode)` | `golden_helpers_test.go` | Sets `m.sortMode` (`SortByName`, `SortByLastAccessed`, `SortByDirtyFirst`) |

### Adding a new golden test

**Step 1 — Add a state builder opt if needed**

If you need a model state that no existing opt covers, add a new `testOpt` function
to `internal/tui/golden_helpers_test.go`:

```go
// withMyOverlay sets up the my-overlay view.
func withMyOverlay() testOpt {
    return func(m *Model) {
        m.activeView = ViewMyOverlay
        m.myState = &MyState{
            // deterministic values — avoid time.Now() except with far-future durations
        }
    }
}
```

**Step 2 — Write the test**

For a structural (NO_COLOR) test:

```go
func TestGolden_MyOverlay(t *testing.T) {
    t.Run("default", func(t *testing.T) {
        m := goldenModel(t, sizeStandard, withItems(3), withMyOverlay())
        golden.RequireEqual(t, []byte(m.View()))
    })
}
```

For a themed test:

```go
func TestGolden_Themed_MyOverlay(t *testing.T) {
    m := goldenModelThemed(t, sizeStandard, withItems(3), withMyOverlay())
    golden.RequireEqualEscape(t, []byte(m.View()), true)
}
```

**Step 3 — Generate the golden file**

```bash
go test ./internal/tui/ -run TestGolden_MyOverlay -update
```

Or use the make target which updates all golden tests at once:

```bash
make test-update-golden
```

**Step 4 — Review the output**

```bash
make golden-view TEST=TestGolden_MyOverlay
```

This prints the content of every matching `.golden` file to stdout. Verify the
layout looks correct before committing.

**Step 5 — Commit the golden file**

Golden files are source of truth — commit them alongside the code that produces them:

```bash
git add internal/tui/testdata/TestGolden_MyOverlay/
git commit -m "test(tui): add golden test for MyOverlay"
```

### Updating existing golden files

After an intentional visual change, regenerate and inspect:

```bash
make golden-diff
```

This updates all golden files and pipes the result through `git diff` so you can
see exactly what changed. Review each hunk before committing.

To update a single test:

```bash
go test ./internal/tui/ -run TestGolden_Dashboard -update
```

To print a specific golden file without modifying it:

```bash
make golden-view TEST=TestGolden_Dashboard
```

### Gotchas

**No `t.Parallel()`** — `goldenModel` and `goldenModelThemed` acquire `goldenMu`
(a package-level `sync.Mutex`) because they mutate the global `Colors` and `Styles`
variables. Parallel subtests would deadlock.

**Spinner determinism** — The spinner is created but never ticked. It always renders
its initial frame (`⠋`). Never send `spinner.TickMsg` before calling `View()` in
a golden test, or the output will vary between runs.

**Time-sensitive content** — If a test renders timestamps or ages, use a far-future
duration (e.g. `24 * time.Hour`) so the display value never changes. See
`withToastVisible` for the pattern.

**Component tests** — Tests that render a single component (not a full `Model`) manage
the mutex and `NO_COLOR` env var manually. See `TestGolden_Component_Header` for the
pattern:

```go
func TestGolden_Component_Header(t *testing.T) {
    for _, size := range []termSize{sizeNarrow, sizeStandard} {
        t.Run(size.name, func(t *testing.T) {
            goldenMu.Lock()
            t.Setenv("NO_COLOR", "1")
            Colors = noColorScheme()
            Styles = NewStyleSet(Colors)
            t.Cleanup(func() {
                Colors = NewColorScheme()
                Styles = NewStyleSet(Colors)
                goldenMu.Unlock()
            })

            h := Header{...}
            golden.RequireEqual(t, []byte(h.View(size.width)))
        })
    }
}
```

---

## tmux Capture

### When to use

- Live interaction testing where you need to verify what the TUI looks like after
  a real key sequence
- Agent visual iteration: change code, build, capture, read output, evaluate, repeat
- Debugging rendering issues that only appear in a real terminal

Golden files are better for regression detection. tmux capture is better when you
need to interact with the running TUI or see actual terminal rendering.

### Prerequisites

- tmux installed (`brew install tmux`)
- Test fixture created (`make test-fixture`)

The fixture creates a realistic git repository at `/tmp/grove-test-fixture/rails-app`
with four worktrees in varied states (clean, dirty, ahead). Grove launches into this
fixture directory.

### Make targets

**Capture default TUI state** (builds binary, opens grove, waits 2 seconds, captures):

```bash
make tui-capture
```

Output is written to `tmp/tui-capture.txt` and printed to stdout.

**Capture after a key sequence**:

```bash
make tui-capture-keys KEYS="j j Enter"
```

Each token in `KEYS` is sent as a separate key press with a 0.3 second delay between
them. Special tokens: `Enter`, `Escape`, `Space`, `Tab`, `Up`, `Down`, `Left`, `Right`.
Any other token is sent as a literal key.

### Script options

The underlying script at `scripts/tui-capture.sh` accepts additional flags:

```bash
./scripts/tui-capture.sh -w 120 -h 40 -k "j j Enter" -d 3 -o tmp/my-capture.txt
```

| Flag | Default | Description |
|------|---------|-------------|
| `-w WIDTH` | `80` | Terminal width |
| `-h HEIGHT` | `24` | Terminal height |
| `-k KEYS` | (none) | Space-separated key sequence |
| `-o OUTPUT` | `tmp/tui-capture.txt` | Output file path |
| `-d DELAY` | `2` | Seconds to wait for initial render |
| `--no-build` | — | Skip building the binary |

### Agent iteration loop

For design iteration, agents can use this loop:

1. Modify TUI code
2. `make tui-capture` (or `make tui-capture-keys KEYS="..."`)
3. Read `tmp/tui-capture.txt`
4. Evaluate the rendered output
5. Repeat from step 1

Golden files can also serve this role without requiring tmux:

1. Modify TUI code
2. `make golden-view TEST=TestGolden_Dashboard`
3. Read the printed output
4. Evaluate
5. Repeat

---

## VHS Tapes

### When to use

- Recording demo GIFs for documentation or README
- Scripted walkthroughs of multi-step flows

VHS produces pixel-perfect recordings by driving a real terminal emulator. It is
not useful for regression detection or agent iteration (too slow, produces binary
output).

### Location

```
tapes/
  dashboard.tape      — Dashboard overview
  create-flow.tape    — Create worktree wizard walkthrough
  overlays.tape       — Overlay panel demonstrations
  showcase.tape       — Hero GIF for README (full feature sizzle reel)
```

### Demo fixture

All tapes depend on a demo fixture at `/tmp/grove-demo`. The `make demo` target
sets this up automatically via `scripts/create-demo.sh`, which:

1. Calls `scripts/create-fixture.sh /tmp/grove-demo` to create a git repo with
   four worktrees in varied states (clean, dirty, ahead)
2. Copies the built `bin/grove` binary into the fixture at
   `/tmp/grove-demo/rails-app/bin/grove`

Tapes `cd /tmp/grove-demo/rails-app` and run `./bin/grove` to launch the TUI.

### Recording

```bash
make demo
```

Requires VHS (`brew install vhs`). Creates the demo fixture, then records all
`.tape` files in `tapes/` and writes output GIFs to `docs/`.

### Creating a new tape

Create a new `.tape` file in `tapes/`. See the [VHS documentation](https://github.com/charmbracelet/vhs)
for the tape syntax. At minimum, set the output path and terminal dimensions to match
the project's standard sizes:

```
Output tapes/my-feature.gif
Set Width 800
Set Height 600
Set FontSize 14
```

---

## Decision Guide

| Scenario | Tool | Make Target |
|----------|------|-------------|
| Regression detection | Golden files | `make golden-diff` |
| Design iteration (agent) | Golden files | `make golden-view TEST=...` |
| Live interaction testing | tmux capture | `make tui-capture` |
| Key sequence verification | tmux capture | `make tui-capture-keys KEYS="..."` |
| Demo / documentation GIF | VHS | `make demo` |
| Publication screenshots | Freeze | (manual) |
