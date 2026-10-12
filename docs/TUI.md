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

- **Wide (> 100 columns):** Side-by-side layout — worktree list on the left, detail panel on the right, separated by a vertical divider.
- **Narrow:** Stacked layout — list on top, a named separator rule, detail panel below.

### Header

Left to right: project name, worktree count, and sort (`⬍ name`, with `↑` when reversed). The cursor's position among the visible rows (`3/12`) sits on the right and tracks any filter. On narrow terminals the count drops its word and the project name truncates.

### Worktree List

Each worktree is one row. Left to right:

| Column | Description |
|--------|-------------|
| Agent | Only under herdr: `!` blocked, `▸` working, `✓` done, blank otherwise |
| Cursor | `❯` on the selected row |
| Status | `●` current (green) or dirty (yellow), `✗` stale (red), `○` clean (muted) |
| Alias | The worktree's short name, truncated in the middle when space is short (`abc-1349-migrate-…-path`) |
| Branch chip | `⎇ branch`, only when the branch's last segment differs from the alias. `tui.list_secondary` picks `branch`, `dir`, or `off`. |
| Counts | `↑n` ahead, `↓n` behind, `~n` changed files |
| Age | Last commit, rounded the way git does: `45s`, `89m`, `35h`, `13d`, `10w`, `11mo`, `2y` |
| Tail | Session `⬢` active / `⬡` open, then container `◆` running / `◇` stopped |

The cursor and status are separate columns, so the selected row keeps its status. As the terminal narrows, the chip goes first, then the age, then the counts collapse to the strongest one (`↓`, then `~`, then `↑`), and only then does the alias shorten, down to 16 columns.

`v` adds a second line under each row with the commit message. One line is the default; the choice is saved per project.

The main worktree always sorts to the top.

### Detail Panel

Selecting a worktree updates the detail panel with:

**Git section**
- Branch name
- Commit hash + message + age

**Status section**
- Working tree status: clean / dirty (N files) / stale
- Sync status: synced / ahead N / behind N (only shown when a remote is tracked)
- Session: active session / detached session under tmux, active workspace / open workspace under herdr (only shown when a session exists)

**Changes section** (only when dirty)
- List of changed files with type indicators: `M` modified, `+` added, `-` deleted

### Footer

A compact hint bar at the bottom shows essential keybindings. Press `?` to open a scrollable, context-sensitive help overlay with the full reference for the current view.

### Update available indicator

When grove's update-check cache (`~/.grove/update-check.json`) shows a newer
release, the dashboard footer renders a passive badge:

    ↑ 0.7.0 → 0.8.0  press u

Pressing `u` opens an `UpdateOverlay` modal showing all install methods
(Brew, `go install`, binary download) and a changelog link. The modal closes
on `esc` or `u`.

The badge and modal are silenced by the same opt-outs as other grove update
surfaces — `GROVE_NO_UPDATE_NOTIFIER=1` and friends suppress all of them.

### Toast Notifications

Short-lived notifications appear in the top-right corner of the screen, above everything else, after operations complete. They auto-dismiss after 3 seconds and fade as they expire.

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
| `g` `g` / `G` | Top / bottom of the list |
| `ctrl+d` / `ctrl+u` | Half a page down / up |
| `pgdown` / `pgup`, `home` / `end` | Page down / up, first / last page |
| `enter` | Switch to selected worktree |
| `U` | Switch to selected worktree and force-start its containers |
| `1`–`9` | Quick-switch to nth worktree |
| `tab` / `l` | Focus the detail panel for scrolling (see below) |
| `B` | Open the selected worktree's associated PR in the browser |
| `n` | Create new worktree |
| `d` | Delete selected worktree |
| `R` | Rename selected worktree |
| `f` | Copy selected worktree (new branch from its HEAD) |
| `b` | Switch the selected worktree to another local branch |
| `s` | Sync changes from another worktree |
| `c` | Open config editor |
| `p` | Browse GitHub PRs |
| `i` | Browse GitHub issues |
| `a` | Bulk delete |
| `o` | Cycle sort mode: name, age, dirty |
| `O` | Reverse the sort |
| `r` | Refresh worktree list |
| `v` | Show or hide the commit message line |
| `u` | Open update modal (only when a newer release is cached — the footer badge indicates availability) |
| `/` | Filter list |
| `?` | Toggle expanded help |
| `q` / `esc` | Quit |

#### Detail Panel (when focused)

Press `tab` or `l` from the dashboard to move focus into the detail panel so you can scroll long content. Its title adds `· focused`. While focused:

| Key | Action |
|-----|--------|
| `↑` / `k`, `↓` / `j` | Scroll up / down |
| `g` / `G` | Jump to top / bottom |
| `ctrl+u` / `ctrl+d` | Half page up / down |
| `B` | Open associated PR in browser |
| `tab` / `esc` / `h` | Return focus to the list |

### Overlay / Wizard Keys

The New Worktree form uses its own keys, listed in its section below.

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

Each overlay opens over the dimmed view beneath it. Press `esc` to close or cancel.

`esc` goes back exactly one level. An overlay opened from the PR or issue browser returns there, and the dashboard cursor returns to the worktree it was on.

### New Worktree (`n`)

One form, three fields. It stays the same size and position while you use it.

- **Branch:** type to search local and remote branches. Each row says whether the branch exists, is remote, or is already in use by a worktree. A name that matches nothing is created as a new branch.
- **Base:** for a new branch only. Defaults to the configured default branch (`default_base_branch`).
- **Alias:** the worktree's short name, derived from the branch until you edit it. The line below shows the folder and session name.

The status line says what `enter` will do. If the branch is already in use, `enter` switches to that worktree instead of creating one. An alias already used by another worktree is an error.

| Key | Action |
|---|---|
| `tab` / `shift+tab` | Next / previous field (`tab` on Branch accepts the highlighted row) |
| `ctrl+j` / `ctrl+k`, `↓` / `↑` | Move through suggestions |
| `enter` | Create, or switch to an in-use branch's worktree |
| `esc` | Close (does nothing while creating) |

The PR browser opens this form with the PR's branch filled in. The issue browser fills in the branch `issue-N`.

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

### Copy Worktree (`f`)

Opens the New Worktree form in Copy mode for the selected worktree. Use it to try a different approach without losing your current work.

- **Branch:** starts as `<branch>-copy`. It must be a new branch; it is cut from the selected worktree's HEAD.
- **Alias:** follows the branch until you edit it.
- **Changes:** appears only when the worktree has uncommitted changes. Use `←` / `→` or `space` on this row:
  - **copy** (default): apply the changes to both worktrees.
  - **move:** apply them to the copy and clean the source.
  - **leave:** the copy starts clean; the source keeps its changes.

`enter` waits until the uncommitted-changes check finishes. The CLI equivalent is `grove fork`.

### Switch Branch (`b`)

Opens the form in Switch mode, to change which branch the selected worktree has checked out.

- **Branch:** suggestions list local branches that no worktree has checked out.
- **Changes:** if the worktree has uncommitted changes, they are stashed (`git stash`) before switching.

`enter` waits until the uncommitted-changes check finishes.

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
| Behavior | `dirty_handling`, `tmux_mode`, `naming_pattern` |
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
- `B` — Open the selected PR in your default browser
- Type to filter by title, branch, author, or number
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
- `B` — Open the selected issue in your default browser
- Type to filter by title, author, number, or labels
- `esc` — Close

## Sort Modes

Press `o` to cycle through sort modes, and `O` to reverse the current one:

| Mode | Description |
|------|-------------|
| `name` | Alphabetical (default) |
| `age` | Most recent commit first |
| `dirty` | Worktrees with uncommitted changes first |

The main worktree always stays at the top regardless of sort mode or direction.

## Filtering

Press `/` to activate the list filter. Type to narrow by worktree name or branch. Results update as you type. Press `esc` to clear the filter and return to normal navigation.

When a filter is active, number quick-switch keys (`1`–`9`) are disabled to avoid switching to hidden items.

## Accessibility

Every text color reads at 4.5:1 or better on the surface, the selected row, and the header bar, in the dark, light, and high-contrast themes (`internal/theme/palette.go`; checked by `TestPalette_TextContrast`). Status never relies on color alone: each state has its own glyph, and the detail panel spells it out.

Grove checks for high-contrast mode. If the `GROVE_HIGH_CONTRAST` environment variable is set or if the terminal reports a high-contrast preference, form elements switch to an accessible rendering mode.

## Agent Notes

Reference for AI agents working on TUI code. For design rationale behind the New Worktree wizard, see `docs/WIZARD_UX_RESEARCH.md`.

The TUI uses **Bubbletea v2** (Elm Architecture):
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
| `internal/theme/palette.go` | The palettes as hex pairs (`DefaultPalette`, `HighContrastPalette`) |
| `internal/tui/theme_v2.go` | `StyleSet` — lipgloss styles built from the color scheme |
| `internal/tui/row_delegate.go` | The dashboard row (`rowDelegate`); one or two lines |
| `internal/tui/row_layout.go` | Column widths and the shed order (`layoutColumns`, `middleTruncate`) |
| `internal/tui/age.go` | Commit age, rounded like git's relative dates (`shortAge`) |
| `internal/tui/view_stack.go` | View stack: frames, push and pop, cursor restore |
| `internal/tui/compositor.go` | Draws an overlay over a dimmed background (`composeOverlay`) and toasts on top (`composeLayerAt`) |
| `internal/tui/worktree_list.go` | The dashboard list constructor (`newWorktreeList`) |

### List View Modes
The `v` key toggles the commit message line at runtime and saves the choice in `.grove/ui_prefs.json` (`compact_mode`). `tui.compact_list = false` in config starts with two lines when no choice is saved.
