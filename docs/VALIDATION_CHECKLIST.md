# Grove Implementation Validation Checklist

Use this checklist to verify each command is implemented correctly.

---

## Pre-Flight Checks

Before testing any command, ensure:

- [ ] Shell integration is installed: `eval "$(grove init zsh)"`
- [ ] `GROVE_SHELL` environment variable is set to `1`
- [ ] You're in a git repository
- [ ] tmux is installed and accessible

---

## grove ls

### Requirements
- [ ] Shows all worktrees for current project only
- [ ] Current worktree marked with `•`
- [ ] Columns: NAME, BRANCH, STATUS, TMUX, PATH
- [ ] NAME shows short name (not full path)
- [ ] Main worktree displays as "main" not project name
- [ ] Paths include project prefix for non-main worktrees
- [ ] TMUX column shows: attached/detached/none/frozen

### Test Cases
```bash
# In project with worktrees
grove ls
# Expected:
# NAME            BRANCH          STATUS     TMUX        PATH
# • main          main            clean      attached    /path/to/project
#   testing       testing         clean      detached    /path/to/project-testing

# With --quiet flag
grove ls -q
# Expected:
# main
# testing

# With --json flag  
grove ls -j
# Expected: valid JSON with all fields

# With --paths flag
grove ls -p
# Expected: one path per line, no headers

# With no worktrees (fresh project)
grove ls
# Expected: helpful message suggesting `grove new`

# Outside git repo
cd /tmp && grove ls
# Expected: error about not being in git repo
```

---

## grove new

### Requirements
- [ ] Creates directory at `{parent}/{project}-{name}`
- [ ] Creates git worktree with `git worktree add`
- [ ] Creates branch if doesn't exist
- [ ] Creates tmux session named `{project}-{name}`
- [ ] Switches to new worktree (unless --no-switch)
- [ ] Shows clear success message with full path
- [ ] Shows clear error if worktree already exists
- [ ] Normalizes input name (lowercase, no spaces)

### Test Cases
```bash
# Basic creation
grove new testing
# Expected:
# ✓ Created worktree 'testing' at /path/to/project-testing
# ✓ Created branch 'testing' from 'main'
# ✓ Created tmux session 'project-testing'
# ✓ Switched to 'testing'

# Verify directory created
ls -la ../project-testing
# Expected: directory exists with git worktree

# Verify tmux session
tmux list-sessions | grep project-testing
# Expected: session exists

# Creating duplicate
grove new testing
# Expected:
# ✗ Worktree 'testing' already exists
# (with path, status, and suggestions)
# Exit code: 1

# With space in name
grove new "my feature"
# Expected: creates "project-my-feature"

# With uppercase
grove new TESTING
# Expected: creates "project-testing"

# With --no-switch
grove new feature --no-switch
# Expected: creates but stays in current worktree

# With --branch
grove new testing --branch existing-branch
# Expected: checks out existing branch

# With --from
grove new testing --from develop
# Expected: creates branch from develop, not main
```

---

## grove to

### Requirements
- [ ] Switches tmux session (if in tmux)
- [ ] Attaches to tmux session (if not in tmux)
- [ ] Changes shell directory (via cd: directive)
- [ ] Creates tmux session if doesn't exist
- [ ] Handles partial name matching
- [ ] Prompts on dirty worktree (configurable)
- [ ] Updates "last" tracking

### Test Cases
```bash
# Basic switch (in tmux)
grove to testing
# Expected: tmux session changes, directory changes

# After switch, verify
grove here
# Expected: shows "testing" as current worktree

# Switch back
grove to main
# Expected: back to main worktree

# Partial match (unambiguous)
grove to test
# Expected: switches to "testing" if only match

# Partial match (ambiguous)
grove to t
# Expected: error listing all matches

# Non-existent worktree
grove to nonexistent
# Expected:
# ✗ Worktree 'nonexistent' not found
# (with suggestions and list)

# Already in target
grove to main  # while in main
# Expected: "Already in 'main'" (exit 0)

# With dirty worktree (dirty_handling: prompt)
# Make changes first, then:
grove to testing
# Expected: prompt asking what to do

# Outside tmux, with shell integration
grove to testing
# Expected: attaches to tmux, cd: directive processed

# Outside tmux, without shell integration
/path/to/grove to testing
# Expected: instructions on how to attach
```

---

## grove rm

### Requirements
- [ ] Removes worktree directory
- [ ] Removes git worktree reference
- [ ] Kills tmux session
- [ ] Cannot remove current worktree
- [ ] Cannot remove main worktree
- [ ] Warns about uncommitted changes (unless --force)
- [ ] Optionally deletes branch

### Test Cases
```bash
# Basic removal
grove rm testing
# Expected:
# ✓ Killed tmux session 'project-testing'
# ✓ Removed worktree 'testing'
# ✓ Deleted branch 'testing'

# Verify removed
grove ls
# Expected: testing not in list

# Remove current worktree
grove to testing
grove rm testing
# Expected: ✗ Cannot remove current worktree

# Remove main worktree
grove rm main
# Expected: ✗ Cannot remove the main worktree

# Remove dirty worktree
# Make changes in worktree, then:
grove rm testing
# Expected: error about uncommitted changes

# Force remove dirty
grove rm testing --force
# Expected: removes anyway

# Remove non-existent
grove rm nonexistent
# Expected: ✗ Worktree 'nonexistent' not found

# Keep branch
grove rm testing --keep-branch
# Expected: worktree removed, branch preserved
```

---

## grove here

### Requirements
- [ ] Shows current worktree info
- [ ] Shows project name
- [ ] Shows short commit hash (7 chars)
- [ ] Shows commit message
- [ ] Shows relative time
- [ ] Shows tmux session status
- [ ] Shows git status (clean/dirty)

### Test Cases
```bash
# Basic info
grove here
# Expected format:
# Worktree: testing
# Project:  project-name
# Branch:   testing
# Path:     /path/to/project-testing
# Commit:   abc1234 (2 hours ago) Commit message
# Status:   clean
# Tmux:     attached (project-testing)

# In main worktree
cd /path/to/project
grove here
# Expected: Worktree: main

# With --quiet
grove here -q
# Expected: just "testing"

# With --json
grove here -j
# Expected: valid JSON with all fields

# In dirty worktree
# Make changes, then:
grove here
# Expected: Status: dirty (with file list)

# Not in worktree
cd /tmp
grove here
# Expected: error with helpful message
```

---

## grove last

### Requirements
- [ ] Switches to previous worktree
- [ ] Works like `cd -`
- [ ] Tracks last per-project
- [ ] Errors if no previous

### Test Cases
```bash
# Setup: switch between worktrees
grove to testing
grove to main

# Use last
grove last
# Expected: switches to testing

# Use last again
grove last
# Expected: switches back to main (toggles)

# First time (no history)
# Clear state first
grove last
# Expected: ✗ No previous worktree recorded

# After removed worktree
grove to testing
grove to main
grove rm testing
grove last
# Expected: error that testing no longer exists
```

---

## grove config

### Requirements
- [ ] Shows current configuration
- [ ] Shows source files
- [ ] No hardcoded "grove-" prefix

### Test Cases
```bash
# Basic config
grove config
# Expected: shows all settings with sources

# With --path
grove config --path
# Expected: just the config file path

# With --edit
grove config --edit
# Expected: opens config in $EDITOR

# With --json
grove config -j
# Expected: valid JSON
```

---

## grove init

### Requirements
- [ ] Outputs valid shell code
- [ ] Sets GROVE_SHELL=1
- [ ] Creates grove() wrapper function
- [ ] Creates alias (configurable)
- [ ] Handles cd: directive

### Test Cases
```bash
# ZSH output
grove init zsh
# Expected: valid zsh code that can be sourced

# Verify it works
eval "$(grove init zsh)"
echo $GROVE_SHELL
# Expected: 1

# Verify alias works
type w
# Expected: w is a function

# Bash output
grove init bash
# Expected: valid bash code

# Invalid shell
grove init fish
# Expected: error or implementation
```

---

## Shell Integration Validation

### cd: Directive Handling

```bash
# Test that cd: works through wrapper
eval "$(grove init zsh)"
grove to testing
pwd
# Expected: /path/to/project-testing

# Test that output is suppressed
grove to testing 2>&1 | grep "cd:"
# Expected: no output (cd: should be intercepted)

# Test mixed output (success message + cd:)
grove new another 2>&1
# Expected: success messages visible, no cd: line
pwd
# Expected: /path/to/project-another
```

---

## Error Message Validation

All error messages should:
- [ ] Start with ✗ (in TTY)
- [ ] Include actionable suggestions
- [ ] Exit with non-zero code
- [ ] Not include stack traces (unless --debug)

### Test Error Messages
```bash
# Not in git repo
cd /tmp && grove ls
# Expected: clear error about git, not panic

# Invalid command
grove invalid
# Expected: shows help

# Missing argument
grove new
# Expected: error with usage

# Permission denied
chmod 000 /some/path && grove new test --path /some/path
# Expected: clear error about permissions
```

---

## Performance Validation

All commands must complete in <500ms:

```bash
# Time each command
time grove ls
time grove here
time grove new perf-test
time grove to perf-test
time grove rm perf-test

# All should show real time < 0.5s
```

---

## Cross-Platform Notes

### macOS
- tmux available via Homebrew
- Notifications via osascript

### Linux  
- tmux available via package manager
- Notifications via notify-send

### WSL
- Special handling for paths
- tmux works normally

---

## Final Validation Script

```bash
#!/bin/bash
# grove-validation.sh

set -e

echo "=== Grove Validation Suite ==="

# Setup
PROJECT=$(mktemp -d)/test-project
mkdir -p "$PROJECT"
cd "$PROJECT"
git init
echo "test" > README.md
git add . && git commit -m "Initial commit"

# Install shell integration
eval "$(grove init zsh)"

echo "1. Testing grove ls (empty)..."
grove ls

echo "2. Testing grove new..."
grove new testing
[[ -d "../test-project-testing" ]] || { echo "FAIL: directory not created"; exit 1; }

echo "3. Testing grove here..."
grove here | grep -q "testing" || { echo "FAIL: not in testing"; exit 1; }

echo "4. Testing grove to..."
grove to main
grove here | grep -q "main" || { echo "FAIL: not in main"; exit 1; }

echo "5. Testing grove last..."
grove last
grove here | grep -q "testing" || { echo "FAIL: last didn't work"; exit 1; }

echo "6. Testing grove rm..."
grove to main
grove rm testing
grove ls | grep -q "testing" && { echo "FAIL: testing still exists"; exit 1; }

echo "7. Testing duplicate prevention..."
grove new duplicate
grove new duplicate 2>&1 | grep -q "already exists" || { echo "FAIL: no duplicate error"; exit 1; }

# Cleanup
cd /
rm -rf "$PROJECT" "${PROJECT}-"*

echo "=== All validations passed ==="
```
