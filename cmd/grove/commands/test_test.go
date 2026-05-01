package commands

import (
	"testing"
)

// resolveTestOptions composes effective test options from CLI flags layered over config.
// Unit-test the resolution logic without invoking docker.
func TestResolveTestOptions_FlagOverridesConfig(t *testing.T) {
	tests := []struct {
		name           string
		cfgIncludeDeps bool
		cfgBindMount   string
		flagWithDeps   bool
		flagBind       string
		wantSkipDeps   bool
		wantBindMount  string
	}{
		{"defaults", false, "", false, "", true, ""},
		{"config opts in to deps", true, "", false, "", false, ""},
		{"flag overrides config off", false, "", true, "", false, ""},
		{"flag bind overrides empty config", false, "", false, "/app", true, "/app"},
		{"flag bind overrides config bind", false, "/old", false, "/new", true, "/new"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			opts := resolveTestOptions(tc.cfgIncludeDeps, tc.cfgBindMount, tc.flagWithDeps, tc.flagBind)
			if opts.SkipDeps != tc.wantSkipDeps {
				t.Errorf("SkipDeps: got %v want %v", opts.SkipDeps, tc.wantSkipDeps)
			}
			if opts.BindMount != tc.wantBindMount {
				t.Errorf("BindMount: got %q want %q", opts.BindMount, tc.wantBindMount)
			}
		})
	}
}
