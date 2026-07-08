package detect

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDetect_RailsProject(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "Gemfile", "source 'https://rubygems.org'")

	profile := Detect(dir)

	if profile.Type != "rails" {
		t.Errorf("expected type 'rails', got %q", profile.Type)
	}
	assertContains(t, profile.Copy, ".env")
	assertContains(t, profile.Copy, "config/master.key")
	assertContains(t, profile.Symlinks, "vendor/bundle")
	assertContains(t, profile.Commands, "bundle install --quiet")
}

func TestDetect_NodeProject(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "package.json", "{}")

	profile := Detect(dir)

	if profile.Type != "node" {
		t.Errorf("expected type 'node', got %q", profile.Type)
	}
	assertContains(t, profile.Copy, ".env")
	assertContains(t, profile.Symlinks, "node_modules")
	assertContains(t, profile.Commands, "npm install")
}

func TestDetect_GoProject(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "go.mod", "module example.com/test")

	profile := Detect(dir)

	if profile.Type != "go" {
		t.Errorf("expected type 'go', got %q", profile.Type)
	}
	assertContains(t, profile.Copy, ".env")
	if len(profile.Symlinks) != 0 {
		t.Errorf("expected no symlinks for go project, got %v", profile.Symlinks)
	}
}

func TestDetect_PythonProject(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "requirements.txt", "flask==2.0")

	profile := Detect(dir)

	if profile.Type != "python" {
		t.Errorf("expected type 'python', got %q", profile.Type)
	}
	assertContains(t, profile.Symlinks, ".venv")
	assertContains(t, profile.Commands, "pip install -r requirements.txt")
}

func TestDetect_MixedProject(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "Gemfile", "source 'https://rubygems.org'")
	writeFile(t, dir, "package.json", "{}")

	profile := Detect(dir)

	if profile.Type != "mixed" {
		t.Errorf("expected type 'mixed', got %q", profile.Type)
	}
	if len(profile.Types) != 2 {
		t.Errorf("expected 2 types, got %d: %v", len(profile.Types), profile.Types)
	}
	// Should have both rails and node hooks, deduplicated
	assertContains(t, profile.Symlinks, "vendor/bundle")
	assertContains(t, profile.Symlinks, "node_modules")
}

func TestDetect_UnknownProject(t *testing.T) {
	dir := t.TempDir()

	profile := Detect(dir)

	if profile.Type != "unknown" {
		t.Errorf("expected type 'unknown', got %q", profile.Type)
	}
}

func TestDetect_AlwaysIncludesEnvIfPresent(t *testing.T) {
	dir := t.TempDir()
	// Unknown project type but .env.local exists
	writeFile(t, dir, ".env.local", "SECRET=foo")

	profile := Detect(dir)

	if profile.Type != "unknown" {
		t.Errorf("expected type 'unknown', got %q", profile.Type)
	}
	assertContains(t, profile.Copy, ".env.local")
}

func TestDetect_DeduplicatesCopyEntries(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "Gemfile", "")
	writeFile(t, dir, "go.mod", "module test")

	profile := Detect(dir)

	// .env should appear only once despite both rules including it
	count := 0
	for _, f := range profile.Copy {
		if f == ".env" {
			count++
		}
	}
	if count != 1 {
		t.Errorf("expected .env once, got %d times in %v", count, profile.Copy)
	}
}

func TestDetect_RailsWithDockerRoutesToContainer(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "Gemfile", "source 'https://rubygems.org'")
	writeFile(t, dir, "docker-compose.yml", `services:
  web:
    image: ruby:3
  postgres:
    image: postgres:15
`)

	profile := Detect(dir)

	if !profile.HasDocker {
		t.Error("expected HasDocker=true")
	}
	if len(profile.Commands) != 0 {
		t.Errorf("expected no host commands when Docker detected, got %v", profile.Commands)
	}
	if len(profile.ContainerCommands) == 0 {
		t.Fatal("expected ContainerCommands populated")
	}
	cc := profile.ContainerCommands[0]
	if cc.Service != "web" {
		t.Errorf("expected service=web, got %q", cc.Service)
	}
	if cc.Command != "bundle install --quiet" {
		t.Errorf("expected bundle install, got %q", cc.Command)
	}
	// Symlinks remain (they're host-side, vendor/bundle is the symlink target).
	assertContains(t, profile.Symlinks, "vendor/bundle")
}

func TestDetect_NodeWithDockerfileOnlyKeepsHostCommands(t *testing.T) {
	// Dockerfile-only (no compose file) projects don't get docker:compose
	// hooks auto-generated — those would error every grove new since there's
	// no compose project to run against. Host commands stay; renderer flags
	// the situation so the user can switch them to docker:exec.
	dir := t.TempDir()
	writeFile(t, dir, "package.json", "{}")
	writeFile(t, dir, "Dockerfile", "FROM node:20")

	profile := Detect(dir)

	if !profile.HasDocker {
		t.Error("expected HasDocker=true (Dockerfile alone)")
	}
	if !profile.DockerComposeMissing {
		t.Error("expected DockerComposeMissing=true for Dockerfile-only project")
	}
	if len(profile.ContainerCommands) != 0 {
		t.Errorf("expected NO container commands (no compose to run against), got %v", profile.ContainerCommands)
	}
	if len(profile.Commands) == 0 {
		t.Errorf("expected host commands preserved, got empty")
	}
}

func TestDetect_AllInfraComposeKeepsHostCommands(t *testing.T) {
	// A compose file with only db/redis (no app service) shouldn't reroute
	// install commands into a service we'd be guessing at.
	dir := t.TempDir()
	writeFile(t, dir, "Gemfile", "")
	writeFile(t, dir, "docker-compose.yml", `services:
  postgres:
    image: postgres:15
  redis:
    image: redis:7
`)
	profile := Detect(dir)

	if !profile.DockerComposeMissing {
		t.Error("expected DockerComposeMissing=true when no app service inferable")
	}
	if len(profile.ContainerCommands) != 0 {
		t.Errorf("expected no container commands when service unknown, got %v", profile.ContainerCommands)
	}
	if len(profile.Commands) == 0 {
		t.Error("expected host commands preserved when service unknown")
	}
}

func TestDetect_NoDockerKeepsHostCommands(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "Gemfile", "source 'https://rubygems.org'")

	profile := Detect(dir)

	if profile.HasDocker {
		t.Error("expected HasDocker=false")
	}
	if len(profile.Commands) == 0 {
		t.Fatal("expected host commands preserved without docker")
	}
	if len(profile.ContainerCommands) != 0 {
		t.Errorf("expected no container commands without docker, got %v", profile.ContainerCommands)
	}
}

func TestDetect_SingleServiceComposeNotFlaggedInferred(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "Gemfile", "")
	writeFile(t, dir, "docker-compose.yml", `services:
  app:
    image: ruby:3
`)
	profile := Detect(dir)
	if profile.DockerService != "app" {
		t.Errorf("expected service=app, got %q", profile.DockerService)
	}
	if profile.DockerServiceInferred {
		t.Error("single-service compose should not be flagged as inferred")
	}
}

func writeFile(t *testing.T, dir, name, content string) {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
}

func assertContains(t *testing.T, slice []string, item string) {
	t.Helper()
	for _, s := range slice {
		if s == item {
			return
		}
	}
	t.Errorf("expected %v to contain %q", slice, item)
}

func TestDetect_ModernComposeFilenames(t *testing.T) {
	// Regression: the docker rule only matched docker-compose.yml, while
	// HasDocker/FindComposeFile accept four filenames — so compose.yaml-only
	// repos were typed "unknown" despite HasDocker being true.
	for _, filename := range []string{"docker-compose.yml", "docker-compose.yaml", "compose.yml", "compose.yaml"} {
		t.Run(filename, func(t *testing.T) {
			dir := t.TempDir()
			writeFile(t, dir, filename, "services:\n  app:\n    image: x\n")

			profile := Detect(dir)
			if profile.Type != "docker" {
				t.Errorf("Type = %q, want %q", profile.Type, "docker")
			}
			if !profile.HasDocker {
				t.Error("HasDocker = false, want true")
			}
		})
	}
}

func TestDetect_MixedWithModernComposeFilename(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "Gemfile", "source 'https://rubygems.org'")
	writeFile(t, dir, "compose.yaml", "services:\n  app:\n    image: x\n")

	profile := Detect(dir)
	if profile.Type != "mixed" {
		t.Errorf("Type = %q, want %q", profile.Type, "mixed")
	}
}
