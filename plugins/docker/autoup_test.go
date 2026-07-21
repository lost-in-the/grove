package docker

import (
	"testing"

	"github.com/lost-in-the/grove/internal/config"
)

func TestShouldAutoUp(t *testing.T) {
	boolPtr := func(v bool) *bool { return &v }

	agentStackCfg := func(autoUp *bool) *config.Config {
		cfg := &config.Config{}
		cfg.Plugins.Docker.Mode = "external"
		cfg.Plugins.Docker.AutoUp = autoUp
		cfg.Plugins.Docker.External = &config.ExternalComposeConfig{
			Agent: &config.AgentStackConfig{Enabled: boolPtr(true)},
		}
		return cfg
	}

	tests := []struct {
		name string
		cfg  *config.Config
		want bool
	}{
		{name: "nil config", cfg: nil, want: false},
		{name: "unset defaults off", cfg: &config.Config{}, want: false},
		{
			name: "explicit true",
			cfg: func() *config.Config {
				cfg := &config.Config{}
				cfg.Plugins.Docker.AutoUp = boolPtr(true)
				return cfg
			}(),
			want: true,
		},
		{
			name: "explicit false",
			cfg: func() *config.Config {
				cfg := &config.Config{}
				cfg.Plugins.Docker.AutoUp = boolPtr(false)
				return cfg
			}(),
			want: false,
		},
		{
			// Agent stacks used to flip auto-up on implicitly; the knob is
			// now explicit-only so `grove new` and the dashboard behave the
			// same way without hidden defaults.
			name: "agent stacks enabled but auto_up unset stays off",
			cfg:  agentStackCfg(nil),
			want: false,
		},
		{
			name: "agent stacks enabled with explicit auto_up",
			cfg:  agentStackCfg(boolPtr(true)),
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ShouldAutoUp(tt.cfg); got != tt.want {
				t.Errorf("ShouldAutoUp() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestAutoUp_DisabledPlugin(t *testing.T) {
	boolPtr := func(v bool) *bool { return &v }
	cfg := &config.Config{}
	cfg.Plugins.Docker.Enabled = boolPtr(false)
	cfg.Plugins.Docker.AutoUp = boolPtr(true)

	started, err := AutoUp(cfg, t.TempDir())
	if err != nil {
		t.Fatalf("AutoUp() with disabled plugin should be a no-op, got error: %v", err)
	}
	if started {
		t.Error("AutoUp() with disabled plugin should not report started")
	}
}
