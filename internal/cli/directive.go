package cli

import (
	"fmt"
	"os"
)

// Directive emits a shell directive to stdout. Directives are NEVER styled
// and must contain zero ANSI escape sequences — the shell wrapper parses
// these lines literally.
//
// Valid kinds: "cd", "tmux-attach", "env"
func Directive(kind, value string) {
	_, _ = fmt.Fprintf(os.Stdout, "%s:%s\n", kind, value)
}

// EnvDirective emits an env: directive that the shell wrapper will export.
// Only emitted when running under shell integration (GROVE_SHELL=1).
func EnvDirective(key, value string) {
	if !IsShellIntegration() {
		return
	}
	Directive("env", key+"="+value)
}

// IsShellIntegration returns true when running under the grove shell wrapper.
func IsShellIntegration() bool {
	return os.Getenv("GROVE_SHELL") == "1"
}
