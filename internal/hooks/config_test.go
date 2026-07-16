package hooks

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// TestFindOverride tests FindOverride with various match patterns
func TestFindOverride(t *testing.T) {
	tests := []struct {
		name      string
		overrides []Override
		branch    string
		worktree  string
		wantNil   bool
		wantMatch string
	}{
		{
			name:      "empty overrides returns nil",
			overrides: nil,
			branch:    "main",
			worktree:  "main",
			wantNil:   true,
		},
		{
			name:      "match on branch name",
			overrides: []Override{{Match: "feature-auth"}},
			branch:    "feature-auth",
			worktree:  "something-else",
			wantNil:   false,
			wantMatch: "feature-auth",
		},
		{
			name:      "match on worktree name",
			overrides: []Override{{Match: "testing"}},
			branch:    "main",
			worktree:  "testing",
			wantNil:   false,
			wantMatch: "testing",
		},
		{
			name:      "glob pattern matches branch",
			overrides: []Override{{Match: "feature/*"}},
			branch:    "feature/login",
			worktree:  "other",
			wantNil:   false,
			wantMatch: "feature/*",
		},
		{
			name:      "glob pattern no match",
			overrides: []Override{{Match: "feature/*"}},
			branch:    "main",
			worktree:  "main",
			wantNil:   true,
		},
		{
			name:      "no match returns nil",
			overrides: []Override{{Match: "specific-branch"}},
			branch:    "other-branch",
			worktree:  "other-worktree",
			wantNil:   true,
		},
		{
			name:      "first match wins",
			overrides: []Override{{Match: "main"}, {Match: "*"}},
			branch:    "main",
			worktree:  "main",
			wantNil:   false,
			wantMatch: "main",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &HooksConfig{Overrides: tt.overrides}
			got := cfg.FindOverride(tt.branch, tt.worktree)
			if tt.wantNil {
				if got != nil {
					t.Errorf("FindOverride() = %v, want nil", got)
				}
				return
			}
			if got == nil {
				t.Fatal("FindOverride() = nil, want non-nil")
			}
			if got.Match != tt.wantMatch {
				t.Errorf("FindOverride().Match = %q, want %q", got.Match, tt.wantMatch)
			}
		})
	}
}

func TestApplyOverride(t *testing.T) {
	baseActions := []HookAction{
		{Type: "copy", From: "a", To: "b"},
		{Type: "symlink", From: "c", To: "d"},
		{Type: "command", Command: "echo hello"},
	}

	t.Run("nil override returns original", func(t *testing.T) {
		result := ApplyOverride(baseActions, nil, "/main")
		if len(result) != len(baseActions) {
			t.Errorf("ApplyOverride() = %d actions, want %d", len(result), len(baseActions))
		}
	})

	t.Run("SkipHooks returns nil", func(t *testing.T) {
		override := &Override{SkipHooks: true}
		result := ApplyOverride(baseActions, override, "/main")
		if result != nil {
			t.Errorf("ApplyOverride() = %v, want nil", result)
		}
	})

	t.Run("skip specific types", func(t *testing.T) {
		override := &Override{Skip: []string{"copy", "symlink"}}
		result := ApplyOverride(baseActions, override, "/main")
		if len(result) != 1 {
			t.Fatalf("ApplyOverride() = %d actions, want 1", len(result))
		}
		if result[0].Type != "command" {
			t.Errorf("remaining action type = %q, want %q", result[0].Type, "command")
		}
	})

	t.Run("ExtraCopy adds copy actions with defaults", func(t *testing.T) {
		override := &Override{ExtraCopy: []string{"config.json", ".env"}}
		result := ApplyOverride(nil, override, "/main")
		if len(result) != 2 {
			t.Fatalf("ApplyOverride() = %d actions, want 2", len(result))
		}
		for _, a := range result {
			if a.Type != "copy" {
				t.Errorf("ExtraCopy action type = %q, want %q", a.Type, "copy")
			}
			if a.OnFailure != "warn" {
				t.Errorf("ExtraCopy action OnFailure = %q, want %q", a.OnFailure, "warn")
			}
			if a.Timeout != 60 {
				t.Errorf("ExtraCopy action Timeout = %d, want 60", a.Timeout)
			}
			if a.From != a.To {
				t.Errorf("ExtraCopy From != To: %q vs %q", a.From, a.To)
			}
		}
		if result[0].From != "config.json" {
			t.Errorf("ExtraCopy[0].From = %q, want %q", result[0].From, "config.json")
		}
	})

	t.Run("ExtraRun adds command actions with defaults", func(t *testing.T) {
		override := &Override{ExtraRun: []string{"npm install", "make build"}}
		result := ApplyOverride(nil, override, "/main")
		if len(result) != 2 {
			t.Fatalf("ApplyOverride() = %d actions, want 2", len(result))
		}
		for _, a := range result {
			if a.Type != "command" {
				t.Errorf("ExtraRun action type = %q, want %q", a.Type, "command")
			}
			if a.WorkingDir != "new" {
				t.Errorf("ExtraRun action WorkingDir = %q, want %q", a.WorkingDir, "new")
			}
			if a.Timeout != 300 {
				t.Errorf("ExtraRun action Timeout = %d, want 300", a.Timeout)
			}
			if a.OnFailure != "warn" {
				t.Errorf("ExtraRun action OnFailure = %q, want %q", a.OnFailure, "warn")
			}
		}
		if result[0].Command != "npm install" {
			t.Errorf("ExtraRun[0].Command = %q, want %q", result[0].Command, "npm install")
		}
		if result[1].Command != "make build" {
			t.Errorf("ExtraRun[1].Command = %q, want %q", result[1].Command, "make build")
		}
	})
}

func TestLoadHooksConfigFromPath(t *testing.T) {
	t.Run("valid TOML file", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "hooks.toml")
		content := `
[[hooks.post_create]]
type = "copy"
from = "source.txt"
to = "dest.txt"
timeout = 30
`
		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			t.Fatalf("WriteFile: %v", err)
		}

		cfg, err := loadHooksConfigFromPath(path)
		if err != nil {
			t.Fatalf("loadHooksConfigFromPath() error = %v", err)
		}
		if len(cfg.Hooks.PostCreate) != 1 {
			t.Fatalf("PostCreate actions = %d, want 1", len(cfg.Hooks.PostCreate))
		}
		action := cfg.Hooks.PostCreate[0]
		if action.Type != "copy" {
			t.Errorf("action.Type = %q, want %q", action.Type, "copy")
		}
		// Defaults applied: OnFailure → "warn", WorkingDir → "new"
		if action.OnFailure != "warn" {
			t.Errorf("action.OnFailure = %q, want %q", action.OnFailure, "warn")
		}
		if action.WorkingDir != "new" {
			t.Errorf("action.WorkingDir = %q, want %q", action.WorkingDir, "new")
		}
		// Explicit timeout preserved
		if action.Timeout != 30 {
			t.Errorf("action.Timeout = %d, want 30", action.Timeout)
		}
	})

	t.Run("invalid TOML returns error", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "hooks.toml")
		if err := os.WriteFile(path, []byte(`key = {broken`), 0644); err != nil {
			t.Fatalf("WriteFile: %v", err)
		}
		_, err := loadHooksConfigFromPath(path)
		if err == nil {
			t.Error("loadHooksConfigFromPath() expected error for invalid TOML, got nil")
		}
	})

	t.Run("missing file returns error", func(t *testing.T) {
		_, err := loadHooksConfigFromPath("/nonexistent/path/hooks.toml")
		if err == nil {
			t.Error("loadHooksConfigFromPath() expected error for missing file, got nil")
		}
	})

	t.Run("docker:compose action decodes service/mode/timeout", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "hooks.toml")
		content := `
[[hooks.post_create]]
type = "docker:compose"
service = "web"
command = "bundle install"
mode = "run"
timeout = 900
`
		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			t.Fatalf("WriteFile: %v", err)
		}
		cfg, err := loadHooksConfigFromPath(path)
		if err != nil {
			t.Fatalf("decode: %v", err)
		}
		a := cfg.Hooks.PostCreate[0]
		if a.Type != "docker:compose" || a.Service != "web" || a.Command != "bundle install" || a.Mode != "run" || a.Timeout != 900 {
			t.Errorf("decoded fields wrong: %+v", a)
		}
	})

	t.Run("docker:exec action decodes container/shell", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "hooks.toml")
		content := `
[[hooks.post_create]]
type = "docker:exec"
container = "my-shell"
command = "ls"
shell = "bash -lc"
`
		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			t.Fatalf("WriteFile: %v", err)
		}
		cfg, err := loadHooksConfigFromPath(path)
		if err != nil {
			t.Fatalf("decode: %v", err)
		}
		a := cfg.Hooks.PostCreate[0]
		if a.Type != "docker:exec" || a.Container != "my-shell" || a.Shell != "bash -lc" {
			t.Errorf("decoded fields wrong: %+v", a)
		}
	})

	t.Run("legacy action without new fields decodes cleanly", func(t *testing.T) {
		// Verifies omitempty additions don't change parsing of existing files.
		dir := t.TempDir()
		path := filepath.Join(dir, "hooks.toml")
		content := `
[[hooks.post_create]]
type = "command"
command = "echo hi"
`
		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			t.Fatalf("WriteFile: %v", err)
		}
		cfg, err := loadHooksConfigFromPath(path)
		if err != nil {
			t.Fatalf("decode: %v", err)
		}
		a := cfg.Hooks.PostCreate[0]
		if a.Service != "" || a.Container != "" || a.Mode != "" || a.Shell != "" {
			t.Errorf("expected new fields to be empty, got: %+v", a)
		}
	})
}

func TestSetDefaultsForActions(t *testing.T) {
	t.Run("zero timeout gets 60", func(t *testing.T) {
		actions := []HookAction{{Type: "copy", Timeout: 0}}
		setDefaultsForActions(actions)
		if actions[0].Timeout != 60 {
			t.Errorf("Timeout = %d, want 60", actions[0].Timeout)
		}
	})

	t.Run("empty WorkingDir gets new", func(t *testing.T) {
		actions := []HookAction{{Type: "copy", WorkingDir: ""}}
		setDefaultsForActions(actions)
		if actions[0].WorkingDir != "new" {
			t.Errorf("WorkingDir = %q, want %q", actions[0].WorkingDir, "new")
		}
	})

	t.Run("empty OnFailure gets warn", func(t *testing.T) {
		actions := []HookAction{{Type: "copy", OnFailure: ""}}
		setDefaultsForActions(actions)
		if actions[0].OnFailure != "warn" {
			t.Errorf("OnFailure = %q, want %q", actions[0].OnFailure, "warn")
		}
	})

	t.Run("non-zero values preserved", func(t *testing.T) {
		actions := []HookAction{{
			Type:       "command",
			Timeout:    120,
			WorkingDir: "main",
			OnFailure:  "ignore",
		}}
		setDefaultsForActions(actions)
		if actions[0].Timeout != 120 {
			t.Errorf("Timeout = %d, want 120", actions[0].Timeout)
		}
		if actions[0].WorkingDir != "main" {
			t.Errorf("WorkingDir = %q, want %q", actions[0].WorkingDir, "main")
		}
		if actions[0].OnFailure != "ignore" {
			t.Errorf("OnFailure = %q, want %q", actions[0].OnFailure, "ignore")
		}
	})
}

func TestMergeHooksConfigs(t *testing.T) {
	globalAction := HookAction{Type: "copy", From: "global.txt", To: "global.txt", Timeout: 60, WorkingDir: "new", OnFailure: "warn"}
	projectAction := HookAction{Type: "copy", From: "project.txt", To: "project.txt", Timeout: 60, WorkingDir: "new", OnFailure: "warn"}

	t.Run("project appends to global by default", func(t *testing.T) {
		global := &HooksConfig{}
		global.Hooks.PostCreate = []HookAction{globalAction}
		project := &HooksConfig{}
		project.Hooks.PostCreate = []HookAction{projectAction}

		result := mergeHooksConfigs(global, project)
		if len(result.Hooks.PostCreate) != 2 {
			t.Fatalf("PostCreate = %d actions, want 2", len(result.Hooks.PostCreate))
		}
		if result.Hooks.PostCreate[0].From != "global.txt" {
			t.Errorf("PostCreate[0].From = %q, want %q", result.Hooks.PostCreate[0].From, "global.txt")
		}
		if result.Hooks.PostCreate[1].From != "project.txt" {
			t.Errorf("PostCreate[1].From = %q, want %q", result.Hooks.PostCreate[1].From, "project.txt")
		}
	})

	t.Run("project overrides global when override flag set", func(t *testing.T) {
		global := &HooksConfig{}
		global.Hooks.PostCreate = []HookAction{globalAction}
		project := &HooksConfig{}
		project.Hooks.PostCreate = []HookAction{projectAction}
		project.Hooks.OverridePostCreate = true

		result := mergeHooksConfigs(global, project)
		if len(result.Hooks.PostCreate) != 1 {
			t.Fatalf("PostCreate = %d actions, want 1 (override)", len(result.Hooks.PostCreate))
		}
		if result.Hooks.PostCreate[0].From != "project.txt" {
			t.Errorf("PostCreate[0].From = %q, want %q", result.Hooks.PostCreate[0].From, "project.txt")
		}
	})

	t.Run("overrides ordering project before global", func(t *testing.T) {
		global := &HooksConfig{Overrides: []Override{{Match: "global-*"}}}
		project := &HooksConfig{Overrides: []Override{{Match: "project-*"}}}

		result := mergeHooksConfigs(global, project)
		if len(result.Overrides) != 2 {
			t.Fatalf("Overrides = %d, want 2", len(result.Overrides))
		}
		if result.Overrides[0].Match != "project-*" {
			t.Errorf("Overrides[0].Match = %q, want %q", result.Overrides[0].Match, "project-*")
		}
		if result.Overrides[1].Match != "global-*" {
			t.Errorf("Overrides[1].Match = %q, want %q", result.Overrides[1].Match, "global-*")
		}
	})

	t.Run("all event types merged independently", func(t *testing.T) {
		global := &HooksConfig{}
		global.Hooks.PreCreate = []HookAction{globalAction}
		global.Hooks.PostCreate = []HookAction{globalAction}
		global.Hooks.PreSwitch = []HookAction{globalAction}
		global.Hooks.PostSwitch = []HookAction{globalAction}
		global.Hooks.PreRemove = []HookAction{globalAction}
		global.Hooks.PostRemove = []HookAction{globalAction}

		project := &HooksConfig{}
		project.Hooks.PreCreate = []HookAction{projectAction}

		result := mergeHooksConfigs(global, project)
		if len(result.Hooks.PreCreate) != 2 {
			t.Errorf("PreCreate = %d, want 2", len(result.Hooks.PreCreate))
		}
		if len(result.Hooks.PostCreate) != 1 {
			t.Errorf("PostCreate = %d, want 1 (only global)", len(result.Hooks.PostCreate))
		}
		if len(result.Hooks.PreSwitch) != 1 {
			t.Errorf("PreSwitch = %d, want 1", len(result.Hooks.PreSwitch))
		}
	})
}

func TestGetActionsForEvent(t *testing.T) {
	action := HookAction{Type: "copy"}
	cfg := &HooksConfig{}
	cfg.Hooks.PreCreate = []HookAction{action}
	cfg.Hooks.PostCreate = []HookAction{action, action}
	cfg.Hooks.PreSwitch = []HookAction{action}
	cfg.Hooks.PostSwitch = []HookAction{action}
	cfg.Hooks.PreRemove = []HookAction{action}
	cfg.Hooks.PostRemove = []HookAction{action}

	tests := []struct {
		event   string
		wantLen int
	}{
		{EventPreCreate, 1},
		{EventPostCreate, 2},
		{EventPreSwitch, 1},
		{EventPostSwitch, 1},
		{EventPreRemove, 1},
		{EventPostRemove, 1},
		{"unknown-event", 0},
		{"", 0},
	}

	for _, tt := range tests {
		t.Run(tt.event, func(t *testing.T) {
			got := cfg.GetActionsForEvent(tt.event)
			if len(got) != tt.wantLen {
				t.Errorf("GetActionsForEvent(%q) = %d actions, want %d", tt.event, len(got), tt.wantLen)
			}
		})
	}
}

func TestHasActionsForEvent(t *testing.T) {
	action := HookAction{Type: "copy"}
	cfg := &HooksConfig{}
	cfg.Hooks.PostCreate = []HookAction{action}

	tests := []struct {
		name  string
		event string
		want  bool
	}{
		{"true when actions exist", EventPostCreate, true},
		{"false when no actions", EventPreCreate, false},
		{"false for unknown event", "bogus-event", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := cfg.HasActionsForEvent(tt.event)
			if got != tt.want {
				t.Errorf("HasActionsForEvent(%q) = %v, want %v", tt.event, got, tt.want)
			}
		})
	}
}

func TestGetHooksConfigPaths_SecondaryWorktreeFallsBackToMain(t *testing.T) {
	// Regression: grove-created secondary worktrees get their own .grove
	// containing only a config.toml symlink (EnsureConfigSymlink), never
	// hooks.toml. FindRoot therefore resolves to the WORKTREE's .grove and
	// project hooks were silently skipped from inside any secondary worktree.
	// When hooks.toml is absent at the discovered root, the lookup must fall
	// through to the main worktree's .grove.
	mainDir, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}

	run := func(dir string, args ...string) {
		t.Helper()
		cmd := exec.Command(args[0], args[1:]...)
		cmd.Dir = dir
		cmd.Env = append(os.Environ(), "GIT_CONFIG_GLOBAL=/dev/null", "GIT_AUTHOR_NAME=test", "GIT_AUTHOR_EMAIL=test@test", "GIT_COMMITTER_NAME=test", "GIT_COMMITTER_EMAIL=test@test")
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("command %v failed: %v\n%s", args, err, out)
		}
	}
	run(mainDir, "git", "init")
	run(mainDir, "git", "commit", "--allow-empty", "-m", "init")

	// Main worktree has .grove with config.toml and hooks.toml.
	mainGrove := filepath.Join(mainDir, ".grove")
	if err := os.MkdirAll(mainGrove, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(mainGrove, "config.toml"), []byte(""), 0644); err != nil {
		t.Fatal(err)
	}
	mainHooks := filepath.Join(mainGrove, "hooks.toml")
	if err := os.WriteFile(mainHooks, []byte(""), 0644); err != nil {
		t.Fatal(err)
	}

	// Secondary worktree with a .grove holding only the config.toml symlink,
	// exactly as grove.EnsureConfigSymlink leaves it after `grove new`.
	wtDir := mainDir + "-wt"
	run(mainDir, "git", "worktree", "add", wtDir, "-b", "test-branch")
	defer func() { _ = os.RemoveAll(wtDir) }()
	wtGrove := filepath.Join(wtDir, ".grove")
	if err := os.MkdirAll(wtGrove, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(mainGrove, "config.toml"), filepath.Join(wtGrove, "config.toml")); err != nil {
		t.Fatal(err)
	}

	t.Chdir(wtDir)

	_, projectPath, err := GetHooksConfigPaths()
	if err != nil {
		t.Fatalf("GetHooksConfigPaths() error = %v", err)
	}
	if projectPath != mainHooks {
		t.Errorf("project hooks path = %q, want %q (main worktree's hooks.toml)", projectPath, mainHooks)
	}

	// A worktree-local hooks.toml, when present, must win over main's.
	wtHooks := filepath.Join(wtGrove, "hooks.toml")
	if err := os.WriteFile(wtHooks, []byte(""), 0644); err != nil {
		t.Fatal(err)
	}
	_, projectPath, err = GetHooksConfigPaths()
	if err != nil {
		t.Fatalf("GetHooksConfigPaths() error = %v", err)
	}
	if projectPath != wtHooks {
		t.Errorf("project hooks path = %q, want %q (worktree-local hooks.toml takes precedence)", projectPath, wtHooks)
	}
}

func TestGetHooksConfigPaths_DiscoversProjectRootFromSubdir(t *testing.T) {
	// Regression: without an explicit groveDir the project hooks.toml was
	// resolved against the bare cwd, so hooks silently didn't run from
	// subdirectories. The fallback must walk up like the rest of grove.
	tmpDir, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	// FindRoot only recognizes .grove inside a git work tree.
	gitCmd := exec.Command("git", "init")
	gitCmd.Dir = tmpDir
	gitCmd.Env = append(os.Environ(), "GIT_CONFIG_GLOBAL=/dev/null")
	if out, err := gitCmd.CombinedOutput(); err != nil {
		t.Fatalf("git init failed: %v\n%s", err, out)
	}
	groveDir := filepath.Join(tmpDir, ".grove")
	subDir := filepath.Join(tmpDir, "src", "nested")
	if err := os.MkdirAll(groveDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(subDir, 0755); err != nil {
		t.Fatal(err)
	}

	t.Chdir(subDir)

	_, projectPath, err := GetHooksConfigPaths()
	if err != nil {
		t.Fatalf("GetHooksConfigPaths() error = %v", err)
	}
	want := filepath.Join(groveDir, "hooks.toml")
	if projectPath != want {
		t.Errorf("project hooks path = %q, want %q (discovered from project root, not cwd)", projectPath, want)
	}
}
