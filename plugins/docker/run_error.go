package docker

import (
	"fmt"
	"os"
	"regexp"
)

// dependencyFailureRE matches compose's "service \"X\" didn't complete successfully" message.
var dependencyFailureRE = regexp.MustCompile(`service "([^"]+)" didn't complete successfully`)

// translateRunError inspects captured compose stderr and rewrites the error
// when it matches a known unactionable pattern. Returns the original error
// when no pattern matches.
func translateRunError(stderr string, original error) error {
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
	if t.w != nil {
		_, _ = t.w.Write(p)
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
	return len(p), nil
}

func (t *teeBuffer) String() string { return string(t.buf) }
