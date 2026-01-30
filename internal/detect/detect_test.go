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
