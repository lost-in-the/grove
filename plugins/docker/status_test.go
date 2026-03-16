package docker

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lost-in-the/grove/internal/config"
	"github.com/lost-in-the/grove/internal/plugins"
)

func TestReadEnvVar(t *testing.T) {
	dir := t.TempDir()
	envContent := `APP_DIR=/some/path
OTHER_VAR=value
EMPTY_VAR=
`
	if err := os.WriteFile(filepath.Join(dir, ".env"), []byte(envContent), 0644); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name string
		key  string
		want string
	}{
		{"existing key", "APP_DIR", "/some/path"},
		{"another key", "OTHER_VAR", "value"},
		{"empty value", "EMPTY_VAR", ""},
		{"missing key", "NONEXISTENT", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := readEnvVar(dir, tt.key)
			if got != tt.want {
				t.Errorf("readEnvVar(%q, %q) = %q, want %q", dir, tt.key, got, tt.want)
			}
		})
	}
}

func TestReadEnvVar_NoFile(t *testing.T) {
	got := readEnvVar(t.TempDir(), "KEY")
	if got != "" {
		t.Errorf("expected empty string for missing .env, got %q", got)
	}
}

func TestWorktreeStatuses_NilStrategy(t *testing.T) {
	p := &Plugin{strategy: nil}
	result := p.WorktreeStatuses([]string{"/some/path"})
	if result != nil {
		t.Errorf("expected nil result for nil strategy, got %v", result)
	}
}

func TestLocalStatuses_NoComposeFile(t *testing.T) {
	dir := t.TempDir()
	result := localStatuses(&localStrategy{}, []string{dir})
	if len(result) != 0 {
		t.Errorf("expected empty result for dir without compose file, got %v", result)
	}
}

func TestLocalStatuses_WithComposeFile(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "docker-compose.yml"), []byte("services: {}"), 0644); err != nil {
		t.Fatalf("failed to create compose file: %v", err)
	}

	result := localStatuses(&localStrategy{}, []string{dir})
	entry, ok := result[dir]
	if !ok {
		t.Fatalf("expected entry for dir with compose file, got %v", result)
	}
	if entry.ProviderName != "docker" {
		t.Errorf("expected ProviderName 'docker', got %q", entry.ProviderName)
	}
	// docker may not be running in CI — at minimum we get StatusInfo ("compose found")
	if entry.Level != plugins.StatusInfo && entry.Level != plugins.StatusActive {
		t.Errorf("expected StatusInfo or StatusActive, got %v", entry.Level)
	}
}

func TestExternalStatuses_NoMatch(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, ".env"), []byte("APP_DIR=/other/path\n"), 0644); err != nil {
		t.Fatalf("failed to create .env: %v", err)
	}

	s := &externalStrategy{
		ext: &config.ExternalComposeConfig{
			Path:   dir,
			EnvVar: "APP_DIR",
		},
	}

	result := externalStatuses(s, []string{"/my/worktree"})
	if len(result) != 0 {
		t.Errorf("expected no matches for unrelated path, got %v", result)
	}
}

func TestExternalStatuses_MatchingPath(t *testing.T) {
	composeDir := t.TempDir()
	worktreePath := t.TempDir()

	envContent := fmt.Sprintf("APP_DIR=%s\n", worktreePath)
	if err := os.WriteFile(filepath.Join(composeDir, ".env"), []byte(envContent), 0644); err != nil {
		t.Fatalf("failed to create .env: %v", err)
	}

	s := &externalStrategy{
		ext: &config.ExternalComposeConfig{
			Path:   composeDir,
			EnvVar: "APP_DIR",
		},
	}

	result := externalStatuses(s, []string{worktreePath})
	entry, ok := result[worktreePath]
	if !ok {
		t.Fatalf("expected entry for matching worktree path, got %v", result)
	}
	if entry.ProviderName != "docker" {
		t.Errorf("expected ProviderName 'docker', got %q", entry.ProviderName)
	}
	// docker may not be running — expect StatusWarning (pointed but not running) or StatusActive
	if entry.Level != plugins.StatusWarning && entry.Level != plugins.StatusActive {
		t.Errorf("expected StatusWarning or StatusActive, got %v", entry.Level)
	}
}

func TestAgentStatuses_NoSlots(t *testing.T) {
	dir := t.TempDir()
	s := &agentExternalStrategy{
		agent: &config.AgentStackConfig{},
		slots: NewSlotManager(filepath.Join(dir, ".slots.json"), 5),
	}

	result := agentStatuses(s, []string{"/some/worktree"})
	if len(result) != 0 {
		t.Errorf("expected empty result with no active slots, got %v", result)
	}
}

func TestAgentStatuses_WithSlot(t *testing.T) {
	dir := t.TempDir()
	slotsFile := filepath.Join(dir, ".slots.json")
	if err := os.WriteFile(slotsFile, []byte(`[{"slot":1,"worktree":"my-worktree"}]`), 0644); err != nil {
		t.Fatalf("failed to write slots file: %v", err)
	}

	s := &agentExternalStrategy{
		agent: &config.AgentStackConfig{},
		slots: NewSlotManager(slotsFile, 5),
	}

	worktreePath := filepath.Join(dir, "my-worktree")
	result := agentStatuses(s, []string{worktreePath})
	entry, ok := result[worktreePath]
	if !ok {
		t.Fatalf("expected entry for worktree with active slot, got %v", result)
	}
	if entry.Level != plugins.StatusActive {
		t.Errorf("expected StatusActive, got %v", entry.Level)
	}
	if entry.Short != "#1" {
		t.Errorf("expected Short '#1', got %q", entry.Short)
	}
}

func TestAgentStatuses_URLPattern(t *testing.T) {
	dir := t.TempDir()
	slotsFile := filepath.Join(dir, ".slots.json")
	if err := os.WriteFile(slotsFile, []byte(`[{"slot":2,"worktree":"feature-x"}]`), 0644); err != nil {
		t.Fatalf("failed to write slots file: %v", err)
	}

	s := &agentExternalStrategy{
		agent: &config.AgentStackConfig{
			URLPattern: "http://localhost:808{slot}",
		},
		slots: NewSlotManager(slotsFile, 5),
	}

	worktreePath := filepath.Join(dir, "feature-x")
	result := agentStatuses(s, []string{worktreePath})
	entry, ok := result[worktreePath]
	if !ok {
		t.Fatalf("expected entry for worktree with slot, got %v", result)
	}
	if entry.Short != "#2" {
		t.Errorf("expected Short '#2', got %q", entry.Short)
	}
	expectedURL := "http://localhost:8082"
	if !strings.Contains(entry.Detail, expectedURL) {
		t.Errorf("expected Detail to contain URL %q, got %q", expectedURL, entry.Detail)
	}
}

func TestComposeRunningCount_NoCompose(t *testing.T) {
	// A temp dir with no compose file — docker compose ps should fail, returning (false, 0).
	dir := t.TempDir()
	running, count := composeRunningCount(dir)
	if running {
		t.Error("expected not running for directory without compose file")
	}
	if count != 0 {
		t.Errorf("expected count 0, got %d", count)
	}
}

func TestCurrentServiceInfo_NilConfig(t *testing.T) {
	result := CurrentServiceInfo(nil, "/some/path")
	if result != nil {
		t.Errorf("expected nil for nil config, got %+v", result)
	}
}

func TestCurrentServiceInfo_DisabledDocker(t *testing.T) {
	disabled := false
	cfg := &config.Config{}
	cfg.Plugins.Docker.Enabled = &disabled
	result := CurrentServiceInfo(cfg, "/some/path")
	if result != nil {
		t.Errorf("expected nil for disabled docker, got %+v", result)
	}
}

func TestCurrentServiceInfo_LocalNoCompose(t *testing.T) {
	dir := t.TempDir()
	cfg := &config.Config{}
	// mode defaults to "local" when empty
	result := CurrentServiceInfo(cfg, dir)
	if result != nil {
		t.Errorf("expected nil for local mode without compose file, got %+v", result)
	}
}

func TestCurrentServiceInfo_UnknownMode(t *testing.T) {
	cfg := &config.Config{}
	cfg.Plugins.Docker.Mode = "custom"
	result := CurrentServiceInfo(cfg, "/some/path")
	if result != nil {
		t.Errorf("expected nil for unknown mode, got %+v", result)
	}
}

func TestPathMatchesEnv(t *testing.T) {
	tests := []struct {
		name         string
		worktreePath string
		envValue     string
		composePath  string
		want         bool
	}{
		{
			name:         "empty env value",
			worktreePath: "/work/project-feature",
			envValue:     "",
			composePath:  "/docker",
			want:         false,
		},
		{
			name:         "absolute match",
			worktreePath: "/work/project-feature",
			envValue:     "/work/project-feature",
			composePath:  "/docker",
			want:         true,
		},
		{
			name:         "absolute no match",
			worktreePath: "/work/project-feature",
			envValue:     "/work/project-other",
			composePath:  "/docker",
			want:         false,
		},
		{
			name:         "relative match",
			worktreePath: "/docker/project-feature",
			envValue:     "./project-feature",
			composePath:  "/docker",
			want:         true,
		},
		{
			name:         "relative no match",
			worktreePath: "/work/project-feature",
			envValue:     "./project-other",
			composePath:  "/docker",
			want:         false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := pathMatchesEnv(tt.worktreePath, tt.envValue, tt.composePath)
			if got != tt.want {
				t.Errorf("pathMatchesEnv(%q, %q, %q) = %v, want %v",
					tt.worktreePath, tt.envValue, tt.composePath, got, tt.want)
			}
		})
	}
}
