package commands

import (
	"testing"

	"github.com/lost-in-the/grove/internal/plugins"
)

func TestContainersSummary(t *testing.T) {
	tests := []struct {
		name    string
		entries []plugins.StatusEntry
		want    string
	}{
		{
			name:    "nil entries",
			entries: nil,
			want:    "",
		},
		{
			name:    "single entry",
			entries: []plugins.StatusEntry{{Short: "3 up"}},
			want:    "3 up",
		},
		{
			name: "multiple entries joined, not overwritten",
			entries: []plugins.StatusEntry{
				{Short: "3 up"},
				{Short: "1 stale"},
			},
			want: "3 up,1 stale",
		},
		{
			name: "empty shorts skipped",
			entries: []plugins.StatusEntry{
				{Short: "3 up"},
				{Short: ""},
			},
			want: "3 up",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := containersSummary(tt.entries); got != tt.want {
				t.Errorf("containersSummary() = %q, want %q", got, tt.want)
			}
		})
	}
}
