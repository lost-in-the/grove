# Grove Workflow Recipes

Read this file when you need step-by-step guidance for a specific task. SKILL.md and AGENTS.md cover the basics; this file provides more depth for common patterns.

---

## 1. Non-Destructive PR Review

**Goal:** Inspect a PR's code without disturbing your current working state or running any hooks.

Requires `gh` CLI installed and authenticated.

```bash
# Create a worktree from the PR (runs post_create hooks — audit first if unfamiliar repo)
grove fetch pr/42

# Switch to it in read-only mode — no pre/post_switch hooks fire, no tmux
grove to pr-42 --peek

# Review files, run grep, read tests
# ...

# Return to your original worktree
grove last

# Clean up the PR worktree when done
grove rm pr-42
```

`--peek` means no `pre_switch` or `post_switch` hooks fire during the switch. The worktree files are fully accessible; you just don't get hook-driven side effects (migrations, asset builds, etc.).

---

## 2. Feature Branch Workflow

**Goal:** Develop a new feature in an isolated worktree with its own Docker stack.

```bash
# Create worktree + branch + tmux session + Docker (if configured)
grove new my-feature

# Switch to it (or grove will prompt; GROVE_AGENT_MODE=1 skips tmux takeover)
grove to my-feature

# Work, commit as normal
git add . && git commit -m "feat: implement thing"

# When done, remove everything
grove rm my-feature
```

If the branch already exists (e.g., you're picking up someone else's work):

```bash
grove new my-feature --from-branch existing-branch-name
```

If you have uncommitted changes you want to carry over:

```bash
grove new my-feature --dirty
```

---

## 3. Running Tests in Another Worktree

**Goal:** Run another worktree's test suite without switching your current directory.

```bash
# Run all tests in the 'feature' worktree
grove test feature

# Pass arguments to the test runner
grove test feature -- -run TestUserCreation

# Run tests in the current worktree (no name needed)
grove test
```

`grove test` runs in the other worktree's directory and respects its config and environment. It does NOT run `pre_switch` or `post_switch` hooks — only the test command itself executes. Your current directory is unchanged.

---

## 4. Parallel Agent Setup

**Goal:** Run multiple agents simultaneously, each with their own Docker stack and unique ports.

```bash
export GROVE_AGENT_MODE=1
export GROVE_NONINTERACTIVE=1
export GROVE_TUI=0

# Check what slots are already in use
grove ps --json

# Find the next free slot without starting anything
SLOT=$(python skills/grove-worktree-management/scripts/allocate_slot.py)

# Start an isolated Docker stack on that slot
grove up --isolated --slot "$SLOT"

# Do your work...

# Clean up when done
grove down --slot "$SLOT"
```

Each slot gets a unique Compose project name (`{project}-{worktree}-slot-{N}`) and a unique port offset. See `references/isolated-slots.md` for the full schema and configuration options.

---

## 5. Syncing a Stale Worktree

**Goal:** Bring a worktree's branch up to date with upstream without switching to it.

```bash
# Sync a specific worktree
grove sync feature

# Sync the current worktree (omit the name)
grove sync
```

`grove sync` fetches from the remote and merges or rebases the tracked branch. If the worktree has uncommitted changes, grove will warn before proceeding.

---

## 6. Adopting an Orphan Worktree

**Goal:** Bring a worktree created manually via `git worktree add` under grove management.

```bash
# Check what git knows about
git worktree list

# Adopt the orphan — grove creates its metadata, registers tmux session name, etc.
grove adopt /path/to/existing-worktree
```

`grove adopt` is idempotent: running it on an already-managed worktree is safe and does nothing.

---

## 7. Applying WIP from One Worktree to Another

**Goal:** Take uncommitted changes from one worktree and apply them to another.

```bash
# Apply uncommitted changes from 'feature-a' into your current worktree
grove graft feature-a

# Fork the current worktree into a new one, then carry over any WIP
grove fork feature-b
grove graft feature-a   # now in feature-b, pulling changes from feature-a
```

`grove graft` (aliases: `apply`, `g`) copies the diff of uncommitted changes from the source worktree and applies it to the current one. The source worktree is not modified.

---

## 8. Batch Cleanup

**Goal:** Remove stale or merged worktrees without manually listing and deleting each one.

```bash
# Interactive: prompts before removing each stale worktree
grove trim

# Non-interactive: removes all stale worktrees without confirmation
grove trim --all
```

`grove trim` (aliases: `prune`, `clean`, `tm`) identifies worktrees whose branches have been merged or deleted from the remote and offers to remove them. Use `--all` carefully in automated scripts.

---

## 9. Probing Repository State Before Acting

**Goal:** Understand current state before making changes. Always do this before writing code or running mutating commands.

```bash
grove here --json       # current worktree: name, branch, SHA, changes, tmux, agent slot
grove ls --json         # all worktrees: name, branch, path, status, tmux, containers
grove context --json    # current worktree: ahead/behind, stash count, recent commits
grove ps --json         # active isolated Docker slots
```

Or use the helper that combines `here` and `ls` into one normalized output:

```bash
python skills/grove-worktree-management/scripts/probe_state.py
```

This is especially important before `grove rm`, `grove graft`, or `grove sync` — verify the target worktree exists and is in the state you expect.

---

## 10. Diagnosing a Broken Worktree

**Goal:** Find out why a worktree is unhealthy and what needs to be repaired.

```bash
# Check the current worktree
grove doctor

# Check a specific worktree by name
grove doctor feature

# Check a specific worktree by full name
grove doctor project-feature
```

`grove doctor` runs health checks: git worktree integrity, tmux session existence, Docker stack status, hook configuration validity, and copy_files/symlink targets. Output is color-coded: green = healthy, yellow = warnings, red = broken.

For hook-related warnings, read `.grove/hooks.toml` and `.grove/config.toml` directly — see `references/trust-model.md` for what to look for.
