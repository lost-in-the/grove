package tui

import (
	"fmt"
	"testing"
	"time"

	"github.com/charmbracelet/x/exp/golden"
)

// =============================================================================
// Dashboard tests
// =============================================================================

func TestGolden_Dashboard(t *testing.T) {
	for _, size := range allSizes {
		t.Run(size.name, func(t *testing.T) {
			m := goldenModel(t, size, withItems(5))
			golden.RequireEqual(t, []byte(m.viewString()))
		})
	}
}

func TestGolden_Dashboard_Empty(t *testing.T) {
	m := goldenModel(t, sizeStandard, withItems(0))
	golden.RequireEqual(t, []byte(m.viewString()))
}

func TestGolden_Dashboard_Loading(t *testing.T) {
	m := goldenModel(t, sizeStandard, withLoading())
	golden.RequireEqual(t, []byte(m.viewString()))
}

func TestGolden_Dashboard_WithToast(t *testing.T) {
	m := goldenModel(t, sizeStandard, withItems(5), withToastVisible("Deleted testing", ToastSuccess))
	golden.RequireEqual(t, []byte(m.viewString()))
}

func TestGolden_Dashboard_SortModes(t *testing.T) {
	modes := []struct {
		name string
		mode SortMode
	}{
		{"name", SortByName},
		{"recent", SortByLastAccessed},
		{"dirty", SortByDirtyFirst},
	}
	for _, tc := range modes {
		t.Run(tc.name, func(t *testing.T) {
			m := goldenModel(t, sizeStandard, withItems(5), withSortMode(tc.mode))
			golden.RequireEqual(t, []byte(m.viewString()))
		})
	}
}

func TestGolden_Dashboard_CompactMode(t *testing.T) {
	for _, size := range []termSize{sizeStandard, sizeWide} {
		t.Run(size.name, func(t *testing.T) {
			m := goldenModel(t, size, withItems(5), withCompactMode())
			golden.RequireEqual(t, []byte(m.viewString()))
		})
	}
}

func TestGolden_Dashboard_HelpExpanded(t *testing.T) {
	for _, size := range []termSize{sizeStandard, sizeWide} {
		t.Run(size.name, func(t *testing.T) {
			m := goldenModel(t, size, withItems(5), withHelpExpanded())
			golden.RequireEqual(t, []byte(m.viewString()))
		})
	}
}

// =============================================================================
// Overlay tests
// =============================================================================

func TestGolden_Overlay_Delete(t *testing.T) {
	t.Run("default", func(t *testing.T) {
		m := goldenModel(t, sizeStandard, withItems(3), withDeleteOverlay())
		golden.RequireEqual(t, []byte(m.viewString()))
	})
	t.Run("with_warnings", func(t *testing.T) {
		m := goldenModel(t, sizeStandard, withItems(3), withDeleteOverlay("Worktree has uncommitted changes", "Branch not merged"))
		golden.RequireEqual(t, []byte(m.viewString()))
	})
}

func TestGolden_Overlay_Create(t *testing.T) {
	t.Run("branch_choice_step", func(t *testing.T) {
		m := goldenModel(t, sizeStandard, withItems(3), withCreateStep(CreateStepBranchChoice))
		golden.RequireEqual(t, []byte(m.viewString()))
	})
	t.Run("branch_select_step", func(t *testing.T) {
		m := goldenModel(t, sizeStandard, withItems(3), withCreateStep(CreateStepBranchSelect))
		golden.RequireEqual(t, []byte(m.viewString()))
	})
	t.Run("name_step", func(t *testing.T) {
		m := goldenModel(t, sizeStandard, withItems(3), withCreateStep(CreateStepName))
		golden.RequireEqual(t, []byte(m.viewString()))
	})
	t.Run("confirm_step", func(t *testing.T) {
		m := goldenModel(t, sizeStandard, withItems(3), withCreateStep(CreateStepConfirm))
		golden.RequireEqual(t, []byte(m.viewString()))
	})
}

func TestGolden_Overlay_Bulk(t *testing.T) {
	t.Run("with_items", func(t *testing.T) {
		m := goldenModel(t, sizeStandard, withItems(5), withBulkOverlay(5))
		golden.RequireEqual(t, []byte(m.viewString()))
	})
	t.Run("empty", func(t *testing.T) {
		m := goldenModel(t, sizeStandard, withItems(3), withBulkOverlay(0))
		golden.RequireEqual(t, []byte(m.viewString()))
	})
}

func TestGolden_Overlay_PRs(t *testing.T) {
	t.Run("with_data", func(t *testing.T) {
		m := goldenModel(t, sizeStandard, withItems(3), withPRData())
		golden.RequireEqual(t, []byte(m.viewString()))
	})
}

func TestGolden_Overlay_Issues(t *testing.T) {
	t.Run("with_data", func(t *testing.T) {
		m := goldenModel(t, sizeStandard, withItems(3), withIssueData())
		golden.RequireEqual(t, []byte(m.viewString()))
	})
}

func TestGolden_Overlay_Fork(t *testing.T) {
	t.Run("confirm", func(t *testing.T) {
		m := goldenModel(t, sizeStandard, withItems(3), withForkOverlay())
		golden.RequireEqual(t, []byte(m.viewString()))
	})
}

func TestGolden_Overlay_Sync(t *testing.T) {
	t.Run("source_step", func(t *testing.T) {
		m := goldenModel(t, sizeStandard, withItems(5), withSyncOverlay())
		golden.RequireEqual(t, []byte(m.viewString()))
	})
}

func TestGolden_Overlay_Config(t *testing.T) {
	t.Run("general_tab", func(t *testing.T) {
		m := goldenModel(t, sizeStandard, withItems(3), withConfigOverlay())
		golden.RequireEqual(t, []byte(m.viewString()))
	})
}

// =============================================================================
// Component tests (isolated render functions)
// =============================================================================

func TestGolden_Component_Header(t *testing.T) {
	for _, size := range []termSize{sizeNarrow, sizeStandard} {
		t.Run(size.name, func(t *testing.T) {
			goldenMu.Lock()
			t.Setenv("NO_COLOR", "1")
			Colors = noColorScheme()
			Styles = NewStyleSet(Colors)
			t.Cleanup(func() {
				Colors = NewColorScheme()
				Styles = NewStyleSet(Colors)
				goldenMu.Unlock()
			})

			h := Header{
				ProjectName:   "grove-cli",
				WorktreeCount: 5,
				CurrentBranch: "main",
				CurrentName:   "root",
			}
			golden.RequireEqual(t, []byte(h.View(size.width)))
		})
	}
}

func TestGolden_Component_Stepper(t *testing.T) {
	steps := []string{"Branch", "Name", "Confirm"}
	for step := 0; step <= len(steps); step++ {
		name := fmt.Sprintf("step_%d", step)
		if step == len(steps) {
			name = "complete"
		}
		t.Run(name, func(t *testing.T) {
			goldenMu.Lock()
			t.Setenv("NO_COLOR", "1")
			Colors = noColorScheme()
			Styles = NewStyleSet(Colors)
			t.Cleanup(func() {
				Colors = NewColorScheme()
				Styles = NewStyleSet(Colors)
				goldenMu.Unlock()
			})

			s := NewStepper(steps...)
			s.Current = step
			golden.RequireEqual(t, []byte(s.View(60)))
		})
	}
}

func TestGolden_Component_Toast(t *testing.T) {
	levels := []struct {
		name  string
		level ToastLevel
	}{
		{"success", ToastSuccess},
		{"warning", ToastWarning},
		{"error", ToastError},
		{"info", ToastInfo},
	}
	for _, tc := range levels {
		t.Run(tc.name, func(t *testing.T) {
			goldenMu.Lock()
			t.Setenv("NO_COLOR", "1")
			Colors = noColorScheme()
			Styles = NewStyleSet(Colors)
			t.Cleanup(func() {
				Colors = NewColorScheme()
				Styles = NewStyleSet(Colors)
				goldenMu.Unlock()
			})

			tm := NewToastModel()
			tm.Current = &Toast{
				Message:   "Operation completed",
				Level:     tc.level,
				Duration:  24 * time.Hour,
				CreatedAt: time.Now(),
			}
			golden.RequireEqual(t, []byte(tm.View(80)))
		})
	}
}

func TestGolden_Component_HelpFooter(t *testing.T) {
	contexts := []struct {
		name string
		view ActiveView
	}{
		{"dashboard", ViewDashboard},
		{"delete", ViewDelete},
		{"create", ViewCreate},
	}
	for _, tc := range contexts {
		t.Run(tc.name, func(t *testing.T) {
			goldenMu.Lock()
			t.Setenv("NO_COLOR", "1")
			Colors = noColorScheme()
			Styles = NewStyleSet(Colors)
			t.Cleanup(func() {
				Colors = NewColorScheme()
				Styles = NewStyleSet(Colors)
				goldenMu.Unlock()
			})

			hf := NewHelpFooter()
			golden.RequireEqual(t, []byte(hf.RenderCompact(tc.view, 80)))
		})
	}
}

// =============================================================================
// Responsive layout tests
// =============================================================================

func TestGolden_Responsive_Layout(t *testing.T) {
	widths := []int{50, 60, 80, 100, 120, 160}
	for _, w := range widths {
		t.Run(fmt.Sprintf("width_%d", w), func(t *testing.T) {
			m := goldenModel(t, termSize{fmt.Sprintf("w%d", w), w, 24}, withItems(5))
			golden.RequireEqual(t, []byte(m.viewString()))
		})
	}
}

// =============================================================================
// Themed tests (Tier 2 — with ANSI color codes)
// =============================================================================

func TestGolden_Themed_Dashboard(t *testing.T) {
	m := goldenModelThemed(t, sizeStandard, withItems(5))
	golden.RequireEqual(t, []byte(m.viewString()))
}

func TestGolden_Themed_StatusBadges(t *testing.T) {
	goldenMu.Lock()
	Colors = defaultColorScheme()
	Styles = NewStyleSet(Colors)
	t.Cleanup(func() {
		Colors = NewColorScheme()
		Styles = NewStyleSet(Colors)
		goldenMu.Unlock()
	})

	items := makeTestItems(5)
	var output string
	for _, item := range items {
		output += item.StatusText() + "  " + item.TmuxText() + "\n"
	}
	golden.RequireEqual(t, []byte(output))
}

func TestGolden_Themed_OverlayBorders(t *testing.T) {
	m := goldenModelThemed(t, sizeStandard, withItems(3), withDeleteOverlay("Worktree has uncommitted changes"))
	golden.RequireEqual(t, []byte(m.viewString()))
}
