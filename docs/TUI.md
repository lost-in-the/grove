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

| Width | Layout |
|---|---|
| 110+ columns | The list, with the detail panel docked on the right. `tab` or `l` focuses it; `esc`, `h`, or `tab` return. |
| 44 to 109 | The list, with a one-line summary of the selected worktree above the footer (`alias · ⎇ branch · commit subject`). `tab` or `l` opens the detail as a full-screen sheet; `esc`, `h`, or `tab` return. |
| under 44 | One line: `grove needs ≥44 columns`. Nothing is lost; widening restores the same screen. |

### Header

Left to right: project name, worktree count, agents that need you (under herdr: `! 2 blocked · ✓ 1 done`, hidden when both are zero), and sort (`⬍ name`, with `↑` when reversed). The cursor's position among the visible rows (`3/12`) sits on the right and tracks any filter; when a newer grove is cached, the upgrade badge (`⭡0.12.0`) sits just before it. On narrow terminals the count and the agent summary drop their words (`! 2 · ✓ 1`) and the project name truncates; the badge and the position stay.

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

### Upgrade badge

When grove's update-check cache (`~/.grove/update-check.json`) shows a newer
release, the header shows a badge before the cursor position:

    ⭡0.12.0  3/12

`g u` opens the upgrade overlay: the current and latest versions, every install
method (Brew, `go install`, binary download), and a changelog link. With nothing
newer cached it says `grove is up to date`. It never runs an upgrade. `esc`
closes it.

The opt-outs that silence other grove update notices
(`GROVE_NO_UPDATE_NOTIFIER=1` and friends) hide the badge. `g u` still opens
and reports the cached release.

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
| `g` | Open the goto strip: `g` top of list, `p` pull requests, `i` issues, `c` config, `u` upgrade grove, `d` a local diff of the selected worktree (see PR Review), and under herdr `a` the next agent needing you. Any other key closes it. |
| `g` `g` / `G` | Top / bottom of the list |
| `ctrl+d` / `ctrl+u` | Half a page down / up |
| `pgdown` / `pgup`, `home` / `end` | Page down / up, first / last page |
| `enter` | Switch to selected worktree |
| `U` | Switch to selected worktree and force-start its containers |
| `1`–`9` | Switch to the worktree pinned to that slot |
| `m` then `1`–`9` | Pin the selected worktree to a slot (`m 0` unpins) |
| `-` | Switch to the previous worktree (as `grove last`); its row shows `↩` |
| `tab` / `l` | Detail: focus the panel at 110+ columns, or open the full-screen sheet below that |
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
| `o` | Cycle sort mode: name, age, dirty, and attention under herdr |
| `O` | Reverse the sort |
| `r` | Refresh worktree list |
| `v` | Show or hide the commit message line |
| `/` | Filter list |
| `space` | Find: search worktrees, actions, and loaded PRs and issues (see Find below) |
| `?` | Find, limited to actions: every dashboard action with its key. `keyboard shortcuts` there opens the full key reference. |
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

The PR browser opens this form (`n`) with the PR's branch filled in. The issue browser fills in the branch `issue-N`.

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

A full-screen editor for the project's grove config. Changes take effect immediately after saving.

The header reads `<project> ▸ settings`, with `● 2 unsaved` on the right while anything differs from the file on disk. Below it are the section tabs, with the active one raised:

| Section | Settings |
|-----|----------|
| `[1] general` | `project_name`, `projects_dir`, `default_branch` |
| `[2] behavior` | `dirty_handling`, `tmux_mode`, `naming_pattern` |
| `[3] plugins` | Docker plugin `enabled`, `auto_start`, `auto_stop` |
| `[4] protection` | `protected` list, `immutable` list |

| Key | Action |
|-----|--------|
| `tab` / `shift+tab` | Next / previous field |
| `enter` | Next field; on a section's last field, the next section; on protection's last field, close (asking first when there are unsaved changes) |
| `ctrl+n` / `ctrl+p` | Next / previous section, from anywhere |
| `1`-`4` | Jump to a section, except while typing in a text field |
| `esc` | Close; with unsaved changes, asks first (`enter` save, `esc` discard) |

Every section has the same height, so the frame never jumps. Undoing a change back to the value on disk clears it from the unsaved count. Narrow terminals show only the active tab's name.

### Bulk Delete (`a`)

Select multiple worktrees for deletion in one operation. The main worktree, protected worktrees, and the current worktree are excluded.

- `↑` / `↓` — Navigate
- `space` — Toggle selection
- `enter` — Delete all selected (no undo)
- `esc` — Cancel

### PR Browser (`p`)

Fetches open pull requests from GitHub (requires `gh` CLI installed and authenticated).

The last list grove fetched is saved in the user cache directory (`~/.cache/grove/prs/` on Linux). `p` shows it at once with `cached · refreshing` above it, and replaces it when the new list arrives, keeping the selected PR. If the refresh fails, the saved list stays and the line reads `refresh failed:` with the reason. Find's PR results also use the saved list.

Each PR shows:
- PR number + title + branch name
- Author, commit count, diff stats (`+N -N`)
- Worktree badge if you already have a worktree checked out on that branch

Controls:
- `↑` / `↓` — Navigate
- `tab` — Toggle PR detail preview (rendered markdown body)
- `enter` — Open the PR review (see below)
- `n` — Create a new worktree from the PR branch
- `B` — Open the selected PR in your default browser
- Type to filter by title, branch, author, or number
- `esc` — Close

### PR Review (`enter` on a PR)

A full-screen review of one pull request, opened from the PR browser or from a PR result in Find. The header names the PR; the tab row reads `[1] conversation  [2] commits  [3] checks  [4] files` with each tab's count once its data has loaded. `esc` goes back to where you opened it.

Opening a review fetches the PR's detail, diff, and review comments in the background with `gh`. Each loads on its own, so a failed diff still leaves the other tabs. A PR that has not changed since you last opened it in this session opens at once. Without `gh`, the frame shows one `gh required` panel.

The review opens on the conversation (`1`): a summary of the checks, then the PR's comments, reviews, and review threads, oldest first, with replies under their thread. A thread on a line that has since changed reads `path (outdated, was line N)`. `j` / `k` move between items, and `enter` on a thread opens the files tab at its file with the line in view.

The commits tab (`2`) lists the PR's commits. `enter` on one shows only that commit's files in the files tab, whose tab then reads `files · <sha>`; `a` returns to the whole PR. Viewed marks belong to the whole PR, so `v` does nothing while one commit is shown.

The checks tab (`3`) lists every check with `✗` failed, `●` running, or `✓` passed, failing first. Two runs of one workflow (a push and a pull request run) both show.

The files tab (`4`) shows one file at a time: its path and counts, then each hunk with old and new line numbers. At 100 columns and wider a rail on the left lists the files as a folder tree with `+/-` counts; the current file is highlighted and viewed files carry `✓`. Control characters in the diff (an escape sequence in a test fixture, say) are drawn as symbols such as `␛`, never sent to the terminal.

| Key | Action |
|---|---|
| `1` - `4` | Conversation, commits, checks, files |
| `j` / `k` (tabs 1-3) | Move between items |
| `enter` | On a thread, open its line; on a commit, show its files |
| `a` | Show all commits' files again |
| `J` / `K`, `ctrl+n` / `ctrl+p` | Next / previous file, in the rail's order |
| `↑/k` `↓/j`, `ctrl+u` / `ctrl+d`, `pgup` / `pgdn` | Scroll the file |
| `g` / `G` | Top / bottom of the file |
| `v` | Mark the file viewed; the mark is saved in `.grove/ui_prefs.json` and dropped when the file's diff changes |
| `z` | Fold each run of 4 or more removed lines into one line |
| `t` | Show or hide the rail; the choice holds when the terminal is resized |
| `enter` | Open the file in `$VISUAL` or `$EDITOR` (else `vi`) at the first line in view, in the worktree checked out on the PR's branch; grove resumes when the editor exits |
| `B` | Open the PR in the browser |
| `esc` | Back |

The editor's line flag follows its name: `+N file` for vi, Neovim, Helix, Nano, Emacs and the like; `-g file:N` for VS Code, Cursor and VSCodium; `file:N` for Sublime Text and Zed; `--line N file` for JetBrains IDEs. A PR whose branch has no worktree here says so; `n` in the PR browser creates one.

**Local diff (`g d`).** The same frame reviews a worktree's own changes, with no `gh`: `g d` on the dashboard, or `d` in the focused detail pane or the full-screen detail sheet. It diffs the worktree against its merge-base with the default branch, so commits on the branch and uncommitted edits to tracked files both show; a new file shows once it is added. Only the files tab is there; `enter` opens the editor in that worktree.

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
| `attention` | Under herdr only: blocked agents, then done, then working, then the rest |

The main worktree always stays at the top regardless of sort mode or direction.

## Agent Triage

Under herdr the dashboard keeps track of which coding agents need you. Under tmux none of this appears.

- **Header:** counts blocked and done agents (`! 2 blocked · ✓ 1 done`).
- **Sort:** `o` reaches `attention`, which puts blocked agents first, then done, then working.
- **`g a`:** moves to the next blocked agent below the cursor, wrapping; with none blocked, to the next done one; with neither, it says `no agents need you`. If the filter hides the target, the filter is cleared.
- **Poll:** every 3 seconds the dashboard asks herdr for agent state (one `herdr workspace list`). Only the session and agent columns change. Under attention sort the rows re-sort and the cursor stays on the same worktree; while you are typing a filter they wait, and catch up on the next poll. A failed poll keeps the last known state and logs a warning.

## Find

`space` opens a search box over the current screen. Before you type it shows agents that need you (under herdr), pinned worktrees, and recently used ones. As you type it lists, in order:

| Group | Matched on | `enter` |
|---|---|---|
| actions | label, plain-language synonyms (`fork`, `checkout`, `trash`) | runs it on the worktree that was selected when you opened Find |
| worktrees | alias, branch | switches to it |
| pull requests, issues | number, title, branch | opens the browser with the cursor on it |

Matching is fuzzy, matched letters are highlighted, and a synonym that matched is shown beside the action (`copy worktree · fork`). PRs and issues come only from data already loaded this session: Find never calls `gh`. `ctrl+j` / `ctrl+k` or the arrows move; `esc` closes. The box keeps its size and position while you type.

`?` opens the same box limited to actions. Searching `messages` (or `warnings`, `errors`, `log`) opens the session's warnings and errors, newest first: toasts disappear after a few seconds, these stay until you quit.

## Pins

Pin up to nine worktrees to the digit keys. Press `m` to open a strip above the footer listing every slot and the worktree in it, then:

| Keys | Result |
|---|---|
| `m 3` | Pin the selected worktree to slot 3 |
| `m 3` again on the same worktree | Unpin it |
| `m 3` when slot 3 holds another worktree | Move the pin; the other worktree loses slot 3 |
| `m 0` | Unpin the selected worktree |
| `m esc`, or `m` and any other key | Close the strip |

A worktree holds one slot at most. A bare `3` switches to slot 3's worktree; an empty slot flashes `slot 3 is empty · m 3 to pin`. Pinned rows show their digit in the first column.

Pins are saved per project in `.grove/ui_prefs.json` (`pins`). Renaming or deleting a worktree in the TUI updates its pin; a pin to a worktree removed outside the TUI is dropped the next time the dashboard loads its list.

## Filtering

Press `/` to activate the list filter. Type to narrow by worktree name or branch. Results update as you type. Press `esc` to clear the filter and return to normal navigation.

Pin keys (`1`–`9`) work with a filter applied, since slots do not depend on row position. While you are typing in the filter they type.

## Accessibility

Every text color reads at 4.5:1 or better on the surface, the selected row, and the header bar, in the dark, light, and high-contrast themes (`internal/theme/palette.go`; checked by `TestPalette_TextContrast`). Status never relies on color alone: each state has its own glyph, and the detail panel spells it out.

Grove checks for high-contrast mode. If the `GROVE_HIGH_CONTRAST` environment variable is set or if the terminal reports a high-contrast preference, form elements switch to an accessible rendering mode.

## Agent Notes

Reference for AI agents working on TUI code. For design rationale behind the New Worktree wizard, see `docs/WIZARD_UX_RESEARCH.md`.

The TUI uses **Bubbletea v2** (Elm Architecture):
- `charm.land/bubbletea/v2` — framework (Model/Update/View)
- `charm.land/lipgloss/v2` — styling (ANSI-aware widths, colors, borders)
- `charm.land/bubbles/v2` — components (list, textinput, viewport)

### Action Registry
Every dashboard key is an `Action` in `internal/tui/actions.go`: its keys, label, plain-language synonyms, layer (`""` or `"g"`), footer position, an enabled check, and what it runs. Dispatch, the footer hints, and the strips read this table. `actions_test.go` checks it against `keys.go` both ways, so a new binding without an action (or an action without a binding) fails the build. Add dashboard keys there, not in `handleDashboardKey`.

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
| `internal/tui/responsive.go` | Width tiers: guard under 44, list with a selection line, split at 110 |
| `internal/tui/detail_sheet.go` | The full-screen detail sheet and the selection line |
| `internal/tui/omni.go` | Find: sources, fuzzy matching, result groups (`omniResults`) |
| `internal/tui/omni_view.go` | The Find overlay: state, keys, fixed frame |
| `internal/tui/messages_view.go` | Session message history overlay |
| `internal/tui/actions.go` | The dashboard action table (keys, labels, synonyms, footer order) |
| `internal/tui/strip.go` | Docked strips (`g` goto, `m` pin) drawn above the footer |
| `internal/tui/goto_agent.go` | `g a`: the next agent needing you |
| `internal/tui/poll.go` | The 3-second herdr agent poll: tick, generation, applying results |
| `internal/tui/pins.go` | Pin rules (`pinSet`): pin, unpin, move, rename, prune |
| `internal/tui/pins_view.go` | Pin and `-` keys, and the `m` strip |
| `internal/tui/age.go` | Commit age, rounded like git's relative dates (`shortAge`) |
| `internal/tui/view_stack.go` | View stack: frames, push and pop, cursor restore |
| `internal/tui/compositor.go` | Draws an overlay over a dimmed background (`composeOverlay`) and toasts on top (`composeLayerAt`) |
| `internal/tui/worktree_list.go` | The dashboard list constructor (`newWorktreeList`) |

### List View Modes
The `v` key toggles the commit message line at runtime and saves the choice in `.grove/ui_prefs.json` (`compact_mode`). `tui.compact_list = false` in config starts with two lines when no choice is saved.
