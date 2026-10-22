package tui

import (
	"sort"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/lost-in-the/grove/plugins/tracker"
)

// A check's state, reduced to the three the review shows (plan 8c
// decision 3). Larger sorts first.
const (
	checkPassed = iota
	checkRunning
	checkFailed
)

// checkBucket sorts a PRCheck state into passed, failed, or running.
func checkBucket(state string) int {
	switch state {
	case "SUCCESS", "NEUTRAL", "SKIPPED":
		return checkPassed
	case "FAILURE", "ERROR", "TIMED_OUT", "ACTION_REQUIRED", "CANCELLED", "STARTUP_FAILURE": //nolint:misspell // GitHub's CheckConclusionState spelling
		return checkFailed
	}
	return checkRunning
}

// checkItems is the checks tab: one row per check, failing first, then
// running, then passing, each as "glyph name  STATE".
func checkItems(checks []tracker.PRCheck) [][]string {
	sorted := append([]tracker.PRCheck(nil), checks...)
	sort.SliceStable(sorted, func(a, b int) bool { return checkBucket(sorted[a].State) > checkBucket(sorted[b].State) })
	items := make([][]string, len(sorted))
	for i, c := range sorted {
		glyph := Styles.SuccessText.Render("✓")
		switch checkBucket(c.State) {
		case checkFailed:
			glyph = Styles.ErrorText.Render("✗")
		case checkRunning:
			glyph = Styles.WarningText.Render("●")
		}
		items[i] = []string{glyph + " " + safeText(c.Name) + "  " + Styles.TextMuted.Render(safeText(c.State))}
	}
	return items
}

// renderItems draws a tab's items, each one or more lines, with a bar in
// the gutter of the cursor's item and the window scrolled to keep it in
// view.
func renderItems(items [][]string, cursor, width, height int) string {
	var lines []string
	at := 0
	for i, it := range items {
		gutter := "  "
		if i == cursor {
			at = len(lines)
			gutter = lipgloss.NewStyle().Foreground(Colors.Primary).Render("▌ ")
		}
		for _, l := range it {
			lines = append(lines, ansi.Truncate(gutter+l, width, "…"))
		}
	}
	start := max(0, min(at-height/3, len(lines)-height))
	return strings.Join(lines[start:min(len(lines), start+height)], "\n")
}
