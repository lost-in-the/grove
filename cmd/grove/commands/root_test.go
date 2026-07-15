package commands

import "testing"

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
