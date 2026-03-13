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

// CompactHints returns context-aware key hints for the given view.
func (h *HelpFooter) CompactHints(view ActiveView) []Hint {
	switch view {
	case ViewDashboard:
		return []Hint{
			{"↑↓", "navigate"},
			{"enter", "switch"},
			{"U", "up"},
			{"n", "new"},
			{"/", "filter"},
			{"o", "sort"},
			{"?", "help"},
			{"q", "quit"},
		}
	case ViewIssues:
		return []Hint{
			{"↑↓", "navigate"},
			{"enter", "create worktree"},
			{"/", "filter"},
			{"esc", "close"},
		}
	case ViewCreate:
		return []Hint{
			{"enter", "continue"},
			{"esc", "cancel"},
		}
	case ViewDelete:
		return []Hint{
			{"y", "confirm"},
			{"n", "cancel"},
			{"space", "toggle branch"},
		}
	case ViewBulk:
		return []Hint{
			{"↑↓", "navigate"},
			{"space", "toggle"},
			{"enter", "delete selected"},
			{"esc", "cancel"},
		}
	case ViewPRs:
		return []Hint{
			{"↑↓", "navigate"},
			{"enter", "create worktree"},
			{"esc", "close"},
		}
	case ViewFork:
		return []Hint{
			{"enter", "continue"},
			{"esc", "cancel"},
		}
	case ViewSync:
		return []Hint{
			{"↑↓", "navigate"},
			{"enter", "select"},
			{"esc", "cancel"},
		}
	case ViewConfig:
		return []Hint{
			{"tab", "next tab"},
			{"↑↓", "navigate"},
			{"enter", "edit"},
			{"esc", "close"},
		}
	case ViewRename:
		return []Hint{
			{"enter", "rename"},
			{"esc", "cancel"},
		}
	case ViewCheckout:
		return []Hint{
			{"enter", "continue"},
			{"esc", "cancel"},
		}
	default:
		return []Hint{
			{"?", "help"},
			{"q", "quit"},
		}
	}
}

// RenderCompact renders a one- or two-line footer with key hints.
// At narrow widths, hints wrap to a second line instead of being dropped.
func (h *HelpFooter) RenderCompact(view ActiveView, width int) string {
	hints := h.CompactHints(view)

	var parts []string
	for _, hint := range hints {
		keyStyle := Styles.HelpKey
		if h.IsHighlighted(hint.Key) {
			keyStyle = Styles.HelpKeyHighlight
		}
		part := keyStyle.Render(hint.Key) + " " + Styles.HelpDesc.Render(hint.Description)
		parts = append(parts, part)
	}

	sep := Styles.HelpSep.Render(" · ")
	line := strings.Join(parts, sep)

	// Fits on one line — done
	if lipgloss.Width(line) <= width || width <= 0 {
		return "  " + line
	}

	// Try two lines: split roughly in half
	for split := len(parts) / 2; split >= 1; split-- {
		line1 := strings.Join(parts[:split], sep)
		line2 := strings.Join(parts[split:], sep)
		if lipgloss.Width(line1) <= width && lipgloss.Width(line2) <= width {
			return "  " + line1 + "\n  " + line2
		}
	}

	// Even two lines overflow — fall back to dropping from right
	for i := len(parts) - 1; i >= 1; i-- {
		line = strings.Join(parts[:i], sep)
		if lipgloss.Width(line) <= width {
			break
		}
	}
	return "  " + line
}

// CompactHeight returns the number of lines the compact footer will occupy.
func (h *HelpFooter) CompactHeight(view ActiveView, width int) int {
	rendered := h.RenderCompact(view, width)
	return strings.Count(rendered, "\n") + 1
}

