package commands

import (
	"testing"

	"github.com/lost-in-the/grove/internal/config"
)

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

func TestShellSingleQuote(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{name: "plain path", in: "/Users/dev/proj-fix", want: "'/Users/dev/proj-fix'"},
		{name: "embedded single quote", in: "/Users/dev/Dev's Projects/app-fix", want: `'/Users/dev/Dev'\''s Projects/app-fix'`},
		{name: "empty string", in: "", want: "''"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := shellSingleQuote(tt.in); got != tt.want {
				t.Errorf("shellSingleQuote(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestResolveTmuxModeHonorsOpenIn(t *testing.T) {
	tests := []struct {
		name     string
		openIn   string
		tmuxMode string
		want     string
	}{
		{name: "unset keeps the configured mode", openIn: "", tmuxMode: tmuxModeAuto, want: tmuxModeAuto},
		{name: "new keeps the configured mode", openIn: config.OpenInNew, tmuxMode: tmuxModeAuto, want: tmuxModeAuto},
		{name: "new keeps manual", openIn: config.OpenInNew, tmuxMode: "manual", want: "manual"},
		// "current" means the worktree lands in the shell already running, so
		// there is no session to create, switch to, or ready — whatever the
		// configured mode says.
		{name: "current suppresses auto", openIn: config.OpenInCurrent, tmuxMode: tmuxModeAuto, want: tmuxModeOff},
		{name: "current suppresses manual", openIn: config.OpenInCurrent, tmuxMode: "manual", want: tmuxModeOff},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &config.Config{}
			cfg.Session.OpenIn = tt.openIn
			cfg.Tmux.Mode = tt.tmuxMode

			if got := resolveTmuxMode(cfg, false, false); got != tt.want {
				t.Errorf("resolveTmuxMode(open_in=%q, mode=%q) = %q, want %q",
					tt.openIn, tt.tmuxMode, got, tt.want)
			}
		})
	}
}
