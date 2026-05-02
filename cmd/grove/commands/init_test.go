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
