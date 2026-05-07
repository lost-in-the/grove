package updatecheck

import (
	"strconv"
	"strings"
)

// Severity describes the difference between current and latest semver.
type Severity int

const (
	SeverityNone Severity = iota
	SeverityPatch
	SeverityMinor
	SeverityMajor
)

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

func parseSemver(v string) ([3]int, bool) {
	v = strings.TrimPrefix(v, "v")
	parts := strings.Split(v, ".")
	if len(parts) != 3 {
		return [3]int{}, false
	}
	var out [3]int
	for i, p := range parts {
		n, err := strconv.Atoi(p)
		if err != nil {
			return [3]int{}, false
		}
		out[i] = n
	}
	return out, true
}
