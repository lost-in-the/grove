# State Management Package

Per-project runtime state for Grove, persisted to `.grove/state.json` in the
**main** worktree. It tracks the project name, the set of worktrees grove knows
about (with their paths, branches, timestamps, and environment/mirror flags),
and the `last_worktree` used by `grove last`.

> Note: an earlier "frozen worktree" API (`Freeze`/`Resume`/`IsFrozen`) and a
> global `~/.config/grove/state/frozen.json` location no longer exist. State is
> per-project and lives in the project's `.grove/` directory.

## Features

- **JSON persistence** at `.grove/state.json` (see [DATA_FLOWS.md](../../docs/DATA_FLOWS.md)).
- **Cross-process safety**: `save()` takes an exclusive `flock` on
  `.grove/state.lock`, re-reads the on-disk state under the lock, re-applies
  schema migration to it (so a legacy file written by an older grove can't
  resurrect pre-migration keys through the merge), rebases this process's
  tracked mutations on top, then writes atomically (unique temp file + fsync +
  rename via `internal/fsutil.AtomicWriteFile`).
- **In-process safety**: an `sync.RWMutex` guards goroutine access.
- **Batched writes**: `Batch` coalesces several mutations into a single save.

## Usage

```go
import "github.com/lost-in-the/grove/internal/state"

// groveDir is the project's .grove directory (must be non-empty).
mgr, err := state.NewManager(groveDir)
if err != nil {
    log.Fatal(err)
}

// Register / update a worktree.
_ = mgr.AddWorktree("feature-auth", &state.WorktreeState{
    Path:   "/work/app-feature-auth",
    Branch: "feature-auth",
})

// Read it back.
ws, _ := mgr.GetWorktree("feature-auth")

// Update last_accessed_at, or the last-used worktree.
_ = mgr.TouchWorktree("feature-auth")
_ = mgr.SetLastWorktree("feature-auth")

// Coalesce several mutations into one save.
_ = mgr.Batch(func() error {
    _ = mgr.RenameWorktree("feature-auth", "auth")
    return mgr.TouchWorktree("auth")
})
```

## State File Format

```json
{
  "version": 1,
  "project": "app",
  "last_worktree": "feature-auth",
  "worktrees": {
    "root": {"path": "/work/app", "branch": "main", "root": true},
    "feature-auth": {"path": "/work/app-feature-auth", "branch": "feature-auth"}
  }
}
```

The main worktree is keyed `"root"` with `"root": true`.

## Testing

```bash
go test ./internal/state/... -v
```
