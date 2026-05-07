package updatecheck

import (
	"os"
	"strings"

	"golang.org/x/term"
)

// skipEnvVars are the env vars that, when non-empty, suppress update notifications.
// CI vars first, then grove-specific opt-out, then the npm convention.
var skipEnvVars = []string{
	"CI", "GITHUB_ACTIONS", "BUILDKITE", "CIRCLECI", "TRAVIS",
	"GROVE_AGENT_MODE", "GROVE_NONINTERACTIVE",
	"NO_UPDATE_NOTIFIER", "GROVE_NO_UPDATE_NOTIFIER",
}

// Skip returns true when update checking should be entirely suppressed.
// Honors CI env vars, grove-specific opt-out, the --no-update-notifier flag,
// non-TTY stdout, and dev/unknown/non-semver versions.
func Skip(noUpdateNotifierFlag bool, currentVersion string) bool {
	env := map[string]string{}
	for _, k := range skipEnvVars {
		if v := os.Getenv(k); v != "" {
			env[k] = v
		}
	}
	stdoutIsTTY := term.IsTerminal(int(os.Stdout.Fd()))
	return skipWithDeps(env, noUpdateNotifierFlag, currentVersion, stdoutIsTTY)
}

// skipWithDeps is the testable core of Skip — pure function over its inputs.
func skipWithDeps(env map[string]string, flag bool, version string, stdoutIsTTY bool) bool {
	if flag {
		return true
	}
	for _, k := range skipEnvVars {
		if env[k] != "" {
			return true
		}
	}
	if !stdoutIsTTY {
		return true
	}
	return !isReleasedVersion(version)
}

// isReleasedVersion returns true for versions that look like real releases
// (parseable semver, no -dev suffix, not "unknown"). A leading "v" is tolerated.
func isReleasedVersion(v string) bool {
	if v == "" || v == "unknown" {
		return false
	}
	if strings.Contains(v, "-dev") {
		return false
	}
	_, ok := parseSemver(v)
	return ok
}
