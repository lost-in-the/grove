# Grove TUI Implementation Agent Prompt

You are implementing the Grove TUI redesign. Follow these instructions precisely.

## Required Reading (Do This First)

1. Read `docs/TUI_IMPLEMENTATION_SPEC.md` - This is your source of truth
2. Read `CLAUDE.md` - Project conventions and rules
3. Read `internal/tui/model.go` - Understand current architecture

## Your Mission

Implement ONE task from the TUI spec, following strict TDD.

## Task Selection

1. Find the JSON checklist in `docs/TUI_IMPLEMENTATION_SPEC.md` (section "Agent Checklist")
2. Identify the first task with `"status": "pending"` in a phase where all dependencies are complete
3. Announce which task you're working on: `Working on: TUI-X.Y - [Task Name]`

## Implementation Process (Strict Order)

### Step 1: Write Tests First

Create test file BEFORE implementation (e.g., `internal/tui/toast_test.go`).

Write table-driven tests covering:
- Happy path cases
- Edge cases
- Error conditions
- All acceptance criteria from the spec

Run tests - they MUST FAIL initially:
```bash
go test -v ./internal/tui/... -run TestYourFeature
```

### Step 2: Implement Minimum Code

Write the minimum code to make tests pass. Reference the code examples in the spec.

### Step 3: Verify

```bash
make test      # All tests pass
make lint      # No linting errors
make build     # Builds successfully
```

### Step 4: Refactor (if needed)

Improve code quality while keeping tests green.

### Step 5: Update Checklist

In `docs/TUI_IMPLEMENTATION_SPEC.md`, update the JSON checklist:
- Change task status from `"pending"` to `"complete"`
- If phase complete, update phase status

### Step 6: Commit

```bash
git add internal/tui/[files] docs/TUI_IMPLEMENTATION_SPEC.md
git commit -m "feat(tui): [description]

- [Detail 1]
- [Detail 2]

Task: TUI-X.Y

Co-Authored-By: Claude <noreply@anthropic.com>"
```

## Critical Rules

1. **ONE TASK PER SESSION** - Complete fully before moving on
2. **TESTS FIRST** - Never write implementation before tests
3. **FOLLOW THE SPEC** - Use exact field names, types, and patterns from spec
4. **CHECK ACCEPTANCE CRITERIA** - Every criterion must be verifiable
5. **UPDATE CHECKLIST** - Always update status after completing task

## Do NOT

- Skip writing tests
- Implement multiple tasks at once
- Deviate from the spec's design decisions
- Forget to update the JSON checklist
- Commit without running `make test lint`

## Validation Checklist (Before Commit)

- [ ] All new code has corresponding tests
- [ ] Tests were written BEFORE implementation
- [ ] `make test` passes
- [ ] `make lint` passes
- [ ] `make build` succeeds
- [ ] Acceptance criteria from spec are met
- [ ] JSON checklist in spec is updated
- [ ] Commit message follows format

## If Blocked

If you cannot complete a task:
1. Document the blocker in the spec under the task
2. Move to next available task
3. Note: `"status": "blocked"` with reason

## Completion Signals

When task is complete, output:
```
<task-complete>
TUI-X.Y - [Task Name]
Tests: [count] passing
Files: [list of files created/modified]
</task-complete>
```

When ALL tasks in ALL phases are complete, output:
```
<promise>TUI_REDESIGN_COMPLETE</promise>
```

When blocked on all available tasks:
```
<promise>BLOCKED</promise>
Reason: [explanation]
```

---

Start now. Read the spec, pick your task, write tests first.
