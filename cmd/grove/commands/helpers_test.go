package commands

import (
	"testing"

	"github.com/lost-in-the/grove/internal/cli"
	"github.com/lost-in-the/grove/internal/config"
	"github.com/lost-in-the/grove/internal/mux"
)

// preparingMux is a backend whose attach command cannot name its target
// (herdr): the hint only works once PrepareAttach has focused it.
type preparingMux struct {
	*mux.OffBackend
	prepared []mux.Target
}

func (p *preparingMux) AttachHint(mux.Target) string { return "herdr --session other" }
func (p *preparingMux) PrepareAttach(t mux.Target) error {
	p.prepared = append(p.prepared, t)
	return nil
}

// Under shell integration grove prints an attach hint instead of attaching.
// Printing it without focusing the target first sent the user to whatever
// workspace herdr last had focused.
func TestAttachToSessionPreparesTheTargetBeforeHinting(t *testing.T) {
	m := &preparingMux{OffBackend: mux.NewOff()}
	target := mux.Target{Name: "grove-testing", Path: "/repos/grove-testing"}

	if err := attachToSession(m, target, nil, true, cli.NewStderr()); err != nil {
		t.Fatalf("attachToSession() error = %v", err)
	}
	if len(m.prepared) != 1 || m.prepared[0].Path != target.Path {
		t.Errorf("PrepareAttach calls = %v, want exactly the target", m.prepared)
	}
}

func TestWantsStackUp(t *testing.T) {
	yes, no := true, false
	withAutoUp := func(v *bool) *config.Config {
		cfg := &config.Config{}
		cfg.Plugins.Docker.AutoUp = v
		return cfg
	}
	tests := []struct {
		name     string
		cfg      *config.Config
		up       bool
		noDocker bool
		want     bool
	}{
		{name: "nothing set", cfg: withAutoUp(nil), want: false},
		{name: "auto_up", cfg: withAutoUp(&yes), want: true},
		{name: "auto_up false", cfg: withAutoUp(&no), want: false},
		{name: "--up", cfg: withAutoUp(nil), up: true, want: true},
		{name: "--up beats auto_up false", cfg: withAutoUp(&no), up: true, want: true},
		{name: "--no-docker beats auto_up", cfg: withAutoUp(&yes), noDocker: true, want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := wantsStackUp(tt.cfg, tt.up, tt.noDocker); got != tt.want {
				t.Errorf("wantsStackUp() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestNewCmdUpFlag(t *testing.T) {
	if newCmd.Flags().Lookup("up") == nil {
		t.Fatal("grove new has no --up flag")
	}
}
