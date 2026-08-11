package config

import "testing"

func TestValidateMuxBackend(t *testing.T) {
	for _, backend := range []string{"", "auto", "tmux", "herdr", "off"} {
		cfg := LoadDefaults()
		cfg.Mux.Backend = backend
		if err := Validate(cfg); err != nil {
			t.Errorf("Validate() with mux.backend=%q error = %v", backend, err)
		}
	}

	cfg := LoadDefaults()
	cfg.Mux.Backend = "zellij"
	if err := Validate(cfg); err == nil {
		t.Error("Validate() accepted an unknown mux.backend")
	}
}

func TestEffectiveMuxBackend(t *testing.T) {
	tests := []struct {
		name     string
		backend  string
		tmuxMode string
		want     string
	}{
		{
			name: "unset defaults to auto",
			want: "auto",
		},
		{
			name:    "explicit backend is passed through",
			backend: "herdr",
			want:    "herdr",
		},
		{
			// tmux.mode=off predates mux.backend and is how existing installs
			// disable session management; it must keep working.
			name:     "legacy tmux.mode=off disables the multiplexer",
			tmuxMode: "off",
			want:     "off",
		},
		{
			name:     "tmux.mode=off wins over an explicit backend",
			backend:  "herdr",
			tmuxMode: "off",
			want:     "off",
		},
		{
			name:     "tmux.mode=auto does not force the tmux backend",
			backend:  "herdr",
			tmuxMode: "auto",
			want:     "herdr",
		},
		{
			name:     "tmux.mode=manual leaves the backend alone",
			backend:  "tmux",
			tmuxMode: "manual",
			want:     "tmux",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := LoadDefaults()
			cfg.Mux.Backend = tt.backend
			cfg.Tmux.Mode = tt.tmuxMode

			if got := cfg.EffectiveMuxBackend(); got != tt.want {
				t.Errorf("EffectiveMuxBackend() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestMergeMuxConfig(t *testing.T) {
	base := LoadDefaults()
	base.Mux.Backend = "tmux"

	override := &Config{}
	merged := mergeConfigs(base, override)
	if merged.Mux.Backend != "tmux" {
		t.Errorf("empty override changed backend to %q", merged.Mux.Backend)
	}

	override = &Config{Mux: MuxConfig{Backend: "herdr"}}
	merged = mergeConfigs(base, override)
	if merged.Mux.Backend != "herdr" {
		t.Errorf("override backend = %q, want herdr", merged.Mux.Backend)
	}
}

func TestMergeCarriesSessionOpenIn(t *testing.T) {
	// A field the merge forgets is a field the config file cannot set: the
	// value parses, survives into the override, and is then dropped on the
	// floor. Only an end-to-end read catches that, so pin it here.
	base := LoadDefaults()
	override := &Config{}
	override.Session.OpenIn = OpenInCurrent

	merged := mergeConfigs(base, override)

	if merged.Session.OpenIn != OpenInCurrent {
		t.Errorf("Session.OpenIn = %q, want %q", merged.Session.OpenIn, OpenInCurrent)
	}
	if got := merged.EffectiveOpenIn(); got != OpenInCurrent {
		t.Errorf("EffectiveOpenIn() = %q, want %q", got, OpenInCurrent)
	}
}

func TestEffectiveOpenInDefaultsToNew(t *testing.T) {
	if got := (&Config{}).EffectiveOpenIn(); got != OpenInNew {
		t.Errorf("EffectiveOpenIn() = %q, want %q for an unset value", got, OpenInNew)
	}
}
