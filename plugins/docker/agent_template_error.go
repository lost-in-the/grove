package docker

import (
	"bytes"
	"fmt"
	"io"
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
// the preceding output. When unsetVars is empty, the original wrapping is
// preserved unchanged.
//
// unsetVars is expected to come from an unsetVarScanWriter that scanned
// compose's stderr as it streamed, rather than from a fixed-size trailing
// buffer — a stack that emits a lot of stderr after the warning would
// otherwise evict it before this function ever sees it.
func enrichAgentStackStartError(unsetVars []string, env []string, original error) error {
	if len(unsetVars) == 0 {
		return fmt.Errorf("failed to start agent stack: %w", original)
	}
	return fmt.Errorf(
		"failed to start agent stack: template referenced variable(s) grove did not export: %s\n"+
			"grove exports: %s\n"+
			"underlying error: %w",
		strings.Join(unsetVars, ", "), strings.Join(exportedEnvNames(env), ", "), original,
	)
}

// maxUnsetVars caps the number of distinct unset-variable names an
// unsetVarScanWriter collects, so a pathological or adversarial stream can't
// grow the set without bound.
const maxUnsetVars = 16

// unsetVarScanWriter tees writes to an inner writer (typically os.Stderr, so
// the user still sees live output) while incrementally scanning each
// complete line for compose's "variable is not set" warning. Detection is
// independent of how much stderr the stack produces after the warning: a
// stack that emits many kilobytes of pull/build noise after an early
// interpolation warning would evict that warning from a fixed-size trailing
// buffer (see stderrBufferLimit in run_error.go) and silently degrade to the
// opaque compose error. Scanning line-by-line as writes arrive avoids that.
//
// unsetVarScanWriter is not safe for concurrent use.
type unsetVarScanWriter struct {
	w       io.Writer
	partial []byte
	names   []string
	seen    map[string]bool
}

// newUnsetVarScanWriter returns a scanner that tees all writes to w.
func newUnsetVarScanWriter(w io.Writer) *unsetVarScanWriter {
	return &unsetVarScanWriter{
		w:    w,
		seen: make(map[string]bool),
	}
}

// maxPartialLine bounds the partial-line buffer. docker's progress rendering
// can rewrite lines with bare \r (handled below as a terminator), but a
// stream with no line terminator at all must not make the scanner buffer the
// whole stream while waiting for one — the teeBuffer this replaced capped
// memory at stderrBufferLimit, and a real unset-variable warning line is far
// shorter than this, so dropping an over-long line loses nothing we scan for.
const maxPartialLine = 8 * 1024

// Write implements io.Writer. It always tees the full input to the inner
// writer and reports len(p) written, regardless of how much of the line
// buffer was scanned.
func (u *unsetVarScanWriter) Write(p []byte) (int, error) {
	n := len(p)
	if u.w != nil {
		_, _ = u.w.Write(p)
	}

	u.partial = append(u.partial, p...)
	for {
		// \r counts as a terminator too: docker progress output rewrites
		// lines with bare carriage returns and may never emit \n.
		idx := bytes.IndexAny(u.partial, "\r\n")
		if idx < 0 {
			break
		}
		u.scanLine(u.partial[:idx])
		u.partial = u.partial[idx+1:]
	}
	if len(u.partial) > maxPartialLine {
		// Over-long unterminated line: cannot be a warning we scan for.
		// Drop it rather than grow without bound; the tee above already
		// forwarded every byte to the inner writer.
		u.partial = nil
	}
	return n, nil
}

// Flush scans any remaining partial line that never received a trailing
// newline (e.g. the process exited mid-line) and clears the buffer. Safe to
// call more than once; a second call is a no-op.
func (u *unsetVarScanWriter) Flush() {
	if len(u.partial) == 0 {
		return
	}
	u.scanLine(u.partial)
	u.partial = nil
}

// UnsetVars returns the collected unset-variable names seen so far, in
// first-seen order, deduplicated, capped at maxUnsetVars. Call Flush first
// to include a trailing partial line.
func (u *unsetVarScanWriter) UnsetVars() []string {
	return u.names
}

// scanLine applies the unset-variable warning pattern to a single completed
// line and merges any matches into the deduplicated, capped result set.
func (u *unsetVarScanWriter) scanLine(line []byte) {
	if len(u.names) >= maxUnsetVars {
		return
	}
	for _, name := range parseUnsetTemplateVars(string(line)) {
		if len(u.names) >= maxUnsetVars {
			return
		}
		if u.seen[name] {
			continue
		}
		u.seen[name] = true
		u.names = append(u.names, name)
	}
}
