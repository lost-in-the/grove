package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestKeyDefinedIn(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	content := "project_name = \"myapp\"\n\n[session]\nopen_in = \"here\"\n\n[plugins.docker]\nenabled = true\n"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	for key, want := range map[string]bool{
		"project_name":           true,
		"session.open_in":        true,
		"plugins.docker.enabled": true,
		"default_base_branch":    false,
		"switch.dirty_handling":  false,
		"plugins.docker.auto_up": false,
	} {
		got, err := KeyDefinedIn(path, key)
		if err != nil || got != want {
			t.Errorf("KeyDefinedIn(%q) = %v, %v; want %v", key, got, err, want)
		}
	}
	if got, err := KeyDefinedIn(filepath.Join(t.TempDir(), "missing.toml"), "project_name"); got || err != nil {
		t.Errorf("missing file: %v, %v; want false, nil", got, err)
	}
	bad := filepath.Join(t.TempDir(), "bad.toml")
	if err := os.WriteFile(bad, []byte("not = [toml"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := KeyDefinedIn(bad, "project_name"); err == nil {
		t.Error("a file that does not parse should be an error")
	}
}
