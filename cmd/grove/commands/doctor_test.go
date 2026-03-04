package commands

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

func TestCheckEnvFileConfig_NonDefault(t *testing.T) {
	direnvFound := func(name string) (string, error) {
		if name == "direnv" {
			return "/usr/bin/direnv", nil
		}
		return "", fmt.Errorf("not found")
	}
	miseFound := func(name string) (string, error) {
		if name == "mise" {
			return "/usr/bin/mise", nil
		}
		return "", fmt.Errorf("not found")
	}
	bothFound := func(name string) (string, error) {
		if name == "direnv" {
			return "/usr/bin/direnv", nil
		}
		if name == "mise" {
			return "/usr/bin/mise", nil
		}
		return "", fmt.Errorf("not found")
	}
	neitherFound := func(name string) (string, error) { return "", fmt.Errorf("not found") }

	tests := []struct {
		name           string
		envFileName    string
		envrcContent   string // "" means no .envrc file
		miseContent    string // "" means no .mise.toml file
		lookPath       func(string) (string, error)
		wantLoader     bool
		wantLoaderName string
		wantConfig     bool
		wantLoads      bool
		wantLoaderErr  bool
		wantConfigErr  bool
	}{
		{
			name:           "direnv installed and envrc references file",
			envFileName:    ".env.local",
			envrcContent:   "dotenv_if_exists .env.local",
			lookPath:       direnvFound,
			wantLoader:     true,
			wantLoaderName: "direnv",
			wantConfig:     true,
			wantLoads:      true,
		},
		{
			name:           "mise installed and mise.toml references file",
			envFileName:    ".env.local",
			miseContent:    "[env]\nfile = \".env.local\"",
			lookPath:       miseFound,
			wantLoader:     true,
			wantLoaderName: "mise",
			wantConfig:     true,
			wantLoads:      true,
		},
		{
			name:           "both installed, direnv preferred",
			envFileName:    ".env.local",
			envrcContent:   "dotenv_if_exists .env.local",
			lookPath:       bothFound,
			wantLoader:     true,
			wantLoaderName: "direnv",
			wantConfig:     true,
			wantLoads:      true,
		},
		{
			name:          "neither direnv nor mise installed",
			envFileName:   ".env.local",
			envrcContent:  "dotenv_if_exists .env.local",
			lookPath:      neitherFound,
			wantLoader:    false,
			wantConfig:    true,
			wantLoads:     true,
			wantLoaderErr: true,
		},
		{
			name:           "direnv installed but no config files",
			envFileName:    ".env.local",
			lookPath:       direnvFound,
			wantLoader:     true,
			wantLoaderName: "direnv",
			wantConfig:     false,
			wantConfigErr:  true,
		},
		{
			name:           "envrc exists but does not reference file",
			envFileName:    ".env.local",
			envrcContent:   "layout ruby",
			lookPath:       direnvFound,
			wantLoader:     true,
			wantLoaderName: "direnv",
			wantConfig:     true,
			wantLoads:      false,
			wantConfigErr:  true,
		},
		{
			name:           "mise installed with mise.toml not referencing file",
			envFileName:    ".env.local",
			miseContent:    "[tools]\nnode = \"20\"",
			lookPath:       miseFound,
			wantLoader:     true,
			wantLoaderName: "mise",
			wantConfig:     true,
			wantLoads:      false,
			wantConfigErr:  true,
		},
		{
			name:           "custom env file name with direnv",
			envFileName:    ".env.grove",
			envrcContent:   "dotenv_if_exists .env.grove",
			lookPath:       direnvFound,
			wantLoader:     true,
			wantLoaderName: "direnv",
			wantConfig:     true,
			wantLoads:      true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()

			if tt.envrcContent != "" {
				if err := os.WriteFile(filepath.Join(tmpDir, ".envrc"), []byte(tt.envrcContent), 0644); err != nil {
					t.Fatal(err)
				}
			}
			if tt.miseContent != "" {
				if err := os.WriteFile(filepath.Join(tmpDir, ".mise.toml"), []byte(tt.miseContent), 0644); err != nil {
					t.Fatal(err)
				}
			}

			result := checkEnvFileConfig(tt.envFileName, tmpDir, tt.lookPath)

			if result.loaderInstalled != tt.wantLoader {
				t.Errorf("loaderInstalled = %v, want %v", result.loaderInstalled, tt.wantLoader)
			}
			if result.loaderName != tt.wantLoaderName {
				t.Errorf("loaderName = %q, want %q", result.loaderName, tt.wantLoaderName)
			}
			if result.configExists != tt.wantConfig {
				t.Errorf("configExists = %v, want %v", result.configExists, tt.wantConfig)
			}
			if result.configLoadsFile != tt.wantLoads {
				t.Errorf("configLoadsFile = %v, want %v", result.configLoadsFile, tt.wantLoads)
			}
			if (result.loaderErr != "") != tt.wantLoaderErr {
				t.Errorf("loaderErr = %q, wantErr = %v", result.loaderErr, tt.wantLoaderErr)
			}
			if (result.configErr != "") != tt.wantConfigErr {
				t.Errorf("configErr = %q, wantErr = %v", result.configErr, tt.wantConfigErr)
			}
		})
	}
}

func TestCheckEnvFileConfig_DefaultEnv(t *testing.T) {
	noopLookPath := func(name string) (string, error) { return "", nil }

	tests := []struct {
		name         string
		envrcContent string
		miseContent  string
		wantHint     bool
	}{
		{
			name:         "envrc with env.local support shows hint",
			envrcContent: "dotenv_if_exists .env.local",
			wantHint:     true,
		},
		{
			name:        "mise.toml with env.local support shows hint",
			miseContent: "[env]\nfile = \".env.local\"",
			wantHint:    true,
		},
		{
			name:         "envrc without env.local support no hint",
			envrcContent: "layout ruby",
			wantHint:     false,
		},
		{
			name:         "no config files no hint",
			envrcContent: "",
			wantHint:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()

			if tt.envrcContent != "" {
				if err := os.WriteFile(filepath.Join(tmpDir, ".envrc"), []byte(tt.envrcContent), 0644); err != nil {
					t.Fatal(err)
				}
			}
			if tt.miseContent != "" {
				if err := os.WriteFile(filepath.Join(tmpDir, ".mise.toml"), []byte(tt.miseContent), 0644); err != nil {
					t.Fatal(err)
				}
			}

			result := checkEnvFileConfig(".env", tmpDir, noopLookPath)

			if result.hintAvailable != tt.wantHint {
				t.Errorf("hintAvailable = %v, want %v", result.hintAvailable, tt.wantHint)
			}
			if result.loaderInstalled {
				t.Error("loaderInstalled should be false in default mode")
			}
			if result.configExists {
				t.Error("configExists should be false in default mode")
			}
		})
	}
}
