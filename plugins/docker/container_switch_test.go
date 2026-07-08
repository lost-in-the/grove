package docker

import (
	"testing"

	"github.com/lost-in-the/grove/internal/config"
	"github.com/lost-in-the/grove/internal/hooks"
)

func TestResolveContainerSwitch(t *testing.T) {
	tests := []struct {
		name          string
		configValue   string
		isInteractive bool
		want          ContainerSwitchAction
	}{
		{
			name:          "auto returns Auto",
			configValue:   "auto",
			isInteractive: true,
			want:          ContainerSwitchAuto,
		},
		{
			name:          "empty string defaults to Auto",
			configValue:   "",
			isInteractive: true,
			want:          ContainerSwitchAuto,
		},
		{
			name:          "prompt interactive returns Prompt",
			configValue:   "prompt",
			isInteractive: true,
			want:          ContainerSwitchPrompt,
		},
		{
			name:          "prompt non-interactive falls back to Auto",
			configValue:   "prompt",
			isInteractive: false,
			want:          ContainerSwitchAuto,
		},
		{
			name:          "off returns Off",
			configValue:   "off",
			isInteractive: true,
			want:          ContainerSwitchOff,
		},
		{
			name:          "off non-interactive still returns Off",
			configValue:   "off",
			isInteractive: false,
			want:          ContainerSwitchOff,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ResolveContainerSwitch(tt.configValue, tt.isInteractive)
			if got != tt.want {
				t.Errorf("ResolveContainerSwitch(%q, %v) = %d, want %d", tt.configValue, tt.isInteractive, got, tt.want)
			}
		})
	}
}

// confirmSwitchAction combines the auto gate, container_switch resolution, and
// the interactive prompt. Tests run non-interactively, so "prompt" resolves to
// Auto and the cli.Confirm path is never reached here.
func TestConfirmSwitchAction(t *testing.T) {
	ctxWithSwitch := func(value string) *hooks.Context {
		return &hooks.Context{Config: &config.Config{
			Switch: config.SwitchConfig{ContainerSwitch: value},
		}}
	}

	tests := []struct {
		name        string
		autoEnabled bool
		ctx         *hooks.Context
		want        bool
	}{
		{
			name:        "auto gate disabled returns false",
			autoEnabled: false,
			ctx:         ctxWithSwitch("auto"),
			want:        false,
		},
		{
			name:        "container_switch off returns false",
			autoEnabled: true,
			ctx:         ctxWithSwitch("off"),
			want:        false,
		},
		{
			name:        "auto proceeds",
			autoEnabled: true,
			ctx:         ctxWithSwitch("auto"),
			want:        true,
		},
		{
			name:        "nil hook config defaults to auto",
			autoEnabled: true,
			ctx:         &hooks.Context{},
			want:        true,
		},
		{
			name:        "prompt non-interactive falls back to auto",
			autoEnabled: true,
			ctx:         ctxWithSwitch("prompt"),
			want:        true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := confirmSwitchAction(tt.autoEnabled, tt.ctx, promptStartContainers, true)
			if got != tt.want {
				t.Errorf("confirmSwitchAction(%v, ...) = %v, want %v", tt.autoEnabled, got, tt.want)
			}
		})
	}
}

func TestDockerAutoFlagDefaults(t *testing.T) {
	boolPtr := func(v bool) *bool { return &v }

	if got := dockerAutoStart(nil, true); !got {
		t.Error("dockerAutoStart(nil, true) = false, want default true")
	}
	if got := dockerAutoStop(nil, false); got {
		t.Error("dockerAutoStop(nil, false) = true, want default false")
	}

	cfg := &config.Config{}
	cfg.Plugins.Docker.AutoStart = boolPtr(false)
	cfg.Plugins.Docker.AutoStop = boolPtr(true)
	if got := dockerAutoStart(cfg, true); got {
		t.Error("dockerAutoStart with explicit false = true, want false")
	}
	if got := dockerAutoStop(cfg, false); !got {
		t.Error("dockerAutoStop with explicit true = false, want true")
	}
}
