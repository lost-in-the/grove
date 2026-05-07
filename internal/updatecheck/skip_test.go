package updatecheck

import (
	"testing"
)

func TestSkip(t *testing.T) {
	cases := []struct {
		name        string
		env         map[string]string
		flag        bool
		version     string
		stdoutIsTTY bool
		want        bool
	}{
		{"happy path", nil, false, "0.6.0", true, false},
		{"CI=true", map[string]string{"CI": "true"}, false, "0.6.0", true, true},
		{"GITHUB_ACTIONS=true", map[string]string{"GITHUB_ACTIONS": "true"}, false, "0.6.0", true, true},
		{"BUILDKITE=true", map[string]string{"BUILDKITE": "true"}, false, "0.6.0", true, true},
		{"CIRCLECI=true", map[string]string{"CIRCLECI": "true"}, false, "0.6.0", true, true},
		{"TRAVIS=true", map[string]string{"TRAVIS": "true"}, false, "0.6.0", true, true},
		{"GROVE_AGENT_MODE=1", map[string]string{"GROVE_AGENT_MODE": "1"}, false, "0.6.0", true, true},
		{"GROVE_NONINTERACTIVE=1", map[string]string{"GROVE_NONINTERACTIVE": "1"}, false, "0.6.0", true, true},
		{"NO_UPDATE_NOTIFIER=1", map[string]string{"NO_UPDATE_NOTIFIER": "1"}, false, "0.6.0", true, true},
		{"GROVE_NO_UPDATE_NOTIFIER=1", map[string]string{"GROVE_NO_UPDATE_NOTIFIER": "1"}, false, "0.6.0", true, true},
		{"flag --no-update-notifier", nil, true, "0.6.0", true, true},
		{"non-TTY stdout", nil, false, "0.6.0", false, true},
		{"version unknown", nil, false, "unknown", true, true},
		{"version -dev suffix", nil, false, "0.7.0-dev", true, true},
		{"version non-semver", nil, false, "abc", true, true},
		{"version with v prefix", nil, false, "v0.6.0", true, false}, // tolerated
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := skipWithDeps(tc.env, tc.flag, tc.version, tc.stdoutIsTTY)
			if got != tc.want {
				t.Errorf("Skip() = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestSkip_PublicIntegration(t *testing.T) {
	// Cover the public Skip() function which reads os.Getenv directly.
	// Force-clear the env vars Skip checks so the happy path can pass when CI=true is set.
	for _, k := range []string{
		"CI", "GITHUB_ACTIONS", "BUILDKITE", "CIRCLECI", "TRAVIS",
		"GROVE_AGENT_MODE", "GROVE_NONINTERACTIVE",
		"NO_UPDATE_NOTIFIER", "GROVE_NO_UPDATE_NOTIFIER",
	} {
		t.Setenv(k, "")
	}

	// With env cleared and a real released version, Skip should be true only if
	// stdout is not a TTY (which it isn't under `go test`). So this exercises the
	// TTY branch in the public function.
	if !Skip(false, "0.6.0") {
		t.Error("Skip should return true under `go test` (stdout is not a TTY)")
	}

	// Setting the flag still suppresses regardless of TTY.
	if !Skip(true, "0.6.0") {
		t.Error("Skip should return true when --no-update-notifier flag is set")
	}

	// Setting GROVE_NO_UPDATE_NOTIFIER suppresses too.
	t.Setenv("GROVE_NO_UPDATE_NOTIFIER", "1")
	if !Skip(false, "0.6.0") {
		t.Error("Skip should return true when GROVE_NO_UPDATE_NOTIFIER=1")
	}
}
