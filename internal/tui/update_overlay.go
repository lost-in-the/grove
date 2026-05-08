package tui

import (
	"strings"

	"charm.land/lipgloss/v2"

	"github.com/lost-in-the/grove/internal/updatecheck"
)

// UpdateOverlay renders an informational modal showing release update details.
// It's purely informational — no auto-update functionality. The user runs the
// suggested install command in their shell to upgrade.
type UpdateOverlay struct {
	Active bool

	// Cached at open-time so View() doesn't repeatedly read the cache file.
	currentVersion string
	latestVersion  string
	latestURL      string
}

// NewUpdateOverlay creates an inactive UpdateOverlay.
func NewUpdateOverlay() *UpdateOverlay {
	return &UpdateOverlay{}
}

// Open activates the overlay populated from the supplied release info.
// The caller is responsible for verifying that an update is actually available
// (typically via updatecheck.CachedRelease) before calling Open.
func (u *UpdateOverlay) Open(currentVersion, latestVersion, latestURL string) {
	u.Active = true
	u.currentVersion = currentVersion
	u.latestVersion = latestVersion
	u.latestURL = latestURL
}

// Close deactivates the overlay.
func (u *UpdateOverlay) Close() {
	u.Active = false
}

// View renders the overlay panel centered to the given terminal dimensions.
func (u *UpdateOverlay) View(width, height int) string {
	w, ht := calcUpdateOverlaySize(width, height)

	// Width() in lipgloss includes border + padding + text. The OverlayBorderInfo
	// uses Border (2) + Padding(1,2) (4 horizontal) — same accounting as HelpOverlay.
	textWidth := w - 6
	if textWidth < 30 {
		textWidth = 30
	}

	title := Styles.OverlayTitle.Render("↑ Update available")

	labelStyle := lipgloss.NewStyle().Foreground(Colors.TextMuted)
	valueStyle := lipgloss.NewStyle().Foreground(Colors.TextNormal)
	emphStyle := lipgloss.NewStyle().Bold(true).Foreground(Colors.TextBright)
	cmdStyle := lipgloss.NewStyle().Foreground(Colors.Primary)
	urlStyle := lipgloss.NewStyle().Foreground(Colors.Info).Underline(true)

	// Align all values at the same column. "Changelog:" is 10 chars; pad
	// labels so values start at column 12 (matches the previous layout).
	const valueColumn = 12
	pad := func(label string) string {
		text := label + ":"
		n := valueColumn - len(text)
		if n < 1 {
			n = 1
		}
		return labelStyle.Render(text + strings.Repeat(" ", n))
	}

	current := pad("Current") + valueStyle.Render(u.currentVersion)
	latest := pad("Latest") + emphStyle.Render(u.latestVersion)

	// Always show all three install methods. The user picks whichever applies
	// to their setup. The CLI box (render.go) still uses DetectInstall to pick
	// a single method for its space-constrained one-liner.
	brewLine := pad("Brew") + cmdStyle.Render(updatecheck.UpdateCommand(updatecheck.InstallBrew))
	goLine := pad("Go") + cmdStyle.Render(updatecheck.UpdateCommand(updatecheck.InstallGoInstall))
	binaryLine := pad("Binary") + urlStyle.Render(updatecheck.UpdateCommand(updatecheck.InstallBinary))

	var changelogLine string
	if u.latestURL != "" {
		changelogLine = pad("Changelog") + urlStyle.Render(u.latestURL)
	}

	footerRule := lipgloss.NewStyle().Foreground(Colors.SurfaceBorder).
		Render(strings.Repeat("─", textWidth))
	// Group both keys with a single action label, matching the project's
	// other overlay footers (help_overlay.go).
	footerKeys := "  " +
		Styles.HelpKey.Render("esc") +
		Styles.HelpSep.Render(" · ") +
		Styles.HelpKey.Render("u") + "   " + Styles.HelpDesc.Render("close")

	parts := []string{
		title,
		"",
		current,
		latest,
		"",
		brewLine,
		goLine,
		binaryLine,
	}
	if changelogLine != "" {
		parts = append(parts, "", changelogLine)
	}
	parts = append(parts, "", footerRule, footerKeys)

	content := strings.Join(parts, "\n")

	// Size Height to the actual content so the box hugs its rows instead of
	// padding the bottom. OverlayBorderInfo adds 2 (border) + 2 (vertical
	// padding) = 4 rows of overhead beyond the content lines. Cap at the
	// terminal-derived max so very short terminals still clip gracefully.
	contentLines := strings.Count(content, "\n") + 1
	const borderOverhead = 4
	desiredH := contentLines + borderOverhead
	if desiredH > ht {
		desiredH = ht
	}

	return Styles.OverlayBorderInfo.
		Width(w).
		Height(desiredH).
		Render(content)
}

// calcUpdateOverlaySize sizes the overlay relative to the terminal.
// h is the maximum allowed height; the overlay shrinks to its content
// when shorter, and clips on tiny terminals. w is sized to fit the
// longest install command without wrapping when possible.
func calcUpdateOverlaySize(termW, termH int) (w, h int) {
	// Target 74 cols so `go install github.com/lost-in-the/grove/cmd/grove@latest`
	// (56 chars) fits on a single line with the 12-char label column and
	// 6-char border+padding overhead. Cap at 80 (standard terminal) and at
	// the terminal width itself to avoid horizontal overrun on tiny screens.
	want := 74
	if termW*70/100 > want {
		want = termW * 70 / 100
	}
	if want > 80 {
		want = 80
	}
	if want > termW {
		want = termW
	}
	w = want
	// Content is up to ~17 lines (title + blank + 2 versions + blank + 3
	// methods + blank + changelog + blank + rule + footer + border/pad).
	// 22 gives headroom; floor 14 prevents micro-boxes on tiny terminals.
	h = clamp(termH*60/100, 14, 22)
	return
}
