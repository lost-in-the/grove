package commands

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lost-in-the/grove/internal/cli"
	"github.com/lost-in-the/grove/internal/config"
	"github.com/lost-in-the/grove/internal/grove"
)

func TestEmitDriftNotice_NamesMissingFiles(t *testing.T) {
	var buf bytes.Buffer
	w := cli.NewWriter(&buf, false)

	emitDriftNotice(w, "drifted-wt", grove.ReasonDriftedWorktree, []string{"config/secret.key", ".env.local"})

	out := buf.String()
	for _, want := range []string{
		"2 configured files are missing here: config/secret.key, .env.local",
		"grove adopt",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("notice missing %q, got:\n%s", want, out)
		}
	}
}

func TestEmitDriftNotice_OneFileAndLongLists(t *testing.T) {
	var one bytes.Buffer
	emitDriftNotice(cli.NewWriter(&one, false), "wt", grove.ReasonDriftedWorktree, []string{".env.local"})
	if !strings.Contains(one.String(), "1 configured file is missing here: .env.local") {
		t.Errorf("singular form wrong:\n%s", one.String())
	}

	var many bytes.Buffer
	names := []string{"a", "b", "c", "d", "e", "f", "g"}
	emitDriftNotice(cli.NewWriter(&many, false), "wt", grove.ReasonDriftedWorktree, names)
	if !strings.Contains(many.String(), "7 configured files are missing here: a, b, c, d, e, and 2 more") {
		t.Errorf("long list not capped:\n%s", many.String())
	}
}

func TestEmitDriftNotice_NoFilesLineWhenNothingMissing(t *testing.T) {
	var buf bytes.Buffer
	emitDriftNotice(cli.NewWriter(&buf, false), "wt", grove.ReasonDriftedWorktree, nil)
	if strings.Contains(buf.String(), "missing here") {
		t.Errorf("unexpected files line:\n%s", buf.String())
	}
}

func TestMissingConfiguredFiles(t *testing.T) {
	main := t.TempDir()
	wt := t.TempDir()
	if err := os.WriteFile(filepath.Join(wt, "present.key"), []byte("k"), 0o600); err != nil {
		t.Fatal(err)
	}
	ext := &config.ExternalComposeConfig{
		CopyFiles:   []string{"present.key", "config/secret.key"},
		SymlinkDirs: []string{"node_modules"},
	}
	cfg := config.LoadDefaults()
	cfg.Plugins.Docker.External = ext

	got := missingConfiguredFiles(cfg, main, wt)
	if strings.Join(got, ",") != "config/secret.key,node_modules" {
		t.Errorf("missing = %v, want [config/secret.key node_modules]", got)
	}

	if got := missingConfiguredFiles(config.LoadDefaults(), main, wt); len(got) != 0 {
		t.Errorf("no external config: missing = %v, want none", got)
	}
	if got := missingConfiguredFiles(nil, main, wt); len(got) != 0 {
		t.Errorf("nil config: missing = %v, want none", got)
	}
}
