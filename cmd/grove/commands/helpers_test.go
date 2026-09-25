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
