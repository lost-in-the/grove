package cli

import (
	"fmt"
	"strings"

	lipgloss "charm.land/lipgloss/v2"

	"github.com/lost-in-the/grove/internal/theme"
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

	useColor := t.w.UseColor()
	widths := t.computeWidths()

	_, _ = fmt.Fprintln(t.w, t.renderHeader(widths, useColor))
	_, _ = fmt.Fprintln(t.w, t.renderSeparator(widths, useColor))

	for _, row := range t.rows {
		_, _ = fmt.Fprintln(t.w, t.renderRow(row, widths, useColor))
	}
}

func (t *Table) computeWidths() []int {
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
	for i, col := range t.columns {
		if col.MaxWidth > 0 && widths[i] > col.MaxWidth {
			widths[i] = col.MaxWidth
		}
	}
	return widths
}

func (t *Table) renderHeader(widths []int, useColor bool) string {
	cs := theme.Colors
	headerStyle := lipgloss.NewStyle()
	if useColor {
		headerStyle = headerStyle.Foreground(cs.TextMuted).Bold(true)
	}

	parts := make([]string, 0, len(t.columns))
	for i, col := range t.columns {
		cell := col.Title + strings.Repeat(" ", max(0, widths[i]-lipgloss.Width(col.Title)))
		if useColor {
			cell = headerStyle.Render(cell)
		}
		parts = append(parts, cell)
	}
	return strings.Join(parts, "  ")
}

func (t *Table) renderSeparator(widths []int, useColor bool) string {
	parts := make([]string, 0, len(widths))
	for _, w := range widths {
		parts = append(parts, strings.Repeat("─", w))
	}
	sep := strings.Join(parts, "──")
	if useColor {
		sep = lipgloss.NewStyle().Foreground(theme.Colors.SurfaceDim).Render(sep)
	}
	return sep
}

func (t *Table) renderRow(row []string, widths []int, useColor bool) string {
	var parts []string
	for i, val := range row {
		if i >= len(t.columns) {
			break
		}
		parts = append(parts, t.formatCell(i, val, widths[i], useColor))
	}
	return strings.Join(parts, "  ")
}

func (t *Table) formatCell(colIdx int, val string, width int, useColor bool) string {
	col := t.columns[colIdx]
	display := val
	if col.MaxWidth > 0 && lipgloss.Width(display) > width {
		display = truncateToWidth(display, width)
	}

	displayWidth := lipgloss.Width(display)
	padding := width - displayWidth

	if useColor && col.ColorFn != nil {
		colored := col.ColorFn(display)
		if padding > 0 {
			colored += strings.Repeat(" ", padding)
		}
		return colored
	}

	if padding > 0 {
		display += strings.Repeat(" ", padding)
	}
	return display
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
