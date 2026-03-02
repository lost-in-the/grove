package tui

// Layout describes the terminal width breakpoint.
type Layout int

const (
	// LayoutNarrow is for terminals under 80 chars — hide non-essential columns.
	LayoutNarrow Layout = iota
	// LayoutMedium is for 80–100 chars — stacked layout, all columns visible.
	LayoutMedium
	// LayoutWide is for >100 chars — side-by-side layout.
	LayoutWide
)

// LayoutBreakpoint returns the Layout tier for a given terminal width.
func LayoutBreakpoint(width int) Layout {
	switch {
	case width > 100:
		return LayoutWide
	case width >= 80:
		return LayoutMedium
	default:
		return LayoutNarrow
	}
}
