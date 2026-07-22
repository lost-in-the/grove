package grove

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/lost-in-the/grove/internal/fsutil"
)

// groveExcludeEntries are grove's machine-local .grove artifacts that must
// never show up as uncommitted changes in any worktree.
//
// Deliberately NOT listed: .grove/config.toml and .grove/hooks.toml — those
// are project-level files meant to be committed and shared (README: "commit
// this to your repo"). An earlier grove excluded config.toml to hide the
// per-worktree config symlink, which silently blocked `git add` of a freshly
// created config.toml in the main worktree; worktrees no longer carry a
// config copy at all (resolution is anchored at the main worktree via
// GitCommonDir), and EnsureGroveExcludes migrates old exclude blocks.
var groveExcludeEntries = []string{
	".grove/state.json",
	".grove/state.json.bak",
	".grove/state.lock",
	".grove/ui_prefs.json",
	".grove/.envrc",
	".grove/config.local.toml",
	// Atomic-write temp files (fsutil createUniqueTemp: "<file>.tmp-<pid>-<n>")
	// leak when the process dies between create and rename; a leaked temp
	// next to state.json or config.toml must not make the main worktree read
	// dirty (B4 symptoms). Any .tmp-* name directly under .grove is by
	// construction such a leftover, so one glob covers every atomic target.
	".grove/*.tmp-*",
	".grove/" + configMigrationSentinel,
}

// legacyGroveExcludeEntries were written by older grove versions; they are
// dropped from the managed block when it is rewritten.
var legacyGroveExcludeEntries = []string{
	".grove/config.toml",
}

// groveExcludeHeader marks the start of the grove-managed block in
// info/exclude. Only lines inside this block are ever rewritten.
const groveExcludeHeader = "# Grove (worktree manager) — machine-local, applies to all worktrees"

// EnsureGroveExcludes records grove's machine-local artifacts in the
// repository's shared exclude file ($GIT_COMMON_DIR/info/exclude) rather than
// a committed .gitignore. This matters for two reasons the old .gitignore
// approach got wrong (B4): the exclude file applies to *every* worktree (so
// grove-created worktrees aren't born dirty with an untracked .grove/, which
// used to make `grove ls` report them dirty, force `grove rm` to demand
// --force, and break `fork --copy-wip`), and it is never committed (so
// `grove init` doesn't leave the repo with an uncommitted .gitignore of its
// own).
//
// The entries live in a single managed block identified by
// groveExcludeHeader. Rewrites are confined to that block: user content
// before and after it is preserved verbatim, and a legacy block written by an
// older grove (which wrongly excluded the committable config.toml) is
// migrated in place. Idempotent — an up-to-date file is left untouched.
//
// Called from `grove init`, from worktree bootstrap, and from the shared
// command context, so fresh clones that never ran init on this machine still
// get the entries recorded and legacy repos self-heal on the first grove
// command after an upgrade.
//
// migrated is true only when a legacy entry was removed from the managed
// block — i.e. this invocation just un-ignored a committable project file.
// Callers surface that one-time event to the user (the notice never fires
// again because the rewrite is idempotent); routine first-time block writes
// return false.
func EnsureGroveExcludes(dir string) (migrated bool, err error) {
	commonDir, err := GitCommonDir(dir)
	if err != nil {
		return false, err
	}

	excludePath := filepath.Join(commonDir, "info", "exclude")
	content, err := os.ReadFile(excludePath)
	if err != nil && !os.IsNotExist(err) {
		return false, fmt.Errorf("read %s: %w", excludePath, err)
	}

	updated, changed, legacyRemoved := spliceGroveExcludeBlock(string(content))
	if !changed {
		return false, nil
	}

	// AtomicWriteFile creates the parent directory (info/) itself.
	if err := fsutil.AtomicWriteFile(excludePath, []byte(updated), 0o644); err != nil {
		return false, err
	}
	return legacyRemoved, nil
}

// spliceGroveExcludeBlock returns content with the grove-managed block
// replaced by (or appended as) the current desired block. changed is false
// when the file already contains exactly the desired block. legacyRemoved
// reports whether the replaced block contained entries from an older grove
// (e.g. the .grove/config.toml exclusion).
func spliceGroveExcludeBlock(content string) (updated string, changed, legacyRemoved bool) {
	desired := append([]string{groveExcludeHeader}, groveExcludeEntries...)

	lines := []string{}
	if content != "" {
		lines = strings.Split(content, "\n")
	}

	known := make(map[string]bool, len(groveExcludeEntries)+len(legacyGroveExcludeEntries))
	for _, e := range groveExcludeEntries {
		known[e] = true
	}
	for _, e := range legacyGroveExcludeEntries {
		known[e] = true
	}

	// Locate the managed block: the header line plus the run of known entries
	// immediately following it. Anything else — including a user's own
	// .grove/* lines outside the block — is left alone.
	start := -1
	end := -1 // exclusive
	for i, line := range lines {
		if strings.TrimSpace(line) == groveExcludeHeader {
			start = i
			end = i + 1
			for end < len(lines) && known[strings.TrimSpace(lines[end])] {
				end++
			}
			break
		}
	}

	if start >= 0 {
		legacy := make(map[string]bool, len(legacyGroveExcludeEntries))
		for _, e := range legacyGroveExcludeEntries {
			legacy[e] = true
		}
		current := make([]string, 0, end-start)
		for _, line := range lines[start:end] {
			trimmed := strings.TrimSpace(line)
			current = append(current, trimmed)
			if legacy[trimmed] {
				legacyRemoved = true
			}
		}
		if slices.Equal(current, desired) {
			return content, false, false
		}
		replaced := append([]string{}, lines[:start]...)
		replaced = append(replaced, desired...)
		replaced = append(replaced, lines[end:]...)
		return strings.Join(replaced, "\n"), true, legacyRemoved
	}

	// No managed block yet — append one, separated from existing content.
	var b strings.Builder
	b.WriteString(content)
	if content != "" && !strings.HasSuffix(content, "\n") {
		b.WriteString("\n")
	}
	if content != "" {
		b.WriteString("\n")
	}
	b.WriteString(strings.Join(desired, "\n"))
	b.WriteString("\n")
	return b.String(), true, false
}
