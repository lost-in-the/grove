package tui

import (
	"strings"
	"testing"
)

func TestSyncStatusText(t *testing.T) {
	tests := []struct {
		name string
		item WorktreeItem
		want string
	}{
		{
			name: "Synced with remote",
			item: WorktreeItem{HasRemote: true, AheadCount: 0, BehindCount: 0},
			want: "✓ synced",
		},
		{
			name: "Ahead only",
			item: WorktreeItem{HasRemote: true, AheadCount: 3},
			want: "↑3",
		},
		{
			name: "Behind only",
			item: WorktreeItem{HasRemote: true, BehindCount: 2},
			want: "↓2",
		},
		{
			name: "Diverged",
			item: WorktreeItem{HasRemote: true, AheadCount: 2, BehindCount: 1},
			want: "↑2 ↓1",
		},
		{
			name: "No remote",
			item: WorktreeItem{HasRemote: false},
			want: "⚠ no remote",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.item.SyncStatusText()
			if !strings.Contains(got, tt.want) {
				t.Errorf("SyncStatusText() = %q, want to contain %q", got, tt.want)
			}
		})
	}
}

func TestRenderSyncValue_AllCases(t *testing.T) {
	tests := []struct {
		name     string
		item     WorktreeItem
		wantText string
	}{
		{
			name:     "Synced shows checkmark",
			item:     WorktreeItem{HasRemote: true, AheadCount: 0, BehindCount: 0},
			wantText: "synced",
		},
		{
			name:     "No remote shows warning",
			item:     WorktreeItem{HasRemote: false},
			wantText: "no remote",
		},
		{
			name:     "Ahead shows up arrow",
			item:     WorktreeItem{HasRemote: true, AheadCount: 5},
			wantText: "↑5",
		},
		{
			name:     "Behind shows down arrow",
			item:     WorktreeItem{HasRemote: true, BehindCount: 3},
			wantText: "↓3",
		},
		{
			name:     "Diverged shows both",
			item:     WorktreeItem{HasRemote: true, AheadCount: 2, BehindCount: 4},
			wantText: "↑2",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := renderSyncValue(&tt.item)
			if !strings.Contains(got, tt.wantText) {
				t.Errorf("renderSyncValue() = %q, want to contain %q", got, tt.wantText)
			}
		})
	}
}

func TestRenderDetailV2_SyncWithHasRemote(t *testing.T) {
	tests := []struct {
		name       string
		item       WorktreeItem
		wantText   string
		wantAbsent string
	}{
		{
			name:     "Has remote and synced shows Sync row",
			item:     WorktreeItem{ShortName: "t", Branch: "main", HasRemote: true},
			wantText: "synced",
		},
		{
			name:       "No remote hides Sync row",
			item:       WorktreeItem{ShortName: "t", Branch: "main", HasRemote: false},
			wantAbsent: "Sync",
		},
		{
			name:     "Has remote ahead shows sync",
			item:     WorktreeItem{ShortName: "t", Branch: "main", HasRemote: true, AheadCount: 3},
			wantText: "↑3",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := renderDetailV2(&tt.item, 60)
			if tt.wantText != "" && !strings.Contains(got, tt.wantText) {
				t.Errorf("expected %q in output, got:\n%s", tt.wantText, got)
			}
			if tt.wantAbsent != "" && strings.Contains(got, tt.wantAbsent) {
				t.Errorf("did not expect %q in output, got:\n%s", tt.wantAbsent, got)
			}
		})
	}
}

func TestGetUpstreamInfo_InvalidPath(t *testing.T) {
	ahead, behind, hasRemote, trackingBranch := getUpstreamInfo("/nonexistent/path")
	if hasRemote {
		t.Error("getUpstreamInfo() should return hasRemote=false for nonexistent path")
	}
	if ahead != 0 || behind != 0 {
		t.Errorf("getUpstreamInfo() = (%d, %d, _, _), want (0, 0, _, _)", ahead, behind)
	}
	if trackingBranch != "" {
		t.Errorf("getUpstreamInfo() trackingBranch = %q, want empty", trackingBranch)
	}
}
