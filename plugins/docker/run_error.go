package docker

import (
	"fmt"
	"os"
	"regexp"
	"strings"
)

// dependencyFailureRE matches compose's "service \"X\" didn't complete successfully" message.
var dependencyFailureRE = regexp.MustCompile(`service "([^"]+)" didn't complete successfully`)

// connectionErrorRE matches connection/DNS failure patterns that suggest a
// missing depends_on service (database, cache, MQ, etc.).
var connectionErrorRE = regexp.MustCompile(`(?i)connection refused|no such host|temporary failure in name resolution|connection reset`)

// noDepHint is appended to the error message when a connection error is detected
// and the user is on the default --no-deps behavior (include_deps not explicitly set).
const noDepHint = "\nhint: this looks like a missing depends_on service. By default, `grove test` runs with --no-deps.\n" +
	"      Try `grove test --with-deps`, or set `[test] include_deps = true` in .grove/config.toml."

// translateRunError inspects captured compose stderr and rewrites the error
// when it matches a known unactionable pattern. Returns the original error
// when no pattern matches.
//
// includeDeps should be true when the caller explicitly opted into dependency
// resolution (via --with-deps or include_deps = true in config). When false
// (the default), a connection-error match appends a hint directing the user
// to --with-deps.
func translateRunError(stderr string, original error, includeDeps bool) error {
	if stderr == "" {
		return original
	}
	if m := dependencyFailureRE.FindStringSubmatch(stderr); len(m) == 2 {
		service := m[1]
		return fmt.Errorf(
			"service %q (a dependency of the test target) failed to start.\n"+
				"This is unrelated to the worktree under test.\n\n"+
				"Suggestions:\n"+
				"  • If you set include_deps = true in .grove/config.toml (or passed --with-deps), remove it — grove skips dependency services by default\n"+
				"  • Or run the test command directly in an ephemeral container:\n"+
				"      docker compose run --rm --no-deps -v $(pwd):/app <service> <test command>\n\n"+
				"underlying error: %w",
			service, original,
		)
	}
	if !includeDeps && connectionErrorRE.MatchString(stderr) {
		return fmt.Errorf("%w%s", original, strings.TrimRight(noDepHint, "\n"))
	}
	return original
}

const stderrBufferLimit = 8 * 1024

// teeBuffer mirrors writes to its inner writer while keeping the last
// stderrBufferLimit bytes in memory for inspection (e.g., to feed translateRunError).
type teeBuffer struct {
	w   *os.File
	buf []byte
}

func (t *teeBuffer) Write(p []byte) (int, error) {
	n := len(p)
	if t.w != nil {
		_, _ = t.w.Write(p)
	}
	if len(p) > stderrBufferLimit {
		p = p[len(p)-stderrBufferLimit:]
	}
	if len(t.buf)+len(p) > stderrBufferLimit {
		excess := len(t.buf) + len(p) - stderrBufferLimit
		if excess >= len(t.buf) {
			t.buf = nil
		} else {
			t.buf = t.buf[excess:]
		}
	}
	t.buf = append(t.buf, p...)
	return n, nil
}

func (t *teeBuffer) String() string { return string(t.buf) }
