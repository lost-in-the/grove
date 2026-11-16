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

// An entry with no Short only carries metadata for the dashboard detail pane
// (where the shared stack is). It is not a service, so --json leaves it out.
func TestLsServices_SkipsEntriesWithoutShort(t *testing.T) {
	got := lsServices([]plugins.StatusEntry{
		{ProviderName: "docker", Stack: &plugins.SharedStack{OnPath: "/work/myapp-main"}},
		{ProviderName: "docker", Short: "3 up", Level: plugins.StatusActive, Detail: "3 running"},
	})

	if len(got) != 1 {
		t.Fatalf("lsServices() returned %d services, want 1: %+v", len(got), got)
	}
	if got[0].Provider != "docker" || got[0].Status != "3 up" || got[0].Detail != "3 running" {
		t.Errorf("lsServices()[0] = %+v, want the docker entry with status %q", got[0], "3 up")
	}
}

func TestLsServices_OnlyMetadataEntriesGiveNoServices(t *testing.T) {
	got := lsServices([]plugins.StatusEntry{
		{ProviderName: "docker", Stack: &plugins.SharedStack{OnPath: "/work/myapp-main"}},
	})
	if got != nil {
		t.Errorf("lsServices() = %+v, want nil so the services key is omitted", got)
	}
}
