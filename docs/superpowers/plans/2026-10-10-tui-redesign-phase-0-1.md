# TUI Redesign Phases 0 and 1 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Bring the Charm stack current. Then replace the overlay line-splicer with a cell compositor, so overlays stop erasing the rows behind them and dim them evenly (issue #19).

**Architecture:**

- **Phase 0** bumps dependencies and moves glamour to its lipgloss v2 release. It also stops bubbles' list from quitting the app on its own.
- **Phase 1** adds `compositor.go`. It strips styling from the background, re-renders it uniformly faint, and places the overlay on a lipgloss `Compositor` canvas clipped to the screen.
- `centerOverlay` keeps its signature and delegates to it, so every existing call site gets the fix.

**Tech Stack:** Go 1.25+, `charm.land/bubbletea/v2`, `charm.land/lipgloss/v2` (Compositor, Layer), `charm.land/bubbles/v2`, `charm.land/glamour/v2`, `github.com/charmbracelet/x/ansi`, `x/exp/golden`.

**Spec:** `docs/superpowers/specs/2026-10-10-tui-redesign-design.md` (sections 15.3, 16, 17 phases 0 and 1).

## Global Constraints

- Targets, verbatim from spec section 16: bubbletea **2.0.10**, lipgloss **2.0.6**, bubbles **2.1.1** (not 2.2.x), glamour **`charm.land/glamour/v2` 2.0.1**, `x/exp/golden` **v0.1.0**, huh **hold at 2.0.3**.
- No prerelease versions. Do not cherry-pick July's commit `d4580f6`.
- Every command must complete in under 500ms (CLAUDE.md).
- Measure visible width with `lipgloss.Width` or `ansi.StringWidth`, never `len`.
- No `panic()`. Wrap errors with context.
- `make lint test` passes before every commit.
- Conventional commits: `type(scope): description`. No AI attribution lines.
- Golden churn is expected in Task 5. Review it with `make golden-diff`; do not bulk-accept.

### Deliberate narrowing of spec 15.3 for Phase 1

These parts of spec 15.3 need the view stack and move to Phase 2:

- Freezing the base frame (frames do not exist yet).
- Removing the `overlayOnDashboard` and `compositeActiveOverlay` wrappers.
- Moving toasts into the compositor (`compositeToastOnHeader` works and is out of #19's path).

One refinement: the dim stays **faint on stripped text**, not the muted color the spec names. Faint carries no color, so it dims identically under `NO_COLOR`, high contrast, and every theme.

## Review Focus

The five inputs most likely to bite a user that no feature test would naturally hit. Each has a test in the owning task.

1. **Overlay taller or wider than the terminal** (config overlay in a 24-row pane). The frame must stay exactly terminal-sized. Test: Task 4, `TestComposeOverlay_ClipsOversizedOverlayToScreen`.
2. **Wide and ambiguous-width glyphs at the overlay edge** (`⬢`, `⎇`, CJK branch names). No line may exceed the terminal width. Test: Task 4, `TestComposeOverlay_WideGlyphsStayWithinWidth`.
3. **Background shorter than the screen** (the PR panel behind the create wizard). Output still has exactly `height` lines. Test: Task 4, `TestComposeOverlay_PadsShortBackgroundToScreenHeight`.
4. **Styled background with inner resets** (bold header). The dim must cover the whole background, not stop at the first reset. Test: Task 4, `TestComposeOverlay_DimsWholeBackgroundUniformly`.
5. **`NO_COLOR` markdown after the glamour v2 move.** PR and issue bodies must render with no escape codes. Test: Task 3, `TestRenderMarkdown_NoColorHasNoEscapes`.

---

## File Structure

| File | Change | Responsibility |
|---|---|---|
| `go.mod`, `go.sum` | Modify | Dependency targets |
| `internal/tui/pr_preview.go` | Modify | Import path to glamour v2 |
| `internal/tui/pr_preview_test.go` | Create | Markdown rendering characterization |
| `internal/tui/model.go` | Modify | Use `newWorktreeList`; `centerOverlay` delegates to the compositor |
| `internal/tui/worktree_list.go` | Create | `newWorktreeList`: the one list constructor, with the quit guard |
| `internal/tui/worktree_list_test.go` | Create | Guard test |
| `internal/tui/helpers_test.go` | Modify | `newTestModel` uses `newWorktreeList` |
| `internal/tui/compositor.go` | Create | `composeOverlay`, `dimBackdrop`, `fitLines` |
| `internal/tui/compositor_test.go` | Create | Unit tests for the compositor |
| `internal/tui/overlay_background_test.go` | Create | Model-level test: rows behind an overlay stay visible |
| `internal/tui/testdata/**` | Regenerate | Golden churn from new dim and kept background |
| `CHANGELOG.md` | Modify | Unreleased entry |

---

### Task 1: Toolchain and green baseline

**Files:** none changed.

**Interfaces:**
- Consumes: nothing.
- Produces: a recorded baseline. Later tasks compare against it.

- [ ] **Step 1: Install Go through mise**

The host has no Go. `mise.toml` declares `go = "latest"`. Ask the user before installing.

Run: `mise install go && mise exec -- go version`
Expected: `go version go1.27.x linux/amd64` (any 1.25 or newer works).

All later `go` and `make` commands run as `mise exec -- <command>`.

- [ ] **Step 2: Confirm the branch and a clean tree**

Run: `git status -sb`
Expected: first line `## docs/tui-redesign` or the implementation branch cut from it. No modified tracked files. (Untracked dotfiles in the repo root are pre-existing; never stage them.)

- [ ] **Step 3: Run the baseline**

Run: `mise exec -- make lint test`
Expected: PASS. If anything fails, stop and report it. Do not build on a red baseline.

---

### Task 2: Bump the Charm stack and guard the list's quit binding

**Files:**
- Create: `internal/tui/worktree_list.go`
- Create: `internal/tui/worktree_list_test.go`
- Modify: `internal/tui/model.go:150-157` (list construction in `NewModel`)
- Modify: `internal/tui/helpers_test.go:22-30` (list construction in `newTestModel`)
- Modify: `go.mod`, `go.sum`

**Interfaces:**
- Consumes: `list.ItemDelegate` from `charm.land/bubbles/v2/list`.
- Produces: `func newWorktreeList(delegate list.ItemDelegate) list.Model`. Phase 4 reuses it.

**Why the guard:**

- bubbles 2.2.x rebinds the list's own `Quit` to `v` (upstream issue #1045).
- Grove handles `q` itself before keys reach the list, so the list's binding is never needed.
- Disabling it means no future bump can make a key quit the app.

- [ ] **Step 1: Write the failing test**

Create `internal/tui/worktree_list_test.go`:

```go
package tui

import "testing"

func TestNewWorktreeList_DisablesListQuit(t *testing.T) {
	l := newWorktreeList(NewWorktreeDelegateV2())
	if l.KeyMap.Quit.Enabled() {
		t.Fatal("list Quit binding is enabled; grove handles quitting, so the list's own binding must be off")
	}
}

func TestNewWorktreeList_KeepsGroveBindings(t *testing.T) {
	l := newWorktreeList(NewWorktreeDelegateV2())
	if got := l.KeyMap.Filter.Keys(); len(got) != 1 || got[0] != "/" {
		t.Errorf("Filter keys = %v, want [/]", got)
	}
	if got := l.KeyMap.CursorUp.Keys(); len(got) != 2 || got[0] != "up" || got[1] != "k" {
		t.Errorf("CursorUp keys = %v, want [up k]", got)
	}
	if got := l.KeyMap.CursorDown.Keys(); len(got) != 2 || got[0] != "down" || got[1] != "j" {
		t.Errorf("CursorDown keys = %v, want [down j]", got)
	}
}
```

- [ ] **Step 2: Run it to verify it fails**

Run: `mise exec -- go test ./internal/tui/ -run TestNewWorktreeList -v`
Expected: FAIL, `undefined: newWorktreeList`.

- [ ] **Step 3: Write the constructor**

Create `internal/tui/worktree_list.go`:

```go
package tui

import "charm.land/bubbles/v2/list"

// newWorktreeList builds the dashboard list. Grove owns quitting, so the
// list's own Quit binding is disabled; otherwise a bubbles release that
// rebinds it would quit the app from an ordinary key.
func newWorktreeList(delegate list.ItemDelegate) list.Model {
	l := list.New(nil, delegate, 0, 0)
	l.SetShowTitle(false)
	l.SetShowStatusBar(false)
	l.SetShowHelp(false)
	l.SetFilteringEnabled(true)
	l.KeyMap.Filter.SetKeys("/")
	l.KeyMap.CursorUp.SetKeys("up", "k")
	l.KeyMap.CursorDown.SetKeys("down", "j")
	l.KeyMap.Quit.SetEnabled(false)
	return l
}
```

- [ ] **Step 4: Use it in both constructors**

In `internal/tui/model.go`, replace the block that starts `l := list.New(nil, delegate, 0, 0)` and ends with `l.KeyMap.CursorDown.SetKeys("down", "j")` with:

```go
	l := newWorktreeList(delegate)
```

In `internal/tui/helpers_test.go` (`newTestModel`), replace the block that starts `l := list.New(nil, delegate, 0, 0)` and ends with `l.KeyMap.CursorDown.SetKeys("down", "j")` with:

```go
	l := newWorktreeList(delegate)
```

Read both blocks first. If either sets anything not listed in Step 3, add that call to `newWorktreeList` too, so both constructors stay identical.

- [ ] **Step 5: Run the new and existing quit tests**

Run: `mise exec -- go test ./internal/tui/ -run 'TestNewWorktreeList|TestQuitKeys|TestEscQuitsDashboard|TestDashboard_FocusQStillQuits' -v`
Expected: PASS. `q` and `esc` still quit through grove's own handler.

- [ ] **Step 6: Bump the dependencies**

```bash
mise exec -- go get charm.land/bubbletea/v2@v2.0.10 charm.land/lipgloss/v2@v2.0.6 charm.land/bubbles/v2@v2.1.1 github.com/charmbracelet/x/exp/golden@v0.1.0
mise exec -- go mod tidy
git diff go.mod
```

Expected in `git diff go.mod`: those four versions changed, and the `go` directive still reads `go 1.25.8`.

If the `go` directive moved, **stop and ask the user.** A higher directive raises the minimum Go version for every contributor and for `go install` users. That is their decision, not the executor's.

- [ ] **Step 7: Run the full suite**

Run: `mise exec -- make lint test`
Expected: PASS.

If goldens fail:

- Run `mise exec -- make golden-diff` and read every changed file.
- July's bump to lipgloss 2.0.5 and bubbletea 2.0.8 produced no golden churn. These targets are newer, so that is a hint, not a guarantee.
- Any change here is a finding. Report it rather than accepting it.

- [ ] **Step 8: Commit**

```bash
git add go.mod go.sum internal/tui/worktree_list.go internal/tui/worktree_list_test.go internal/tui/model.go internal/tui/helpers_test.go
git commit -m "build(deps): bump bubbletea 2.0.10, lipgloss 2.0.6, bubbles 2.1.1, golden 0.1.0

Builds the dashboard list in one constructor and disables the list's own
Quit binding, since grove handles quitting and bubbles 2.2 rebinds it."
```

---

### Task 3: Move glamour to its lipgloss v2 release

**Files:**
- Create: `internal/tui/pr_preview_test.go`
- Modify: `internal/tui/pr_preview.go:7`
- Modify: `go.mod`, `go.sum`

**Interfaces:**
- Consumes: `renderMarkdown(md string, width int) string` (unchanged).
- Produces: the same function on `charm.land/glamour/v2`. No signature change.

**Why:** glamour 0.10.0 pulls lipgloss v1, termenv, and reflow into the build. glamour v2 runs on lipgloss v2, and its options API is identical (`NewTermRenderer`, `WithWordWrap`, `WithStylePath("notty")`, `WithStandardStyle("dark")`, `Render`).

- [ ] **Step 1: Write characterization tests against the current glamour**

Create `internal/tui/pr_preview_test.go`:

```go
package tui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
)

const previewSample = "# Fix login\n\nThis **fixes** the redirect loop.\n\n- first\n- second\n"

func TestRenderMarkdown_NoColorHasNoEscapes(t *testing.T) {
	t.Setenv("NO_COLOR", "1")
	out := renderMarkdown(previewSample, 60)
	if strings.Contains(out, "\x1b[") {
		t.Fatalf("NO_COLOR output contains escape codes: %q", out)
	}
	for _, want := range []string{"Fix login", "fixes", "redirect loop", "first", "second"} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q:\n%s", want, out)
		}
	}
}

func TestRenderMarkdown_ColorKeepsText(t *testing.T) {
	t.Setenv("NO_COLOR", "")
	t.Setenv("GROVE_NO_COLOR", "")
	plain := ansi.Strip(renderMarkdown(previewSample, 60))
	for _, want := range []string{"Fix login", "fixes", "redirect loop", "first", "second"} {
		if !strings.Contains(plain, want) {
			t.Errorf("output missing %q:\n%s", want, plain)
		}
	}
}

func TestRenderMarkdown_WrapsToWidth(t *testing.T) {
	t.Setenv("NO_COLOR", "1")
	long := strings.Repeat("word ", 40)
	for _, line := range strings.Split(renderMarkdown(long, 30), "\n") {
		if w := ansi.StringWidth(line); w > 30 {
			t.Errorf("line width %d exceeds 30: %q", w, line)
		}
	}
}
```

- [ ] **Step 2: Run them on the current glamour**

Run: `mise exec -- go test ./internal/tui/ -run TestRenderMarkdown -v`
Expected: PASS. These pin today's behavior; they must still pass after the move. If one fails now, fix the test to match today's real output before changing anything, and note it.

- [ ] **Step 3: Switch the import**

In `internal/tui/pr_preview.go`, replace:

```go
	"github.com/charmbracelet/glamour"
```

with:

```go
	"charm.land/glamour/v2"
```

- [ ] **Step 4: Fetch and tidy**

```bash
mise exec -- go get charm.land/glamour/v2@v2.0.1
mise exec -- go mod tidy
grep -E 'charmbracelet/glamour|charmbracelet/lipgloss |muesli/termenv|muesli/reflow' go.mod || echo "v1 island gone"
```

Expected: `v1 island gone`. If a line remains, run `mise exec -- go mod why -m <module>` and report which package still needs it. Do not force-remove it.

- [ ] **Step 5: Run the tests**

Run: `mise exec -- go test ./internal/tui/ -run TestRenderMarkdown -v`
Expected: PASS.

Then: `mise exec -- make lint test`
Expected: PASS. If PR or issue preview goldens change, read the diff in `make golden-diff`. Spacing changes in rendered markdown are acceptable; lost text is not.

- [ ] **Step 6: Commit**

```bash
git add go.mod go.sum internal/tui/pr_preview.go internal/tui/pr_preview_test.go
git commit -m "build(deps): move to charm.land/glamour/v2 2.0.1

Glamour v2 runs on lipgloss v2, which drops lipgloss v1, termenv, and
reflow from the build. The renderer options are unchanged."
```

If goldens changed, add `internal/tui/testdata` to the `git add` line after reviewing them.

---

### Task 4: The compositor

**Files:**
- Create: `internal/tui/compositor.go`
- Create: `internal/tui/compositor_test.go`

**Interfaces:**
- Consumes: `lipgloss.NewCompositor`, `lipgloss.NewLayer` (lipgloss 2.0.6, `layer.go`), `ansi.Strip`, `ansi.Truncate`, `ansi.StringWidth`.
- Produces:
  - `func composeOverlay(bg, overlay string, width, height int) string`: overlay centered over a dimmed `bg`, exactly `height` lines, no line wider than `width`.
  - `func dimBackdrop(bg string, width, height int) string`: `bg` stripped, padded or trimmed to `width` x `height`, every line faint.
  - `func fitLines(s string, width, height int) []string`: exactly `height` lines, each truncated to `width` cells.
  - Phase 2 adds an anchor parameter for top-anchored overlays. Do not add it now.

**How it works:** the compositor draws each layer onto a cell canvas. Background cells outside the overlay's rectangle are never touched, so the rows beside the overlay survive. `Canvas.Render` trims trailing spaces per line but keeps the line count.

- [ ] **Step 1: Write the failing tests**

Create `internal/tui/compositor_test.go`:

```go
package tui

import (
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
)

func TestComposeOverlay_KeepsBackgroundBesideOverlay(t *testing.T) {
	bg := strings.TrimSuffix(strings.Repeat("abcdefghij\n", 5), "\n")
	lines := strings.Split(ansi.Strip(composeOverlay(bg, "XX", 10, 5)), "\n")

	// "XX" is 2 wide in a 10x5 screen: column (10-2)/2 = 4, row (5-1)/2 = 2.
	if lines[2] != "abcdXXghij" {
		t.Errorf("overlay row = %q, want %q", lines[2], "abcdXXghij")
	}
	for _, i := range []int{0, 1, 3, 4} {
		if lines[i] != "abcdefghij" {
			t.Errorf("row %d = %q, want the background unchanged", i, lines[i])
		}
	}
}

func TestComposeOverlay_DimsWholeBackgroundUniformly(t *testing.T) {
	red := lipgloss.NewStyle().Foreground(lipgloss.Color("#FF0000"))
	bold := lipgloss.NewStyle().Bold(true)
	styled := red.Render("alpha") + " plain " + bold.Render("beta")
	plain := "alpha plain beta"

	got := composeOverlay(styled, "X", 20, 3)
	want := composeOverlay(plain, "X", 20, 3)
	if got != want {
		t.Errorf("background styling leaked through the dim\n got: %q\nwant: %q", got, want)
	}
	if !strings.Contains(got, "\x1b[2m") {
		t.Errorf("background is not faint: %q", got)
	}
}

func TestComposeOverlay_PadsShortBackgroundToScreenHeight(t *testing.T) {
	got := composeOverlay("one line", "X", 12, 5)
	if n := len(strings.Split(got, "\n")); n != 5 {
		t.Errorf("got %d lines, want 5", n)
	}
}

func TestComposeOverlay_ClipsOversizedOverlayToScreen(t *testing.T) {
	row := strings.Repeat("O", 30)
	overlay := strings.TrimSuffix(strings.Repeat(row+"\n", 10), "\n")
	lines := strings.Split(composeOverlay("bg", overlay, 20, 6), "\n")
	if len(lines) != 6 {
		t.Fatalf("got %d lines, want 6", len(lines))
	}
	for i, l := range lines {
		if w := lipgloss.Width(l); w > 20 {
			t.Errorf("line %d is %d wide, want <= 20", i, w)
		}
	}
}

func TestComposeOverlay_WideGlyphsStayWithinWidth(t *testing.T) {
	row := strings.Repeat("⬢界⎇ab", 4)
	bg := row + "\n" + row + "\n" + row
	for _, width := range []int{9, 10, 11} {
		for i, l := range strings.Split(composeOverlay(bg, "XYZ", width, 3), "\n") {
			if w := lipgloss.Width(l); w > width {
				t.Errorf("width %d: line %d is %d wide", width, i, w)
			}
		}
	}
}

func TestComposeOverlay_ZeroSizeReturnsOverlay(t *testing.T) {
	if got := composeOverlay("bg", "X", 0, 0); got != "X" {
		t.Errorf("got %q, want the overlay unchanged", got)
	}
}

func TestFitLines_TrimsAndPads(t *testing.T) {
	got := fitLines("a\nb\nc\nd", 5, 2)
	if len(got) != 2 || got[0] != "a" || got[1] != "b" {
		t.Errorf("trim: got %q", got)
	}
	got = fitLines("a", 5, 3)
	if len(got) != 3 || got[0] != "a" || got[1] != "" || got[2] != "" {
		t.Errorf("pad: got %q", got)
	}
	got = fitLines("abcdefgh", 3, 1)
	if got[0] != "abc" {
		t.Errorf("truncate: got %q", got[0])
	}
}
```

- [ ] **Step 2: Run them to verify they fail**

Run: `mise exec -- go test ./internal/tui/ -run 'TestComposeOverlay|TestFitLines' -v`
Expected: FAIL, `undefined: composeOverlay` and `undefined: fitLines`.

- [ ] **Step 3: Write the compositor**

Create `internal/tui/compositor.go`:

```go
package tui

import (
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
)

// backdropStyle dims everything behind an overlay. Faint carries no color,
// so the dim looks the same under NO_COLOR, high contrast, and every theme.
var backdropStyle = lipgloss.NewStyle().Faint(true)

// composeOverlay draws overlay centered over a dimmed copy of bg. The result
// is exactly height lines, none wider than width. Background cells outside
// the overlay's rectangle are kept.
func composeOverlay(bg, overlay string, width, height int) string {
	if width <= 0 || height <= 0 {
		return overlay
	}
	box := strings.Join(fitLines(overlay, width, min(lipgloss.Height(overlay), height)), "\n")
	x := max(0, (width-lipgloss.Width(box))/2)
	y := max(0, (height-lipgloss.Height(box))/2)
	return lipgloss.NewCompositor(
		lipgloss.NewLayer(dimBackdrop(bg, width, height)),
		lipgloss.NewLayer(box).X(x).Y(y).Z(1),
	).Render()
}

// dimBackdrop strips all styling from bg and renders each line faint. Styling
// is stripped first because an inner reset would otherwise end the dim partway
// through a line. Lines are padded to width so the canvas is always full size.
func dimBackdrop(bg string, width, height int) string {
	lines := fitLines(bg, width, height)
	for i, line := range lines {
		plain := ansi.Strip(line)
		if pad := width - ansi.StringWidth(plain); pad > 0 {
			plain += strings.Repeat(" ", pad)
		}
		lines[i] = backdropStyle.Render(plain)
	}
	return strings.Join(lines, "\n")
}

// fitLines returns exactly height lines of s, each truncated to width cells.
func fitLines(s string, width, height int) []string {
	lines := strings.Split(s, "\n")
	if len(lines) > height {
		lines = lines[:height]
	}
	for len(lines) < height {
		lines = append(lines, "")
	}
	for i, line := range lines {
		lines[i] = ansi.Truncate(line, width, "")
	}
	return lines
}
```

- [ ] **Step 4: Run the tests**

Run: `mise exec -- go test ./internal/tui/ -run 'TestComposeOverlay|TestFitLines' -v`
Expected: PASS.

The `\x1b[2m` check holds under `NO_COLOR` and in color:

- lipgloss 2.0.6's `Style.Render` never consults a color profile.
- ultraviolet serializes a faint-only cell as exactly `\x1b[2m` (`cell.go`, `Style.String`).
- If it still fails, print `got` with `%q` and report it. Do not loosen the assertion.

- [ ] **Step 5: Commit**

```bash
git add internal/tui/compositor.go internal/tui/compositor_test.go
git commit -m "feat(tui): add a cell compositor for overlays

Draws an overlay over a stripped, uniformly faint copy of the background
on a lipgloss canvas, clipped to the screen. Not wired in yet."
```

---

### Task 5: Route every overlay through the compositor (fixes #19)

**Files:**
- Create: `internal/tui/overlay_background_test.go`
- Modify: `internal/tui/model.go:2296-2343` (`centerOverlay`)
- Regenerate: `internal/tui/testdata/**`

**Interfaces:**
- Consumes: `composeOverlay` from Task 4. Test helpers `goldenModel`, `withItems`, `withDeleteOverlay`, `makeTestItems`, `sizeUltraWide`, and `Model.renderDashboard`, `Model.viewString`.
- Produces: `centerOverlay(bg, overlay string, width, height int) string` with its signature unchanged. Its callers are `overlayOnDashboard`, `compositeActiveOverlay`, the create overlay in `viewForActiveView`, and the existing-worktree prompts in `view_prs_v2.go:132` and `view_issues.go:296`. All get the fix without edits.

- [ ] **Step 1: Write the failing model-level test**

Create `internal/tui/overlay_background_test.go`:

```go
package tui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
)

// Each row's commit message sits in the left column. At 160 columns the
// delete overlay is centered well to its right, so every message visible
// without the overlay must stay visible with it.
func TestOverlay_KeepsBackgroundRowsVisible(t *testing.T) {
	m := goldenModel(t, sizeUltraWide, withItems(10))

	before := ansi.Strip(m.renderDashboard())
	var visible []string
	for _, item := range makeTestItems(10) {
		if strings.Contains(before, "test commit for "+item.ShortName) {
			visible = append(visible, item.ShortName)
		}
	}
	if len(visible) < 8 {
		t.Fatalf("only %d of 10 rows visible without an overlay; the test needs at least 8", len(visible))
	}

	withDeleteOverlay()(&m)
	after := ansi.Strip(m.viewString())
	for _, name := range visible {
		if !strings.Contains(after, "test commit for "+name) {
			t.Errorf("row %q disappeared behind the delete overlay", name)
		}
	}
}
```

- [ ] **Step 2: Run it to verify it fails**

Run: `mise exec -- go test ./internal/tui/ -run TestOverlay_KeepsBackgroundRowsVisible -v`
Expected: FAIL with `row "..." disappeared behind the delete overlay` for the rows the overlay spans vertically. Today's `centerOverlay` replaces those whole lines with `padding + overlay`.

If it fails with the "only N of 10 rows visible" message instead, the dashboard paginates at this size:

- Lower `withItems(10)` to the largest count that fits, and lower the floor of 8 to match.
- Keep at least 2 visible rows below the overlay's top edge, so the test still crosses the overlay.

- [ ] **Step 3: Replace `centerOverlay`'s body**

In `internal/tui/model.go`, replace the whole `centerOverlay` function (from `func centerOverlay(bg, overlay string, width, height int) string {` to its closing brace, about 47 lines) with:

```go
// centerOverlay draws overlay centered over a dimmed bg. See composeOverlay.
func centerOverlay(bg, overlay string, width, height int) string {
	return composeOverlay(bg, overlay, width, height)
}
```

- [ ] **Step 4: Run the new test**

Run: `mise exec -- go test ./internal/tui/ -run TestOverlay_KeepsBackgroundRowsVisible -v`
Expected: PASS.

- [ ] **Step 5: Regenerate and review goldens**

Run: `mise exec -- make golden-diff`

Expected changes, and only these:

- Every `TestGolden_Overlay_*` and `TestGolden_Dashboard_HelpExpanded` file: background rows beside the overlay now keep their text; the whole background is one faint run per line; trailing spaces are trimmed.
- `TestGolden_Themed_OverlayBorders.golden`: the same, plus the background loses its colors under the dim.

Open at least `TestGolden_Overlay_Delete/default.golden` and `TestGolden_Dashboard_HelpExpanded/standard_80x24.golden` with `cat -v`. Confirm:

1. No line contains text from two different background rows.
2. The overlay box's border characters are intact on every line.
3. Each file has the same line count as before.

Any change outside overlay goldens (for example plain `TestGolden_Dashboard`) is a bug. Stop and report it.

- [ ] **Step 6: Run the full suite**

Run: `mise exec -- make lint test`
Expected: PASS.

- [ ] **Step 7: Check the frame budget**

Run: `mise exec -- go test ./internal/tui/ -run '^$' -bench 'BenchmarkView(DeleteOverlay|BulkOverlay|Help|Dashboard)$' -benchmem`
Then the same command on the parent commit: `git stash && <command> && git stash pop`.

Expected:

- `BenchmarkViewDashboard` is unchanged. It never composites.
- The three overlay benchmarks may be slower, since the canvas works per cell.
- Report the before and after `ns/op` for each in the task summary.
- Any overlay render that approaches the 500ms command budget is a blocker.

- [ ] **Step 8: Commit**

```bash
git add internal/tui/model.go internal/tui/overlay_background_test.go internal/tui/testdata
git commit -m "fix(tui): keep rows behind overlays visible and evenly dimmed

centerOverlay replaced each background line under an overlay with
padding plus the overlay line, erasing everything beside it, and its
faint style stopped at the first inner reset. It now delegates to the
cell compositor, which keeps background cells outside the overlay and
dims the stripped background uniformly.

Fixes #19"
```

---

### Task 6: Changelog

**Files:**
- Modify: `CHANGELOG.md` (the `## [Unreleased]` section)

**Interfaces:** none.

- [ ] **Step 1: Add the entries**

Under `## [Unreleased]` in `CHANGELOG.md`, add:

```markdown
### Fixed
- Overlays in the dashboard no longer erase the rows beside them, and the background behind them is dimmed evenly instead of stopping at the first styled word. (#19)

### Changed
- Updated the terminal UI libraries: bubbletea 2.0.10, lipgloss 2.0.6, bubbles 2.1.1, and glamour v2, which removes the older lipgloss v1 from the build.
```

- [ ] **Step 2: Check docs for stale overlay descriptions**

Run: `grep -n -i 'overlay' docs/TUI.md docs/VISUAL_TESTING.md`
Expected: `docs/TUI.md` says overlays open "centered over the dimmed dashboard background". That is still true; no change. If any doc describes the old line-splicing, update that sentence.

- [ ] **Step 3: Commit**

```bash
git add CHANGELOG.md
git commit -m "docs(changelog): note the overlay fix and dependency updates"
```

---

## Done when

- `make lint test` passes on the final commit.
- `go.mod` has no `github.com/charmbracelet/glamour`, `github.com/charmbracelet/lipgloss` (v1), `termenv`, or `reflow` lines, unless Task 3 Step 4 reported why.
- `TestOverlay_KeepsBackgroundRowsVisible` and every `TestComposeOverlay_*` test pass.
- Golden churn is limited to overlay goldens and was reviewed file by file.
- Phase 2 (view stack) gets its own plan, written against the code as it stands after this one.
