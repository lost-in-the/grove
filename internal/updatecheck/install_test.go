package updatecheck

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDetectInstallFromPath(t *testing.T) {
	cases := []struct {
		name string
		path string
		want InstallMethod
	}{
		{"homebrew apple silicon", "/opt/homebrew/Cellar/grove/0.6.0/bin/grove", InstallBrew},
		{"homebrew intel", "/usr/local/Cellar/grove/0.6.0/bin/grove", InstallBrew},
		{"go install", "/Users/leah/go/bin/grove", InstallGoInstall},
		{"binary download", "/usr/local/bin/grove", InstallBinary},
		{"empty", "", InstallUnknown},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := detectInstallFromPath(tc.path); got != tc.want {
				t.Errorf("detectInstallFromPath(%q) = %v, want %v", tc.path, got, tc.want)
			}
		})
	}
}

func TestDetectInstall_ResolvesSymlinks(t *testing.T) {
	dir := t.TempDir()
	// Simulate brew layout: bin/grove → Cellar/grove/0.6.0/bin/grove
	cellarDir := filepath.Join(dir, "Cellar", "grove", "0.6.0", "bin")
	if err := os.MkdirAll(cellarDir, 0o755); err != nil {
		t.Fatal(err)
	}
	cellarBin := filepath.Join(cellarDir, "grove")
	if err := os.WriteFile(cellarBin, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	binDir := filepath.Join(dir, "bin")
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(binDir, "grove")
	if err := os.Symlink(cellarBin, link); err != nil {
		t.Fatal(err)
	}

	// Direct test of detection on the resolved path (the public DetectInstall calls os.Executable
	// which we can't easily mock, so we verify the resolve+detect contract by hand).
	resolved, err := filepath.EvalSymlinks(link)
	if err != nil {
		t.Fatalf("EvalSymlinks: %v", err)
	}
	got := detectInstallFromPath(resolved)
	if got != InstallBrew {
		t.Errorf("detectInstallFromPath(resolved symlink) = %v, want InstallBrew (resolved path: %s)", got, resolved)
	}
}

func TestUpdateCommand(t *testing.T) {
	cases := []struct {
		method InstallMethod
		want   string
	}{
		{InstallBrew, "brew upgrade lost-in-the/tap/grove"},
		{InstallGoInstall, "go install github.com/lost-in-the/grove/cmd/grove@latest"},
		{InstallBinary, "Visit https://github.com/lost-in-the/grove/releases for the latest binary"},
		{InstallUnknown, "Visit https://github.com/lost-in-the/grove/releases for the latest binary"},
	}
	for _, tc := range cases {
		t.Run(tc.method.String(), func(t *testing.T) {
			if got := UpdateCommand(tc.method); got != tc.want {
				t.Errorf("UpdateCommand(%v) = %q, want %q", tc.method, got, tc.want)
			}
		})
	}
}
