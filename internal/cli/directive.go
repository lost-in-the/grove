package cli

import (
	"fmt"
	"os"
	"strings"
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
	if f, ok := openCdFile(os.O_TRUNC); ok {
		_, werr := f.WriteString(path)
		cerr := f.Close()
		if werr == nil && cerr == nil {
			return true
		}
	}
	if IsShellIntegration() {
		Directive("cd", path)
		return true
	}
	return false
}

// CanCd reports whether CdDirective would reach a shell wrapper, without
// writing anything. False means a directory change cannot happen.
func CanCd() bool {
	if f, ok := openCdFile(0); ok {
		_ = f.Close()
		return true
	}
	return IsShellIntegration()
}

// openCdFile opens the wrapper's GROVE_CD_FILE for writing, with extra open
// flags. The wrapper mktemps this file before launching grove (see
// SHELL_INTEGRATION), so it must already exist. Open WITHOUT O_CREATE: a
// set-but-missing path means a stale value leaked into this process's env
// (for example, a tmux server started by `grove prs` whose temp file was
// since removed), and recreating it would silently write the cd target to a
// file no wrapper is reading. On a missing/unwritable file, callers fall
// through to the stdout cd: line the capture-based commands already use.
func openCdFile(flags int) (*os.File, bool) {
	cdFile := os.Getenv("GROVE_CD_FILE")
	if cdFile == "" {
		return nil, false
	}
	f, err := os.OpenFile(cdFile, os.O_WRONLY|flags, 0600)
	if err != nil {
		return nil, false
	}
	return f, true
}

// ExplainNoCd tells the user grove could not change their directory to
// path: why, how to load shell integration, and the cd to run now. Every
// command that would move the user prints it when CdDirective fails.
func ExplainNoCd(w *Writer, path string) {
	explainNoCd(w, path, "grove could not change your directory: shell integration is not loaded.")
}

// ExplainCannotMove is ExplainNoCd for the dashboard, which tried a session
// switch first.
func ExplainCannotMove(w *Writer, path string) {
	explainNoCd(w, path, "grove could not move you: no session could be switched, and shell integration is not loaded.")
}

func explainNoCd(w *Writer, path, headline string) {
	Warning(w, "%s", headline)
	Faint(w, "  Directory changes go through the grove shell function.")
	Faint(w, "  Running the binary by its path skips that function.")
	Faint(w, "  A shell opened before grove was set up needs a new shell.")
	_, _ = fmt.Fprintf(w, "\n")
	Faint(w, "  To load it, add this to ~/.zshrc or ~/.bashrc, then open a new shell:")
	Faint(w, "    eval \"$(grove install zsh)\"   # for zsh")
	Faint(w, "    eval \"$(grove install bash)\"  # for bash")
	_, _ = fmt.Fprintf(w, "\n")
	Faint(w, "  To change directory now:")
	Faint(w, "    cd %s", ShellQuote(path))
}

// ShellQuote wraps s in single quotes for safe interpolation into a POSIX
// shell command line. Each embedded single quote closes the quoting, adds an
// escaped quote, and opens it again.
func ShellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
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
