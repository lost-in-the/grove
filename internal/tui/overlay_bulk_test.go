package tui

import (
	"strings"
	"testing"
)

// TestRenderBulk_SurfacesDirtyState: bulk delete force-removes every selected
// worktree in one confirm, so the overlay must show which candidates carry
// uncommitted changes — the single-delete overlay already warns, and without
// parity here a bulk delete silently discarded WIP the CLI would refuse
// without --force.
func TestRenderBulk_SurfacesDirtyState(t *testing.T) {
	s := &BulkState{
		Items: []WorktreeItem{
			{ShortName: "clean-wt", Branch: "feat/clean"},
			{ShortName: "dirty-wt", Branch: "feat/dirty", IsDirty: true},
		},
		Selected: []bool{true, true},
	}

	out := renderBulk(s)

	if !strings.Contains(out, "uncommitted changes") {
		t.Errorf("bulk overlay does not flag the dirty worktree:\n%s", out)
	}
	// The summary warning must count only SELECTED dirty items.
	if !strings.Contains(out, "1 selected") {
		t.Errorf("bulk overlay missing selected-dirty summary:\n%s", out)
	}

	// Deselect the dirty one — the summary warning must disappear, the
	// per-row marker stays.
	s.Selected[1] = false
	out = renderBulk(s)
	if strings.Contains(out, "1 selected") {
		t.Errorf("summary warning shown with no dirty item selected:\n%s", out)
	}
}
