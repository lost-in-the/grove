@scripts/plan.md @scripts/activity.md @CLAUDE.md @docs/IMPLEMENTATION_PLAN.md @docs/COMMAND_SPECIFICATIONS.md

You are validating the grove-cli project. Your job is to verify that the implementation matches the plan and all quality gates pass.

## Instructions

1. Read scripts/plan.md to see validation tasks (JSON array)
2. Read scripts/activity.md to see what was done in previous iterations
3. Pick ONE task where `"passes": false`
4. Execute that single validation task thoroughly
5. Update scripts/plan.md: change `"passes"` to `true` if validation passes, or leave `false` if issues found
6. Log results to scripts/activity.md with timestamp
7. If ALL tasks have `"passes": true`, output: `<promise>COMPLETE</promise>`
8. If issues require user action to fix, output: `<promise>USER_ACTION_REQUIRED</promise>` and list what needs fixing

## Validation Approach Per Category

### phase0 - Foundation Commands
- Build the binary: `make build`
- Check each command exists and shows help:
  - `./bin/grove ls --help`
  - `./bin/grove new --help`
  - `./bin/grove to --help`
  - `./bin/grove rm --help`
  - `./bin/grove here --help`
  - `./bin/grove last --help`
- Check shell integration: `./bin/grove init zsh` and `./bin/grove init bash` produce valid shell code
- Check hook system: verify internal/hooks package exists and has tests
- Passes if: All 6 core commands exist with help text, shell init works

### phase1 - Docker Plugin
- Check Docker plugin commands:
  - `./bin/grove up --help`
  - `./bin/grove down --help`
  - `./bin/grove logs --help`
  - `./bin/grove restart --help`
- Verify plugins/docker/ exists with plugin.go
- Passes if: All 4 Docker commands exist

### phase2 - State Management
- Check state commands:
  - `./bin/grove freeze --help`
  - `./bin/grove resume --help`
- Verify internal/state/ package exists
- Passes if: freeze/resume commands exist, state package exists

### phase3 - Time Tracking
- Check time commands:
  - `./bin/grove time --help`
  - Check for --all, --json flags in help
- Verify plugins/time/ exists
- Passes if: time command exists with expected flags

### phase4 - Issue Integration
- Check tracker commands:
  - `./bin/grove fetch --help`
  - `./bin/grove issues --help`
  - `./bin/grove prs --help`
  - `./bin/grove browse --help`
- Verify plugins/tracker/ exists
- Passes if: All tracker commands exist

### phase5 - Polish
- Check documentation exists:
  - README.md (with installation instructions)
  - CONTRIBUTING.md
  - CHANGELOG.md
- Check GoReleaser config: .goreleaser.yaml or .goreleaser.yml
- Check shell completions: shell/completions/ directory
- Passes if: All docs exist, GoReleaser configured

### coverage - Test Coverage
- Run: `go test -coverprofile=coverage.out ./...`
- Parse output for per-package coverage
- Target: 80% for internal/ packages per IMPLEMENTATION_PLAN.md
- List packages below threshold
- Passes if: All internal/ packages >= 80%
- If packages below threshold: Mark as `USER_ACTION_REQUIRED` with list of packages needing improvement

### tests - All Tests Pass
- Run: `make test` or `go test -race ./...`
- All tests must pass
- Race detector must pass
- Passes if: Zero test failures
- If tests fail: Mark as `USER_ACTION_REQUIRED` with failing test names

### lint - Linting
- Run: `go vet ./...`
- Run: `golangci-lint run` (if available, otherwise note it's not installed)
- Check gofmt: `gofmt -l .` should produce no output for .go files
- Passes if: go vet passes, gofmt compliant
- If lint errors: Mark as `USER_ACTION_REQUIRED` with issues

### ci - CI Configuration
- Check .github/workflows/ci.yml exists with:
  - Test job (runs go test)
  - Lint job (runs golangci-lint or go vet)
  - Build job (runs make build)
- Check .github/workflows/release.yml exists
- Note any gaps vs IMPLEMENTATION_PLAN.md (coverage threshold, integration tests)
- Passes if: Both workflow files exist with required jobs

### cleanup - Unnecessary Files
- Look for files that shouldn't be committed:
  - coverage.out, coverage.html
  - *.tmp, *.bak
  - .DS_Store
  - bin/ directory (should be gitignored)
- Check .gitignore covers: bin/, coverage.*, *.out, .DS_Store
- Passes if: No unnecessary files found, .gitignore covers artifacts
- If cleanup needed: Mark as `USER_ACTION_REQUIRED` with file list

### practices - Best Practices
- Check for panic() in non-test files: `grep -r "panic(" --include="*.go" . | grep -v "_test.go" | grep -v "vendor/"`
  - Should be rare/justified (only in truly unrecoverable situations)
- Check exported functions have doc comments (sample a few key files)
- Check error wrapping pattern: errors should use `fmt.Errorf("context: %w", err)`
- Passes if: No unjustified panics, good documentation patterns
- If issues: Note them but may still pass if minor

## Critical Rules

- **ONLY WORK ON A SINGLE TASK** per iteration
- Always update plan.md when task completes (or when issues prevent completion)
- Always log to activity.md what you did with timestamp
- Be specific about what passed and what failed
- If a check fails, describe exactly what needs fixing
- When outputting USER_ACTION_REQUIRED, include a clear list of what the user must fix

## Activity Log Format

When logging to activity.md, use this format:

```markdown
### [TIMESTAMP] - [category]
**Status:** PASS | FAIL | USER_ACTION_REQUIRED
**Details:**
- What was checked
- What passed
- What failed (if any)
**Issues requiring user action:** (if any)
- Issue 1
- Issue 2
```

## Example Output

If all tasks pass:
```
All 12 validation tasks have passed.
<promise>COMPLETE</promise>
```

If user action required:
```
Validation found issues requiring user action:

**Coverage below 80%:**
- internal/config: 28.6% (need +51.4%)
- internal/hooks: 52.2% (need +27.8%)

Please increase test coverage and re-run validation.
<promise>USER_ACTION_REQUIRED</promise>
```
