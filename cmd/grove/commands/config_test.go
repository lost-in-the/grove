package commands

import (
	"os"
	"path/filepath"
	"testing"
)

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
