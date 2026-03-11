package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/glamour"

	"github.com/lost-in-the/grove/plugins/tracker"
)

// renderPRPreview renders a detailed preview panel for a single PR.
func renderPRPreview(pr *tracker.PullRequest, width int) string {
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
	b.WriteString(Styles.Footer.Render("[enter] Create worktree  [o] Open in browser  [tab] Back  [esc] Close"))

	return Styles.OverlayBorderInfo.Render(b.String())
}

// renderMarkdown renders markdown to styled terminal output using glamour.
func renderMarkdown(md string, width int) string {
	r, err := glamour.NewTermRenderer(
		glamour.WithAutoStyle(),
		glamour.WithWordWrap(width),
	)
	if err != nil {
		return md
	}
	out, err := r.Render(md)
	if err != nil {
		return md
	}
	return strings.TrimSpace(out)
}
