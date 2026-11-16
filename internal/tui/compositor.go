package tui

import (
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
)

// backdropStyle dims everything behind an overlay. Faint carries no color,
// so the dim looks the same under NO_COLOR, high contrast, and every theme.
var backdropStyle = lipgloss.NewStyle().Faint(true)

// composeOverlay draws overlay centered over a dimmed copy of bg.
func composeOverlay(bg, overlay string, width, height int) string {
	return composeOverlayAt(bg, overlay, width, height, -1)
}

// composeOverlayAt draws overlay over a dimmed copy of bg, horizontally
// centered, with its top edge at row top (top < 0 centers vertically). An
// overlay that would run off the bottom moves up. The result is exactly
// height lines, none wider than width. Background cells outside the
// overlay's rectangle are kept.
func composeOverlayAt(bg, overlay string, width, height, top int) string {
	if width <= 0 || height <= 0 {
		return overlay
	}
	box := strings.Join(fitLines(overlay, width, min(lipgloss.Height(overlay), height)), "\n")
	boxH := lipgloss.Height(box)
	x := max(0, (width-lipgloss.Width(box))/2)
	y := (height - boxH) / 2
	if top >= 0 {
		y = top
	}
	y = max(0, min(y, height-boxH))
	return lipgloss.NewCompositor(
		lipgloss.NewLayer(dimBackdrop(bg, width, height)),
		lipgloss.NewLayer(box).X(x).Y(y).Z(1),
	).Render()
}

// composeLayerAt draws layer over bg without dimming it, with the layer's
// top-left corner at column x, row y. The result is exactly height lines,
// none wider than width. Toasts use it to sit above every other layer.
func composeLayerAt(bg, layer string, width, height, x, y int) string {
	if width <= 0 || height <= 0 || layer == "" {
		return bg
	}
	base := fitLines(bg, width, height)
	for i, line := range base {
		if pad := width - ansi.StringWidth(line); pad > 0 {
			base[i] = line + strings.Repeat(" ", pad)
		}
	}
	box := strings.Join(fitLines(layer, width, min(lipgloss.Height(layer), height)), "\n")
	x = max(0, min(x, width-lipgloss.Width(box)))
	y = max(0, min(y, height-lipgloss.Height(box)))
	return lipgloss.NewCompositor(
		lipgloss.NewLayer(strings.Join(base, "\n")),
		lipgloss.NewLayer(box).X(x).Y(y).Z(2),
	).Render()
}

// dimBackdrop strips all styling from bg and renders each line faint. Styling
// is stripped first because an inner reset would otherwise end the dim partway
// through a line. Lines are padded to width so the canvas is always full size.
func dimBackdrop(bg string, width, height int) string {
	lines := fitLines(bg, width, height)
	for i, line := range lines {
		plain := ansi.Strip(line)
		if pad := width - ansi.StringWidth(plain); pad > 0 {
			plain += strings.Repeat(" ", pad)
		}
		lines[i] = backdropStyle.Render(plain)
	}
	return strings.Join(lines, "\n")
}

// fitLines returns exactly height lines of s, each truncated to width cells.
func fitLines(s string, width, height int) []string {
	lines := strings.Split(s, "\n")
	if len(lines) > height {
		lines = lines[:height]
	}
	for len(lines) < height {
		lines = append(lines, "")
	}
	for i, line := range lines {
		lines[i] = ansi.Truncate(line, width, "")
	}
	return lines
}
