package docker

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/lost-in-the/grove/internal/config"
)

// newTestAgentConfigWithSlots builds an agent-mode config (like
// newTestAgentConfig) whose external.path is composePath, so tests can
// control the exact directory that appears in fake docker occupancy output
// and compare it against what grove resolves.
func newTestAgentConfigWithSlots(t *testing.T, projectName, composePath string) *config.Config {
	t.Helper()
	enabled := true
	return &config.Config{
		ProjectName: projectName,
		Plugins: config.PluginsConfig{
			Docker: config.DockerPluginConfig{
				Mode: "external",
				External: &config.ExternalComposeConfig{
					Path:     composePath,
					EnvVar:   "APP_DIR",
					Services: []string{"app"},
					Agent: &config.AgentStackConfig{
						Enabled:      &enabled,
						MaxSlots:     3,
						Services:     []string{"app"},
						TemplatePath: "agent-stacks/template.yml",
						URLPattern:   "http://localhost:{slot}",
					},
				},
			},
		},
	}
}

func writeFakeDocker(t *testing.T, lines ...string) {
	t.Helper()
	if _, err := exec.LookPath("sh"); err != nil {
		t.Skip("sh not available")
	}

	script := "#!/bin/sh\n"
	for _, l := range lines {
		script += "echo '" + l + "'\n"
	}
	script += "exit 0\n"

	fakeDockerDir := t.TempDir()
	fakeDockerPath := fakeDockerDir + "/docker"
	if err := os.WriteFile(fakeDockerPath, []byte(script), 0755); err != nil {
		t.Fatalf("write fake docker: %v", err)
	}

	origPath := os.Getenv("PATH")
	t.Setenv("PATH", fakeDockerDir+":"+origPath)
}

func TestListStackSlots_NotConfigured(t *testing.T) {
	cfg := &config.Config{
		ProjectName: "myapp",
		Plugins: config.PluginsConfig{
			Docker: config.DockerPluginConfig{
				Mode: "external",
				External: &config.ExternalComposeConfig{
					Path:     t.TempDir(),
					EnvVar:   "APP_DIR",
					Services: []string{"app"},
				},
			},
		},
	}

	if _, err := ListStackSlots(cfg); err == nil {
		t.Error("ListStackSlots() expected error when agent stack isn't configured")
	}
}

func TestListStackSlots_FallsBackToLocalRecordsWhenOccupancyUnknown(t *testing.T) {
	composePath := t.TempDir()
	cfg := newTestAgentConfigWithSlots(t, "myapp", composePath)

	slotsFile := filepath.Join(composePath, "agent-stacks", ".slots.json")
	_ = os.MkdirAll(filepath.Dir(slotsFile), 0755)
	sm := NewSlotManager(slotsFile, 3)
	if _, err := sm.Allocate("myapp-feature-x"); err != nil {
		t.Fatalf("Allocate() error = %v", err)
	}

	// Point docker at a script that fails, simulating "docker unreachable" —
	// occupancy should degrade to the local record rather than erroring out.
	writeFakeDocker(t) // no lines: empty docker ps output, still "succeeds"

	slots, err := ListStackSlots(cfg)
	if err != nil {
		t.Fatalf("ListStackSlots() error = %v", err)
	}
	if len(slots) != 1 {
		t.Fatalf("ListStackSlots() = %v, want 1 entry", slots)
	}
	if slots[0].Worktree != "myapp-feature-x" || slots[0].Foreign {
		t.Errorf("ListStackSlots()[0] = %+v, want local attribution to myapp-feature-x, not foreign", slots[0])
	}
}

func TestListStackSlots_AttributesForeignSlotInsteadOfClaimingIt(t *testing.T) {
	composePath := t.TempDir()
	cfg := newTestAgentConfigWithSlots(t, "myapp", composePath)

	// This grove project (myapp) has never allocated slot 1 in its own
	// records, but a different grove project's compose stack ("otherapp")
	// already holds it at the same shared compose path — ground truth from
	// docker should surface that instead of silently reporting nothing (#147).
	writeFakeDocker(t, "otherapp-agent-1|"+composePath)

	slots, err := ListStackSlots(cfg)
	if err != nil {
		t.Fatalf("ListStackSlots() error = %v", err)
	}
	if len(slots) != 1 {
		t.Fatalf("ListStackSlots() = %v, want 1 entry", slots)
	}
	got := slots[0]
	if !got.Foreign {
		t.Errorf("ListStackSlots()[0].Foreign = false, want true")
	}
	if got.Project != "otherapp-agent-1" {
		t.Errorf("ListStackSlots()[0].Project = %q, want %q", got.Project, "otherapp-agent-1")
	}
	if got.Worktree != "" {
		t.Errorf("ListStackSlots()[0].Worktree = %q, want empty — slot isn't ours", got.Worktree)
	}
}

func TestListStackSlots_OwnSlotConfirmedByDockerIsNotForeign(t *testing.T) {
	composePath := t.TempDir()
	cfg := newTestAgentConfigWithSlots(t, "myapp", composePath)

	slotsFile := filepath.Join(composePath, "agent-stacks", ".slots.json")
	_ = os.MkdirAll(filepath.Dir(slotsFile), 0755)
	sm := NewSlotManager(slotsFile, 3)
	if _, err := sm.Allocate("myapp-feature-x"); err != nil { // slot 1
		t.Fatalf("Allocate() error = %v", err)
	}

	// Ground truth agrees: myapp-agent-1 (this project's own naming) holds slot 1.
	writeFakeDocker(t, "myapp-agent-1|"+composePath)

	slots, err := ListStackSlots(cfg)
	if err != nil {
		t.Fatalf("ListStackSlots() error = %v", err)
	}
	if len(slots) != 1 {
		t.Fatalf("ListStackSlots() = %v, want 1 entry", slots)
	}
	got := slots[0]
	if got.Foreign {
		t.Errorf("ListStackSlots()[0].Foreign = true, want false for this project's own slot")
	}
	if got.Worktree != "myapp-feature-x" {
		t.Errorf("ListStackSlots()[0].Worktree = %q, want %q", got.Worktree, "myapp-feature-x")
	}
}
