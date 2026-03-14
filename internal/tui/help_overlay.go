package tui

import (
	"strings"

	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

// HelpOverlay renders context-sensitive help as a scrollable overlay.
type HelpOverlay struct {
	Active    bool
	ForView   ActiveView
	viewport  viewport.Model
	cache     map[ActiveView]string
	lastWidth int
}

// NewHelpOverlay creates a HelpOverlay with an empty cache.
func NewHelpOverlay() *HelpOverlay {
	return &HelpOverlay{
		cache: make(map[ActiveView]string),
	}
}

// Open activates the overlay for the given view.
func (h *HelpOverlay) Open(view ActiveView, width, height int) {
	h.Active = true
	h.ForView = view

	w, ht := calcHelpOverlaySize(width, height)

	// OverlayBorderInfo has Border (2) + Padding(1,2) (4 horizontal).
	// Width() in lipgloss includes border + padding + text, so
	// text area = Width - border(2) - padding(4) = Width - 6.
	textWidth := w - 6
	if textWidth < 20 {
		textWidth = 20
	}

	if width != h.lastWidth {
		h.cache = make(map[ActiveView]string)
		h.lastWidth = width
	}

	rendered, ok := h.cache[view]
	if !ok {
		rendered = renderHelpContent(view, textWidth)
		h.cache[view] = rendered
	}

	vpHeight := ht - 5
	if vpHeight < 5 {
		vpHeight = 5
	}

	h.viewport = viewport.New(
		viewport.WithWidth(textWidth),
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

	textWidth := w - 6
	if textWidth < 20 {
		textWidth = 20
	}

	vpContent := h.viewport.View()
	footerRule := lipgloss.NewStyle().Foreground(Colors.SurfaceBorder).
		Render(strings.Repeat("─", textWidth))
	footerKeys := "  " +
		Styles.HelpKey.Render("↑↓") + " " + Styles.HelpDesc.Render("scroll") +
		Styles.HelpSep.Render(" · ") +
		Styles.HelpKey.Render("esc") + " " + Styles.HelpDesc.Render("close")
	content := vpContent + "\n" + footerRule + "\n" + footerKeys

	return Styles.OverlayBorderInfo.
		Width(w).
		Height(ht).
		Render(content)
}

// calcHelpOverlaySize computes overlay dimensions from terminal size.
func calcHelpOverlaySize(termW, termH int) (w, h int) {
	w = clamp(termW*75/100, 60, 100)
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

// helpEntry is a key/action pair displayed in the help overlay.
type helpEntry struct {
	key  string
	desc string
}

// helpSection is a titled group of help entries.
type helpSection struct {
	title string
	note  string
	items []helpEntry
}

// renderHelpContent builds styled help text with manual two-column layout.
// Each section: full-width header, horizontal rule, then key-description rows
// with alternating backgrounds. Avoids lipgloss table border rendering issues.
func renderHelpContent(view ActiveView, width int) string {
	sections := helpSectionsFor(view)

	// Compute max key width across ALL sections for consistent columns
	maxKeyW := 0
	for _, sec := range sections {
		for _, item := range sec.items {
			if w := lipgloss.Width(item.key); w > maxKeyW {
				maxKeyW = w
			}
		}
	}
	// Below this width, two-column layout wraps awkwardly — use stacked.
	stacked := width < 60

	// Key column: 1 padding + key + 1 padding, capped at 40% of width
	// so CLI Companions' long keys don't squeeze the description column.
	keyColW := maxKeyW + 2
	maxKeyCol := width * 2 / 5
	if keyColW > maxKeyCol {
		keyColW = maxKeyCol
	}
	descColW := width - keyColW
	if descColW < 10 {
		descColW = 10
	}

	headerStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(Colors.Primary).
		Padding(0, 1)

	noteStyle := lipgloss.NewStyle().
		Foreground(Colors.TextMuted).
		Padding(0, 1)

	ruleStyle := lipgloss.NewStyle().Foreground(Colors.SurfaceBorder)
	rule := ruleStyle.Render(strings.Repeat("─", width))

	evenBg := Colors.SelectionBg

	var b strings.Builder
	rowIdx := 0

	for i, sec := range sections {
		if i > 0 {
			b.WriteString("\n")
		}

		b.WriteString(headerStyle.Render(sec.title))
		b.WriteString("\n")
		if sec.note != "" {
			b.WriteString(noteStyle.Render(sec.note))
			b.WriteString("\n")
		}
		b.WriteString(rule)
		b.WriteString("\n")

		for _, item := range sec.items {
			if stacked {
				// Stacked: key on its own line, description with └ connector below.
				// Connector and desc rendered as separate blocks with matching
				// backgrounds — avoids pre-rendered ANSI codes that prevent
				// background propagation when nested inside another Render().
				ks := lipgloss.NewStyle().Bold(true).Foreground(Colors.TextBright).Padding(0, 1).Width(width)
				cs := lipgloss.NewStyle().Foreground(Colors.TextMuted)
				ds := lipgloss.NewStyle().Foreground(Colors.TextNormal).Width(width - 3)
				if rowIdx%2 == 0 {
					ks = ks.Background(evenBg)
					cs = cs.Background(evenBg)
					ds = ds.Background(evenBg)
				}
				b.WriteString(ks.Render(item.key))
				b.WriteString("\n")
				b.WriteString(cs.Render(" └ ") + ds.Render(item.desc))
				b.WriteString("\n")
			} else {
				// Two-column: key and description side by side
				ks := lipgloss.NewStyle().Bold(true).Foreground(Colors.TextBright).Padding(0, 1).Width(keyColW)
				ds := lipgloss.NewStyle().Foreground(Colors.TextNormal).Padding(0, 1).Width(descColW)
				if rowIdx%2 == 0 {
					ks = ks.Background(evenBg)
					ds = ds.Background(evenBg)
				}
				b.WriteString(ks.Render(item.key) + ds.Render(item.desc))
				b.WriteString("\n")
			}
			rowIdx++
		}
	}

	return b.String()
}

// helpSectionsFor returns structured help data for the given view.
func helpSectionsFor(view ActiveView) []helpSection {
	switch view {
	case ViewDashboard, ViewHelp:
		return helpSectionsDashboard
	case ViewDelete:
		return helpSectionsDelete
	case ViewCreate:
		return helpSectionsCreate
	case ViewBulk:
		return helpSectionsBulk
	case ViewPRs:
		return helpSectionsPRs
	case ViewIssues:
		return helpSectionsIssues
	case ViewFork:
		return helpSectionsFork
	case ViewSync:
		return helpSectionsSync
	case ViewConfig:
		return helpSectionsConfig
	case ViewRename:
		return helpSectionsRename
	case ViewCheckout:
		return helpSectionsCheckout
	default:
		return helpSectionsDashboard
	}
}

var helpSectionsDashboard = []helpSection{
	{title: "Navigation", items: []helpEntry{
		{"↑/k ↓/j", "Move up / down"},
		{"enter", "Switch to selected worktree"},
		{"U", "Switch and start containers"},
		{"1-9", "Quick-switch by number"},
		{"/", "Filter worktrees"},
		{"tab", "Focus detail panel"},
	}},
	{title: "Detail Panel (when focused)", items: []helpEntry{
		{"↑/k ↓/j", "Scroll up / down"},
		{"g / G", "Jump to top / bottom"},
		{"ctrl+u / ctrl+d", "Half page up / down"},
		{"B", "Open associated PR in browser"},
		{"tab / esc", "Return to list"},
	}},
	{title: "Worktree Actions", items: []helpEntry{
		{"n", "Create new worktree"},
		{"d", "Delete worktree"},
		{"R", "Rename worktree"},
		{"f", "Fork (copy) worktree"},
		{"a", "Bulk delete stale worktrees"},
	}},
	{title: "Workflow", items: []helpEntry{
		{"s", "Sync environment worktree"},
		{"b", "Switch git branch in-place"},
		{"p", "Browse pull requests"},
		{"i", "Browse issues"},
	}},
	{title: "Display", items: []helpEntry{
		{"o", "Cycle sort (name → recent → dirty)"},
		{"v", "Toggle compact/detailed view"},
		{"r", "Refresh worktree list"},
		{"c", "Open configuration"},
	}},
	{title: "CLI Companions", note: "Commands with more options than TUI shortcuts:", items: []helpEntry{
		{"grove to <name> --peek", "Switch without hooks (read-only)"},
		{"grove fork <name> --move-wip", "Fork with uncommitted changes"},
		{"grove test <name>", "Run tests in another worktree"},
		{"grove diff <name>", "Diff against another worktree"},
		{"grove doctor", "Health check for grove setup"},
	}},
}

var helpSectionsDelete = []helpSection{
	{title: "Delete Worktree", note: "Remove a worktree and optionally its git branch.", items: []helpEntry{
		{"y", "Confirm deletion"},
		{"n", "Cancel"},
		{"space", "Toggle branch deletion"},
		{"esc", "Close"},
	}},
}

var helpSectionsCreate = []helpSection{
	{title: "Create Worktree", note: "Create a new worktree with a new or existing branch.", items: []helpEntry{
		{"tab / shift+tab", "Navigate fields"},
		{"enter", "Create worktree"},
		{"esc", "Cancel"},
	}},
}

var helpSectionsBulk = []helpSection{
	{title: "Bulk Delete", note: "Select multiple stale worktrees for deletion.", items: []helpEntry{
		{"↑/k ↓/j", "Navigate worktrees"},
		{"space", "Toggle selection"},
		{"enter", "Delete selected worktrees"},
		{"a", "Select all"},
		{"esc", "Cancel"},
	}},
}

var helpSectionsPRs = []helpSection{
	{title: "Pull Requests", note: "Browse and create worktrees from open pull requests.", items: []helpEntry{
		{"↑/k ↓/j", "Navigate PRs"},
		{"enter", "Create worktree from PR"},
		{"B", "Open PR in browser"},
		{"tab", "Focus detail panel"},
		{"/", "Filter PRs"},
		{"esc", "Close"},
	}},
	{title: "Detail Panel (when focused)", items: []helpEntry{
		{"↑/k ↓/j", "Scroll up / down"},
		{"g / G", "Jump to top / bottom"},
		{"ctrl+u / ctrl+d", "Half page up / down"},
		{"tab / esc", "Return to list"},
	}},
}

var helpSectionsIssues = []helpSection{
	{title: "Issues", note: "Browse open issues from GitHub.", items: []helpEntry{
		{"↑/k ↓/j", "Navigate issues"},
		{"enter", "Create worktree from issue"},
		{"tab", "Focus detail panel"},
		{"/", "Filter issues"},
		{"esc", "Close"},
	}},
	{title: "Detail Panel (when focused)", items: []helpEntry{
		{"↑/k ↓/j", "Scroll up / down"},
		{"g / G", "Jump to top / bottom"},
		{"ctrl+u / ctrl+d", "Half page up / down"},
		{"tab / esc", "Return to list"},
	}},
}

var helpSectionsFork = []helpSection{
	{title: "Fork Worktree", note: "Copy a worktree to a new branch, optionally moving uncommitted changes.", items: []helpEntry{
		{"tab / shift+tab", "Navigate fields"},
		{"enter", "Create fork"},
		{"esc", "Cancel"},
	}},
}

var helpSectionsSync = []helpSection{
	{title: "Sync Environment", note: "Sync configuration and dependencies from the main worktree.", items: []helpEntry{
		{"↑/k ↓/j", "Navigate items"},
		{"enter", "Start sync"},
		{"space", "Toggle item"},
		{"esc", "Cancel"},
	}},
}

var helpSectionsConfig = []helpSection{
	{title: "Configuration", note: "Edit grove settings. Changes saved to .grove/config.toml.", items: []helpEntry{
		{"tab / shift+tab", "Switch config tab"},
		{"↑/k ↓/j", "Navigate fields"},
		{"enter", "Edit selected field"},
		{"esc", "Close configuration"},
	}},
}

var helpSectionsRename = []helpSection{
	{title: "Rename Worktree", note: "Change the short name of a worktree.", items: []helpEntry{
		{"enter", "Confirm rename"},
		{"esc", "Cancel"},
	}},
}

var helpSectionsCheckout = []helpSection{
	{title: "Switch Branch", note: "Switch the git branch of the current worktree.", items: []helpEntry{
		{"↑/k ↓/j", "Navigate branches"},
		{"/", "Filter branches"},
		{"enter", "Switch to branch"},
		{"esc", "Cancel"},
	}},
}
