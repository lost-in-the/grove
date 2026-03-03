package commands

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

func TestCheckEnvFileConfig_NonDefault(t *testing.T) {
	direnvFound := func(name string) (string, error) { return "/usr/bin/direnv", nil }
	direnvMissing := func(name string) (string, error) { return "", fmt.Errorf("not found") }

	tests := []struct {
		name          string
		envFileName   string
		envrcContent  string // "" means no .envrc file
		lookPath      func(string) (string, error)
		wantDirenv    bool
		wantEnvrc     bool
		wantLoads     bool
		wantDirenvErr bool
		wantEnvrcErr  bool
	}{
		{
			name:         "all good: direnv installed and envrc references file",
			envFileName:  ".env.local",
			envrcContent: "dotenv_if_exists .env.local",
			lookPath:     direnvFound,
			wantDirenv:   true,
			wantEnvrc:    true,
			wantLoads:    true,
		},
		{
			name:          "direnv not installed",
			envFileName:   ".env.local",
			envrcContent:  "dotenv_if_exists .env.local",
			lookPath:      direnvMissing,
			wantDirenv:    false,
			wantEnvrc:     true,
			wantLoads:     true,
			wantDirenvErr: true,
		},
		{
			name:         "envrc missing",
			envFileName:  ".env.local",
			envrcContent: "",
			lookPath:     direnvFound,
			wantDirenv:   true,
			wantEnvrc:    false,
			wantEnvrcErr: true,
		},
		{
			name:         "envrc exists but does not reference file",
			envFileName:  ".env.local",
			envrcContent: "layout ruby",
			lookPath:     direnvFound,
			wantDirenv:   true,
			wantEnvrc:    true,
			wantLoads:    false,
			wantEnvrcErr: true,
		},
		{
			name:         "custom env file name",
			envFileName:  ".env.grove",
			envrcContent: "dotenv_if_exists .env.grove",
			lookPath:     direnvFound,
			wantDirenv:   true,
			wantEnvrc:    true,
			wantLoads:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()

			if tt.envrcContent != "" {
				envrcPath := filepath.Join(tmpDir, ".envrc")
				if err := os.WriteFile(envrcPath, []byte(tt.envrcContent), 0644); err != nil {
					t.Fatal(err)
				}
			}

			result := checkEnvFileConfig(tt.envFileName, tmpDir, tt.lookPath)

			if result.direnvInstalled != tt.wantDirenv {
				t.Errorf("direnvInstalled = %v, want %v", result.direnvInstalled, tt.wantDirenv)
			}
			if result.envrcExists != tt.wantEnvrc {
				t.Errorf("envrcExists = %v, want %v", result.envrcExists, tt.wantEnvrc)
			}
			if result.envrcLoadsFile != tt.wantLoads {
				t.Errorf("envrcLoadsFile = %v, want %v", result.envrcLoadsFile, tt.wantLoads)
			}
			if (result.direnvErr != "") != tt.wantDirenvErr {
				t.Errorf("direnvErr = %q, wantErr = %v", result.direnvErr, tt.wantDirenvErr)
			}
			if (result.envrcErr != "") != tt.wantEnvrcErr {
				t.Errorf("envrcErr = %q, wantErr = %v", result.envrcErr, tt.wantEnvrcErr)
			}
		})
	}
}

func TestCheckEnvFileConfig_DefaultEnv(t *testing.T) {
	noopLookPath := func(name string) (string, error) { return "", nil }

	tests := []struct {
		name         string
		envrcContent string
		wantHint     bool
	}{
		{
			name:         "envrc with env.local support shows hint",
			envrcContent: "dotenv_if_exists .env.local",
			wantHint:     true,
		},
		{
			name:         "envrc without env.local support no hint",
			envrcContent: "layout ruby",
			wantHint:     false,
		},
		{
			name:         "no envrc no hint",
			envrcContent: "",
			wantHint:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()

			if tt.envrcContent != "" {
				envrcPath := filepath.Join(tmpDir, ".envrc")
				if err := os.WriteFile(envrcPath, []byte(tt.envrcContent), 0644); err != nil {
					t.Fatal(err)
				}
			}

			result := checkEnvFileConfig(".env", tmpDir, noopLookPath)

			if result.hintAvailable != tt.wantHint {
				t.Errorf("hintAvailable = %v, want %v", result.hintAvailable, tt.wantHint)
			}
			// Default mode should not set direnv/envrc fields
			if result.direnvInstalled {
				t.Error("direnvInstalled should be false in default mode")
			}
			if result.envrcExists {
				t.Error("envrcExists should be false in default mode")
			}
		})
	}
}
