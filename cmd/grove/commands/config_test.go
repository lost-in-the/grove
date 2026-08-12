package commands

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lost-in-the/grove/internal/cli"
	"github.com/lost-in-the/grove/internal/config"
)

// captureConfigOutput renders the `grove config` summary into a string.
func captureConfigOutput(t *testing.T, cfg *config.Config) string {
	t.Helper()
	var buf bytes.Buffer
	renderConfigSummary(cli.NewWriter(&buf, false), cfg)
	return buf.String()
}

func TestLoadGlobalOnlyConfig_ReadsOnlyGlobalFile(t *testing.T) {
	dir := t.TempDir()
	globalPath := filepath.Join(dir, "config.toml")
	if err := os.WriteFile(globalPath, []byte("default_base_branch = \"trunk\"\n"), 0644); err != nil {
		t.Fatalf("write global config: %v", err)
	}

	cfg, err := loadGlobalOnlyConfig(globalPath)
	if err != nil {
		t.Fatalf("loadGlobalOnlyConfig: %v", err)
	}
	if cfg.DefaultBranch != "trunk" {
		t.Errorf("DefaultBranch = %q, want %q (value from the global file)", cfg.DefaultBranch, "trunk")
	}
}

func TestLoadGlobalOnlyConfig_MissingFileYieldsDefaults(t *testing.T) {
	dir := t.TempDir()
	cfg, err := loadGlobalOnlyConfig(filepath.Join(dir, "nope.toml"))
	if err != nil {
		t.Fatalf("expected defaults for missing file, got error: %v", err)
	}
	if cfg == nil {
		t.Fatal("expected non-nil defaults config")
	}
}

func TestLoadGlobalOnlyConfig_CorruptFileErrors(t *testing.T) {
	dir := t.TempDir()
	globalPath := filepath.Join(dir, "config.toml")
	if err := os.WriteFile(globalPath, []byte("not [valid toml"), 0644); err != nil {
		t.Fatalf("write corrupt config: %v", err)
	}
	if _, err := loadGlobalOnlyConfig(globalPath); err == nil {
		t.Error("expected error for corrupt global config")
	}
}

func TestConfigShowsMuxBackend(t *testing.T) {
	// The [mux] section was added after `grove config` was written; without an
	// explicit row a user can set a backend the command never shows back.
	cfg := config.LoadDefaults()
	cfg.Mux.Backend = "herdr"

	out := captureConfigOutput(t, cfg)
	if !strings.Contains(out, "[mux]") {
		t.Errorf("config output has no [mux] section:\n%s", out)
	}
	if !strings.Contains(out, "herdr") {
		t.Errorf("config output does not show the backend:\n%s", out)
	}
}

func TestConfigShowsEffectiveBackendWhenOverridden(t *testing.T) {
	// tmux.mode = "off" silently overrides the backend, so the effective value
	// has to be visible or the config reads as a contradiction.
	cfg := config.LoadDefaults()
	cfg.Mux.Backend = "herdr"
	cfg.Tmux.Mode = "off"

	out := captureConfigOutput(t, cfg)
	if !strings.Contains(out, "(effective)") {
		t.Errorf("config output does not flag the override:\n%s", out)
	}
}
