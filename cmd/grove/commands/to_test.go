package commands

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/lost-in-the/grove/internal/config"
	"github.com/lost-in-the/grove/internal/worktree"
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

func TestShouldRepointStack(t *testing.T) {
	const target = "/work/myapp-feature"
	tests := []struct {
		name       string
		external   bool
		configured string
		ok         bool
		peek       bool
		want       bool
	}{
		{name: "external, configured for another worktree", external: true, configured: "/work/myapp-main", ok: true, want: true},
		{name: "external, configured for the target", external: true, configured: target, ok: true, want: false},
		{name: "external, configured path has trailing slash", external: true, configured: target + "/", ok: true, want: false},
		{name: "external, env var unset", external: true, ok: false, want: false},
		{name: "external, other worktree but peek", external: true, configured: "/work/myapp-main", ok: true, peek: true, want: false},
		{name: "not external, would differ", external: false, configured: "/work/myapp-main", ok: true, want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := shouldRepointStack(tt.external, tt.configured, tt.ok, target, tt.peek); got != tt.want {
				t.Errorf("shouldRepointStack() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestRepointSucceeded(t *testing.T) {
	const target = "/work/myapp-feature"
	tests := []struct {
		name       string
		configured string
		ok         bool
		want       bool
	}{
		{name: "env file now names the target", configured: target, ok: true, want: true},
		{name: "trailing slash", configured: target + "/", ok: true, want: true},
		{name: "env file still names another worktree", configured: "/work/myapp-main", ok: true, want: false},
		{name: "env var unset after hooks", ok: false, want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := repointSucceeded(tt.configured, tt.ok, target); got != tt.want {
				t.Errorf("repointSucceeded() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestUsesSharedExternalStack_FollowsConfigAgentMode(t *testing.T) {
	cfg := &config.Config{}
	cfg.Plugins.Docker.Mode = "external"
	if !usesSharedExternalStack(cfg) {
		t.Error("external mode should use the shared stack")
	}
	cfg.AgentMode = true
	if usesSharedExternalStack(cfg) {
		t.Error("cfg.AgentMode should select isolated stacks")
	}
}

func TestSwitchConfigForStack(t *testing.T) {
	tests := []struct {
		name    string
		setting string
		up      bool
		want    string
	}{
		{name: "off stays off", setting: "off", want: "off"},
		{name: "unset stays unset", setting: "", want: ""},
		{name: "auto stays auto", setting: "auto", want: "auto"},
		{name: "prompt stays prompt", setting: "prompt", want: "prompt"},
		{name: "--up forces auto over off", setting: "off", up: true, want: "auto"},
		{name: "--up forces auto over prompt", setting: "prompt", up: true, want: "auto"},
		{name: "--up forces auto over unset", setting: "", up: true, want: "auto"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &config.Config{Switch: config.SwitchConfig{ContainerSwitch: tt.setting}}
			got := switchConfigForStack(cfg, tt.up)
			if got.Switch.ContainerSwitch != tt.want {
				t.Errorf("ContainerSwitch = %q, want %q", got.Switch.ContainerSwitch, tt.want)
			}
			if cfg.Switch.ContainerSwitch != tt.setting {
				t.Errorf("the loaded config was changed to %q", cfg.Switch.ContainerSwitch)
			}
		})
	}
}

func TestStackFollowsSwitch(t *testing.T) {
	for setting, want := range map[string]bool{"": false, "off": false, "auto": true} {
		cfg := &config.Config{Switch: config.SwitchConfig{ContainerSwitch: setting}}
		if got := stackFollowsSwitch(cfg); got != want {
			t.Errorf("stackFollowsSwitch(%q) = %v, want %v", setting, got, want)
		}
	}
}

func TestToAndLastHaveUpFlag(t *testing.T) {
	for _, cmd := range []*cobra.Command{toCmd, lastCmd} {
		f := cmd.Flags().Lookup("up")
		if f == nil {
			t.Fatalf("%s has no --up flag", cmd.Name())
		}
		if f.Shorthand != "u" {
			t.Errorf("%s --up shorthand = %q, want u", cmd.Name(), f.Shorthand)
		}
	}
}

func externalStackConfig(t *testing.T, envValue string) *config.Config {
	t.Helper()
	dir := t.TempDir()
	if envValue != "" {
		if err := os.WriteFile(filepath.Join(dir, ".env.local"), []byte("APP_DIR="+envValue+"\n"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	cfg := &config.Config{}
	cfg.Plugins.Docker.Mode = "external"
	cfg.Plugins.Docker.External = &config.ExternalComposeConfig{Path: dir, EnvVar: "APP_DIR", EnvFile: ".env.local"}
	return cfg
}

func TestStackStatusFor(t *testing.T) {
	name := func(p string) string { return strings.TrimPrefix(filepath.Base(p), "myapp-") }
	const here = "/work/myapp-feature"

	tests := []struct {
		name     string
		cfg      func(t *testing.T) *config.Config
		tracked  bool
		on       string
		isHere   bool
		wantLine string
	}{
		{
			name:     "elsewhere",
			cfg:      func(t *testing.T) *config.Config { return externalStackConfig(t, "/work/myapp-main") },
			tracked:  true,
			on:       "main",
			wantLine: "Stack: on main (not here). Run `grove up` to bring it.",
		},
		{
			name:     "here",
			cfg:      func(t *testing.T) *config.Config { return externalStackConfig(t, here) },
			tracked:  true,
			on:       "feature",
			isHere:   true,
			wantLine: "Stack: on feature (here).",
		},
		{
			name:     "unset",
			cfg:      func(t *testing.T) *config.Config { return externalStackConfig(t, "") },
			tracked:  true,
			wantLine: "Stack: not on any worktree. Run `grove up` to bring it.",
		},
		{
			name: "local mode has no shared stack",
			cfg:  func(t *testing.T) *config.Config { return &config.Config{} },
		},
		{
			name: "agent mode has isolated stacks",
			cfg: func(t *testing.T) *config.Config {
				cfg := externalStackConfig(t, "/work/myapp-main")
				cfg.AgentMode = true
				return cfg
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := stackStatusFor(tt.cfg(t), here, name)
			if got.Tracked != tt.tracked || got.On != tt.on || got.Here != tt.isHere {
				t.Errorf("stackStatusFor() = %+v, want tracked=%v on=%q here=%v", got, tt.tracked, tt.on, tt.isHere)
			}
			if line := got.Line(); line != tt.wantLine {
				t.Errorf("Line() = %q, want %q", line, tt.wantLine)
			}
		})
	}
}

func TestSwitchResult_StackFields(t *testing.T) {
	tree := &worktree.Worktree{Path: "/work/myapp-feature", Branch: "feature"}
	marshal := func(s stackStatus) map[string]any {
		raw, err := json.Marshal(switchResult(tree, s))
		if err != nil {
			t.Fatal(err)
		}
		var m map[string]any
		_ = json.Unmarshal(raw, &m)
		return m
	}

	m := marshal(stackStatus{Tracked: true, On: "main"})
	if m["stack_on"] != "main" || m["stack_here"] != false {
		t.Errorf("elsewhere: got %v", m)
	}
	m = marshal(stackStatus{Tracked: true, On: "feature", Here: true})
	if m["stack_on"] != "feature" || m["stack_here"] != true {
		t.Errorf("here: got %v", m)
	}
	m = marshal(stackStatus{Tracked: true})
	if _, ok := m["stack_on"]; ok || m["stack_here"] != false {
		t.Errorf("unset: got %v", m)
	}
	m = marshal(stackStatus{})
	if _, ok := m["stack_here"]; ok {
		t.Errorf("not tracked should omit the stack fields: got %v", m)
	}
}
