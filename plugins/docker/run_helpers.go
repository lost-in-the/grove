package docker

import (
	"os"
	"os/exec"
)

// runWithErrorTranslation runs cmd, mirroring its output to os.Stderr while
// capturing the last 8KB of stderr in a teeBuffer. On non-nil exit, the
// captured stderr is fed to translateRunError(stderr, err, includeDeps) for
// dependency-failure rewriting and --no-deps hint detection. Stdin is wired
// through so interactive commands work.
//
// All three docker strategies (local, external, agent) call this so error
// translation stays consistent across them.
func runWithErrorTranslation(cmd *exec.Cmd, includeDeps bool) error {
	cmd.Stdout = os.Stderr
	stderrBuf := &teeBuffer{w: os.Stderr}
	cmd.Stderr = stderrBuf
	cmd.Stdin = os.Stdin
	if err := cmd.Run(); err != nil {
		return translateRunError(stderrBuf.String(), err, includeDeps)
	}
	return nil
}
