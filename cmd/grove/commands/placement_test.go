package commands

import (
	"slices"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"github.com/lost-in-the/grove/internal/config"
)

func TestHereFlagOnEveryCommandThatOpensASession(t *testing.T) {
	for _, cmd := range []*cobra.Command{toCmd, lastCmd, newCmd, forkCmd, openCmd, fetchCmd} {
		f := cmd.Flags().Lookup("here")
		if f == nil {
			t.Errorf("%s has no --here flag", cmd.Name())
			continue
		}
		if f.Hidden || f.Deprecated != "" {
			t.Errorf("%s --here must be a visible, current flag", cmd.Name())
		}
	}
}

func TestNoTmuxIsAHiddenAliasOfHere(t *testing.T) {
	for _, cmd := range []*cobra.Command{toCmd, newCmd} {
		f := cmd.Flags().Lookup("no-tmux")
		if f == nil {
			t.Fatalf("%s lost its --no-tmux flag; old scripts would break", cmd.Name())
		}
		if !f.Hidden {
			t.Errorf("%s --no-tmux should be hidden from help", cmd.Name())
		}
		if !strings.Contains(f.Deprecated, "--here") {
			t.Errorf("%s --no-tmux deprecation = %q, want it to name --here", cmd.Name(), f.Deprecated)
		}
	}
}

func TestToAndLastShareTheSwitchFlags(t *testing.T) {
	for _, cmd := range []*cobra.Command{toCmd, lastCmd} {
		for _, name := range []string{"json", "up", "here", "no-hooks", "peek"} {
			if cmd.Flags().Lookup(name) == nil {
				t.Errorf("%s has no --%s flag", cmd.Name(), name)
			}
		}
	}
}

func TestUpConflictsWithEveryFlagThatSkipsHooks(t *testing.T) {
	for _, cmd := range []*cobra.Command{toCmd, lastCmd} {
		groups := cmd.Flags().Lookup("up").Annotations["cobra_annotation_mutually_exclusive"]
		for _, want := range []string{"up peek", "up no-hooks"} {
			if !slices.Contains(groups, want) {
				t.Errorf("%s: --up exclusive groups = %v, want %q among them", cmd.Name(), groups, want)
			}
		}
	}
}

func TestSwitchFlagsOptions(t *testing.T) {
	tests := []struct {
		name        string
		flags       switchFlags
		wantHere    bool
		wantNoHooks bool
	}{
		{name: "no flags", flags: switchFlags{}},
		{name: "--here", flags: switchFlags{here: true}, wantHere: true},
		{name: "--no-hooks", flags: switchFlags{noHooks: true}, wantNoHooks: true},
		{name: "--peek is --here --no-hooks", flags: switchFlags{peek: true}, wantHere: true, wantNoHooks: true},
		{name: "--here --no-hooks", flags: switchFlags{here: true, noHooks: true}, wantHere: true, wantNoHooks: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.flags.options()
			if got.Here != tt.wantHere || got.NoHooks != tt.wantNoHooks {
				t.Errorf("options() = {Here: %v, NoHooks: %v}, want {Here: %v, NoHooks: %v}",
					got.Here, got.NoHooks, tt.wantHere, tt.wantNoHooks)
			}
		})
	}

	up := switchFlags{json: true, up: true}.options()
	if !up.JSON || !up.Up {
		t.Errorf("options() dropped --json or --up: %+v", up)
	}
}

func TestLandsHere(t *testing.T) {
	tests := []struct {
		name   string
		openIn string
		here   bool
		want   bool
	}{
		{name: "default", want: false},
		{name: "open_in session", openIn: "session", want: false},
		{name: "open_in here", openIn: "here", want: true},
		{name: "old spelling current", openIn: "current", want: true},
		{name: "--here beats open_in session", openIn: "session", here: true, want: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &config.Config{}
			cfg.Session.OpenIn = tt.openIn
			if got := landsHere(cfg, tt.here); got != tt.want {
				t.Errorf("landsHere(open_in=%q, here=%v) = %v, want %v", tt.openIn, tt.here, got, tt.want)
			}
		})
	}
}

// Placement help is the same under tmux and herdr, so it must not name only one.
func TestPlacementHelpIsBackendNeutral(t *testing.T) {
	for _, cmd := range []*cobra.Command{toCmd, lastCmd, newCmd, forkCmd, fetchCmd} {
		// The one allowed mention defines "session" for both backends.
		help := strings.Join(strings.Fields(cmd.Short+" "+cmd.Long), " ")
		help = strings.ReplaceAll(help, "a tmux session or a herdr workspace", "")
		if strings.Contains(strings.ToLower(help), "tmux") {
			t.Errorf("%s help names tmux:\n%s\n%s", cmd.Name(), cmd.Short, cmd.Long)
		}
		cmd.Flags().VisitAll(func(f *pflag.Flag) {
			if f.Hidden {
				return
			}
			if strings.Contains(strings.ToLower(f.Usage), "tmux") {
				t.Errorf("%s --%s usage names tmux: %q", cmd.Name(), f.Name, f.Usage)
			}
		})
	}
}

func TestToHelpUsesThePlacementWords(t *testing.T) {
	for _, want := range []string{"session", "--here", "--no-hooks", "--peek", "--here --no-hooks", "open_in"} {
		if !strings.Contains(toCmd.Long, want) {
			t.Errorf("grove to help should mention %q", want)
		}
	}
}

func TestCheckOpenInSpelling(t *testing.T) {
	for _, current := range []string{"", "session", "here"} {
		cfg := &config.Config{}
		cfg.Session.OpenIn = current
		if _, err := checkOpenInSpelling(cfg); err != nil {
			t.Errorf("checkOpenInSpelling(open_in=%q) = %v, want no finding", current, err)
		}
	}

	for old, renamed := range map[string]string{"new": "session", "current": "here"} {
		cfg := &config.Config{}
		cfg.Session.OpenIn = old
		_, err := checkOpenInSpelling(cfg)
		if err == nil {
			t.Fatalf("checkOpenInSpelling(open_in=%q) found nothing, want a rename notice", old)
		}
		want := `open_in = "` + renamed + `"`
		if !strings.Contains(err.Error(), want) {
			t.Errorf("notice = %q, want it to contain %q", err, want)
		}
	}
}

func TestConfigShowsPlacement(t *testing.T) {
	tests := []struct {
		openIn string
		want   string
	}{
		{openIn: "", want: "session (default)"},
		{openIn: "here", want: "here"},
		{openIn: "current", want: `current (old spelling of "here")`},
	}
	for _, tt := range tests {
		cfg := config.LoadDefaults()
		cfg.Session.OpenIn = tt.openIn
		if out := captureConfigOutput(t, cfg); !strings.Contains(out, tt.want) {
			t.Errorf("open_in=%q: config output should contain %q:\n%s", tt.openIn, tt.want, out)
		}
	}
}
