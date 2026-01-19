# State Management Package

This package provides state persistence for Grove, specifically managing frozen worktree states.

## Features

- **JSON-based persistence**: State is stored in `$HOME/.config/grove/state/frozen.json`
- **Thread-safe operations**: All operations are protected with mutex locks
- **Idempotent operations**: Safe to call freeze/resume multiple times
- **Atomic writes**: State files are written atomically using temp files and rename

## Usage

```go
import "github.com/LeahArmstrong/grove-cli/internal/state"

// Create a manager (empty string uses default location)
mgr, err := state.NewManager("")
if err != nil {
    log.Fatal(err)
}

// Freeze a worktree
if err := mgr.Freeze("feature-auth"); err != nil {
    log.Fatal(err)
}

// Check if frozen
frozen, err := mgr.IsFrozen("feature-auth")
if err != nil {
    log.Fatal(err)
}

// List all frozen worktrees
list, err := mgr.ListFrozen()
if err != nil {
    log.Fatal(err)
}

// Resume a worktree
if err := mgr.Resume("feature-auth"); err != nil {
    log.Fatal(err)
}
```

## State File Format

The state is stored as JSON:

```json
{
  "frozen": {
    "feature-auth": {
      "name": "feature-auth",
      "frozen_at": "2024-01-18T12:34:56Z"
    },
    "bugfix-123": {
      "name": "bugfix-123",
      "frozen_at": "2024-01-18T11:22:33Z"
    }
  }
}
```

## Design Decisions

1. **JSON over binary**: Human-readable and debuggable
2. **Map structure**: O(1) lookups for frozen checks
3. **Mutex protection**: Safe for concurrent use
4. **Sorted list output**: Consistent and predictable
5. **Atomic writes**: Prevents partial writes on crashes

## Testing

The package has comprehensive test coverage (85.7%) including:
- Basic operations (freeze, resume, check)
- Persistence across manager instances
- Concurrent operations
- Edge cases (empty names, idempotency)

Run tests:
```bash
go test ./internal/state/... -v
```

## Performance

All operations complete in <1ms under normal conditions:
- Freeze/Resume: Single file write
- IsFrozen: Map lookup (O(1))
- ListFrozen: Map iteration + sort
