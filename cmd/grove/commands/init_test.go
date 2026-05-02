package commands

import (
	"strings"
	"testing"

	"github.com/lost-in-the/grove/internal/config"
	"github.com/lost-in-the/grove/internal/detect"
)

func TestGenerateHooksToml_HostCommand(t *testing.T) {
	p := &detect.ProjectProfile{
		Type:     "rails",
		Commands: []string{"bundle install --quiet"},
	}
	got := generateHooksToml(p)
	if !strings.Contains(got, `type = "command"`) {
		t.Error("expected command-type hook for plain Rails")
	}
	if strings.Contains(got, `type = "docker:compose"`) {
		t.Error("expected NO compose-type hook when Docker not detected")
	}
}

func TestGenerateHooksToml_ContainerCommandRendersAsCompose(t *testing.T) {
	p := &detect.ProjectProfile{
		Type:                  "rails",
		HasDocker:             true,
		DockerService:         "web",
		DockerServiceInferred: true,
		ContainerCommands: []detect.ContainerCommand{
			{Service: "web", Command: "bundle install --quiet"},
		},
	}
	got := generateHooksToml(p)

	if !strings.Contains(got, `type = "docker:compose"`) {
		t.Errorf("expected compose-type hook, got:\n%s", got)
	}
	if !strings.Contains(got, `service = "web"`) {
		t.Error("expected service=web in rendered hook")
	}
	if !strings.Contains(got, `command = "bundle install --quiet"`) {
		t.Error("expected bundle install command in rendered hook")
	}
	if !strings.Contains(got, `mode = "run"`) {
		t.Error("expected mode=run default")
	}
	if !strings.Contains(got, "service name was inferred") {
		t.Error("expected inferred-service comment when DockerServiceInferred=true")
	}
}

func TestGenerateHooksToml_NoInferredCommentWhenSingleService(t *testing.T) {
	p := &detect.ProjectProfile{
		Type:                  "rails",
		HasDocker:             true,
		DockerService:         "app",
		DockerServiceInferred: false,
		ContainerCommands: []detect.ContainerCommand{
			{Service: "app", Command: "bundle install"},
		},
	}
	got := generateHooksToml(p)
	if strings.Contains(got, "service name was inferred") {
		t.Errorf("should not emit inferred comment when single-service compose. Got:\n%s", got)
	}
}

func TestGenerateHooksToml_MixedHostAndContainer(t *testing.T) {
	p := &detect.ProjectProfile{
		Type:      "mixed",
		HasDocker: true,
		Commands:  []string{"echo host"},
		ContainerCommands: []detect.ContainerCommand{
			{Service: "app", Command: "echo container"},
		},
	}
	got := generateHooksToml(p)
	if !strings.Contains(got, `type = "command"`) {
		t.Error("expected host command type")
	}
	if !strings.Contains(got, `type = "docker:compose"`) {
		t.Error("expected compose type")
	}
}

// TestResolveInitMode_FlagPrecedence covers the non-interactive precedence
// rules of resolveInitMode. The interactive prompt path is not exercised
// because cli.IsInteractive reads os.Stdin.Fd() directly; in `go test`
// stdin is not a TTY, so IsInteractive() is false and the prompt branch
// is unreachable from a test process. That branch is covered manually.
func TestResolveInitMode_FlagPrecedence(t *testing.T) {
	// Save and restore package-level flag globals so cases don't leak.
	origAuto, origWalkthrough, origYes := initAuto, initWalkthrough, initYes
	t.Cleanup(func() {
		initAuto, initWalkthrough, initYes = origAuto, origWalkthrough, origYes
	})

	cases := []struct {
		name            string
		auto            bool
		walkthrough     bool
		yes             bool
		wantMode        string
		wantSkipConfirm bool
	}{
		{
			name:            "walkthrough flag wins over auto and yes",
			walkthrough:     true,
			auto:            true,
			yes:             true,
			wantMode:        initModeWalkthrough,
			wantSkipConfirm: true,
		},
		{
			name:            "walkthrough alone keeps confirm",
			walkthrough:     true,
			wantMode:        initModeWalkthrough,
			wantSkipConfirm: false,
		},
		{
			name:            "auto with yes skips confirm",
			auto:            true,
			yes:             true,
			wantMode:        initModeAuto,
			wantSkipConfirm: true,
		},
		{
			name:            "auto alone keeps confirm",
			auto:            true,
			wantMode:        initModeAuto,
			wantSkipConfirm: false,
		},
		{
			name:            "yes alone implies auto and skips confirm",
			yes:             true,
			wantMode:        initModeAuto,
			wantSkipConfirm: true,
		},
		{
			// In `go test`, stdin is not a TTY, so this falls through to
			// the non-TTY default rather than the interactive prompt.
			name:            "no flags in non-TTY defaults to auto+skipconfirm",
			wantMode:        initModeAuto,
			wantSkipConfirm: true,
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			initAuto, initWalkthrough, initYes = c.auto, c.walkthrough, c.yes
			got := resolveInitMode()
			if got.Mode != c.wantMode {
				t.Errorf("Mode = %q, want %q", got.Mode, c.wantMode)
			}
			if got.SkipConfirm != c.wantSkipConfirm {
				t.Errorf("SkipConfirm = %v, want %v", got.SkipConfirm, c.wantSkipConfirm)
			}
		})
	}
}

func TestFilterProfileForExternalDocker_StripsContainerCommands(t *testing.T) {
	// When vendor/bundle is symlinked from external compose, the matching
	// container `bundle install` should also be skipped.
	p := &detect.ProjectProfile{
		HasDocker: true,
		ContainerCommands: []detect.ContainerCommand{
			{Service: "app", Command: "bundle install --quiet"},
			{Service: "app", Command: "rails db:setup"},
		},
		Commands: []string{"npm install"},
		Symlinks: []string{"vendor/bundle"},
	}
	ext := &config.ExternalComposeConfig{
		SymlinkDirs: []string{"vendor/bundle", "node_modules"},
	}
	filterProfileForExternalDocker(p, ext)

	for _, cc := range p.ContainerCommands {
		if strings.Contains(cc.Command, "bundle install") {
			t.Errorf("expected bundle install removed from container commands, got %v", p.ContainerCommands)
		}
	}
	if len(p.Commands) != 0 {
		t.Errorf("expected npm install host command stripped (node_modules symlinked), got %v", p.Commands)
	}
}
