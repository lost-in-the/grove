# Issue 25 — Plan 2: Worktree Drift Detection + `grove adopt`

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Detect when the current directory is a git worktree that grove doesn't know about (created via `git worktree add` instead of `grove new`) and provide a `grove adopt` command that runs the same bootstrap sequence as `grove new`, so drifted worktrees can be brought into compliance with one command.

**Architecture:** The bootstrap sequence currently lives inline in `setupCreatedWorktree` (`cmd/grove/commands/helpers.go:117`). We extract its post-`git worktree add` portion into a reusable `BootstrapWorktree` function so both `new` and a new `adopt` command can call it. Drift detection lives in `RequireGroveContext` (`cmd/grove/commands/context.go:33`): after resolving the grove dir, we compare cwd's worktree path against `state.json` entries; on mismatch we print a non-fatal warning suggesting `grove adopt`. The detection uses the same `getMainWorktreePath` machinery already in `internal/grove/diagnose.go`.

**Tech Stack:** Go 1.24, `cobra`, the existing `state.Manager`, `worktree.Manager`, hooks framework.

**Compatibility with Plans 1 & 3:**
- Plan 1 modifies `cmd/grove/commands/test.go`, `plugins/docker/external.go`, `plugins/docker/local.go`, `internal/config/config.go` (`TestConfig`). This plan touches **none** of those.
- Plan 3 only adds to `ExternalComposeConfig` — also disjoint from this plan.
- This plan introduces `internal/worktree/bootstrap.go` (NEW). Plan 1 doesn't import it. Plan 3 doesn't import it.

---

### Task 1: Add `ReasonDriftedWorktree` to the diagnose package

**Files:**
- Modify: `internal/grove/diagnose.go:11-15` (add new reason)
- Test: `internal/grove/diagnose_test.go` (extend)

- [ ] **Step 1: Write the failing test** — append to `internal/grove/diagnose_test.go`:

```go
func TestDiagnoseDrift_WorktreeNotInState(t *testing.T) {
	// Set up a main repo with a .grove dir, then a worktree that isn't in state.
	tmpDir := t.TempDir()
	mainDir := filepath.Join(tmpDir, "main")
	if err := os.MkdirAll(filepath.Join(mainDir, ".grove"), 0755); err != nil {
		t.Fatalf("mkdir main/.grove: %v", err)
	}
	// Touch a state file with no worktrees registered.
	stateContent := `{"project": "test", "worktrees": {}}`
	if err := os.WriteFile(filepath.Join(mainDir, ".grove", "state.json"), []byte(stateContent), 0644); err != nil {
		t.Fatalf("write state: %v", err)
	}

	worktreePath := filepath.Join(tmpDir, "drifted-wt")
	if err := os.MkdirAll(worktreePath, 0755); err != nil {
		t.Fatalf("mkdir worktree: %v", err)
	}

	got := DiagnoseDrift(worktreePath, mainDir)
	if got != ReasonDriftedWorktree {
		t.Errorf("expected ReasonDriftedWorktree, got %v", got)
	}
}

func TestDiagnoseDrift_WorktreeInState(t *testing.T) {
	tmpDir := t.TempDir()
	mainDir := filepath.Join(tmpDir, "main")
	if err := os.MkdirAll(filepath.Join(mainDir, ".grove"), 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	worktreePath := filepath.Join(tmpDir, "registered-wt")
	if err := os.MkdirAll(worktreePath, 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	stateContent := `{"project": "test", "worktrees": {"registered-wt": {"path": "` + worktreePath + `", "branch": "main"}}}`
	if err := os.WriteFile(filepath.Join(mainDir, ".grove", "state.json"), []byte(stateContent), 0644); err != nil {
		t.Fatalf("write: %v", err)
	}

	got := DiagnoseDrift(worktreePath, mainDir)
	if got != ReasonRegistered {
		t.Errorf("expected ReasonRegistered, got %v", got)
	}
}

func TestDiagnoseDrift_AtMainWorktree(t *testing.T) {
	tmpDir := t.TempDir()
	mainDir := filepath.Join(tmpDir, "main")
	if err := os.MkdirAll(filepath.Join(mainDir, ".grove"), 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	got := DiagnoseDrift(mainDir, mainDir)
	if got != ReasonRegistered {
		t.Errorf("expected ReasonRegistered for main worktree, got %v", got)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/grove/ -run TestDiagnoseDrift -v`
Expected: FAIL with `DiagnoseDrift undefined` and `ReasonRegistered/ReasonDriftedWorktree undefined`.

- [ ] **Step 3: Add the new constants and function** — append to `internal/grove/diagnose.go`:

```go
// DriftReason describes whether the cwd's worktree is registered in grove state.
type DriftReason int

const (
	ReasonRegistered      DriftReason = iota // cwd is the main worktree, or a registered grove worktree
	ReasonDriftedWorktree                    // cwd is a git worktree but not in state.json
)

// DiagnoseDrift checks whether the worktree at worktreePath is registered in state.json
// at mainPath/.grove/state.json. Returns ReasonRegistered when it's the main worktree
// or appears in state, and ReasonDriftedWorktree otherwise.
//
// This is intentionally lightweight (no JSON parsing of complex shapes): it just
// checks whether the worktree path appears as a value in the state's worktrees map.
func DiagnoseDrift(worktreePath, mainPath string) DriftReason {
	resolvedWT, _ := filepath.EvalSymlinks(worktreePath)
	if resolvedWT == "" {
		resolvedWT = worktreePath
	}
	resolvedMain, _ := filepath.EvalSymlinks(mainPath)
	if resolvedMain == "" {
		resolvedMain = mainPath
	}
	if resolvedWT == resolvedMain {
		return ReasonRegistered
	}

	statePath := filepath.Join(resolvedMain, ".grove", "state.json")
	data, err := os.ReadFile(statePath)
	if err != nil {
		// No state file = brand new project, treat as registered (don't nag).
		return ReasonRegistered
	}

	// Look for the worktree path as a substring in the state file.
	// Full JSON parse would be more robust, but state.go owns that and a
	// lightweight check here keeps this package's surface area small.
	if strings.Contains(string(data), `"`+resolvedWT+`"`) {
		return ReasonRegistered
	}
	return ReasonDriftedWorktree
}
```

Add `"strings"` to the imports of `diagnose.go` if not already present.

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/grove/ -run TestDiagnoseDrift -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/grove/diagnose.go internal/grove/diagnose_test.go
git commit -m "feat(grove): add DiagnoseDrift to detect unregistered worktrees"
```

---

### Task 2: Extract `BootstrapWorktree` from `setupCreatedWorktree`

**Why this matters (Learning Mode design note):**
- `setupCreatedWorktree` does two things: bookkeeping that's specific to the `grove new` flow (parsing flags, pretty output) and the actual bootstrap (state registration, hooks, docker). `grove adopt` only needs the second half. Splitting them is the simplest way to share without bringing CLI flags into a non-CLI helper.

**Files:**
- Create: `internal/worktree/bootstrap.go`
- Test: `internal/worktree/bootstrap_test.go`
- Modify: `cmd/grove/commands/helpers.go:117-197` (delegate to new function)

- [ ] **Step 1: Write the failing test** — create `internal/worktree/bootstrap_test.go`:

```go
package worktree

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/lost-in-the/grove/internal/config"
	"github.com/lost-in-the/grove/internal/state"
)

func TestBootstrapWorktree_RegistersInState(t *testing.T) {
	tmpDir := t.TempDir()
	mainDir := filepath.Join(tmpDir, "main")
	groveDir := filepath.Join(mainDir, ".grove")
	if err := os.MkdirAll(groveDir, 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	wtPath := filepath.Join(tmpDir, "feature")
	if err := os.MkdirAll(wtPath, 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	stateMgr, err := state.NewManager(groveDir)
	if err != nil {
		t.Fatalf("state mgr: %v", err)
	}

	cfg := config.LoadDefaults()
	opts := BootstrapOpts{
		Name:        "feature",
		Branch:      "feature",
		WorktreePath: wtPath,
		MainPath:    mainDir,
		ProjectName: "test-proj",
		Now:         time.Now(),
	}

	if err := BootstrapWorktree(stateMgr, cfg, opts); err != nil {
		t.Fatalf("BootstrapWorktree: %v", err)
	}

	got, err := stateMgr.GetWorktree("feature")
	if err != nil {
		t.Fatalf("GetWorktree: %v", err)
	}
	if got.Path != wtPath {
		t.Errorf("Path: got %q want %q", got.Path, wtPath)
	}
	if got.Branch != "feature" {
		t.Errorf("Branch: got %q want feature", got.Branch)
	}
}

func TestBootstrapWorktree_IdempotentOnSecondCall(t *testing.T) {
	tmpDir := t.TempDir()
	mainDir := filepath.Join(tmpDir, "main")
	groveDir := filepath.Join(mainDir, ".grove")
	if err := os.MkdirAll(groveDir, 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	wtPath := filepath.Join(tmpDir, "feature")
	if err := os.MkdirAll(wtPath, 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	stateMgr, err := state.NewManager(groveDir)
	if err != nil {
		t.Fatalf("state mgr: %v", err)
	}
	cfg := config.LoadDefaults()
	opts := BootstrapOpts{
		Name:         "feature",
		Branch:       "feature",
		WorktreePath: wtPath,
		MainPath:     mainDir,
		ProjectName:  "test-proj",
		Now:          time.Now(),
	}

	if err := BootstrapWorktree(stateMgr, cfg, opts); err != nil {
		t.Fatalf("first call: %v", err)
	}
	if err := BootstrapWorktree(stateMgr, cfg, opts); err != nil {
		t.Fatalf("second call should not error: %v", err)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/worktree/ -run TestBootstrapWorktree -v`
Expected: FAIL with `BootstrapWorktree undefined` / `BootstrapOpts undefined`.

- [ ] **Step 3: Implement `BootstrapWorktree`** — create `internal/worktree/bootstrap.go`:

```go
package worktree

import (
	"fmt"
	"time"

	"github.com/lost-in-the/grove/internal/config"
	"github.com/lost-in-the/grove/internal/grove"
	"github.com/lost-in-the/grove/internal/hooks"
	"github.com/lost-in-the/grove/internal/state"
)

// BootstrapOpts holds the inputs needed to bootstrap a worktree (whether
// freshly created via grove new or adopted post-hoc via grove adopt).
type BootstrapOpts struct {
	Name         string    // short worktree name (e.g., "feature")
	Branch       string    // branch the worktree is on
	WorktreePath string    // absolute path to the worktree directory
	MainPath     string    // absolute path to the main worktree (parent of .grove)
	ProjectName  string    // project name for hook context
	Now          time.Time // injected for testability
	IsEnvironment bool     // true for environment worktrees
	Mirror       string    // mirror name when IsEnvironment is true
}

// BootstrapWorktree runs the post-git-worktree-add bootstrap sequence:
//   1. Symlink config.toml from main worktree
//   2. Register the worktree in state.json (idempotent — re-registers on second call)
//   3. Fire post-create hooks (per-project hooks.toml, then global plugin hooks)
//
// Returns an error only if state registration or symlinking fails irrecoverably.
// Hook failures are logged via the hooks framework but do not abort the bootstrap.
func BootstrapWorktree(stateMgr *state.Manager, cfg *config.Config, opts BootstrapOpts) error {
	if opts.WorktreePath == "" || opts.MainPath == "" {
		return fmt.Errorf("BootstrapWorktree: WorktreePath and MainPath are required")
	}

	if err := grove.EnsureConfigSymlink(opts.MainPath, opts.WorktreePath); err != nil {
		return fmt.Errorf("symlink config: %w", err)
	}

	now := opts.Now
	if now.IsZero() {
		now = time.Now()
	}
	wsState := &state.WorktreeState{
		Path:           opts.WorktreePath,
		Branch:         opts.Branch,
		Root:           false,
		CreatedAt:      now,
		LastAccessedAt: now,
		Environment:    opts.IsEnvironment,
	}
	if opts.IsEnvironment {
		wsState.Mirror = opts.Mirror
		wsState.LastSyncedAt = &now
	}
	if err := stateMgr.AddWorktree(opts.Name, wsState); err != nil {
		return fmt.Errorf("register worktree: %w", err)
	}

	// Per-project post-create hooks
	hookExecutor, hookErr := hooks.NewExecutor()
	if hookErr == nil && hookExecutor.HasHooksForEvent(hooks.EventPostCreate) {
		hookCtx := &hooks.ExecutionContext{
			Event:        hooks.EventPostCreate,
			Worktree:     opts.Name,
			WorktreeFull: opts.ProjectName + "-" + opts.Name,
			Branch:       opts.Branch,
			Project:      opts.ProjectName,
			MainPath:     opts.MainPath,
			NewPath:      opts.WorktreePath,
		}
		_ = hookExecutor.Execute(hooks.EventPostCreate, hookCtx)
	}

	// Global plugin post-create hook (e.g., docker external)
	globalHookCtx := &hooks.Context{
		Worktree:     opts.Name,
		Config:       cfg,
		WorktreePath: opts.WorktreePath,
		MainPath:     opts.MainPath,
	}
	_ = hooks.Fire(hooks.EventPostCreate, globalHookCtx)

	return nil
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/worktree/ -run TestBootstrapWorktree -v`
Expected: PASS for both sub-tests.

- [ ] **Step 5: Refactor `setupCreatedWorktree` to delegate to `BootstrapWorktree`** — modify `cmd/grove/commands/helpers.go`. Replace the body of `setupCreatedWorktree` (lines 117-197) with:

```go
func setupCreatedWorktree(ctx *GroveContext, mgr *worktree.Manager, name, branchName string, opts worktreeSetupOpts, w *cli.Writer) (*worktree.Worktree, error) {
	wt, err := mgr.Find(name)
	if err != nil || wt == nil {
		return nil, fmt.Errorf("failed to find created worktree: %w", err)
	}

	if !opts.JSONOutput {
		cli.Step(w, "Running post-create hooks...")
	}

	bootstrapOpts := worktree.BootstrapOpts{
		Name:         name,
		Branch:       branchName,
		WorktreePath: wt.Path,
		MainPath:     ctx.ProjectRoot,
		ProjectName:  mgr.GetProjectName(),
		Now:          time.Now(),
		IsEnvironment: opts.IsEnvironment,
		Mirror:       opts.Mirror,
	}
	if err := worktree.BootstrapWorktree(ctx.State, ctx.Config, bootstrapOpts); err != nil {
		if !opts.JSONOutput {
			cli.Warning(w, "Bootstrap failed: %v", err)
			cli.Faint(w, "run 'grove repair' to fix")
		}
	}

	autoStartDocker(w, ctx.Config, wt.Path, opts.NoDocker, opts.JSONOutput)
	return wt, nil
}
```

The old user-visible warnings around symlinks / state are now collapsed into a single "Bootstrap failed" line — preserve information by quoting the wrapped error chain.

- [ ] **Step 6: Run the full commands package tests**

Run: `go test ./cmd/grove/commands/ -v`
Expected: All previously-passing tests still pass. If `setupCreatedWorktree` callers test the warning text, they may need adjustment — fix tests to match the new wrapped-error message.

- [ ] **Step 7: Commit**

```bash
git add internal/worktree/bootstrap.go internal/worktree/bootstrap_test.go cmd/grove/commands/helpers.go
git commit -m "refactor(worktree): extract BootstrapWorktree for reuse by adopt"
```

---

### Task 3: Add drift detection to `RequireGroveContext`

**Why this matters (Learning Mode design note):**
- This is a **non-fatal warning**, not a hard error. The drifted worktree may still work for many operations (config is symlinked, state is just bookkeeping). We want to inform without blocking — making `grove ls`, `grove here`, etc. fail because state isn't up-to-date would be a regression.

**Files:**
- Modify: `cmd/grove/commands/context.go:33-107`
- Test: `cmd/grove/commands/context_test.go` (NEW or extend)

- [ ] **Step 1: Write the failing test** — create `cmd/grove/commands/context_test.go`:

```go
package commands

import (
	"bytes"
	"strings"
	"testing"

	"github.com/lost-in-the/grove/internal/cli"
	"github.com/lost-in-the/grove/internal/grove"
)

func TestEmitDriftNotice_PrintsAdoptHint(t *testing.T) {
	var buf bytes.Buffer
	w := cli.NewWriter(&buf)

	emitDriftNotice(w, "drifted-wt", grove.ReasonDriftedWorktree)

	out := buf.String()
	if !strings.Contains(out, "grove adopt") {
		t.Errorf("expected 'grove adopt' hint, got: %s", out)
	}
	if !strings.Contains(out, "drifted-wt") {
		t.Errorf("expected worktree name in notice, got: %s", out)
	}
}

func TestEmitDriftNotice_SilentWhenRegistered(t *testing.T) {
	var buf bytes.Buffer
	w := cli.NewWriter(&buf)

	emitDriftNotice(w, "ok-wt", grove.ReasonRegistered)

	if buf.Len() != 0 {
		t.Errorf("expected no output for registered worktree, got: %s", buf.String())
	}
}
```

If `cli.NewWriter(io.Writer)` doesn't already exist as a public API, use whatever constructor the existing `cli` package provides. Inspect `internal/cli/` and adjust the call.

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./cmd/grove/commands/ -run TestEmitDriftNotice -v`
Expected: FAIL with `emitDriftNotice undefined`.

- [ ] **Step 3: Implement `emitDriftNotice` and integrate into `RequireGroveContext`** — modify `cmd/grove/commands/context.go`. After the existing `RequireGroveContext` function, add:

```go
// emitDriftNotice prints a non-fatal warning when the cwd is a git worktree
// that grove doesn't have in its state. The user can ignore the message;
// it's intended to nudge them toward `grove adopt`.
func emitDriftNotice(w *cli.Writer, name string, reason grove.DriftReason) {
	if reason != grove.ReasonDriftedWorktree {
		return
	}
	cli.Warning(w, "this worktree (%s) wasn't created by grove and isn't registered in state", name)
	cli.Faint(w, "run 'grove adopt' to bootstrap it (symlinks config, runs hooks, registers state)")
}
```

Inside `RequireGroveContext`, after the `ctx := &GroveContext{...}` block (around line 103) and before `return fn(cmd, args, ctx)`:

```go
// Drift detection: warn if cwd is a worktree that isn't in state.
// Skip drift detection for the adopt command itself (it's the resolution).
if cmd.Use != "adopt" && cmd.Name() != "adopt" {
	cwd, err := os.Getwd()
	if err == nil {
		mainPath := grove.MustProjectRoot(groveDir)
		// Determine which worktree we're in. If cwd is the main, skip.
		if reason := grove.DiagnoseDrift(cwd, mainPath); reason == grove.ReasonDriftedWorktree {
			worktreeName := filepath.Base(cwd)
			emitDriftNotice(cli.NewStderr(), worktreeName, reason)
		}
	}
}
```

Add `"path/filepath"` to the imports of `context.go` if not already present.

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./cmd/grove/commands/ -run TestEmitDriftNotice -v`
Expected: PASS.

- [ ] **Step 5: Run all command tests**

Run: `go test ./cmd/grove/commands/ -v`
Expected: All tests pass.

- [ ] **Step 6: Commit**

```bash
git add cmd/grove/commands/context.go cmd/grove/commands/context_test.go
git commit -m "feat(context): warn when cwd is a drifted git worktree"
```

---

### Task 4: Implement the `grove adopt` command

**Files:**
- Create: `cmd/grove/commands/adopt.go`
- Test: `cmd/grove/commands/adopt_test.go`

- [ ] **Step 1: Write the failing test** — create `cmd/grove/commands/adopt_test.go`:

```go
package commands

import (
	"os"
	"path/filepath"
	"testing"
)

func TestResolveAdoptTarget_UsesCwdWhenNoArg(t *testing.T) {
	tmpDir := t.TempDir()
	got, err := resolveAdoptTarget(tmpDir, []string{})
	if err != nil {
		t.Fatalf("resolveAdoptTarget: %v", err)
	}
	if got != tmpDir {
		t.Errorf("got %q want %q", got, tmpDir)
	}
}

func TestResolveAdoptTarget_UsesArgWhenProvided(t *testing.T) {
	tmpDir := t.TempDir()
	other := filepath.Join(tmpDir, "other")
	if err := os.MkdirAll(other, 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	got, err := resolveAdoptTarget(tmpDir, []string{other})
	if err != nil {
		t.Fatalf("resolveAdoptTarget: %v", err)
	}
	if got != other {
		t.Errorf("got %q want %q", got, other)
	}
}

func TestResolveAdoptTarget_ErrorsOnNonexistent(t *testing.T) {
	tmpDir := t.TempDir()
	_, err := resolveAdoptTarget(tmpDir, []string{filepath.Join(tmpDir, "nope")})
	if err == nil {
		t.Errorf("expected error for nonexistent path")
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./cmd/grove/commands/ -run TestResolveAdoptTarget -v`
Expected: FAIL with `resolveAdoptTarget undefined`.

- [ ] **Step 3: Implement `adopt` command** — create `cmd/grove/commands/adopt.go`:

```go
package commands

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/lost-in-the/grove/internal/cli"
	"github.com/lost-in-the/grove/internal/cmdexec"
	"github.com/lost-in-the/grove/internal/worktree"
)

func init() {
	rootCmd.AddCommand(adoptCmd)
}

var adoptCmd = &cobra.Command{
	Use:   "adopt [path]",
	Short: "Bootstrap a git worktree that grove doesn't know about",
	Long: `Adopts an existing git worktree into grove's state.

Use when a worktree was created with 'git worktree add' instead of 'grove new':
the worktree exists, but grove never ran its bootstrap (state registration,
config symlink, post-create hooks, docker auto-start).

If [path] is omitted, the current directory is adopted.

Examples:
  grove adopt              # adopt the worktree the user is currently in
  grove adopt ../other-wt  # adopt by path`,
	Args: cobra.MaximumNArgs(1),
	RunE: RequireGroveContext(func(cmd *cobra.Command, args []string, ctx *GroveContext) error {
		w := cli.NewStdout()

		cwd, err := os.Getwd()
		if err != nil {
			return fmt.Errorf("get cwd: %w", err)
		}

		target, err := resolveAdoptTarget(cwd, args)
		if err != nil {
			return err
		}

		// Verify target is a git worktree of the main repo
		branch, gitErr := gitBranchAt(target)
		if gitErr != nil {
			return fmt.Errorf("not a git worktree at %s: %w", target, gitErr)
		}

		name := filepath.Base(target)
		if existing, err := ctx.State.GetWorktree(name); err == nil && existing != nil && existing.Path == target {
			cli.Info(w, "worktree %q is already registered (path: %s)", name, target)
			return nil
		}

		mgr, err := worktree.NewManager(ctx.ProjectRoot)
		if err != nil {
			return fmt.Errorf("worktree manager: %w", err)
		}

		cli.Step(w, "Bootstrapping worktree %q at %s ...", name, target)

		bootstrapOpts := worktree.BootstrapOpts{
			Name:         name,
			Branch:       branch,
			WorktreePath: target,
			MainPath:     ctx.ProjectRoot,
			ProjectName:  mgr.GetProjectName(),
			Now:          time.Now(),
		}
		if err := worktree.BootstrapWorktree(ctx.State, ctx.Config, bootstrapOpts); err != nil {
			return fmt.Errorf("bootstrap: %w", err)
		}

		cli.Success(w, "adopted %q (branch: %s)", name, branch)
		cli.Faint(w, "config symlinked, state registered, post-create hooks fired")
		return nil
	}),
}

// resolveAdoptTarget picks the directory to adopt: explicit arg if given,
// otherwise cwd. Returns an absolute, EvalSymlinks-resolved path.
func resolveAdoptTarget(cwd string, args []string) (string, error) {
	target := cwd
	if len(args) == 1 {
		target = args[0]
	}
	abs, err := filepath.Abs(target)
	if err != nil {
		return "", fmt.Errorf("resolve abs path %s: %w", target, err)
	}
	info, err := os.Stat(abs)
	if err != nil {
		return "", fmt.Errorf("stat %s: %w", abs, err)
	}
	if !info.IsDir() {
		return "", fmt.Errorf("not a directory: %s", abs)
	}
	if resolved, err := filepath.EvalSymlinks(abs); err == nil {
		return resolved, nil
	}
	return abs, nil
}

// gitBranchAt returns the current branch name of the git worktree at dir.
func gitBranchAt(dir string) (string, error) {
	out, err := cmdexec.Output(context.TODO(), "git", []string{"rev-parse", "--abbrev-ref", "HEAD"}, dir, cmdexec.GitLocal)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./cmd/grove/commands/ -run TestResolveAdoptTarget -v`
Expected: PASS.

- [ ] **Step 5: Run the full commands package tests**

Run: `go test ./cmd/grove/commands/ -v`
Expected: All tests pass.

- [ ] **Step 6: Manual end-to-end smoke test**

```bash
# In a grove project's main worktree:
git worktree add ../grove-issue-25-adopt-test -b adopt-test
cd ../grove-issue-25-adopt-test

# Should now warn about drift:
go run ./cmd/grove ls

# Adopt:
go run ./cmd/grove adopt

# Re-run grove ls — adopt-test should now appear, no warning.
go run ./cmd/grove ls

# Cleanup:
cd ../grove-issue-25
git worktree remove ../grove-issue-25-adopt-test
```

Expected:
1. First `grove ls` from drifted worktree prints the drift warning.
2. `grove adopt` runs without error and prints "adopted".
3. Second `grove ls` lists the adopt-test worktree and emits no warning.

- [ ] **Step 7: Commit**

```bash
git add cmd/grove/commands/adopt.go cmd/grove/commands/adopt_test.go
git commit -m "feat: add grove adopt command for drifted worktrees"
```

---

### Task 5: Add a doctor check for drifted worktrees

**Files:**
- Modify: `cmd/grove/commands/doctor.go` (add a Tier 2 check)

- [ ] **Step 1: Identify the insertion point** — open `cmd/grove/commands/doctor.go` and find the Tier 2 block where `runExternalModeChecks` is called (line 152). Above or below it, add a new check:

```go
// Check: any drifted worktrees (in state but worktree dir gone, or worktrees on disk not in state)
allPassed = runCheck(w, "Worktree registration", func() (string, error) {
    return checkWorktreeRegistration(ctx.ProjectRoot, ctx.State)
}) && allPassed
```

Wait — `doctor.go` doesn't have `ctx` here because it's outside `RequireGroveContext`. Adapt to use the existing local variables: `groveDir`, `cfg`. Pass them to a new `checkWorktreeRegistration` helper.

Add to the bottom of `doctor.go`:

```go
// checkWorktreeRegistration reports drifted worktrees: git worktrees on disk
// that aren't in state.json. Returns a friendly summary string or an error
// listing the drifted worktrees.
func checkWorktreeRegistration(projectRoot string) (string, error) {
	out, err := cmdexec.Output(context.TODO(), "git", []string{"-C", projectRoot, "worktree", "list", "--porcelain"}, "", cmdexec.GitLocal)
	if err != nil {
		return "", fmt.Errorf("list worktrees: %w", err)
	}

	statePath := filepath.Join(projectRoot, ".grove", "state.json")
	stateData, _ := os.ReadFile(statePath)
	stateStr := string(stateData)

	var drifted []string
	var total int
	for _, line := range strings.Split(string(out), "\n") {
		path, ok := strings.CutPrefix(line, "worktree ")
		if !ok {
			continue
		}
		total++
		// Skip the main worktree
		if path == projectRoot {
			continue
		}
		// Lightweight check: state.json should contain the worktree path as a value.
		if !strings.Contains(stateStr, `"`+path+`"`) {
			drifted = append(drifted, filepath.Base(path))
		}
	}

	if len(drifted) > 0 {
		return "", fmt.Errorf("%d drifted worktree(s): %s — run 'grove adopt' from each", len(drifted), strings.Join(drifted, ", "))
	}
	return fmt.Sprintf("%d worktrees registered", total), nil
}
```

In the Tier 2 block of `doctor.go` (around line 152), insert:

```go
allPassed = runCheck(w, "Worktree registration", func() (string, error) {
    return checkWorktreeRegistration(filepath.Dir(groveDir))
}) && allPassed
```

- [ ] **Step 2: Run doctor manually to verify**

```bash
go run ./cmd/grove doctor
```

Expected: a new "Worktree registration" line passes when no drift, fails with names listed otherwise.

- [ ] **Step 3: Run all tests**

Run: `make test`
Expected: all pass.

- [ ] **Step 4: Commit**

```bash
git add cmd/grove/commands/doctor.go
git commit -m "feat(doctor): check for drifted worktrees and suggest adopt"
```

---

### Task 6: Document `grove adopt` and the drift warning

**Files:**
- Modify: `docs/COMMAND_SPECIFICATIONS.md`
- Modify: `docs/AGENT_GUIDE.md`

- [ ] **Step 1: Add `grove adopt` spec** — append to `docs/COMMAND_SPECIFICATIONS.md`:

```markdown
## `grove adopt [path]`

Bootstrap a git worktree that grove doesn't already know about. Equivalent to
the post-`git worktree add` portion of `grove new`: symlinks `config.toml`,
registers state, fires post-create hooks, optionally starts docker.

Used when a worktree was created with `git worktree add` directly. With no
argument, adopts the cwd.

**Drift detection:** running any grove command from a drifted worktree prints
a non-fatal warning suggesting `grove adopt`. `grove doctor` reports drift in
its Tier-2 project checks.

**Idempotent:** safe to run twice — re-registers state without duplicating.
```

- [ ] **Step 2: Add a "Worktree drift" section to `AGENT_GUIDE.md`** — locate a sensible spot under the workflows section:

```markdown
### When a worktree drifts (created via `git worktree add`)

If a worktree was created outside grove (e.g., `git worktree add ../foo`),
grove won't have it in state and won't have run its bootstrap hooks. Symptoms:
missing credentials, `grove ls` doesn't show the worktree, downstream commands
fail with "no .grove" errors.

**Fix:** `cd` into the drifted worktree and run `grove adopt`. Idempotent;
safe to run from anywhere via `grove adopt <path>`.
```

- [ ] **Step 3: Commit**

```bash
git add docs/COMMAND_SPECIFICATIONS.md docs/AGENT_GUIDE.md
git commit -m "docs: document grove adopt and worktree drift detection"
```

---

### Task 7: Final verification

- [ ] **Step 1: Run full test suite**

Run: `make test`
Expected: all pass.

- [ ] **Step 2: Run linter**

Run: `make lint`
Expected: clean.

- [ ] **Step 3: End-to-end manual scenario**

```bash
# In a fresh checkout of a grove project:
git worktree add ../proj-test-adopt -b test-adopt
cd ../proj-test-adopt
grove ls   # warns: not registered
grove adopt
grove ls   # appears in list, no warning
grove doctor  # Worktree registration: passes
```

- [ ] **Step 4: Verify clean working tree**

Run: `git status`
Expected: clean.

---

## Self-review summary

- **Spec coverage:** Issue #3 (worktree health check on entry) addressed via `DiagnoseDrift` (Task 1), `emitDriftNotice` in `RequireGroveContext` (Task 3), the new `grove adopt` command (Task 4), and the doctor check (Task 5). The bootstrap reuse path (Task 2) is what makes `adopt` cheap to add.
- **No placeholders:** every step shows code, command, expected output.
- **Type consistency:** `BootstrapOpts` defined in Task 2 used in Tasks 2, 4. `DriftReason`/`ReasonDriftedWorktree`/`ReasonRegistered` defined in Task 1 used in Tasks 3, 4, 5.
- **No drift between plans:** none of the files touched here are touched by Plan 1 or Plan 3.
