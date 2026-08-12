package mux

import (
	"os"
	"path/filepath"
)

// isExistingDir reports whether path is a directory that exists right now.
// One stat per session per listing, against grove's 500ms command budget.
func isExistingDir(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

// Index is a lookup over a single List() result, so a command that needs the
// status of every worktree pays for one backend call instead of one per tree.
//
// It resolves a Target against sessions using whichever key the backend can
// report. Path wins when both are present: herdr labels are cosmetic and a
// user renaming a workspace must not desync grove, while the checkout path is
// the thing grove and herdr genuinely agree on.
//
// A nil *Index behaves like an empty one — callers degrade a failed List into
// "no sessions" rather than an error, and shouldn't need a nil check.
type Index struct {
	byName map[string]Session
	byPath map[string]Session
}

// NewIndex builds a lookup over the sessions a backend reported.
func NewIndex(sessions []Session) *Index {
	ix := &Index{
		byName: make(map[string]Session, len(sessions)),
		byPath: make(map[string]Session, len(sessions)),
	}
	for _, s := range sessions {
		if s.Name != "" {
			if _, dup := ix.byName[s.Name]; !dup {
				ix.byName[s.Name] = s
			}
		}
		// A path the backend reports but that no longer exists cannot identify
		// any live worktree, and indexing it is actively harmful: herdr records
		// a workspace's checkout when it opens and never updates it, so after
		// `grove rename` the workspace still names the pre-rename directory.
		// Since Lookup prefers paths, that dead entry would shadow the real
		// workspace of whatever worktree later occupies the same path, and the
		// name fallback — the thing that makes rename self-heal — would never
		// run. Dropping it leaves such a workspace reachable by name only,
		// which is exactly right.
		if s.Path == "" || !isExistingDir(s.Path) {
			continue
		}
		// Index under both the cleaned and the symlink-resolved path so a
		// lookup matches whichever form the caller holds.
		for _, key := range pathKeys(s.Path) {
			if _, dup := ix.byPath[key]; !dup {
				ix.byPath[key] = s
			}
		}
	}
	return ix
}

// Lookup finds the session for a target, preferring a path match.
func (ix *Index) Lookup(t Target) (Session, bool) {
	if ix == nil {
		return Session{}, false
	}
	if t.Path != "" {
		for _, key := range pathKeys(t.Path) {
			if s, ok := ix.byPath[key]; ok {
				return s, true
			}
		}
	}
	if t.Name != "" {
		if s, ok := ix.byName[t.Name]; ok {
			return s, true
		}
	}
	return Session{}, false
}

// StatusFor returns the target's attachment status, or StatusNone.
func (ix *Index) StatusFor(t Target) Status {
	s, ok := ix.Lookup(t)
	if !ok || s.Status == "" {
		return StatusNone
	}
	return s.Status
}

// AgentFor returns the target's agent state, or AgentUnreported.
func (ix *Index) AgentFor(t Target) AgentStatus {
	s, ok := ix.Lookup(t)
	if !ok {
		return AgentUnreported
	}
	return s.Agent
}

// pathKeys returns the forms a path may be matched under: cleaned, and
// symlink-resolved when that differs and the path exists. Order matters —
// callers try them in sequence.
func pathKeys(path string) []string {
	clean := filepath.Clean(path)
	resolved, err := filepath.EvalSymlinks(clean)
	if err != nil || resolved == clean {
		return []string{clean}
	}
	return []string{clean, resolved}
}
