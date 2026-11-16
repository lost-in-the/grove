package tui

import (
	"os"
	"strings"

	"charm.land/glamour/v2"
)

// renderMarkdown renders markdown to styled terminal output using glamour.
// Used for user-provided content (PR bodies, issue bodies).
// Respects NO_COLOR and GROVE_NO_COLOR environment variables.
//
// Uses a fixed "dark" style: glamour v2 does not detect the terminal
// background, so the style is chosen here.
//
// md is someone else's text, so its control characters are removed first,
// and glamour's output is filtered too: markdown's character references
// (&#x1b;) become control characters inside glamour, after any filter on
// its input. Only glamour's own SGR styling survives.
func renderMarkdown(md string, width int) string {
	md = safeMarkdown(md)
	var opts []glamour.TermRendererOption
	opts = append(opts, glamour.WithWordWrap(width))
	if os.Getenv("NO_COLOR") != "" || os.Getenv("GROVE_NO_COLOR") != "" {
		opts = append(opts, glamour.WithStylePath("notty"))
	} else {
		opts = append(opts, glamour.WithStandardStyle("dark"))
	}
	r, err := glamour.NewTermRenderer(opts...)
	if err != nil {
		return safeRendered(md)
	}
	out, err := r.Render(md)
	if err != nil {
		return safeRendered(md)
	}
	return strings.TrimSpace(safeRendered(out))
}
