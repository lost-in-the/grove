package config

import (
	"strings"
	"testing"
)

func fileEntryConfig(ext *ExternalComposeConfig) *Config {
	return &Config{
		DefaultBranch: "main",
		Switch:        SwitchConfig{DirtyHandling: "prompt"},
		Plugins:       PluginsConfig{Docker: DockerPluginConfig{External: ext}},
	}
}

func TestValidate_FileEntries(t *testing.T) {
	tests := []struct {
		name    string
		ext     *ExternalComposeConfig
		wantErr string // substring; empty means valid
	}{
		{"literal paths pass", &ExternalComposeConfig{
			CopyFiles: []string{"config/secret.key", ".env.local"}, SymlinkFiles: []string{"config/dev.key"}, SymlinkDirs: []string{"node_modules"},
		}, ""},
		{"glob star in copy_files", &ExternalComposeConfig{CopyFiles: []string{"config/*.key"}}, `copy_files[0]: "config/*.key" has a glob character`},
		{"glob question in symlink_files", &ExternalComposeConfig{SymlinkFiles: []string{"a", "b?.key"}}, `symlink_files[1]: "b?.key" has a glob character`},
		{"glob bracket in copy_files", &ExternalComposeConfig{CopyFiles: []string{"key[0-9]"}}, "globs are not supported"},
		{"absolute copy_files", &ExternalComposeConfig{CopyFiles: []string{"/etc/hosts"}}, `copy_files[0]: "/etc/hosts" must be a relative path`},
		{"escaping symlink_dirs", &ExternalComposeConfig{SymlinkDirs: []string{"../outside"}}, `symlink_dirs[0]: "../outside" must stay inside the worktree`},
		{"empty entry", &ExternalComposeConfig{CopyFiles: []string{""}}, "must not be empty"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := Validate(fileEntryConfig(tt.ext))
			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("Validate() = %v, want nil", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("Validate() = %v, want error containing %q", err, tt.wantErr)
			}
			if !strings.Contains(err.Error(), "plugins.docker.external.") {
				t.Errorf("error %q should name the config key path", err)
			}
		})
	}
}
