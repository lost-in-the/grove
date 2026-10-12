package tui

import (
	"fmt"
	"time"
)

// shortAge formats how long ago t was in at most three cells: now, 59m,
// 23h, 6d, 52w, 2y. There is no month unit, so m always means minutes. A
// zero time gives "".
func shortAge(t, now time.Time) string {
	if t.IsZero() {
		return ""
	}
	d := now.Sub(t)
	day := 24 * time.Hour
	switch {
	case d < time.Minute:
		return "now"
	case d < time.Hour:
		return fmt.Sprintf("%dm", int(d/time.Minute))
	case d < day:
		return fmt.Sprintf("%dh", int(d/time.Hour))
	case d < 7*day:
		return fmt.Sprintf("%dd", int(d/day))
	case d < 365*day:
		return fmt.Sprintf("%dw", int(d/(7*day)))
	}
	return fmt.Sprintf("%dy", int(d/(365*day)))
}
