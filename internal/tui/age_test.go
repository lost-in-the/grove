package tui

import (
	"testing"
	"time"
)

// shortAge follows git's relative dates (date.c show_date_relative): the
// same unit thresholds and rounding, written short. A row then agrees with
// git log --date=relative and with the detail pane.
func TestShortAge(t *testing.T) {
	now := time.Date(2026, 10, 11, 12, 0, 0, 0, time.UTC)
	sec, min, hour, day := time.Second, time.Minute, time.Hour, 24*time.Hour
	for _, c := range []struct {
		ago  time.Duration
		want string
	}{
		{0, "0s"}, {59 * sec, "59s"}, {89 * sec, "89s"},
		{90 * sec, "2m"}, {89 * min, "89m"},
		{90 * min, "2h"}, {35 * hour, "35h"},
		{36 * hour, "2d"}, {13 * day, "13d"},
		{14 * day, "2w"}, {69 * day, "10w"},
		{70 * day, "2mo"}, {364 * day, "12mo"},
		{365 * day, "1y"}, {800 * day, "2y"}, {2000 * day, "5y"},
		{-hour, "now"},
	} {
		if got := shortAge(now.Add(-c.ago), now); got != c.want {
			t.Errorf("%v ago: got %q, want %q", c.ago, got, c.want)
		}
	}
	if got := shortAge(time.Time{}, now); got != "" {
		t.Errorf("zero time: got %q, want empty", got)
	}
}
