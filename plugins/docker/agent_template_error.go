package docker

import (
	"fmt"
	"regexp"
	"strings"
)

// unsetVariableRE matches docker compose's warning that a template variable
// referenced in the compose file has no value, e.g.:
//
//	warning: The "AGENT_MYAPP_DIR" variable is not set. Defaulting to a blank string.
var unsetVariableRE = regexp.MustCompile(`The "([^"]+)" variable is not set`)

// parseUnsetTemplateVars extracts the names of template variables that compose
// warned were unset (and so interpolated to an empty string), in first-seen
// order with duplicates removed. Returns nil when stderr contains no such
// warning.
func parseUnsetTemplateVars(stderr string) []string {
	matches := unsetVariableRE.FindAllStringSubmatch(stderr, -1)
	if len(matches) == 0 {
		return nil
	}
	seen := make(map[string]bool, len(matches))
	names := make([]string, 0, len(matches))
	for _, m := range matches {
		name := m[1]
		if seen[name] {
			continue
		}
		seen[name] = true
		names = append(names, name)
	}
	return names
}

// exportedEnvNames extracts just the variable names (before '=') from a slice
// of "NAME=value" env var strings. Used to report what grove exported without
// leaking values — those are worktree paths, but names alone are what's
// actionable here.
func exportedEnvNames(env []string) []string {
	names := make([]string, 0, len(env))
	for _, e := range env {
		if idx := strings.IndexByte(e, '='); idx >= 0 {
			names = append(names, e[:idx])
		} else {
			names = append(names, e)
		}
	}
	return names
}

// enrichAgentStackStartError wraps a failed 'agent stack up' with the names of
// any template variables compose warned were unset, plus the set of variable
// names grove exported for the run. Without this, an unset template variable
// (e.g. a template referencing ${AGENT_MYAPP_DIR} that grove never exports)
// surfaces as an opaque compose error like:
//
//	invalid spec: :/app:delegated: empty section between colons
//
// while the compose warning naming the actual unset variable scrolls past in
// the preceding output. When stderr contains no unset-variable warning, the
// original wrapping is preserved unchanged.
func enrichAgentStackStartError(stderr string, env []string, original error) error {
	unset := parseUnsetTemplateVars(stderr)
	if len(unset) == 0 {
		return fmt.Errorf("failed to start agent stack: %w", original)
	}
	return fmt.Errorf(
		"failed to start agent stack: template referenced variable(s) grove did not export: %s\n"+
			"grove exports: %s\n"+
			"underlying error: %w",
		strings.Join(unset, ", "), strings.Join(exportedEnvNames(env), ", "), original,
	)
}
