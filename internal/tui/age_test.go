package tui

import (
	"testing"
	"time"
)

func TestShortAge(t *testing.T) {
	now := time.Date(2026, 10, 11, 12, 0, 0, 0, time.UTC)
	day := 24 * time.Hour
	for _, c := range []struct {
		ago  time.Duration
		want string
	}{
		{0, "now"}, {59 * time.Second, "now"},
		{time.Minute, "1m"}, {59 * time.Minute, "59m"},
		{time.Hour, "1h"}, {23 * time.Hour, "23h"},
		{day, "1d"}, {6 * day, "6d"},
		{7 * day, "1w"}, {52 * 7 * day, "52w"},
		{365 * day, "1y"}, {800 * day, "2y"},
		{-time.Hour, "now"},
	} {
		if got := shortAge(now.Add(-c.ago), now); got != c.want {
			t.Errorf("%v ago: got %q, want %q", c.ago, got, c.want)
		}
	}
	if got := shortAge(time.Time{}, now); got != "" {
		t.Errorf("zero time: got %q, want empty", got)
	}
}
