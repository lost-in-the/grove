package docker

import (
	"testing"

	"github.com/lost-in-the/grove/internal/config"
)

func TestLocalStrategy_GetAutoStart_ExplicitValues(t *testing.T) {
	trueVal := true
	falseVal := false

	tests := []struct {
		name string
		cfg  *config.Config
		want bool
	}{
		{
			name: "explicit true",
			cfg: &config.Config{
				Plugins: config.PluginsConfig{
					Docker: config.DockerPluginConfig{
						AutoStart: &trueVal,
					},
				},
			},
			want: true,
		},
		{
			name: "explicit false",
			cfg: &config.Config{
				Plugins: config.PluginsConfig{
					Docker: config.DockerPluginConfig{
						AutoStart: &falseVal,
					},
				},
			},
			want: false,
		},
		{
			name: "nil AutoStart defaults to true",
			cfg:  &config.Config{},
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &localStrategy{cfg: tt.cfg}
			if got := s.getAutoStart(); got != tt.want {
				t.Errorf("getAutoStart() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestLocalStrategy_GetAutoStop_ExplicitValues(t *testing.T) {
	trueVal := true
	falseVal := false

	tests := []struct {
		name string
		cfg  *config.Config
		want bool
	}{
		{
			name: "explicit true",
			cfg: &config.Config{
				Plugins: config.PluginsConfig{
					Docker: config.DockerPluginConfig{
						AutoStop: &trueVal,
					},
				},
			},
			want: true,
		},
		{
			name: "explicit false",
			cfg: &config.Config{
				Plugins: config.PluginsConfig{
					Docker: config.DockerPluginConfig{
						AutoStop: &falseVal,
					},
				},
			},
			want: false,
		},
		{
			// AutoStop nil defaults to false (opposite of AutoStart which defaults to true)
			name: "nil AutoStop defaults to false",
			cfg:  &config.Config{},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &localStrategy{cfg: tt.cfg}
			if got := s.getAutoStop(); got != tt.want {
				t.Errorf("getAutoStop() = %v, want %v", got, tt.want)
			}
		})
	}
}
