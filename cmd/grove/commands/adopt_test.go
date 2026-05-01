package commands

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestResolveAdoptTarget_UsesCwdWhenNoArg(t *testing.T) {
	tmpDir := t.TempDir()
	got, err := resolveAdoptTarget(tmpDir, []string{})
	if err != nil {
		t.Fatalf("resolveAdoptTarget: %v", err)
	}
	expected, _ := filepath.EvalSymlinks(tmpDir)
	if expected == "" {
		expected = tmpDir
	}
	if got != expected {
		t.Errorf("got %q want %q", got, expected)
	}
}

func TestResolveAdoptTarget_UsesArgWhenProvided(t *testing.T) {
	tmpDir := t.TempDir()
	other := filepath.Join(tmpDir, "other")
	if err := os.MkdirAll(other, 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	got, err := resolveAdoptTarget(tmpDir, []string{other})
	if err != nil {
		t.Fatalf("resolveAdoptTarget: %v", err)
	}
	expected, _ := filepath.EvalSymlinks(other)
	if expected == "" {
		expected = other
	}
	if got != expected {
		t.Errorf("got %q want %q", got, expected)
	}
}

func TestResolveAdoptTarget_ErrorsOnNonexistent(t *testing.T) {
	tmpDir := t.TempDir()
	_, err := resolveAdoptTarget(tmpDir, []string{filepath.Join(tmpDir, "nope")})
	if err == nil {
		t.Errorf("expected error for nonexistent path")
	}
}

func TestAdopt_StripProjectPrefixForName(t *testing.T) {
	tests := []struct {
		name        string
		dirBase     string
		projectName string
		want        string
	}{
		{"strips matching prefix", "grove-feature", "grove", "feature"},
		{"no prefix when project doesn't match", "myproj-feature", "grove", "myproj-feature"},
		{"no prefix when name equals project", "grove", "grove", "grove"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.dirBase
			if prefix := tt.projectName + "-"; strings.HasPrefix(got, prefix) {
				got = strings.TrimPrefix(got, prefix)
			}
			if got != tt.want {
				t.Errorf("got %q want %q", got, tt.want)
			}
		})
	}
}
