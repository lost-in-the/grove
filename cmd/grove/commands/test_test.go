package commands

import (
	"testing"

	"github.com/lost-in-the/grove/internal/config"
)

// resolveTestOptions composes effective test options from CLI flags layered over config.
// Unit-test the resolution logic without invoking docker.
func TestResolveTestOptions_FlagOverridesConfig(t *testing.T) {
	trueVal := true
	falseVal := false

	tests := []struct {
		name            string
		cfg             config.TestConfig
		flagWithDeps    bool
		flagBind        string
		wantIncludeDeps bool
		wantBindMount   string
	}{
		{"defaults", config.TestConfig{}, false, "", false, ""},
		{"config opts in to deps", config.TestConfig{IncludeDeps: &trueVal}, false, "", true, ""},
		{"config false explicit", config.TestConfig{IncludeDeps: &falseVal}, false, "", false, ""},
		{"flag overrides config off", config.TestConfig{}, true, "", true, ""},
		{"flag overrides config nil", config.TestConfig{IncludeDeps: nil}, true, "", true, ""},
		{"flag bind overrides empty config", config.TestConfig{}, false, "/app", false, "/app"},
		{"flag bind overrides config bind", config.TestConfig{BindMount: "/old"}, false, "/new", false, "/new"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			includeDeps, bindMount := resolveTestOptions(tc.cfg, tc.flagWithDeps, tc.flagBind)
			if includeDeps != tc.wantIncludeDeps {
				t.Errorf("IncludeDeps: got %v want %v", includeDeps, tc.wantIncludeDeps)
			}
			if bindMount != tc.wantBindMount {
				t.Errorf("BindMount: got %q want %q", bindMount, tc.wantBindMount)
			}
		})
	}
}
