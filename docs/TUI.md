# TUI Dashboard

Grove's TUI is a full-screen interactive dashboard for managing your worktrees. It launches when you run `grove` with no arguments.

## Launching

```bash
grove          # open the TUI dashboard
grove prs      # open directly in PR browser
grove issues   # open directly in issue browser
```

To disable the TUI and get plain command output, set `GROVE_TUI=0`:

```bash
GROVE_TUI=0 grove
```

## Dashboard Layout

The dashboard adjusts to your terminal width:

- **Wide (> 120 columns):** Side-by-side layout — worktree list on the left, detail panel on the right, separated by a vertical divider.
- **Narrow:** Stacked layout — list on top, a named separator rule, detail panel below.

### Header

The top bar shows:

- **Project name** and worktree count
- **Current branch** (the branch of your active worktree)
- **Current worktree indicator** (green dot + name, right-aligned)

### Worktree List

Each row in the list shows:

| Column | Description |
|--------|-------------|
| Number | Position in the list (1–N) for quick-switch |
| Indicator | Selection/status: `❯` selected, `●` current (green) or dirty (yellow), `✗` stale (red), `○` clean (muted) |
| Name | Short name (e.g., `testing`, not `project-testing`) |
| Branch | Git branch name |
| Age | Time since last commit |
| ± | Git status symbol: `✓` clean, `●` dirty, `✗` stale |
| ↕ | Sync status: `↑N` ahead, `↓N` behind |
| ⬡ | Tmux session: `⬢` attached, `⬡` detached |
| ◆ | Container status from plugins |

The selected row always shows `❯` regardless of worktree status — the indicator column prioritizes selection cursor over status symbols.

The main worktree always sorts to the top. The current worktree is highlighted.

### Detail Panel

Selecting a worktree updates the detail panel with:

**Git section**
- Branch name
- Commit hash + message + age

**Status section**
- Working tree status: clean / dirty (N files) / stale
- Sync status: synced / ahead N / behind N (only shown when a remote is tracked)
- Tmux: active session / detached session (only shown when a session exists)

**Changes section** (only when dirty)
- List of changed files with type indicators: `M` modified, `+` added, `-` deleted

### Footer

A compact hint bar at the bottom shows essential keybindings. Press `?` to open a scrollable, context-sensitive help overlay with the full reference for the current view.

### Toast Notifications

Short-lived notifications appear in the top-right corner of the header after operations complete. They auto-dismiss after 3 seconds and fade as they expire.

Toast levels:
- **Success** (green) — operation completed
- **Warning** (yellow) — completed with caveats
- **Error** (red) — operation failed

## Keybindings

### Dashboard (main view)

| Key | Action |
|-----|--------|
| `↑` / `k` | Move up |
| `↓` / `j` | Move down |
| `enter` | Switch to selected worktree |
| `1`–`9` | Quick-switch to nth worktree |
| `n` | Create new worktree |
| `d` | Delete selected worktree |
| `R` | Rename selected worktree |
| `f` | Fork selected worktree |
| `b` | Switch branch of selected worktree |
| `s` | Sync changes from another worktree |
| `c` | Open config editor |
| `p` | Browse GitHub PRs |
| `i` | Browse GitHub issues |
| `a` | Bulk delete |
| `o` | Cycle sort mode |
| `r` | Refresh worktree list |
| `/` | Filter list |
| `?` | Toggle expanded help |
| `q` / `esc` | Quit |

### Overlay / Wizard Keys

| Key | Action |
|-----|--------|
| `enter` | Confirm / advance step |
| `esc` | Cancel / close overlay |
| `backspace` | Go back one step |
| `tab` / `shift+tab` | Next / previous tab (config editor) |
| `↑` / `↓` | Navigate list |
| `space` | Toggle selection / checkbox |
| `y` | Confirm (delete overlay) |
| `n` | Cancel (delete overlay) |

## Overlays

Each overlay opens centered over the dimmed dashboard background. Press `esc` to close or cancel.

### New Worktree (`n`)

A multi-step wizard:

1. **Branch** — Choose "Select an existing branch" or "Create a new branch."
   - *Select existing:* A navigable branch list. Press `/` to filter by name (`j`/`k` navigate, all keys reach the filter input). Press `esc` to exit filter mode.
   - *Create new:* A focused text input for the new branch name (all keys including `j`/`k` are typed as text).
   - If you select an existing branch, Grove asks whether to use it as-is (split) or create a new branch from it (fork). You can save this preference to skip the prompt.
2. **Name** — Enter a short name (e.g., `my-feature`). Grove displays the full name preview (`project-my-feature`) as you type and warns about conflicts with existing worktrees.
3. **Confirm** — Review the name and branch, then press `enter` to create.

### Delete Worktree (`d`)

A confirmation dialog showing:
- Worktree name
- Warnings (uncommitted changes, environment worktree)
- Option to also delete the associated branch (`space` to toggle)

Press `y` to confirm, `n` or `esc` to cancel.

> The main worktree and protected worktrees cannot be deleted.

### Rename Worktree (`R`)

A text input overlay for renaming the selected worktree:
- Shows the current name
- Text input for the new name
- Validates that the new name is not empty, not the same, and not taken

Press `enter` to rename, `esc` to cancel.

Renaming moves the git worktree directory, updates the state entry, and renames the associated tmux session.

> The main worktree and protected worktrees cannot be renamed.

### Fork Worktree (`f`)

Fork creates a new worktree branched from the currently selected one — useful when you want to try a different approach without losing your current work.

Steps:
1. **Name** — Enter a name for the forked worktree. The new branch will be named `{source-branch}-{name}`.
2. **WIP** (skipped if source is clean) — Choose how to handle uncommitted changes:
   - **Move** — Transfer changes to the fork; source becomes clean.
   - **Copy** — Apply changes to both the fork and the source.
   - **Leave** — Fork starts from the commit HEAD; source keeps its changes.
3. **Confirm** — Review and press `enter` to create.

### Switch Branch (`b`)

Changes which branch a worktree has checked out — without needing to delete and recreate it.

Steps:
1. **Branch** — Select a target branch from a filterable list. Branches already used by other worktrees are excluded.
2. **WIP** (skipped if worktree is clean) — Choose how to handle uncommitted changes:
   - **Stash** — Stash changes before switching (`git stash`).
   - **Cancel** — Abort the branch switch.
3. **Confirm** — Review current and target branches, then press `enter` to switch.

> The main worktree and protected worktrees cannot have their branch changed.

### Sync Changes (`s`)

Copies uncommitted changes from one worktree into the current one, without switching directories. Useful for pulling in WIP work from a sister branch.

Steps:
1. **Source** — Select which worktree to pull changes from. Worktrees with uncommitted changes are highlighted.
2. **Preview** — See the list of files that will be applied.
3. **Confirm** — Press `enter` to apply.

> Source changes are preserved (sync copies, it does not move).

### Config Editor (`c`)

An in-TUI editor for your grove config. Changes take effect immediately after saving.

**Tabs** — Use `tab` / `shift+tab` to move between:

| Tab | Settings |
|-----|----------|
| General | `project_name`, `alias`, `projects_dir`, `default_branch` |
| Behavior | `dirty_handling`, `tmux_mode`, `naming_pattern`, `skip_branch_notice`, `default_branch_action` |
| Plugins | Docker plugin `enabled`, `auto_start`, `auto_stop` |
| Protection | `protected` list, `immutable` list |

**Editing a field:**
1. Navigate to the field with `↑` / `↓`.
2. Press `enter` to open the inline editor (text input or dropdown).
3. Confirm with `enter` or cancel with `esc`.

Changed fields are highlighted in yellow. When you press `esc` with unsaved changes, a save confirmation prompt appears — press `enter` to save, `esc` to discard.

### Bulk Delete (`a`)

Select multiple worktrees for deletion in one operation. The main worktree, protected worktrees, and the current worktree are excluded.

- `↑` / `↓` — Navigate
- `space` — Toggle selection
- `enter` — Delete all selected (no undo)
- `esc` — Cancel

### PR Browser (`p`)

Fetches open pull requests from GitHub (requires `gh` CLI installed and authenticated).

Each PR shows:
- PR number + title + branch name
- Author, commit count, diff stats (`+N -N`)
- Worktree badge if you already have a worktree checked out on that branch

Controls:
- `↑` / `↓` — Navigate
- `tab` — Toggle PR detail preview (rendered markdown body)
- `enter` — Create a new worktree from the PR branch
- Type to filter by title, author, number, or labels
- `esc` — Close

### Issue Browser (`i`)

Fetches open GitHub issues (requires `gh` CLI installed and authenticated).

Each issue shows:
- Issue number + title
- Author, age, labels

Controls:
- `↑` / `↓` — Navigate
- `tab` — Toggle issue detail preview (rendered markdown body)
- `enter` — Create a new worktree for the issue
- Type to filter by title, author, number, or labels
- `esc` — Close

## Sort Modes

Press `o` to cycle through sort modes:

| Mode | Description |
|------|-------------|
| `name` | Alphabetical (default) |
| `recent` | Most recently accessed first |
| `dirty` | Worktrees with uncommitted changes first |

The main worktree always stays at the top regardless of sort mode.

## Filtering

Press `/` to activate the list filter. Type to narrow by worktree name or branch. Results update as you type. Press `esc` to clear the filter and return to normal navigation.

When a filter is active, number quick-switch keys (`1`–`9`) are disabled to avoid switching to hidden items.

## Accessibility

Grove checks for high-contrast mode. If the `GROVE_HIGH_CONTRAST` environment variable is set or if the terminal reports a high-contrast preference, form elements switch to an accessible rendering mode.

## Agent Notes

Reference for AI agents working on TUI code. The TUI uses **Bubbletea v2** (Elm Architecture):
- `charm.land/bubbletea/v2` — framework (Model/Update/View)
- `charm.land/lipgloss/v2` — styling (ANSI-aware widths, colors, borders)
- `charm.land/bubbles/v2` — components (list, textinput, viewport)

### Key Patterns
- **ANSI-aware string measurement** — always use `lipgloss.Width(s)` not `len(s)` for visible width
- **Style.Render()** for applying styles, not `Style.SetString()` + `Style.String()`

### File Map
| File | Purpose |
|------|---------|
| `internal/theme/colors.go` | Centralized `ColorScheme` with semantic colors |
| `internal/tui/theme_v2.go` | `StyleSet` — lipgloss styles built from the color scheme |
| `internal/tui/list_v2.go` | **Default** two-line delegate with indicator/status columns (`NewWorktreeDelegateV2`) |
| `internal/tui/list.go` | Compact single-line V1 delegate with column headers (`NewWorktreeDelegate`) |

### List View Modes
The `v` key toggles between V2 (default two-line) and V1 (compact single-line) at runtime. Set permanently via `tui.compact_list = true` in config.
