package mux

import "testing"

func TestResolveBackend(t *testing.T) {
	tests := []struct {
		name string
		pref Backend
		env  Env
		want Backend
	}{
		{
			name: "explicit off wins over everything",
			pref: BackendOff,
			env:  Env{InsideHerdr: true, HerdrAvailable: true, TmuxAvailable: true},
			want: BackendOff,
		},
		{
			name: "explicit tmux honored even inside herdr",
			pref: BackendTmux,
			env:  Env{InsideHerdr: true, HerdrAvailable: true, TmuxAvailable: true},
			want: BackendTmux,
		},
		{
			name: "explicit herdr honored even inside tmux",
			pref: BackendHerdr,
			env:  Env{InsideTmux: true, TmuxAvailable: true, HerdrAvailable: true},
			want: BackendHerdr,
		},
		{
			name: "explicit backend honored when its binary is missing",
			pref: BackendHerdr,
			env:  Env{TmuxAvailable: true},
			want: BackendHerdr,
		},
		{
			name: "auto prefers herdr when running inside a herdr pane",
			pref: BackendAuto,
			env:  Env{InsideHerdr: true, HerdrAvailable: true, TmuxAvailable: true},
			want: BackendHerdr,
		},
		{
			name: "auto prefers tmux when running inside tmux",
			pref: BackendAuto,
			env:  Env{InsideTmux: true, TmuxAvailable: true, HerdrAvailable: true},
			want: BackendTmux,
		},
		{
			name: "inside herdr beats inside tmux when both are set",
			pref: BackendAuto,
			env: Env{
				InsideTmux: true, InsideHerdr: true,
				TmuxAvailable: true, HerdrAvailable: true,
			},
			want: BackendHerdr,
		},
		{
			name: "auto outside both prefers tmux for backward compatibility",
			pref: BackendAuto,
			env:  Env{TmuxAvailable: true, HerdrAvailable: true},
			want: BackendTmux,
		},
		{
			name: "auto falls back to herdr when tmux is missing",
			pref: BackendAuto,
			env:  Env{HerdrAvailable: true},
			want: BackendHerdr,
		},
		{
			name: "auto falls back to tmux when herdr is missing",
			pref: BackendAuto,
			env:  Env{TmuxAvailable: true},
			want: BackendTmux,
		},
		{
			name: "auto resolves to off when neither binary exists",
			pref: BackendAuto,
			env:  Env{},
			want: BackendOff,
		},
		{
			name: "empty preference is treated as auto",
			pref: "",
			env:  Env{TmuxAvailable: true},
			want: BackendTmux,
		},
		{
			name: "unknown preference is treated as auto",
			pref: "screen",
			env:  Env{HerdrAvailable: true},
			want: BackendHerdr,
		},
		{
			name: "inside herdr without the binary still falls back sanely",
			pref: BackendAuto,
			env:  Env{InsideHerdr: true, TmuxAvailable: true},
			want: BackendTmux,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ResolveBackend(tt.pref, tt.env); got != tt.want {
				t.Errorf("ResolveBackend(%q, %+v) = %q, want %q", tt.pref, tt.env, got, tt.want)
			}
		})
	}
}

func TestParseBackend(t *testing.T) {
	tests := []struct {
		in      string
		want    Backend
		wantErr bool
	}{
		{in: "", want: BackendAuto},
		{in: "auto", want: BackendAuto},
		{in: "tmux", want: BackendTmux},
		{in: "herdr", want: BackendHerdr},
		{in: "off", want: BackendOff},
		{in: "AUTO", want: BackendAuto},
		{in: "  tmux  ", want: BackendTmux},
		{in: "zellij", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.in, func(t *testing.T) {
			got, err := ParseBackend(tt.in)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("ParseBackend(%q) expected error, got %q", tt.in, got)
				}
				return
			}
			if err != nil {
				t.Fatalf("ParseBackend(%q) unexpected error: %v", tt.in, err)
			}
			if got != tt.want {
				t.Errorf("ParseBackend(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}
