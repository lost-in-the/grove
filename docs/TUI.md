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

## Input model

One rule for every screen: **`j`/`k` move. While you type, hold `ctrl`.** The `ctrl` part is for `j`/`k` only; `ctrl+h` and `ctrl+l` are never bound. Future screens follow this section.

### Two modes

The footer's first word names the mode, so it is always on screen.

| Mode | When | Letters |
|---|---|---|
| `move` | No text field has focus | Commands. `j`/`k` and the arrows move. |
| `type` | A text field has focus (a text cursor shows) | Type, digits and `/` too. |

### Two kinds of text entry

| | Filter | Finder |
|---|---|---|
| Where | `/` on the dashboard list, the PR browser, and the issue browser | Go to (`space`, `?`), and the form's Branch and Base fields |
| Typing | Narrows the list in place | Lists results or suggestions under the field |
| Rows while typing | Do not move | `ctrl+j`/`ctrl+k`, `ctrl+n`/`ctrl+p`, and the arrows move them |
| `enter` | Keeps the filter, back to `move` on the list | Acts on the highlighted row |
| `tab` | Same as `enter` | Go to: next row (`shift+tab` previous). Form: next field |
| `esc` | Clears the text and leaves the filter | Closes go to, or cancels the form |

On the list with a filter applied, `esc` clears the filter and `/` opens it again with its text. The filter row stays visible while a filter is applied (see Filtering).

### Keys

| Key | Alternates | Does |
|---|---|---|
| `j` / `k` | `↓` / `↑` | Move through rows or fields, in `move` |
| `ctrl+j` / `ctrl+k` | `ctrl+n` / `ctrl+p`, `↓` / `↑` | Pick the next or previous row in a finder, while typing |
| `esc` | `ctrl+[` | Leave: `back`, `close`, `clear`, or `cancel` |
| `enter` | `ctrl+m` | The action the footer names |
| `tab` | `ctrl+i` | Next field, the detail pane, or keep a filter |
| `ctrl+u` | | While typing: delete before the cursor. `ctrl+k` never deletes. |
| `q` | `ctrl+c` | Quit, from the dashboard list. `esc` at the root never quits. |

Why three pick sets: each multiplexer and terminal takes some `ctrl` pairs for itself, and a different pair on each. Binding all three means one always arrives. `ctrl+[`, `ctrl+m`, and `ctrl+i` are `esc`, `enter`, and `tab` on most terminals; under the kitty keyboard protocol and tmux extended keys they arrive as keys of their own, so grove binds them too.

### Footer

Every screen's key line has one format: `<mode> · <keys> <verb> · ...`. A key pair is joined with a slash: `↑↓/jk move`, `↑↓/ctrl+jk pick`. The leave key comes last. Keys are spelled as words, never as a glyph: `space go to`, `space toggle`.

| Verb | Means |
|---|---|
| `move` | Move the cursor through rows or fields |
| `pick` | Move through results or suggestions in a finder |
| `scroll` | Scroll a pane that has no cursor |
| `change` | Change a value |
| `enter <verb>` | The action, named: `switch`, `create`, `review`, `save` |
| `esc back` | Return focus within a screen (the detail pane to the list) |
| `esc close` | Close a screen or overlay (go to, settings, a browser, help) |
| `esc clear` | Remove a filter |
| `esc cancel` | Drop what you started (the form, a prompt, a text edit) |
| `tab <verb>` | `next` field, `detail` pane, `keep` a filter |

When a footer does not fit, it takes a second line where the screen allows one; then hints drop from the right, one at a time. The mode word, the `enter` hint, and the leave key never drop. Panels with their own key line (the form, go to, prompts) take the same format. The form and go to reserve a second row only at widths where their longest line needs it, so the line never moves while you work.

### Focus

One `❯` is on screen, in the focused pane. When the New Worktree form has focus, the list beside or above it draws no `❯` and no selection bar; the selected row keeps its bold name, at full contrast. Inside the form only the focused field has `❯`; the highlighted suggestion has a lighter `›`. The focused pane's border or title rule is in the accent color with a bold title, as the focused detail pane's is. (Go to and the centered prompts draw over a dimmed screen; the dimmed list behind them keeps its row marked.)

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

A compact hint bar at the bottom shows essential keybindings, in the format the Input model section sets: `move · ↑↓/jk move · enter switch · space go to · tab detail · / filter · g more · ? help · q quit`. It takes two lines on a narrow terminal. The full reference, with every alternate key, is `keyboard shortcuts` in `?`.

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
| `g` | Open the more strip: `g` top of list, `p` pull requests, `i` issues, `c` config, `u` upgrade grove, `d` a local diff of the selected worktree (see PR Review), and under herdr `a` the next agent needing you. Any other key closes it. |
| `g` `g` / `G` | Top / bottom of the list |
| `ctrl+d` / `ctrl+u` | Half a page down / up |
| `pgdown` / `pgup`, `home` / `end` | Page down / up, first / last page |
| `enter` | Switch to selected worktree |
| `U` | Switch to selected worktree and bring the stack, exactly `grove to <name> --up`: the old stack goes down per `auto_stop`, and the stack comes up per `auto_start` before you move |
| `1`–`9` | Switch to the worktree pinned to that slot |
| `m` then `1`–`9` | Pin the selected worktree to a slot (`m 0` unpins) |
| `-` | Switch to the previous worktree (as `grove last`); its row shows `↩` |
| `tab` / `l` | Detail: focus the panel at 110+ columns, or open the full-screen sheet below that (`ctrl+i` works as `tab`) |
| `B` | Open the selected worktree's associated PR in the browser |
| `n` | Create new worktree |
| `d` | Delete selected worktree |
| `R` | Rename selected worktree |
| `f` | Copy selected worktree (new branch from its HEAD) |
| `b` | Switch the selected worktree to another local branch |
| `s` | Sync changes from another worktree |
| `c` | Open settings |
| `p` | Browse GitHub PRs |
| `i` | Browse GitHub issues |
| `a` | Bulk delete |
| `o` | Cycle sort mode: name, age, dirty, and attention under herdr |
| `O` | Reverse the sort |
| `r` | Refresh worktree list |
| `v` | Show or hide the commit message line |
| `/` | Filter list (see Filtering) |
| `space` | Go to: worktrees, loaded PRs and issues, and actions, as `grove to <name>` does for worktrees (see Go to below) |
| `?` | Go to, limited to actions: every dashboard action with its key. `keyboard shortcuts` there opens the full key reference. |
| `esc` | Clear an applied filter; otherwise nothing |
| `q` / `ctrl+c` | Quit, with no confirm |

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

The New Worktree form uses its own keys, listed in its section below. `esc`, `enter`, and `tab` also accept `ctrl+[`, `ctrl+m`, and `ctrl+i` (see Input model).

| Key | Action |
|-----|--------|
| `enter` | Confirm / advance step |
| `esc` | Cancel / close overlay |
| `backspace` | Go back one step |
| `tab` / `shift+tab` | Next / previous field (settings) |
| `↑` / `↓`, `k` / `j` | Move through the list |
| `space` | Toggle selection / checkbox |
| `y` | Confirm (delete overlay) |
| `n` | Cancel (delete overlay) |

## Overlays

Each overlay opens over the dimmed view beneath it, except the New Worktree form, which docks beside or under the list. Press `esc` to close or cancel.

`esc` goes back exactly one level. At the dashboard root it only clears an applied filter; it never quits (`q` and `ctrl+c` do). An overlay opened from the PR or issue browser returns there, and the dashboard cursor returns to the worktree it was on.

### New Worktree (`n`)

One form, three fields. It docks into the screen, so the list you opened it from stays in view, with its cursor row:

| Terminal | Where the form sits |
|---|---|
| 110+ columns | The right pane, in the detail panel's place. The list keeps its width. |
| Under 110 columns | The bottom of the screen, full width. The list is shorter and scrolls to keep the cursor row. |
| Under 110 columns and under 17 rows (too short to keep 3 list rows) | The whole screen. |

The form's key line replaces the footer. The form keeps one size for a given terminal size: typing, suggestions, and errors never change its height.

```
 ── New worktree ──────────────────────────────
 ❯ Branch  leaks
             ci/check-leaks            existing
           › + new "leaks"            from main

   Base    main
   Name    leaks
           folder myapp-leaks
           session myapp-leaks

 ✓ ready
 type · ↑↓/ctrl+jk pick · tab next · enter create · esc cancel
```

- **Branch:** type to search local and remote branches. Each row says whether the branch exists, is remote, or is already in use by a worktree. A name that matches nothing is created as a new branch, from the base the `+ new` row names.
- **Base:** shown only for a new branch, and skipped by `tab` otherwise. Defaults to the configured default branch (`default_base_branch`).
- **Name:** the worktree's short name (`grove new [name]`), derived from the branch until you edit it. The lines below show the folder and the session it makes.

Suggestions open under the field you are in: Branch or Base. The status line says what `enter` will do. If the branch is already in use, `enter` switches to that worktree instead of creating one. A name already used by another worktree is an error.

| Key | Action |
|---|---|
| `tab` / `shift+tab` | Next / previous field (`tab` on Branch accepts the highlighted row) |
| `ctrl+j` / `ctrl+k`, `ctrl+n` / `ctrl+p`, `↓` / `↑` | Move through the focused field's suggestions |
| `enter` | Create, or switch to an in-use branch's worktree |
| `esc` | Cancel and close (does nothing while creating) |

The key line takes two rows in the right pane, where one is too narrow for it; under the list it fits on one. While the form has focus the list keeps its rows but draws no `❯` (see Focus).

The PR browser opens this form (`n`) with the PR's branch filled in. The issue browser fills in the branch `issue-N`. From either browser the form docks beside or under that browser's list, the same way.

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

Renaming moves the git worktree directory, updates the state entry, and renames the worktree's session.

> The main worktree and protected worktrees cannot be renamed.

### Copy Worktree (`f`)

Opens the New Worktree form in Copy mode for the selected worktree. Use it to try a different approach without losing your current work.

- **Branch:** starts as `<branch>-copy`. It must be a new branch; it is cut from the selected worktree's HEAD.
- **Name:** follows the branch until you edit it.
- **Changes:** appears only when the worktree has uncommitted changes. Use `←` / `→`, `h` / `l`, or `space` on this row (it has no text cursor, so the key line says `move`):
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

### Settings (`c`)

A full-screen panel for the project's grove config. Every change is written to `.grove/config.toml` at once, so there is no save step and nothing unsaved.

```
 myapp ▸ settings                                              ✓ saved · u undo
  [1] general   [2] behavior   [3] plugins   [4] protection

  Moving between worktrees
 ❯ Uncommitted files      ignore
   Docker stack           off
   Where you land         session
   Sessions               auto

  Names
   Folder pattern         {project}-{name}
 ──────────────────────────────────────────────────────────────────────────────
  Uncommitted files  switch.dirty_handling
  What grove does when the worktree you leave has uncommitted files.

   ○ warn         Tell you, then move (default)
   ● ignore       Move, say nothing
   ○ prompt       Ask first
   ○ refuse       Do not move
   ○ auto-stash   Stash the files, then move

  move · ↑↓/jk move · ←→/hl change · saves as you go · u undo · esc close
```

- **The list** has one row per field: a plain label, then the value exactly as the file spells it. Behavior groups its fields under two headings.
- **The context panel** under the rule explains the focused field: its label with its key in the file right after it, what it does, and every choice with its meaning. The current choice has `●`, the default says `(default)`. A text or list field shows its value, its default, and what to type.
- **The header** says `✓ saved` for a few seconds after a write, and `✗ not saved` when a write fails. A failed write changes nothing on disk, and the panel shows the error under the field.

| Section | Fields (key) |
|-----|----------|
| `[1] general` | Project name (`project_name`), Projects folder (`projects_dir`), Default branch (`default_base_branch`) |
| `[2] behavior` | Uncommitted files (`switch.dirty_handling`), Docker stack (`switch.container_switch`), Where you land (`session.open_in`), Sessions (`tmux.mode`), Folder pattern (`naming.pattern`) |
| `[3] plugins` | Docker plugin (`plugins.docker.enabled`), Stack up on arrival (`plugins.docker.auto_start`), Stack down on leaving (`plugins.docker.auto_stop`) |
| `[4] protection` | Protected worktrees (`protection.protected`), No incoming changes (`protection.immutable`) |

| Key | Action |
|-----|--------|
| `↑` / `↓`, `k` / `j` | Previous / next field |
| `tab` / `shift+tab` | Next / previous field, wrapping |
| `←` / `→`, `h` / `l` | Change a choice or a true/false field, and write it |
| `space` | Switch a true/false field |
| `enter` | Edit a text or list field |
| `u` | Undo the last change: writes the value before it, and moves to that field |
| `1`-`4`, `H` / `L` | Go to a section, or the previous / next one; each section keeps its own cursor |
| `?` | Key reference |
| `esc` | Close |

**Editing a text or list field.** `enter` opens the value for typing in the context panel. Every key types, digits and letters too, so the tabs drop their numbers and the footer shows only `type · enter save · esc cancel`. `enter` checks the value with the same rules config loading uses (a folder pattern needs `{project}` and `{name}` once each, the default branch cannot be empty) and writes it only when it passes; otherwise the reason shows under the value. `esc` drops the edit. A list is typed as names separated by commas.

**Project name and Folder pattern.** Grove reads a worktree's short name from its folder name with these two values, and changing them does not rename folders. So when the dashboard lists worktrees besides root, the context panel warns under the description: `⚠ This project has 2 worktrees besides root. Changing this does not rename their folders. Grove will stop recognizing them by their short names.` The count comes from the dashboard's list. Committing a changed value then takes a second `enter`: the first one writes nothing, the status line reads `Press enter again to change. esc keeps "myapp".`, and the footer shows only `type · enter change · esc cancel`. The second `enter` writes it as any edit does. `esc` goes back to the edit with the typed text, and any other key goes back to the edit and types. A refused value never reaches this step, and an unchanged value, a project with root alone, every other field, and `u` write at once. When the context panel is short of rows, the help under the value gives way first, then the description; the warning and the status line stay whole.

The frame has one layout at a given size: every section and every field put the rule on the same row. Narrow terminals show only the active tab's name and wrap the context panel's text; a title and key too long for one line put the key on the next.

At 110 columns and up (the dashboard's split width, `splitWidth`) the list and the context panel sit side by side, as the dashboard's list and detail do. The list is a column of 44 to 56 cells, and a value too long for it ends in `…` (the context panel shows it whole). The context panel is to its right, past the dashboard's divider, and wraps at 76 columns however wide the terminal is. The header, tabs, and footer stay full width.

### Bulk Delete (`a`)

Select multiple worktrees for deletion in one operation. The main worktree, protected worktrees, and the current worktree are excluded.

- `↑` / `↓` — Navigate
- `space` — Toggle selection
- `enter` — Delete all selected (no undo)
- `esc` — Cancel

### PR Browser (`p`)

Fetches open pull requests from GitHub (requires `gh` CLI installed and authenticated).

The last list grove fetched is saved in the user cache directory (`~/.cache/grove/prs/` on Linux). `p` shows it at once with `cached · refreshing` above it, and replaces it when the new list arrives, keeping the selected PR. If the refresh fails, the saved list stays and the line reads `refresh failed:` with the reason. Go to's PR results also use the saved list.

Each PR shows:
- PR number + title + branch name
- Author, commit count, diff stats (`+N -N`)
- Worktree badge if you already have a worktree checked out on that branch

Controls:
- `↑` / `↓`, `k` / `j`: Move
- `tab`: Toggle PR detail preview (rendered markdown body)
- `enter`: Open the PR review (see below)
- `n`: Create a new worktree from the PR branch
- `B`: Open the selected PR in your default browser
- `/`: Filter by title, branch, author, or number (see Filtering)
- `esc`: Clear an applied filter, else close

### PR Review (`enter` on a PR)

A full-screen review of one pull request, opened from the PR browser or from a PR result in go to. The header names the PR; the tab row reads `[1] conversation  [2] commits  [3] checks  [4] files` with each tab's count once its data has loaded. `esc` goes back to where you opened it.

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
| `esc` | Close: back to where you opened it |

The editor's line flag follows its name: `+N file` for vi, Neovim, Helix, Nano, Emacs and the like; `-g file:N` for VS Code, Cursor and VSCodium; `file:N` for Sublime Text and Zed; `--line N file` for JetBrains IDEs. A PR whose branch has no worktree here says so; `n` in the PR browser creates one.

**Local diff (`g d`).** The same frame reviews a worktree's own changes, with no `gh`: `g d` on the dashboard, or `d` in the focused detail pane or the full-screen detail sheet. It diffs the worktree against its merge-base with the default branch, so commits on the branch and uncommitted edits to tracked files both show; a new file shows once it is added. Only the files tab is there; `enter` opens the editor in that worktree.

### Issue Browser (`i`)

Fetches open GitHub issues (requires `gh` CLI installed and authenticated).

Each issue shows:
- Issue number + title
- Author, age, labels

Controls:
- `↑` / `↓`, `k` / `j`: Move
- `tab`: Toggle issue detail preview (rendered markdown body)
- `enter`: Create a new worktree for the issue
- `B`: Open the selected issue in your default browser
- `/`: Filter by title, author, number, or labels (see Filtering)
- `esc`: Clear an applied filter, else close

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

## Go to

`space` opens a box titled `Go to` over the current screen, with the prompt `Go to: ` in the form of the filter row's `Filter: `. Before you type it shows agents that need you (under herdr), pinned worktrees, and recently used ones. As you type it lists, in order, with the first row highlighted:

| Group | Matched on | `enter` |
|---|---|---|
| worktrees | alias, branch | switches to it |
| pull requests, issues | number, title, branch | opens the browser with the cursor on it |
| actions | label, plain-language synonyms (`fork`, `checkout`, `trash`) | runs it on the worktree that was selected when you opened go to |

Places come before actions, so `space`, a few letters, `enter` goes to a worktree when one matches, as `grove to <name>` does.

Matching is fuzzy, matched letters are highlighted, and a synonym that matched is shown beside the action (`copy worktree · fork`). PRs and issues come only from data already loaded this session: go to never calls `gh`. While you type, `ctrl+j` / `ctrl+k`, `ctrl+n` / `ctrl+p`, the arrows, and `tab` / `shift+tab` move through the results; `enter` runs the highlighted one; `esc` closes. The box keeps its size and position while you type.

`?` opens the same box limited to actions, titled `Actions`. Searching `messages` (or `warnings`, `errors`, `log`) opens the session's warnings and errors, newest first: toasts disappear after a few seconds, these stay until you quit.

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

The dashboard list, the PR browser, and the issue browser filter the same way (see Input model, Filter).

- `/` opens the filter: the footer says `type`, and letters, digits, and `/` type. Results update as you type. No key moves the rows while you type.
- `enter` or `tab` keeps the filter and returns to the list in `move`.
- `esc` while typing clears the text and leaves the filter.
- On the list with a filter applied, `esc` clears it and `/` opens it again with its text.

Each list draws a filter row as its first row, under the header: `Filter: auth  4 of 23 · esc clear` while a filter is applied, the prompt and the text cursor while you type. With no filter, the dashboard keeps that row blank, and the PR and issue browsers show `N open` there, so the list does not move when a filter opens or closes.

A worktree created while a filter is applied is selected. If the filter hides it, the filter is cleared, as `g a` does.

Pin keys (`1`–`9`) work with a filter applied, since slots do not depend on row position. While you are typing in the filter they type.

## Accessibility

Every text color reads at 4.5:1 or better on the surface, the selected row, and the header bar, in the dark, light, and high-contrast themes (`internal/theme/palette.go`; checked by `TestPalette_TextContrast`). Status never relies on color alone: each state has its own glyph, and the detail panel spells it out.

Grove checks for high-contrast mode. If the `GROVE_HIGH_CONTRAST` environment variable is set or if the terminal reports a high-contrast preference, form elements switch to an accessible rendering mode.

Screen readers are untested, and the TUI is not built for them. It is a full-screen view that redraws in place, so a screen reader meets a changing grid of characters rather than announcements, and state glyphs (`●`, `✓`, `✗`, `⭡`) are read by their character names if at all. The CLI is the dependable path when output has to be read aloud: `grove ls`, `grove here`, `grove to`, `grove new`, and most other commands print plain lines, and take `--json` for tools.

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
| `internal/tui/omni.go` | Go to (code name omni): sources, fuzzy matching, result groups (`omniResults`) |
| `internal/tui/omni_view.go` | The go to overlay: state, keys, fixed frame |
| `internal/tui/messages_view.go` | Session message history overlay; long entries wrap under their text |
| `internal/tui/provision_report.go` | After create or copy: the warning toast and message history lines for files not copied, a failed registration, or a required hook |
| `internal/tui/actions.go` | The dashboard action table (keys, labels, synonyms, footer order) |
| `internal/tui/help_v2.go` | The footer: mode word, hints, leave key, and the drop order (`Footer`, `fitFooter`) |
| `internal/tui/keys.go` | The key map, with `ctrl+[`, `ctrl+m`, `ctrl+i`, and the finder pick keys (`finderDown`, `finderUp`) |
| `internal/tui/strip.go` | Docked strips (`g` more, `m` pin) drawn above the footer |
| `internal/tui/goto_agent.go` | `g a`: the next agent needing you |
| `internal/tui/poll.go` | The 3-second herdr agent poll: tick, generation, applying results |
| `internal/tui/pins.go` | Pin rules (`pinSet`): pin, unpin, move, rename, prune |
| `internal/tui/pins_view.go` | Pin and `-` keys, and the `m` strip |
| `internal/tui/age.go` | Commit age, rounded like git's relative dates (`shortAge`) |
| `internal/tui/view_stack.go` | View stack: frames, push and pop, cursor restore |
| `internal/tui/branch_form.go` | The New, Copy, and Switch form: fields, suggestions, validation, keys |
| `internal/tui/branch_form_view.go` | The form's docked layout (`branchFormLayout`), its rows, and docking it beside or under the list (`renderDockedForm`) |
| `internal/tui/compositor.go` | Draws an overlay over a dimmed background (`composeOverlay`) and toasts on top (`composeLayerAt`) |
| `internal/tui/worktree_list.go` | The dashboard list constructor (`newWorktreeList`) |
| `internal/tui/config_fields.go` | The settings fields: label, key, description, choices and their meanings, default, validation |
| `internal/tui/overlay_config.go` | The settings panel's state and keys: writes each change at once (`saveProjectConfig`), undo, text edits, the second `enter` on fields that name worktree folders (`renamesWorktrees`, `otherWorktreeCount`) |
| `internal/tui/config_view.go` | The settings frame: list, then the context panel under a rule or beside a divider, footer, one layout per size (`configLayout`) |

### List View Modes
The `v` key toggles the commit message line at runtime and saves the choice in `.grove/ui_prefs.json` (`compact_mode`). `tui.compact_list = false` in config starts with two lines when no choice is saved.
