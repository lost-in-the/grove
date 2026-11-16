package commands

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/lost-in-the/grove/internal/config"
	"github.com/lost-in-the/grove/internal/hooks"
	"github.com/lost-in-the/grove/internal/worktree"
)

func TestEffectiveTmuxMode(t *testing.T) {
	tests := []struct {
		name      string
		mode      string
		agentMode bool
		here      bool
		want      string
	}{
		{name: "auto mode unchanged", mode: tmuxModeAuto, want: tmuxModeAuto},
		{name: "manual mode unchanged", mode: "manual", want: "manual"},
		{name: "off mode unchanged", mode: tmuxModeOff, want: tmuxModeOff},
		{name: "agent mode forces off", mode: tmuxModeAuto, agentMode: true, want: tmuxModeOff},
		{name: "here forces off", mode: tmuxModeAuto, here: true, want: tmuxModeOff},
		{name: "here forces off in manual mode", mode: "manual", here: true, want: tmuxModeOff},
		{name: "all overrides together", mode: tmuxModeAuto, agentMode: true, here: true, want: tmuxModeOff},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := effectiveTmuxMode(tt.mode, tt.agentMode, tt.here)
			if got != tt.want {
				t.Errorf("effectiveTmuxMode(%q, %v, %v) = %q, want %q",
					tt.mode, tt.agentMode, tt.here, got, tt.want)
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
		here     bool
		want     string
	}{
		{name: "unset keeps the configured mode", openIn: "", tmuxMode: tmuxModeAuto, want: tmuxModeAuto},
		{name: "session keeps the configured mode", openIn: config.OpenInSession, tmuxMode: tmuxModeAuto, want: tmuxModeAuto},
		{name: "session keeps manual", openIn: config.OpenInSession, tmuxMode: "manual", want: "manual"},
		{name: "old spelling new keeps the configured mode", openIn: "new", tmuxMode: tmuxModeAuto, want: tmuxModeAuto},
		// "here" means you stay in the shell already running, so there is no
		// session to create, switch to, or ready, whatever the mode says.
		{name: "here suppresses auto", openIn: config.OpenInHere, tmuxMode: tmuxModeAuto, want: tmuxModeOff},
		{name: "here suppresses manual", openIn: config.OpenInHere, tmuxMode: "manual", want: tmuxModeOff},
		{name: "old spelling current suppresses auto", openIn: "current", tmuxMode: tmuxModeAuto, want: tmuxModeOff},
		{name: "--here suppresses auto for one run", openIn: config.OpenInSession, tmuxMode: tmuxModeAuto, here: true, want: tmuxModeOff},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &config.Config{}
			cfg.Session.OpenIn = tt.openIn
			cfg.Tmux.Mode = tt.tmuxMode

			if got := resolveTmuxMode(cfg, tt.here); got != tt.want {
				t.Errorf("resolveTmuxMode(open_in=%q, mode=%q, here=%v) = %q, want %q",
					tt.openIn, tt.tmuxMode, tt.here, got, tt.want)
			}
		})
	}
}

func TestShouldRepointStack(t *testing.T) {
	const target = "/work/myapp-feature"
	tests := []struct {
		name       string
		external   bool
		follows    bool
		configured string
		ok         bool
		noHooks    bool
		want       bool
	}{
		{name: "stack on another worktree", external: true, follows: true, configured: "/work/myapp-main", ok: true, want: true},
		{name: "stack on the target", external: true, follows: true, configured: target, ok: true, want: false},
		{name: "trailing slash on the target", external: true, follows: true, configured: target + "/", ok: true, want: false},
		{name: "env var unset", external: true, follows: true, ok: false, want: true},
		{name: "other worktree but --no-hooks", external: true, follows: true, configured: "/work/myapp-main", ok: true, noHooks: true, want: false},
		{name: "other worktree but the stack does not follow a move", external: true, follows: false, configured: "/work/myapp-main", ok: true, want: false},
		{name: "env var unset but the stack does not follow a move", external: true, follows: false, ok: false, want: false},
		{name: "not external", external: false, follows: true, configured: "/work/myapp-main", ok: true, want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := shouldRepointStack(tt.external, tt.follows, tt.configured, tt.ok, target, tt.noHooks); got != tt.want {
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

// A self-switch (`grove to <current>`) fires the Docker switch hooks only when
// the stack has something to do. --up always has: after `grove down` the env
// file still names this worktree, and the stack must start anyway.
func TestSelfSwitchStackEvents(t *testing.T) {
	const target = "/work/myapp-feature"
	repoint := []string{hooks.EventPreSwitch, hooks.EventPostSwitch}
	startOnly := []string{hooks.EventPostSwitch}
	tests := []struct {
		name       string
		external   bool
		follows    bool
		up         bool
		configured string
		ok         bool
		noHooks    bool
		want       []string
	}{
		{name: "--up with the stack on another worktree repoints", external: true, follows: true, up: true, configured: "/work/myapp-main", ok: true, want: repoint},
		{name: "--up with the stack already here still starts it", external: true, follows: true, up: true, configured: target, ok: true, want: startOnly},
		{name: "--up in local mode starts the stack", follows: true, up: true, want: startOnly},
		{name: "no --up with the stack already here does nothing", external: true, configured: target, ok: true, want: nil},
		{name: "no --up in local mode does nothing", want: nil},
		{name: "container_switch auto with the stack already here does nothing", external: true, follows: true, configured: target, ok: true, want: nil},
		{name: "container_switch auto with the stack elsewhere repoints", external: true, follows: true, configured: "/work/myapp-main", ok: true, want: repoint},
		{name: "--no-hooks never fires hooks", external: true, follows: true, up: true, configured: target, ok: true, noHooks: true, want: nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := selfSwitchStackEvents(tt.external, tt.follows, tt.up, tt.configured, tt.ok, target, tt.noHooks)
			if !slices.Equal(got, tt.want) {
				t.Errorf("selfSwitchStackEvents() = %v, want %v", got, tt.want)
			}
		})
	}
}
