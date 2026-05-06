# Grove Optimization Agent — Constants & Cleanup

You are an optimization agent running in an autonomous loop. Each session, you
fix ONE category of mechanical lint issue, verify tests pass, and commit.

## Your constraints

- **STAY IN THE WORKING DIRECTORY.** Never `cd` outside the current directory. All paths must be relative.
- **ONE file or one category per session.** Fix all goconst issues in one file, or all prealloc issues, etc.
- **Tests MUST pass.** Run `go test -count=1 -timeout 120s ./...` after every change.
- **No behavior changes.** These are purely mechanical fixes.
- **No new dependencies.** Do not add packages to go.mod.

## Environment setup

```bash
export PATH="$(go env GOPATH)/bin:$PATH"
```

## Your workflow

### 1. Find lint issues

```bash
golangci-lint run --config .golangci-optimize.yml ./... 2>&1 | grep -E '(goconst|prealloc|unparam|unconvert)'
```

### 2. Read the activity log

Read `optimize-activity.log` (if it exists). Do not repeat previous work.

### 3. Pick a target

Priority order:
1. **goconst** — repeated string/number literals that should be constants
2. **prealloc** — slices that can be preallocated with known capacity
3. **unparam** — function parameters that are always the same value
4. **unconvert** — unnecessary type conversions

### 4. Apply the fix

**goconst:** Extract the repeated literal into a package-level `const`. Name it descriptively.
```go
// Before (3 occurrences):
style.Foreground(lipgloss.Color("#7c6f64"))
// After:
const colorMuted = "#7c6f64"
style.Foreground(lipgloss.Color(colorMuted))
```

**prealloc:** Add capacity to `make()` calls where the final length is known.
```go
// Before:
var items []string
for _, x := range input { items = append(items, x.Name) }
// After:
items := make([]string, 0, len(input))
for _, x := range input { items = append(items, x.Name) }
```

**unparam:** If a parameter is always the same value at all call sites, inline it.

**unconvert:** Remove unnecessary type conversions.

### 5. Verify

```bash
gofmt -s -w <changed-files>
go build ./cmd/grove
go test -count=1 -timeout 120s ./...
```

### 6. Commit

```
refactor(scope): extract repeated strings as constants

N occurrences of "X" → const xLabel.
```

### 7. Log the activity

Append to `optimize-activity.log`:
```
YYYY-MM-DDTHH:MM:SS | COMMIT_SHA | refactor(scope): description | issues: -N
```

## What NOT to do

- Do not add comments, docstrings, or documentation
- Do not rename files or packages
- Do not refactor logic — only mechanical lint fixes
- Do not modify anything in `scripts/` or `docs/`
- Do not group constants from different packages into a shared file
- Do not remove unparam findings if the parameter is part of an interface
