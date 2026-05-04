//go:build integration

package integration_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/lost-in-the/grove/internal/detect"
)

// fixturesDir returns the absolute path to testdata/fixtures/.
// Tests use file copying rather than symlinking so fixture dirs stay read-only.
func fixturesDir(t *testing.T) string {
	t.Helper()
	// Walk up from tests/integration/ to the repo root.
	// __file__ equivalent: use the known relative path from module root.
	// In Go tests, os.Getwd() returns the package directory.
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	// wd is something like .../grove/tests/integration
	root := filepath.Join(wd, "..", "..")
	return filepath.Join(root, "testdata", "fixtures")
}

// copyFixture copies all files from src directory into a new temp dir,
// preserving the relative directory structure.
func copyFixture(t *testing.T, src string) string {
	t.Helper()
	dst := t.TempDir()
	entries, err := os.ReadDir(src)
	if err != nil {
		t.Fatalf("ReadDir %s: %v", src, err)
	}
	for _, e := range entries {
		if e.IsDir() {
			continue // fixtures are flat
		}
		data, err := os.ReadFile(filepath.Join(src, e.Name()))
		if err != nil {
			t.Fatalf("ReadFile %s: %v", e.Name(), err)
		}
		if err := os.WriteFile(filepath.Join(dst, e.Name()), data, 0644); err != nil {
			t.Fatalf("WriteFile %s: %v", e.Name(), err)
		}
	}
	return dst
}

// TestDetectProjectType_RailsCompose asserts that the rails-compose fixture is
// detected as a "mixed" (rails+docker) project with a compose-managed Docker service.
func TestDetectProjectType_RailsCompose(t *testing.T) {
	dir := copyFixture(t, filepath.Join(fixturesDir(t), "rails-compose"))

	profile := detect.Detect(dir)

	if profile.Type != "mixed" {
		t.Errorf("Type: got %q, want %q", profile.Type, "mixed")
	}
	if !profile.HasDocker {
		t.Error("HasDocker: want true for rails-compose fixture")
	}
	if profile.DockerService == "" {
		t.Error("DockerService: want non-empty — should infer 'web' from compose file")
	}
	if profile.DockerService != "web" {
		t.Errorf("DockerService: got %q, want %q", profile.DockerService, "web")
	}
	// Commands should have been moved to ContainerCommands since compose is present.
	if len(profile.Commands) != 0 {
		t.Errorf("Commands: want 0 (moved to ContainerCommands), got %d: %v", len(profile.Commands), profile.Commands)
	}
	if len(profile.ContainerCommands) == 0 {
		t.Error("ContainerCommands: want at least one (bundle install) for rails+docker")
	}
}

// TestDetectProjectType_NodeCompose asserts that the node-compose fixture is
// detected as a "mixed" (node+docker) project with Docker compose present.
func TestDetectProjectType_NodeCompose(t *testing.T) {
	dir := copyFixture(t, filepath.Join(fixturesDir(t), "node-compose"))

	profile := detect.Detect(dir)

	if profile.Type != "mixed" {
		t.Errorf("Type: got %q, want %q", profile.Type, "mixed")
	}
	if !profile.HasDocker {
		t.Error("HasDocker: want true for node-compose fixture")
	}
	if profile.DockerService == "" {
		t.Error("DockerService: want non-empty — should infer 'app' from compose file")
	}
	if profile.DockerService != "app" {
		t.Errorf("DockerService: got %q, want %q", profile.DockerService, "app")
	}
	// npm install should have been routed to ContainerCommands.
	if len(profile.ContainerCommands) == 0 {
		t.Error("ContainerCommands: want at least one (npm install) for node+docker")
	}
	for _, cc := range profile.ContainerCommands {
		if cc.Service == "" {
			t.Errorf("ContainerCommand %q: service must be non-empty", cc.Command)
		}
	}
}

// TestDetectProjectType_DockerfileOnly asserts that the dockerfile-only fixture
// has HasDocker=true (Dockerfile present) but Type="unknown" because the detect
// rules only fire on language markers and docker-compose.yml — a bare Dockerfile
// alone does not match any rule. DockerService is empty (no compose file to infer
// from) and ContainerCommands is empty (no language toolchain means no setup
// commands to move).
func TestDetectProjectType_DockerfileOnly(t *testing.T) {
	dir := copyFixture(t, filepath.Join(fixturesDir(t), "dockerfile-only"))

	profile := detect.Detect(dir)

	// No rule fires for a plain Dockerfile — type stays "unknown".
	if profile.Type != "unknown" {
		t.Errorf("Type: got %q, want %q (bare Dockerfile has no rule)", profile.Type, "unknown")
	}
	// HasDocker is set by compose.HasDocker which checks for Dockerfile.
	if !profile.HasDocker {
		t.Error("HasDocker: want true — Dockerfile is present")
	}
	// No compose file → DockerService should be empty.
	if profile.DockerService != "" {
		t.Errorf("DockerService: got %q, want empty (no compose file)", profile.DockerService)
	}
	// No language marker → no commands → ContainerCommands empty.
	if len(profile.ContainerCommands) != 0 {
		t.Errorf("ContainerCommands: got %d, want 0 for dockerfile-only", len(profile.ContainerCommands))
	}
}
