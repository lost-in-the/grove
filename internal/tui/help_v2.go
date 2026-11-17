package tui

import (
	"strings"
	"time"

	"charm.land/lipgloss/v2"
)

// highlightDuration is how long a key hint stays highlighted after being pressed.
const highlightDuration = 500 * time.Millisecond

// Hint represents a single key-description pair for the help footer.
type Hint struct {
	Key         string
	Description string
}

// HelpFooter manages compact key hints in the footer bar.
type HelpFooter struct {
	// Highlight state: briefly flashes the key hint when pressed.
	highlightedKey string
	highlightedAt  time.Time

	// listToggle is the list mode v switches to, for the dashboard footer;
	// "" reads as "detailed". The model keeps it current.
	listToggle string
	// sidebarKey offers t in the dashboard footer: 110+ columns, where
	// there is a pane to show or hide, and only when it fits on one line.
	sidebarKey bool
}

// NewHelpFooter creates a HelpFooter.
func NewHelpFooter() *HelpFooter {
	return &HelpFooter{}
}

// SetHighlight marks a key as highlighted, recording the current time.
func (h *HelpFooter) SetHighlight(key string) {
	h.highlightedKey = key
	h.highlightedAt = time.Now()
}

// IsHighlighted returns true if the given key is currently highlighted and
// the highlight has not expired.
func (h *HelpFooter) IsHighlighted(key string) bool {
	if h.highlightedKey == "" || h.highlightedKey != key {
		return false
	}
	return time.Since(h.highlightedAt) < highlightDuration
}

// ClearExpiredHighlight clears the highlight if it has expired.
// Returns true if a highlight was cleared (caller should trigger a redraw).
func (h *HelpFooter) ClearExpiredHighlight() bool {
	if h.highlightedKey == "" {
		return false
	}
	if time.Since(h.highlightedAt) >= highlightDuration {
		h.highlightedKey = ""
		return true
	}
	return false
}

// HasHighlight returns true if any key is currently highlighted (not yet expired).
func (h *HelpFooter) HasHighlight() bool {
	return h.highlightedKey != "" && time.Since(h.highlightedAt) < highlightDuration
}

// inputMode is the footer's first word (docs/TUI.md, Input model): in
// move, letters are commands; in type, a text field has focus.
type inputMode string

const (
	modeMove inputMode = "move"
	modeType inputMode = "type"
	modeNone inputMode = "" // nothing takes keys, such as while creating
)

// Footer is one screen's key line: the mode word, the hints, and the key
// that leaves. When it does not fit, hints drop from the right; the mode
// word, the enter hint, and the leave key never drop.
type Footer struct {
	Mode  inputMode
	Hints []Hint
	Leave Hint // zero when nothing leaves
}

func moveFooter(leave Hint, hints ...Hint) Footer {
	return Footer{Mode: modeMove, Hints: hints, Leave: leave}
}

func typeFooter(leave Hint, hints ...Hint) Footer {
	return Footer{Mode: modeType, Hints: hints, Leave: leave}
}

// filterTypingFooter is every list filter's footer while you type.
func filterTypingFooter() Footer {
	return typeFooter(Hint{"esc", "clear"}, Hint{"enter/tab", "keep"})
}

// CompactFooter returns the footer for the given view in its usual state.
// Views whose footer depends on their state build it themselves.
func (h *HelpFooter) CompactFooter(view ActiveView) Footer {
	switch view {
	case ViewDetailSheet:
		return detailFooter(Hint{"esc/tab", "back"})
	case ViewDashboard:
		toggle := h.listToggle
		if toggle == "" {
			toggle = "detailed"
		}
		return moveFooter(Hint{"q", "quit"}, dashboardFooterHints(toggle, h.sidebarKey)...)
	case ViewIssues:
		return moveFooter(Hint{"esc", "close"},
			Hint{"↑↓/jk", "move"}, Hint{"enter", "new worktree"}, Hint{"B", "open in browser"},
			Hint{"tab", "detail"}, Hint{"/", "filter"})
	case ViewBranchForm:
		return typeFooter(Hint{"esc", "cancel"}, Hint{"↑↓/ctrl+jk", "pick"}, Hint{"tab", "next"}, Hint{"enter", "create"})
	case ViewDelete:
		return moveFooter(Hint{"esc/n", "cancel"}, Hint{"y", "confirm"}, Hint{"space", "toggle branch"})
	case ViewBulk:
		return moveFooter(Hint{"esc", "cancel"}, Hint{"↑↓/jk", "move"}, Hint{"space", "toggle"}, Hint{"enter", "delete selected"})
	case ViewPRs:
		return moveFooter(Hint{"esc", "close"},
			Hint{"↑↓/jk", "move"}, Hint{"enter", "review"}, Hint{"n", "new worktree"},
			Hint{"B", "open in browser"}, Hint{"tab", "detail"}, Hint{"/", "filter"})
	case ViewSync:
		return moveFooter(Hint{"esc", "cancel"}, Hint{"↑↓/jk", "move"}, Hint{"enter", "select"})
	case ViewPRReview:
		return moveFooter(Hint{"esc", "close"},
			Hint{"J/K", "file"}, Hint{"v", "viewed"}, Hint{"z", "fold"}, Hint{"t", "rail"}, Hint{"1-4", "tab"})
	case ViewConfig: // the panel builds its own line: configFooter
		return moveFooter(Hint{"esc", "close"}, Hint{"↑↓/jk", "move"})
	case ViewRename:
		return typeFooter(Hint{"esc", "cancel"}, Hint{"enter", "rename"})
	default:
		return moveFooter(Hint{"q", "quit"}, Hint{"?", "help"})
	}
}

// detailFooter is the footer of a focused detail pane or the detail sheet.
func detailFooter(leave Hint, extra ...Hint) Footer {
	hints := append([]Hint{{"↑↓/jk", "scroll"}, {"g/G", "top/bottom"}}, extra...)
	return moveFooter(leave, hints...)
}

// RenderCompact renders a one- or two-line footer for the view.
func (h *HelpFooter) RenderCompact(view ActiveView, width int) string {
	return h.Render(h.CompactFooter(view), width)
}

// CompactHeight returns the number of lines the compact footer will occupy.
func (h *HelpFooter) CompactHeight(view ActiveView, width int) int {
	rendered := h.RenderCompact(view, width)
	return strings.Count(rendered, "\n") + 1
}

// Render draws f on one line, or two when one is too narrow, indented two
// cells, with a just-pressed key lit.
func (h *HelpFooter) Render(f Footer, width int) string {
	lines := fitFooter(f, width, 2, h.IsHighlighted)
	return "  " + strings.Join(lines, "\n  ")
}

// renderKeyLine draws f in at most maxLines lines of width cells, for
// panels that carry their own key line (the form, find, prompts).
func renderKeyLine(f Footer, width, maxLines int) []string {
	return fitFooter(f, width, maxLines, func(string) bool { return false })
}

// fitsOneLine reports whether f fits one line of width cells with every
// hint kept.
func fitsOneLine(f Footer, width int) bool {
	return fitParts(f.parts(f.Hints, func(string) bool { return false }), width, 1) != nil
}

// keyLine is f on one line of width cells.
func keyLine(f Footer, width int) string {
	return renderKeyLine(f, width, 1)[0]
}

// promptKeys is a prompt's key line: up to two lines of width cells, each
// after indent.
func promptKeys(f Footer, width int, indent string) string {
	return indent + strings.Join(renderKeyLine(f, width, 2), "\n"+indent)
}

// fitFooter lays f out in at most maxLines lines of width cells, dropping
// hints from the right until it fits. With nothing left to drop it returns
// what remains on one line.
func fitFooter(f Footer, width, maxLines int, lit func(string) bool) []string {
	hints := f.Hints
	for {
		parts := f.parts(hints, lit)
		if lines := fitParts(parts, width, maxLines); lines != nil {
			return lines
		}
		drop := -1
		for i := len(hints) - 1; i >= 0; i-- {
			if !isEnterHint(hints[i]) {
				drop = i
				break
			}
		}
		if drop < 0 {
			return []string{strings.Join(parts, Styles.HelpSep.Render(" · "))}
		}
		hints = append(hints[:drop:drop], hints[drop+1:]...)
	}
}

func isEnterHint(h Hint) bool { return h.Key == "enter" || strings.HasPrefix(h.Key, "enter/") }

// parts renders the mode word, hints, and leave key.
func (f Footer) parts(hints []Hint, lit func(string) bool) []string {
	parts := make([]string, 0, len(hints)+2)
	if f.Mode != modeNone {
		parts = append(parts, Styles.HelpMode.Render(string(f.Mode)))
	}
	for _, h := range append(hints[:len(hints):len(hints)], f.Leave) {
		switch {
		case h.Key == "" && h.Description == "":
			continue
		case h.Key == "": // a note, such as "saves as you go"
			parts = append(parts, Styles.HelpDesc.Render(h.Description))
		default:
			keyStyle := Styles.HelpKey
			if lit(h.Key) {
				keyStyle = Styles.HelpKeyHighlight
			}
			parts = append(parts, keyStyle.Render(h.Key)+" "+Styles.HelpDesc.Render(h.Description))
		}
	}
	return parts
}

// fitParts joins parts into at most maxLines lines of width cells, or nil
// when they do not fit. Two lines split as evenly as the parts allow.
func fitParts(parts []string, width, maxLines int) []string {
	sep := Styles.HelpSep.Render(" · ")
	line := strings.Join(parts, sep)
	if lipgloss.Width(line) <= width || width <= 0 {
		return []string{line}
	}
	if maxLines < 2 {
		return nil
	}
	for split := len(parts) / 2; split >= 1; split-- {
		line1 := strings.Join(parts[:split], sep)
		line2 := strings.Join(parts[split:], sep)
		if lipgloss.Width(line1) <= width && lipgloss.Width(line2) <= width {
			return []string{line1, line2}
		}
	}
	for split := len(parts)/2 + 1; split < len(parts); split++ {
		line1 := strings.Join(parts[:split], sep)
		line2 := strings.Join(parts[split:], sep)
		if lipgloss.Width(line1) <= width && lipgloss.Width(line2) <= width {
			return []string{line1, line2}
		}
	}
	return nil
}
