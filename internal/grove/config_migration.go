package grove

import (
	"context"
	"os"
	"path/filepath"
	"strings"

	"github.com/lost-in-the/grove/internal/cmdexec"
)

// configMigrationSentinel is a machine-local marker (kept out of git via the
// managed exclude block) recording that the one-time config-layout upgrade
// notice has already been shown for this clone.
const configMigrationSentinel = ".upgrade-notified"

// NeedsConfigMigrationNotice reports whether the one-time config-layout upgrade
// notice should be shown for this repo, and records that it has been so it fires
// exactly once per clone even though the legacy artifacts persist until the user
// cleans them up.
//
// The trigger is the presence of legacy per-worktree .grove/config.toml symlinks
// — what a repo created by pre-0.10 grove carries. (The exclude-block migration
// can't be the trigger for real upgraders: released grove never wrote
// config.toml into the shared exclude, so nothing is ever removed from it.)
//
// groveDir is the main worktree's .grove (holds the sentinel); projectRoot is
// used to enumerate worktrees. After the sentinel exists this is a single stat —
// the worktree walk runs at most once, and grove never creates new legacy
// symlinks, so a negative result stays negative.
func NeedsConfigMigrationNotice(groveDir, projectRoot string) bool {
	sentinel := filepath.Join(groveDir, configMigrationSentinel)
	if _, err := os.Stat(sentinel); err == nil {
		return false
	}
	needed := hasLegacyConfigSymlinks(projectRoot)
	// Write the sentinel regardless so the worktree walk runs only once.
	_ = os.WriteFile(sentinel, []byte("shown\n"), 0o644)
	return needed
}

// hasLegacyConfigSymlinks reports whether any worktree still carries the
// per-worktree .grove/config.toml symlink that pre-0.10 grove created. Current
// grove resolves config from the main worktree and never creates these, so
// their presence marks a repo that predates the config-layout change.
func hasLegacyConfigSymlinks(projectRoot string) bool {
	out, err := cmdexec.Output(context.TODO(), "git", []string{"-C", projectRoot, "worktree", "list", "--porcelain"}, "", cmdexec.GitLocal)
	if err != nil {
		return false
	}
	for _, line := range strings.Split(string(out), "\n") {
		path, found := strings.CutPrefix(line, "worktree ")
		if !found {
			continue
		}
		if _, err := os.Readlink(filepath.Join(path, ".grove", "config.toml")); err == nil {
			return true
		}
	}
	return false
}
