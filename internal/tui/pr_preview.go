package tui

import (
	"fmt"
	"os"
	"strings"

	"github.com/charmbracelet/glamour"

	"github.com/lost-in-the/grove/plugins/tracker"
)

// renderPRPreview renders a detailed preview panel for a single PR.
func renderPRPreview(pr *tracker.PullRequest, width int, footer string) string {
	contentWidth := max(width-6, 30)

	var b strings.Builder

	// Title
	title := pr.Title
	if pr.IsDraft {
		title = Styles.WarningText.Render("[DRAFT]") + " " + title
	}
	b.WriteString(Styles.OverlayTitle.Render(fmt.Sprintf("#%d  %s", pr.Number, title)))
	b.WriteString("\n\n")

	// Metadata row
	meta := []string{
		Styles.DetailDim.Render("Branch: ") + pr.Branch,
		Styles.DetailDim.Render("Author: ") + "@" + pr.Author,
	}
	if pr.CommitCount > 0 {
		meta = append(meta, Styles.DetailDim.Render(formatCommitCount(pr.CommitCount)))
	}
	if pr.Additions > 0 || pr.Deletions > 0 {
		meta = append(meta, formatDiffStats(pr.Additions, pr.Deletions))
	}
	if pr.ReviewDecision != "" {
		style := Styles.DetailDim
		switch pr.ReviewDecision {
		case "APPROVED":
			style = Styles.SuccessText
		case "CHANGES_REQUESTED":
			style = Styles.ErrorText
		}
		meta = append(meta, style.Render(pr.ReviewDecision))
	}
	b.WriteString(strings.Join(meta, "  ·  "))
	b.WriteString("\n")
	b.WriteString(Styles.DetailDim.Render(strings.Repeat("─", contentWidth)))
	b.WriteString("\n\n")

	// Body
	if pr.Body == "" {
		b.WriteString(Styles.DetailDim.Render("No description provided."))
	} else {
		rendered := renderMarkdown(pr.Body, contentWidth)
		b.WriteString(rendered)
	}

	b.WriteString("\n\n")
	b.WriteString(footer)

	return Styles.OverlayBorderInfo.Render(b.String())
}

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
