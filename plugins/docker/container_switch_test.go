package docker

import "testing"

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
