package commands

import (
	"testing"

	"github.com/lost-in-the/grove/internal/tui"
)

func TestSwitchAfterDashboard_UpRunsGroveToWithUp(t *testing.T) {
	name, opts, ok := switchAfterDashboard(tui.Exit{Path: "/src/myapp-feature", Name: "feature", Up: true})

	if !ok {
		t.Fatal("U on the dashboard must run the shared switch flow")
	}
	if name != "feature" {
		t.Errorf("name = %q, want %q", name, "feature")
	}
	if opts != (switchOptions{Up: true}) {
		t.Errorf("opts = %+v, want exactly grove to --up", opts)
	}
}

func TestSwitchAfterDashboard_EnterAndQuitLeaveTheMoveToTheDashboard(t *testing.T) {
	for _, exit := range []tui.Exit{
		{Path: "/src/myapp-feature", Name: "feature"},
		{},
	} {
		if _, _, ok := switchAfterDashboard(exit); ok {
			t.Errorf("switchAfterDashboard(%+v) ran the switch flow; the dashboard already moved you", exit)
		}
	}
}

// Bare `grove` outside a grove project must diagnose (like every
// RequireGroveContext command) rather than silently show help or launch the
// TUI against a non-project directory (#138).
func TestDecideBareAction(t *testing.T) {
	tests := []struct {
		name        string
		isTTY       bool
		tuiDisabled bool
		inProject   bool
		want        bareAction
	}{
		{"non-TTY in project shows help", false, false, true, bareShowHelp},
		{"non-TTY outside project shows help", false, false, false, bareShowHelp},
		{"GROVE_TUI=0 in project shows help", true, true, true, bareShowHelp},
		{"GROVE_TUI=0 outside project shows help", true, true, false, bareShowHelp},
		{"interactive outside project diagnoses", true, false, false, bareDiagnose},
		{"interactive in project launches TUI", true, false, true, bareLaunchTUI},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := decideBareAction(tt.isTTY, tt.tuiDisabled, tt.inProject)
			if got != tt.want {
				t.Errorf("decideBareAction(%v, %v, %v) = %v, want %v",
					tt.isTTY, tt.tuiDisabled, tt.inProject, got, tt.want)
			}
		})
	}
}
