package cli

import (
	"fmt"
	"os"
)

// Directive emits a shell directive to stdout. Directives are NEVER styled
// and must contain zero ANSI escape sequences — the shell wrapper parses
// these lines literally.
//
// Valid kinds: "cd", "tmux-attach", "tmux-attach-cc", "env"
func Directive(kind, value string) {
	_, _ = fmt.Fprintf(os.Stdout, "%s:%s\n", kind, value)
}

// CdDirective requests a directory change through whichever channel the shell
// wrapper is listening on. Commands the wrapper runs WITHOUT output capture —
// the bare-`grove` TUI and the issue/PR browsers — get GROVE_CD_FILE set and
// read the target from that file; capture-based commands (to, new, …) have no
// such file and receive a cd: line on stdout. Preferring the file when present
// keeps the directive off the terminal for the un-captured commands (B27).
// Returns true when a directive was emitted.
func CdDirective(path string) bool {
	if cdFile := os.Getenv("GROVE_CD_FILE"); cdFile != "" {
		// The wrapper mktemps this file before launching grove (see
		// SHELL_INTEGRATION), so it must already exist. Open WITHOUT O_CREATE: a
		// set-but-missing path means a stale value leaked into this process's env
		// — e.g. a tmux server started by `grove prs` whose temp file was since
		// removed — and recreating it would silently write the cd target to a
		// file no wrapper is reading. On a missing/unwritable file, fall through
		// to the stdout cd: line the capture-based commands already use.
		if f, err := os.OpenFile(cdFile, os.O_WRONLY|os.O_TRUNC, 0600); err == nil {
			_, werr := f.WriteString(path)
			cerr := f.Close()
			if werr == nil && cerr == nil {
				return true
			}
		}
	}
	if IsShellIntegration() {
		Directive("cd", path)
		return true
	}
	return false
}

// TmuxAttachDirective emits the appropriate tmux attach directive based on control mode.
func TmuxAttachDirective(sessionName string, controlMode bool) {
	if controlMode {
		Directive("tmux-attach-cc", sessionName)
	} else {
		Directive("tmux-attach", sessionName)
	}
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
