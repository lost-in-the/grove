package tui

import (
	"os"
	"strings"

	"github.com/charmbracelet/glamour"
)

// renderMarkdown renders markdown to styled terminal output using glamour.
// Used for user-provided content (PR bodies, issue bodies).
// Respects NO_COLOR and GROVE_NO_COLOR environment variables.
//
// Uses WithStandardStyle("dark") instead of WithAutoStyle() to avoid
// terminal queries (OSC 11) that leak through Bubbletea's input parser
// as spurious key press events.
func renderMarkdown(md string, width int) string {
	var opts []glamour.TermRendererOption
	opts = append(opts, glamour.WithWordWrap(width))
	if os.Getenv("NO_COLOR") != "" || os.Getenv("GROVE_NO_COLOR") != "" {
		opts = append(opts, glamour.WithStylePath("notty"))
	} else {
		opts = append(opts, glamour.WithStandardStyle("dark"))
	}
	r, err := glamour.NewTermRenderer(opts...)
	if err != nil {
		return md
	}
	out, err := r.Render(md)
	if err != nil {
		return md
	}
	return strings.TrimSpace(out)
}
