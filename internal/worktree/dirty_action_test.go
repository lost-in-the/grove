package worktree

import (
	"testing"
)

func TestResolveDirtyAction(t *testing.T) {
	tests := []struct {
		name          string
		dirtyHandling string
		isDirty       bool
		isPeek        bool
		isInteractive bool
		want          DirtyAction
	}{
		// Peek always allows, regardless of dirty state or config
		{
			name:          "peek skips dirty handling even when dirty",
			dirtyHandling: "refuse",
			isDirty:       true,
			isPeek:        true,
			isInteractive: true,
			want:          DirtyAllow,
		},
		{
			name:          "peek skips dirty handling when clean",
			dirtyHandling: "refuse",
			isDirty:       false,
			isPeek:        true,
			isInteractive: false,
			want:          DirtyAllow,
		},

		// Clean worktree always allows, regardless of config
		{
			name:          "refuse allows clean worktree",
			dirtyHandling: "refuse",
			isDirty:       false,
			isPeek:        false,
			isInteractive: true,
			want:          DirtyAllow,
		},
		{
			name:          "auto-stash allows clean worktree",
			dirtyHandling: "auto-stash",
			isDirty:       false,
			isPeek:        false,
			isInteractive: true,
			want:          DirtyAllow,
		},
		{
			name:          "prompt allows clean worktree",
			dirtyHandling: "prompt",
			isDirty:       false,
			isPeek:        false,
			isInteractive: true,
			want:          DirtyAllow,
		},

		// Refuse mode with dirty worktree
		{
			name:          "refuse blocks dirty worktree",
			dirtyHandling: "refuse",
			isDirty:       true,
			isPeek:        false,
			isInteractive: true,
			want:          DirtyRefuse,
		},
		{
			name:          "refuse blocks dirty worktree non-interactive",
			dirtyHandling: "refuse",
			isDirty:       true,
			isPeek:        false,
			isInteractive: false,
			want:          DirtyRefuse,
		},

		// Auto-stash mode with dirty worktree
		{
			name:          "auto-stash stashes dirty worktree",
			dirtyHandling: "auto-stash",
			isDirty:       true,
			isPeek:        false,
			isInteractive: true,
			want:          DirtyStash,
		},
		{
			name:          "auto-stash stashes dirty worktree non-interactive",
			dirtyHandling: "auto-stash",
			isDirty:       true,
			isPeek:        false,
			isInteractive: false,
			want:          DirtyStash,
		},

		// Prompt mode
		{
			name:          "prompt prompts on dirty interactive",
			dirtyHandling: "prompt",
			isDirty:       true,
			isPeek:        false,
			isInteractive: true,
			want:          DirtyPrompt,
		},
		{
			name:          "prompt refuses on dirty non-interactive",
			dirtyHandling: "prompt",
			isDirty:       true,
			isPeek:        false,
			isInteractive: false,
			want:          DirtyRefuse,
		},

		// Unknown config value defaults to refuse (safety fallback)
		{
			name:          "unknown handling defaults to refuse",
			dirtyHandling: "bogus",
			isDirty:       true,
			isPeek:        false,
			isInteractive: true,
			want:          DirtyRefuse,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ResolveDirtyAction(tt.dirtyHandling, tt.isDirty, tt.isPeek, tt.isInteractive)
			if got != tt.want {
				t.Errorf("ResolveDirtyAction(%q, dirty=%v, peek=%v, interactive=%v) = %v, want %v",
					tt.dirtyHandling, tt.isDirty, tt.isPeek, tt.isInteractive, got, tt.want)
			}
		})
	}
}
