package cli

import (
	"fmt"
	"strings"

	lipgloss "charm.land/lipgloss/v2"

	"github.com/LeahArmstrong/grove-cli/internal/theme"
)

// Column defines a table column with optional styling.
type Column struct {
	Title    string
	MinWidth int
	MaxWidth int
	// ColorFn optionally colors cell values. Receives the raw cell value,
	// returns the styled string. Only called when color is enabled.
	ColorFn func(value string) string
}

// Table renders a styled columnar table.
type Table struct {
	columns []Column
	rows    [][]string
	w       *Writer
}

// NewTable creates a new table for the given writer and column definitions.
func NewTable(w *Writer, columns ...Column) *Table {
	return &Table{
		columns: columns,
		w:       w,
	}
}

// AddRow appends a data row. Values are positional by column.
func (t *Table) AddRow(values ...string) {
	// Pad to column count
	row := make([]string, len(t.columns))
	for i := range row {
		if i < len(values) {
			row[i] = values[i]
		}
	}
	t.rows = append(t.rows, row)
}

// Render prints the table to the writer.
func (t *Table) Render() {
	if len(t.rows) == 0 {
		return
	}

	cs := theme.Colors
	useColor := t.w.UseColor()

	// Compute column widths from headers and data using visual width
	widths := make([]int, len(t.columns))
	for i, col := range t.columns {
		widths[i] = lipgloss.Width(col.Title)
		if col.MinWidth > widths[i] {
			widths[i] = col.MinWidth
		}
	}
	for _, row := range t.rows {
		for i, val := range row {
			if vw := lipgloss.Width(val); i < len(widths) && vw > widths[i] {
				widths[i] = vw
			}
		}
	}
	// Apply max width constraints
	for i, col := range t.columns {
		if col.MaxWidth > 0 && widths[i] > col.MaxWidth {
			widths[i] = col.MaxWidth
		}
	}

	// Print header
	headerStyle := lipgloss.NewStyle()
	if useColor {
		headerStyle = headerStyle.Foreground(cs.TextMuted).Bold(true)
	}

	var headerParts []string
	for i, col := range t.columns {
		cell := col.Title + strings.Repeat(" ", max(0, widths[i]-lipgloss.Width(col.Title)))
		if useColor {
			cell = headerStyle.Render(cell)
		}
		headerParts = append(headerParts, cell)
	}
	_, _ = fmt.Fprintln(t.w, strings.Join(headerParts, "  "))

	// Print separator
	var sepParts []string
	for _, w := range widths {
		sepParts = append(sepParts, strings.Repeat("─", w))
	}
	sep := strings.Join(sepParts, "──")
	if useColor {
		sep = lipgloss.NewStyle().Foreground(cs.SurfaceDim).Render(sep)
	}
	_, _ = fmt.Fprintln(t.w, sep)

	// Print rows
	for _, row := range t.rows {
		var parts []string
		for i, val := range row {
			if i >= len(t.columns) {
				break
			}

			// Truncate if needed (rune-aware)
			display := val
			if t.columns[i].MaxWidth > 0 && lipgloss.Width(display) > widths[i] {
				display = truncateToWidth(display, widths[i])
			}

			displayWidth := lipgloss.Width(display)

			// Apply color function if available.
			// ColorFn receives the truncated display string so the rendered
			// width matches the column. Callers that need status-based coloring
			// (e.g., "clean" → green) pass short values that won't be truncated.
			if useColor && t.columns[i].ColorFn != nil {
				colored := t.columns[i].ColorFn(display)
				padding := widths[i] - displayWidth
				if padding > 0 {
					colored += strings.Repeat(" ", padding)
				}
				parts = append(parts, colored)
			} else {
				padding := widths[i] - displayWidth
				if padding > 0 {
					display += strings.Repeat(" ", padding)
				}
				parts = append(parts, display)
			}
		}
		_, _ = fmt.Fprintln(t.w, strings.Join(parts, "  "))
	}
}

// truncateToWidth truncates a string to fit within maxWidth visual cells,
// appending "…" if truncated. Operates on runes to avoid breaking multibyte chars.
func truncateToWidth(s string, maxWidth int) string {
	if maxWidth <= 1 {
		return "…"
	}
	var result []rune
	w := 0
	for _, r := range s {
		rw := lipgloss.Width(string(r))
		if w+rw > maxWidth-1 { // -1 for ellipsis
			break
		}
		result = append(result, r)
		w += rw
	}
	return string(result) + "…"
}
