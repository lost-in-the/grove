package tui

// Layout is the dashboard's width tier (spec 5.6).
type Layout int

const (
	// LayoutGuard is under 44 columns: one line asks for more room.
	LayoutGuard Layout = iota
	// LayoutList is 44 to 109 columns: the list and a selection line.
	LayoutList
	// LayoutSplit is 110 columns and up: the list and a docked detail pane.
	LayoutSplit
)

const (
	minWidth   = 44
	splitWidth = 110
)

// LayoutBreakpoint returns the tier for a terminal width.
func LayoutBreakpoint(width int) Layout {
	switch {
	case width >= splitWidth:
		return LayoutSplit
	case width >= minWidth:
		return LayoutList
	}
	return LayoutGuard
}

// isWideLayout reports whether the width docks the detail pane beside the
// list. Centralizes the breakpoint so panel sizing and rendering never
// disagree.
func isWideLayout(width int) bool {
	return LayoutBreakpoint(width) == LayoutSplit
}
