package updatecheck

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"charm.land/lipgloss/v2"
)

// Severity describes the difference between current and latest semver.
type Severity int

const (
	SeverityNone Severity = iota
	SeverityPatch
	SeverityMinor
	SeverityMajor
)

// String renders the severity as a human-readable name.
func (s Severity) String() string {
	switch s {
	case SeverityPatch:
		return "patch"
	case SeverityMinor:
		return "minor"
	case SeverityMajor:
		return "major"
	default:
		return "none"
	}
}

// CompareSemver returns the severity of upgrade from current to latest.
// SeverityNone means current is already at or ahead of latest, or either is unparseable.
func CompareSemver(current, latest string) Severity {
	c, ok := parseSemver(current)
	if !ok {
		return SeverityNone
	}
	l, ok := parseSemver(latest)
	if !ok {
		return SeverityNone
	}
	if l[0] > c[0] {
		return SeverityMajor
	}
	if l[0] < c[0] {
		return SeverityNone
	}
	if l[1] > c[1] {
		return SeverityMinor
	}
	if l[1] < c[1] {
		return SeverityNone
	}
	if l[2] > c[2] {
		return SeverityPatch
	}
	return SeverityNone
}

// RenderBox returns the formatted box notification as a string.
// When NO_COLOR is set, the output is plain ASCII (no ANSI escapes).
func RenderBox(currentVersion, latestVersion, latestURL, updateCmd string) string {
	severity := CompareSemver(currentVersion, latestVersion)
	body := []string{
		fmt.Sprintf("Update available  %s  →  %s", currentVersion, latestVersion),
		"Run: " + updateCmd,
		"Changelog: " + latestURL,
	}
	if os.Getenv("NO_COLOR") != "" {
		return renderPlain(body)
	}
	return renderColored(body, severity)
}

func renderPlain(lines []string) string {
	width := 0
	for _, l := range lines {
		if len(l) > width {
			width = len(l)
		}
	}
	var b strings.Builder
	b.WriteString("+" + strings.Repeat("-", width+2) + "+\n")
	for _, l := range lines {
		b.WriteString("| ")
		b.WriteString(l)
		b.WriteString(strings.Repeat(" ", width-len(l)))
		b.WriteString(" |\n")
	}
	b.WriteString("+" + strings.Repeat("-", width+2) + "+\n")
	return b.String()
}

func renderColored(lines []string, severity Severity) string {
	color := severityColor(severity)
	style := lipgloss.NewStyle().
		Border(lipgloss.NormalBorder()).
		BorderForeground(lipgloss.Color(color)).
		Padding(0, 1)
	return style.Render(strings.Join(lines, "\n")) + "\n"
}

func severityColor(s Severity) string {
	switch s {
	case SeverityMajor:
		return "9" // bright red
	case SeverityMinor:
		return "11" // yellow
	case SeverityPatch:
		return "8" // dim/gray
	default:
		return "7"
	}
}

func parseSemver(v string) ([3]int, bool) {
	v = strings.TrimPrefix(v, "v")
	parts := strings.Split(v, ".")
	if len(parts) != 3 {
		return [3]int{}, false
	}
	var out [3]int
	for i, p := range parts {
		n, err := strconv.Atoi(p)
		if err != nil || n < 0 {
			return [3]int{}, false
		}
		out[i] = n
	}
	return out, true
}
