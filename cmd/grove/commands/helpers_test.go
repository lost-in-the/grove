package commands

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lost-in-the/grove/internal/cli"
	"github.com/lost-in-the/grove/internal/config"
	"github.com/lost-in-the/grove/internal/mux"
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

func TestWantsStackUp(t *testing.T) {
	yes, no := true, false
	withAutoUp := func(v *bool) *config.Config {
		cfg := &config.Config{}
		cfg.Plugins.Docker.AutoUp = v
		return cfg
	}
	tests := []struct {
		name     string
		cfg      *config.Config
		up       bool
		noDocker bool
		want     bool
	}{
		{name: "nothing set", cfg: withAutoUp(nil), want: false},
		{name: "auto_up", cfg: withAutoUp(&yes), want: true},
		{name: "auto_up false", cfg: withAutoUp(&no), want: false},
		{name: "--up", cfg: withAutoUp(nil), up: true, want: true},
		{name: "--up beats auto_up false", cfg: withAutoUp(&no), up: true, want: true},
		{name: "--no-docker beats auto_up", cfg: withAutoUp(&yes), noDocker: true, want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := wantsStackUp(tt.cfg, tt.up, tt.noDocker); got != tt.want {
				t.Errorf("wantsStackUp() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestNewCmdUpFlag(t *testing.T) {
	if newCmd.Flags().Lookup("up") == nil {
		t.Fatal("grove new has no --up flag")
	}
}
