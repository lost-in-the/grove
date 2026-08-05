package tui

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/lost-in-the/grove/internal/config"
	"github.com/lost-in-the/grove/internal/mux"
)

func TestMuxForHonorsConfiguredBackend(t *testing.T) {
	cfg := config.LoadDefaults()
	cfg.Mux.Backend = "herdr"

	// An explicitly configured backend is honored regardless of what is
	// installed — otherwise a project pinned to herdr would silently get tmux.
	if got := muxFor(cfg).Backend(); got != mux.BackendHerdr {
		t.Errorf("muxFor(herdr).Backend() = %q, want %q", got, mux.BackendHerdr)
	}

	cfg.Mux.Backend = "off"
	if got := muxFor(cfg).Backend(); got != mux.BackendOff {
		t.Errorf("muxFor(off).Backend() = %q, want %q", got, mux.BackendOff)
	}
}

func TestMuxForNilConfigFallsBackToAuto(t *testing.T) {
	// A nil config can't say anything about the backend, so resolution has to
	// come from the environment. It must not panic.
	if muxFor(nil) == nil {
		t.Fatal("muxFor(nil) returned nil")
	}
}

func TestMuxForRepoReadsProjectConfig(t *testing.T) {
	// The delete / fork / rename paths have no loaded config in hand. Reading
	// it from the repo is what keeps them from ignoring [mux] backend.
	repo := t.TempDir()
	groveDir := filepath.Join(repo, ".grove")
	if err := os.MkdirAll(groveDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	cfgPath := filepath.Join(groveDir, "config.toml")
	if err := os.WriteFile(cfgPath, []byte("[mux]\nbackend = \"herdr\"\n"), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	if got := muxForRepo(repo).Backend(); got != mux.BackendHerdr {
		t.Errorf("muxForRepo() = %q, want %q — project config was ignored", got, mux.BackendHerdr)
	}
}

func TestMuxForRepoWithoutConfigDoesNotPanic(t *testing.T) {
	if muxForRepo(t.TempDir()) == nil {
		t.Fatal("muxForRepo() returned nil for a repo with no .grove")
	}
}

func TestMuxForRepoRespectsLegacyTmuxOff(t *testing.T) {
	repo := t.TempDir()
	groveDir := filepath.Join(repo, ".grove")
	if err := os.MkdirAll(groveDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	cfgPath := filepath.Join(groveDir, "config.toml")
	if err := os.WriteFile(cfgPath, []byte("[tmux]\nmode = \"off\"\n"), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	if got := muxForRepo(repo).Backend(); got != mux.BackendOff {
		t.Errorf("muxForRepo() = %q, want %q — legacy tmux.mode=off ignored", got, mux.BackendOff)
	}
}
