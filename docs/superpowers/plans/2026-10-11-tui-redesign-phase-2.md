# TUI Redesign Phase 2 (View Stack) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace the flat `activeView` field with a view stack. `esc` goes back exactly one level, and the dashboard cursor returns to the same worktree.

**Architecture:**

- A small immutable `viewStack` type (new `view_stack.go`) holds frames above the dashboard root. Every push copies, so value copies of `Model` never share frames.
- `Model` loses its `activeView` field and gains `views viewStack` plus four methods: `activeView()`, `pushView`, `popView`, `popToRoot`. About 70 navigation sites map onto those four operations.
- The help and update overlays become frames instead of flags.
- Rendering walks the stack: an overlay frame draws over the frame beneath it. This removes `overlayOnDashboard`, `compositeActiveOverlay`, the create-wizard special case, and the `centerOverlay` wrapper.

**Tech Stack:** Go 1.26+, `charm.land/bubbletea/v2`, `charm.land/bubbles/v2/list`, the Phase 1 compositor (`composeOverlay`).

**Spec:** `docs/superpowers/specs/2026-10-10-tui-redesign-design.md` (N1; section 15.2; the Phase 2 items deferred from 15.3).

## Global Constraints

- Every command must complete in under 500ms (CLAUDE.md).
- No `panic()`. Wrap errors with context.
- The gate is `make lint test` with **golangci-lint v2.14** (CI's version), not the `go vet` fallback.
- Conventional commits: `type(scope): description`. No AI attribution lines.
- **Expected golden churn: zero in every task.** This phase changes navigation, not rendering. Any golden diff is a finding: report it, do not accept it.
- Measure visible width with `lipgloss.Width` or `ansi.StringWidth`, never `len`.

## Decisions for review

| # | Decision | Spec says | Why |
|---|---|---|---|
| 1 | **Do not freeze the base frame** under overlays | 15.3: freeze, re-render on resize | Overlay frames cost about 5ms, far under budget. Toasts render into the header inside the base, so a frozen base would hide any toast that fires while an overlay is open. Phase 6's agent poll would also go stale behind overlays. |
| 2 | **Toasts stay in `compositeToastOnHeader`** | 15.3: toasts at compositor Z2 | It works and never touched #19. Moving it adds churn for no visible change. |
| 3 | **Per-view state stays in `Model` fields** (`deleteState`, `createState`, ...) | 15.2: frame holds a pointer to view state | Frames only need the view and a return position. Moving states into frames would touch every handler for no behavior change. |
| 4 | **The PR/issue "worktree exists" prompt stays a view-local popup** | 15.2: one navigation system | It lives inside the PR and issue browsers, which Phase 8 rebuilds. Its two `centerOverlay` calls switch to `composeOverlay` here. |
| 5 | **Success returns to the dashboard; `esc` goes back one level** | 15.2: `esc` pops one frame | Success handlers use `popToRoot`, which keeps today's "back to the dashboard after create, delete, fork" behavior. |

## Review Focus

The five inputs most likely to bite a user that no feature test would naturally hit. Each has a test in the owning task.

1. **A refresh reorders the list while an overlay is open** (sort mode, later the agent poll). `esc` must land on the same worktree, not the same row. Test: Task 3, `TestEscRestoresCursorToSameWorktreeAfterReorder`.
2. **The worktree under an overlay is deleted** before the overlay closes. Restoring must fall back to a valid row, never panic. Test: Task 2, `TestRestoreCursor_FallsBackToClampedIndex`.
3. **A filter is active when a frame pops.** The cursor must index the visible rows, not all rows. Test: Task 2, `TestRestoreCursor_UsesVisibleRowsUnderFilter`.
4. **Help opened over another overlay** (the delete prompt). `esc` twice must show the delete prompt again, then the dashboard. Test: Task 4, `TestHelpOverDelete_EscWalksBack`.
5. **Two copies of the model after an update** (tests hold old copies; Bubble Tea may too). A push on one must not change the other. Test: Task 2, `TestViewStack_CopiesDoNotShareFrames`.

---

## File Structure

| File | Change | Responsibility |
|---|---|---|
| `internal/tui/view_stack.go` | Create | `viewFrame`, `viewStack`, and the `Model` navigation methods |
| `internal/tui/view_stack_test.go` | Create | Stack and cursor-restore unit tests |
| `internal/tui/navigation_test.go` | Create | Model-level navigation tests |
| `internal/tui/model.go` | Modify | Field swap; ~50 navigation sites; help/update frames; frame rendering |
| `internal/tui/helpers.go`, `overlay_checkout.go`, `overlay_config.go`, `overlay_fork.go`, `overlay_sync.go`, `view_issues.go`, `view_prs.go`, `view_prs_v2.go`, `overlay_create.go` | Modify | Navigation sites; `ReturnView` removal; `composeOverlay` calls |
| `internal/tui/helpers_test.go` | Modify | `setViewForTest` helper |
| `internal/tui/*_test.go` (14 files) | Modify | Mechanical migration of `activeView` reads and writes |
| `internal/tui/compositor_test.go` | Modify | Exact-row wide-glyph test (Phase 1 deferred minor) |
| `internal/tui/pr_preview.go` | Modify | Stale glamour comment (Phase 1 deferred minor) |
| `docs/TUI.md` | Modify | `esc` behavior; file map |
| `CHANGELOG.md` | Modify | Unreleased entry |

---

### Task 1: Workspace, toolchain, and baseline

**Files:** none changed.

**Interfaces:**
- Consumes: nothing.
- Produces: `goenv.sh` and `gate.sh` in the plan workspace. Every later task gates with `gate.sh`.

- [ ] **Step 1: Create the workspace and environment file**

The sandbox makes `~/go` and `~/.cache` read-only, and mise sets `GOBIN` to a read-only directory. All Go state lives in `$TMPDIR`.

```bash
# From the phase 2 worktree root. SKILLS is the superpowers skills directory.
W=$($SKILLS/subagent-driven-development/scripts/sdd-workspace docs/superpowers/plans/2026-10-11-tui-redesign-phase-2.md)
cat > $W/goenv.sh <<EOF
export GOPATH=$TMPDIR/gopath GOMODCACHE=$TMPDIR/gopath/pkg/mod GOCACHE=$TMPDIR/gocache GOFLAGS=-modcacherw
export GOLANGCI_LINT_CACHE=$TMPDIR/golangci-cache TMUX_TMPDIR=$TMPDIR
EOF
```

- [ ] **Step 2: Make sure golangci-lint v2.14 exists**

```bash
. $W/goenv.sh
test -x $TMPDIR/gopath/bin/golangci-lint || mise exec -- env GOBIN=$TMPDIR/gopath/bin go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.14.0
mise exec -- $TMPDIR/gopath/bin/golangci-lint version
```

Expected: `golangci-lint has version 2.14.0`.

- [ ] **Step 3: Write the gate**

`internal/shell` and `internal/tmux` fail inside the sandbox for environment reasons only (hardcoded `/tmp`, no Unix sockets). Phase 1 confirmed this. The gate skips them; the full suite runs outside the sandbox at the end.

```bash
cat > $W/gate.sh <<'EOF'
#!/usr/bin/env bash
set -e
W=.superpowers/sdd/2026-10-11-tui-redesign-phase-2
. $W/goenv.sh
mise exec -- $TMPDIR/gopath/bin/golangci-lint run --timeout=5m
mise exec -- go test -race -cover $(mise exec -- go list ./... | grep -v -E 'internal/(shell|tmux)$')
EOF
chmod +x $W/gate.sh
```

- [ ] **Step 4: Run the baseline**

Run: `$W/gate.sh`
Expected: `0 issues.` and every package `ok`. If anything fails, stop and report.

---

### Task 2: The stack type and cursor restore

**Files:**
- Create: `internal/tui/view_stack.go`
- Create: `internal/tui/view_stack_test.go`

**Interfaces:**
- Consumes: `ActiveView`, `ViewDashboard` (model.go), `Model.list`, `Model.updateDetailContent()` (pointer receiver), `WorktreeItem.Path`.
- Produces:
  - `type viewFrame struct { view ActiveView; returnPath string; returnIndex int }`
  - `type viewStack []viewFrame` with `top() ActiveView`, `push(viewFrame) viewStack`, `pop() (viewStack, viewFrame, bool)`, `toRoot() (viewStack, viewFrame, bool)`
  - `func (m *Model) restoreCursor(f viewFrame)`
  - Task 3 adds the `Model` navigation methods. They cannot exist yet: `Model` still has a field named `activeView`.

- [ ] **Step 1: Write the failing tests**

Create `internal/tui/view_stack_test.go`:

```go
package tui

import (
	"testing"

	"charm.land/bubbles/v2/list"
)

func TestViewStack_EmptyTopIsDashboard(t *testing.T) {
	if got := (viewStack{}).top(); got != ViewDashboard {
		t.Errorf("top of empty stack = %v, want ViewDashboard", got)
	}
}

func TestViewStack_PushPopOrder(t *testing.T) {
	s := viewStack{}.push(viewFrame{view: ViewPRs}).push(viewFrame{view: ViewCreate})
	if s.top() != ViewCreate {
		t.Fatalf("top = %v, want ViewCreate", s.top())
	}
	s, f, ok := s.pop()
	if !ok || f.view != ViewCreate || s.top() != ViewPRs {
		t.Errorf("pop = (%v, %v), top now %v; want (ViewCreate, true), top ViewPRs", f.view, ok, s.top())
	}
}

func TestViewStack_PopAtRootIsNoOp(t *testing.T) {
	s, _, ok := viewStack{}.pop()
	if ok || len(s) != 0 {
		t.Errorf("pop at root: ok=%v len=%d, want false and 0", ok, len(s))
	}
}

func TestViewStack_ToRootReturnsBottomFrame(t *testing.T) {
	s := viewStack{}.
		push(viewFrame{view: ViewPRs, returnPath: "/a"}).
		push(viewFrame{view: ViewCreate, returnPath: "/b"})
	s, f, ok := s.toRoot()
	if !ok || len(s) != 0 || f.returnPath != "/a" {
		t.Errorf("toRoot: ok=%v len=%d path=%q; want true, 0, /a", ok, len(s), f.returnPath)
	}
}

func TestViewStack_CopiesDoNotShareFrames(t *testing.T) {
	base := viewStack{}.push(viewFrame{view: ViewPRs}).push(viewFrame{view: ViewCreate})
	popped, _, _ := base.pop()
	branched := popped.push(viewFrame{view: ViewFork})
	if base.top() != ViewCreate {
		t.Errorf("original copy changed: top = %v, want ViewCreate", base.top())
	}
	if branched.top() != ViewFork {
		t.Errorf("branched top = %v, want ViewFork", branched.top())
	}
}

func TestRestoreCursor_FindsWorktreeByPath(t *testing.T) {
	m := newTestModel(withItems(5), withSize(120, 40))
	m.restoreCursor(viewFrame{returnPath: "/tmp/test-project-testing", returnIndex: 0})
	if item, _ := m.selectedItem(); item.ShortName != "testing" {
		t.Errorf("selected %q, want testing", item.ShortName)
	}
}

func TestRestoreCursor_FallsBackToClampedIndex(t *testing.T) {
	m := newTestModel(withItems(3), withSize(120, 40))
	m.restoreCursor(viewFrame{returnPath: "/tmp/gone", returnIndex: 9})
	if got := m.list.Index(); got != 2 {
		t.Errorf("index = %d, want 2 (last row)", got)
	}
}

func TestRestoreCursor_EmptyListIsNoOp(t *testing.T) {
	m := newTestModel(withItems(0), withSize(120, 40))
	m.restoreCursor(viewFrame{returnPath: "/tmp/x", returnIndex: 3})
	if got := m.list.Index(); got != 0 {
		t.Errorf("index = %d, want 0", got)
	}
}

func TestRestoreCursor_UsesVisibleRowsUnderFilter(t *testing.T) {
	m := newTestModel(withItems(5), withSize(120, 40))
	m.list.SetFilterText("t")
	m.list.SetFilterState(list.FilterApplied)
	var want string
	for _, li := range m.list.VisibleItems() {
		if it := li.(WorktreeItem); it.ShortName == "testing" {
			want = it.Path
		}
	}
	if want == "" {
		t.Fatal("filter 't' should keep testing visible")
	}
	m.restoreCursor(viewFrame{returnPath: want})
	if item, _ := m.selectedItem(); item.Path != want {
		t.Errorf("selected %q, want %q", item.Path, want)
	}
}
```

- [ ] **Step 2: Run them to verify they fail**

Run: `. $W/goenv.sh && mise exec -- go test ./internal/tui/ -run 'TestViewStack|TestRestoreCursor' -v`
Expected: FAIL, `undefined: viewStack`, `undefined: viewFrame`.

- [ ] **Step 3: Write the stack**

Create `internal/tui/view_stack.go`:

```go
package tui

// viewFrame is one level of navigation above the dashboard: the view shown,
// and the dashboard cursor to restore when the frame is popped.
type viewFrame struct {
	view        ActiveView
	returnPath  string // worktree selected when the frame was pushed
	returnIndex int    // fallback row when that worktree is gone
}

// viewStack holds the frames above the dashboard root. Model is copied by
// value on every update, so push always allocates: two copies of a stack
// never share a backing array that one of them could overwrite.
type viewStack []viewFrame

func (s viewStack) top() ActiveView {
	if len(s) == 0 {
		return ViewDashboard
	}
	return s[len(s)-1].view
}

func (s viewStack) push(f viewFrame) viewStack {
	out := make(viewStack, len(s), len(s)+1)
	copy(out, s)
	return append(out, f)
}

// pop removes the top frame. At the root it changes nothing and ok is false.
func (s viewStack) pop() (viewStack, viewFrame, bool) {
	if len(s) == 0 {
		return s, viewFrame{}, false
	}
	return s[:len(s)-1 : len(s)-1], s[len(s)-1], true
}

// toRoot removes every frame and returns the bottom one, which holds the
// cursor from before any frame opened.
func (s viewStack) toRoot() (viewStack, viewFrame, bool) {
	if len(s) == 0 {
		return s, viewFrame{}, false
	}
	return nil, s[0], true
}

// restoreCursor selects the worktree saved in f. A refresh can reorder or
// remove rows while a frame is open, so the path wins over the row index.
// Indexes refer to visible rows, which differ from all rows under a filter.
func (m *Model) restoreCursor(f viewFrame) {
	rows := m.list.VisibleItems()
	if len(rows) == 0 {
		return
	}
	target := min(f.returnIndex, len(rows)-1)
	if f.returnPath != "" {
		for i, row := range rows {
			if item, ok := row.(WorktreeItem); ok && item.Path == f.returnPath {
				target = i
				break
			}
		}
	}
	m.list.Select(target)
	m.updateDetailContent()
}
```

- [ ] **Step 4: Run the tests**

Run: `mise exec -- go test ./internal/tui/ -run 'TestViewStack|TestRestoreCursor' -v`
Expected: PASS, 9 tests.

If `TestRestoreCursor_UsesVisibleRowsUnderFilter` fails because the filter never applied:

- `SetFilterState(list.FilterApplied)` may need a matching filter pass first.
- Apply the filter the way grove's tests do (search `*_test.go` for `SetFilterText`), and note it in the ledger.
- Do not weaken the assertion.

- [ ] **Step 5: Commit**

```bash
git add internal/tui/view_stack.go internal/tui/view_stack_test.go
git commit -m "feat(tui): add a view stack type with cursor restore by path

Frames above the dashboard record the selected worktree. Restoring
prefers that worktree's path over the row index, so a refresh that
reorders rows does not move the cursor. Not wired in yet."
```

---

### Task 3: Replace `activeView` with the stack

**Files:**
- Create: `internal/tui/navigation_test.go`
- Modify: `internal/tui/view_stack.go` (Model methods)
- Modify: `internal/tui/model.go`, `helpers.go`, `overlay_checkout.go`, `overlay_config.go`, `overlay_fork.go`, `overlay_sync.go`, `view_issues.go`, `view_prs.go`, `overlay_create.go`
- Modify: `internal/tui/helpers_test.go` and 14 other `*_test.go` files

**Interfaces:**
- Consumes: Task 2's `viewStack`, `viewFrame`, `restoreCursor`.
- Produces, used by Tasks 4 and 5:
  - `func (m Model) activeView() ActiveView`
  - `func (m *Model) pushView(v ActiveView)`
  - `func (m *Model) popView()`
  - `func (m *Model) popToRoot()`
  - Test-only: `func (m *Model) setViewForTest(v ActiveView)`

**This task cannot be split into compiling steps.** Removing the field breaks every site at once. Steps 3 to 7 are one edit; the code compiles again only at Step 8.

- [ ] **Step 1: Write the failing model-level test**

It uses only keys and messages that exist today, so it compiles before the migration.

Create `internal/tui/navigation_test.go`:

```go
package tui

import "testing"

// A refresh that reorders rows while an overlay is open must not move the
// cursor: esc returns to the same worktree, not the same row.
func TestEscRestoresCursorToSameWorktreeAfterReorder(t *testing.T) {
	m := newTestModel(withItems(5), withSize(120, 40))
	m.list.Select(2)
	before, _ := m.selectedItem()
	if before.ShortName != "fix-bug" {
		t.Fatalf("setup: selected %q, want fix-bug", before.ShortName)
	}

	next, _ := m.Update(makeKeyMsg("R"))
	m = next.(Model)

	items := makeTestItems(5)
	reordered := []WorktreeItem{items[0], items[1], items[3], items[4], items[2]}
	next, _ = m.Update(worktreesFetchedMsg{items: reordered})
	m = next.(Model)

	next, _ = m.Update(makeKeyMsg("esc"))
	m = next.(Model)

	after, _ := m.selectedItem()
	if after.Path != before.Path {
		t.Errorf("after esc, selected %q; want %q", after.ShortName, before.ShortName)
	}
}
```

- [ ] **Step 2: Run it to verify it fails**

Run: `mise exec -- go test ./internal/tui/ -run TestEscRestoresCursorToSameWorktreeAfterReorder -v`
Expected: FAIL, `after esc, selected "testing"; want "fix-bug"`. Today the cursor stays on row 2.

If `R` does not open rename at index 2, read `handleDashboardRenameKey` (model.go) for the condition and pick a row that satisfies it. Keep a reorder that moves the selected worktree.

- [ ] **Step 3: Add the Model navigation methods**

Append to `internal/tui/view_stack.go`:

```go
// activeView is the view on top of the stack, or the dashboard at the root.
func (m Model) activeView() ActiveView { return m.views.top() }

// pushView opens v, remembering the selected worktree to return to.
func (m *Model) pushView(v ActiveView) {
	f := viewFrame{view: v, returnIndex: m.list.Index()}
	if item, ok := m.selectedItem(); ok {
		f.returnPath = item.Path
	}
	m.views = m.views.push(f)
}

// popView goes back exactly one level. At the root it does nothing.
func (m *Model) popView() {
	var f viewFrame
	var ok bool
	if m.views, f, ok = m.views.pop(); ok {
		m.restoreCursor(f)
	}
}

// popToRoot closes every frame and returns to the dashboard.
func (m *Model) popToRoot() {
	var f viewFrame
	var ok bool
	if m.views, f, ok = m.views.toRoot(); ok {
		m.restoreCursor(f)
	}
}
```

- [ ] **Step 4: Swap the field**

In `internal/tui/model.go`:

- In `type Model struct`, replace the line `activeView    ActiveView` with `views         viewStack`.
- In `NewModel`'s struct literal, delete the line `activeView:          ViewDashboard,`. The zero value is the dashboard root.

- [ ] **Step 5: Rewrite the navigation sites with a checked script**

Line numbers are from commit `52c9d3b`. The script checks each line's text before editing and stops on any mismatch, so drift cannot apply an edit to the wrong line. Operations:

- `push:V` → `m.pushView(V)`
- `pop` → `m.popView()`
- `root` → `m.popToRoot()`
- `rootfork` → `m.popToRoot()` then `m.pushView(ViewFork)`
- `del` → delete the line (the `returnView := s.ReturnView` lines and `ReturnView` fields)

```bash
cat > $W/migrate_sites.py <<'EOF'
import re, sys
OPS = {
 'helpers.go': {239: 'root', 254: 'rootfork'},
 'model.go': {
   397: 'root', 437: 'root', 461: 'root', 501: 'root', 541: 'root', 553: 'root',
   579: 'root', 620: 'root', 635: 'root', 673: 'root', 947: 'pop',
   1041: 'push:ViewSync', 1046: 'push:ViewConfig', 1089: 'push:ViewCreate',
   1096: 'del', 1120: 'push:ViewDelete', 1160: 'push:ViewFork',
   1168: 'push:ViewRename', 1178: 'push:ViewCheckout',
   1227: 'pop', 1238: 'pop', 1247: 'pop', 1261: 'pop', 1271: 'pop', 1318: 'pop',
   1349: 'del', 1351: 'pop', 1410: 'del', 1412: 'pop', 1441: 'del', 1443: 'pop',
   1450: 'del', 1452: 'pop', 1527: 'del', 1529: 'pop', 1576: 'del', 1578: 'pop',
   1653: 'del', 1655: 'pop', 1662: 'del', 1664: 'pop', 1672: 'del', 1674: 'pop',
   1710: 'root', 1746: 'push:ViewPRs', 1757: 'push:ViewIssues',
   1789: 'push:ViewBulk', 1799: 'pop', 1812: 'pop',
   2309: 'push:ViewPRs', 2320: 'push:ViewIssues'},
 'overlay_checkout.go': {132: 'pop', 159: 'pop', 223: 'pop', 251: 'pop', 265: 'pop'},
 'overlay_config.go': {301: 'pop', 312: 'pop', 317: 'pop', 333: 'pop', 376: 'pop', 380: 'pop', 387: 'pop'},
 'overlay_fork.go': {223: 'pop', 233: 'pop', 333: 'pop'},
 'overlay_sync.go': {138: 'pop', 160: 'pop'},
 'view_issues.go': {57: 'pop', 71: 'pop', 108: 'pop', 162: 'del', 164: 'push:ViewCreate'},
 'view_prs.go': {60: 'pop', 74: 'pop', 119: 'pop', 179: 'del', 184: 'push:ViewCreate'},
 'overlay_create.go': {31: 'del'},
}
for f, ops in OPS.items():
    path = 'internal/tui/' + f
    lines = open(path).read().split('\n')
    for n, op in ops.items():
        line = lines[n - 1]
        indent = line[:len(line) - len(line.lstrip())]
        if op == 'del':
            if 'ReturnView' not in line:
                sys.exit(f'{f}:{n}: expected a ReturnView line, got {line!r}')
            lines[n - 1] = None
            continue
        if not re.search(r'\bactiveView\s*=[^=]', line):
            sys.exit(f'{f}:{n}: expected an activeView assignment, got {line!r}')
        if op.startswith('push:'):
            lines[n - 1] = f'{indent}m.pushView({op[5:]})'
        elif op == 'pop':
            lines[n - 1] = f'{indent}m.popView()'
        elif op == 'root':
            lines[n - 1] = f'{indent}m.popToRoot()'
        elif op == 'rootfork':
            lines[n - 1] = f'{indent}m.popToRoot()\n{indent}m.pushView(ViewFork)'
    open(path, 'w').write('\n'.join(l for l in lines if l is not None))
    print(f'{f}: {len(ops)} edits')
EOF
python3 $W/migrate_sites.py
```

Expected: one `N edits` line per file, no `expected ...` exit.

- [ ] **Step 6: Convert the remaining production reads**

After Step 5, the only `activeView` references left in production code are reads. Each becomes a method call:

```bash
cd internal/tui
for f in $(ls *.go | grep -v _test.go); do
  perl -pi -e 's/\.activeView\b(?!\s*\()/.activeView()/g' "$f"
done
grep -n 'activeView' *.go | grep -v _test | grep -v 'activeView()' ; cd ../..
```

Expected: the final `grep` prints only the method definition in `view_stack.go` (`func (m Model) activeView() ActiveView`).

If it prints anything else, convert it by hand:

- A write becomes the matching operation from Step 5: `esc` and cancel pop, success pops to root, opening pushes.
- A read gains `()`.

- [ ] **Step 7: Migrate the tests**

Add to `internal/tui/helpers_test.go`:

```go
// setViewForTest makes v the only frame above the dashboard root.
func (m *Model) setViewForTest(v ActiveView) {
	m.views = nil
	if v != ViewDashboard {
		m.pushView(v)
	}
}
```

Then convert writes, then reads:

```bash
cd internal/tui
for f in *_test.go; do
  perl -pi -e 's/\b(\w+)\.activeView\s*=\s*(?!=)([^\n]+?)\s*$/$1.setViewForTest($2)\n/' "$f"
  perl -pi -e 's/\.activeView\b(?!\s*\()/.activeView()/g' "$f"
done
grep -n 'activeView\s*=[^=]\|activeView:' *_test.go ; cd ../..
```

Expected: the final `grep` prints exactly one line, `helpers_test.go:38: activeView:  ViewDashboard,` in `newTestModel`'s struct literal. Delete that line (the zero stack is the dashboard root), then rerun the `grep`: it must print nothing. Writes became `setViewForTest(...)`; reads became `activeView()`.

Two tests assign the view twice (`TestPR_FocusEscUnfocuses`, `TestIssue_FocusEscUnfocuses` in `focus_test.go`). Both set `ViewPRs` or `ViewIssues` again to reset state, which `setViewForTest` does correctly. No hand edits.

- [ ] **Step 8: Build, format, and run the new test**

```bash
mise exec -- gofmt -l internal/tui
mise exec -- gofmt -w internal/tui
mise exec -- go build ./...
mise exec -- go vet ./internal/tui/
mise exec -- go test ./internal/tui/ -run TestEscRestoresCursorToSameWorktreeAfterReorder -v
```

Expected: build and vet silent; the test PASSES.

If the build fails, the message names the file and line. Fix it with the same four operations; never reintroduce a field.

- [ ] **Step 9: Run the gate**

Run: `$W/gate.sh`
Expected: `0 issues.` and all packages `ok`. **No golden file changes** (`git status --short internal/tui/testdata` prints nothing). A navigation test failure means a site got the wrong operation. Check it against the table: `esc` and cancel are `pop`, success is `root`.

- [ ] **Step 10: Commit**

```bash
git add internal/tui
git commit -m "refactor(tui): replace the active-view field with a view stack

Opening a view pushes a frame; esc pops exactly one; success returns
to the dashboard. The create wizard's ReturnView field is gone: the
stack already knows the PR or issue browser underneath.

Each frame records the selected worktree, so esc lands on the same
worktree even after a refresh reorders the list."
```

---

### Task 4: Help and update overlays become frames

**Files:**
- Modify: `internal/tui/model.go` (enum; `handleKey`; resize re-open; footer height)
- Modify: `internal/tui/view_stack.go` (`layoutView`)
- Modify: `internal/tui/view_stack_test.go`, `internal/tui/navigation_test.go`
- Modify: `internal/tui/golden_helpers_test.go` (`withHelpExpanded`)

**Interfaces:**
- Consumes: Task 3's methods.
- Produces:
  - New enum value `ViewUpdate`, appended after `ViewCheckout`.
  - `func (s viewStack) layoutView() ActiveView`: the top frame that is not `ViewHelp` or `ViewUpdate`.
  - `func (m Model) layoutView() ActiveView { return m.views.layoutView() }`

**Why `layoutView`:** footer height and help content describe the view *under* help. If they read `activeView()` with help on top, the footer height changes, the list shifts, and every help golden churns.

- [ ] **Step 1: Write the failing tests**

Append to `internal/tui/view_stack_test.go`:

```go
func TestViewStack_LayoutViewSkipsHelpAndUpdate(t *testing.T) {
	s := viewStack{}.push(viewFrame{view: ViewPRs}).push(viewFrame{view: ViewHelp})
	if got := s.layoutView(); got != ViewPRs {
		t.Errorf("layoutView with help on PRs = %v, want ViewPRs", got)
	}
	s = viewStack{}.push(viewFrame{view: ViewUpdate})
	if got := s.layoutView(); got != ViewDashboard {
		t.Errorf("layoutView with update on root = %v, want ViewDashboard", got)
	}
}
```

Append to `internal/tui/navigation_test.go`:

```go
func TestHelpOpensAsFrameAndEscCloses(t *testing.T) {
	m := newTestModel(withItems(3), withSize(120, 40))
	next, _ := m.Update(makeKeyMsg("?"))
	m = next.(Model)
	if m.activeView() != ViewHelp || !m.helpOverlay.Active {
		t.Fatalf("after ?: view=%v active=%v, want ViewHelp and true", m.activeView(), m.helpOverlay.Active)
	}
	next, _ = m.Update(makeKeyMsg("esc"))
	m = next.(Model)
	if m.activeView() != ViewDashboard || m.helpOverlay.Active {
		t.Errorf("after esc: view=%v active=%v, want ViewDashboard and false", m.activeView(), m.helpOverlay.Active)
	}
}

// The create wizard keeps ? for its text fields, so the reachable nested
// case is help over the delete prompt.
func TestHelpOverDelete_EscWalksBack(t *testing.T) {
	m := newTestModel(withItems(3), withSize(120, 40), withDeleteOverlay())
	next, _ := m.Update(makeKeyMsg("?"))
	m = next.(Model)
	if m.activeView() != ViewHelp {
		t.Fatalf("after ?: view=%v, want ViewHelp", m.activeView())
	}
	for _, want := range []ActiveView{ViewDelete, ViewDashboard} {
		next, _ = m.Update(makeKeyMsg("esc"))
		m = next.(Model)
		if m.activeView() != want {
			t.Fatalf("after esc: view=%v, want %v", m.activeView(), want)
		}
	}
}

func TestHelpKeepsFooterOfViewBeneath(t *testing.T) {
	m := newTestModel(withItems(3), withSize(120, 40))
	before := m.helpFooter.CompactHeight(m.layoutView(), m.width-4)
	next, _ := m.Update(makeKeyMsg("?"))
	m = next.(Model)
	if after := m.helpFooter.CompactHeight(m.layoutView(), m.width-4); after != before {
		t.Errorf("footer height with help open = %d, want %d", after, before)
	}
}
```

`withDeleteOverlay` sets the view through `setViewForTest` after Task 3's migration, so the stack is `[ViewDelete]` over the dashboard before `?`.

- [ ] **Step 2: Run them to verify they fail**

Run: `mise exec -- go test ./internal/tui/ -run 'TestViewStack_LayoutView|TestHelp' -v`
Expected: FAIL, `undefined: ViewUpdate` and `layoutView undefined`.

- [ ] **Step 3: Add the enum value and `layoutView`**

In `internal/tui/model.go`, add `ViewUpdate` as the last line of the `ActiveView` const block, after `ViewCheckout`.

Append to `internal/tui/view_stack.go`:

```go
// layoutView is the top view that owns layout and help context. Help and
// update overlays describe the view beneath them, so they are skipped.
func (s viewStack) layoutView() ActiveView {
	for i := len(s) - 1; i >= 0; i-- {
		if v := s[i].view; v != ViewHelp && v != ViewUpdate {
			return v
		}
	}
	return ViewDashboard
}

func (m Model) layoutView() ActiveView { return m.views.layoutView() }
```

- [ ] **Step 4: Route help and update through the stack in `handleKey`**

In `handleKey` (model.go), replace the two intercept blocks (from `// Help overlay intercepts all keys when active` through the closing brace of the `m.updateOverlay != nil && m.updateOverlay.Active` block) with:

```go
	switch m.activeView() {
	case ViewHelp:
		if key.Matches(msg, m.keys.Help) || key.Matches(msg, m.keys.Escape) {
			m.helpOverlay.Close()
			m.popView()
			return m, nil
		}
		var cmd tea.Cmd
		m.helpOverlay, cmd = m.helpOverlay.Update(msg)
		return m, cmd
	case ViewUpdate:
		if key.Matches(msg, m.keys.Update) || key.Matches(msg, m.keys.Escape) {
			m.updateOverlay.Close()
			m.popView()
		}
		return m, nil
	}
```

In the same function, after `m.helpOverlay.Open(m.activeView(), m.width, m.height)` add `m.pushView(ViewHelp)`. After `m.updateOverlay.Open(version.Version, m.updateLatestVersion, m.updateLatestURL)` add `m.pushView(ViewUpdate)`.

In the dispatch `switch m.activeView()` below, delete the old `case ViewHelp:` branch (now unreachable; Task 3 turned it into `m.popView()`).

- [ ] **Step 5: Use `layoutView` where the view beneath matters**

In `internal/tui/model.go`:

- Footer height: `m.helpFooter.CompactHeight(m.activeView(), m.width-4)` becomes `m.helpFooter.CompactHeight(m.layoutView(), m.width-4)`.
- Resize re-open: `if m.helpOverlay.Active {` (in the `tea.WindowSizeMsg` handling) becomes `if m.activeView() == ViewHelp {`.
- `handleKeyPress`: `if m.activeView() != ViewDashboard {` stays as is. Help on top routes to `handleKey`, which now handles it first.

Search for any other read of `helpOverlay.Active` or `updateOverlay.Active` in production code: `grep -n 'Overlay.Active' internal/tui/*.go | grep -v _test | grep -v -E '^internal/tui/(help|update)_overlay.go'`. Expected after this step: only the two in `compositeActiveOverlay`, which Task 5 deletes.

- [ ] **Step 6: Update the golden builder**

In `internal/tui/golden_helpers_test.go`, `withHelpExpanded` becomes:

```go
func withHelpExpanded() testOpt {
	return func(m *Model) {
		m.helpOverlay.Open(m.activeView(), m.width, m.height)
		m.pushView(ViewHelp)
	}
}
```

- [ ] **Step 7: Run the tests and the gate**

Run: `mise exec -- go test ./internal/tui/ -run 'TestViewStack|TestHelp|TestRestoreCursor|TestEsc' -v`
Expected: PASS, including `TestHelpOverDelete_EscWalksBack`.

Run: `$W/gate.sh`
Expected: `0 issues.`, all `ok`, **no golden changes**. Help rendering still goes through `compositeActiveOverlay` until Task 5, which reads `helpOverlay.Active`; `Open` and `Close` still set it.

- [ ] **Step 8: Commit**

```bash
git add internal/tui
git commit -m "refactor(tui): open help and update overlays as stack frames

The help and update overlays were flags checked before every key. They
are now frames, so esc from help returns to whatever it covered,
including another overlay. Footer height and help content read the
view beneath them."
```

---

### Task 5: Render frames by walking the stack

**Files:**
- Modify: `internal/tui/model.go` (`viewContent`, `viewForActiveView`, `compositeActiveOverlay`, `overlayOnDashboard`, `centerOverlay`)
- Modify: `internal/tui/view_prs_v2.go:132`, `internal/tui/view_issues.go:296`
- Modify: `internal/tui/navigation_test.go`

**Interfaces:**
- Consumes: Tasks 3 and 4.
- Produces:
  - `func (m Model) renderFrame(i int) string`: frame `i` of the stack, `-1` for the dashboard root.
  - `func (m Model) renderOverlay(v ActiveView) string`: the panel for an overlay view, or `""` when its state is nil.
  - `centerOverlay`, `overlayOnDashboard`, `compositeActiveOverlay`, and `viewForActiveView` are deleted.

- [ ] **Step 1: Write the failing test**

Append to `internal/tui/navigation_test.go`:

```go
import "github.com/charmbracelet/x/ansi"
```

(Merge into the file's import block.)

```go
// An overlay draws over whatever frame it was opened from, so help over
// the delete overlay keeps the delete box's text visible beside it.
func TestRenderFrame_HelpOverDeleteShowsBothLayers(t *testing.T) {
	m := goldenModel(t, sizeUltraWide, withItems(5), withDeleteOverlay())
	withHelpExpanded()(&m)
	out := ansi.Strip(m.viewString())
	if !strings.Contains(out, "Navigation") {
		t.Error("help panel missing")
	}
	if !strings.Contains(out, "test commit for root") {
		t.Error("dashboard rows missing beneath both overlays")
	}
	if !strings.Contains(out, "Delete") {
		t.Error("delete overlay missing beneath help")
	}
}
```

Add `"strings"` to the imports as well.

- [ ] **Step 2: Run it**

Run: `mise exec -- go test ./internal/tui/ -run TestRenderFrame -v`

Expected: it may already PASS, and that is fine.

- `compositeActiveOverlay` already draws help over the delete composite today.
- This test guards the rewrite, not a new behavior. Record "characterization: passes before and after" in the ledger.
- If the delete box's title is not "Delete" at 160 columns, use a word from `renderDeleteV2`'s output.

- [ ] **Step 3: Replace the render path**

In `internal/tui/model.go`:

1. In `viewContent`, replace

```go
	result := m.viewForActiveView()
	if result == "" {
		result = m.renderDashboard()
	}
	return m.compositeActiveOverlay(result)
```

with

```go
	return m.renderFrame(len(m.views) - 1)
```

2. Delete `viewForActiveView`, `compositeActiveOverlay`, `overlayOnDashboard`, and `centerOverlay` (with their doc comments).

3. Add:

```go
// renderFrame renders frame i of the view stack; -1 is the dashboard root.
// Overlay frames draw over the frame beneath them, so an overlay opened
// from the PR browser shows the PR browser behind it.
func (m Model) renderFrame(i int) string {
	v := ViewDashboard
	if i >= 0 {
		v = m.views[i].view
	}
	switch v {
	case ViewDashboard:
		return m.renderDashboard()
	case ViewPRs:
		if m.prState != nil {
			return m.renderPRPanel()
		}
		return m.renderFrame(i - 1)
	case ViewIssues:
		if m.issueState != nil {
			return m.renderIssuePanel()
		}
		return m.renderFrame(i - 1)
	}
	overlay := m.renderOverlay(v)
	if overlay == "" {
		return m.renderFrame(i - 1)
	}
	return composeOverlay(m.renderFrame(i-1), overlay, m.width, m.height)
}

// renderOverlay returns the panel for an overlay view, or "" when its state
// is gone.
func (m Model) renderOverlay(v ActiveView) string {
	switch v {
	case ViewDelete:
		if m.deleteState != nil {
			return renderDeleteV2(m.deleteState, m.width)
		}
	case ViewCreate:
		if m.createState != nil {
			return renderCreateV2(m.createState, m.width, m.spinner.View())
		}
	case ViewBulk:
		if m.bulkState != nil {
			return renderBulk(m.bulkState)
		}
	case ViewFork:
		if m.forkState != nil {
			return renderFork(m.forkState, m.width)
		}
	case ViewSync:
		if m.syncState != nil {
			return renderSync(m.syncState, m.width)
		}
	case ViewConfig:
		if m.configState != nil {
			return renderConfig(m.configState, m.width)
		}
	case ViewRename:
		if m.renameState != nil {
			return renderRename(m.renameState, m.width)
		}
	case ViewCheckout:
		if m.checkoutState != nil {
			return renderCheckout(m.checkoutState, m.width)
		}
	case ViewHelp:
		return m.helpOverlay.View(m.width, m.height)
	case ViewUpdate:
		if m.updateOverlay != nil {
			return m.updateOverlay.View(m.width, m.height)
		}
	}
	return ""
}
```

Before deleting `viewForActiveView`, compare its cases against `renderOverlay`:

- Every overlay it rendered must appear in `renderOverlay`, with the same render call and arguments.
- Its `ViewCreate` case chose the PR or issue panel as background by `createState.Source`.
- The stack now supplies that background, because the create frame sits on the PR or issue frame.

4. In `internal/tui/view_prs_v2.go:132` and `internal/tui/view_issues.go:296`, replace `centerOverlay(` with `composeOverlay(`. The arguments are unchanged.

- [ ] **Step 4: Build and run the tests**

```bash
mise exec -- go build ./...
mise exec -- go test ./internal/tui/ -run 'TestRenderFrame|TestHelp|TestEsc|TestOverlay_KeepsBackgroundRowsVisible' -v
```

Expected: PASS.

- [ ] **Step 5: Run the gate**

Run: `$W/gate.sh`
Expected: `0 issues.`, all `ok`, and **no golden changes**.

The create-wizard golden files are the most likely to move, since `Source` used to choose their background:

- If one changes, the frame under `ViewCreate` in that golden's builder is wrong.
- `withCreateStep` must leave the stack as `[ViewCreate]` over the dashboard, which matches its old background.
- Fix the builder, not the renderer.

- [ ] **Step 6: Commit**

```bash
git add internal/tui
git commit -m "refactor(tui): render overlays by walking the view stack

Each overlay frame draws over the frame beneath it. This replaces
overlayOnDashboard, compositeActiveOverlay, the create wizard's
background special case, and the centerOverlay wrapper."
```

---

### Task 6: Phase 1 follow-ups and docs

**Files:**
- Modify: `internal/tui/compositor_test.go`
- Modify: `internal/tui/pr_preview.go:10-16` (comment only)
- Modify: `docs/TUI.md`
- Modify: `CHANGELOG.md`

**Interfaces:** none.

- [ ] **Step 1: Pin the wide-glyph cut**

The Phase 1 reviewer verified this output against ultraviolet's source. Append to `internal/tui/compositor_test.go`:

```go
// An overlay edge that lands inside a wide glyph blanks the orphaned half.
func TestComposeOverlay_BlanksHalfCutWideGlyph(t *testing.T) {
	got := ansi.Strip(composeOverlay("界界界界界", "XXX", 10, 1))
	if got != "界 XXX界界" {
		t.Errorf("got %q, want %q", got, "界 XXX界界")
	}
}
```

Run: `mise exec -- go test ./internal/tui/ -run TestComposeOverlay_BlanksHalfCutWideGlyph -v`
Expected: PASS. This pins existing behavior, so it passes immediately. If the output differs, record the actual string in the ledger and report it. Do not change the assertion to match.

- [ ] **Step 2: Fix the stale glamour comment**

In `internal/tui/pr_preview.go`, the comment above `renderMarkdown` justifies `WithStandardStyle("dark")` by saying `WithAutoStyle()` sends an OSC 11 terminal query. glamour v2 no longer queries the terminal. Replace the last paragraph of that comment with:

```go
// Uses a fixed "dark" style: glamour v2 does not detect the terminal
// background, so the style is chosen here.
```

- [ ] **Step 3: Update `docs/TUI.md`**

- Under `## Overlays`, after "Each overlay opens centered over the dimmed dashboard background. Press `esc` to close or cancel.", add: "`esc` goes back exactly one level. An overlay opened from the PR or issue browser returns there, and the dashboard cursor returns to the worktree it was on."
- In `### File Map`, add three rows:

```markdown
| `internal/tui/view_stack.go` | View stack: frames, push and pop, cursor restore |
| `internal/tui/compositor.go` | Draws an overlay over a dimmed background (`composeOverlay`) |
| `internal/tui/worktree_list.go` | The dashboard list constructor (`newWorktreeList`) |
```

- [ ] **Step 4: Add the changelog entry**

Under `## [Unreleased]` in `CHANGELOG.md`, add to `### Fixed` (create the heading if the section has none):

```markdown
- `esc` in the dashboard goes back exactly one level, and returns the cursor to the worktree it was on, even after the list refreshes.
```

- [ ] **Step 5: Gate and commit**

Run: `$W/gate.sh`
Expected: `0 issues.`, all `ok`.

```bash
git add internal/tui/compositor_test.go internal/tui/pr_preview.go docs/TUI.md CHANGELOG.md
git commit -m "docs(tui): document the view stack and pin the wide-glyph cut"
```

---

### Task 7: Hands-on check in the herdr session

The user granted permission to run the TUI in the current herdr session. No other session is using it.

**Files:** none changed.

**Interfaces:** none.

- [ ] **Step 1: Build a dev binary**

```bash
mise exec -- go build -o $TMPDIR/grove-dev ./cmd/grove
```

- [ ] **Step 2: Run it in a herdr pane**

Load the `herdr` skill. Open a pane in the main grove checkout (the sibling directory `../grove`) and run `$TMPDIR/grove-dev`.

**Safety rules for this session:**

- Never press `y` in a delete prompt, and never `⏎` in the create, fork, rename, switch-branch, or sync flows. Leave every flow with `esc`.
- Do not press `⏎` or `U` on the dashboard; they switch worktrees.
- Quit with `q`.

- [ ] **Step 3: Walk these paths and capture the pane after each**

| # | Keys | Expected |
|---|---|---|
| 1 | `j` `j` then `R` then `esc` | Rename opens over the dashboard; `esc` closes it; cursor on the same worktree |
| 2 | `?` then `esc` | Help opens and closes; footer unchanged |
| 3 | `d` then `?` then `esc` `esc` | Help over the delete box; first `esc` shows the delete box again; second closes it |
| 4 | `p`, wait for PRs, `n` on a PR, then `esc` | Create wizard over the PR browser; `esc` returns to the PR browser, not the dashboard. Skip if `gh` is not authenticated in that pane, and say so. |
| 5 | Resize the pane narrower and wider with help open | Help stays open and re-centers |

Record each result in the ledger as `Task 7: path N: <observed>`. A mismatch is a bug: write a failing test for it in `navigation_test.go`, fix it, and gate before continuing.

- [ ] **Step 4: Quit and close the pane**

Press `q`. Close the pane you opened. Leave other panes alone.

---

## Done when

- `make lint test` passes with golangci-lint v2.14 on the final commit.
- No production file mentions `activeView` as a field, `ReturnView`, `overlayOnDashboard`, `compositeActiveOverlay`, or `centerOverlay`.
- `TestEscRestoresCursorToSameWorktreeAfterReorder` failed before Task 3 and passes after.
- Zero golden files changed across the whole branch.
- Task 7's five paths match their expected results.
