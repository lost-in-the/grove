package commands

import "testing"

func TestEffectiveTmuxMode(t *testing.T) {
	tests := []struct {
		name      string
		mode      string
		agentMode bool
		noTmux    bool
		peek      bool
		want      string
	}{
		{name: "auto mode unchanged", mode: tmuxModeAuto, want: tmuxModeAuto},
		{name: "manual mode unchanged", mode: "manual", want: "manual"},
		{name: "off mode unchanged", mode: tmuxModeOff, want: tmuxModeOff},
		{name: "agent mode forces off", mode: tmuxModeAuto, agentMode: true, want: tmuxModeOff},
		{name: "no-tmux forces off", mode: tmuxModeAuto, noTmux: true, want: tmuxModeOff},
		{name: "peek forces off", mode: tmuxModeAuto, peek: true, want: tmuxModeOff},
		{name: "peek forces off in manual mode", mode: "manual", peek: true, want: tmuxModeOff},
		{name: "all overrides together", mode: tmuxModeAuto, agentMode: true, noTmux: true, peek: true, want: tmuxModeOff},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := effectiveTmuxMode(tt.mode, tt.agentMode, tt.noTmux, tt.peek)
			if got != tt.want {
				t.Errorf("effectiveTmuxMode(%q, %v, %v, %v) = %q, want %q",
					tt.mode, tt.agentMode, tt.noTmux, tt.peek, got, tt.want)
			}
		})
	}
}
