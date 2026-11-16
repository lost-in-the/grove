package tui

import (
	"fmt"
	"time"
)

// shortAge formats how long ago t was the way git does for relative dates
// (date.c show_date_relative): the same unit thresholds and rounding, written
// short in at most four cells: 45s, 89m, 35h, 13d, 10w, 11mo, 2y. A future
// time gives "now"; a zero time gives "".
func shortAge(t, now time.Time) string {
	if t.IsZero() {
		return ""
	}
	if now.Before(t) {
		return "now"
	}
	diff := int64(now.Sub(t) / time.Second)
	if diff < 90 {
		return fmt.Sprintf("%ds", diff)
	}
	diff = (diff + 30) / 60 // minutes
	if diff < 90 {
		return fmt.Sprintf("%dm", diff)
	}
	diff = (diff + 30) / 60 // hours
	if diff < 36 {
		return fmt.Sprintf("%dh", diff)
	}
	diff = (diff + 12) / 24 // days
	switch {
	case diff < 14:
		return fmt.Sprintf("%dd", diff)
	case diff < 70:
		return fmt.Sprintf("%dw", (diff+3)/7)
	case diff < 365:
		return fmt.Sprintf("%dmo", (diff+15)/30)
	case diff < 1825:
		// git says "1 year, 2 months"; the row keeps the years.
		return fmt.Sprintf("%dy", (diff*12*2+365)/(365*2)/12)
	}
	return fmt.Sprintf("%dy", (diff+183)/365)
}
