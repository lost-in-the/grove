package tui

import "strings"

// overlayIndent is the consistent left-padding for all content inside overlays.
const overlayIndent = "  "

// padToHeight pads content with blank lines so it reaches at least minLines.
// This prevents overlay height jitter when switching between wizard steps.
func padToHeight(content string, minLines int) string {
	lines := strings.Count(content, "\n")
	if lines >= minLines {
		return content
	}
	return content + strings.Repeat("\n", minLines-lines)
}

// indentBlock prepends indent to every line of a multi-line string.
func indentBlock(s, indent string) string {
	return indent + strings.ReplaceAll(s, "\n", "\n"+indent)
}

// calcOverlayWidth computes the standard overlay width from terminal width.
// The overlay is 50% of terminal width, clamped to [50, 70].
func calcOverlayWidth(width int) int {
	w := width * 50 / 100
	if w < 50 {
		w = 50
	}
	if w > 70 {
		w = 70
	}
	return w
}

// overlayDims holds precomputed overlay layout dimensions.
type overlayDims struct {
	overlay int // total overlay width (border + padding + content)
	content int // usable width inside the border
	inner   int // content minus indent on both sides
	indent  string
}

// calcOverlayDims computes standard overlay dimensions from terminal width.
func calcOverlayDims(termWidth int) overlayDims {
	ow := calcOverlayWidth(termWidth)
	cw := ow - 6
	return overlayDims{
		overlay: ow,
		content: cw,
		inner:   cw - len(overlayIndent)*2,
		indent:  overlayIndent,
	}
}
