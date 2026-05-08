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
	updateCommand  string
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
	u.updateCommand = updatecheck.UpdateCommand(updatecheck.DetectInstall())
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

	current := labelStyle.Render("Current:  ") + valueStyle.Render(u.currentVersion)
	latest := labelStyle.Render("Latest:   ") + emphStyle.Render(u.latestVersion)

	runLine := labelStyle.Render("Run:        ") + cmdStyle.Render(u.updateCommand)
	var changelogLine string
	if u.latestURL != "" {
		changelogLine = labelStyle.Render("Changelog:  ") + urlStyle.Render(u.latestURL)
	}

	footerRule := lipgloss.NewStyle().Foreground(Colors.SurfaceBorder).
		Render(strings.Repeat("─", textWidth))
	footerKeys := "  " +
		Styles.HelpKey.Render("esc") + " " + Styles.HelpDesc.Render("close") +
		Styles.HelpSep.Render(" · ") +
		Styles.HelpKey.Render("u") + " " + Styles.HelpDesc.Render("close")

	parts := []string{
		title,
		"",
		current,
		latest,
		"",
		runLine,
	}
	if changelogLine != "" {
		parts = append(parts, changelogLine)
	}
	parts = append(parts, "", footerRule, footerKeys)

	content := strings.Join(parts, "\n")

	return Styles.OverlayBorderInfo.
		Width(w).
		Height(ht).
		Render(content)
}

// calcUpdateOverlaySize sizes the overlay relative to the terminal.
// Tighter bounds than the help overlay because the modal is short.
func calcUpdateOverlaySize(termW, termH int) (w, h int) {
	w = clamp(termW*60/100, 50, 80)
	h = clamp(termH*40/100, 11, 16)
	return
}
