package tui

import (
	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
)

// HelpOverlay renders context-sensitive help content using Glamour markdown
// and a viewport for scrolling.
type HelpOverlay struct {
	Active    bool
	ForView   ActiveView
	viewport  viewport.Model
	cache     map[ActiveView]string // rendered markdown per view
	lastWidth int                   // invalidate cache on resize
}

// NewHelpOverlay creates a HelpOverlay with an empty cache.
func NewHelpOverlay() *HelpOverlay {
	return &HelpOverlay{
		cache: make(map[ActiveView]string),
	}
}

// Open activates the overlay for the given view, rendering help content
// at the appropriate size.
func (h *HelpOverlay) Open(view ActiveView, width, height int) {
	h.Active = true
	h.ForView = view

	w, ht := calcHelpOverlaySize(width, height)

	// Content width accounts for border + padding (2 border + 4 padding = 6)
	contentWidth := w - 6
	if contentWidth < 20 {
		contentWidth = 20
	}

	// Invalidate cache on width change
	if width != h.lastWidth {
		h.cache = make(map[ActiveView]string)
		h.lastWidth = width
	}

	// Render or use cached content
	rendered, ok := h.cache[view]
	if !ok {
		raw := helpContentFor(view)
		rendered = renderMarkdown(raw, contentWidth)
		h.cache[view] = rendered
	}

	// Viewport height accounts for border + padding (2 border + 2 padding = 4)
	// plus footer line (1)
	vpHeight := ht - 5
	if vpHeight < 5 {
		vpHeight = 5
	}

	h.viewport = viewport.New(
		viewport.WithWidth(contentWidth),
		viewport.WithHeight(vpHeight),
	)
	h.viewport.SetContent(rendered)
}

// Close deactivates the overlay.
func (h *HelpOverlay) Close() {
	h.Active = false
}

// Update handles scroll keys within the help viewport.
func (h *HelpOverlay) Update(msg tea.KeyMsg) (*HelpOverlay, tea.Cmd) {
	var cmd tea.Cmd
	h.viewport, cmd = h.viewport.Update(msg)
	return h, cmd
}

// View renders the overlay panel with border, content, and footer hints.
func (h *HelpOverlay) View(width, height int) string {
	w, ht := calcHelpOverlaySize(width, height)

	// Content width for the border style
	contentWidth := w - 6
	if contentWidth < 20 {
		contentWidth = 20
	}

	vpContent := h.viewport.View()

	footer := Styles.TextMuted.Render("↑↓ scroll · esc close")

	content := vpContent + "\n" + footer

	return Styles.OverlayBorderInfo.
		Width(contentWidth).
		Height(ht - 4). // subtract border (2) + padding (2)
		Render(content)
}

// calcHelpOverlaySize computes overlay dimensions from terminal size.
func calcHelpOverlaySize(termW, termH int) (w, h int) {
	w = clamp(termW*70/100, 60, 90)
	h = clamp(termH*80/100, 15, 35)
	return
}

// clamp constrains val to [lo, hi].
func clamp(val, lo, hi int) int {
	if val < lo {
		return lo
	}
	if val > hi {
		return hi
	}
	return val
}

// helpContentFor returns raw markdown help content for the given view.
func helpContentFor(view ActiveView) string {
	switch view {
	case ViewDashboard, ViewHelp:
		return helpDashboard
	case ViewDelete:
		return helpDelete
	case ViewCreate:
		return helpCreate
	case ViewBulk:
		return helpBulk
	case ViewPRs:
		return helpPRs
	case ViewIssues:
		return helpIssues
	case ViewFork:
		return helpFork
	case ViewSync:
		return helpSync
	case ViewConfig:
		return helpConfig
	case ViewRename:
		return helpRename
	case ViewCheckout:
		return helpCheckout
	default:
		return helpDashboard
	}
}

//nolint:lll
const helpDashboard = `## Navigation

| Key | Action |
|-----|--------|
| ↑/k ↓/j | Move up / down |
| enter | Switch to selected worktree |
| U | Switch and start containers |
| 1-9 | Quick-switch by number |
| / | Filter worktrees |

## Worktree Actions

| Key | Action |
|-----|--------|
| n | Create new worktree |
| d | Delete worktree |
| R | Rename worktree |
| f | Fork (copy) worktree |
| a | Bulk delete stale worktrees |

## Workflow

| Key | Action |
|-----|--------|
| s | Sync environment worktree |
| b | Switch git branch in-place |
| p | Browse pull requests |
| i | Browse issues |

## Display

| Key | Action |
|-----|--------|
| o | Cycle sort (name → recent → dirty) |
| v | Toggle compact/detailed view |
| r | Refresh worktree list |
| c | Open configuration |

## CLI Companions

These commands offer more options than TUI shortcuts:

| Command | Purpose |
|---------|---------|
| ` + "`grove to <name> --peek`" + ` | Switch without hooks (read-only) |
| ` + "`grove fork <name> --move-wip`" + ` | Fork with uncommitted changes |
| ` + "`grove test <name>`" + ` | Run tests in another worktree |
| ` + "`grove diff <name>`" + ` | Diff against another worktree |
| ` + "`grove doctor`" + ` | Health check for grove setup |
`

const helpDelete = `## Delete Worktree

Remove a worktree and optionally its git branch.

| Key | Action |
|-----|--------|
| y | Confirm deletion |
| n | Cancel |
| space | Toggle branch deletion |
| esc | Close |
`

const helpCreate = `## Create Worktree

Create a new worktree with a new or existing branch.

| Key | Action |
|-----|--------|
| tab / shift+tab | Navigate fields |
| enter | Create worktree |
| esc | Cancel |
`

const helpBulk = `## Bulk Delete

Select multiple stale worktrees for deletion.

| Key | Action |
|-----|--------|
| ↑/k ↓/j | Navigate worktrees |
| space | Toggle selection |
| enter | Delete selected worktrees |
| a | Select all |
| esc | Cancel |
`

const helpPRs = `## Pull Requests

Browse and create worktrees from open pull requests.

| Key | Action |
|-----|--------|
| ↑/k ↓/j | Navigate PRs |
| enter | Create worktree from PR |
| o | Open PR in browser |
| tab | Preview PR details |
| / | Filter PRs |
| esc | Close |
`

const helpIssues = `## Issues

Browse open issues from GitHub.

| Key | Action |
|-----|--------|
| ↑/k ↓/j | Navigate issues |
| enter | Create worktree from issue |
| o | Open issue in browser |
| / | Filter issues |
| esc | Close |
`

const helpFork = `## Fork Worktree

Copy a worktree to a new branch, optionally moving uncommitted changes.

| Key | Action |
|-----|--------|
| tab / shift+tab | Navigate fields |
| enter | Create fork |
| esc | Cancel |
`

const helpSync = `## Sync Environment

Sync configuration and dependencies from the main worktree.

| Key | Action |
|-----|--------|
| ↑/k ↓/j | Navigate items |
| enter | Start sync |
| space | Toggle item |
| esc | Cancel |
`

const helpConfig = `## Configuration

Edit grove settings. Changes are saved to ` + "`.grove/config.toml`" + `.

| Key | Action |
|-----|--------|
| tab / shift+tab | Switch config tab |
| ↑/k ↓/j | Navigate fields |
| enter | Edit selected field |
| esc | Close configuration |
`

const helpRename = `## Rename Worktree

Change the short name of a worktree.

| Key | Action |
|-----|--------|
| enter | Confirm rename |
| esc | Cancel |
`

const helpCheckout = `## Switch Branch

Switch the git branch of the current worktree.

| Key | Action |
|-----|--------|
| ↑/k ↓/j | Navigate branches |
| / | Filter branches |
| enter | Switch to branch |
| esc | Cancel |
`
