package config

import (
	"strings"
	"testing"
)

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
	override.Session.OpenIn = OpenInHere

	merged := mergeConfigs(base, override)

	if merged.Session.OpenIn != OpenInHere {
		t.Errorf("Session.OpenIn = %q, want %q", merged.Session.OpenIn, OpenInHere)
	}
	if got := merged.EffectiveOpenIn(); got != OpenInHere {
		t.Errorf("EffectiveOpenIn() = %q, want %q", got, OpenInHere)
	}
}

func TestEffectiveOpenIn(t *testing.T) {
	tests := []struct {
		name   string
		openIn string
		want   string
	}{
		{name: "unset means session", openIn: "", want: OpenInSession},
		{name: "session", openIn: "session", want: OpenInSession},
		{name: "here", openIn: "here", want: OpenInHere},
		{name: "old spelling new means session", openIn: "new", want: OpenInSession},
		{name: "old spelling current means here", openIn: "current", want: OpenInHere},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &Config{}
			cfg.Session.OpenIn = tt.openIn
			if got := cfg.EffectiveOpenIn(); got != tt.want {
				t.Errorf("EffectiveOpenIn() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestOpenInRename(t *testing.T) {
	tests := []struct {
		openIn  string
		wantNew string
		wantOld bool
	}{
		{openIn: "", wantOld: false},
		{openIn: "session", wantOld: false},
		{openIn: "here", wantOld: false},
		{openIn: "new", wantNew: "session", wantOld: true},
		{openIn: "current", wantNew: "here", wantOld: true},
	}
	for _, tt := range tests {
		cfg := &Config{}
		cfg.Session.OpenIn = tt.openIn
		gotNew, gotOld := cfg.OpenInRename()
		if gotNew != tt.wantNew || gotOld != tt.wantOld {
			t.Errorf("OpenInRename() with open_in=%q = (%q, %v), want (%q, %v)", tt.openIn, gotNew, gotOld, tt.wantNew, tt.wantOld)
		}
	}
}

func TestValidateOpenIn(t *testing.T) {
	for _, v := range []string{"", "session", "here", "new", "current"} {
		cfg := LoadDefaults()
		cfg.Session.OpenIn = v
		if err := Validate(cfg); err != nil {
			t.Errorf("Validate(open_in=%q) = %v, want nil", v, err)
		}
	}

	cfg := LoadDefaults()
	cfg.Session.OpenIn = "pane"
	err := Validate(cfg)
	if err == nil {
		t.Fatal("Validate(open_in=\"pane\") = nil, want an error")
	}
	if want := "session.open_in must be one of: session, here"; !strings.Contains(err.Error(), want) {
		t.Errorf("error = %q, want it to contain %q", err, want)
	}
}
