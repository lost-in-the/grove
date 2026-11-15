package commands

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lost-in-the/grove/internal/cli"
	"github.com/lost-in-the/grove/internal/config"
	"github.com/lost-in-the/grove/internal/hooks"
	"github.com/lost-in-the/grove/internal/mux"
	"github.com/lost-in-the/grove/internal/worktree"
)

func TestRunFileSetup_NilConfig(t *testing.T) {
	w := cli.NewWriter(&bytes.Buffer{}, false)
	runFileSetup(nil, "/tmp/new", "/tmp/main", w, false)
	// No panic, no action — passing means the nil-guard works.
}

func TestRunFileSetup_NoExternalConfig(t *testing.T) {
	cfg := &config.Config{} // Plugins.Docker.External is nil
	w := cli.NewWriter(&bytes.Buffer{}, false)
	runFileSetup(cfg, "/tmp/new", "/tmp/main", w, false)
	// No panic, no action.
}

func TestRunFileSetup_CopiesFiles(t *testing.T) {
	mainPath := t.TempDir()
	newPath := t.TempDir()

	if err := os.MkdirAll(filepath.Join(mainPath, "config"), 0755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	if err := os.WriteFile(filepath.Join(mainPath, "config", "dev.key"), []byte("devkey"), 0600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	cfg := &config.Config{
		Plugins: config.PluginsConfig{
			Docker: config.DockerPluginConfig{
				External: &config.ExternalComposeConfig{
					CopyFiles: []string{"config/dev.key"},
				},
			},
		},
	}

	var buf bytes.Buffer
	w := cli.NewWriter(&buf, false)
	runFileSetup(cfg, newPath, mainPath, w, false)

	data, err := os.ReadFile(filepath.Join(newPath, "config", "dev.key"))
	if err != nil {
		t.Fatalf("expected file to be copied: %v", err)
	}
	if string(data) != "devkey" {
		t.Errorf("file contents = %q, want %q", string(data), "devkey")
	}
}

func TestRunFileSetup_WarnsOnError(t *testing.T) {
	mainPath := t.TempDir()
	newPath := t.TempDir()

	cfg := &config.Config{
		Plugins: config.PluginsConfig{
			Docker: config.DockerPluginConfig{
				External: &config.ExternalComposeConfig{
					CopyFiles: []string{"nonexistent.key"},
				},
			},
		},
	}

	var buf bytes.Buffer
	w := cli.NewWriter(&buf, false)
	runFileSetup(cfg, newPath, mainPath, w, false)

	if !strings.Contains(buf.String(), "File setup had issues") {
		t.Errorf("expected warning in output, got %q", buf.String())
	}
}

func TestRunFileSetup_SuppressesWarningInJSON(t *testing.T) {
	mainPath := t.TempDir()
	newPath := t.TempDir()

	cfg := &config.Config{
		Plugins: config.PluginsConfig{
			Docker: config.DockerPluginConfig{
				External: &config.ExternalComposeConfig{
					CopyFiles: []string{"nonexistent.key"},
				},
			},
		},
	}

	var buf bytes.Buffer
	w := cli.NewWriter(&buf, false)
	runFileSetup(cfg, newPath, mainPath, w, true)

	if buf.String() != "" {
		t.Errorf("expected no output in JSON mode, got %q", buf.String())
	}
}

// preparingMux is a backend whose attach command cannot name its target
// (herdr): the hint only works once PrepareAttach has focused it.
type preparingMux struct {
	*mux.OffBackend
	prepared []mux.Target
}

func (p *preparingMux) AttachHint(mux.Target) string { return "herdr --session other" }
func (p *preparingMux) PrepareAttach(t mux.Target) error {
	p.prepared = append(p.prepared, t)
	return nil
}

// Under shell integration grove prints an attach hint instead of attaching.
// Printing it without focusing the target first sent the user to whatever
// workspace herdr last had focused.
func TestAttachToSessionPreparesTheTargetBeforeHinting(t *testing.T) {
	m := &preparingMux{OffBackend: mux.NewOff()}
	target := mux.Target{Name: "grove-testing", Path: "/repos/grove-testing"}

	if err := attachToSession(m, target, nil, true, cli.NewStderr()); err != nil {
		t.Fatalf("attachToSession() error = %v", err)
	}
	if len(m.prepared) != 1 || m.prepared[0].Path != target.Path {
		t.Errorf("PrepareAttach calls = %v, want exactly the target", m.prepared)
	}
}

func TestWillSwitchAfterCreate(t *testing.T) {
	tests := []struct {
		name     string
		noSwitch bool
		json     bool
		want     bool
	}{
		{"plain", false, false, true},
		{"--no-switch", true, false, false},
		{"--json returns without switching", false, true, false},
		{"--json --no-switch", true, true, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := willSwitchAfterCreate(tt.noSwitch, tt.json); got != tt.want {
				t.Errorf("willSwitchAfterCreate(noSwitch=%v, json=%v) = %v, want %v", tt.noSwitch, tt.json, got, tt.want)
			}
		})
	}
}

func TestOpenSetupOpts_JSONDoesNotSwitch(t *testing.T) {
	if openSetupOpts(false, true).WillSwitch {
		t.Error("open --json must reach post-create hooks as non-switching")
	}
	if !openSetupOpts(false, false).WillSwitch {
		t.Error("plain open ends in the worktree, so it must be switching")
	}
}

func TestSetupCreatedWorktree_ForwardsSwitchIntentToPostCreateHook(t *testing.T) {
	tests := []struct {
		name string
		opts worktreeSetupOpts
		want bool
	}{
		{"zero value does not switch", worktreeSetupOpts{}, false},
		{"switching", worktreeSetupOpts{WillSwitch: true}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root := t.TempDir()
			repoRoot := filepath.Join(root, "myapp")
			groveDir := filepath.Join(repoRoot, ".grove")
			if err := os.MkdirAll(groveDir, 0o755); err != nil {
				t.Fatal(err)
			}
			mgr, err := worktree.NewManager(repoRoot)
			if err != nil {
				t.Fatal(err)
			}
			wtPath := mgr.PathForName("feature")
			if err := os.MkdirAll(wtPath, 0o755); err != nil {
				t.Fatal(err)
			}

			var got *hooks.Context
			hooks.Register(hooks.EventPostCreate, func(c *hooks.Context) error {
				if c.WorktreePath == wtPath {
					got = c
				}
				return nil
			})

			ctx := &GroveContext{GroveDir: groveDir, ProjectRoot: repoRoot, Config: config.LoadDefaults()}
			w := cli.NewWriter(&bytes.Buffer{}, false)
			if _, err := setupCreatedWorktree(ctx, mgr, "feature", "feature", tt.opts, w); err != nil {
				t.Fatalf("setupCreatedWorktree: %v", err)
			}
			if got == nil {
				t.Fatal("post-create hook did not fire")
			}
			if got.WillSwitch != tt.want {
				t.Errorf("hook ctx WillSwitch = %v, want %v", got.WillSwitch, tt.want)
			}
		})
	}
}
