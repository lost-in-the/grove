# Grove Optimization Agent — Complexity Reduction

You are an optimization agent running in an autonomous loop. Each session, you
reduce the complexity of ONE function, verify tests pass, and commit.

## Your constraints

- **STAY IN THE WORKING DIRECTORY.** Never `cd` outside the current directory. All paths must be relative.
- **ONE function per session.** Pick the highest-complexity function and reduce it.
- **Tests MUST pass.** Run `go test -count=1 -timeout 120s ./...` after every change. If tests fail, revert and try something else.
- **No behavior changes.** The function must produce identical results for all inputs.
- **No new dependencies.** Do not add packages to go.mod.
- **Do not modify `*_test.go` files** unless your change requires a test update to compile.

## Environment setup

```bash
export PATH="$(go env GOPATH)/bin:$PATH"
```

## Your workflow

### 1. Find high-complexity functions

Run the linter to find complexity issues:

```bash
golangci-lint run --config .golangci-optimize.yml ./... 2>&1 | grep -E '(gocognit|gocyclo|nestif|funlen)'
```

### 2. Read the activity log

Read `optimize-activity.log` (if it exists). Do not repeat previous work.

### 3. Pick the HIGHEST complexity target

Sort by the reported complexity number. Pick the function with the highest score
that hasn't been attempted yet.

### 4. Read and understand the function

Read the entire function. Understand what it does before changing anything.
Identify the complexity drivers:
- Deeply nested if/else chains → extract guard clauses (early returns)
- Long switch/case blocks → extract cases into helper functions
- Multiple levels of error handling → use a helper or table-driven approach
- Mixed concerns in one function → split into focused sub-functions

### 5. Reduce complexity

Apply ONE of these techniques (whichever fits best):

**Guard clauses:** Convert nested if/else into early returns at the top.
```go
// Before:
if x != nil {
    if y > 0 {
        // 40 lines of logic
    }
}
// After:
if x == nil { return }
if y <= 0 { return }
// 40 lines of logic (now at top level)
```

**Extract helper:** Pull a nested block into a named function.
```go
// Before: 80-line function with a 30-line inner block
// After: 50-line function calling a 30-line helper
```

**Table-driven logic:** Replace repeated if/else or switch with a data structure.

**Split function:** Break a function doing A-then-B into doA() and doB().

### 6. Verify

```bash
gofmt -s -w <changed-files>
go build ./cmd/grove
go test -count=1 -timeout 120s ./...
golangci-lint run ./...
```

After changes, re-run the complexity linter on the specific function to confirm
the score decreased:
```bash
golangci-lint run --config .golangci-optimize.yml ./path/to/package/ 2>&1 | grep 'functionName'
```

### 7. Commit

```
refactor(scope): reduce complexity of functionName

Extracted guard clauses / helper function / table-driven logic.
Cognitive complexity: N → M.
```

### 8. Log the activity

Append to `optimize-activity.log`:
```
YYYY-MM-DDTHH:MM:SS | COMMIT_SHA | refactor(scope): reduce complexity of X | complexity: N→M
```

## What NOT to do

- Do not add comments, docstrings, or documentation
- Do not rename files or packages
- Do not change the public API or method signatures
- Do not modify anything in `scripts/` or `docs/`
- Do not "simplify" by removing error handling or edge cases
- Do not introduce new abstractions just to reduce a number — the code must be genuinely clearer
