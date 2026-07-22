package grove

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"sync"

	"github.com/lost-in-the/grove/internal/cmdexec"
)

// commonDirCache memoizes GitCommonDir per input directory. A worktree's common
// dir is stable for the life of a process, and several call sites resolve it on
// the same command (FindRoot, then EnsureGroveExcludes), so this collapses the
// duplicate `git rev-parse --git-common-dir` spawns to one. Only successful
// results are cached, so a transient failure is retried.
var commonDirCache sync.Map // dir string -> string

// GitCommonDir resolves the repository's shared .git directory from dir via
// `git rev-parse --git-common-dir`. For a linked worktree this is the MAIN
// worktree's .git directory, which makes it the anchor for everything that
// must not fragment across worktrees: the .grove project dir (FindRoot), the
// shared info/exclude file (EnsureGroveExcludes), and adoption checks.
//
// The returned path is absolute, cleaned, and symlink-resolved (best effort),
// so callers can compare it against other resolved paths. Errors when dir is
// not inside a git repository.
func GitCommonDir(dir string) (string, error) {
	if v, ok := commonDirCache.Load(dir); ok {
		return v.(string), nil
	}
	out, err := cmdexec.Output(context.TODO(), "git", []string{"-C", dir, "rev-parse", "--git-common-dir"}, "", cmdexec.GitLocal)
	if err != nil {
		return "", fmt.Errorf("resolve git common dir: %w", err)
	}
	commonDir := strings.TrimSpace(string(out))
	if commonDir == "" {
		return "", fmt.Errorf("resolve git common dir: empty output")
	}
	// Git prints the path relative to its own cwd (the -C dir) when possible.
	if !filepath.IsAbs(commonDir) {
		commonDir = filepath.Join(dir, commonDir)
	}
	commonDir = filepath.Clean(commonDir)
	if resolved, err := filepath.EvalSymlinks(commonDir); err == nil {
		commonDir = resolved
	}
	commonDirCache.Store(dir, commonDir)
	return commonDir, nil
}
