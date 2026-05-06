package fsutil

import (
	"fmt"
	"path/filepath"
	"strings"
)

// SafeJoin joins a base directory with a user-supplied relative path
// and rejects results that escape the base directory.
//
// Use SafeJoin for any path constructed from user-controlled config
// (hooks.toml from/to, symlink_files entries) where the path is
// expected to live inside the worktree. Absolute paths supplied by
// grove internals (credential copy from ~/.config) should bypass
// SafeJoin and use filepath.Join directly.
func SafeJoin(base, rel string) (string, error) {
	if filepath.IsAbs(rel) {
		return "", fmt.Errorf("path %q escapes base directory %q", rel, base)
	}
	cleaned := filepath.Clean(rel)
	joined := filepath.Join(base, cleaned)
	relPath, err := filepath.Rel(base, joined)
	if err != nil {
		return "", err
	}
	if strings.HasPrefix(relPath, "..") {
		return "", fmt.Errorf("path %q escapes base directory %q", rel, base)
	}
	return joined, nil
}
