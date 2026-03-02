package cli

import (
	"fmt"
	"os"
)

// Directive emits a shell directive to stdout. Directives are NEVER styled
// and must contain zero ANSI escape sequences — the shell wrapper parses
// these lines literally.
//
// Valid kinds: "cd", "tmux-attach"
func Directive(kind, value string) {
	_, _ = fmt.Fprintf(os.Stdout, "%s:%s\n", kind, value)
}
